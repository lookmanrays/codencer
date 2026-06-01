package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	appversion "agent-bridge/internal/app"
	"agent-bridge/internal/domain"
	"agent-bridge/internal/relayproto"
	"agent-bridge/internal/security"
)

type plannerInstance struct {
	InstanceID  string          `json:"instance_id"`
	ConnectorID string          `json:"connector_id"`
	RepoRoot    string          `json:"repo_root"`
	BaseURL     string          `json:"base_url"`
	Online      bool            `json:"online"`
	Status      string          `json:"status"`
	LastSeenAt  time.Time       `json:"last_seen_at"`
	Instance    json.RawMessage `json:"instance,omitempty"`
}

type plannerProject struct {
	ProjectID      string          `json:"project_id"`
	ConnectorID    string          `json:"connector_id"`
	InstanceID     string          `json:"instance_id"`
	RepoLabel      string          `json:"repo_label,omitempty"`
	RepoRootHash   string          `json:"repo_root_hash,omitempty"`
	DefaultAdapter string          `json:"default_adapter,omitempty"`
	AdapterProfile string          `json:"adapter_profile,omitempty"`
	Online         bool            `json:"online"`
	Status         string          `json:"status"`
	LastSeenAt     time.Time       `json:"last_seen_at"`
	Project        json.RawMessage `json:"project,omitempty"`
}

type relayStatusResponse struct {
	Version                   string    `json:"version"`
	StartedAt                 time.Time `json:"started_at"`
	PlannerAuthMode           string    `json:"planner_auth_mode"`
	MCPProtectedResource      bool      `json:"mcp_protected_resource_metadata"`
	BootstrapEnrollmentSecret bool      `json:"bootstrap_enrollment_secret_enabled"`
	ConnectorCount            int       `json:"connector_count"`
	OnlineConnectorCount      int       `json:"online_connector_count"`
	InstanceCount             int       `json:"instance_count"`
	OnlineInstanceCount       int       `json:"online_instance_count"`
	ProjectCount              int       `json:"project_count"`
	OnlineProjectCount        int       `json:"online_project_count"`
}

type plannerConnector struct {
	ConnectorID       string    `json:"connector_id"`
	MachineID         string    `json:"machine_id"`
	Label             string    `json:"label,omitempty"`
	Disabled          bool      `json:"disabled"`
	Online            bool      `json:"online"`
	Status            string    `json:"status"`
	LastSeenAt        time.Time `json:"last_seen_at,omitempty"`
	SharedInstanceIDs []string  `json:"shared_instance_ids,omitempty"`
	SharedProjectIDs  []string  `json:"shared_project_ids,omitempty"`
}

func (s *Server) handleInstances(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		records, err := s.store.ListInstances(r.Context())
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "relay_internal_error", err.Error())
			return
		}
		response := make([]plannerInstance, 0, len(records))
		for _, record := range records {
			status, err := s.instanceStatus(r.Context(), &record)
			if err != nil {
				writeAPIError(w, http.StatusInternalServerError, "relay_internal_error", err.Error())
				return
			}
			response = append(response, plannerInstance{
				InstanceID:  record.InstanceID,
				ConnectorID: record.ConnectorID,
				RepoRoot:    record.RepoRoot,
				BaseURL:     record.BaseURL,
				Online:      status.Online,
				Status:      status.Status,
				LastSeenAt:  record.LastSeenAt,
				Instance:    json.RawMessage(record.InstanceJSON),
			})
		}
		writeJSON(w, http.StatusOK, response)
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	records, err := s.store.ListProjects(r.Context())
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "relay_internal_error", err.Error())
		return
	}
	principal := plannerFromContext(r.Context())
	response := make([]plannerProject, 0, len(records))
	for _, record := range records {
		if !projectAllowed(principal, "projects:read", record) {
			continue
		}
		payload, err := s.projectPayload(r.Context(), &record)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "relay_internal_error", err.Error())
			return
		}
		response = append(response, payload)
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleProjectScoped(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v2/projects/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeAPIError(w, http.StatusBadRequest, "malformed_request", "project_id is required")
		return
	}
	projectID := parts[0]
	principal := plannerFromContext(r.Context())
	record, apiErr := s.resolveProjectRecord(r.Context(), principal, projectID, scopeForProjectRoute(parts, r.Method))
	if apiErr != nil {
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}

	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		payload, err := s.projectPayload(r.Context(), record)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "relay_internal_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, payload)
	case len(parts) == 2 && parts[1] == "runs" && r.Method == http.MethodGet:
		s.projectProxyAndWrite(w, r, record, http.MethodGet, fmt.Sprintf("/codencer/v1/projects/%s/runs", projectID), "", nil, "list_project_runs", "runs:read")
	case len(parts) == 2 && parts[1] == "runs" && r.Method == http.MethodPost:
		body, _ := io.ReadAll(r.Body)
		s.projectProxyAndWrite(w, r, record, http.MethodPost, fmt.Sprintf("/codencer/v1/projects/%s/runs", projectID), "", body, "start_project_run", "runs:write")
	case len(parts) == 3 && parts[1] == "runs" && r.Method == http.MethodGet:
		s.projectProxyAndWrite(w, r, record, http.MethodGet, fmt.Sprintf("/codencer/v1/projects/%s/runs/%s", projectID, parts[2]), "", nil, "get_project_run", "runs:read")
	case len(parts) == 2 && parts[1] == "submit" && r.Method == http.MethodPost:
		body, _ := io.ReadAll(r.Body)
		s.projectProxyAndWrite(w, r, record, http.MethodPost, fmt.Sprintf("/codencer/v1/projects/%s/submit", projectID), "", body, "submit_project_task", "steps:write")
	case len(parts) == 2 && parts[1] == "run-plan" && r.Method == http.MethodPost:
		body, _ := io.ReadAll(r.Body)
		s.projectProxyAndWrite(w, r, record, http.MethodPost, fmt.Sprintf("/codencer/v1/projects/%s/run-plan", projectID), "", body, "run_project_manifest", "runs:write")
	case len(parts) == 4 && parts[1] == "reports" && parts[2] == "run-plans" && r.Method == http.MethodGet:
		s.projectProxyAndWrite(w, r, record, http.MethodGet, fmt.Sprintf("/codencer/v1/projects/%s/reports/run-plans/%s", projectID, parts[3]), "", nil, "get_execution_report", "reports:read")
	case len(parts) == 4 && parts[1] == "steps" && r.Method == http.MethodGet:
		s.projectProxyAndWrite(w, r, record, http.MethodGet, fmt.Sprintf("/codencer/v1/projects/%s/steps/%s/%s", projectID, parts[2], parts[3]), "", nil, "get_project_step_"+parts[3], scopeForProjectEvidence(parts[3]))
	default:
		writeAPIError(w, http.StatusNotFound, "malformed_request", "unsupported project route")
	}
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	connectors, err := s.store.ListConnectors(r.Context())
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "relay_internal_error", err.Error())
		return
	}
	instances, err := s.store.ListInstances(r.Context())
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "relay_internal_error", err.Error())
		return
	}
	projects, err := s.store.ListProjects(r.Context())
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "relay_internal_error", err.Error())
		return
	}
	onlineConnectors := 0
	for _, connector := range connectors {
		if status := s.connectorStatus(&connector); status.Online {
			onlineConnectors++
		}
	}
	onlineInstances := 0
	for _, instance := range instances {
		status, err := s.instanceStatus(r.Context(), &instance)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "relay_internal_error", err.Error())
			return
		}
		if status.Online {
			onlineInstances++
		}
	}
	onlineProjects := 0
	for _, project := range projects {
		status, err := s.projectStatus(r.Context(), &project)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "relay_internal_error", err.Error())
			return
		}
		if status.Online {
			onlineProjects++
		}
	}
	writeJSON(w, http.StatusOK, relayStatusResponse{
		Version:                   appversion.Version,
		StartedAt:                 s.startedAt,
		PlannerAuthMode:           s.plannerAuthMode(),
		MCPProtectedResource:      true,
		BootstrapEnrollmentSecret: s.cfg.EnrollmentSecret != "",
		ConnectorCount:            len(connectors),
		OnlineConnectorCount:      onlineConnectors,
		InstanceCount:             len(instances),
		OnlineInstanceCount:       onlineInstances,
		ProjectCount:              len(projects),
		OnlineProjectCount:        onlineProjects,
	})
}

func (s *Server) handleConnectors(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	connectors, err := s.store.ListConnectors(r.Context())
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "relay_internal_error", err.Error())
		return
	}
	response := make([]plannerConnector, 0, len(connectors))
	for _, connector := range connectors {
		payload, err := s.connectorPayload(r.Context(), connector)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "relay_internal_error", err.Error())
			return
		}
		response = append(response, payload)
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleConnectorScoped(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v2/connectors/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeAPIError(w, http.StatusBadRequest, "malformed_request", "connector_id is required")
		return
	}

	connectorID := parts[0]
	principal := plannerFromContext(r.Context())
	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		if apiErr := authorizePrincipal(principal, "admin:read", ""); apiErr != nil {
			writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
			return
		}
		record, err := s.store.GetConnector(r.Context(), connectorID)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "relay_internal_error", err.Error())
			return
		}
		if record == nil {
			writeAPIError(w, http.StatusNotFound, "connector_not_found", "connector not found")
			return
		}
		payload, err := s.connectorPayload(r.Context(), *record)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "relay_internal_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, payload)
		return
	case len(parts) == 2 && r.Method == http.MethodPost && (parts[1] == "disable" || parts[1] == "enable"):
		if apiErr := authorizePrincipal(principal, "admin:write", ""); apiErr != nil {
			writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
			return
		}
		record, err := s.store.GetConnector(r.Context(), connectorID)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "relay_internal_error", err.Error())
			return
		}
		if record == nil {
			writeAPIError(w, http.StatusNotFound, "connector_not_found", "connector not found")
			return
		}

		disabled := parts[1] == "disable"
		if err := s.store.SetConnectorDisabled(r.Context(), connectorID, disabled); err != nil {
			writeAPIError(w, http.StatusInternalServerError, "relay_internal_error", err.Error())
			return
		}
		record.Disabled = disabled
		payload, err := s.connectorPayload(r.Context(), *record)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "relay_internal_error", err.Error())
			return
		}
		action := "enable_connector"
		if disabled {
			action = "disable_connector"
		}
		actorID := ""
		if principal != nil {
			actorID = principal.Name
		}
		s.auditor.Record(r.Context(), AuditEvent{
			ActorType:         "planner",
			ActorID:           actorID,
			Action:            action,
			ResourceKind:      "connector",
			ResourceID:        connectorID,
			TargetConnectorID: connectorID,
			Outcome:           "ok",
		})
		writeJSON(w, http.StatusOK, payload)
		return
	default:
		writeAPIError(w, http.StatusNotFound, "malformed_request", "unsupported connector route")
		return
	}
}

func (s *Server) connectorPayload(ctx context.Context, connector ConnectorRecord) (plannerConnector, error) {
	instances, err := s.store.ListInstancesByConnector(ctx, connector.ConnectorID)
	if err != nil {
		return plannerConnector{}, err
	}
	projects, err := s.store.ListProjectsByConnector(ctx, connector.ConnectorID)
	if err != nil {
		return plannerConnector{}, err
	}
	sharedIDs := make([]string, 0, len(instances))
	for _, instance := range instances {
		sharedIDs = append(sharedIDs, instance.InstanceID)
	}
	sharedProjectIDs := make([]string, 0, len(projects))
	for _, project := range projects {
		sharedProjectIDs = append(sharedProjectIDs, project.ProjectID)
	}
	status := s.connectorStatus(&connector)
	return plannerConnector{
		ConnectorID:       connector.ConnectorID,
		MachineID:         connector.MachineID,
		Label:             connector.Label,
		Disabled:          connector.Disabled,
		Online:            status.Online,
		Status:            status.Status,
		LastSeenAt:        connector.LastSeenAt,
		SharedInstanceIDs: sharedIDs,
		SharedProjectIDs:  sharedProjectIDs,
	}, nil
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	limit := parseAuditLimit(r.URL.Query().Get("limit"))
	events, err := s.store.ListAuditEventsLimit(r.Context(), limit)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "relay_internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func (s *Server) handleEnrollmentTokens(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	var req struct {
		Label            string `json:"label"`
		ExpiresInSeconds int    `json:"expires_in_seconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "malformed_request", err.Error())
		return
	}
	var expiresIn time.Duration
	if req.ExpiresInSeconds > 0 {
		expiresIn = time.Duration(req.ExpiresInSeconds) * time.Second
	}
	actor := plannerFromContext(r.Context())
	createdBy := "planner"
	if actor != nil && actor.Name != "" {
		createdBy = actor.Name
	}
	record, secret, err := s.enrollment.CreateToken(r.Context(), createdBy, req.Label, expiresIn)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "relay_internal_error", err.Error())
		return
	}
	s.auditor.Record(r.Context(), AuditEvent{
		ActorType: "planner",
		ActorID:   createdBy,
		Action:    "create_enrollment_token",
		Outcome:   "ok",
	})
	writeJSON(w, http.StatusCreated, map[string]any{
		"token_id":   record.TokenID,
		"secret":     secret,
		"label":      record.Label,
		"created_at": record.CreatedAt,
		"expires_at": record.ExpiresAt,
	})
}

func (s *Server) handleInstanceScoped(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v2/instances/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeAPIError(w, http.StatusBadRequest, "malformed_request", "instance_id is required")
		return
	}
	instanceID := parts[0]
	if len(parts) == 1 && r.Method == http.MethodGet {
		record, err := s.store.GetInstance(r.Context(), instanceID)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "relay_internal_error", err.Error())
			return
		}
		if record == nil {
			writeAPIError(w, http.StatusNotFound, "instance_not_found", "instance not found")
			return
		}
		status, err := s.instanceStatus(r.Context(), record)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "relay_internal_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, plannerInstance{
			InstanceID:  record.InstanceID,
			ConnectorID: record.ConnectorID,
			RepoRoot:    record.RepoRoot,
			BaseURL:     record.BaseURL,
			Online:      status.Online,
			Status:      status.Status,
			LastSeenAt:  record.LastSeenAt,
			Instance:    json.RawMessage(record.InstanceJSON),
		})
		return
	}
	if len(parts) < 2 {
		writeAPIError(w, http.StatusBadRequest, "malformed_request", "instance resource path is required")
		return
	}
	resource := parts[1]
	switch {
	case resource == "runs" && len(parts) == 2 && r.Method == http.MethodGet:
		s.proxyAndWrite(w, r, instanceID, http.MethodGet, "/api/v1/runs", r.URL.RawQuery, nil, "", "", "list_runs", "runs:read")
	case resource == "runs" && len(parts) == 2 && r.Method == http.MethodPost:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "malformed_request", err.Error())
			return
		}
		s.proxyAndWrite(w, r, instanceID, http.MethodPost, "/api/v1/runs", "", body, "run", "id", "start_run", "runs:write")
	case resource == "runs" && len(parts) == 3 && r.Method == http.MethodGet:
		s.proxyAndWrite(w, r, instanceID, http.MethodGet, fmt.Sprintf("/api/v1/runs/%s", parts[2]), "", nil, "", "", "get_run", "runs:read")
	case resource == "runs" && len(parts) == 4 && parts[3] == "steps" && r.Method == http.MethodPost:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "malformed_request", err.Error())
			return
		}
		s.proxyAndWrite(w, r, instanceID, http.MethodPost, fmt.Sprintf("/api/v1/runs/%s/steps", parts[2]), "", body, "step", "id", "submit_task", "steps:write")
	case resource == "runs" && len(parts) == 4 && parts[3] == "gates" && r.Method == http.MethodGet:
		s.proxyAndWrite(w, r, instanceID, http.MethodGet, fmt.Sprintf("/api/v1/runs/%s/gates", parts[2]), "", nil, "gate", "id", "list_run_gates", "gates:read")
	case resource == "runs" && len(parts) == 4 && parts[3] == "abort" && r.Method == http.MethodPost:
		body, _ := json.Marshal(map[string]string{"action": "abort"})
		s.proxyAndWrite(w, r, instanceID, http.MethodPatch, fmt.Sprintf("/api/v1/runs/%s", parts[2]), "", body, "", "", "abort_run", "runs:write")
	default:
		writeAPIError(w, http.StatusNotFound, "malformed_request", "unsupported instance route")
	}
}

func (s *Server) handleStepScoped(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v2/steps/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeAPIError(w, http.StatusBadRequest, "malformed_request", "step_id is required")
		return
	}
	targetPath := fmt.Sprintf("/api/v1/steps/%s", parts[0])
	action := "get_step"
	scope := "steps:read"
	routeKind := ""
	routeIDField := ""
	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
	case len(parts) == 2 && parts[1] == "result" && r.Method == http.MethodGet:
		targetPath += "/result"
		action = "get_step_result"
	case len(parts) == 2 && parts[1] == "artifacts" && r.Method == http.MethodGet:
		targetPath += "/artifacts"
		action = "list_step_artifacts"
		scope = "artifacts:read"
		routeKind = "artifact"
		routeIDField = "id"
	case len(parts) == 2 && parts[1] == "logs" && r.Method == http.MethodGet:
		targetPath += "/logs"
		action = "get_step_logs"
	case len(parts) == 2 && parts[1] == "validations" && r.Method == http.MethodGet:
		targetPath += "/validations"
		action = "get_step_validations"
	case len(parts) == 2 && parts[1] == "wait" && r.Method == http.MethodPost:
		targetPath += "/wait"
		action = "wait_step"
	case len(parts) == 2 && parts[1] == "retry" && r.Method == http.MethodPost:
		targetPath += "/retry"
		action = "retry_step"
		scope = "steps:write"
	default:
		writeAPIError(w, http.StatusNotFound, "malformed_request", "unsupported step route")
		return
	}
	var body []byte
	if r.Body != nil && (r.Method == http.MethodPost || r.Method == http.MethodPatch) {
		body, _ = io.ReadAll(r.Body)
	}
	instanceID, apiErr := s.resolveResourceRoute(r.Context(), plannerFromContext(r.Context()), "step", parts[0], scope, "")
	if apiErr != nil {
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	s.proxyAndWrite(w, r, instanceID, r.Method, targetPath, "", body, routeKind, routeIDField, action, scope)
}

func (s *Server) handleArtifactScoped(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v2/artifacts/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 || parts[1] != "content" || r.Method != http.MethodGet {
		writeAPIError(w, http.StatusBadRequest, "malformed_request", "artifact_id/content path required")
		return
	}
	instanceID, apiErr := s.resolveResourceRoute(r.Context(), plannerFromContext(r.Context()), "artifact", parts[0], "artifacts:read", "")
	if apiErr != nil {
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	s.proxyAndWrite(w, r, instanceID, http.MethodGet, fmt.Sprintf("/api/v1/artifacts/%s/content", parts[0]), "", nil, "", "", "get_artifact_content", "artifacts:read")
}

func (s *Server) handleGateScoped(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v2/gates/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 || r.Method != http.MethodPost {
		writeAPIError(w, http.StatusBadRequest, "malformed_request", "gate_id/action path required")
		return
	}
	var action string
	switch parts[1] {
	case "approve":
		action = "approve"
	case "reject":
		action = "reject"
	default:
		writeAPIError(w, http.StatusBadRequest, "malformed_request", "unsupported gate action")
		return
	}
	instanceID, apiErr := s.resolveResourceRoute(r.Context(), plannerFromContext(r.Context()), "gate", parts[0], "gates:write", "")
	if apiErr != nil {
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	body, _ := json.Marshal(map[string]string{"action": action})
	s.proxyAndWrite(w, r, instanceID, http.MethodPost, fmt.Sprintf("/api/v1/gates/%s", parts[0]), "", body, "", "", "gate_"+action, "gates:write")
}

func (s *Server) proxyAndWrite(w http.ResponseWriter, r *http.Request, instanceID, method, path, query string, body []byte, routeKind, routeIDField, action, scope string) {
	principal := plannerFromContext(r.Context())
	actorID := ""
	if principal != nil {
		actorID = principal.Name
	}
	if err := authorizePrincipal(principal, scope, instanceID); err != nil {
		s.auditor.Record(r.Context(), AuditEvent{
			ActorType:        "planner",
			ActorID:          actorID,
			Action:           action,
			Method:           method,
			Scope:            scope,
			TargetInstanceID: instanceID,
			Outcome:          "error",
			ErrorCode:        err.Code,
		})
		writeAPIError(w, err.Status, err.Code, err.Message)
		return
	}
	response, resourceRecord, apiErr := s.proxyRequest(r.Context(), instanceID, method, path, query, body, scope)
	if apiErr != nil {
		s.auditor.Record(r.Context(), AuditEvent{
			ActorType:        "planner",
			ActorID:          actorID,
			Action:           action,
			Method:           method,
			Scope:            scope,
			TargetInstanceID: instanceID,
			Outcome:          "error",
			ErrorCode:        apiErr.Code,
		})
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	if response.ContentType != "" {
		w.Header().Set("Content-Type", response.ContentType)
	}
	if len(response.Body) > 0 && response.ContentEncoding == "" {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(response.Body)))
	}
	w.WriteHeader(response.StatusCode)
	payload := decodeRelayBody(response)
	if len(payload) > 0 {
		_, _ = w.Write(payload)
	}
	s.captureRoutes(r.Context(), instanceID, routeKind, routeIDField, payload)
	s.auditor.Record(r.Context(), AuditEvent{
		ActorType:         "planner",
		ActorID:           actorID,
		Action:            action,
		Method:            method,
		Scope:             scope,
		TargetConnectorID: resourceRecord.ConnectorID,
		TargetInstanceID:  instanceID,
		Outcome:           "ok",
	})
}

func (s *Server) projectProxyAndWrite(w http.ResponseWriter, r *http.Request, project *ProjectRecord, method, path, query string, body []byte, action, scope string) {
	principal := plannerFromContext(r.Context())
	actorID := ""
	if principal != nil {
		actorID = principal.Name
	}
	if err := authorizeProject(principal, scope, project); err != nil {
		s.auditor.Record(r.Context(), AuditEvent{
			ActorType:         "planner",
			ActorID:           actorID,
			Action:            action,
			Method:            method,
			Scope:             scope,
			ResourceKind:      "project",
			ResourceID:        project.ProjectID,
			TargetConnectorID: project.ConnectorID,
			TargetInstanceID:  project.InstanceID,
			Outcome:           "error",
			ErrorCode:         err.Code,
		})
		writeAPIError(w, err.Status, err.Code, err.Message)
		return
	}
	response, apiErr := s.proxyProjectRequest(r.Context(), project, method, path, query, body)
	if apiErr != nil {
		s.auditor.Record(r.Context(), AuditEvent{
			ActorType:         "planner",
			ActorID:           actorID,
			Action:            action,
			Method:            method,
			Scope:             scope,
			ResourceKind:      "project",
			ResourceID:        project.ProjectID,
			TargetConnectorID: project.ConnectorID,
			TargetInstanceID:  project.InstanceID,
			Outcome:           "error",
			ErrorCode:         apiErr.Code,
		})
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	if response.ContentType != "" {
		w.Header().Set("Content-Type", response.ContentType)
	}
	payload := decodeRelayBody(response)
	payload = security.SanitizeRemoteJSON(payload)
	if len(payload) > 0 && response.ContentEncoding == "" {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(payload)))
	}
	w.WriteHeader(response.StatusCode)
	if len(payload) > 0 {
		_, _ = w.Write(payload)
	}
	s.auditor.Record(r.Context(), AuditEvent{
		ActorType:         "planner",
		ActorID:           actorID,
		Action:            action,
		Method:            method,
		Scope:             scope,
		ResourceKind:      "project",
		ResourceID:        project.ProjectID,
		TargetConnectorID: project.ConnectorID,
		TargetInstanceID:  project.InstanceID,
		Outcome:           "ok",
	})
}

func (s *Server) proxyProjectRequest(ctx context.Context, project *ProjectRecord, method, path, query string, body []byte) (*relayproto.CommandResponse, *apiError) {
	if project == nil {
		return nil, &apiError{Status: http.StatusNotFound, Code: "project_not_found", Message: "project not found"}
	}
	connectorRecord, err := s.store.GetConnector(ctx, project.ConnectorID)
	if err != nil {
		return nil, &apiError{Status: http.StatusInternalServerError, Code: "relay_internal_error", Message: err.Error()}
	}
	if connectorRecord != nil && connectorRecord.Disabled {
		return nil, &apiError{Status: http.StatusForbidden, Code: "connector_disabled", Message: "connector for this project is disabled"}
	}
	session := s.hub.Connector(project.ConnectorID)
	if session == nil {
		return nil, &apiError{Status: http.StatusServiceUnavailable, Code: "connector_offline", Message: "connector for this project is offline"}
	}
	requestID, err := randomID("req")
	if err != nil {
		return nil, &apiError{Status: http.StatusInternalServerError, Code: "relay_internal_error", Message: err.Error()}
	}
	timeout := s.proxyTimeout(path, body)
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	response, err := session.proxy(requestCtx, relayproto.CommandRequest{
		Type:        "request",
		RequestID:   requestID,
		InstanceID:  project.InstanceID,
		Method:      method,
		Path:        path,
		Query:       query,
		Body:        body,
		ContentType: contentTypeForBody(body),
		TimeoutMs:   int(timeout / time.Millisecond),
	})
	if err != nil {
		if strings.Contains(err.Error(), "timed out") || strings.Contains(err.Error(), "deadline exceeded") {
			return nil, &apiError{Status: http.StatusGatewayTimeout, Code: "upstream_timeout", Message: err.Error()}
		}
		return nil, &apiError{Status: http.StatusBadGateway, Code: "connector_offline", Message: err.Error()}
	}
	if response.StatusCode >= 400 {
		return nil, &apiError{
			Status:  translateUpstreamStatus(response.StatusCode),
			Code:    upstreamErrorCode(response.StatusCode, response.Error),
			Message: strings.TrimSpace(response.Error),
		}
	}
	return response, nil
}

func (s *Server) proxyRequest(ctx context.Context, instanceID, method, path, query string, body []byte, _ string) (*relayproto.CommandResponse, *InstanceRecord, *apiError) {
	record, err := s.store.GetInstance(ctx, instanceID)
	if err != nil {
		return nil, nil, &apiError{Status: http.StatusInternalServerError, Code: "relay_internal_error", Message: err.Error()}
	}
	if record == nil {
		return nil, nil, &apiError{Status: http.StatusNotFound, Code: "instance_not_found", Message: "instance not found"}
	}
	connectorRecord, err := s.store.GetConnector(ctx, record.ConnectorID)
	if err != nil {
		return nil, record, &apiError{Status: http.StatusInternalServerError, Code: "relay_internal_error", Message: err.Error()}
	}
	if connectorRecord != nil && connectorRecord.Disabled {
		return nil, record, &apiError{Status: http.StatusForbidden, Code: "connector_disabled", Message: "connector for this instance is disabled"}
	}
	session := s.hub.Get(instanceID)
	if session == nil {
		return nil, record, &apiError{Status: http.StatusServiceUnavailable, Code: "connector_offline", Message: "connector for this instance is offline"}
	}
	requestID, err := randomID("req")
	if err != nil {
		return nil, record, &apiError{Status: http.StatusInternalServerError, Code: "relay_internal_error", Message: err.Error()}
	}
	timeout := s.proxyTimeout(path, body)
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	response, err := session.proxy(requestCtx, relayproto.CommandRequest{
		Type:        "request",
		RequestID:   requestID,
		InstanceID:  instanceID,
		Method:      method,
		Path:        path,
		Query:       query,
		Body:        body,
		ContentType: contentTypeForBody(body),
		TimeoutMs:   int(timeout / time.Millisecond),
	})
	if err != nil {
		if strings.Contains(err.Error(), "timed out") || strings.Contains(err.Error(), "deadline exceeded") {
			return nil, record, &apiError{Status: http.StatusGatewayTimeout, Code: "upstream_timeout", Message: err.Error()}
		}
		return nil, record, &apiError{Status: http.StatusBadGateway, Code: "connector_offline", Message: err.Error()}
	}
	if response.StatusCode >= 400 {
		return nil, record, &apiError{
			Status:  translateUpstreamStatus(response.StatusCode),
			Code:    upstreamErrorCode(response.StatusCode, response.Error),
			Message: strings.TrimSpace(response.Error),
		}
	}
	return response, record, nil
}

func (s *Server) proxyTimeout(path string, body []byte) time.Duration {
	const transportGrace = 2 * time.Second

	timeout := time.Duration(s.cfg.ProxyTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 300 * time.Second
	}
	if !strings.HasSuffix(path, "/wait") || len(body) == 0 {
		return timeout
	}

	var request struct {
		TimeoutMS int `json:"timeout_ms"`
	}
	if err := json.Unmarshal(body, &request); err != nil || request.TimeoutMS <= 0 {
		return timeout
	}

	requested := time.Duration(request.TimeoutMS)*time.Millisecond + transportGrace
	if requested > timeout {
		return timeout
	}
	return requested
}

func (s *Server) captureRoutes(ctx context.Context, instanceID, kind, idField string, body []byte) {
	if kind == "" || idField == "" || len(body) == 0 {
		return
	}
	var single map[string]any
	if err := json.Unmarshal(body, &single); err == nil && single[idField] != nil {
		if id, ok := single[idField].(string); ok {
			_ = s.store.SaveResourceRoute(ctx, kind, id, instanceID)
		}
		return
	}
	var many []map[string]any
	if err := json.Unmarshal(body, &many); err == nil {
		for _, item := range many {
			if id, ok := item[idField].(string); ok {
				_ = s.store.SaveResourceRoute(ctx, kind, id, instanceID)
			}
		}
	}
}

func decodeRelayBody(response *relayproto.CommandResponse) []byte {
	switch response.ContentEncoding {
	case "", "json":
		return response.Body
	case "utf-8":
		var text string
		if err := json.Unmarshal(response.Body, &text); err == nil {
			return []byte(text)
		}
		return response.Body
	default:
		return response.Body
	}
}

func contentTypeForBody(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	return "application/json"
}

func upstreamErrorCode(status int, message string) string {
	switch status {
	case http.StatusForbidden:
		if strings.Contains(strings.ToLower(message), "instance") {
			return "instance_not_shared"
		}
		return "auth_failed"
	case http.StatusGatewayTimeout:
		return "upstream_timeout"
	case http.StatusRequestEntityTooLarge:
		return "artifact_too_large"
	case http.StatusBadRequest:
		return "malformed_request"
	case http.StatusNotFound:
		return "instance_not_found"
	default:
		return "upstream_error"
	}
}

func translateUpstreamStatus(status int) int {
	switch status {
	case 0:
		return http.StatusBadGateway
	default:
		return status
	}
}

func relayBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, r.Host)
}

func websocketURL(r *http.Request, path string) string {
	scheme := "ws"
	if r.TLS != nil {
		scheme = "wss"
	}
	return fmt.Sprintf("%s://%s%s", scheme, r.Host, path)
}

func parseInstanceInfo(raw string) (*domain.InstanceInfo, error) {
	if raw == "" {
		return nil, nil
	}
	var info domain.InstanceInfo
	if err := json.Unmarshal([]byte(raw), &info); err != nil {
		return nil, err
	}
	return &info, nil
}

func (s *Server) connectorStatus(record *ConnectorRecord) InstanceStatus {
	if record == nil {
		return InstanceStatus{Online: false, Status: "not_found"}
	}
	if record.Disabled {
		return InstanceStatus{Online: false, Status: "disabled"}
	}
	if s.hub.Connector(record.ConnectorID) != nil {
		return InstanceStatus{Online: true, Status: "online"}
	}
	return InstanceStatus{Online: false, Status: "offline"}
}

func (s *Server) instanceStatus(ctx context.Context, record *InstanceRecord) (InstanceStatus, error) {
	if record == nil {
		return InstanceStatus{Online: false, Status: "not_found"}, nil
	}
	connectorRecord, err := s.store.GetConnector(ctx, record.ConnectorID)
	if err != nil {
		return InstanceStatus{}, err
	}
	if connectorRecord != nil && connectorRecord.Disabled {
		return InstanceStatus{Online: false, Status: "disabled"}, nil
	}
	if s.hub.Get(record.InstanceID) != nil {
		return InstanceStatus{Online: true, Status: "online"}, nil
	}
	return InstanceStatus{Online: false, Status: "offline"}, nil
}

func (s *Server) projectStatus(ctx context.Context, record *ProjectRecord) (InstanceStatus, error) {
	if record == nil {
		return InstanceStatus{Online: false, Status: "not_found"}, nil
	}
	connectorRecord, err := s.store.GetConnector(ctx, record.ConnectorID)
	if err != nil {
		return InstanceStatus{}, err
	}
	if connectorRecord != nil && connectorRecord.Disabled {
		return InstanceStatus{Online: false, Status: "disabled"}, nil
	}
	if s.hub.Connector(record.ConnectorID) != nil {
		return InstanceStatus{Online: true, Status: "online"}, nil
	}
	return InstanceStatus{Online: false, Status: "offline"}, nil
}

func (s *Server) projectPayload(ctx context.Context, record *ProjectRecord) (plannerProject, error) {
	status, err := s.projectStatus(ctx, record)
	if err != nil {
		return plannerProject{}, err
	}
	repoLabel, repoHash := security.SafePathLabel(record.RepoRoot)
	return plannerProject{
		ProjectID:      record.ProjectID,
		ConnectorID:    record.ConnectorID,
		InstanceID:     record.InstanceID,
		RepoLabel:      repoLabel,
		RepoRootHash:   repoHash,
		DefaultAdapter: record.DefaultAdapter,
		AdapterProfile: record.AdapterProfile,
		Online:         status.Online,
		Status:         status.Status,
		LastSeenAt:     record.LastSeenAt,
		Project:        json.RawMessage(security.SanitizeRemoteJSON([]byte(record.ProjectJSON))),
	}, nil
}

func (s *Server) resolveProjectRecord(ctx context.Context, principal *plannerPrincipal, projectID, scope string) (*ProjectRecord, *apiError) {
	records, err := s.store.ListProjectsByID(ctx, projectID)
	if err != nil {
		return nil, &apiError{Status: http.StatusInternalServerError, Code: "relay_internal_error", Message: err.Error()}
	}
	candidates := make([]ProjectRecord, 0, len(records))
	for _, record := range records {
		if projectAllowed(principal, scope, record) {
			candidates = append(candidates, record)
		}
	}
	switch len(candidates) {
	case 0:
		if len(records) == 0 {
			return nil, &apiError{Status: http.StatusNotFound, Code: "project_not_found", Message: "project not found"}
		}
		return nil, &apiError{Status: http.StatusForbidden, Code: "project_denied", Message: "planner token is not authorized for this project"}
	case 1:
		return &candidates[0], nil
	default:
		connectors := make([]string, 0, len(candidates))
		for _, candidate := range candidates {
			connectors = append(connectors, candidate.ConnectorID)
		}
		sort.Strings(connectors)
		return nil, &apiError{Status: http.StatusConflict, Code: "project_ambiguous", Message: fmt.Sprintf("project %s is shared by multiple connectors: %s", projectID, strings.Join(connectors, ", "))}
	}
}

func scopeForProjectRoute(parts []string, method string) string {
	if len(parts) <= 1 {
		return "projects:read"
	}
	switch parts[1] {
	case "runs":
		if method == http.MethodPost {
			return "runs:write"
		}
		return "runs:read"
	case "submit":
		return "steps:write"
	case "run-plan":
		return "runs:write"
	case "reports":
		return "reports:read"
	case "steps":
		if len(parts) > 3 {
			return scopeForProjectEvidence(parts[3])
		}
		return "steps:read"
	default:
		return "projects:read"
	}
}

func scopeForProjectEvidence(resource string) string {
	switch resource {
	case "artifacts":
		return "artifacts:read"
	default:
		return "steps:read"
	}
}

func parseAuditLimit(raw string) int {
	const (
		defaultLimit = 100
		maxLimit     = 1000
	)
	if raw == "" {
		return defaultLimit
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}

func (s *Server) resolveResourceRoute(ctx context.Context, principal *plannerPrincipal, resourceKind, resourceID, scope, expectedInstanceID string) (string, *apiError) {
	routedInstance, err := s.store.LookupResourceRoute(ctx, resourceKind, resourceID)
	if err != nil {
		return "", &apiError{Status: http.StatusInternalServerError, Code: "relay_internal_error", Message: err.Error()}
	}
	var hintedErr *apiError
	if routedInstance != "" {
		switch {
		case expectedInstanceID != "" && routedInstance == expectedInstanceID:
			if apiErr := authorizePrincipal(principal, scope, routedInstance); apiErr == nil {
				if ok, routeErr := s.canRouteToInstance(ctx, routedInstance); ok {
					return routedInstance, nil
				} else if routeErr != nil {
					hintedErr = routeErr
					if routeErr.Code == "instance_not_found" {
						_ = s.store.DeleteResourceRoute(ctx, resourceKind, resourceID)
					}
				}
			}
		case expectedInstanceID == "":
			if apiErr := authorizePrincipal(principal, scope, routedInstance); apiErr == nil {
				if ok, routeErr := s.canRouteToInstance(ctx, routedInstance); ok {
					return routedInstance, nil
				} else if routeErr != nil {
					hintedErr = routeErr
					if routeErr.Code == "instance_not_found" {
						_ = s.store.DeleteResourceRoute(ctx, resourceKind, resourceID)
					}
				}
			}
		}
	}

	candidates, apiErr := s.resourceProbeCandidates(ctx, principal, scope, expectedInstanceID)
	if apiErr != nil {
		if hintedErr != nil && (apiErr.Code == "connector_offline" || apiErr.Code == "instance_not_found") {
			return "", hintedErr
		}
		return "", apiErr
	}

	matches := make([]string, 0, 1)
	var lastErr *apiError
	for _, candidate := range candidates {
		found, probeErr := s.probeResourceRoute(ctx, candidate.InstanceID, resourceKind, resourceID)
		if probeErr != nil {
			if probeErr.Status == http.StatusNotFound {
				continue
			}
			lastErr = probeErr
			continue
		}
		if found {
			matches = append(matches, candidate.InstanceID)
		}
	}

	switch len(matches) {
	case 1:
		_ = s.store.SaveResourceRoute(ctx, resourceKind, resourceID, matches[0])
		return matches[0], nil
	case 0:
		if hintedErr != nil {
			return "", hintedErr
		}
		if lastErr != nil {
			return "", lastErr
		}
		if expectedInstanceID != "" {
			return "", &apiError{Status: http.StatusNotFound, Code: "instance_not_found", Message: fmt.Sprintf("%s not found on instance %s", resourceKind, expectedInstanceID)}
		}
		return "", &apiError{Status: http.StatusNotFound, Code: "instance_not_found", Message: fmt.Sprintf("%s route not found", resourceKind)}
	default:
		sort.Strings(matches)
		return "", &apiError{Status: http.StatusConflict, Code: "instance_ambiguous", Message: fmt.Sprintf("%s %s matched multiple instances: %s", resourceKind, resourceID, strings.Join(matches, ", "))}
	}
}

func (s *Server) resourceProbeCandidates(ctx context.Context, principal *plannerPrincipal, scope, expectedInstanceID string) ([]InstanceRecord, *apiError) {
	if expectedInstanceID != "" {
		if apiErr := authorizePrincipal(principal, scope, expectedInstanceID); apiErr != nil {
			return nil, apiErr
		}
		record, err := s.store.GetInstance(ctx, expectedInstanceID)
		if err != nil {
			return nil, &apiError{Status: http.StatusInternalServerError, Code: "relay_internal_error", Message: err.Error()}
		}
		if record == nil {
			return nil, &apiError{Status: http.StatusNotFound, Code: "instance_not_found", Message: "instance not found"}
		}
		connectorRecord, err := s.store.GetConnector(ctx, record.ConnectorID)
		if err != nil {
			return nil, &apiError{Status: http.StatusInternalServerError, Code: "relay_internal_error", Message: err.Error()}
		}
		if connectorRecord != nil && connectorRecord.Disabled {
			return nil, &apiError{Status: http.StatusForbidden, Code: "connector_disabled", Message: "connector for this instance is disabled"}
		}
		if s.hub.Get(expectedInstanceID) == nil {
			return nil, &apiError{Status: http.StatusServiceUnavailable, Code: "connector_offline", Message: "connector for this instance is offline"}
		}
		return []InstanceRecord{*record}, nil
	}

	records, err := s.store.ListInstances(ctx)
	if err != nil {
		return nil, &apiError{Status: http.StatusInternalServerError, Code: "relay_internal_error", Message: err.Error()}
	}
	candidates := make([]InstanceRecord, 0, len(records))
	for _, record := range records {
		if authorizePrincipal(principal, scope, record.InstanceID) != nil {
			continue
		}
		connectorRecord, err := s.store.GetConnector(ctx, record.ConnectorID)
		if err != nil {
			return nil, &apiError{Status: http.StatusInternalServerError, Code: "relay_internal_error", Message: err.Error()}
		}
		if connectorRecord != nil && connectorRecord.Disabled {
			continue
		}
		if s.hub.Get(record.InstanceID) == nil {
			continue
		}
		candidates = append(candidates, record)
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].InstanceID < candidates[j].InstanceID })
	if len(candidates) == 0 {
		return nil, &apiError{Status: http.StatusServiceUnavailable, Code: "connector_offline", Message: "no authorized online instances are available"}
	}
	return candidates, nil
}

func (s *Server) probeResourceRoute(ctx context.Context, instanceID, resourceKind, resourceID string) (bool, *apiError) {
	path := resourceProbePath(resourceKind, resourceID)
	response, _, apiErr := s.proxyRequest(ctx, instanceID, http.MethodGet, path, "", nil, "")
	if apiErr != nil {
		return false, apiErr
	}
	if response == nil || response.StatusCode >= 400 {
		return false, &apiError{Status: http.StatusNotFound, Code: "instance_not_found", Message: fmt.Sprintf("%s route not found", resourceKind)}
	}
	return true, nil
}

func (s *Server) canRouteToInstance(ctx context.Context, instanceID string) (bool, *apiError) {
	record, err := s.store.GetInstance(ctx, instanceID)
	if err != nil {
		return false, &apiError{Status: http.StatusInternalServerError, Code: "relay_internal_error", Message: err.Error()}
	}
	if record == nil {
		return false, &apiError{Status: http.StatusNotFound, Code: "instance_not_found", Message: "instance not found"}
	}
	connectorRecord, err := s.store.GetConnector(ctx, record.ConnectorID)
	if err != nil {
		return false, &apiError{Status: http.StatusInternalServerError, Code: "relay_internal_error", Message: err.Error()}
	}
	if connectorRecord != nil && connectorRecord.Disabled {
		return false, &apiError{Status: http.StatusForbidden, Code: "connector_disabled", Message: "connector for this instance is disabled"}
	}
	if s.hub.Get(instanceID) == nil {
		return false, &apiError{Status: http.StatusServiceUnavailable, Code: "connector_offline", Message: "connector for this instance is offline"}
	}
	return true, nil
}

func resourceProbePath(resourceKind, resourceID string) string {
	switch resourceKind {
	case "step":
		return fmt.Sprintf("/api/v1/steps/%s", resourceID)
	case "artifact":
		return fmt.Sprintf("/api/v1/artifacts/%s", resourceID)
	case "gate":
		return fmt.Sprintf("/api/v1/gates/%s", resourceID)
	default:
		return ""
	}
}
