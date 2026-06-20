package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"agent-bridge/internal/security"
)

func (s *Server) requireStore() *apiError {
	if s.store == nil {
		return &apiError{Status: http.StatusServiceUnavailable, Code: "gateway_store_not_configured", Message: "Gateway persistent store is not configured"}
	}
	return nil
}

func (s *Server) handleDeviceAuthorize(w http.ResponseWriter, r *http.Request) {
	if apiErr := s.requireStore(); apiErr != nil {
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	var req struct {
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	auth, err := s.store.CreateDeviceAuthorization(r.Context(), req.Email, req.DisplayName, strings.TrimRight(s.cfg.PublicBaseURL, "/")+"/device")
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "gateway_internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, auth)
}

func (s *Server) handleDeviceApprove(w http.ResponseWriter, r *http.Request) {
	if apiErr := s.requireStore(); apiErr != nil {
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	var req struct {
		UserCode string `json:"user_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "malformed_request", err.Error())
		return
	}
	account, err := s.store.ApproveDeviceCode(r.Context(), req.UserCode, s.cfg.DefaultRelay)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "approval_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "user": account.User, "workspace": account.Workspace})
}

func (s *Server) handleDeviceToken(w http.ResponseWriter, r *http.Request) {
	if apiErr := s.requireStore(); apiErr != nil {
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	var req struct {
		DeviceCode string `json:"device_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "malformed_request", err.Error())
		return
	}
	token, apiErr := s.store.PollDeviceToken(r.Context(), req.DeviceCode, s.cfg)
	if apiErr != nil {
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	writePrivateJSON(w, http.StatusOK, token)
}

func (s *Server) handleDevicePage(w http.ResponseWriter, r *http.Request) {
	if apiErr := s.requireStore(); apiErr != nil {
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><head><meta charset="utf-8"><title>Codencer Device Login</title></head><body><h1>Codencer Device Login</h1><form method="post"><label>Code <input name="user_code" autocomplete="one-time-code"></label><button type="submit">Approve</button></form></body></html>`))
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			writeAPIError(w, http.StatusBadRequest, "malformed_request", err.Error())
			return
		}
		account, err := s.store.ApproveDeviceCode(r.Context(), r.Form.Get("user_code"), s.cfg.DefaultRelay)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "approval_failed", err.Error())
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, "<!doctype html><html><body><h1>Approved</h1><p>Codencer login approved for workspace %s.</p></body></html>", account.Workspace.ID)
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (s *Server) handleWhoami(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	principal, apiErr := s.authenticate(r)
	if apiErr != nil {
		s.addAuthChallenge(w, r, "")
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":      principal.UserID,
		"workspace_id": principal.WorkspaceID,
		"scopes":       principal.Scopes,
		"mcp_url":      s.cfg.MCPURL,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	token := bearerToken(r.Header.Get("Authorization"))
	if s.store != nil && token != "" {
		_ = s.store.RevokeAccessToken(r.Context(), token)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleRelays(w http.ResponseWriter, r *http.Request) {
	if apiErr := s.requireStore(); apiErr != nil {
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	principal, apiErr := s.authenticate(r)
	if apiErr != nil {
		s.addAuthChallenge(w, r, "")
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	if principal.WorkspaceID == "" {
		writeAPIError(w, http.StatusForbidden, "workspace_required", "Gateway token is not bound to a workspace")
		return
	}
	switch r.Method {
	case http.MethodGet:
		if !principalAllows(principal, []string{"projects:read"}) {
			writeAPIError(w, http.StatusForbidden, "insufficient_scope", "Gateway token is missing required scope")
			return
		}
		profiles, err := s.store.ListRelayProfiles(r.Context(), principal.WorkspaceID)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "gateway_internal_error", err.Error())
			return
		}
		out := make([]map[string]any, 0, len(profiles))
		for _, profile := range profiles {
			status := "disabled"
			if profile.Enabled {
				status = s.relayAvailability(r.Context(), profile.ToRelayProfile())
			}
			out = append(out, profile.SafeMap(status))
		}
		writeJSON(w, http.StatusOK, map[string]any{"relays": out})
	case http.MethodPost:
		if !principalAllows(principal, []string{"projects:write"}) {
			writeAPIError(w, http.StatusForbidden, "insufficient_scope", "Gateway token is missing required scope")
			return
		}
		var req struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			URL       string `json:"url"`
			TokenEnv  string `json:"token_env"`
			TokenFile string `json:"token_file"`
			Type      string `json:"type"`
			Enabled   *bool  `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAPIError(w, http.StatusBadRequest, "malformed_request", err.Error())
			return
		}
		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}
		profile, err := s.store.UpsertRelayProfile(r.Context(), RelayProfileRecord{
			ID:          req.ID,
			WorkspaceID: principal.WorkspaceID,
			Type:        firstNonEmpty(req.Type, "self_host"),
			Name:        req.Name,
			URL:         req.URL,
			TokenEnv:    req.TokenEnv,
			TokenFile:   req.TokenFile,
			Enabled:     enabled,
			Status:      "available",
		})
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "relay_profile_invalid", err.Error())
			return
		}
		_ = s.store.RecordAudit(r.Context(), AuditEvent{WorkspaceID: principal.WorkspaceID, ActorUserID: principal.UserID, Type: "relay.add", Summary: "Added or updated Gateway Relay profile " + profile.ID})
		writeJSON(w, http.StatusOK, map[string]any{"relay": profile.SafeMap("")})
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (s *Server) handleRelayByID(w http.ResponseWriter, r *http.Request) {
	if apiErr := s.requireStore(); apiErr != nil {
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	principal, apiErr := s.authenticate(r)
	if apiErr != nil {
		s.addAuthChallenge(w, r, "")
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/gateway/v1/relays/"), "/")
	if id == "" {
		writeAPIError(w, http.StatusBadRequest, "relay_profile_required", "relay profile id is required")
		return
	}
	switch r.Method {
	case http.MethodGet:
		if !principalAllows(principal, []string{"projects:read"}) {
			writeAPIError(w, http.StatusForbidden, "insufficient_scope", "Gateway token is missing required scope")
			return
		}
		profile, err := s.store.GetRelayProfile(r.Context(), principal.WorkspaceID, id)
		if err != nil {
			writeAPIError(w, http.StatusNotFound, "relay_profile_not_found", "relay profile not found")
			return
		}
		status := "disabled"
		if profile.Enabled {
			status = s.relayAvailability(r.Context(), profile.ToRelayProfile())
		}
		writeJSON(w, http.StatusOK, map[string]any{"relay": profile.SafeMap(status)})
	case http.MethodDelete:
		if !principalAllows(principal, []string{"projects:write"}) {
			writeAPIError(w, http.StatusForbidden, "insufficient_scope", "Gateway token is missing required scope")
			return
		}
		if err := s.store.RemoveRelayProfile(r.Context(), principal.WorkspaceID, id); err != nil {
			writeAPIError(w, http.StatusBadRequest, "relay_profile_remove_failed", err.Error())
			return
		}
		_ = s.store.RecordAudit(r.Context(), AuditEvent{WorkspaceID: principal.WorkspaceID, ActorUserID: principal.UserID, Type: "relay.remove", Summary: "Removed Gateway Relay profile " + id})
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "relay_profile_id": id})
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (s *Server) handleConnectorLogin(w http.ResponseWriter, r *http.Request) {
	if apiErr := s.requireStore(); apiErr != nil {
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	principal, apiErr := s.authenticate(r)
	if apiErr != nil {
		s.addAuthChallenge(w, r, "")
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	if !principalAllows(principal, []string{"projects:write"}) {
		writeAPIError(w, http.StatusForbidden, "insufficient_scope", "Gateway token is missing required scope")
		return
	}
	var req struct {
		RelayProfileID string `json:"relay_profile_id"`
		Relay          string `json:"relay"`
		Machine        struct {
			MachineID string `json:"machine_id"`
			Hostname  string `json:"hostname"`
			HostLabel string `json:"host_label"`
			OS        string `json:"os"`
			Arch      string `json:"arch"`
		} `json:"machine"`
		Label string `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "malformed_request", err.Error())
		return
	}
	relayID := firstNonEmpty(req.RelayProfileID, req.Relay, "default")
	if relayID == "managed" || relayID == "official" {
		relayID = "default"
	}
	machine, err := s.store.UpsertMachine(r.Context(), MachineRecord{
		ID:          req.Machine.MachineID,
		WorkspaceID: principal.WorkspaceID,
		UserID:      principal.UserID,
		Hostname:    req.Machine.Hostname,
		HostLabel:   req.Machine.HostLabel,
		OS:          req.Machine.OS,
		Arch:        req.Machine.Arch,
		Status:      "active",
	})
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "machine_binding_failed", err.Error())
		return
	}
	profile, err := s.store.GetRelayProfile(r.Context(), principal.WorkspaceID, relayID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, "relay_profile_not_found", "relay profile not found")
		return
	}
	binding, err := s.store.CreateConnectorBinding(r.Context(), principal.WorkspaceID, machine.ID, profile.ID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "connector_binding_failed", err.Error())
		return
	}
	secret, err := s.createRelayEnrollmentToken(r.Context(), profile, firstNonEmpty(req.Label, machine.HostLabel, machine.ID))
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, "relay_unavailable", err.Error())
		return
	}
	_ = s.store.RecordAudit(r.Context(), AuditEvent{WorkspaceID: principal.WorkspaceID, ActorUserID: principal.UserID, Type: "connector.login", Summary: "Created connector login binding " + binding.ID})
	writePrivateJSON(w, http.StatusOK, map[string]any{
		"binding_id":         binding.ID,
		"workspace_id":       principal.WorkspaceID,
		"machine":            machine,
		"relay_profile":      profile.SafeMap("available"),
		"relay_url":          profile.URL,
		"enrollment_token":   secret,
		"expires_in_seconds": 600,
	})
}

func (s *Server) handleConnectorComplete(w http.ResponseWriter, r *http.Request) {
	if apiErr := s.requireStore(); apiErr != nil {
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	principal, apiErr := s.authenticate(r)
	if apiErr != nil {
		s.addAuthChallenge(w, r, "")
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	if !principalAllows(principal, []string{"projects:write"}) {
		writeAPIError(w, http.StatusForbidden, "insufficient_scope", "Gateway token is missing required scope")
		return
	}
	var req struct {
		BindingID        string `json:"binding_id"`
		RelayConnectorID string `json:"relay_connector_id"`
		RelayMachineID   string `json:"relay_machine_id"`
		PublicKey        string `json:"public_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "malformed_request", err.Error())
		return
	}
	existing, err := s.store.GetConnectorBinding(r.Context(), req.BindingID)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "connector_binding_failed", err.Error())
		return
	}
	if existing.WorkspaceID != principal.WorkspaceID {
		writeAPIError(w, http.StatusForbidden, "workspace_mismatch", "connector binding does not belong to this workspace")
		return
	}
	binding, err := s.store.CompleteConnectorBinding(r.Context(), req.BindingID, req.RelayConnectorID, req.RelayMachineID, req.PublicKey)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "connector_binding_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "binding": binding})
}

func (s *Server) createRelayEnrollmentToken(ctx context.Context, profile RelayProfileRecord, label string) (string, error) {
	token, err := profile.ToRelayProfile().Token()
	if err != nil {
		return "", err
	}
	body, _ := json.Marshal(map[string]any{"label": label, "expires_in_seconds": 600})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(profile.URL, "/")+"/api/v2/connectors/enrollment-tokens", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("relay enrollment token failed: %s", security.Redact(string(data)))
	}
	var decoded struct {
		Secret string `json:"secret"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return "", err
	}
	if strings.TrimSpace(decoded.Secret) == "" {
		return "", fmt.Errorf("relay did not return enrollment secret")
	}
	return decoded.Secret, nil
}
