package connectors

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLinearClientValidateUsesBearerToken(t *testing.T) {
	t.Parallel()

	var gotAuth string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/graphql" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		gotBody, _ = ioReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"viewer":{"id":"u-1","name":"Coder","email":"coder@example.com"}}}`))
	}))
	defer srv.Close()

	client := &LinearClient{
		BaseURL:    srv.URL,
		Token:      "linear-token",
		HTTPClient: srv.Client(),
	}
	if err := client.Validate(context.Background()); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if gotAuth != "Bearer linear-token" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if !bytes.Contains(gotBody, []byte("viewer")) {
		t.Fatalf("viewer query not sent: %s", string(gotBody))
	}
}

func TestLinearClientCreateIssue(t *testing.T) {
	t.Parallel()

	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/graphql" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		gotBody, _ = ioReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"issueCreate":{"success":true,"issue":{"id":"issue-1","title":"Ship it","url":"https://linear.example/issue/issue-1"}}}}`))
	}))
	defer srv.Close()

	client := &LinearClient{
		BaseURL:    srv.URL,
		Token:      "linear-token",
		HTTPClient: srv.Client(),
	}
	issue, err := client.CreateIssue(context.Background(), "team-1", "Ship it", "Detailed context")
	if err != nil {
		t.Fatalf("CreateIssue failed: %v", err)
	}
	if !bytes.Contains(gotBody, []byte("issueCreate")) || !bytes.Contains(gotBody, []byte(`"teamId":"team-1"`)) {
		t.Fatalf("mutation payload missing fields: %s", string(gotBody))
	}
	if issue.ID != "issue-1" || issue.Title != "Ship it" {
		t.Fatalf("unexpected issue result: %#v", issue)
	}
}

func TestLinearWebhookSignatureAndNormalization(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"action":"create",
		"type":"Issue",
		"actor":{"name":"Ada"},
		"createdAt":"2026-04-14T12:00:00Z",
		"data":{"id":"issue-1","title":"Ship it","body":"Ready to go","url":"https://linear.example/issue/issue-1"},
		"url":"https://linear.example/issue/issue-1",
		"webhookTimestamp":1713096000000,
		"webhookId":"wh-1"
	}`)
	secret := "linear-secret"
	sig := linearSignature(secret, raw)
	req := httptest.NewRequest(http.MethodPost, "/linear", bytes.NewReader(raw))
	req.Header.Set("Linear-Signature", sig)

	ev, err := NormalizeLinearWebhookRequest(req, secret, time.UnixMilli(1713096000000))
	if err != nil {
		t.Fatalf("NormalizeLinearWebhookRequest failed: %v", err)
	}
	if ev.Provider != "linear" || ev.Kind != "issue" || ev.Action != "create" {
		t.Fatalf("unexpected event metadata: %#v", ev)
	}
	if ev.EntityID != "issue-1" || ev.Title != "Ship it" || ev.Actor != "Ada" {
		t.Fatalf("unexpected normalized payload: %#v", ev)
	}
}

func TestLinearWebhookRejectsBadSignature(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"type":"Issue","action":"create","webhookTimestamp":1713096000000}`)
	req := httptest.NewRequest(http.MethodPost, "/linear", bytes.NewReader(raw))
	req.Header.Set("Linear-Signature", "bad-signature")

	if _, err := NormalizeLinearWebhookRequest(req, "linear-secret", time.UnixMilli(1713096000000)); err == nil {
		t.Fatal("expected signature failure")
	}
}

func linearSignature(secret string, raw []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(raw)
	return hex.EncodeToString(mac.Sum(nil))
}
