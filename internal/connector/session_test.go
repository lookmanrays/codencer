package connector

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/relayproto"
	"github.com/gorilla/websocket"
)

func TestClientRun_HandshakeAdvertiseAndProxy(t *testing.T) {
	cfg := &Config{
		RelayURL:                 "http://relay.invalid",
		ConnectorID:              "connector-1",
		MachineID:                "machine-1",
		HeartbeatIntervalSeconds: 1,
		Instances:                []SharedInstanceConfig{{InstanceID: "inst-1", DaemonURL: "", Share: true}},
		ConfigPath:               filepath.Join(t.TempDir(), "connector.json"),
	}
	if err := EnsureKeypair(cfg); err != nil {
		t.Fatal(err)
	}

	var daemon *httptest.Server
	daemon = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/instance":
			_ = json.NewEncoder(w).Encode(domain.InstanceInfo{ID: "inst-1", BaseURL: daemon.URL, RepoRoot: "/repo"})
		case "/api/v1/steps/step-1/result":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"v1","run_id":"run-1","step_id":"step-1","state":"completed","summary":"done"}`))
		case "/api/v1/steps/step-1/validations":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"name":"tests","status":"passed"}]`))
		case "/api/v1/steps/step-1/artifacts":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"artifact_id":"art-1","label":"report"}]`))
		case "/api/v1/steps/step-1/logs":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"stream":"stdout","chunk":"done"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer daemon.Close()
	cfg.Instances[0].DaemonURL = daemon.URL

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	var advertised atomic.Bool
	var heartbeats atomic.Int32
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/connectors/challenge":
			_ = json.NewEncoder(w).Encode(relayproto.ChallengeResponse{
				ChallengeID: "challenge-1",
				Nonce:       "nonce-1",
				Relay: relayproto.RelayMetadata{
					WebsocketURL:             "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/connectors",
					HeartbeatIntervalSeconds: 1,
				},
			})
		case "/ws/connectors":
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()

			var hello relayproto.HelloMessage
			if err := conn.ReadJSON(&hello); err != nil {
				t.Fatal(err)
			}
			publicKey, _ := base64.StdEncoding.DecodeString(cfg.PublicKey)
			signature, _ := base64.StdEncoding.DecodeString(hello.Signature)
			payload := []byte("challenge-1:nonce-1:connector-1:machine-1")
			if !ed25519.Verify(ed25519.PublicKey(publicKey), payload, signature) {
				t.Fatal("expected connector hello signature to verify")
			}

			var advertise relayproto.AdvertiseMessage
			if err := conn.ReadJSON(&advertise); err != nil {
				t.Fatal(err)
			}
			if len(advertise.Instances) != 1 {
				t.Fatalf("expected one advertised instance, got %d", len(advertise.Instances))
			}
			advertised.Store(true)

			requests := []struct {
				id       string
				path     string
				contains string
			}{
				{id: "req-1", path: "/api/v1/steps/step-1/result", contains: `"summary":"done"`},
				{id: "req-2", path: "/api/v1/steps/step-1/validations", contains: `"status":"passed"`},
				{id: "req-3", path: "/api/v1/steps/step-1/artifacts", contains: `"artifact_id":"art-1"`},
				{id: "req-4", path: "/api/v1/steps/step-1/logs", contains: `"chunk":"done"`},
			}
			for _, req := range requests {
				if err := conn.WriteJSON(relayproto.CommandRequest{
					Type:       "request",
					RequestID:  req.id,
					InstanceID: "inst-1",
					Method:     http.MethodGet,
					Path:       req.path,
				}); err != nil {
					t.Fatal(err)
				}

				var response relayproto.CommandResponse
				if err := conn.ReadJSON(&response); err != nil {
					t.Fatal(err)
				}
				if response.StatusCode != http.StatusOK || !strings.Contains(string(response.Body), req.contains) {
					t.Fatalf("unexpected proxy response for %s: %+v", req.path, response)
				}
			}

			var heartbeat relayproto.HeartbeatMessage
			if err := conn.ReadJSON(&heartbeat); err == nil && heartbeat.Type == "heartbeat" {
				heartbeats.Add(1)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg.RelayURL = server.URL
	cfg.WebsocketURL = "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/connectors"
	client := NewClient(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
	defer cancel()
	_ = client.Run(ctx)

	if !advertised.Load() {
		t.Fatal("expected connector to advertise shared instances")
	}
	if heartbeats.Load() == 0 {
		t.Fatal("expected connector heartbeat")
	}

	status, err := LoadStatus(cfg.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if status.ConnectorID != "connector-1" || status.MachineID != "machine-1" {
		t.Fatalf("unexpected status identity: %+v", status)
	}
	if status.LastConnectAt == "" || status.LastHeartbeatAt == "" || len(status.SharedInstances) != 1 || status.SharedInstances[0] != "inst-1" {
		t.Fatalf("unexpected status after connect: %+v", status)
	}
}

func TestClientHandleRequestRejectsUnsafeStepPath(t *testing.T) {
	var daemon *httptest.Server
	daemon = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/instance" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(domain.InstanceInfo{ID: "inst-1", BaseURL: daemon.URL, RepoRoot: "/repo"})
	}))
	defer daemon.Close()

	client := NewClient(&Config{
		Instances: []SharedInstanceConfig{{InstanceID: "inst-1", DaemonURL: daemon.URL, Share: true}},
	})

	response := client.handleRequest(context.Background(), relayproto.CommandRequest{
		RequestID:  "req-unsafe",
		Method:     http.MethodGet,
		Path:       "/api/v1/steps/step-1/private",
		InstanceID: "inst-1",
	})
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected forbidden response, got %+v", response)
	}
	if !strings.Contains(response.Error, "connector denied") {
		t.Fatalf("expected deny message, got %+v", response)
	}
}

func TestClientRun_ReconnectsAndReAdvertises(t *testing.T) {
	cfg := &Config{
		RelayURL:                 "http://relay.invalid",
		ConnectorID:              "connector-2",
		MachineID:                "machine-2",
		HeartbeatIntervalSeconds: 1,
		Instances:                []SharedInstanceConfig{{InstanceID: "inst-2", Share: true}},
		ConfigPath:               filepath.Join(t.TempDir(), "connector.json"),
	}
	if err := EnsureKeypair(cfg); err != nil {
		t.Fatal(err)
	}

	var daemon *httptest.Server
	daemon = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/instance" {
			_ = json.NewEncoder(w).Encode(domain.InstanceInfo{ID: "inst-2", BaseURL: daemon.URL, RepoRoot: "/repo"})
			return
		}
		http.NotFound(w, r)
	}))
	defer daemon.Close()
	cfg.Instances[0].DaemonURL = daemon.URL

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	var connections atomic.Int32
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/connectors/challenge":
			_ = json.NewEncoder(w).Encode(relayproto.ChallengeResponse{
				ChallengeID: "challenge-reconnect",
				Nonce:       "nonce-reconnect",
				Relay: relayproto.RelayMetadata{
					WebsocketURL:             "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/connectors",
					HeartbeatIntervalSeconds: 1,
				},
			})
		case "/ws/connectors":
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			connections.Add(1)

			var hello relayproto.HelloMessage
			if err := conn.ReadJSON(&hello); err != nil {
				t.Fatal(err)
			}
			var advertise relayproto.AdvertiseMessage
			if err := conn.ReadJSON(&advertise); err != nil {
				t.Fatal(err)
			}
			if connections.Load() == 1 {
				_ = conn.Close()
				return
			}
			time.Sleep(150 * time.Millisecond)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg.RelayURL = server.URL
	cfg.WebsocketURL = "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/connectors"
	client := NewClient(cfg)
	client.backoff = NewBackoff(10*time.Millisecond, 20*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_ = client.Run(ctx)

	if connections.Load() < 2 {
		t.Fatalf("expected connector to reconnect, got %d connections", connections.Load())
	}

	status, err := LoadStatus(cfg.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if status.LastConnectAt == "" || len(status.SharedInstances) != 1 || status.SharedInstances[0] != "inst-2" {
		t.Fatalf("expected reconnect to refresh status and re-advertise, got %+v", status)
	}
}
