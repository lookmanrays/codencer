package connector

import (
	"net/http"
	"strings"
)

func AllowedLocalProxy(method, path string) bool {
	switch {
	case method == http.MethodGet && path == "/api/v1/instance":
		return true
	case method == http.MethodGet && path == "/api/v1/runs":
		return true
	case method == http.MethodPost && path == "/api/v1/runs":
		return true
	case strings.HasPrefix(path, "/api/v1/runs/") && method == http.MethodGet:
		return true
	case strings.HasPrefix(path, "/api/v1/runs/") && method == http.MethodPatch:
		return true
	case strings.HasPrefix(path, "/api/v1/runs/") && strings.HasSuffix(path, "/steps") && method == http.MethodPost:
		return true
	case strings.HasPrefix(path, "/api/v1/runs/") && strings.HasSuffix(path, "/gates") && method == http.MethodGet:
		return true
	case strings.HasPrefix(path, "/api/v1/steps/") && method == http.MethodGet:
		return true
	case strings.HasPrefix(path, "/api/v1/steps/") && strings.HasSuffix(path, "/retry") && method == http.MethodPost:
		return true
	case strings.HasPrefix(path, "/api/v1/steps/") && strings.HasSuffix(path, "/wait") && method == http.MethodPost:
		return true
	case strings.HasPrefix(path, "/api/v1/artifacts/") && method == http.MethodGet:
		return true
	case strings.HasPrefix(path, "/api/v1/artifacts/") && strings.HasSuffix(path, "/content") && method == http.MethodGet:
		return true
	case strings.HasPrefix(path, "/api/v1/gates/") && method == http.MethodGet:
		return true
	case strings.HasPrefix(path, "/api/v1/gates/") && method == http.MethodPost:
		return true
	default:
		return false
	}
}
