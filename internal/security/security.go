package security

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"
)

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9._~+/=-]+`),
	regexp.MustCompile(`(?i)("?(?:token|planner_token|enrollment_secret|private_key|secret)"?\s*[:=]\s*"?)[^"\s,}]+`),
	regexp.MustCompile(`(?i)(Authorization:\s*Bearer\s+)[A-Za-z0-9._~+/=-]+`),
}

var localPathPatterns = []*regexp.Regexp{
	regexp.MustCompile(`/Users/[^\s"',)}\]]+`),
	regexp.MustCompile(`/home/[^\s"',)}\]]+`),
	regexp.MustCompile(`/tmp/[^\s"',)}\]]+`),
	regexp.MustCompile(`/private/tmp/[^\s"',)}\]]+`),
	regexp.MustCompile(`/var/folders/[^\s"',)}\]]+`),
	regexp.MustCompile(`/var/tmp/[^\s"',)}\]]+`),
	regexp.MustCompile(`/workspace/[^\s"',)}\]]+`),
}

func Redact(value string) string {
	out := value
	for _, pattern := range secretPatterns {
		out = pattern.ReplaceAllString(out, `${1}<redacted>`)
	}
	return out
}

func RedactJSON(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, item := range typed {
			lower := strings.ToLower(key)
			if isSecretKey(lower) {
				out[key] = "<redacted>"
				continue
			}
			out[key] = RedactJSON(item)
		}
		return out
	case map[string]string:
		out := map[string]string{}
		for key, item := range typed {
			if isSecretKey(strings.ToLower(key)) {
				out[key] = "<redacted>"
				continue
			}
			out[key] = Redact(item)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = RedactJSON(item)
		}
		return out
	case string:
		return Redact(typed)
	default:
		return value
	}
}

func isSecretKey(lower string) bool {
	switch lower {
	case "token", "planner_token", "enrollment_secret", "private_key", "secret", "authorization":
		return true
	default:
		return strings.HasSuffix(lower, "_token") || strings.HasSuffix(lower, "_secret") || strings.HasSuffix(lower, "_private_key")
	}
}

func ContainsObviousSecret(value string) bool {
	lower := strings.ToLower(value)
	return strings.Contains(lower, "bearer ") ||
		strings.Contains(lower, "planner-token") ||
		strings.Contains(lower, "private_key") ||
		strings.Contains(lower, "enrollment_secret")
}

func SafePathLabel(path string) (label, hash string) {
	cleaned := filepath.Clean(strings.TrimSpace(path))
	if cleaned == "." || cleaned == "" {
		return "", ""
	}
	sum := sha256.Sum256([]byte(cleaned))
	return filepath.Base(cleaned), hex.EncodeToString(sum[:8])
}

func SanitizeRemoteJSON(data []byte) []byte {
	if len(data) == 0 || !json.Valid(data) {
		return data
	}
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		return data
	}
	sanitized := sanitizeRemoteValue(payload)
	out, err := json.Marshal(sanitized)
	if err != nil {
		return data
	}
	return out
}

func sanitizeRemoteValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := map[string]any{}
		hadPathField := false
		for key, item := range typed {
			lower := strings.ToLower(key)
			switch lower {
			case "repo_root":
				if lower == "repo_root" {
					if path, ok := item.(string); ok {
						label, hash := SafePathLabel(path)
						if label != "" {
							out["repo_label"] = label
						}
						if hash != "" {
							out["repo_root_hash"] = hash
						}
					}
				}
				continue
			case "report_path", "manifest_path", "project_config_path", "logs_ref", "normalized_task_ref", "original_input_ref", "allowed_paths", "forbidden_paths", "daemon_url":
				continue
			case "path":
				if str, ok := item.(string); ok && containsLocalPath(str) {
					hadPathField = true
					continue
				}
				out[key] = sanitizeRemoteValue(item)
			default:
				out[key] = sanitizeRemoteValue(item)
			}
		}
		if hadPathField {
			if id, ok := out["id"]; ok {
				if _, exists := out["artifact_id"]; !exists {
					out["artifact_id"] = id
				}
			}
		}
		normalizeReportState(out)
		return RedactJSON(out)
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = sanitizeRemoteValue(item)
		}
		return out
	case string:
		return sanitizeRemoteString(typed)
	default:
		return value
	}
}

func sanitizeRemoteString(value string) string {
	out := Redact(value)
	for _, pattern := range localPathPatterns {
		out = pattern.ReplaceAllString(out, "<redacted-local-path>")
	}
	out = strings.ReplaceAll(out, "CODENCER_HOME", "<redacted-local-env>")
	out = strings.ReplaceAll(out, ".codencer-live-test", "<redacted-local-home>")
	return out
}

func containsLocalPath(value string) bool {
	if strings.Contains(value, "CODENCER_HOME") || strings.Contains(value, ".codencer-live-test") {
		return true
	}
	for _, pattern := range localPathPatterns {
		if pattern.MatchString(value) {
			return true
		}
	}
	return false
}

func normalizeReportState(out map[string]any) {
	status, _ := out["status"].(string)
	if !isTerminalReportStatus(status) {
		return
	}
	run, _ := out["run"].(map[string]any)
	if run == nil {
		return
	}
	if state, _ := run["state"].(string); state != "" && state != status {
		run["state"] = status
	}
}

func isTerminalReportStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "failed", "blocked", "cancelled", "canceled", "timeout", "timed_out":
		return true
	default:
		return false
	}
}
