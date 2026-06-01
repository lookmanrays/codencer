package security

import (
	"strings"
	"testing"
)

func TestSanitizeRemoteJSONRemovesLocalPathFields(t *testing.T) {
	in := []byte(`{"project":{"repo_root":"/Users/me/secret/repo","allowed_paths":["."],"forbidden_paths":[".env"],"daemon_url":"http://127.0.0.1:8085","id":"p"}}`)
	out := string(SanitizeRemoteJSON(in))
	for _, forbidden := range []string{"/Users/me", "allowed_paths", "forbidden_paths", "daemon_url"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("sanitized payload leaked %q: %s", forbidden, out)
		}
	}
	if !strings.Contains(out, "repo_label") || !strings.Contains(out, "repo_root_hash") {
		t.Fatalf("expected safe repo label/hash: %s", out)
	}
}

func TestRedactJSONKeepsTokenEnvButRedactsToken(t *testing.T) {
	out := RedactJSON(map[string]any{
		"token_env":      "CODENCER_TOKEN",
		"token_included": true,
		"token":          "secret",
	}).(map[string]any)
	if out["token_env"] != "CODENCER_TOKEN" || out["token_included"] != true {
		t.Fatalf("non-secret token metadata was redacted: %+v", out)
	}
	if out["token"] != "<redacted>" {
		t.Fatalf("token was not redacted: %+v", out)
	}
}
