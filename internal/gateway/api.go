package gateway

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"agent-bridge/internal/profile"
	"agent-bridge/internal/security"
)

type paginationResponse struct {
	Limit      int  `json:"limit"`
	Offset     int  `json:"offset"`
	HasMore    bool `json:"has_more"`
	NextOffset *int `json:"next_offset,omitempty"`
}

type auditEventGroup struct {
	ID           string    `json:"id"`
	RunHistoryID string    `json:"run_history_id"`
	RunID        string    `json:"run_id,omitempty"`
	ProjectID    string    `json:"project_id,omitempty"`
	EventCount   int       `json:"event_count"`
	Types        []string  `json:"types"`
	FirstEventAt time.Time `json:"first_event_at"`
	LastEventAt  time.Time `json:"last_event_at"`
	Summary      string    `json:"summary"`
}

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

func (s *Server) authenticateConsoleAPI(w http.ResponseWriter, r *http.Request, scopes []string) (*authPrincipal, bool) {
	principal, apiErr := s.authenticate(r)
	if apiErr != nil {
		s.addAuthChallenge(w, r, strings.Join(scopes, " "))
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return nil, false
	}
	if !principalAllows(principal, scopes) {
		writeAPIError(w, http.StatusForbidden, "insufficient_scope", "Gateway token is missing required scope")
		return nil, false
	}
	if principal.WorkspaceID == "" {
		writeAPIError(w, http.StatusForbidden, "workspace_required", "Gateway token is not bound to a workspace")
		return nil, false
	}
	return principal, true
}

func (s *Server) handleWorkspace(w http.ResponseWriter, r *http.Request) {
	if apiErr := s.requireStore(); apiErr != nil {
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	principal, ok := s.authenticateConsoleAPI(w, r, []string{"projects:read"})
	if !ok {
		return
	}
	user, err := s.store.GetUser(r.Context(), principal.UserID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "gateway_store_error", err.Error())
		return
	}
	workspace, err := s.store.GetWorkspace(r.Context(), principal.WorkspaceID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "gateway_store_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user":            user,
		"workspace":       workspace,
		"mcp_url":         s.cfg.MCPURL,
		"public_base_url": s.cfg.PublicBaseURL,
		"mode":            "live",
	})
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

func (s *Server) handleExecutors(w http.ResponseWriter, r *http.Request) {
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
	if !principalAllows(principal, []string{"projects:read"}) {
		writeAPIError(w, http.StatusForbidden, "insufficient_scope", "Gateway token is missing required scope")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"executors": profile.List()})
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
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/gateway/v1/relays/"), "/"), "/")
	id := ""
	if len(parts) > 0 {
		id = parts[0]
	}
	if id == "" {
		writeAPIError(w, http.StatusBadRequest, "relay_profile_required", "relay profile id is required")
		return
	}
	if len(parts) == 2 && parts[1] == "health" {
		if r.Method != http.MethodGet {
			writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
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
		var latency *int64
		if profile.Enabled {
			started := time.Now()
			status = s.relayAvailability(r.Context(), profile.ToRelayProfile())
			ms := time.Since(started).Milliseconds()
			if status == "available" {
				latency = &ms
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"health": map[string]any{
			"relay_profile_id": profile.ID,
			"status":           status,
			"latency_ms":       latency,
			"checked_at":       time.Now().UTC(),
		}})
		return
	}
	if len(parts) > 1 {
		writeAPIError(w, http.StatusNotFound, "route_not_found", "relay route not found")
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

func (s *Server) handleMachines(w http.ResponseWriter, r *http.Request) {
	if apiErr := s.requireStore(); apiErr != nil {
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	principal, ok := s.authenticateConsoleAPI(w, r, []string{"projects:read"})
	if !ok {
		return
	}
	machines, err := s.store.ListMachines(r.Context(), principal.WorkspaceID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "gateway_store_error", err.Error())
		return
	}
	machines = s.mergeProjectLocationMachines(r.Context(), principal, machines)
	writeJSON(w, http.StatusOK, map[string]any{"machines": machines})
}

func (s *Server) handleConnectors(w http.ResponseWriter, r *http.Request) {
	if apiErr := s.requireStore(); apiErr != nil {
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	principal, ok := s.authenticateConsoleAPI(w, r, []string{"projects:read"})
	if !ok {
		return
	}
	connectors, err := s.store.ListConnectorBindings(r.Context(), principal.WorkspaceID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "gateway_store_error", err.Error())
		return
	}
	out := make([]map[string]any, 0, len(connectors))
	for _, connector := range connectors {
		out = append(out, map[string]any{
			"id":                 connector.ID,
			"workspace_id":       connector.WorkspaceID,
			"machine_id":         connector.MachineID,
			"relay_profile_id":   connector.RelayProfileID,
			"relay_connector_id": connector.RelayConnectorID,
			"relay_machine_id":   connector.RelayMachineID,
			"status":             connector.Status,
			"last_seen_at":       connector.LastSeenAt,
			"created_at":         connector.CreatedAt,
			"updated_at":         connector.UpdatedAt,
		})
	}
	out = s.mergeProjectLocationConnectors(r.Context(), principal, out)
	writeJSON(w, http.StatusOK, map[string]any{"connectors": out})
}

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	principal, ok := s.authenticateConsoleAPI(w, r, []string{"projects:read"})
	if !ok {
		return
	}
	projects, relayErrors := s.aggregateProjects(r.Context(), principal)
	writeJSON(w, http.StatusOK, map[string]any{"projects": projects, "relay_errors": relayErrors})
}

func (s *Server) handleRuns(w http.ResponseWriter, r *http.Request) {
	if apiErr := s.requireStore(); apiErr != nil {
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	principal, ok := s.authenticateConsoleAPI(w, r, []string{"projects:read", "runs:read"})
	if !ok {
		return
	}
	limit, offset := parseListPage(r, 100, 200)
	runs, err := s.store.ListRunRecords(r.Context(), principal.WorkspaceID, RunRecordFilters{
		ProjectID: r.URL.Query().Get("project_id"),
		Status:    r.URL.Query().Get("status"),
		Scope:     r.URL.Query().Get("scope"),
		Limit:     limit + 1,
		Offset:    offset,
	})
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "gateway_store_error", err.Error())
		return
	}
	hasMore := len(runs) > limit
	if hasMore {
		runs = runs[:limit]
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": runs, "pagination": buildPagination(limit, offset, hasMore)})
}

func (s *Server) handleRunByID(w http.ResponseWriter, r *http.Request) {
	if apiErr := s.requireStore(); apiErr != nil {
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	principal, ok := s.authenticateConsoleAPI(w, r, []string{"projects:read", "runs:read"})
	if !ok {
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/gateway/v1/runs/"), "/"), "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		writeAPIError(w, http.StatusBadRequest, "run_id_required", "run history id is required")
		return
	}
	record, err := s.store.GetRunRecord(r.Context(), principal.WorkspaceID, parts[0])
	if err == sql.ErrNoRows {
		writeAPIError(w, http.StatusNotFound, "run_not_found", "run not found")
		return
	}
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "gateway_store_error", err.Error())
		return
	}
	if len(parts) == 2 && parts[1] == "events" {
		limit, offset := parseListPage(r, 100, 200)
		events, err := s.store.ListAuditEvents(r.Context(), principal.WorkspaceID, AuditEventFilters{
			RunHistoryID: record.ID,
			Limit:        limit + 1,
			Offset:       offset,
		})
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "gateway_store_error", err.Error())
			return
		}
		hasMore := len(events) > limit
		if hasMore {
			events = events[:limit]
		}
		sort.SliceStable(events, func(i, j int) bool {
			return events[i].CreatedAt.Before(events[j].CreatedAt)
		})
		writeJSON(w, http.StatusOK, map[string]any{
			"events":     events,
			"groups":     groupAuditEvents(events),
			"pagination": buildPagination(limit, offset, hasMore),
		})
		return
	}
	if len(parts) > 1 {
		writeAPIError(w, http.StatusNotFound, "route_not_found", "run route not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"run": record})
}

func (s *Server) handleProjectByID(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/gateway/v1/projects/"), "/"), "/")
	projectID := ""
	if len(parts) > 0 {
		projectID = parts[0]
	}
	if projectID == "" {
		writeAPIError(w, http.StatusBadRequest, "project_id_required", "project id is required")
		return
	}
	if len(parts) == 2 && parts[1] == "runs" {
		s.handleProjectRunCreate(w, r, projectID)
		return
	}
	if len(parts) == 4 && parts[1] == "runs" && parts[3] == "report" {
		s.handleProjectRunReportGet(w, r, projectID, parts[2])
		return
	}
	if len(parts) > 1 {
		writeAPIError(w, http.StatusNotFound, "route_not_found", "project route not found")
		return
	}
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	principal, ok := s.authenticateConsoleAPI(w, r, []string{"projects:read"})
	if !ok {
		return
	}
	projects, relayErrors := s.aggregateProjects(r.Context(), principal)
	for _, project := range projects {
		if project.ProjectID == projectID {
			writeJSON(w, http.StatusOK, map[string]any{"project": project, "relay_errors": relayErrors})
			return
		}
	}
	writeAPIError(w, http.StatusNotFound, "project_not_found", "project not found")
}

func (s *Server) handleProjectRunCreate(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	principal, ok := s.authenticateConsoleAPI(w, r, []string{"projects:read", "runs:write"})
	if !ok {
		return
	}
	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "malformed_request", err.Error())
		return
	}
	if req == nil {
		req = map[string]any{}
	}
	req["project_id"] = projectID
	mode := strings.TrimSpace(stringArg(req, "mode"))
	runRecord, auditMetadata := s.beginRunRecord(r.Context(), principal, projectID, mode, req)
	s.recordGatewayAuditWithMetadata(r.Context(), principal, "task_submitted", "Submitted "+executionKindLabel(mode)+" for project "+projectID, auditMetadata)
	route := submitProjectTaskRoute
	if mode == "manifest" {
		route = runProjectManifestRoute
	}
	match, apiErr := s.resolveProject(r.Context(), principal, projectID, req, true)
	if apiErr != nil {
		runRecord.Status = "failed"
		runRecord.ResultSummary = "Route resolution failed: " + apiErr.Code
		runRecord, auditMetadata = s.finishRunRecord(r.Context(), runRecord, map[string]any{"status": "failed", "summary": runRecord.ResultSummary}, "unavailable")
		s.recordGatewayAuditWithMetadata(r.Context(), principal, "run_failed", "Route resolution failed for project "+projectID+": "+apiErr.Code, auditMetadata)
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	runRecord, auditMetadata = s.applyRouteToRunRecord(r.Context(), runRecord, principal, match, req)
	s.recordProjectRouteAudit(r.Context(), principal, match, req, auditMetadata)
	path, body, apiErr := route(req)
	if apiErr != nil {
		runRecord.Status = "failed"
		runRecord.ResultSummary = "Run request validation failed: " + apiErr.Code
		runRecord, auditMetadata = s.finishRunRecord(r.Context(), runRecord, map[string]any{"status": "failed", "summary": runRecord.ResultSummary}, "unavailable")
		s.recordGatewayAuditWithMetadata(r.Context(), principal, "run_failed", "Run request validation failed for project "+projectID+": "+apiErr.Code, auditMetadata)
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	path = appendSelector(path, req)
	s.recordGatewayAuditWithMetadata(r.Context(), principal, "run_started", "Started "+executionKindLabel(mode)+" for project "+projectID, auditMetadata)
	_, response, apiErr := s.callRelay(r.Context(), match.Profile, http.MethodPost, path, body)
	payload, apiErr := responsePayload(match.Profile, response, apiErr)
	if apiErr != nil {
		runRecord.Status = "failed"
		runRecord.ResultSummary = "Run failed: " + apiErr.Code
		runRecord, auditMetadata = s.finishRunRecord(r.Context(), runRecord, map[string]any{"status": "failed", "summary": runRecord.ResultSummary}, "unavailable")
		s.recordGatewayAuditWithMetadata(r.Context(), principal, "run_failed", "Run failed for project "+projectID+": "+apiErr.Code, auditMetadata)
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	runRecord, auditMetadata = s.finishRunRecord(r.Context(), runRecord, payload, "available")
	s.recordGatewayAuditWithMetadata(r.Context(), principal, terminalAuditType(payload), terminalAuditSummary(projectID, payload), auditMetadata)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "project_id": projectID, "run_history_id": runRecord.ID, "result": payload})
}

func (s *Server) handleProjectRunReportGet(w http.ResponseWriter, r *http.Request, projectID, runID string) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	principal, ok := s.authenticateConsoleAPI(w, r, []string{"projects:read", "runs:read", "reports:read"})
	if !ok {
		return
	}
	args := map[string]any{"project_id": projectID, "run_id": runID}
	for _, key := range []string{"relay_profile_id", "machine_id", "host_label"} {
		if value := strings.TrimSpace(r.URL.Query().Get(key)); value != "" {
			args[key] = value
		}
	}
	match, apiErr := s.resolveProject(r.Context(), principal, projectID, args, true)
	if apiErr != nil {
		s.recordGatewayAuditWithMetadata(r.Context(), principal, "report_read", "Run report route resolution failed for project "+projectID+": "+apiErr.Code, map[string]any{"project_id": projectID, "run_id": runID})
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	path := appendSelector("/api/v2/projects/"+projectID+"/reports/run-plans/"+runID, args)
	_, response, apiErr := s.callRelay(r.Context(), match.Profile, http.MethodGet, path, nil)
	payload, apiErr := responsePayload(match.Profile, response, apiErr)
	if apiErr != nil {
		s.recordGatewayAuditWithMetadata(r.Context(), principal, "report_read", "Run report read failed for project "+projectID+": "+apiErr.Code, map[string]any{"project_id": projectID, "run_id": runID, "relay_profile_id": match.Profile.ID})
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	runRecord, auditMetadata := s.refreshRunRecordFromReport(r.Context(), principal, match, args, payload)
	s.recordGatewayAuditWithMetadata(r.Context(), principal, "report_read", "Read run report "+runID+" for project "+projectID, auditMetadata)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "project_id": projectID, "run_id": runID, "run_history_id": runRecord.ID, "result": payload})
}

func (s *Server) handleAuditEvents(w http.ResponseWriter, r *http.Request) {
	if apiErr := s.requireStore(); apiErr != nil {
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	principal, ok := s.authenticateConsoleAPI(w, r, []string{"projects:read"})
	if !ok {
		return
	}
	limit, offset := parseListPage(r, 100, 200)
	eventType := firstNonEmpty(r.URL.Query().Get("type"), r.URL.Query().Get("event_type"))
	events, err := s.store.ListAuditEvents(r.Context(), principal.WorkspaceID, AuditEventFilters{
		Type:         eventType,
		ProjectID:    r.URL.Query().Get("project_id"),
		RunID:        r.URL.Query().Get("run_id"),
		RunHistoryID: r.URL.Query().Get("run_history_id"),
		Limit:        limit + 1,
		Offset:       offset,
	})
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "gateway_store_error", err.Error())
		return
	}
	hasMore := len(events) > limit
	if hasMore {
		events = events[:limit]
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"audit_events": events,
		"events":       events,
		"groups":       groupAuditEvents(events),
		"pagination":   buildPagination(limit, offset, hasMore),
	})
}

func parseListPage(r *http.Request, defaultLimit, maxLimit int) (int, int) {
	limit := defaultLimit
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			offset = parsed
		}
	}
	return limit, offset
}

func buildPagination(limit, offset int, hasMore bool) paginationResponse {
	page := paginationResponse{Limit: limit, Offset: offset, HasMore: hasMore}
	if hasMore {
		next := offset + limit
		page.NextOffset = &next
	}
	return page
}

func groupAuditEvents(events []AuditEvent) []auditEventGroup {
	byRun := map[string]*auditEventGroup{}
	order := []string{}
	for _, event := range events {
		runHistoryID := metadataString(event.Metadata, "run_history_id")
		if runHistoryID == "" {
			continue
		}
		group := byRun[runHistoryID]
		if group == nil {
			group = &auditEventGroup{
				ID:           "run:" + runHistoryID,
				RunHistoryID: runHistoryID,
				RunID:        metadataString(event.Metadata, "run_id"),
				ProjectID:    metadataString(event.Metadata, "project_id"),
				FirstEventAt: event.CreatedAt,
				LastEventAt:  event.CreatedAt,
			}
			byRun[runHistoryID] = group
			order = append(order, runHistoryID)
		}
		group.EventCount++
		if event.CreatedAt.Before(group.FirstEventAt) {
			group.FirstEventAt = event.CreatedAt
		}
		if event.CreatedAt.After(group.LastEventAt) {
			group.LastEventAt = event.CreatedAt
		}
		if !containsString(group.Types, event.Type) {
			group.Types = append(group.Types, event.Type)
		}
		if group.RunID == "" {
			group.RunID = metadataString(event.Metadata, "run_id")
		}
		if group.ProjectID == "" {
			group.ProjectID = metadataString(event.Metadata, "project_id")
		}
	}
	groups := make([]auditEventGroup, 0, len(order))
	for _, id := range order {
		group := byRun[id]
		sort.Strings(group.Types)
		runLabel := firstNonEmpty(group.RunID, group.RunHistoryID)
		group.Summary = fmt.Sprintf("%d lifecycle events for run %s", group.EventCount, runLabel)
		groups = append(groups, *group)
	}
	sort.SliceStable(groups, func(i, j int) bool {
		return groups[i].LastEventAt.After(groups[j].LastEventAt)
	})
	return groups
}

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, ok := metadata[key]
	if !ok {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	default:
		return ""
	}
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func (s *Server) handleActivationCommands(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	principal, ok := s.authenticateConsoleAPI(w, r, []string{"projects:read"})
	if !ok {
		return
	}
	gatewayURL := strings.TrimRight(s.cfg.PublicBaseURL, "/")
	mcpURL := strings.TrimRight(firstNonEmpty(s.cfg.MCPURL, gatewayURL+"/mcp"), "/")
	commands := []map[string]any{
		{"id": "login", "title": "Log in to Gateway", "description": "Creates a workspace-bound Gateway session under CODENCER_HOME.", "target": "gateway", "command": "codencer login --gateway " + gatewayURL},
		{"id": "connector-login", "title": "Bind local connector", "description": "Requests a short-lived Relay enrollment secret through Gateway; output is redacted.", "target": "gateway", "command": "codencer connector login --gateway " + gatewayURL + " --relay default --json"},
		{"id": "project-init", "title": "Create project config", "description": "Commits only .codencer/project.json; local state stays in CODENCER_HOME.", "target": "local", "command": "codencer project init --repo . --json"},
		{"id": "project-share", "title": "Share project explicitly", "description": "Connector advertises this project to the selected Relay.", "target": "local", "command": "codencer project share <project-id> --json"},
		{"id": "codex", "title": "Codex MCP setup", "description": "AI clients point to Gateway, not a user Relay.", "target": "client", "command": "codencer setup mcp --client codex --endpoint " + mcpURL + " --json"},
		{"id": "claude", "title": "Claude Code MCP setup", "description": "Generates the Gateway MCP command for Claude Code.", "target": "client", "command": "codencer setup mcp --client claude-code --endpoint " + mcpURL + " --json"},
		{"id": "chatgpt", "title": "ChatGPT custom MCP setup", "description": "Uses Gateway OAuth dev metadata for controlled testing.", "target": "client", "command": "codencer activation self-host --gateway " + gatewayURL + " --project <project-id> --token-env CODENCER_GATEWAY_MCP_TOKEN --json"},
		{"id": "curl", "title": "Gateway curl smoke", "description": "Runs MCP initialize/tools/list against Gateway.", "target": "gateway", "command": "curl -fsS " + mcpURL + " -H 'Authorization: Bearer $CODENCER_GATEWAY_MCP_TOKEN'"},
	}
	writeJSON(w, http.StatusOK, map[string]any{"activation_commands": commands, "commands": commands, "workspace_id": principal.WorkspaceID})
}

func (s *Server) mergeProjectLocationMachines(ctx context.Context, principal *authPrincipal, stored []MachineRecord) []MachineRecord {
	out := append([]MachineRecord{}, stored...)
	seen := make(map[string]bool, len(out))
	for _, machine := range out {
		if strings.TrimSpace(machine.ID) != "" {
			seen[machine.ID] = true
		}
	}
	projects, _ := s.aggregateProjects(ctx, principal)
	now := time.Now().UTC()
	for _, project := range projects {
		for _, relay := range project.RelayProfiles {
			for _, location := range relay.Locations {
				machineID := strings.TrimSpace(location.MachineID)
				if machineID == "" || seen[machineID] {
					continue
				}
				seen[machineID] = true
				status := locationStatus(location)
				out = append(out, MachineRecord{
					ID:          machineID,
					WorkspaceID: principal.WorkspaceID,
					UserID:      principal.UserID,
					Hostname:    location.Hostname,
					HostLabel:   firstNonEmpty(location.HostLabel, machineID),
					OS:          "unknown",
					Arch:        "unknown",
					Status:      status,
					CreatedAt:   now,
					UpdatedAt:   now,
				})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		left := firstNonEmpty(out[i].HostLabel, out[i].ID)
		right := firstNonEmpty(out[j].HostLabel, out[j].ID)
		if left == right {
			return out[i].ID < out[j].ID
		}
		return left < right
	})
	return out
}

func (s *Server) mergeProjectLocationConnectors(ctx context.Context, principal *authPrincipal, stored []map[string]any) []map[string]any {
	out := append([]map[string]any{}, stored...)
	seen := make(map[string]bool, len(out))
	for _, connector := range out {
		if id, _ := connector["id"].(string); strings.TrimSpace(id) != "" {
			seen[id] = true
		}
		if relayID, _ := connector["relay_connector_id"].(string); strings.TrimSpace(relayID) != "" {
			seen[relayID] = true
		}
	}
	projects, _ := s.aggregateProjects(ctx, principal)
	now := time.Now().UTC()
	for _, project := range projects {
		for _, relay := range project.RelayProfiles {
			for _, location := range relay.Locations {
				connectorID := strings.TrimSpace(location.ConnectorID)
				machineID := strings.TrimSpace(location.MachineID)
				if connectorID == "" || seen[connectorID] {
					continue
				}
				seen[connectorID] = true
				out = append(out, map[string]any{
					"id":                 connectorID,
					"workspace_id":       principal.WorkspaceID,
					"machine_id":         machineID,
					"relay_profile_id":   relay.RelayProfileID,
					"relay_connector_id": connectorID,
					"relay_machine_id":   machineID,
					"status":             locationStatus(location),
					"last_seen_at":       now,
					"created_at":         now,
					"updated_at":         now,
				})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		left, _ := out[i]["id"].(string)
		right, _ := out[j]["id"].(string)
		return left < right
	})
	return out
}

func locationStatus(location projectLocation) string {
	if location.Online {
		return "online"
	}
	status := strings.TrimSpace(location.Status)
	if status != "" {
		return status
	}
	return "offline"
}

func (s *Server) recordGatewayAudit(ctx context.Context, principal *authPrincipal, eventType, summary string) {
	s.recordGatewayAuditWithMetadata(ctx, principal, eventType, summary, nil)
}

func (s *Server) recordGatewayAuditWithMetadata(ctx context.Context, principal *authPrincipal, eventType, summary string, metadata map[string]any) {
	if s.store == nil || principal == nil || strings.TrimSpace(principal.WorkspaceID) == "" {
		return
	}
	_ = s.store.RecordAudit(ctx, AuditEvent{
		WorkspaceID: principal.WorkspaceID,
		ActorUserID: principal.UserID,
		Type:        strings.TrimSpace(eventType),
		Summary:     security.Redact(strings.TrimSpace(summary)),
		Metadata:    sanitizeMap(metadata),
	})
}

func (s *Server) recordProjectRouteAudit(ctx context.Context, principal *authPrincipal, match relayProjectMatch, args map[string]any, metadata map[string]any) {
	projectID := match.Project.ProjectID
	s.recordGatewayAuditWithMetadata(ctx, principal, "route_resolved", "Resolved project route for "+projectID, metadata)
	s.recordGatewayAuditWithMetadata(ctx, principal, "relay_selected", "Selected Relay profile "+match.Profile.ID+" for project "+projectID, metadata)
	location := selectedProjectLocation(match.Project, args)
	if location.ConnectorID != "" || location.MachineID != "" || location.HostLabel != "" {
		summary := "Selected connector " + firstNonEmpty(location.ConnectorID, "unknown") + " on machine " + firstNonEmpty(location.MachineID, location.HostLabel, "unknown") + " for project " + projectID
		s.recordGatewayAuditWithMetadata(ctx, principal, "connector_selected", summary, metadata)
	}
	executor := resolvedExecutorLabel(match.Project, args)
	if executor != "" {
		s.recordGatewayAuditWithMetadata(ctx, principal, "executor_selected", "Selected executor profile "+executor+" for project "+projectID, metadata)
	}
}

func (s *Server) handleOAuthConsent(w http.ResponseWriter, r *http.Request) {
	if s.oauth == nil {
		writeAPIError(w, http.StatusNotFound, "oauth_dev_not_configured", "Gateway OAuth dev mode is not enabled")
		return
	}
	switch r.Method {
	case http.MethodGet:
		values := r.URL.Query()
		writeJSON(w, http.StatusOK, map[string]any{
			"request": oauthConsentRequest(values, s.oauth.cfg.ClientID, s.oauth.defaultWorkspaceID, s.oauth.issuer(s.baseURL(r))),
		})
	case http.MethodPost:
		var req struct {
			Decision            string `json:"decision"`
			OperatorCode        string `json:"operator_code"`
			ResponseType        string `json:"response_type"`
			ClientID            string `json:"client_id"`
			RedirectURI         string `json:"redirect_uri"`
			Scope               string `json:"scope"`
			State               string `json:"state"`
			CodeChallenge       string `json:"code_challenge"`
			CodeChallengeMethod string `json:"code_challenge_method"`
			Resource            string `json:"resource"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAPIError(w, http.StatusBadRequest, "malformed_request", err.Error())
			return
		}
		values := url.Values{}
		values.Set("response_type", req.ResponseType)
		values.Set("client_id", req.ClientID)
		values.Set("redirect_uri", req.RedirectURI)
		values.Set("scope", req.Scope)
		values.Set("state", req.State)
		values.Set("code_challenge", req.CodeChallenge)
		values.Set("code_challenge_method", req.CodeChallengeMethod)
		values.Set("resource", req.Resource)
		if strings.EqualFold(strings.TrimSpace(req.Decision), "deny") {
			location, err := appendRedirectParams(req.RedirectURI, map[string]string{"error": "access_denied", "state": req.State})
			if err != nil {
				writeAPIError(w, http.StatusBadRequest, "invalid_redirect_uri", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true, "decision": "denied", "redirect_uri": location})
			return
		}
		values.Set("operator_code", req.OperatorCode)
		code, redirectURI, state, apiErr := s.oauth.CreateAuthorizationCode(values, s.baseURL(r))
		if apiErr != nil {
			writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
			return
		}
		location, err := appendRedirectParams(redirectURI, map[string]string{"code": code, "state": state})
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid_redirect_uri", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "decision": "approved", "redirect_uri": location, "authorization_code_issued": true})
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func oauthConsentRequest(values url.Values, defaultClientID, workspaceID, issuer string) map[string]any {
	scope := strings.Fields(strings.TrimSpace(values.Get("scope")))
	if len(scope) == 0 {
		scope = []string{"projects:read"}
	}
	resource := strings.TrimRight(strings.TrimSpace(values.Get("resource")), "/")
	if resource == "" {
		resource = strings.TrimRight(issuer, "/") + "/mcp"
	}
	return map[string]any{
		"response_type":         values.Get("response_type"),
		"client_id":             firstNonEmpty(values.Get("client_id"), defaultClientID),
		"client_name":           "Codencer MCP client",
		"workspace_id":          workspaceID,
		"redirect_uri":          values.Get("redirect_uri"),
		"scope":                 strings.Join(scope, " "),
		"scopes":                scope,
		"state":                 values.Get("state"),
		"code_challenge":        values.Get("code_challenge"),
		"code_challenge_method": values.Get("code_challenge_method"),
		"resource":              resource,
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
