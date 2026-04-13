package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-bridge/internal/connector"
	"agent-bridge/internal/domain"
)

func TestRunShareUnshareListAndConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "connector.json")
	cfg := &connector.Config{
		RelayURL:       "http://relay.invalid",
		ConnectorID:    "connector-1",
		MachineID:      "machine-1",
		PrivateKey:     "secret-key",
		PublicKey:      "public-key",
		ConfigPath:     configPath,
		Instances:      []connector.SharedInstanceConfig{{InstanceID: "inst-known", DaemonURL: "http://127.0.0.1:8085", Share: true}},
		DiscoveryRoots: []string{"/repos"},
	}
	if err := connector.SaveConfig(configPath, cfg); err != nil {
		t.Fatal(err)
	}

	var daemon *httptest.Server
	daemon = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/instance" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(domain.InstanceInfo{
			ID:           "inst-new",
			BaseURL:      daemon.URL,
			ManifestPath: "/repo/.codencer/instance.json",
		})
	}))
	defer daemon.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := run(context.Background(), []string{"share", "--config", configPath, "--daemon-url", daemon.URL}, &stdout, &stderr); err != nil {
		t.Fatalf("share failed: %v stderr=%s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "state=shared") || !strings.Contains(stdout.String(), "instance_id=inst-new") {
		t.Fatalf("unexpected share output: %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := run(context.Background(), []string{"unshare", "--config", configPath, "--instance-id", "inst-new"}, &stdout, &stderr); err != nil {
		t.Fatalf("unshare failed: %v stderr=%s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "state=unshared") {
		t.Fatalf("unexpected unshare output: %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := run(context.Background(), []string{"list", "--config", configPath}, &stdout, &stderr); err != nil {
		t.Fatalf("list failed: %v stderr=%s", err, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "instance_id=inst-known") || !strings.Contains(output, "instance_id=inst-new") {
		t.Fatalf("expected list output to include both known instances, got %s", output)
	}
	if !strings.Contains(output, "state=unshared") {
		t.Fatalf("expected list output to include unshared entries, got %s", output)
	}

	stdout.Reset()
	stderr.Reset()
	if err := run(context.Background(), []string{"config", "--config", configPath, "--json"}, &stdout, &stderr); err != nil {
		t.Fatalf("config failed: %v stderr=%s", err, stderr.String())
	}
	if strings.Contains(stdout.String(), "secret-key") {
		t.Fatalf("expected redacted config output, got %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "[redacted]") {
		t.Fatalf("expected redacted marker in config output, got %s", stdout.String())
	}
}

func TestRunStatusTextStaysInformativeAndStatusJSONPassesThrough(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "connector.json")
	cfg := &connector.Config{
		RelayURL:    "http://relay.invalid",
		ConnectorID: "connector-1",
		MachineID:   "machine-1",
		ConfigPath:  configPath,
		Instances: []connector.SharedInstanceConfig{
			{InstanceID: "inst-shared", Share: true},
			{InstanceID: "inst-hidden", Share: false},
		},
	}
	if err := connector.SaveConfig(configPath, cfg); err != nil {
		t.Fatal(err)
	}
	store := connector.NewStatusStore(configPath)
	if err := store.Seed(cfg); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkConnected(cfg, []string{"inst-shared"}, time.Unix(10, 0)); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := run(context.Background(), []string{"status", "--config", configPath}, &stdout, &stderr); err != nil {
		t.Fatalf("status failed: %v stderr=%s", err, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "configured_instances=2") || !strings.Contains(output, "unshared_config=1") {
		t.Fatalf("expected richer status output, got %s", output)
	}
	if !strings.Contains(output, "state=unshared instance_id=inst-hidden") {
		t.Fatalf("expected status output to include configured unshared instance, got %s", output)
	}

	stdout.Reset()
	stderr.Reset()
	if err := run(context.Background(), []string{"status", "--config", configPath, "--json"}, &stdout, &stderr); err != nil {
		t.Fatalf("status --json failed: %v stderr=%s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "\"session_state\": \"connected\"") {
		t.Fatalf("expected raw status json output, got %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "inst-hidden") {
		t.Fatalf("expected status --json to remain raw status file output, got %s", stdout.String())
	}
}
