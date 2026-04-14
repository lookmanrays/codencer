package relay

import (
	"testing"
	"time"

	"agent-bridge/internal/relayproto"
)

func TestHubExpiresStaleSessions(t *testing.T) {
	hub := NewHub(10 * time.Millisecond)
	session := &session{
		connectorID: "connector-1",
		instanceIDs: map[string]struct{}{"inst-1": {}},
		pending:     make(map[string]chan relayproto.CommandResponse),
		lastSeenAt:  time.Now().UTC().Add(-time.Second),
	}
	hub.RegisterConnector(session)
	hub.Register("inst-1", session)

	time.Sleep(15 * time.Millisecond)

	if got := hub.Get("inst-1"); got != nil {
		t.Fatalf("expected stale instance session to expire, got %+v", got)
	}
	if got := hub.Connector("connector-1"); got != nil {
		t.Fatalf("expected stale connector session to expire, got %+v", got)
	}
}
