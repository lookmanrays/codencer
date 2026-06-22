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

	gatewaypkg "agent-bridge/internal/gateway"
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

func TestHelpCommandsExitZeroAndDescribeSelfHost(t *testing.T) {
	commands := [][]string{
		{"--help"},
		{"project", "--help"},
		{"project", "init", "--help"},
		{"project", "get", "--help"},
		{"project", "list", "--help"},
		{"project", "status", "--help"},
		{"machine", "--help"},
		{"connector", "--help"},
		{"connector", "login", "--help"},
		{"connector", "status", "--help"},
		{"gateway", "--help"},
		{"gateway", "relay", "--help"},
		{"login", "--help"},
		{"setup", "--help"},
		{"setup", "self-host", "--help"},
		{"setup", "relay", "--help"},
		{"setup", "gateway", "--help"},
		{"activation", "--help"},
		{"config", "--help"},
	}
	for _, command := range commands {
		stdout, stderr, err := runCLI(command...)
		if err != nil {
			t.Fatalf("%v help failed: %v stderr=%s stdout=%s", command, err, stderr, stdout)
		}
		if stderr != "" {
			t.Fatalf("%v help wrote stderr: %s", command, stderr)
		}
		for _, forbidden := range []string{"unknown command", "unknown project command", "unknown connector command", "unknown flag --help"} {
			if strings.Contains(stdout, forbidden) {
				t.Fatalf("%v help contained %q: %s", command, forbidden, stdout)
			}
		}
		for _, want := range []string{"Usage:", "Subcommands:", "Common flags:", "Examples:"} {
			if !strings.Contains(stdout, want) {
				t.Fatalf("%v help missing %q: %s", command, want, stdout)
			}
		}
	}

	for _, command := range [][]string{
		{"connector", "login", "--help"},
		{"gateway", "relay", "--help"},
		{"setup", "--help"},
		{"setup", "self-host", "--help"},
		{"setup", "relay", "--help"},
		{"setup", "gateway", "--help"},
		{"activation", "--help"},
		{"login", "--help"},
	} {
		stdout, _, err := runCLI(command...)
		if err != nil {
			t.Fatalf("%v help failed: %v", command, err)
		}
		if !strings.Contains(stdout, "127.0.0.1") {
			t.Fatalf("%v help missing self-host example: %s", command, stdout)
		}
	}

	helpRequirements := map[string][]string{
		"setup self-host": {"--gateway-url", "--relay-url", "--console-url", "--listen", "--token-env", "--token-file", "--default-relay-token-env", "--default-relay-token-file", "--enable-oauth-dev", "--oauth-client-secret", "--relay-request-timeout-seconds"},
		"setup relay":     {"--base-url", "--mcp-url", "--relay-config", "--connector-config", "--planner-token", "--planner-token-env", "--generate-planner-token", "--proxy-timeout-seconds", "--enable-chatgpt-oauth-dev", "--install-services", "--start-services", "--manager", "--bin-dir", "--strict"},
	}
	for command, wants := range helpRequirements {
		parts := append(strings.Fields(command), "--help")
		stdout, _, err := runCLI(parts...)
		if err != nil {
			t.Fatalf("%s help failed: %v", command, err)
		}
		for _, want := range wants {
			if !strings.Contains(stdout, want) {
				t.Fatalf("%s help missing %q: %s", command, want, stdout)
			}
		}
	}
}

func TestConfigProfilesAndSelfHostDefaultsJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODENCER_HOME", home)

	stdout, stderr, err := runCLI("init", "--json")
	if err != nil {
		t.Fatalf("init failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}

	stdout, stderr, err = runCLI("config", "show", "--json")
	if err != nil {
		t.Fatalf("config show failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if strings.Contains(stdout, "mcp.codencer.dev") || !strings.Contains(stdout, `"gateway_url": "http://127.0.0.1:19090"`) || !strings.Contains(stdout, `"active_profile": "self-host"`) {
		t.Fatalf("default config should be self-host/local, got %s", stdout)
	}

	stdout, stderr, err = runCLI("config", "profiles", "list", "--json")
	if err != nil {
		t.Fatalf("profiles list failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"active_profile": "self-host"`) || !strings.Contains(stdout, `"name": "self-host"`) {
		t.Fatalf("profiles list missing self-host profile: %s", stdout)
	}

	t.Setenv("CODENCER_GATEWAY_URL", "http://127.0.0.1:19191")
	stdout, stderr, err = runCLI("config", "show", "--json")
	if err != nil {
		t.Fatalf("config show with env failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"gateway_url": "http://127.0.0.1:19191"`) || !strings.Contains(stdout, `"source": "env:CODENCER_GATEWAY_URL"`) {
		t.Fatalf("env override did not win: %s", stdout)
	}
	t.Setenv("CODENCER_GATEWAY_URL", "")

	stdout, stderr, err = runCLI("config", "set", "gateway.url", "http://127.0.0.1:19091", "--json")
	if err != nil {
		t.Fatalf("config set failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"gateway_url": "http://127.0.0.1:19091"`) {
		t.Fatalf("config set output wrong: %s", stdout)
	}

	stdout, stderr, err = runCLI("config", "profiles", "use", "self-host", "--json")
	if err != nil {
		t.Fatalf("profiles use failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"active_profile": "self-host"`) {
		t.Fatalf("profiles use output wrong: %s", stdout)
	}

	stdout, stderr, err = runCLI("setup", "self-host", "--gateway-url", "http://127.0.0.1:19092", "--relay-url", "http://127.0.0.1:8092", "--listen", "127.0.0.1:19092", "--token-env", "CODENCER_GATEWAY_MCP_TOKEN", "--enable-oauth-dev", "--oauth-client-secret", "self-host-client-secret", "--json")
	if err != nil {
		t.Fatalf("setup self-host failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"mode": "self-host"`) || !strings.Contains(stdout, `"profile": "self-host"`) || strings.Contains(stdout, "self-host-client-secret") || strings.Contains(stdout, "mcp.codencer.dev") {
		t.Fatalf("setup self-host output wrong: %s", stdout)
	}
}

func TestSetupTimeoutFlagsWriteConfigs(t *testing.T) {
	t.Run("self-host default gateway timeout", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("CODENCER_HOME", home)
		stdout, stderr, err := runCLI("setup", "self-host", "--gateway-url", "http://127.0.0.1:19090", "--relay-url", "http://127.0.0.1:8090", "--json")
		if err != nil {
			t.Fatalf("setup self-host failed: %v stderr=%s stdout=%s", err, stderr, stdout)
		}
		if !strings.Contains(stdout, `"relay_request_timeout_seconds": 300`) {
			t.Fatalf("self-host output missing default gateway timeout: %s", stdout)
		}
		if got := readGatewayRelayRequestTimeout(t, home); got != 300 {
			t.Fatalf("default gateway relay request timeout = %d, want 300", got)
		}
	})

	t.Run("self-host custom gateway timeout", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("CODENCER_HOME", home)
		stdout, stderr, err := runCLI("setup", "self-host", "--gateway-url", "http://127.0.0.1:19090", "--relay-url", "http://127.0.0.1:8090", "--relay-request-timeout-seconds", "600", "--json")
		if err != nil {
			t.Fatalf("setup self-host custom failed: %v stderr=%s stdout=%s", err, stderr, stdout)
		}
		if !strings.Contains(stdout, `"relay_request_timeout_seconds": 600`) {
			t.Fatalf("self-host output missing custom gateway timeout: %s", stdout)
		}
		if got := readGatewayRelayRequestTimeout(t, home); got != 600 {
			t.Fatalf("custom gateway relay request timeout = %d, want 600", got)
		}
	})

	t.Run("gateway custom timeout", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("CODENCER_HOME", home)
		stdout, stderr, err := runCLI("setup", "gateway", "--base-url", "http://127.0.0.1:19090", "--relay-request-timeout-seconds", "650", "--json")
		if err != nil {
			t.Fatalf("setup gateway custom failed: %v stderr=%s stdout=%s", err, stderr, stdout)
		}
		if !strings.Contains(stdout, `"relay_request_timeout_seconds": 650`) {
			t.Fatalf("gateway output missing custom timeout: %s", stdout)
		}
		if got := readGatewayRelayRequestTimeout(t, home); got != 650 {
			t.Fatalf("custom gateway timeout = %d, want 650", got)
		}
	})

	t.Run("relay default proxy timeout", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("CODENCER_HOME", home)
		stdout, stderr, err := runCLI("setup", "relay", "--base-url", "http://127.0.0.1:8090", "--generate-planner-token", "--json")
		if err != nil {
			t.Fatalf("setup relay failed: %v stderr=%s stdout=%s", err, stderr, stdout)
		}
		if !strings.Contains(stdout, `"proxy_timeout_seconds": 300`) {
			t.Fatalf("relay output missing default proxy timeout: %s", stdout)
		}
		if got := readRelayProxyTimeout(t, home); got != 300 {
			t.Fatalf("default relay proxy timeout = %d, want 300", got)
		}
	})

	t.Run("relay custom proxy timeout", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("CODENCER_HOME", home)
		stdout, stderr, err := runCLI("setup", "relay", "--base-url", "http://127.0.0.1:8090", "--generate-planner-token", "--proxy-timeout-seconds", "600", "--json")
		if err != nil {
			t.Fatalf("setup relay custom failed: %v stderr=%s stdout=%s", err, stderr, stdout)
		}
		if !strings.Contains(stdout, `"proxy_timeout_seconds": 600`) {
			t.Fatalf("relay output missing custom proxy timeout: %s", stdout)
		}
		if got := readRelayProxyTimeout(t, home); got != 600 {
			t.Fatalf("custom relay proxy timeout = %d, want 600", got)
		}
	})

	for _, command := range [][]string{
		{"setup", "self-host", "--relay-request-timeout-seconds", "0", "--json"},
		{"setup", "gateway", "--relay-request-timeout-seconds", "-1", "--json"},
		{"setup", "relay", "--proxy-timeout-seconds", "abc", "--json"},
	} {
		home := t.TempDir()
		t.Setenv("CODENCER_HOME", home)
		stdout, _, err := runCLI(command...)
		if err == nil {
			t.Fatalf("%v unexpectedly succeeded: %s", command, stdout)
		}
		if !strings.Contains(stdout, "positive integer") {
			t.Fatalf("%v error did not explain positive integer requirement: %s", command, stdout)
		}
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
	stdout, stderr, err = runCLI("executor", "list", "--json")
	if err != nil {
		t.Fatalf("executor list failed: %v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, `"executors"`) || !strings.Contains(stdout, `"fake-success"`) {
		t.Fatalf("executor list missing fake profile: %s", stdout)
	}
	stdout, stderr, err = runCLI("executor", "test", "fake-success", "--json")
	if err != nil {
		t.Fatalf("executor test fake failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"installed": true`) || !strings.Contains(stdout, `"adapter": "fake"`) {
		t.Fatalf("executor test fake output wrong: %s", stdout)
	}
	stdout, stderr, err = runCLI("executor", "default", "fake-blocker", "--repo", repo, "--json")
	if err != nil {
		t.Fatalf("executor default failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"registry_updated": true`) || !strings.Contains(stdout, `"id": "fake-blocker"`) {
		t.Fatalf("executor default output wrong: %s", stdout)
	}
	projectJSON, err := os.ReadFile(filepath.Join(repo, ".codencer", "project.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(projectJSON), `"default_profile": "fake-blocker"`) {
		t.Fatalf("executor default did not update project config: %s", string(projectJSON))
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

func TestConnectorFacadeJSON(t *testing.T) {
	relay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v2/connectors/enroll" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		writeHTTPJSON(t, w, http.StatusOK, map[string]any{
			"connector_id": "conn-1",
			"machine_id":   "machine-1",
			"relay": map[string]any{
				"relay_url":                  relayURLFromRequest(r),
				"websocket_url":              "ws://relay.example.com/api/v2/connectors/ws",
				"heartbeat_interval_seconds": 5,
			},
		})
	}))
	defer relay.Close()

	home := t.TempDir()
	t.Setenv("CODENCER_HOME", home)
	configPath := filepath.Join(home, "runtime", "connector", "config.json")
	stdout, stderr, err := runCLI("connector", "enroll",
		"--relay-url", relay.URL,
		"--daemon-url", "http://127.0.0.1:1",
		"--enrollment-token", "enroll-secret",
		"--config", configPath,
		"--codencer-home", home,
		"--label", "test-connector",
		"--json")
	if err != nil {
		t.Fatalf("connector enroll failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	var enroll map[string]any
	decodeJSON(t, stdout, &enroll)
	if enroll["connector_id"] != "conn-1" || enroll["local_config_updated"] != true {
		t.Fatalf("unexpected enroll report: %s", stdout)
	}

	stdout, stderr, err = runCLI("connector", "status", "--config", configPath, "--json")
	if err != nil {
		t.Fatalf("connector status failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"connector_id": "conn-1"`) {
		t.Fatalf("status missing connector id: %s", stdout)
	}

	stdout, stderr, err = runCLI("connector", "config", "show", "--config", configPath, "--json")
	if err != nil {
		t.Fatalf("connector config show failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if strings.Contains(stdout, "PRIVATE KEY") || strings.Contains(stdout, "enroll-secret") {
		t.Fatalf("config show leaked secret: %s", stdout)
	}
	if !strings.Contains(stdout, `"codencer_home": "`+home+`"`) {
		t.Fatalf("config show missing codencer_home: %s", stdout)
	}
}

func relayURLFromRequest(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
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

	stdout, stderr, err = runCLI("setup", "gateway", "--base-url", "http://127.0.0.1:19090", "--mcp-url", "http://127.0.0.1:19090/mcp", "--listen", "127.0.0.1:19090", "--token-env", "CODENCER_GATEWAY_MCP_TOKEN", "--enable-oauth-dev", "--oauth-client-secret", "gateway-client-secret", "--json")
	if err != nil {
		t.Fatalf("setup gateway failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"mode": "gateway"`) || !strings.Contains(stdout, `"gateway_config"`) || strings.Contains(stdout, "gateway-client-secret") {
		t.Fatalf("setup gateway output wrong: %s", stdout)
	}

	stdout, stderr, err = runCLI("gateway", "relay", "add", "--id", "personal", "--url", "https://relay.example.com", "--token-env", "CODENCER_RELAY_PERSONAL_TOKEN", "--json")
	if err != nil {
		t.Fatalf("gateway relay add failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"id": "personal"`) || strings.Contains(stdout, "relay-secret") {
		t.Fatalf("gateway relay add output wrong: %s", stdout)
	}

	stdout, stderr, err = runCLI("gateway", "relay", "list", "--json")
	if err != nil {
		t.Fatalf("gateway relay list failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"token_env": "CODENCER_RELAY_PERSONAL_TOKEN"`) {
		t.Fatalf("gateway relay list output wrong: %s", stdout)
	}

	stdout, stderr, err = runCLI("activation", "gateway", "--gateway", "https://mcp.codencer.dev", "--relay", "https://relay.example.com", "--project", "codencer", "--token", "literal-gateway-token", "--json")
	if err != nil {
		t.Fatalf("activation gateway failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"mode": "gateway"`) || !strings.Contains(stdout, `"mcp_url": "https://mcp.codencer.dev/mcp"`) || strings.Contains(stdout, "literal-gateway-token") {
		t.Fatalf("activation gateway output wrong: %s", stdout)
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
	if !strings.Contains(stdout, `"mcp_endpoint": "https://relay.example.com/mcp"`) ||
		!strings.Contains(stdout, `"chatgpt_ui_steps"`) ||
		!strings.Contains(stdout, `"evidence_checklist"`) {
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
	if !strings.Contains(stdout, `claude mcp add --transport http --header \"Authorization: Bearer $CODENCER_MCP_TOKEN\" codencer https://relay.example.com/mcp`) {
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

func TestProjectConfigInitAdoptScanAndMachineJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODENCER_HOME", home)
	repo := makeTestRepo(t)

	stdout, stderr, err := runCLI("init", "--json")
	if err != nil {
		t.Fatalf("init failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if _, err := os.Stat(filepath.Join(home, "machine.json")); err != nil {
		t.Fatalf("machine.json missing: %v", err)
	}

	stdout, stderr, err = runCLI("project", "init", "--id", "codencer", "--name", "Codencer", "--repo", repo, "--json")
	if err != nil {
		t.Fatalf("project init failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	projectConfigPath := filepath.Join(repo, ".codencer", "project.json")
	before, err := os.ReadFile(projectConfigPath)
	if err != nil {
		t.Fatalf("project config missing: %v", err)
	}
	if got := listRelativeFiles(t, filepath.Join(repo, ".codencer")); strings.Join(got, ",") != "project.json" {
		t.Fatalf("project init created unexpected .codencer footprint: %v", got)
	}
	var initPayload map[string]any
	decodeJSON(t, stdout, &initPayload)
	if initPayload["project_config_action"] != "created" {
		t.Fatalf("expected created project config action, got %s", stdout)
	}
	projectPayload := initPayload["project"].(map[string]any)
	if projectPayload["machine_id"] == "" || projectPayload["project_config_path"] != projectConfigPath {
		t.Fatalf("project registry missing machine/config path fields: %s", stdout)
	}

	stdout, stderr, err = runCLI("project", "init", "--repo", repo, "--json")
	if err != nil {
		t.Fatalf("project init adopt existing failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	after, err := os.ReadFile(projectConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("project init overwrote existing project config\nbefore=%s\nafter=%s", before, after)
	}
	if !strings.Contains(stdout, `"project_config_action": "adopted"`) {
		t.Fatalf("expected adopted action, got %s", stdout)
	}

	missingRepo := makeTestRepo(t)
	stdout, _, err = runCLI("project", "adopt", "--repo", missingRepo, "--json")
	if err == nil || !strings.Contains(stdout, ".codencer/project.json is required") {
		t.Fatalf("expected adopt missing config error, got err=%v stdout=%s", err, stdout)
	}

	scanRepo := t.TempDir()
	if err := os.WriteFile(filepath.Join(scanRepo, "go.mod"), []byte("module example.test/scan\n"), 0644); err != nil {
		t.Fatal(err)
	}
	beforeScan := listRelativeFiles(t, scanRepo)
	stdout, stderr, err = runCLI("project", "scan", "--repo", scanRepo, "--json")
	if err != nil {
		t.Fatalf("project scan failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	afterScan := listRelativeFiles(t, scanRepo)
	if strings.Join(beforeScan, ",") != strings.Join(afterScan, ",") {
		t.Fatalf("project scan wrote files: before=%v after=%v", beforeScan, afterScan)
	}
	if _, err := os.Stat(filepath.Join(scanRepo, ".codencer")); !os.IsNotExist(err) {
		t.Fatalf("project scan should not create .codencer, stat err=%v", err)
	}

	stdout, stderr, err = runCLI("machine", "set-label", "MacBook Test", "--json")
	if err != nil {
		t.Fatalf("machine set-label failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"host_label": "macbook-test"`) {
		t.Fatalf("host label override missing: %s", stdout)
	}
}

func TestAccountLoginRelayRegistryAndConnectorLoginJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODENCER_HOME", home)
	t.Setenv("CODENCER_DEFAULT_RELAY_TOKEN", "relay-secret")
	t.Setenv("CODENCER_SELFHOST_RELAY_TOKEN", "relay-secret")

	relay := newOfficialConnectorFakeRelay(t)
	defer relay.Close()
	cfg := gatewaypkg.DefaultConfig()
	cfg.PublicBaseURL = "http://127.0.0.1:19090"
	cfg.MCPURL = "http://127.0.0.1:19090/mcp"
	cfg.Store.Path = filepath.Join(t.TempDir(), "gateway.db")
	cfg.DefaultRelay.URL = relay.URL
	cfg.DefaultRelay.TokenEnv = "CODENCER_DEFAULT_RELAY_TOKEN"
	server, err := gatewaypkg.NewServer(cfg, gatewaypkg.ServerOptions{})
	if err != nil {
		t.Fatalf("new gateway: %v", err)
	}
	gatewayServer := httptest.NewServer(server.Handler())
	defer gatewayServer.Close()

	stdout, stderr, err := runCLI("login", "--gateway", gatewayServer.URL, "--email", "dev@example.com", "--display-name", "Dev", "--dev-approve", "--timeout", "5s", "--json")
	if err != nil {
		t.Fatalf("login failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if strings.Contains(stdout, "access_token") || strings.Contains(stdout, "relay-secret") {
		t.Fatalf("login output leaked token material: %s", stdout)
	}
	if _, err := os.Stat(filepath.Join(home, "session.json")); err != nil {
		t.Fatalf("session not written: %v", err)
	}

	stdout, stderr, err = runCLI("whoami", "--json")
	if err != nil {
		t.Fatalf("whoami failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"workspace_id"`) || strings.Contains(stdout, "access_token") {
		t.Fatalf("whoami output wrong: %s", stdout)
	}

	stdout, stderr, err = runCLI("gateway", "relay", "list", "--json")
	if err != nil {
		t.Fatalf("remote relay list failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"id": "default"`) || strings.Contains(stdout, "relay-secret") {
		t.Fatalf("remote relay list output wrong: %s", stdout)
	}

	stdout, stderr, err = runCLI("gateway", "relay", "add", "--name", "personal", "--url", relay.URL, "--token-env", "CODENCER_SELFHOST_RELAY_TOKEN", "--json")
	if err != nil {
		t.Fatalf("remote relay add failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"id": "personal"`) || !strings.Contains(stdout, `"token_configured": true`) || strings.Contains(stdout, "relay-secret") {
		t.Fatalf("remote relay add output wrong: %s", stdout)
	}

	stdout, stderr, err = runCLI("gateway", "relay", "status", "personal", "--json")
	if err != nil {
		t.Fatalf("remote relay status failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"id": "personal"`) || strings.Contains(stdout, "relay-secret") {
		t.Fatalf("remote relay status output wrong: %s", stdout)
	}

	stdout, stderr, err = runCLI("connector", "login", "--gateway", gatewayServer.URL, "--relay", "default", "--json")
	if err != nil {
		t.Fatalf("connector login failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"connector_id": "conn-fake"`) || !strings.Contains(stdout, `"relay_profile_id": "default"`) {
		t.Fatalf("connector login output missing ids: %s", stdout)
	}
	for _, forbidden := range []string{"enroll-secret", "private_key", "relay-secret"} {
		if strings.Contains(stdout, forbidden) {
			t.Fatalf("connector login output leaked %q: %s", forbidden, stdout)
		}
	}
	if _, err := os.Stat(filepath.Join(home, "runtime", "connector", "config.json")); err != nil {
		t.Fatalf("connector config not written: %v", err)
	}

	stdout, stderr, err = runCLI("gateway", "relay", "remove", "personal", "--json")
	if err != nil {
		t.Fatalf("remote relay remove failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"relay_profile_id": "personal"`) {
		t.Fatalf("remote relay remove output wrong: %s", stdout)
	}

	stdout, stderr, err = runCLI("logout", "--json")
	if err != nil {
		t.Fatalf("logout failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if _, err := os.Stat(filepath.Join(home, "session.json")); !os.IsNotExist(err) {
		t.Fatalf("session still present after logout: %v", err)
	}
}

func TestProjectListBackfillsOldRegistryMachineMetadata(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODENCER_HOME", home)
	repo := makeTestRepo(t)
	if _, _, err := runCLI("init", "--json"); err != nil {
		t.Fatal(err)
	}
	if stdout, stderr, err := runCLI("project", "init", "--id", "codencer", "--repo", repo, "--json"); err != nil {
		t.Fatalf("project init failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	registryPath := filepath.Join(home, "projects.json")
	raw, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatal(err)
	}
	var registry map[string]any
	decodeJSON(t, string(raw), &registry)
	projects := registry["projects"].([]any)
	project := projects[0].(map[string]any)
	delete(project, "machine_id")
	delete(project, "host_label")
	delete(project, "hostname")
	rewritten, _ := json.MarshalIndent(registry, "", "  ")
	if err := os.WriteFile(registryPath, append(rewritten, '\n'), 0600); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := runCLI("project", "list", "--json")
	if err != nil {
		t.Fatalf("project list failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"machine_id": "mach_`) || !strings.Contains(stdout, `"host_label":`) {
		t.Fatalf("expected project list to backfill machine metadata, got %s", stdout)
	}
}

func runCLI(args ...string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	err := run(args, &stdout, &stderr)
	return stdout.String(), stderr.String(), err
}

func newOfficialConnectorFakeRelay(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	requireRelayAuth := func(w http.ResponseWriter, r *http.Request) bool {
		if r.Header.Get("Authorization") != "Bearer relay-secret" {
			writeHTTPJSON(t, w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"code": "auth_failed", "message": "bad relay token"}})
			return false
		}
		return true
	}
	var server *httptest.Server
	mux.HandleFunc("/api/v2/status", func(w http.ResponseWriter, r *http.Request) {
		if !requireRelayAuth(w, r) {
			return
		}
		writeHTTPJSON(t, w, http.StatusOK, map[string]any{"ok": true})
	})
	mux.HandleFunc("/api/v2/connectors/enrollment-tokens", func(w http.ResponseWriter, r *http.Request) {
		if !requireRelayAuth(w, r) {
			return
		}
		writeHTTPJSON(t, w, http.StatusOK, map[string]any{"secret": "enroll-secret"})
	})
	mux.HandleFunc("/api/v2/connectors/enroll", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeHTTPJSON(t, w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		if req["enrollment_token"] != "enroll-secret" && req["enrollment_secret"] != "enroll-secret" {
			writeHTTPJSON(t, w, http.StatusForbidden, map[string]any{"error": "bad enrollment token"})
			return
		}
		writeHTTPJSON(t, w, http.StatusOK, map[string]any{
			"connector_id": "conn-fake",
			"machine_id":   "mach-relay",
			"relay": map[string]any{
				"relay_url":                  server.URL,
				"websocket_url":              strings.Replace(server.URL, "http://", "ws://", 1) + "/api/v2/connectors/ws",
				"heartbeat_interval_seconds": 15,
			},
		})
	})
	server = httptest.NewServer(mux)
	return server
}

func listRelativeFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
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

func readGatewayRelayRequestTimeout(t *testing.T, home string) int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, "runtime", "gateway", "config.json"))
	if err != nil {
		t.Fatalf("read gateway config: %v", err)
	}
	var cfg struct {
		RelayRequestTimeoutSeconds int `json:"relay_request_timeout_seconds"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("decode gateway config: %v", err)
	}
	return cfg.RelayRequestTimeoutSeconds
}

func readRelayProxyTimeout(t *testing.T, home string) int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, "runtime", "relay", "config.json"))
	if err != nil {
		t.Fatalf("read relay config: %v", err)
	}
	var cfg struct {
		ProxyTimeoutSeconds int `json:"proxy_timeout_seconds"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("decode relay config: %v", err)
	}
	return cfg.ProxyTimeoutSeconds
}

func writeHTTPJSON(t *testing.T, w http.ResponseWriter, status int, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("write json: %v", err)
	}
}
