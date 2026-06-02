package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionPathsAndDoctorJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODENCER_HOME", home)

	stdout, stderr, err := runCLI("version", "--json")
	if err != nil {
		t.Fatalf("version failed: %v stderr=%s", err, stderr)
	}
	assertJSON(t, stdout)
	if !strings.Contains(stdout, `"version"`) {
		t.Fatalf("version JSON missing version: %s", stdout)
	}

	stdout, stderr, err = runCLI("paths", "--json")
	if err != nil {
		t.Fatalf("paths failed: %v stderr=%s", err, stderr)
	}
	var paths map[string]any
	decodeJSON(t, stdout, &paths)
	if paths["home"] != home {
		t.Fatalf("paths home = %v want %s", paths["home"], home)
	}

	stdout, _, err = runCLI("doctor", "toolchain", "--json")
	if err != nil {
		var ee exitError
		if !errors.As(err, &ee) {
			t.Fatalf("unexpected doctor error: %v", err)
		}
	}
	assertJSON(t, stdout)
	if !strings.Contains(stdout, `"checks"`) {
		t.Fatalf("doctor output missing checks: %s", stdout)
	}
}

func TestExecutionCommandsJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/runs":
			writeHTTPJSON(t, w, http.StatusCreated, map[string]any{"id": "run-1", "project_id": "proj", "state": "running"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/runs":
			writeHTTPJSON(t, w, http.StatusOK, []map[string]any{{"id": "run-1", "project_id": "proj", "state": "completed"}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/runs/run-1/steps":
			writeHTTPJSON(t, w, http.StatusAccepted, map[string]any{"id": "step-1", "state": "running", "adapter": "fake-success"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-1":
			writeHTTPJSON(t, w, http.StatusOK, map[string]any{"id": "step-1", "state": "completed", "adapter": "fake-success"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-1/result":
			writeHTTPJSON(t, w, http.StatusOK, map[string]any{"step_id": "step-1", "state": "completed", "summary": "done"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-1/artifacts":
			writeHTTPJSON(t, w, http.StatusOK, []map[string]any{})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-1/validations":
			writeHTTPJSON(t, w, http.StatusOK, map[string]any{})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-1/logs":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("logs"))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	home := t.TempDir()
	t.Setenv("CODENCER_HOME", home)
	repo := makeTestRepo(t)
	if _, _, err := runCLI("init", "--json"); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	if stdout, stderr, err := runCLI("project", "init", "--id", "proj", "--repo", repo, "--adapter", "fake", "--profile", "fake-success", "--daemon-url", server.URL, "--json"); err != nil {
		t.Fatalf("project init failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}

	stdout, stderr, err := runCLI("run", "start", "--project", "proj", "--json")
	if err != nil {
		t.Fatalf("run start failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"id": "run-1"`) {
		t.Fatalf("run start output wrong: %s", stdout)
	}

	stdout, stderr, err = runCLI("run", "list", "--project", "proj", "--json")
	if err != nil {
		t.Fatalf("run list failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"runs"`) {
		t.Fatalf("run list output wrong: %s", stdout)
	}

	stdout, stderr, err = runCLI("submit", "--project", "proj", "--run", "run-1", "--goal", "do it", "--wait", "--json")
	if err != nil {
		t.Fatalf("submit failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"status": "completed"`) || !strings.Contains(stdout, `"evidence"`) {
		t.Fatalf("submit output wrong: %s", stdout)
	}

	manifest := filepath.Join(t.TempDir(), "manifest.yaml")
	if err := os.WriteFile(manifest, []byte(`version: codencer.io/v1alpha1
kind: RunManifest
tasks:
  - id: one
    goal: do it from a manifest
`), 0644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err = runCLI("run-plan", manifest, "--project", "proj", "--wait", "--json")
	if err != nil {
		t.Fatalf("run-plan failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"report_path"`) || !strings.Contains(stdout, `"task_id": "one"`) {
		t.Fatalf("run-plan output wrong: %s", stdout)
	}
}

func TestProfileAndDaemonNotRunningJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODENCER_HOME", home)
	repo := makeTestRepo(t)
	if _, _, err := runCLI("init", "--json"); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	if stdout, stderr, err := runCLI("project", "init", "--id", "proj", "--repo", repo, "--adapter", "fake", "--profile", "fake-success", "--daemon-url", "http://127.0.0.1:1", "--json"); err != nil {
		t.Fatalf("project init failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}

	stdout, stderr, err := runCLI("profile", "list", "--json")
	if err != nil {
		t.Fatalf("profile list failed: %v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, `"fake-success"`) {
		t.Fatalf("profile list missing fake profile: %s", stdout)
	}

	stdout, _, err = runCLI("submit", "--project", "proj", "--goal", "do it", "--wait", "--json")
	if err == nil {
		t.Fatal("expected submit to fail when daemon is not running")
	}
	var ee exitError
	if !errors.As(err, &ee) || ee.code != 23 {
		t.Fatalf("expected exit 23, got %v", err)
	}
	if !strings.Contains(stdout, `"type": "daemon_not_running"`) {
		t.Fatalf("expected daemon_not_running JSON, got %s", stdout)
	}
}

func TestServiceWatchdogAndRecoverJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/health":
			writeHTTPJSON(t, w, http.StatusOK, map[string]string{"status": "ok"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/runs":
			writeHTTPJSON(t, w, http.StatusOK, []map[string]any{})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/runs/run-terminal":
			writeHTTPJSON(t, w, http.StatusOK, map[string]any{"id": "run-terminal", "project_id": "proj", "state": "completed"})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	home := t.TempDir()
	t.Setenv("CODENCER_HOME", home)
	repo := makeTestRepo(t)
	bin := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(bin, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"orchestratord", "codencer-relayd", "codencer-connectord"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := runCLI("init", "--json"); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	if stdout, stderr, err := runCLI("project", "init", "--id", "proj", "--repo", repo, "--adapter", "fake", "--profile", "fake-success", "--daemon-url", server.URL, "--json"); err != nil {
		t.Fatalf("project init failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}

	stdout, stderr, err := runCLI("service", "status", "--all", "--json", "--manager", "manual", "--bin-dir", bin)
	if err != nil {
		t.Fatalf("service status failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"services"`) || !strings.Contains(stdout, `"daemon"`) {
		t.Fatalf("service status output wrong: %s", stdout)
	}

	stdout, stderr, err = runCLI("service", "install", "daemon", "--dry-run", "--json", "--manager", "launchd", "--bin-dir", bin)
	if err != nil {
		t.Fatalf("service install dry-run failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"dry_run": true`) || !strings.Contains(stdout, "io.codencer.daemon") {
		t.Fatalf("service dry-run output wrong: %s", stdout)
	}

	stdout, stderr, err = runCLI("service", "render", "daemon", "--format", "systemd", "--bin-dir", bin)
	if err != nil {
		t.Fatalf("service render failed: %v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, "ExecStart=") {
		t.Fatalf("systemd render missing ExecStart: %s", stdout)
	}

	stdout, stderr, err = runCLI("watchdog", "once", "--json", "--manager", "manual", "--bin-dir", bin)
	if err != nil {
		t.Fatalf("watchdog failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"checks"`) || strings.Contains(stdout, "suggested_next_action") {
		t.Fatalf("watchdog output wrong: %s", stdout)
	}

	lockDir := filepath.Join(repo, ".codencer", "workspace")
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lockDir, ".codencer.lock"), []byte("run-terminal"), 0644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err = runCLI("recover", "locks", "--dry-run", "--json", "--manager", "manual", "--bin-dir", bin)
	if err != nil {
		t.Fatalf("recover locks failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"remove_stale_lock"`) || !strings.Contains(stdout, `"dry_run": true`) {
		t.Fatalf("recover output wrong: %s", stdout)
	}
}

func TestLiveAndReadinessJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODENCER_HOME", home)
	for _, name := range []string{
		"CODENCER_LIVE_ALL",
		"CODENCER_LIVE_CODEX",
		"CODENCER_LIVE_CLAUDE",
		"CODENCER_LIVE_RELAY_MCP",
		"CODENCER_LIVE_CODEX_MCP",
		"CODENCER_LIVE_CLAUDE_MCP",
		"CODENCER_LIVE_WSL",
		"CODENCER_LIVE_SERVICE_RESTART",
		"CODENCER_LIVE_CODEX_SMOKE",
		"CODENCER_LIVE_CLAUDE_SMOKE",
	} {
		t.Setenv(name, "0")
	}
	repo := makeTestRepo(t)
	if _, _, err := runCLI("init", "--json"); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	stdout, stderr, err := runCLI("live", "matrix", "--profile", "local", "--repo", repo, "--json")
	if err != nil {
		t.Fatalf("live matrix failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"report_path"`) || !strings.Contains(stdout, `"codex_live_execution"`) || strings.Contains(stdout, "suggested_next_action") {
		t.Fatalf("live matrix output wrong: %s", stdout)
	}

	stdout, stderr, err = runCLI("live", "codex", "--repo", repo, "--json")
	if err != nil {
		t.Fatalf("guarded live codex should skip without env: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"skipped"`) {
		t.Fatalf("expected skipped codex report: %s", stdout)
	}

	stdout, stderr, err = runCLI("live", "codex-mcp", "--endpoint", "https://relay.example.com/mcp", "--repo", repo, "--json")
	if err != nil {
		t.Fatalf("guarded codex-mcp should skip without env: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `config_generated`) || !strings.Contains(stdout, `"skipped"`) {
		t.Fatalf("expected MCP config and skipped proof: %s", stdout)
	}

	stdout, stderr, err = runCLI("readiness", "--repo", repo, "--json")
	if err != nil {
		t.Fatalf("readiness failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"verdict"`) || !strings.Contains(stdout, `"report_path"`) {
		t.Fatalf("readiness output wrong: %s", stdout)
	}

	stdout, stderr, err = runCLI("live", "reports", "--json")
	if err != nil {
		t.Fatalf("live reports failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"reports"`) {
		t.Fatalf("live reports output wrong: %s", stdout)
	}

	stdout, stderr, err = runCLI("readiness", "reports", "--json")
	if err != nil {
		t.Fatalf("readiness reports failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"reports"`) {
		t.Fatalf("readiness reports output wrong: %s", stdout)
	}
}

func TestSetupAcceptProofCommandsJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODENCER_HOME", home)
	repo := makeTestRepo(t)
	if _, _, err := runCLI("init", "--json"); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	stdout, stderr, err := runCLI("setup", "local", "--repo", repo, "--json")
	if err != nil {
		t.Fatalf("setup local failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"mode": "local"`) || strings.Contains(stdout, "suggested_next_action") {
		t.Fatalf("setup local output wrong: %s", stdout)
	}

	stdout, stderr, err = runCLI("setup", "relay", "--base-url", "https://relay.example.com", "--json")
	if err != nil {
		t.Fatalf("setup relay guidance should not fail: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"configured": false`) || !strings.Contains(stdout, "generate-planner-token") {
		t.Fatalf("setup relay guidance output wrong: %s", stdout)
	}

	stdout, stderr, err = runCLI("setup", "mcp", "--client", "chatgpt", "--endpoint", "https://relay.example.com/mcp", "--json")
	if err != nil {
		t.Fatalf("setup mcp failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"client": "chatgpt"`) || !strings.Contains(stdout, "oauth-dev-or-front-door") {
		t.Fatalf("setup mcp output wrong: %s", stdout)
	}

	stdout, stderr, err = runCLI("setup", "relay", "--base-url", "https://relay.example.com", "--generate-planner-token", "--enable-chatgpt-oauth-dev", "--oauth-client-secret", "client-secret", "--chatgpt-dev-noauth", "--json")
	if err != nil {
		t.Fatalf("setup relay OAuth failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"chatgpt_oauth_dev"`) || strings.Contains(stdout, "client-secret") {
		t.Fatalf("setup relay OAuth output wrong: %s", stdout)
	}

	stdout, stderr, err = runCLI("activation", "package", "--relay", "https://relay.example.com", "--project", "codencer", "--token", "literal-token", "--json")
	if err != nil {
		t.Fatalf("activation package failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"mode": "package"`) || !strings.Contains(stdout, `"package_path"`) || strings.Contains(stdout, "literal-token") {
		t.Fatalf("activation package output wrong: %s", stdout)
	}

	stdout, stderr, err = runCLI("activation", "chatgpt", "--relay", "https://relay.example.com", "--project", "codencer", "--auth", "oauth", "--json")
	if err != nil {
		t.Fatalf("activation chatgpt failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"client": "chatgpt"`) || !strings.Contains(stdout, "pending_manual_product_proof") {
		t.Fatalf("activation chatgpt output wrong: %s", stdout)
	}

	stdout, stderr, err = runCLI("activation", "codex", "--relay", "https://relay.example.com", "--token-env", "CODENCER_MCP_TOKEN", "--json")
	if err != nil {
		t.Fatalf("activation codex failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "codex mcp add") {
		t.Fatalf("activation codex output wrong: %s", stdout)
	}

	stdout, stderr, err = runCLI("activation", "claude-code", "--relay", "https://relay.example.com", "--token-env", "CODENCER_MCP_TOKEN", "--json")
	if err != nil {
		t.Fatalf("activation claude-code failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "claude mcp add") {
		t.Fatalf("activation claude-code output wrong: %s", stdout)
	}

	stdout, stderr, err = runCLI("accept", "reports", "--json")
	if err != nil {
		t.Fatalf("accept reports failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"reports"`) {
		t.Fatalf("accept reports output wrong: %s", stdout)
	}

	stdout, stderr, err = runCLI("proof", "bundle", "--repo", repo, "--json")
	if err != nil {
		t.Fatalf("proof bundle failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"proof_path"`) || strings.Contains(stdout, "planner-token") {
		t.Fatalf("proof bundle output wrong: %s", stdout)
	}
}

func TestInitAndProjectLifecycleJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODENCER_HOME", home)
	repo := makeTestRepo(t)

	stdout, stderr, err := runCLI("init", "--json")
	if err != nil {
		t.Fatalf("init failed: %v stderr=%s", err, stderr)
	}
	assertJSON(t, stdout)
	if _, err := os.Stat(filepath.Join(home, "projects.json")); err != nil {
		t.Fatalf("projects.json missing: %v", err)
	}

	stdout, stderr, err = runCLI("project", "init", "--id", "codencer", "--repo", repo, "--adapter", "codex", "--json")
	if err != nil {
		t.Fatalf("project init failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	var initPayload map[string]any
	decodeJSON(t, stdout, &initPayload)
	if initPayload["current_project_id"] != "codencer" {
		t.Fatalf("current project not set: %s", stdout)
	}

	stdout, stderr, err = runCLI("project", "list", "--json")
	if err != nil {
		t.Fatalf("project list failed: %v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, `"id": "codencer"`) {
		t.Fatalf("project list missing project: %s", stdout)
	}

	stdout, stderr, err = runCLI("project", "get", "codencer", "--json")
	if err != nil {
		t.Fatalf("project get failed: %v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, `"repo_root": "`+repo+`"`) {
		t.Fatalf("project get missing repo root: %s", stdout)
	}

	stdout, stderr, err = runCLI("project", "share", "codencer", "--relay-instance-id", "inst-1", "--daemon-url", "http://127.0.0.1:18085", "--json")
	if err != nil {
		t.Fatalf("project share failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"shared_to_relay": true`) || !strings.Contains(stdout, `"relay_instance_id": "inst-1"`) {
		t.Fatalf("project share output wrong: %s", stdout)
	}

	stdout, stderr, err = runCLI("project", "unshare", "codencer", "--json")
	if err != nil {
		t.Fatalf("project unshare failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"shared_to_relay": false`) {
		t.Fatalf("project unshare output wrong: %s", stdout)
	}

	stdout, stderr, err = runCLI("project", "use", "codencer", "--json")
	if err != nil {
		t.Fatalf("project use failed: %v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, `"current_project_id": "codencer"`) {
		t.Fatalf("project use output wrong: %s", stdout)
	}

	stdout, stderr, err = runCLI("project", "status", "codencer", "--json")
	if err != nil {
		t.Fatalf("project status failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	assertJSON(t, stdout)
	if !strings.Contains(stdout, `"project"`) || !strings.Contains(stdout, `"daemon"`) {
		t.Fatalf("project status missing fields: %s", stdout)
	}

	stdout, stderr, err = runCLI("project", "remove", "codencer", "--json")
	if err != nil {
		t.Fatalf("project remove failed: %v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, `"removed_project"`) {
		t.Fatalf("project remove output wrong: %s", stdout)
	}
}

func TestProjectInitNonGitWarningAndMissingProjectJSONError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODENCER_HOME", home)
	if _, _, err := runCLI("init", "--json"); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	repo := t.TempDir()

	stdout, stderr, err := runCLI("project", "init", "--id", "plain", "--repo", repo, "--adapter", "codex", "--json")
	if err != nil {
		t.Fatalf("project init non-git failed: %v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, "not a git repository") {
		t.Fatalf("expected non-git warning, got %s", stdout)
	}

	stdout, _, err = runCLI("project", "get", "missing", "--json")
	if err == nil {
		t.Fatal("expected missing project to fail")
	}
	if !strings.Contains(stdout, `"error"`) {
		t.Fatalf("expected JSON error, got %s", stdout)
	}
}

func runCLI(args ...string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	err := run(args, &stdout, &stderr)
	return stdout.String(), stderr.String(), err
}

func makeTestRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	return repo
}

func assertJSON(t *testing.T, raw string) {
	t.Helper()
	var value any
	decodeJSON(t, raw, &value)
}

func decodeJSON(t *testing.T, raw string, target any) {
	t.Helper()
	if err := json.Unmarshal([]byte(raw), target); err != nil {
		t.Fatalf("invalid JSON %q: %v", raw, err)
	}
}

func writeHTTPJSON(t *testing.T, w http.ResponseWriter, status int, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("write json: %v", err)
	}
}
