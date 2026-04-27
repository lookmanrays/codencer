package mcpauth

import (
	"fmt"
	"net/http"
	"strings"
)

// ProtectedResourceConfig describes the OAuth protected resource metadata
// Codencer exposes for product-facing remote MCP integrations. Codencer remains
// a resource server here; token issuance belongs to the configured issuer or
// front door.
type ProtectedResourceConfig struct {
	AuthorizationServers   []string
	ScopesSupported        []string
	ResourceDocumentation  string
	ResourceName           string
	BearerMethodsSupported []string
}

func BaseURL(r *http.Request, configured string) string {
	if configured = strings.TrimRight(strings.TrimSpace(configured), "/"); configured != "" {
		return configured
	}
	if r == nil {
		return ""
	}
	scheme := forwardedValue(r.Header.Get("Forwarded"), "proto")
	if scheme == "" {
		scheme = firstHeaderValue(r.Header.Get("X-Forwarded-Proto"))
	}
	if scheme == "" {
		scheme = "http"
		if r.TLS != nil {
			scheme = "https"
		}
	}
	host := forwardedValue(r.Header.Get("Forwarded"), "host")
	if host == "" {
		host = firstHeaderValue(r.Header.Get("X-Forwarded-Host"))
	}
	if host == "" {
		host = r.Host
	}
	return fmt.Sprintf("%s://%s", strings.TrimSpace(scheme), strings.TrimSpace(host))
}

func Metadata(baseURL, resourcePath string, cfg ProtectedResourceConfig) map[string]any {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	resourcePath = normalizeResourcePath(resourcePath)
	payload := map[string]any{
		"resource":                 baseURL + resourcePath,
		"bearer_methods_supported": methodsOrDefault(cfg.BearerMethodsSupported),
	}
	if cfg.ResourceName != "" {
		payload["resource_name"] = strings.TrimSpace(cfg.ResourceName)
	}
	if servers := cleanList(cfg.AuthorizationServers); len(servers) > 0 {
		payload["authorization_servers"] = servers
	}
	if scopes := cleanList(cfg.ScopesSupported); len(scopes) > 0 {
		payload["scopes_supported"] = scopes
	}
	if docs := strings.TrimSpace(cfg.ResourceDocumentation); docs != "" {
		payload["resource_documentation"] = docs
	}
	return payload
}

func Challenge(baseURL, resourcePath string, scope string) string {
	metadataURL := strings.TrimRight(strings.TrimSpace(baseURL), "/") +
		"/.well-known/oauth-protected-resource" + normalizeResourcePath(resourcePath)
	parts := []string{`resource_metadata="` + escapeChallenge(metadataURL) + `"`}
	if scope = strings.TrimSpace(scope); scope != "" {
		parts = append(parts, `scope="`+escapeChallenge(scope)+`"`)
	}
	return "Bearer " + strings.Join(parts, ", ")
}

func ResourcePathFromWellKnownRequest(requestPath, prefix, fallback string) string {
	resourcePath := strings.TrimPrefix(requestPath, prefix)
	if resourcePath = strings.TrimSpace(resourcePath); resourcePath != "" && resourcePath != "/" {
		return normalizeResourcePath(resourcePath)
	}
	return normalizeResourcePath(fallback)
}

func cleanList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func methodsOrDefault(values []string) []string {
	if methods := cleanList(values); len(methods) > 0 {
		return methods
	}
	return []string{"header"}
}

func normalizeResourcePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

func firstHeaderValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if idx := strings.Index(value, ","); idx >= 0 {
		value = value[:idx]
	}
	return strings.TrimSpace(value)
}

func forwardedValue(header, key string) string {
	header = firstHeaderValue(header)
	key = strings.ToLower(strings.TrimSpace(key))
	if header == "" || key == "" {
		return ""
	}
	for _, part := range strings.Split(header, ";") {
		name, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok || strings.ToLower(strings.TrimSpace(name)) != key {
			continue
		}
		return strings.Trim(strings.TrimSpace(value), `"`)
	}
	return ""
}

func escapeChallenge(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, `"`, `\"`)
}
