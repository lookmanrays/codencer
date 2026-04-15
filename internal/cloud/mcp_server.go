package cloud

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	mcpHeaderProtocolVersion = "MCP-Protocol-Version"
	mcpHeaderSessionID       = "MCP-Session-Id"

	mcpDefaultProtocolVersion = "2025-03-26"
	mcpLatestProtocolVersion  = "2025-11-25"
)

var supportedMCPProtocolVersions = []string{
	"2025-11-25",
	"2025-06-18",
	"2025-03-26",
}

type mcpServer struct {
	cloud    *Server
	tools    map[string]mcpTool
	mu       sync.Mutex
	sessions map[string]*mcpSession
}

type mcpSession struct {
	ID              string
	ProtocolVersion string
	CreatedAt       time.Time
	LastSeenAt      time.Time
	done            chan struct{}
	closeOnce       sync.Once
}

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Name    string          `json:"name,omitempty"`
	Args    json.RawMessage `json:"arguments,omitempty"`
}

type mcpResponse struct {
	JSONRPC string       `json:"jsonrpc"`
	ID      any          `json:"id,omitempty"`
	Result  any          `json:"result,omitempty"`
	Error   *mcpRPCError `json:"error,omitempty"`
}

type mcpRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type mcpToolResult struct {
	Content           []map[string]string `json:"content,omitempty"`
	StructuredContent any                 `json:"structuredContent,omitempty"`
	IsError           bool                `json:"isError,omitempty"`
}

func newMCPServer(cloudServer *Server) *mcpServer {
	server := &mcpServer{
		cloud:    cloudServer,
		sessions: make(map[string]*mcpSession),
	}
	server.tools = buildMCPTools(server)
	return server
}

func (s *mcpServer) Handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Codencer-MCP-Surface", "cloud-runtime")

	if apiErr := s.applyOriginHeaders(w, r); apiErr != nil {
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPost && r.Method != http.MethodDelete {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	token, apiErr := s.cloud.authenticateToken(r, "runtime_instances:read")
	if apiErr != nil {
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	r = r.WithContext(context.WithValue(r.Context(), cloudTokenKey{}, token))

	session, apiErr := s.sessionFromRequest(r)
	if apiErr != nil {
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.handleStream(w, r, session)
	case http.MethodDelete:
		s.handleSessionDelete(w, session)
	case http.MethodPost:
		s.handlePost(w, r, session)
	}
}

func (s *mcpServer) handleStream(w http.ResponseWriter, r *http.Request, session *mcpSession) {
	if session == nil {
		writeAPIError(w, http.StatusBadRequest, "malformed_request", "MCP-Session-Id header is required")
		return
	}
	protocolVersion, apiErr := s.resolveProtocolVersion(r, session)
	if apiErr != nil {
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	if !acceptsEventStream(r.Header.Values("Accept")) {
		writeAPIError(w, http.StatusBadRequest, "malformed_request", "Accept must include text/event-stream")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeAPIError(w, http.StatusInternalServerError, "cloud_internal_error", "response streaming is unavailable")
		return
	}

	s.applySessionHeaders(w, session, protocolVersion)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	_, _ = w.Write([]byte(": codencer-cloud-mcp-stream\n\n"))
	flusher.Flush()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-session.done:
			return
		case <-ticker.C:
			if _, err := w.Write([]byte(": keepalive\n\n")); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *mcpServer) handleSessionDelete(w http.ResponseWriter, session *mcpSession) {
	if session == nil {
		writeAPIError(w, http.StatusBadRequest, "malformed_request", "MCP-Session-Id header is required")
		return
	}
	s.deleteSession(session.ID)
	w.WriteHeader(http.StatusNoContent)
}

func (s *mcpServer) handlePost(w http.ResponseWriter, r *http.Request, session *mcpSession) {
	headerProtocolVersion := strings.TrimSpace(r.Header.Get(mcpHeaderProtocolVersion))
	if headerProtocolVersion != "" && !isSupportedMCPProtocolVersion(headerProtocolVersion) {
		writeAPIError(w, http.StatusBadRequest, "unsupported_protocol_version", "unsupported MCP protocol version")
		return
	}

	var req mcpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeRPC(w, mcpResponse{
			JSONRPC: "2.0",
			Error:   &mcpRPCError{Code: -32700, Message: "parse error", Data: err.Error()},
		}, session, protocolVersionOrDefault(headerProtocolVersion))
		return
	}
	if req.JSONRPC == "" {
		req.JSONRPC = "2.0"
	}
	if req.Method == "" && req.Name != "" {
		params, _ := json.Marshal(map[string]any{
			"name":      req.Name,
			"arguments": rawObjectOrEmpty(req.Args),
		})
		req.Method = "tools/call"
		req.Params = params
	}
	if req.Method == "" {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	switch req.Method {
	case "initialize":
		s.handleInitialize(w, req, session, headerProtocolVersion)
	case "notifications/initialized":
		protocolVersion, apiErr := s.resolveProtocolVersion(r, session)
		if apiErr != nil {
			writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
			return
		}
		s.applySessionHeaders(w, session, protocolVersion)
		w.WriteHeader(http.StatusAccepted)
	case "tools/list":
		protocolVersion, apiErr := s.resolveProtocolVersion(r, session)
		if apiErr != nil {
			writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
			return
		}
		s.writeRPC(w, mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]any{"tools": s.listTools()},
		}, session, protocolVersion)
	case "tools/call":
		s.handleToolCall(w, r, req, session)
	default:
		protocolVersion, apiErr := s.resolveProtocolVersion(r, session)
		if apiErr != nil {
			writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
			return
		}
		s.writeRPC(w, mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &mcpRPCError{Code: -32601, Message: "method not found", Data: req.Method},
		}, session, protocolVersion)
	}
}

func (s *mcpServer) handleInitialize(w http.ResponseWriter, req mcpRequest, session *mcpSession, headerProtocolVersion string) {
	var params struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	_ = json.Unmarshal(req.Params, &params)
	protocolVersion := negotiateProtocolVersion(params.ProtocolVersion, headerProtocolVersion)
	session = s.ensureSession(session, protocolVersion)
	s.writeRPC(w, mcpResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": "codencer-cloud", "version": "v1-alpha"},
		},
	}, session, protocolVersion)
}

func (s *mcpServer) handleToolCall(w http.ResponseWriter, r *http.Request, req mcpRequest, session *mcpSession) {
	protocolVersion, apiErr := s.resolveProtocolVersion(r, session)
	if apiErr != nil {
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.writeRPC(w, mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &mcpRPCError{Code: -32602, Message: "invalid params", Data: err.Error()},
		}, session, protocolVersion)
		return
	}
	tool, ok := s.tools[params.Name]
	if !ok {
		s.writeRPC(w, mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  errorToolResult("tool_not_found", fmt.Sprintf("unknown tool: %s", params.Name)),
		}, session, protocolVersion)
		return
	}
	if params.Arguments == nil {
		params.Arguments = map[string]any{}
	}
	token, _ := r.Context().Value(cloudTokenKey{}).(*APIToken)
	if !TokenHasScope(token, tool.Scope) {
		s.writeRPC(w, mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  errorToolResult("scope_denied", "api token lacks required scope"),
		}, session, protocolVersion)
		return
	}
	result, apiErr := tool.Invoke(r.Context(), token, params.Arguments)
	if apiErr != nil {
		s.writeRPC(w, mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  errorToolResult(apiErr.Code, apiErr.Message),
		}, session, protocolVersion)
		return
	}
	s.writeRPC(w, mcpResponse{JSONRPC: "2.0", ID: req.ID, Result: result}, session, protocolVersion)
}

func (s *mcpServer) listTools() []map[string]any {
	tools := make([]map[string]any, 0, len(s.tools))
	for _, tool := range toolOrder(s.tools) {
		tools = append(tools, map[string]any{
			"name":        tool.Name,
			"description": tool.Description,
			"inputSchema": tool.InputSchema,
		})
	}
	return tools
}

func (s *mcpServer) applyOriginHeaders(w http.ResponseWriter, r *http.Request) *apiError {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return nil
	}
	if !originAllowed(origin, r.Host, s.cloud.cfg.Host) {
		return &apiError{Status: http.StatusForbidden, Code: "origin_denied", Message: "origin is not allowed for the cloud MCP surface"}
	}
	w.Header().Set("Vary", "Origin")
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, "+mcpHeaderProtocolVersion+", "+mcpHeaderSessionID+", Last-Event-ID")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Expose-Headers", mcpHeaderProtocolVersion+", "+mcpHeaderSessionID)
	return nil
}

func (s *mcpServer) sessionFromRequest(r *http.Request) (*mcpSession, *apiError) {
	sessionID := strings.TrimSpace(r.Header.Get(mcpHeaderSessionID))
	if sessionID == "" {
		return nil, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[sessionID]
	if !ok {
		return nil, &apiError{Status: http.StatusNotFound, Code: "session_not_found", Message: "unknown MCP session"}
	}
	session.LastSeenAt = time.Now().UTC()
	return session, nil
}

func (s *mcpServer) ensureSession(existing *mcpSession, protocolVersion string) *mcpSession {
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing != nil {
		session := s.sessions[existing.ID]
		if session == nil {
			session = &mcpSession{ID: existing.ID, CreatedAt: existing.CreatedAt, done: make(chan struct{})}
		}
		session.ProtocolVersion = protocolVersion
		if session.CreatedAt.IsZero() {
			session.CreatedAt = now
		}
		if session.done == nil {
			session.done = make(chan struct{})
		}
		session.LastSeenAt = now
		s.sessions[session.ID] = session
		return session
	}
	session := &mcpSession{
		ID:              newMCPSessionID(),
		ProtocolVersion: protocolVersion,
		CreatedAt:       now,
		LastSeenAt:      now,
		done:            make(chan struct{}),
	}
	s.sessions[session.ID] = session
	return session
}

func (s *mcpServer) deleteSession(sessionID string) {
	if strings.TrimSpace(sessionID) == "" {
		return
	}
	s.mu.Lock()
	session := s.sessions[sessionID]
	delete(s.sessions, sessionID)
	s.mu.Unlock()
	if session != nil {
		session.closeOnce.Do(func() {
			if session.done != nil {
				close(session.done)
			}
		})
	}
}

func (s *mcpServer) resolveProtocolVersion(r *http.Request, session *mcpSession) (string, *apiError) {
	headerVersion := strings.TrimSpace(r.Header.Get(mcpHeaderProtocolVersion))
	if headerVersion != "" && !isSupportedMCPProtocolVersion(headerVersion) {
		return "", &apiError{Status: http.StatusBadRequest, Code: "unsupported_protocol_version", Message: "unsupported MCP protocol version"}
	}
	if session != nil {
		if headerVersion != "" && headerVersion != session.ProtocolVersion {
			return "", &apiError{Status: http.StatusBadRequest, Code: "protocol_version_mismatch", Message: "MCP-Protocol-Version does not match the negotiated session protocol"}
		}
		return session.ProtocolVersion, nil
	}
	return protocolVersionOrDefault(headerVersion), nil
}

func (s *mcpServer) applySessionHeaders(w http.ResponseWriter, session *mcpSession, protocolVersion string) {
	if session != nil {
		w.Header().Set(mcpHeaderSessionID, session.ID)
	}
	w.Header().Set(mcpHeaderProtocolVersion, protocolVersion)
}

func (s *mcpServer) writeRPC(w http.ResponseWriter, response mcpResponse, session *mcpSession, protocolVersion string) {
	s.applySessionHeaders(w, session, protocolVersion)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

func (s *mcpServer) authorizedRuntimeInstanceIDs(ctx context.Context, token *APIToken) ([]string, *apiError) {
	if token == nil {
		return nil, &apiError{Status: http.StatusUnauthorized, Code: "auth_failed", Message: "cloud token required"}
	}
	if err := s.cloud.syncRuntimeScope(ctx, token.OrgID, token.WorkspaceID, token.ProjectID); err != nil {
		return nil, &apiError{Status: http.StatusInternalServerError, Code: "runtime_sync_failed", Message: err.Error()}
	}
	instances, err := s.cloud.store.ListRuntimeInstances(ctx, token.OrgID, token.WorkspaceID, token.ProjectID, "")
	if err != nil {
		return nil, &apiError{Status: http.StatusInternalServerError, Code: "runtime_lookup_failed", Message: err.Error()}
	}
	ids := make([]string, 0, len(instances))
	for _, instance := range instances {
		if !instance.Shared || !instance.Enabled {
			continue
		}
		ids = append(ids, instance.ID)
	}
	if len(ids) == 0 {
		return nil, &apiError{Status: http.StatusNotFound, Code: "instance_not_found", Message: "no authorized runtime instances are available"}
	}
	return ids, nil
}

func (s *mcpServer) loadAuthorizedInstance(ctx context.Context, token *APIToken, instanceID string) (*RuntimeInstance, *apiError) {
	if strings.TrimSpace(instanceID) == "" {
		return nil, &apiError{Status: http.StatusBadRequest, Code: "malformed_request", Message: "instance_id is required"}
	}
	instance, err := s.cloud.store.GetRuntimeInstance(ctx, instanceID)
	if err != nil {
		return nil, &apiError{Status: http.StatusNotFound, Code: "instance_not_found", Message: err.Error()}
	}
	if !TokenAllowsTarget(token, instance.OrgID, instance.WorkspaceID, instance.ProjectID) {
		return nil, &apiError{Status: http.StatusForbidden, Code: "scope_denied", Message: "token is not allowed to access this runtime instance"}
	}
	connector, err := s.cloud.store.GetRuntimeConnectorInstallation(ctx, instance.RuntimeConnectorInstallationID)
	if err == nil && connector != nil {
		if _, _, syncErr := s.cloud.syncRuntimeConnectorFromRelay(ctx, *connector); syncErr == nil {
			instance, _ = s.cloud.store.GetRuntimeInstance(ctx, instanceID)
		}
	}
	if instance == nil || !instance.Shared {
		return nil, &apiError{Status: http.StatusConflict, Code: "instance_not_shared", Message: "runtime instance is not currently shared"}
	}
	if !instance.Enabled {
		return nil, &apiError{Status: http.StatusConflict, Code: "instance_disabled", Message: "runtime instance is disabled"}
	}
	return instance, nil
}

func (s *mcpServer) callRuntimeRoute(ctx context.Context, token *APIToken, method, path string, body []byte, instanceIDs []string, scope string) (int, http.Header, []byte, *apiError) {
	if s.cloud.runtime == nil || s.cloud.runtime.Server == nil {
		return 0, nil, nil, &apiError{Status: http.StatusServiceUnavailable, Code: "runtime_bridge_missing", Message: "cloud runtime bridge is not configured"}
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://codencer.internal"+path, bytes.NewReader(body))
	if err != nil {
		return 0, nil, nil, &apiError{Status: http.StatusInternalServerError, Code: "cloud_internal_error", Message: err.Error()}
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	name := "cloud"
	if token != nil {
		name = "cloud:" + firstNonEmpty(token.SubjectName, token.Name, token.ID)
	}
	s.cloud.runtime.Server.ServeAsPlanner(recorder, req, name, []string{scope}, instanceIDs)
	resp := recorder.Result()
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, nil, &apiError{Status: http.StatusInternalServerError, Code: "cloud_internal_error", Message: err.Error()}
	}
	if resp.StatusCode >= 400 {
		return resp.StatusCode, resp.Header.Clone(), nil, decodeHTTPAPIError(resp.StatusCode, data)
	}
	return resp.StatusCode, resp.Header.Clone(), data, nil
}

type cloudTokenKey struct{}

func decodeHTTPAPIError(status int, body []byte) *apiError {
	var payload struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err == nil && payload.Error.Code != "" {
		return &apiError{Status: status, Code: payload.Error.Code, Message: payload.Error.Message}
	}
	return &apiError{Status: status, Code: "upstream_error", Message: strings.TrimSpace(string(body))}
}

func rawObjectOrEmpty(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		return map[string]any{}
	}
	return value
}

func acceptsEventStream(values []string) bool {
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			if strings.TrimSpace(strings.ToLower(part)) == "text/event-stream" {
				return true
			}
		}
	}
	return false
}

func negotiateProtocolVersion(bodyVersion, headerVersion string) string {
	for _, candidate := range []string{strings.TrimSpace(headerVersion), strings.TrimSpace(bodyVersion)} {
		if candidate != "" && isSupportedMCPProtocolVersion(candidate) {
			return candidate
		}
	}
	return mcpLatestProtocolVersion
}

func protocolVersionOrDefault(value string) string {
	if strings.TrimSpace(value) == "" {
		return mcpDefaultProtocolVersion
	}
	return value
}

func isSupportedMCPProtocolVersion(value string) bool {
	for _, version := range supportedMCPProtocolVersions {
		if value == version {
			return true
		}
	}
	return false
}

func newMCPSessionID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err == nil {
		return "mcp-" + base64.RawURLEncoding.EncodeToString(buf[:])
	}
	return fmt.Sprintf("mcp-%d", time.Now().UnixNano())
}

func originAllowed(origin, requestHost, cfgHost string) bool {
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	originHost := normalizeHost(parsed.Host)
	requestHost = normalizeHost(requestHost)
	cfgHost = normalizeHost(cfgHost)
	if originHost == "" {
		return false
	}
	if originHost == requestHost || (cfgHost != "" && originHost == cfgHost) {
		return true
	}
	return isLoopbackHost(originHost) && (isLoopbackHost(requestHost) || isLoopbackHost(cfgHost))
}

func normalizeHost(value string) string {
	host, _, err := net.SplitHostPort(value)
	if err == nil {
		return strings.ToLower(strings.TrimSpace(host))
	}
	return strings.ToLower(strings.TrimSpace(value))
}

func isLoopbackHost(value string) bool {
	value = normalizeHost(value)
	switch value {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	ip := net.ParseIP(value)
	return ip != nil && ip.IsLoopback()
}
