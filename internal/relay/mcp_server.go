package relay

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
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
	relay    *Server
	tools    map[string]mcpTool
	mu       sync.Mutex
	sessions map[string]*mcpSession
}

type mcpSession struct {
	ID              string
	ProtocolVersion string
	CreatedAt       time.Time
	LastSeenAt      time.Time
}

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Name    string          `json:"name,omitempty"`
	Args    json.RawMessage `json:"arguments,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   any             `json:"error,omitempty"`
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

func newMCPServer(relayServer *Server) *mcpServer {
	server := &mcpServer{
		relay:    relayServer,
		sessions: make(map[string]*mcpSession),
	}
	server.tools = buildMCPTools(server)
	return server
}

func (s *mcpServer) Handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Codencer-MCP-Surface", "relay-public")

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

	principal, err := s.relay.authenticatePlanner(r, "", "")
	if err != nil {
		writeAPIError(w, err.Status, err.Code, err.Message)
		return
	}
	r = r.WithContext(context.WithValue(r.Context(), plannerPrincipalKey{}, principal))

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
	protocolVersion, apiErr := s.resolveProtocolVersion(r, session)
	if apiErr != nil {
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}

	if session != nil {
		s.applySessionHeaders(w, session, protocolVersion)
	} else {
		w.Header().Set(mcpHeaderProtocolVersion, protocolVersion)
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	_, _ = w.Write([]byte(": codencer-relay-mcp-stream\n\n"))
}

func (s *mcpServer) handleSessionDelete(w http.ResponseWriter, session *mcpSession) {
	if session == nil {
		writeAPIError(w, http.StatusBadRequest, "malformed_request", "MCP-Session-Id header is required")
		return
	}
	s.mu.Lock()
	delete(s.sessions, session.ID)
	s.mu.Unlock()
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
			ID:      nil,
			Error: &mcpRPCError{
				Code:    -32700,
				Message: "parse error",
				Data:    err.Error(),
			},
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
		if session != nil {
			s.applySessionHeaders(w, session, protocolVersion)
		} else {
			w.Header().Set(mcpHeaderProtocolVersion, protocolVersion)
		}
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
			Result: map[string]any{
				"tools": s.listTools(),
			},
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
			Error: &mcpRPCError{
				Code:    -32601,
				Message: "method not found",
				Data:    req.Method,
			},
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
			"capabilities": map[string]any{
				"tools": map[string]any{"listChanged": false},
			},
			"serverInfo": map[string]any{
				"name":    "codencer-relay",
				"version": "v2",
			},
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
			Error: &mcpRPCError{
				Code:    -32602,
				Message: "invalid params",
				Data:    err.Error(),
			},
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

	principal := plannerFromContext(r.Context())
	if err := authorizePrincipal(principal, tool.Scope, tool.instanceID(params.Arguments)); err != nil {
		s.writeRPC(w, mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  errorToolResult(err.Code, err.Message),
		}, session, protocolVersion)
		return
	}

	result, apiErr := tool.Invoke(r.Context(), principal, params.Arguments)
	if apiErr != nil {
		s.writeRPC(w, mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  errorToolResult(apiErr.Code, apiErr.Message),
		}, session, protocolVersion)
		return
	}
	s.writeRPC(w, mcpResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}, session, protocolVersion)
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
	if !s.originAllowed(origin, r.Host) {
		return &apiError{Status: http.StatusForbidden, Code: "origin_denied", Message: "origin is not allowed for the relay MCP surface"}
	}
	w.Header().Set("Vary", "Origin")
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, "+mcpHeaderProtocolVersion+", "+mcpHeaderSessionID+", Last-Event-ID")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Expose-Headers", mcpHeaderProtocolVersion+", "+mcpHeaderSessionID)
	return nil
}

func (s *mcpServer) originAllowed(origin, requestHost string) bool {
	if origin == "" {
		return true
	}
	if len(s.relay.cfg.AllowedOrigins) > 0 {
		for _, allowed := range s.relay.cfg.AllowedOrigins {
			if allowed == "*" || strings.EqualFold(strings.TrimSpace(allowed), origin) {
				return true
			}
		}
		return false
	}

	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	originHost := normalizeHost(parsed.Host)
	requestHost = normalizeHost(requestHost)
	cfgHost := normalizeHost(s.relay.cfg.Host)
	if originHost == "" {
		return false
	}
	if originHost == requestHost || (cfgHost != "" && originHost == cfgHost) {
		return true
	}
	return isLoopbackHost(originHost) && (isLoopbackHost(requestHost) || isLoopbackHost(cfgHost))
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
	return cloneMCPSession(session), nil
}

func (s *mcpServer) ensureSession(existing *mcpSession, protocolVersion string) *mcpSession {
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing != nil {
		session := s.sessions[existing.ID]
		if session == nil {
			session = &mcpSession{ID: existing.ID, CreatedAt: existing.CreatedAt}
		}
		session.ProtocolVersion = protocolVersion
		if session.CreatedAt.IsZero() {
			session.CreatedAt = now
		}
		session.LastSeenAt = now
		s.sessions[session.ID] = session
		return cloneMCPSession(session)
	}
	session := &mcpSession{
		ID:              newMCPSessionID(),
		ProtocolVersion: protocolVersion,
		CreatedAt:       now,
		LastSeenAt:      now,
	}
	s.sessions[session.ID] = session
	return cloneMCPSession(session)
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

func errorToolResult(code, message string) mcpToolResult {
	return mcpToolResult{
		IsError: true,
		Content: []map[string]string{{
			"type": "text",
			"text": message,
		}},
		StructuredContent: map[string]any{
			"error": map[string]any{
				"code":    code,
				"message": message,
			},
		},
	}
}

func successToolResult(summary string, payload any) mcpToolResult {
	result := mcpToolResult{StructuredContent: payload}
	if summary != "" {
		result.Content = []map[string]string{{
			"type": "text",
			"text": summary,
		}}
	}
	return result
}

func (s *mcpServer) callPlannerRoute(ctx context.Context, authHeader, method, path string, body []byte) (int, http.Header, []byte, *apiError) {
	req, err := http.NewRequest(method, path, bytes.NewReader(body))
	if err != nil {
		return 0, nil, nil, &apiError{Status: http.StatusInternalServerError, Code: "relay_internal_error", Message: err.Error()}
	}
	req = req.WithContext(ctx)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	recorder := newResponseRecorder()
	handler, _ := s.relay.server.Handler.(*http.ServeMux)
	handler.ServeHTTP(recorder, req)
	bodyBytes := recorder.body.Bytes()
	if recorder.statusCode >= 400 {
		return recorder.statusCode, recorder.header, bodyBytes, decodeAPIError(recorder.statusCode, bodyBytes)
	}
	return recorder.statusCode, recorder.header, bodyBytes, nil
}

func decodeAPIError(status int, body []byte) *apiError {
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

func artifactContentPayload(contentType string, body []byte) map[string]any {
	payload := map[string]any{
		"content_type": contentType,
	}
	if strings.HasPrefix(contentType, "text/") || strings.Contains(contentType, "json") {
		payload["encoding"] = "utf-8"
		payload["text"] = string(body)
		return payload
	}
	payload["encoding"] = "base64"
	payload["base64"] = base64.StdEncoding.EncodeToString(body)
	return payload
}

func negotiateProtocolVersion(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if isSupportedMCPProtocolVersion(value) {
			return value
		}
	}
	return mcpLatestProtocolVersion
}

func protocolVersionOrDefault(value string) string {
	value = strings.TrimSpace(value)
	if value != "" && isSupportedMCPProtocolVersion(value) {
		return value
	}
	return mcpDefaultProtocolVersion
}

func isSupportedMCPProtocolVersion(value string) bool {
	for _, supported := range supportedMCPProtocolVersions {
		if value == supported {
			return true
		}
	}
	return false
}

func newMCPSessionID() string {
	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("mcp-%d", time.Now().UnixNano())
	}
	return "mcp-" + base64.RawURLEncoding.EncodeToString(buf)
}

func cloneMCPSession(session *mcpSession) *mcpSession {
	if session == nil {
		return nil
	}
	clone := *session
	return &clone
}

func normalizeHost(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.Contains(value, "://") {
		parsed, err := url.Parse(value)
		if err == nil {
			return normalizeHost(parsed.Host)
		}
	}
	if host, _, err := net.SplitHostPort(value); err == nil && host != "" {
		return strings.Trim(host, "[]")
	}
	if strings.Count(value, ":") == 1 {
		if host, _, found := strings.Cut(value, ":"); found && host != "" {
			return strings.Trim(host, "[]")
		}
	}
	return strings.Trim(value, "[]")
}

func isLoopbackHost(value string) bool {
	switch strings.TrimSpace(strings.Trim(value, "[]")) {
	case "127.0.0.1", "localhost", "::1":
		return true
	default:
		return false
	}
}
