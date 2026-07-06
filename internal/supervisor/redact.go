package supervisor

import (
	"regexp"
	"strings"
)

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9._~+/=-]+`),
	regexp.MustCompile(`(?i)("?(?:token|planner_token|private_key|enrollment_secret)"?\s*[:=]\s*"?)[^"\s,}]+`),
	regexp.MustCompile(`(?i)((?:TOKEN|SECRET|PRIVATE_KEY)=)[^\s]+`),
}

func redact(value string) string {
	out := value
	for _, pattern := range secretPatterns {
		out = pattern.ReplaceAllString(out, `${1}<redacted>`)
	}
	return strings.TrimSpace(out)
}
