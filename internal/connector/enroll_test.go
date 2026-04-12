package connector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/relayproto"
)

func TestEnroll_PersistsIdentityAndSharedInstance(t *testing.T) {
	var gotEnroll relayproto.EnrollmentRequest
	var relay *httptest.Server
	relay = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/connectors/enroll" {
			http.NotFound(w, r)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&gotEnroll); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(relayproto.EnrollmentResponse{
			ConnectorID: "connector-1",
			MachineID:   "machine-1",
			Relay: relayproto.RelayMetadata{
				RelayURL:                 relay.URL,
				WebsocketURL:             "ws" + strings.TrimPrefix(relay.URL, "http") + "/ws/connectors",
				HeartbeatIntervalSeconds: 9,
			},
		})
	}))
	defer relay.Close()

	var daemon *httptest.Server
	daemon = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/instance":
			_ = json.NewEncoder(w).Encode(domain.InstanceInfo{
				ID:           "inst-1",
				BaseURL:      daemon.URL,
				ManifestPath: "/repo/.codencer/instance.json",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer daemon.Close()

	configPath := filepath.Join(t.TempDir(), "connector.json")
	cfg, err := Enroll(context.Background(), relay.URL, daemon.URL, "token-1", "local", configPath)
	if err != nil {
		t.Fatal(err)
	}
	if gotEnroll.PublicKey == "" || gotEnroll.Machine.Hostname == "" {
		t.Fatalf("expected public key and machine metadata in enrollment payload: %+v", gotEnroll)
	}
	if cfg.ConnectorID != "connector-1" || cfg.MachineID != "machine-1" {
		t.Fatalf("unexpected persisted identity: %+v", cfg)
	}
	if len(cfg.Instances) != 1 || !cfg.Instances[0].Share || cfg.Instances[0].InstanceID != "inst-1" {
		t.Fatalf("expected enrolled config to contain one shared instance, got %+v", cfg.Instances)
	}
}
