package relay_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"path/filepath"
	"testing"

	"agent-bridge/internal/relay"
	"agent-bridge/internal/relayproto"
)

func TestEnrollmentService_CreateAndConsumeOneTimeToken(t *testing.T) {
	t.Parallel()

	store, err := relay.OpenStore(filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	svc := relay.NewEnrollmentService(&relay.Config{ChallengeTTLSeconds: 30}, store)
	_, secret, err := svc.CreateToken(context.Background(), "planner", "local-dev", 0)
	if err != nil {
		t.Fatal(err)
	}

	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	req := relayproto.EnrollmentRequest{
		EnrollmentToken: secret,
		Label:           "connector-a",
		PublicKey:       base64.StdEncoding.EncodeToString(pub),
	}
	resp, record, err := svc.Enroll(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.ConnectorID == "" || resp.MachineID == "" || record == nil {
		t.Fatalf("expected assigned connector identity, got resp=%+v record=%+v", resp, record)
	}

	_, _, err = svc.Enroll(context.Background(), req)
	if err == nil || err.Error() != "enrollment token already used" {
		t.Fatalf("expected one-time token to be consumed, got %v", err)
	}
}
