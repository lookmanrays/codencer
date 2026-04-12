package connector

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

			if err := conn.WriteJSON(relayproto.CommandRequest{
				Type:       "request",
				RequestID:  "req-1",
				InstanceID: "inst-1",
				Method:     http.MethodGet,
				Path:       "/api/v1/steps/step-1/result",
			}); err != nil {
				t.Fatal(err)
			}

			var response relayproto.CommandResponse
			if err := conn.ReadJSON(&response); err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != http.StatusOK || !strings.Contains(string(response.Body), `"summary":"done"`) {
				t.Fatalf("unexpected proxy response: %+v", response)
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
}

func TestClientRun_ReconnectsAndReAdvertises(t *testing.T) {
	cfg := &Config{
		RelayURL:                 "http://relay.invalid",
		ConnectorID:              "connector-2",
		MachineID:                "machine-2",
		HeartbeatIntervalSeconds: 1,
		Instances:                []SharedInstanceConfig{{InstanceID: "inst-2", Share: true}},
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
}
