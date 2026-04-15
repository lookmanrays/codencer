package cloud

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	cloudconnectors "agent-bridge/internal/cloud/connectors"
)

type cloudStatusResponse struct {
	Status             string                     `json:"status"`
	Version            string                     `json:"version"`
	StartedAt          string                     `json:"started_at"`
	CloudAPIBase       string                     `json:"cloud_api_base"`
	RelayComposed      bool                       `json:"relay_composed"`
	ConnectorProviders []cloudconnectors.Provider `json:"connector_providers"`
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/api/cloud/v1/status", s.handleStatus)
	mux.HandleFunc("/api/cloud/v1/orgs", s.handleOrgs)
	mux.HandleFunc("/api/cloud/v1/workspaces", s.handleWorkspaces)
	mux.HandleFunc("/api/cloud/v1/projects", s.handleProjects)
	mux.HandleFunc("/api/cloud/v1/tokens", s.handleTokens)
	mux.HandleFunc("/api/cloud/v1/tokens/", s.handleTokenByID)
	mux.HandleFunc("/api/cloud/v1/installations", s.handleInstallations)
	mux.HandleFunc("/api/cloud/v1/installations/", s.handleInstallationByID)
	mux.HandleFunc("/api/cloud/v1/runtime/connectors", s.handleRuntimeConnectors)
	mux.HandleFunc("/api/cloud/v1/runtime/connectors/", s.handleRuntimeConnectorByID)
	mux.HandleFunc("/api/cloud/v1/runtime/instances", s.handleRuntimeInstances)
	mux.HandleFunc("/api/cloud/v1/runtime/instances/", s.handleRuntimeInstanceByID)
	mux.HandleFunc("/api/cloud/v1/events", s.handleEvents)
	mux.HandleFunc("/api/cloud/v1/audit", s.handleAudit)
	mux.HandleFunc("/", s.handleRoot)
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireToken(w, r, "cloud:read"); !ok {
		return
	}
	providers := []cloudconnectors.Provider{}
	if s.connectors != nil {
		providers = s.connectors.Names()
	}
	writeJSON(w, http.StatusOK, cloudStatusResponse{
		Status:             "ok",
		Version:            "cloud-v1-alpha",
		StartedAt:          s.startedAt.UTC().Format(time.RFC3339),
		CloudAPIBase:       "/api/cloud/v1",
		RelayComposed:      s.runtime != nil && s.runtime.Server != nil && s.runtime.Store != nil,
		ConnectorProviders: providers,
	})
}

func (s *Server) handleOrgs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		token, ok := s.requireToken(w, r, "orgs:read")
		if !ok {
			return
		}
		orgs, err := s.store.ListOrgs(r.Context())
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "list_failed", err.Error())
			return
		}
		if token.OrgID != "" {
			filtered := make([]Org, 0, len(orgs))
			for _, org := range orgs {
				if org.ID == token.OrgID {
					filtered = append(filtered, org)
				}
			}
			orgs = filtered
		}
		writeJSON(w, http.StatusOK, orgs)
	case http.MethodPost:
		token, ok := s.requireToken(w, r, "orgs:write")
		if !ok {
			return
		}
		var req Org
		if err := decodeJSON(r.Body, &req); err != nil {
			writeAPIError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		if token.OrgID != "" && req.ID != "" && token.OrgID != req.ID {
			writeAPIError(w, http.StatusForbidden, "scope_denied", "token is not allowed to create another org")
			return
		}
		created, err := s.store.CreateOrg(r.Context(), req)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "create_failed", err.Error())
			return
		}
		s.recordAudit(r, token, "create_org", "org", created.ID, created.OrgID(), "", "", "ok", map[string]any{"slug": created.Slug})
		writeJSON(w, http.StatusCreated, created)
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (s *Server) handleWorkspaces(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		token, ok := s.requireToken(w, r, "workspaces:read")
		if !ok {
			return
		}
		orgID := firstNonEmpty(r.URL.Query().Get("org_id"), token.OrgID)
		if !TokenAllowsTarget(token, orgID, "", "") {
			writeAPIError(w, http.StatusForbidden, "scope_denied", "token is not allowed to read this org")
			return
		}
		workspaces, err := s.store.ListWorkspaces(r.Context(), orgID)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "list_failed", err.Error())
			return
		}
		if token.WorkspaceID != "" {
			filtered := make([]Workspace, 0, len(workspaces))
			for _, workspace := range workspaces {
				if workspace.ID == token.WorkspaceID {
					filtered = append(filtered, workspace)
				}
			}
			workspaces = filtered
		}
		writeJSON(w, http.StatusOK, workspaces)
	case http.MethodPost:
		token, ok := s.requireToken(w, r, "workspaces:write")
		if !ok {
			return
		}
		var req Workspace
		if err := decodeJSON(r.Body, &req); err != nil {
			writeAPIError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		if !TokenAllowsTarget(token, req.OrgID, "", "") {
			writeAPIError(w, http.StatusForbidden, "scope_denied", "token is not allowed to create a workspace in this org")
			return
		}
		created, err := s.store.CreateWorkspace(r.Context(), req)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "create_failed", err.Error())
			return
		}
		s.recordAudit(r, token, "create_workspace", "workspace", created.ID, created.OrgID, created.ID, "", "ok", map[string]any{"slug": created.Slug})
		writeJSON(w, http.StatusCreated, created)
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		token, ok := s.requireToken(w, r, "projects:read")
		if !ok {
			return
		}
		workspaceID := firstNonEmpty(r.URL.Query().Get("workspace_id"), token.WorkspaceID)
		projects, err := s.store.ListProjects(r.Context(), workspaceID)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "list_failed", err.Error())
			return
		}
		filtered := make([]Project, 0, len(projects))
		for _, project := range projects {
			if !TokenAllowsTarget(token, project.OrgID, project.WorkspaceID, project.ID) {
				continue
			}
			filtered = append(filtered, project)
		}
		writeJSON(w, http.StatusOK, filtered)
	case http.MethodPost:
		token, ok := s.requireToken(w, r, "projects:write")
		if !ok {
			return
		}
		var req Project
		if err := decodeJSON(r.Body, &req); err != nil {
			writeAPIError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		if !TokenAllowsTarget(token, req.OrgID, req.WorkspaceID, "") {
			writeAPIError(w, http.StatusForbidden, "scope_denied", "token is not allowed to create a project in this workspace")
			return
		}
		created, err := s.store.CreateProject(r.Context(), req)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "create_failed", err.Error())
			return
		}
		s.recordAudit(r, token, "create_project", "project", created.ID, created.OrgID, created.WorkspaceID, created.ID, "ok", map[string]any{"slug": created.Slug})
		writeJSON(w, http.StatusCreated, created)
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (s *Server) handleTokens(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		token, ok := s.requireToken(w, r, "tokens:read")
		if !ok {
			return
		}
		orgID := firstNonEmpty(r.URL.Query().Get("org_id"), token.OrgID)
		workspaceID := firstNonEmpty(r.URL.Query().Get("workspace_id"), token.WorkspaceID)
		projectID := firstNonEmpty(r.URL.Query().Get("project_id"), token.ProjectID)
		if !TokenAllowsTarget(token, orgID, workspaceID, projectID) {
			writeAPIError(w, http.StatusForbidden, "scope_denied", "token is not allowed to read this token scope")
			return
		}
		tokens, err := s.store.ListAPITokens(r.Context(), orgID, workspaceID, projectID)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "list_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, tokens)
	case http.MethodPost:
		token, ok := s.requireToken(w, r, "tokens:write")
		if !ok {
			return
		}
		var req struct {
			OrgID       string   `json:"org_id"`
			WorkspaceID string   `json:"workspace_id,omitempty"`
			ProjectID   string   `json:"project_id,omitempty"`
			Name        string   `json:"name"`
			Kind        string   `json:"kind,omitempty"`
			Scopes      []string `json:"scopes"`
		}
		if err := decodeJSON(r.Body, &req); err != nil {
			writeAPIError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		if !TokenAllowsTarget(token, req.OrgID, req.WorkspaceID, req.ProjectID) {
			writeAPIError(w, http.StatusForbidden, "scope_denied", "token is not allowed to create a token for this scope")
			return
		}
		raw, err := GenerateAPIToken()
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "token_generation_failed", err.Error())
			return
		}
		record, err := s.store.CreateAPIToken(r.Context(), APIToken{
			OrgID:       req.OrgID,
			WorkspaceID: req.WorkspaceID,
			ProjectID:   req.ProjectID,
			Name:        req.Name,
			Kind:        req.Kind,
			Scopes:      req.Scopes,
		}, raw)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "create_failed", err.Error())
			return
		}
		s.recordAudit(r, token, "create_api_token", "api_token", record.ID, record.OrgID, record.WorkspaceID, record.ProjectID, "ok", map[string]any{"name": record.Name, "scopes": record.Scopes})
		writeJSON(w, http.StatusCreated, map[string]any{
			"token":  raw,
			"record": record,
		})
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (s *Server) handleTokenByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/cloud/v1/tokens/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 || parts[1] != "revoke" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	token, ok := s.requireToken(w, r, "tokens:write")
	if !ok {
		return
	}
	if err := s.store.RevokeAPIToken(r.Context(), parts[0]); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "revoke_failed", err.Error())
		return
	}
	s.recordAudit(r, token, "revoke_api_token", "api_token", parts[0], token.OrgID, token.WorkspaceID, token.ProjectID, "ok", nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked", "token_id": parts[0]})
}

func (s *Server) handleInstallations(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		token, ok := s.requireToken(w, r, "installations:read")
		if !ok {
			return
		}
		orgID := firstNonEmpty(r.URL.Query().Get("org_id"), token.OrgID)
		workspaceID := firstNonEmpty(r.URL.Query().Get("workspace_id"), token.WorkspaceID)
		projectID := firstNonEmpty(r.URL.Query().Get("project_id"), token.ProjectID)
		if !TokenAllowsTarget(token, orgID, workspaceID, projectID) {
			writeAPIError(w, http.StatusForbidden, "scope_denied", "token is not allowed to read these installations")
			return
		}
		installations, err := s.store.ListConnectorInstallations(r.Context(), orgID, workspaceID, projectID)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "list_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, installations)
	case http.MethodPost:
		token, ok := s.requireToken(w, r, "installations:write")
		if !ok {
			return
		}
		var req struct {
			OrgID                  string            `json:"org_id"`
			WorkspaceID            string            `json:"workspace_id,omitempty"`
			ProjectID              string            `json:"project_id,omitempty"`
			ConnectorKey           string            `json:"connector_key"`
			ExternalInstallationID string            `json:"external_installation_id,omitempty"`
			ExternalAccount        string            `json:"external_account,omitempty"`
			Name                   string            `json:"name,omitempty"`
			Config                 map[string]string `json:"config,omitempty"`
			Secrets                map[string]string `json:"secrets,omitempty"`
		}
		if err := decodeJSON(r.Body, &req); err != nil {
			writeAPIError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		if !TokenAllowsTarget(token, req.OrgID, req.WorkspaceID, req.ProjectID) {
			writeAPIError(w, http.StatusForbidden, "scope_denied", "token is not allowed to create an installation for this scope")
			return
		}
		if s.connectors == nil {
			writeAPIError(w, http.StatusInternalServerError, "connector_registry_missing", "cloud connector registry is not configured")
			return
		}
		if _, exists := s.connectors.Get(cloudconnectors.Provider(req.ConnectorKey)); !exists {
			writeAPIError(w, http.StatusBadRequest, "unsupported_connector", "no registered connector matches the installation")
			return
		}
		cfgJSON, err := json.Marshal(req.Config)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "bad_config", err.Error())
			return
		}
		installation, err := s.store.CreateConnectorInstallation(r.Context(), ConnectorInstallation{
			OrgID:                  req.OrgID,
			WorkspaceID:            req.WorkspaceID,
			ProjectID:              req.ProjectID,
			ConnectorKey:           req.ConnectorKey,
			ExternalInstallationID: req.ExternalInstallationID,
			ExternalAccount:        req.ExternalAccount,
			Name:                   req.Name,
			Status:                 "created",
			Enabled:                true,
			ConfigJSON:             cfgJSON,
		})
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "create_failed", err.Error())
			return
		}
		for key, value := range req.Secrets {
			if _, err := s.store.PutInstallationSecret(r.Context(), installation.ID, key, []byte(value)); err != nil {
				_ = s.store.DeleteConnectorInstallation(r.Context(), installation.ID)
				writeAPIError(w, http.StatusInternalServerError, "secret_store_failed", err.Error())
				return
			}
		}
		s.recordAudit(r, token, "create_installation", "installation", installation.ID, installation.OrgID, installation.WorkspaceID, installation.ProjectID, "ok", map[string]any{"connector_key": installation.ConnectorKey})
		writeJSON(w, http.StatusCreated, installation)
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (s *Server) handleInstallationByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/cloud/v1/installations/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	installationID := parts[0]
	sub := ""
	if len(parts) > 1 {
		sub = parts[1]
	}
	switch {
	case sub == "" && r.Method == http.MethodGet:
		s.handleInstallationGet(w, r, installationID)
	case sub == "validate" && r.Method == http.MethodPost:
		s.handleInstallationValidate(w, r, installationID)
	case sub == "enable" && r.Method == http.MethodPost:
		s.handleInstallationEnableDisable(w, r, installationID, true)
	case sub == "disable" && r.Method == http.MethodPost:
		s.handleInstallationEnableDisable(w, r, installationID, false)
	case sub == "actions" && r.Method == http.MethodPost:
		s.handleInstallationAction(w, r, installationID)
	case sub == "webhook" && r.Method == http.MethodPost:
		s.handleInstallationWebhook(w, r, installationID)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleInstallationGet(w http.ResponseWriter, r *http.Request, installationID string) {
	token, ok := s.requireToken(w, r, "installations:read")
	if !ok {
		return
	}
	installation, err := s.store.GetConnectorInstallation(r.Context(), installationID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	if !TokenAllowsTarget(token, installation.OrgID, installation.WorkspaceID, installation.ProjectID) {
		writeAPIError(w, http.StatusForbidden, "scope_denied", "token is not allowed to read this installation")
		return
	}
	writeJSON(w, http.StatusOK, installation)
}

func (s *Server) handleInstallationValidate(w http.ResponseWriter, r *http.Request, installationID string) {
	token, ok := s.requireToken(w, r, "installations:write")
	if !ok {
		return
	}
	installation, connector, cfg, ok := s.resolveInstallationConnector(w, r, installationID)
	if !ok {
		return
	}
	if !TokenAllowsTarget(token, installation.OrgID, installation.WorkspaceID, installation.ProjectID) {
		writeAPIError(w, http.StatusForbidden, "scope_denied", "token is not allowed to validate this installation")
		return
	}
	result, err := connector.ValidateInstallation(r.Context(), cfg)
	if err != nil {
		installation.Status = "error"
		installation.LastError = err.Error()
	} else {
		installation.Status = "active"
		installation.LastError = ""
		now := time.Now().UTC()
		installation.LastSeenAt = &now
	}
	installation, _ = s.store.CreateConnectorInstallation(r.Context(), *installation)
	status := connector.DeriveStatus(result, cloudconnectors.WebhookVerification{})
	s.recordAudit(r, token, "validate_installation", "installation", installation.ID, installation.OrgID, installation.WorkspaceID, installation.ProjectID, outcomeFromErr(err), map[string]any{"provider": connector.Provider(), "message": result.Message})
	writeJSON(w, httpStatusFromErr(err, http.StatusOK), map[string]any{
		"validation": result,
		"status":     status,
		"error":      errorString(err),
	})
}

func (s *Server) handleInstallationEnableDisable(w http.ResponseWriter, r *http.Request, installationID string, enabled bool) {
	token, ok := s.requireToken(w, r, "installations:write")
	if !ok {
		return
	}
	installation, err := s.store.GetConnectorInstallation(r.Context(), installationID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	if !TokenAllowsTarget(token, installation.OrgID, installation.WorkspaceID, installation.ProjectID) {
		writeAPIError(w, http.StatusForbidden, "scope_denied", "token is not allowed to update this installation")
		return
	}
	installation.Enabled = enabled
	if enabled {
		if installation.Status == "disabled" {
			installation.Status = "created"
		}
		installation.LastError = ""
	} else {
		installation.Status = "disabled"
	}
	updated, err := s.store.CreateConnectorInstallation(r.Context(), *installation)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "update_failed", err.Error())
		return
	}
	action := "disable_installation"
	if enabled {
		action = "enable_installation"
	}
	s.recordAudit(r, token, action, "installation", updated.ID, updated.OrgID, updated.WorkspaceID, updated.ProjectID, "ok", map[string]any{"enabled": updated.Enabled})
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleInstallationAction(w http.ResponseWriter, r *http.Request, installationID string) {
	token, ok := s.requireToken(w, r, "installations:write")
	if !ok {
		return
	}
	installation, connector, cfg, ok := s.resolveInstallationConnector(w, r, installationID)
	if !ok {
		return
	}
	if !TokenAllowsTarget(token, installation.OrgID, installation.WorkspaceID, installation.ProjectID) {
		writeAPIError(w, http.StatusForbidden, "scope_denied", "token is not allowed to act on this installation")
		return
	}
	if !installation.Enabled {
		writeAPIError(w, http.StatusConflict, "installation_disabled", "connector installation is disabled")
		return
	}
	var req cloudconnectors.ActionRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	result, err := connector.ExecuteAction(r.Context(), req, cfg)
	responseJSON, _ := json.Marshal(result)
	status := "completed"
	if err != nil {
		status = "failed"
	}
	_, _ = s.store.CreateConnectorActionLog(r.Context(), ConnectorActionLog{
		InstallationID: installation.ID,
		ActionName:     string(req.Action),
		Status:         status,
		ResponseJSON:   responseJSON,
		ErrorMessage:   errorString(err),
		StartedAt:      time.Now().UTC(),
	})
	s.recordAudit(r, token, "connector_action", "installation", installation.ID, installation.OrgID, installation.WorkspaceID, installation.ProjectID, outcomeFromErr(err), map[string]any{"action": req.Action, "provider": connector.Provider()})
	writeJSON(w, httpStatusFromErr(err, http.StatusOK), map[string]any{
		"result": result,
		"error":  errorString(err),
	})
}

func (s *Server) handleInstallationWebhook(w http.ResponseWriter, r *http.Request, installationID string) {
	installation, connector, cfg, ok := s.resolveInstallationConnector(w, r, installationID)
	if !ok {
		return
	}
	if !installation.Enabled {
		writeAPIError(w, http.StatusConflict, "installation_disabled", "connector installation is disabled")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	verification, err := connector.VerifyWebhook(r.Header, body, cfg)
	if err != nil {
		installation.Status = "error"
		installation.LastError = err.Error()
		_, _ = s.store.CreateConnectorInstallation(r.Context(), *installation)
		writeAPIError(w, http.StatusUnauthorized, "verification_failed", err.Error())
		return
	}
	events, err := connector.NormalizeEvent(r.Header, body, cfg)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "normalize_failed", err.Error())
		return
	}
	for _, item := range events {
		payload, _ := json.Marshal(item)
		sourceEventID := firstNonEmpty(item.ExternalID, item.SubjectID, verification.DeliveryID)
		_, _ = s.store.CreateConnectorEvent(r.Context(), ConnectorEvent{
			InstallationID: installation.ID,
			SourceEventID:  sourceEventID,
			EventType:      item.Kind,
			Action:         item.Action,
			Status:         "received",
			PayloadJSON:    payload,
			OccurredAt:     nonZeroTime(item.OccurredAt, time.Now().UTC()),
			ReceivedAt:     time.Now().UTC(),
		})
	}
	now := time.Now().UTC()
	installation.Status = "active"
	installation.LastSeenAt = &now
	installation.LastError = ""
	_, _ = s.store.CreateConnectorInstallation(r.Context(), *installation)
	_, _ = s.store.CreateCloudAuditEvent(r.Context(), CloudAuditEvent{
		ActorType:    "connector_webhook",
		Action:       "webhook_ingest",
		ResourceType: "installation",
		ResourceID:   installation.ID,
		OrgID:        installation.OrgID,
		WorkspaceID:  installation.WorkspaceID,
		ProjectID:    installation.ProjectID,
		Outcome:      "ok",
		DetailsJSON:  mustJSON(map[string]any{"provider": connector.Provider(), "event_type": verification.EventType, "delivery_id": verification.DeliveryID, "count": len(events)}),
		CreatedAt:    time.Now().UTC(),
	})
	writeJSON(w, http.StatusAccepted, map[string]any{
		"verification": verification,
		"events":       events,
	})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	token, ok := s.requireToken(w, r, "events:read")
	if !ok {
		return
	}
	installationID := strings.TrimSpace(r.URL.Query().Get("installation_id"))
	limit := parseIntDefault(r.URL.Query().Get("limit"), 100)
	if installationID != "" {
		installation, err := s.store.GetConnectorInstallation(r.Context(), installationID)
		if err != nil {
			writeAPIError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		if !TokenAllowsTarget(token, installation.OrgID, installation.WorkspaceID, installation.ProjectID) {
			writeAPIError(w, http.StatusForbidden, "scope_denied", "token is not allowed to read this installation's events")
			return
		}
	}
	events, err := s.store.ListConnectorEvents(r.Context(), installationID, limit)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	token, ok := s.requireToken(w, r, "audit:read")
	if !ok {
		return
	}
	limit := parseIntDefault(r.URL.Query().Get("limit"), 100)
	events, err := s.store.ListCloudAuditEvents(r.Context(), limit)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	if token.OrgID != "" {
		filtered := make([]CloudAuditEvent, 0, len(events))
		for _, event := range events {
			if event.OrgID == "" || event.OrgID == token.OrgID {
				filtered = append(filtered, event)
			}
		}
		events = filtered
	}
	writeJSON(w, http.StatusOK, events)
}

func (s *Server) requireToken(w http.ResponseWriter, r *http.Request, scope string) (*APIToken, bool) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		writeAPIError(w, http.StatusUnauthorized, "auth_required", "bearer token is required")
		return nil, false
	}
	raw := strings.TrimSpace(header[len("Bearer "):])
	token, err := s.store.LookupAPIToken(r.Context(), raw)
	if err != nil {
		status := http.StatusUnauthorized
		if errors.Is(err, ErrAPITokenInvalid) {
			status = http.StatusBadRequest
		}
		writeAPIError(w, status, "auth_failed", err.Error())
		return nil, false
	}
	if token.Disabled || token.RevokedAt != nil {
		writeAPIError(w, http.StatusUnauthorized, "auth_failed", "api token is disabled or revoked")
		return nil, false
	}
	if !TokenHasScope(token, scope) {
		writeAPIError(w, http.StatusForbidden, "scope_denied", "api token lacks required scope")
		return nil, false
	}
	_ = s.store.TouchAPITokenUsage(r.Context(), token.ID)
	return token, true
}

func (s *Server) resolveInstallationConnector(w http.ResponseWriter, r *http.Request, installationID string) (*ConnectorInstallation, cloudconnectors.Connector, cloudconnectors.InstallationConfig, bool) {
	if s.connectors == nil {
		writeAPIError(w, http.StatusInternalServerError, "connector_registry_missing", "cloud connector registry is not configured")
		return nil, nil, cloudconnectors.InstallationConfig{}, false
	}
	installation, err := s.store.GetConnectorInstallation(r.Context(), installationID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, "not_found", err.Error())
		return nil, nil, cloudconnectors.InstallationConfig{}, false
	}
	connector, ok := s.connectors.Get(cloudconnectors.Provider(installation.ConnectorKey))
	if !ok {
		writeAPIError(w, http.StatusBadRequest, "unsupported_connector", "no registered connector matches the installation")
		return nil, nil, cloudconnectors.InstallationConfig{}, false
	}
	cfg, err := s.installationConfig(r, installation)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "installation_config_error", err.Error())
		return nil, nil, cloudconnectors.InstallationConfig{}, false
	}
	return installation, connector, cfg, true
}

func (s *Server) installationConfig(r *http.Request, installation *ConnectorInstallation) (cloudconnectors.InstallationConfig, error) {
	cfg := cloudconnectors.InstallationConfig{}
	if installation == nil {
		return cfg, fmt.Errorf("installation is required")
	}
	if len(installation.ConfigJSON) > 0 {
		var raw map[string]string
		if err := json.Unmarshal(installation.ConfigJSON, &raw); err != nil {
			return cfg, fmt.Errorf("decode installation config: %w", err)
		}
		cfg.APIBaseURL = raw["api_base_url"]
		cfg.Username = raw["username"]
	}
	token, err := s.store.GetInstallationSecret(r.Context(), installation.ID, "token")
	if err == nil {
		cfg.Token = string(token)
	} else if !strings.Contains(err.Error(), "not found") && !errors.Is(err, ErrSecretBoxRequired) {
		return cfg, err
	}
	webhook, err := s.store.GetInstallationSecret(r.Context(), installation.ID, "webhook_secret")
	if err == nil {
		cfg.WebhookSecret = string(webhook)
	} else if !strings.Contains(err.Error(), "not found") && !errors.Is(err, ErrSecretBoxRequired) {
		return cfg, err
	}
	return cfg, nil
}

func (s *Server) recordAudit(r *http.Request, token *APIToken, action, resourceType, resourceID, orgID, workspaceID, projectID, outcome string, details map[string]any) {
	if s.store == nil {
		return
	}
	_, _ = s.store.CreateCloudAuditEvent(r.Context(), CloudAuditEvent{
		ActorType:    "api_token",
		ActorID:      token.ID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		OrgID:        firstNonEmpty(orgID, token.OrgID),
		WorkspaceID:  firstNonEmpty(workspaceID, token.WorkspaceID),
		ProjectID:    firstNonEmpty(projectID, token.ProjectID),
		Outcome:      outcome,
		DetailsJSON:  mustJSON(details),
		CreatedAt:    time.Now().UTC(),
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}

func decodeJSON(body io.Reader, out any) error {
	dec := json.NewDecoder(io.LimitReader(body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(out)
}

func parseIntDefault(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func mustJSON(value any) json.RawMessage {
	if value == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return raw
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func outcomeFromErr(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}

func httpStatusFromErr(err error, okStatus int) int {
	if err == nil {
		return okStatus
	}
	return http.StatusBadGateway
}

func nonZeroTime(value time.Time, fallback time.Time) time.Time {
	if value.IsZero() {
		return fallback
	}
	return value
}

func (o Org) OrgID() string {
	return o.ID
}
