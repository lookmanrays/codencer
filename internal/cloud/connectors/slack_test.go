package connectors

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestSlackClientValidateUsesAuthTest(t *testing.T) {
	t.Parallel()

	var gotAuth string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/api/auth.test" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		gotBody, _ = ioReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"team_id":"T123","team":"team","user_id":"U123","user":"bot"}`))
	}))
	defer srv.Close()

	client := &SlackClient{
		BaseURL:    srv.URL,
		BotToken:   "xoxb-token",
		HTTPClient: srv.Client(),
	}
	if err := client.Validate(context.Background()); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if gotAuth != "Bearer xoxb-token" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if len(gotBody) != 0 {
		t.Fatalf("auth.test should send empty body, got: %s", string(gotBody))
	}
}

func TestSlackClientPostMessage(t *testing.T) {
	t.Parallel()

	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/chat.postMessage" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer xoxb-token" {
			t.Fatalf("missing auth header: %q", r.Header.Get("Authorization"))
		}
		decoded, _ := ioReadAll(r.Body)
		_ = json.Unmarshal(decoded, &body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"channel":"C123","ts":"1713096000.000100","text":"Hello"}`))
	}))
	defer srv.Close()

	client := &SlackClient{
		BaseURL:    srv.URL,
		BotToken:   "xoxb-token",
		HTTPClient: srv.Client(),
	}
	res, err := client.PostMessage(context.Background(), "C123", "Hello", SlackPostMessageOptions{ThreadTS: "1713095999.000001"})
	if err != nil {
		t.Fatalf("PostMessage failed: %v", err)
	}
	if body["channel"] != "C123" || body["text"] != "Hello" || body["thread_ts"] != "1713095999.000001" {
		t.Fatalf("unexpected postMessage body: %#v", body)
	}
	if res.Channel != "C123" || res.TS != "1713096000.000100" {
		t.Fatalf("unexpected result: %#v", res)
	}
}

func TestSlackClientUpdateMessage(t *testing.T) {
	t.Parallel()

	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/chat.update" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer xoxb-token" {
			t.Fatalf("missing auth header: %q", r.Header.Get("Authorization"))
		}
		decoded, _ := ioReadAll(r.Body)
		_ = json.Unmarshal(decoded, &body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"channel":"C123","ts":"1713096000.000100","text":"Updated"}`))
	}))
	defer srv.Close()

	client := &SlackClient{
		BaseURL:    srv.URL,
		BotToken:   "xoxb-token",
		HTTPClient: srv.Client(),
	}
	res, err := client.UpdateMessage(context.Background(), "C123", "1713096000.000100", "Updated")
	if err != nil {
		t.Fatalf("UpdateMessage failed: %v", err)
	}
	if body["channel"] != "C123" || body["ts"] != "1713096000.000100" || body["text"] != "Updated" {
		t.Fatalf("unexpected updateMessage body: %#v", body)
	}
	if res.Channel != "C123" || res.TS != "1713096000.000100" {
		t.Fatalf("unexpected result: %#v", res)
	}
}

func TestSlackNormalizeEventCallback(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"type":"event_callback",
		"event_id":"Ev123",
		"event":{
			"type":"app_mention",
			"user":"U1",
			"channel":"C1",
			"text":"please approve",
			"ts":"1713096000.000100"
		}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/slack", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	setSlackSignature(req.Header, "slack-secret", raw, 1713096000)

	events, err := NormalizeSlackRequest(req, "slack-secret", time.Unix(1713096000, 0))
	if err != nil {
		t.Fatalf("NormalizeSlackRequest failed: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected one event, got %d", len(events))
	}
	ev := events[0]
	if ev.Kind != "app_mention" || ev.Action != "mention" || ev.ChannelID != "C1" || ev.UserID != "U1" {
		t.Fatalf("unexpected normalized event: %#v", ev)
	}
}

func TestSlackNormalizeInteractiveApproval(t *testing.T) {
	t.Parallel()

	payload := `{"type":"block_actions","callback_id":"gate-1","user":{"id":"U2"},"channel":{"id":"C2"},"message":{"ts":"1713096000.000100","thread_ts":"1713095999.000001"},"actions":[{"action_id":"approve","value":"approve"}]}`
	form := url.Values{}
	form.Set("payload", payload)
	raw := []byte(form.Encode())
	req := httptest.NewRequest(http.MethodPost, "/slack", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	setSlackSignature(req.Header, "slack-secret", raw, 1713096000)

	events, err := NormalizeSlackRequest(req, "slack-secret", time.Unix(1713096000, 0))
	if err != nil {
		t.Fatalf("NormalizeSlackRequest failed: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected one event, got %d", len(events))
	}
	ev := events[0]
	if ev.Kind != "block_actions" || ev.ApprovalAction != "approve" || ev.CallbackID != "gate-1" {
		t.Fatalf("unexpected interactive normalization: %#v", ev)
	}
}

func TestSlackNormalizeSlashCommand(t *testing.T) {
	t.Parallel()

	form := url.Values{}
	form.Set("command", "/codencer")
	form.Set("channel_id", "C3")
	form.Set("user_id", "U3")
	form.Set("text", "status")
	form.Set("response_url", "https://slack.example/response")
	raw := []byte(form.Encode())
	req := httptest.NewRequest(http.MethodPost, "/slack", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	setSlackSignature(req.Header, "slack-secret", raw, 1713096000)

	events, err := NormalizeSlackRequest(req, "slack-secret", time.Unix(1713096000, 0))
	if err != nil {
		t.Fatalf("NormalizeSlackRequest failed: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected one event, got %d", len(events))
	}
	ev := events[0]
	if ev.Kind != "slash_command" || ev.Command != "/codencer" || ev.Text != "status" {
		t.Fatalf("unexpected slash command normalization: %#v", ev)
	}
}

func TestSlackRejectsBadSignature(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"type":"event_callback","event":{"type":"message"}}`)
	req := httptest.NewRequest(http.MethodPost, "/slack", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Slack-Request-Timestamp", "1713096000")
	req.Header.Set("X-Slack-Signature", "v0=bad")
	if _, err := NormalizeSlackRequest(req, "slack-secret", time.Unix(1713096000, 0)); err == nil {
		t.Fatal("expected signature failure")
	}
}

func setSlackSignature(h http.Header, secret string, raw []byte, ts int64) {
	base := fmt.Sprintf("v0:%d:%s", ts, string(raw))
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(base))
	h.Set("X-Slack-Request-Timestamp", fmt.Sprintf("%d", ts))
	h.Set("X-Slack-Signature", "v0="+hex.EncodeToString(mac.Sum(nil)))
}
