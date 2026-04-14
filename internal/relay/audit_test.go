package relay_test

import (
	"context"
	"path/filepath"
	"testing"

	"agent-bridge/internal/relay"
)

func TestAuditEventsPersist(t *testing.T) {
	t.Parallel()

	store, err := relay.OpenStore(filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.AppendAudit(context.Background(), relay.AuditEvent{
		ActorType:        "planner",
		ActorID:          "token-a",
		Action:           "list_instances",
		Method:           "GET",
		TargetInstanceID: "inst-1",
		Outcome:          "ok",
	}); err != nil {
		t.Fatal(err)
	}

	events, err := store.ListAuditEvents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Action != "list_instances" {
		t.Fatalf("unexpected audit events: %+v", events)
	}
}

func TestAuditEventsLimitNewestFirst(t *testing.T) {
	t.Parallel()

	store, err := relay.OpenStore(filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	for _, action := range []string{"first", "second", "third"} {
		if err := store.AppendAudit(context.Background(), relay.AuditEvent{
			ActorType: "planner",
			Action:    action,
			Outcome:   "ok",
		}); err != nil {
			t.Fatal(err)
		}
	}

	events, err := store.ListAuditEventsLimit(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("expected two events, got %d", len(events))
	}
	if events[0].Action != "third" || events[1].Action != "second" {
		t.Fatalf("expected newest-first limited events, got %+v", events)
	}
}
