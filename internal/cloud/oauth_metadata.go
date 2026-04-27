package cloud

import (
	"net/http"

	"agent-bridge/internal/mcpauth"
)

const (
	cloudOAuthMetadataPrefix = "/.well-known/oauth-protected-resource"
	cloudMCPResourcePath     = "/api/cloud/v1/mcp"
)

func (s *Server) handleOAuthProtectedResource(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	resourcePath := mcpauth.ResourcePathFromWellKnownRequest(r.URL.Path, cloudOAuthMetadataPrefix, cloudMCPResourcePath)
	writeJSON(w, http.StatusOK, mcpauth.Metadata(s.publicBaseURL(r), resourcePath, s.protectedResourceConfig()))
}

func (s *Server) addTokenAuthChallenge(w http.ResponseWriter, r *http.Request, scope string) {
	if s == nil || s.cfg == nil {
		return
	}
	resourcePath := cloudMCPResourcePath
	if r != nil && r.URL != nil && r.URL.Path != "" {
		resourcePath = r.URL.Path
	}
	w.Header().Set("WWW-Authenticate", mcpauth.Challenge(s.publicBaseURL(r), resourcePath, scope))
}

func (s *Server) protectedResourceConfig() mcpauth.ProtectedResourceConfig {
	return mcpauth.ProtectedResourceConfig{
		AuthorizationServers:   s.cfg.OAuthAuthorizationServers,
		ScopesSupported:        s.cfg.OAuthScopesSupported,
		ResourceDocumentation:  s.cfg.OAuthResourceDocumentation,
		ResourceName:           "Codencer Cloud MCP",
		BearerMethodsSupported: []string{"header"},
	}
}

func (s *Server) publicBaseURL(r *http.Request) string {
	if s != nil && s.cfg != nil {
		return mcpauth.BaseURL(r, s.cfg.PublicBaseURL)
	}
	return mcpauth.BaseURL(r, "")
}

func (s *Server) authMode() string {
	if s == nil || s.cfg == nil || len(s.cfg.OAuthAuthorizationServers) == 0 {
		return "hashed_api_bearer_tokens"
	}
	return "hashed_api_bearer_tokens+oauth_protected_resource"
}
