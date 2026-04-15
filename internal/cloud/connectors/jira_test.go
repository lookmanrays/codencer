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

func TestJiraConnectorValidateInstallationReportsProviderDetails(t *testing.T) {
	t.Parallel()

	connector := NewJiraConnector(nil)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"accountId":"acct-1","displayName":"Jane Doe","emailAddress":"jane@example.com","accountType":"atlassian"}`))
	}))
	defer srv.Close()

	connector.client = srv.Client()
	validation, err := connector.ValidateInstallation(context.Background(), InstallationConfig{
		APIBaseURL: srv.URL,
		Username:   "jane@example.com",
		Token:      "jira-token",
	})
	if err != nil {
		t.Fatalf("ValidateInstallation failed: %v", err)
	}
	if !validation.OK || validation.Identity != "jane@example.com" {
		t.Fatalf("unexpected validation: %#v", validation)
	}
	if got := validation.Details["username"]; got != "jane@example.com" {
		t.Fatalf("unexpected validation details: %#v", validation.Details)
	}
	if got := validation.Details["api_base_url"]; got != srv.URL {
		t.Fatalf("unexpected validation details: %#v", validation.Details)
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

func TestJiraClientTransitionIssue(t *testing.T) {
	t.Parallel()

	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/rest/api/3/issue/PROJ-17/transitions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		gotBody, _ = ioReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := &JiraClient{
		BaseURL:    srv.URL,
		Email:      "tester@example.com",
		APIToken:   "jira-token",
		HTTPClient: srv.Client(),
	}
	res, err := client.TransitionIssue(context.Background(), "PROJ-17", "31")
	if err != nil {
		t.Fatalf("TransitionIssue failed: %v", err)
	}
	if !strings.Contains(string(gotBody), `"id":"31"`) {
		t.Fatalf("transition payload not posted: %s", string(gotBody))
	}
	if res.IssueKey != "PROJ-17" || res.TransitionID != "31" {
		t.Fatalf("unexpected transition result: %#v", res)
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

func TestJiraConnectorStatusIncludesPollingMode(t *testing.T) {
	t.Parallel()

	connector := NewJiraConnector(nil)
	status := connector.DeriveStatus(ValidationResult{
		Provider: ProviderJira,
		OK:       true,
		Identity: "jane@example.com",
	}, WebhookVerification{})
	if got := status.Details["polling_mode"]; got != "polling-first" {
		t.Fatalf("unexpected status details: %#v", status.Details)
	}
	if got := status.Details["webhook_ingest"]; got != "disabled" {
		t.Fatalf("unexpected status details: %#v", status.Details)
	}
}

func ioReadAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}
