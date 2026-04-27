package connector

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStatusStoreTransitions(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "connector.json")
	store := NewStatusStore(configPath)
	cfg := &Config{
		RelayURL:                 "http://relay.invalid",
		ConnectorID:              "connector-1",
		MachineID:                "machine-1",
		ConfigPath:               configPath,
		Instances:                []SharedInstanceConfig{{InstanceID: "inst-1", Share: true}},
		HeartbeatIntervalSeconds: 1,
	}

	if err := store.Seed(cfg); err != nil {
		t.Fatal(err)
	}

	status, err := LoadStatus(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if status.SessionState != SessionStateDisconnected {
		t.Fatalf("expected disconnected seed state, got %s", status.SessionState)
	}
	if len(status.SharedInstances) != 1 || status.SharedInstances[0] != "inst-1" {
		t.Fatalf("unexpected shared instances after seed: %+v", status.SharedInstances)
	}

	if err := store.MarkConnecting(cfg); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkConnected(cfg, []string{"inst-1"}, time.Unix(10, 0)); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkHeartbeat(cfg, []string{"inst-1"}, time.Unix(20, 0)); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkDisconnected(cfg, time.Unix(30, 0)); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkFailure(cfg, "boom", time.Unix(40, 0)); err != nil {
		t.Fatal(err)
	}

	status, err = LoadStatus(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if status.SessionState != SessionStateError {
		t.Fatalf("expected error state, got %s", status.SessionState)
	}
	if status.LastConnectAt == "" || status.LastHeartbeatAt == "" || status.LastDisconnectAt == "" || status.LastError != "boom" {
		t.Fatalf("unexpected lifecycle fields: %+v", status)
	}
}
