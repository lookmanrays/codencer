package gateway

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"agent-bridge/internal/mcpauth"
	"agent-bridge/internal/security"
)

const (
	headerProtocolVersion = "MCP-Protocol-Version"
	headerSessionID       = "MCP-Session-Id"

	defaultProtocolVersion = "2025-03-26"
	latestProtocolVersion  = "2025-11-25"
	sessionIdleTTL         = 30 * time.Minute
	gatewayInstructions    = "Codencer is a bridge, not a planner. Official clients connect to Codencer Gateway. Use codencer.list_projects first, select relay_profile_id/machine_id/host_label when required, and stop on structured blockers."
)

var supportedProtocolVersions = []string{"2025-11-25", "2025-06-18", "2025-03-26"}

type Server struct {
	cfg            *Config
	store          *Store
	client         *http.Client
	tools          map[string]Tool
	oauth          *oauthDevService
	mu             sync.Mutex
	sessions       map[string]*session
	started        time.Time
	devUserID      string
	devWorkspaceID string
}

type Tool struct {
	Name           string
	Description    string
	InputSchema    map[string]any
	ReadOnly       bool
	RequiredScopes []string
	Invoke         func(ctx context.Context, principal *authPrincipal, args map[string]any) (ToolResult, *apiError)
}

type ToolResult struct {
	Content           []map[string]string `json:"content,omitempty"`
	StructuredContent any                 `json:"structuredContent,omitempty"`
	IsError           bool                `json:"isError,omitempty"`
}

type apiError struct {
	Status  int            `json:"-"`
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Blocker map[string]any `json:"blocker,omitempty"`
}

type session struct {
	ID              string
	TokenHash       string
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

type authPrincipal struct {
	Name        string
	TokenHash   string
	UserID      string
	WorkspaceID string
	Scopes      []string
}

type ServerOptions struct {
	HTTPClient *http.Client
}

func NewServer(cfg *Config, opts ServerOptions) (*Server, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	var store *Store
	if strings.TrimSpace(cfg.Store.Path) != "" {
		opened, err := OpenStore(cfg.Store.Path)
		if err != nil {
			return nil, err
		}
		store = opened
	}
	server := &Server{
		cfg:      cfg,
		store:    store,
		client:   client,
		oauth:    newOAuthDevService(cfg),
		sessions: make(map[string]*session),
		started:  time.Now().UTC(),
	}
	if server.store != nil {
		if account, err := server.store.EnsureUserWorkspace(context.Background(), "gateway-dev@codencer.local", "Gateway Dev User", cfg.DefaultRelay); err == nil {
			server.devUserID = account.User.ID
			server.devWorkspaceID = account.Workspace.ID
			if server.oauth != nil {
				server.oauth.defaultUserID = account.User.ID
				server.oauth.defaultWorkspaceID = account.Workspace.ID
			}
		}
	}
	server.tools = buildTools(server)
	return server, nil
}

func (s *Server) Start(ctx context.Context) error {
	httpServer := &http.Server{Addr: s.cfg.ListenAddr, Handler: s.Handler()}
	errCh := make(chan error, 1)
	go func() {
		err := httpServer.ListenAndServe()
		if err == http.ErrServerClosed {
			err = nil
		}
		errCh <- err
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
		return <-errCh
	case err := <-errCh:
		return err
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/gateway/v1/status", s.handleStatus)
	mux.HandleFunc("/api/gateway/v1/device/authorize", s.handleDeviceAuthorize)
	mux.HandleFunc("/api/gateway/v1/device/approve", s.handleDeviceApprove)
	mux.HandleFunc("/api/gateway/v1/device/token", s.handleDeviceToken)
	mux.HandleFunc("/api/gateway/v1/whoami", s.handleWhoami)
	mux.HandleFunc("/api/gateway/v1/logout", s.handleLogout)
	mux.HandleFunc("/api/gateway/v1/relays", s.handleRelays)
	mux.HandleFunc("/api/gateway/v1/relays/", s.handleRelayByID)
	mux.HandleFunc("/api/gateway/v1/connectors/login", s.handleConnectorLogin)
	mux.HandleFunc("/api/gateway/v1/connectors/complete", s.handleConnectorComplete)
	mux.HandleFunc("/device", s.handleDevicePage)
	mux.HandleFunc("/.well-known/oauth-protected-resource/mcp", s.handleProtectedResource)
	mux.HandleFunc("/.well-known/oauth-authorization-server", s.handleOAuthAuthorizationServer)
	mux.HandleFunc("/.well-known/openid-configuration", s.handleOpenIDConfiguration)
	mux.HandleFunc("/oauth/authorize", s.handleOAuthAuthorize)
	mux.HandleFunc("/oauth/token", s.handleOAuthToken)
	mux.HandleFunc("/mcp", s.handleMCP)
	mux.HandleFunc("/mcp/call", s.handleMCP)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "service": "codencer-gatewayd"})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if _, apiErr := s.authenticate(r); apiErr != nil {
		s.addAuthChallenge(w, r, "")
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                  true,
		"service":             "codencer-gatewayd",
		"started_at":          s.started,
		"mcp_url":             s.cfg.MCPURL,
		"public_base_url":     s.cfg.PublicBaseURL,
		"auth_mode":           s.cfg.Auth.Mode,
		"oauth_dev_enabled":   s.cfg.OAuthDev.Enabled,
		"relay_profile_count": len(s.cfg.RelayProfiles),
	})
}

func (s *Server) handleProtectedResource(w http.ResponseWriter, r *http.Request) {
	if apiErr := s.applyOriginHeaders(w, r); apiErr != nil {
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, mcpauth.Metadata(s.baseURL(r), "/mcp", mcpauth.ProtectedResourceConfig{
		AuthorizationServers:   s.authorizationServers(r),
		ScopesSupported:        s.oauthScopes(),
		ResourceDocumentation:  s.cfg.MCPURL,
		ResourceName:           "Codencer Gateway MCP",
		BearerMethodsSupported: []string{"header"},
	}))
}

func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Codencer-MCP-Surface", "official-gateway")
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
	principal, apiErr := s.authenticate(r)
	if apiErr != nil {
		s.addAuthChallenge(w, r, "")
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	sess, apiErr := s.sessionFromRequest(r, principal)
	if apiErr != nil {
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.handleStream(w, r, sess)
	case http.MethodDelete:
		s.handleSessionDelete(w, sess)
	case http.MethodPost:
		s.handleMCPPost(w, r, sess, principal)
	}
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request, sess *session) {
	if sess == nil {
		writeAPIError(w, http.StatusBadRequest, "malformed_request", "MCP-Session-Id header is required")
		return
	}
	protocolVersion, apiErr := s.resolveProtocolVersion(r, sess)
	if apiErr != nil {
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeAPIError(w, http.StatusInternalServerError, "gateway_internal_error", "response streaming is unavailable")
		return
	}
	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
	s.applySessionHeaders(w, sess, protocolVersion)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write([]byte(": codencer-gateway-mcp-stream\n\n"))
	flusher.Flush()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-sess.done:
			return
		case <-ticker.C:
			if _, err := w.Write([]byte(": keepalive\n\n")); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *Server) handleSessionDelete(w http.ResponseWriter, sess *session) {
	if sess == nil {
		writeAPIError(w, http.StatusBadRequest, "malformed_request", "MCP-Session-Id header is required")
		return
	}
	s.deleteSession(sess.ID)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMCPPost(w http.ResponseWriter, r *http.Request, sess *session, principal *authPrincipal) {
	headerVersion := strings.TrimSpace(r.Header.Get(headerProtocolVersion))
	if headerVersion != "" && !isSupportedProtocolVersion(headerVersion) {
		writeAPIError(w, http.StatusBadRequest, "unsupported_protocol_version", "unsupported MCP protocol version")
		return
	}
	var req mcpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeRPC(w, mcpResponse{JSONRPC: "2.0", Error: &mcpRPCError{Code: -32700, Message: "parse error", Data: err.Error()}}, sess, protocolVersionOrDefault(headerVersion))
		return
	}
	if req.JSONRPC == "" {
		req.JSONRPC = "2.0"
	}
	if req.Method == "" && req.Name != "" {
		params, _ := json.Marshal(map[string]any{"name": req.Name, "arguments": rawObjectOrEmpty(req.Args)})
		req.Method = "tools/call"
		req.Params = params
	}
	switch req.Method {
	case "":
		w.WriteHeader(http.StatusAccepted)
	case "initialize":
		s.handleInitialize(w, r, req, sess, principal, headerVersion)
	case "notifications/initialized":
		protocolVersion, apiErr := s.resolveProtocolVersion(r, sess)
		if apiErr != nil {
			writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
			return
		}
		s.applySessionHeaders(w, sess, protocolVersion)
		w.WriteHeader(http.StatusAccepted)
	case "tools/list":
		protocolVersion, apiErr := s.resolveProtocolVersion(r, sess)
		if apiErr != nil {
			writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
			return
		}
		s.writeRPC(w, mcpResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"tools": s.listTools()}}, sess, protocolVersion)
	case "tools/call":
		s.handleToolCall(w, r, req, sess, principal)
	default:
		protocolVersion, apiErr := s.resolveProtocolVersion(r, sess)
		if apiErr != nil {
			writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
			return
		}
		s.writeRPC(w, mcpResponse{JSONRPC: "2.0", ID: req.ID, Error: &mcpRPCError{Code: -32601, Message: "method not found", Data: req.Method}}, sess, protocolVersion)
	}
}

func (s *Server) handleInitialize(w http.ResponseWriter, r *http.Request, req mcpRequest, sess *session, principal *authPrincipal, headerVersion string) {
	var params struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	_ = json.Unmarshal(req.Params, &params)
	protocolVersion := negotiateProtocolVersion(params.ProtocolVersion, headerVersion)
	sess = s.ensureSession(sess, principal, protocolVersion)
	s.writeRPC(w, mcpResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
		"serverInfo":      map[string]any{"name": "codencer-gateway", "version": "v1-mvp"},
		"instructions":    gatewayInstructions,
	}}, sess, protocolVersion)
}

func (s *Server) handleToolCall(w http.ResponseWriter, r *http.Request, req mcpRequest, sess *session, principal *authPrincipal) {
	protocolVersion, apiErr := s.resolveProtocolVersion(r, sess)
	if apiErr != nil {
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.writeRPC(w, mcpResponse{JSONRPC: "2.0", ID: req.ID, Error: &mcpRPCError{Code: -32602, Message: "invalid params", Data: err.Error()}}, sess, protocolVersion)
		return
	}
	tool, ok := s.tools[params.Name]
	if !ok {
		s.writeRPC(w, mcpResponse{JSONRPC: "2.0", ID: req.ID, Result: apiErrorToolResult(&apiError{Status: http.StatusNotFound, Code: "tool_not_found", Message: "unknown tool: " + params.Name})}, sess, protocolVersion)
		return
	}
	if !principalAllows(principal, tool.RequiredScopes) {
		s.writeRPC(w, mcpResponse{JSONRPC: "2.0", ID: req.ID, Result: apiErrorToolResult(&apiError{Status: http.StatusForbidden, Code: "insufficient_scope", Message: "Gateway token is missing required scope"})}, sess, protocolVersion)
		return
	}
	if params.Arguments == nil {
		params.Arguments = map[string]any{}
	}
	result, apiErr := tool.Invoke(r.Context(), principal, params.Arguments)
	if apiErr != nil {
		s.writeRPC(w, mcpResponse{JSONRPC: "2.0", ID: req.ID, Result: apiErrorToolResult(apiErr)}, sess, protocolVersion)
		return
	}
	s.writeRPC(w, mcpResponse{JSONRPC: "2.0", ID: req.ID, Result: result}, sess, protocolVersion)
}

func (s *Server) listTools() []map[string]any {
	names := make([]string, 0, len(s.tools))
	for name := range s.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]map[string]any, 0, len(names))
	for _, name := range names {
		tool := s.tools[name]
		payload := map[string]any{
			"name":        tool.Name,
			"description": tool.Description,
			"inputSchema": tool.InputSchema,
		}
		if tool.ReadOnly {
			payload["annotations"] = map[string]any{"readOnlyHint": true}
		}
		out = append(out, payload)
	}
	return out
}

func (s *Server) authenticate(r *http.Request) (*authPrincipal, *apiError) {
	token := bearerToken(r.Header.Get("Authorization"))
	if token == "" {
		return nil, &apiError{Status: http.StatusUnauthorized, Code: "auth_failed", Message: "gateway bearer token required"}
	}
	if s.store != nil {
		if record, err := s.store.LookupAccessToken(r.Context(), token); err == nil {
			return &authPrincipal{
				Name:        "gateway-token",
				TokenHash:   record.TokenHash,
				UserID:      record.UserID,
				WorkspaceID: record.WorkspaceID,
				Scopes:      append([]string(nil), record.Scopes...),
			}, nil
		}
	}
	expected, err := s.cfg.Auth.Token()
	if err == nil && expected != "" && token == expected {
		return &authPrincipal{Name: "gateway-bearer-dev", TokenHash: tokenHash(token), UserID: s.devUserID, WorkspaceID: s.devWorkspaceID, Scopes: []string{"*"}}, nil
	}
	if s.oauth != nil {
		if principal, apiErr := s.oauth.Authenticate(token); apiErr == nil && principal != nil {
			return principal, nil
		}
	}
	return nil, &apiError{Status: http.StatusUnauthorized, Code: "auth_failed", Message: "gateway authorization failed"}
}

func principalAllows(principal *authPrincipal, required []string) bool {
	if len(required) == 0 {
		return true
	}
	if principal == nil {
		return false
	}
	have := map[string]struct{}{}
	for _, scope := range principal.Scopes {
		if scope == "*" {
			return true
		}
		have[scope] = struct{}{}
	}
	for _, want := range required {
		if _, ok := have[want]; !ok {
			return false
		}
	}
	return true
}

func (s *Server) addAuthChallenge(w http.ResponseWriter, r *http.Request, scope string) {
	w.Header().Set("WWW-Authenticate", mcpauth.Challenge(s.baseURL(r), "/mcp", scope))
}

func (s *Server) applyOriginHeaders(w http.ResponseWriter, r *http.Request) *apiError {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return nil
	}
	w.Header().Set("Vary", "Origin")
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, "+headerProtocolVersion+", "+headerSessionID+", Last-Event-ID")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Expose-Headers", headerProtocolVersion+", "+headerSessionID+", WWW-Authenticate")
	return nil
}

func (s *Server) baseURL(r *http.Request) string {
	return mcpauth.BaseURL(r, s.cfg.PublicBaseURL)
}

func (s *Server) authorizationServers(r *http.Request) []string {
	if s.cfg.OAuthDev.Enabled {
		return []string{strings.TrimRight(firstNonEmpty(s.cfg.OAuthDev.Issuer, s.baseURL(r)), "/")}
	}
	return nil
}

func (s *Server) oauthScopes() []string {
	if s.cfg.OAuthDev.Enabled {
		return append([]string(nil), s.cfg.OAuthDev.Scopes...)
	}
	return []string{"projects:read", "projects:write", "runs:read", "runs:write", "steps:read", "artifacts:read", "reports:read"}
}

func (s *Server) sessionFromRequest(r *http.Request, principal *authPrincipal) (*session, *apiError) {
	sessionID := strings.TrimSpace(r.Header.Get(headerSessionID))
	if sessionID == "" {
		return nil, nil
	}
	hash := ""
	if principal != nil {
		hash = principal.TokenHash
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneExpiredSessionsLocked(time.Now().UTC())
	sess := s.sessions[sessionID]
	if sess == nil || (hash != "" && sess.TokenHash != "" && sess.TokenHash != hash) {
		return nil, &apiError{Status: http.StatusNotFound, Code: "session_not_found", Message: "unknown MCP session"}
	}
	sess.LastSeenAt = time.Now().UTC()
	return sess, nil
}

func (s *Server) ensureSession(existing *session, principal *authPrincipal, protocolVersion string) *session {
	now := time.Now().UTC()
	hash := ""
	if principal != nil {
		hash = principal.TokenHash
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneExpiredSessionsLocked(now)
	if existing != nil {
		sess := s.sessions[existing.ID]
		if sess == nil {
			sess = existing
		}
		if sess.TokenHash == "" {
			sess.TokenHash = hash
		}
		sess.ProtocolVersion = protocolVersion
		sess.LastSeenAt = now
		if sess.CreatedAt.IsZero() {
			sess.CreatedAt = now
		}
		if sess.done == nil {
			sess.done = make(chan struct{})
		}
		s.sessions[sess.ID] = sess
		return sess
	}
	sess := &session{ID: newSessionID(), TokenHash: hash, ProtocolVersion: protocolVersion, CreatedAt: now, LastSeenAt: now, done: make(chan struct{})}
	s.sessions[sess.ID] = sess
	return sess
}

func (s *Server) deleteSession(sessionID string) {
	s.mu.Lock()
	sess := s.sessions[sessionID]
	delete(s.sessions, sessionID)
	s.mu.Unlock()
	if sess != nil {
		sess.closeOnce.Do(func() { close(sess.done) })
	}
}

func (s *Server) pruneExpiredSessionsLocked(now time.Time) {
	for id, sess := range s.sessions {
		if sess == nil || now.Sub(sess.LastSeenAt) <= sessionIdleTTL {
			continue
		}
		delete(s.sessions, id)
		sess.closeOnce.Do(func() { close(sess.done) })
	}
}

func (s *Server) resolveProtocolVersion(r *http.Request, sess *session) (string, *apiError) {
	headerVersion := strings.TrimSpace(r.Header.Get(headerProtocolVersion))
	if headerVersion != "" && !isSupportedProtocolVersion(headerVersion) {
		return "", &apiError{Status: http.StatusBadRequest, Code: "unsupported_protocol_version", Message: "unsupported MCP protocol version"}
	}
	if sess != nil {
		if headerVersion != "" && headerVersion != sess.ProtocolVersion {
			return "", &apiError{Status: http.StatusBadRequest, Code: "protocol_version_mismatch", Message: "MCP-Protocol-Version does not match the negotiated session protocol"}
		}
		return sess.ProtocolVersion, nil
	}
	return protocolVersionOrDefault(headerVersion), nil
}

func (s *Server) applySessionHeaders(w http.ResponseWriter, sess *session, protocolVersion string) {
	if sess != nil {
		w.Header().Set(headerSessionID, sess.ID)
	}
	w.Header().Set(headerProtocolVersion, protocolVersion)
}

func (s *Server) writeRPC(w http.ResponseWriter, response mcpResponse, sess *session, protocolVersion string) {
	s.applySessionHeaders(w, sess, protocolVersion)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

func (s *Server) callRelay(ctx context.Context, profile RelayProfile, method, path string, body []byte) (int, []byte, *apiError) {
	token, err := profile.Token()
	if err != nil {
		return 0, nil, relayUnavailable(profile, err.Error())
	}
	target := strings.TrimRight(profile.URL, "/") + path
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		return 0, nil, &apiError{Status: http.StatusBadRequest, Code: "malformed_request", Message: err.Error()}
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, nil, relayUnavailable(profile, err.Error())
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	data = security.SanitizeRemoteJSON(data)
	if resp.StatusCode >= 400 {
		return resp.StatusCode, data, normalizeRelayError(profile, resp.StatusCode, data)
	}
	return resp.StatusCode, data, nil
}

func normalizeRelayError(profile RelayProfile, status int, body []byte) *apiError {
	var decoded struct {
		Error apiError `json:"error"`
	}
	if json.Unmarshal(body, &decoded) == nil && decoded.Error.Code != "" {
		err := decoded.Error
		err.Status = status
		if err.Blocker != nil {
			err.Blocker = sanitizeMap(err.Blocker)
		}
		if err.Code == "connector_offline" || err.Code == "upstream_timeout" {
			return relayUnavailable(profile, err.Message)
		}
		return &err
	}
	return &apiError{Status: status, Code: "relay_error", Message: string(body)}
}

func relayUnavailable(profile RelayProfile, message string) *apiError {
	if strings.TrimSpace(message) == "" {
		message = "relay is unavailable"
	}
	blocker := map[string]any{
		"type":                      "relay_unavailable",
		"planner_decision_required": true,
		"relay_profile_id":          profile.ID,
		"relay_profile_name":        profile.Name,
		"observed_facts":            []string{message},
	}
	return &apiError{Status: http.StatusServiceUnavailable, Code: "relay_unavailable", Message: message, Blocker: blocker}
}

func apiErrorToolResult(err *apiError) ToolResult {
	if err == nil {
		err = &apiError{Status: http.StatusInternalServerError, Code: "gateway_internal_error", Message: "unknown gateway error"}
	}
	structured := any(map[string]any{"error": map[string]any{"code": err.Code, "message": err.Message}})
	if len(err.Blocker) > 0 {
		blocker := sanitizeMap(err.Blocker)
		blocker["error"] = map[string]any{"code": err.Code, "message": err.Message}
		structured = blocker
	}
	return ToolResult{
		IsError: true,
		Content: []map[string]string{{
			"type": "text",
			"text": security.Redact(err.Message),
		}},
		StructuredContent: structured,
	}
}

func successToolResult(summary string, payload any) ToolResult {
	payload = sanitizeAny(payload)
	result := ToolResult{StructuredContent: payload}
	if summary != "" {
		result.Content = []map[string]string{{"type": "text", "text": summary}}
	}
	if payload != nil {
		if data, err := json.Marshal(payload); err == nil {
			result.Content = append(result.Content, map[string]string{"type": "text", "text": string(data)})
		}
	}
	return result
}

func sanitizeAny(value any) any {
	data, err := json.Marshal(value)
	if err != nil {
		return security.RedactJSON(value)
	}
	data = security.SanitizeRemoteJSON(data)
	var out any
	if json.Unmarshal(data, &out) != nil {
		return security.RedactJSON(value)
	}
	return security.RedactJSON(out)
}

func sanitizeMap(value map[string]any) map[string]any {
	sanitized, _ := sanitizeAny(value).(map[string]any)
	if sanitized == nil {
		return map[string]any{}
	}
	return sanitized
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": code, "message": security.Redact(message)}})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(sanitizeAny(payload))
}

func writePrivateJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func bearerToken(header string) string {
	header = strings.TrimSpace(header)
	if len(header) < len("Bearer ") || !strings.EqualFold(header[:len("Bearer ")], "Bearer ") {
		return ""
	}
	return strings.TrimSpace(header[len("Bearer "):])
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newSessionID() string {
	data := make([]byte, 18)
	if _, err := rand.Read(data); err != nil {
		return fmt.Sprintf("mcp-session-%d", time.Now().UnixNano())
	}
	return "mcp_" + base64.RawURLEncoding.EncodeToString(data)
}

func isSupportedProtocolVersion(version string) bool {
	for _, supported := range supportedProtocolVersions {
		if version == supported {
			return true
		}
	}
	return false
}

func negotiateProtocolVersion(requested, header string) string {
	for _, candidate := range []string{strings.TrimSpace(requested), strings.TrimSpace(header)} {
		if candidate != "" && isSupportedProtocolVersion(candidate) {
			return candidate
		}
	}
	return latestProtocolVersion
}

func protocolVersionOrDefault(value string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return defaultProtocolVersion
}

func rawObjectOrEmpty(raw json.RawMessage) any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var decoded any
	if json.Unmarshal(raw, &decoded) == nil {
		return decoded
	}
	return map[string]any{}
}

func appendSelector(path string, args map[string]any) string {
	values := url.Values{}
	if machineID, _ := args["machine_id"].(string); strings.TrimSpace(machineID) != "" {
		values.Set("machine_id", strings.TrimSpace(machineID))
	}
	if hostLabel, _ := args["host_label"].(string); strings.TrimSpace(hostLabel) != "" {
		values.Set("host_label", strings.TrimSpace(hostLabel))
	}
	if len(values) == 0 {
		return path
	}
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	return path + sep + values.Encode()
}

func jsonBody(value any) ([]byte, *apiError) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, &apiError{Status: http.StatusBadRequest, Code: "malformed_request", Message: err.Error()}
	}
	return data, nil
}
