package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-bridge/internal/cloud"
)

func TestRunBootstrapCreatesScopedStoreAndToken(t *testing.T) {
	t.Setenv("CODENCER_CLOUD_DB_PATH", "")
	t.Setenv("CODENCER_CLOUD_HOST", "")
	t.Setenv("CODENCER_CLOUD_PORT", "")
	t.Setenv("CODENCER_CLOUD_MASTER_KEY", "")
	t.Setenv("CODENCER_CLOUD_RELAY_CONFIG", "")

	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "cloud.json")
	cfg := cloud.DefaultConfig()
	cfg.DBPath = filepath.Join(tempDir, "cloud.db")
	cfg.MasterKey = "cloud-master-key"
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, data, 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	args := []string{
		"bootstrap",
		"--config", cfgPath,
		"--org-slug", "acme",
		"--workspace-slug", "platform",
		"--project-slug", "core",
		"--token-name", "operator",
		"--scope", "cloud:read",
		"--json",
	}
	if err := run(args, &stdout, &stderr); err != nil {
		t.Fatalf("bootstrap failed: %v stderr=%s", err, stderr.String())
	}

	var payload struct {
		Org       cloud.Org       `json:"org"`
		Workspace cloud.Workspace `json:"workspace"`
		Project   cloud.Project   `json:"project"`
		Token     string          `json:"token"`
		Record    cloud.APIToken  `json:"record"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode bootstrap payload: %v body=%s", err, stdout.String())
	}
	if payload.Org.Slug != "acme" || payload.Workspace.Slug != "platform" || payload.Project.Slug != "core" {
		t.Fatalf("unexpected bootstrap scope: %+v", payload)
	}
	if !strings.HasPrefix(payload.Token, "cct_") {
		t.Fatalf("expected generated cloud token, got %q", payload.Token)
	}
	if payload.Record.Name != "operator" {
		t.Fatalf("unexpected token record: %+v", payload.Record)
	}

	store, err := cloud.OpenStore(cfg.DBPath, cfg.MasterKey)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	found, err := store.LookupAPIToken(context.Background(), payload.Token)
	if err != nil {
		t.Fatal(err)
	}
	if found == nil || found.ID == "" || found.Name != "operator" {
		t.Fatalf("expected persisted bootstrap token, got %+v", found)
	}
}

func TestRunStatusUsesAuthAndCloudURL(t *testing.T) {
	var seenMethod, seenPath, seenAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenMethod = r.Method
		seenPath = r.URL.Path
		seenAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"relay_composed":false}`))
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	if err := run([]string{"status", "--cloud-url", srv.URL, "--token", "tok", "--json"}, &stdout, &stderr); err != nil {
		t.Fatalf("status command failed: %v stderr=%s", err, stderr.String())
	}
	if seenMethod != http.MethodGet || seenPath != "/api/cloud/v1/status" {
		t.Fatalf("unexpected request: method=%s path=%s", seenMethod, seenPath)
	}
	if seenAuth != "Bearer tok" {
		t.Fatalf("unexpected authorization header: %q", seenAuth)
	}
	if got := strings.TrimSpace(stdout.String()); got != `{"ok":true,"relay_composed":false}` {
		t.Fatalf("unexpected status output: %s", got)
	}
}

func TestRunRuntimeConnectorCommandsUseCloudRuntimePaths(t *testing.T) {
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.RequestURI()+" "+r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/cloud/v1/runtime/connectors":
			_, _ = w.Write([]byte(`[]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/cloud/v1/runtime/connectors":
			_, _ = w.Write([]byte(`{"id":"rconn_1","connector_id":"conn-1"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/cloud/v1/runtime/instances":
			_, _ = w.Write([]byte(`[]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	if err := run([]string{"runtime-connectors", "list", "--cloud-url", srv.URL, "--token", "tok", "--org-id", "org-1", "--json"}, &stdout, &stderr); err != nil {
		t.Fatalf("runtime-connectors list failed: %v stderr=%s", err, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if err := run([]string{"runtime-connectors", "claim", "--cloud-url", srv.URL, "--token", "tok", "--org-id", "org-1", "--workspace-id", "ws-1", "--project-id", "proj-1", "--connector-id", "conn-1", "--json"}, &stdout, &stderr); err != nil {
		t.Fatalf("runtime-connectors claim failed: %v stderr=%s", err, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if err := run([]string{"runtime-instances", "list", "--cloud-url", srv.URL, "--token", "tok", "--runtime-connector-id", "rconn_1", "--include-unshared", "--json"}, &stdout, &stderr); err != nil {
		t.Fatalf("runtime-instances list failed: %v stderr=%s", err, stderr.String())
	}

	expected := []string{
		"GET /api/cloud/v1/runtime/connectors?org_id=org-1 Bearer tok",
		"POST /api/cloud/v1/runtime/connectors Bearer tok",
		"GET /api/cloud/v1/runtime/instances?runtime_connector_id=rconn_1&include_unshared=true Bearer tok",
	}
	if len(seen) != len(expected) {
		t.Fatalf("unexpected request count: got %v want %v", seen, expected)
	}
	for i := range expected {
		if seen[i] != expected[i] {
			t.Fatalf("unexpected request %d: got %q want %q", i, seen[i], expected[i])
		}
	}
}
