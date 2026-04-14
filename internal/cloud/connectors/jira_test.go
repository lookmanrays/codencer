package connectors

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJiraClientValidateUsesBasicAuth(t *testing.T) {
	t.Parallel()

	client := &JiraClient{
		BaseURL:  "https://jira.example",
		Email:    "tester@example.com",
		APIToken: "jira-token",
	}
	if client.SupportsWebhook() {
		t.Fatal("jira should remain polling-first in this run")
	}

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/rest/api/3/myself" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"accountId":"acct-1"}`))
	}))
	defer srv.Close()

	client.HTTPClient = srv.Client()
	client.BaseURL = srv.URL
	if err := client.Validate(context.Background()); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("tester@example.com:jira-token"))
	if gotAuth != want {
		t.Fatalf("Authorization = %q, want %q", gotAuth, want)
	}
}

func TestJiraClientAddComment(t *testing.T) {
	t.Parallel()

	var body []byte
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/rest/api/3/issue/PROJ-17/comment" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		body, _ = ioReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"10001",
			"self":"https://jira.example/rest/api/3/issue/PROJ-17/comment/10001",
			"body":{"type":"doc","text":"posted"},
			"author":{"displayName":"Codex"},
			"created":"2026-04-14T12:34:56.000+0000",
			"visibility":{"value":"Administrators"}
		}`))
	}))
	defer srv.Close()

	client := &JiraClient{
		BaseURL:    srv.URL,
		Email:      "tester@example.com",
		APIToken:   "jira-token",
		HTTPClient: srv.Client(),
	}
	res, err := client.AddComment(context.Background(), "PROJ-17", "hello jira")
	if err != nil {
		t.Fatalf("AddComment failed: %v", err)
	}
	if want := "Basic " + base64.StdEncoding.EncodeToString([]byte("tester@example.com:jira-token")); gotAuth != want {
		t.Fatalf("Authorization = %q, want %q", gotAuth, want)
	}
	if !strings.Contains(string(body), `"body":"hello jira"`) {
		t.Fatalf("comment body not posted: %s", string(body))
	}
	if res.CommentID != "10001" || res.IssueKey != "PROJ-17" || res.Author != "Codex" {
		t.Fatalf("unexpected result: %#v", res)
	}
}

func TestNormalizeJiraIssuePayload(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"key":"PROJ-17",
		"self":"https://jira.example/rest/api/3/issue/PROJ-17",
		"fields":{
			"summary":"Fix the bug",
			"status":{"name":"In Progress"},
			"assignee":{"displayName":"Ada Lovelace"},
			"updated":"2026-04-14T12:34:56.000+0000"
		}
	}`)
	ev, err := NormalizeJiraIssuePayload(raw)
	if err != nil {
		t.Fatalf("NormalizeJiraIssuePayload failed: %v", err)
	}
	if ev.Provider != "jira" || ev.Kind != "issue_snapshot" || ev.Action != "snapshot" {
		t.Fatalf("unexpected event metadata: %#v", ev)
	}
	if ev.Issue.Key != "PROJ-17" || ev.Issue.Summary != "Fix the bug" || ev.Issue.Status != "In Progress" {
		t.Fatalf("unexpected issue snapshot: %#v", ev.Issue)
	}
	if ev.Issue.Assignee != "Ada Lovelace" {
		t.Fatalf("unexpected assignee: %#v", ev.Issue)
	}
}

func TestNormalizeJiraCommentPayload(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"issue":{
			"key":"PROJ-17",
			"self":"https://jira.example/rest/api/3/issue/PROJ-17",
			"fields":{"summary":"Fix the bug","status":{"name":"Done"}}
		},
		"comment":{
			"body":{"type":"doc","text":"Looks good"},
			"author":{"displayName":"Grace Hopper"}
		}
	}`)
	ev, err := NormalizeJiraIssuePayload(raw)
	if err != nil {
		t.Fatalf("NormalizeJiraIssuePayload failed: %v", err)
	}
	if ev.Kind != "issue_comment" || ev.Action != "commented" {
		t.Fatalf("unexpected comment event: %#v", ev)
	}
	if ev.Comment != "Looks good" || ev.Author != "Grace Hopper" {
		t.Fatalf("unexpected comment normalization: %#v", ev)
	}
}

func ioReadAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}
