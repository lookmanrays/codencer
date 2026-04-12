package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/relayproto"
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
			status := s.instanceStatus(&record)
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
		status := s.instanceStatus(record)
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
	instanceID, err := s.store.LookupResourceRoute(r.Context(), "step", parts[0])
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "relay_internal_error", err.Error())
		return
	}
	if instanceID == "" {
		writeAPIError(w, http.StatusNotFound, "instance_not_found", "step route not found")
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
	s.proxyAndWrite(w, r, instanceID, r.Method, targetPath, "", body, routeKind, routeIDField, action, scope)
}

func (s *Server) handleArtifactScoped(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v2/artifacts/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 || parts[1] != "content" || r.Method != http.MethodGet {
		writeAPIError(w, http.StatusBadRequest, "malformed_request", "artifact_id/content path required")
		return
	}
	instanceID, err := s.store.LookupResourceRoute(r.Context(), "artifact", parts[0])
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "relay_internal_error", err.Error())
		return
	}
	if instanceID == "" {
		writeAPIError(w, http.StatusNotFound, "instance_not_found", "artifact route not found")
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
	instanceID, err := s.store.LookupResourceRoute(r.Context(), "gate", parts[0])
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "relay_internal_error", err.Error())
		return
	}
	if instanceID == "" {
		writeAPIError(w, http.StatusNotFound, "instance_not_found", "gate route not found")
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

func (s *Server) proxyRequest(ctx context.Context, instanceID, method, path, query string, body []byte, _ string) (*relayproto.CommandResponse, *InstanceRecord, *apiError) {
	record, err := s.store.GetInstance(ctx, instanceID)
	if err != nil {
		return nil, nil, &apiError{Status: http.StatusInternalServerError, Code: "relay_internal_error", Message: err.Error()}
	}
	if record == nil {
		return nil, nil, &apiError{Status: http.StatusNotFound, Code: "instance_not_found", Message: "instance not found"}
	}
	session := s.hub.Get(instanceID)
	if session == nil {
		return nil, record, &apiError{Status: http.StatusServiceUnavailable, Code: "connector_offline", Message: "connector for this instance is offline"}
	}
	requestID, err := randomID("req")
	if err != nil {
		return nil, record, &apiError{Status: http.StatusInternalServerError, Code: "relay_internal_error", Message: err.Error()}
	}
	timeout := 15 * time.Second
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
