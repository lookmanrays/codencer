package relay

import (
	"net/http"
	"strings"

	"agent-bridge/internal/mcpauth"
)

const (
	relayOAuthMetadataPrefix = "/.well-known/oauth-protected-resource"
	relayMCPResourcePath     = "/mcp"
)

func (s *Server) handleOAuthProtectedResource(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	resourcePath := mcpauth.ResourcePathFromWellKnownRequest(r.URL.Path, relayOAuthMetadataPrefix, relayMCPResourcePath)
	writeJSON(w, http.StatusOK, mcpauth.Metadata(s.relayBaseURL(r), resourcePath, s.relayProtectedResourceConfig()))
}

func (s *Server) addPlannerAuthChallenge(w http.ResponseWriter, r *http.Request, scope string) {
	if s == nil || s.cfg == nil {
		return
	}
	resourcePath := relayMCPResourcePath
	if r != nil && r.URL != nil && r.URL.Path != "" {
		resourcePath = r.URL.Path
	}
	w.Header().Set("WWW-Authenticate", mcpauth.Challenge(s.relayBaseURL(r), resourcePath, scope))
}

func (s *Server) relayProtectedResourceConfig() mcpauth.ProtectedResourceConfig {
	return mcpauth.ProtectedResourceConfig{
		AuthorizationServers:   s.cfg.OAuthAuthorizationServers,
		ScopesSupported:        s.cfg.OAuthScopesSupported,
		ResourceDocumentation:  s.cfg.OAuthResourceDocumentation,
		ResourceName:           "Codencer Relay MCP",
		BearerMethodsSupported: []string{"header"},
	}
}

func (s *Server) plannerAuthMode() string {
	if s == nil || s.cfg == nil || len(s.cfg.OAuthAuthorizationServers) == 0 {
		return "static_bearer_tokens"
	}
	return "static_bearer_tokens+oauth_protected_resource"
}

func (s *Server) relayBaseURL(r *http.Request) string {
	if s != nil && s.cfg != nil {
		return mcpauth.BaseURL(r, s.cfg.PublicBaseURL)
	}
	return mcpauth.BaseURL(r, "")
}

func (s *Server) websocketURL(r *http.Request, path string) string {
	baseURL := s.relayBaseURL(r)
	if stripped := strings.TrimPrefix(baseURL, "https://"); stripped != baseURL {
		return "wss://" + stripped + path
	}
	return "ws://" + strings.TrimPrefix(baseURL, "http://") + path
}
