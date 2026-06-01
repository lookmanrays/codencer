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
		for key, item := range typed {
			lower := strings.ToLower(key)
			switch lower {
			case "repo_root", "allowed_paths", "forbidden_paths", "daemon_url":
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
			default:
				out[key] = sanitizeRemoteValue(item)
			}
		}
		return RedactJSON(out)
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = sanitizeRemoteValue(item)
		}
		return out
	case string:
		return Redact(typed)
	default:
		return value
	}
}
