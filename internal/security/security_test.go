package security

import (
	"strings"
	"testing"
)

func TestSanitizeRemoteJSONRemovesLocalPathFields(t *testing.T) {
	in := []byte(`{"project":{"repo_root":"/home/me/secret/repo","project_config_path":"/home/me/secret/repo/.codencer/project.json","allowed_paths":["."],"forbidden_paths":[".env"],"daemon_url":"http://127.0.0.1:8085","id":"p"}}`)
	out := string(SanitizeRemoteJSON(in))
	for _, forbidden := range []string{"/home/me", "project_config_path", "allowed_paths", "forbidden_paths", "daemon_url"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("sanitized payload leaked %q: %s", forbidden, out)
		}
	}
	if !strings.Contains(out, "repo_label") || !strings.Contains(out, "repo_root_hash") {
		t.Fatalf("expected safe repo label/hash: %s", out)
	}
}

func TestSanitizeRemoteJSONRemovesNestedRunReportPaths(t *testing.T) {
	in := []byte(`{
		"ok": true,
		"status": "completed",
		"report_path": "/home/me/.codencer-live-test/artifacts/run-plans/run-1.json",
		"project": {
			"id": "codencer",
			"repo_root": "/home/me/Projects/codencer",
			"daemon_url": "http://127.0.0.1:8085"
		},
		"run": {
			"id": "run-1",
			"state": "running"
		},
		"tasks": [{
			"task_id": "one",
			"status": "completed",
			"evidence": {
				"logs_ref": "/var/folders/x/codencer/stdout.log",
				"artifacts": [{
					"id": "artifact-1",
					"name": "stdout.log",
					"type": "stdout",
					"mime_type": "text/plain",
					"size": 12,
					"hash": "abc",
					"path": "/tmp/codencer/artifacts/stdout.log"
				}],
				"result": {
					"summary": "done from /home/me/private/repo",
					"artifacts": {
						"normalized_task_ref": "/home/me/.codencer-live-test/runtime/normalized-task.json",
						"original_input_ref": "/home/me/.codencer-live-test/runtime/original-input.txt"
					}
				}
			}
		}]
	}`)
	out := string(SanitizeRemoteJSON(in))
	for _, forbidden := range []string{
		"/home/me",
		"/tmp/",
		"/var/folders/",
		".codencer-live-test",
		"report_path",
		"daemon_url",
		"logs_ref",
		"normalized_task_ref",
		"original_input_ref",
		`"path"`,
		`"state":"running"`,
	} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("sanitized payload leaked %q: %s", forbidden, out)
		}
	}
	for _, want := range []string{`"artifact_id":"artifact-1"`, `"name":"stdout.log"`, `"mime_type":"text/plain"`, `"size":12`, `"hash":"abc"`, `"repo_root_hash"`, `"repo_label"`, `"state":"completed"`, "redacted-local-path"} {
		if !strings.Contains(out, want) {
			t.Fatalf("sanitized payload missing %q: %s", want, out)
		}
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
