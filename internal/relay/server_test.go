package relay

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/relayproto"
)

func TestHandleAdvertiseReplacesSharedInstancesAndPrunesRoutes(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	server := NewServer(&Config{
		Host:              "127.0.0.1",
		Port:              0,
		DBPath:            filepath.Join(t.TempDir(), "unused.db"),
		PlannerToken:      "planner-token",
		SessionTTLSeconds: 60,
	}, store)
	session := &session{
		connectorID: "connector-1",
		instanceIDs: map[string]struct{}{},
		pending:     make(map[string]chan relayproto.CommandResponse),
	}
	server.hub.RegisterConnector(session)

	first := mustAdvertiseMessage(t, "inst-1", "/repo-a", "http://127.0.0.1:8085")
	if err := server.handleAdvertise(context.Background(), session, first); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveResourceRoute(context.Background(), "step", "step-1", "inst-1"); err != nil {
		t.Fatal(err)
	}
	if got := server.hub.Get("inst-1"); got == nil {
		t.Fatal("expected inst-1 to be routable after initial advertise")
	}

	second := mustAdvertiseMessage(t, "inst-2", "/repo-b", "http://127.0.0.1:8086")
	if err := server.handleAdvertise(context.Background(), session, second); err != nil {
		t.Fatal(err)
	}

	inst1, err := store.GetInstance(context.Background(), "inst-1")
	if err != nil {
		t.Fatal(err)
	}
	if inst1 != nil {
		t.Fatalf("expected inst-1 to be pruned from store, got %+v", inst1)
	}
	inst2, err := store.GetInstance(context.Background(), "inst-2")
	if err != nil {
		t.Fatal(err)
	}
	if inst2 == nil || inst2.ConnectorID != "connector-1" {
		t.Fatalf("expected inst-2 to remain stored, got %+v", inst2)
	}
	route, err := store.LookupResourceRoute(context.Background(), "step", "step-1")
	if err != nil {
		t.Fatal(err)
	}
	if route != "" {
		t.Fatalf("expected step route hint to be pruned, got %q", route)
	}
	if got := server.hub.Get("inst-1"); got != nil {
		t.Fatalf("expected inst-1 to be removed from live hub, got %+v", got)
	}
	if got := server.hub.Get("inst-2"); got != session {
		t.Fatalf("expected inst-2 to remain live, got %+v", got)
	}
}

func TestResolveResourceRouteIgnoresOfflineHint(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.SaveConnector(context.Background(), "connector-1", "machine-1", "pub", "offline"); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveInstance(context.Background(), InstanceRecord{
		InstanceID:   "inst-offline",
		ConnectorID:  "connector-1",
		RepoRoot:     "/repo",
		BaseURL:      "http://127.0.0.1:8085",
		InstanceJSON: `{"id":"inst-offline"}`,
		LastSeenAt:   time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveResourceRoute(context.Background(), "step", "step-1", "inst-offline"); err != nil {
		t.Fatal(err)
	}

	server := NewServer(&Config{
		Host:         "127.0.0.1",
		Port:         0,
		DBPath:       filepath.Join(t.TempDir(), "unused.db"),
		PlannerToken: "planner-token",
	}, store)

	instanceID, apiErr := server.resolveResourceRoute(context.Background(), &plannerPrincipal{
		Name:   "operator",
		Scopes: []string{"*"},
	}, "step", "step-1", "steps:read", "")
	if apiErr == nil {
		t.Fatalf("expected offline route hint to fail closed, got instance %s", instanceID)
	}
	if instanceID != "" {
		t.Fatalf("expected no routed instance for offline hint, got %s", instanceID)
	}
	if apiErr.Code != "connector_offline" {
		t.Fatalf("expected connector_offline, got %+v", apiErr)
	}
}

func mustAdvertiseMessage(t *testing.T, instanceID, repoRoot, baseURL string) []byte {
	t.Helper()
	info, err := json.Marshal(domain.InstanceInfo{
		ID:       instanceID,
		RepoRoot: repoRoot,
		BaseURL:  baseURL,
	})
	if err != nil {
		t.Fatal(err)
	}
	message, err := json.Marshal(relayproto.AdvertiseMessage{
		Type:      "advertise",
		Instances: []relayproto.InstanceAdvertisement{{Instance: info}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return message
}
