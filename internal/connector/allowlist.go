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
	case isRunPath(path) && method == http.MethodGet:
		return true
	case isRunPath(path) && method == http.MethodPatch:
		return true
	case isRunStepsPath(path) && method == http.MethodPost:
		return true
	case isRunGatesPath(path) && method == http.MethodGet:
		return true
	case isStepPath(path) && method == http.MethodGet:
		return true
	case isStepActionPath(path, "retry") && method == http.MethodPost:
		return true
	case isStepActionPath(path, "wait") && method == http.MethodPost:
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

func isRunPath(path string) bool {
	return matchesPath(splitPath(path), "api", "v1", "runs", "*")
}

func isRunStepsPath(path string) bool {
	return matchesPath(splitPath(path), "api", "v1", "runs", "*", "steps")
}

func isRunGatesPath(path string) bool {
	return matchesPath(splitPath(path), "api", "v1", "runs", "*", "gates")
}

func isStepPath(path string) bool {
	segments := splitPath(path)
	if len(segments) == 4 && matchesPath(segments, "api", "v1", "steps", "*") {
		return true
	}
	if len(segments) != 5 || !matchesPath(segments, "api", "v1", "steps", "*", "*") {
		return false
	}
	switch segments[4] {
	case "result", "validations", "artifacts", "logs":
		return true
	default:
		return false
	}
}

func isStepActionPath(path, action string) bool {
	return matchesPath(splitPath(path), "api", "v1", "steps", "*", action)
}

func splitPath(path string) []string {
	return strings.Split(strings.Trim(path, "/"), "/")
}

func matchesPath(segments []string, expected ...string) bool {
	if len(segments) != len(expected) {
		return false
	}
	for i := range expected {
		if expected[i] == "*" {
			if segments[i] == "" {
				return false
			}
			continue
		}
		if segments[i] != expected[i] {
			return false
		}
	}
	return true
}
