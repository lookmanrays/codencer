package connectors

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGitLabConnectorValidateVerifyNormalizeAndWrite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/user":
			if got := r.Header.Get("PRIVATE-TOKEN"); got != "token-abc" {
				t.Fatalf("unexpected token header: %q", got)
			}
			_, _ = w.Write([]byte(`{"id":7,"username":"gitlab-user","name":"GitLab User","web_url":"https://gitlab.local/u/gitlab-user"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/group/repo/issues/11/notes":
			if got := r.Header.Get("PRIVATE-TOKEN"); got != "token-abc" {
				t.Fatalf("unexpected token header: %q", got)
			}
			if got := r.URL.RawPath; got != "/api/v4/projects/group%2Frepo/issues/11/notes" {
				t.Fatalf("unexpected raw path: %q", got)
			}
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm failed: %v", err)
			}
			if got := r.Form.Get("body"); got != "Hello from Codencer" {
				t.Fatalf("unexpected body: %q", got)
			}
			_, _ = w.Write([]byte(`{"id":88,"body":"Hello from Codencer","web_url":"https://gitlab.local/group/repo/-/issues/11#note_88"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/group/repo/issues":
			if got := r.Header.Get("PRIVATE-TOKEN"); got != "token-abc" {
				t.Fatalf("unexpected token header: %q", got)
			}
			if got := r.URL.RawPath; got != "/api/v4/projects/group%2Frepo/issues" {
				t.Fatalf("unexpected raw path: %q", got)
			}
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm failed: %v", err)
			}
			if got := r.Form.Get("title"); got != "Ship it" {
				t.Fatalf("unexpected title: %q", got)
			}
			if got := r.Form.Get("description"); got != "Planned from Codencer" {
				t.Fatalf("unexpected description: %q", got)
			}
			_, _ = w.Write([]byte(`{"id":101,"iid":12,"web_url":"https://gitlab.local/group/repo/-/issues/12"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	connector := NewGitLabConnector(server.Client())
	cfg := InstallationConfig{
		APIBaseURL:    server.URL + "/api/v4",
		Token:         "token-abc",
		WebhookSecret: "gitlab-secret",
	}

	validation, err := connector.ValidateInstallation(context.Background(), cfg)
	if err != nil {
		t.Fatalf("ValidateInstallation failed: %v", err)
	}
	if !validation.OK || validation.Identity != "gitlab-user" {
		t.Fatalf("unexpected validation: %#v", validation)
	}
	if got := validation.Details["name"]; got != "GitLab User" {
		t.Fatalf("unexpected validation details: %#v", validation.Details)
	}

	body := []byte(`{"object_kind":"issue","event_type":"issue","user_username":"alice","project":{"path_with_namespace":"group/repo","web_url":"https://gitlab.local/group/repo"},"object_attributes":{"action":"open","iid":11,"title":"Bug","url":"https://gitlab.local/group/repo/-/issues/11"}}`)
	hdr := http.Header{}
	hdr.Set("X-Gitlab-Token", "gitlab-secret")
	hdr.Set("X-Gitlab-Event", "Issue Hook")
	hdr.Set("X-Request-Id", "delivery-1")
	verify, err := connector.VerifyWebhook(hdr, body, cfg)
	if err != nil || !verify.Verified {
		t.Fatalf("unexpected webhook verification: %#v err=%v", verify, err)
	}

	eventHeaders := http.Header{}
	eventHeaders.Set("X-Gitlab-Event", "Issue Hook")
	events, err := connector.NormalizeEvent(eventHeaders, body, cfg)
	if err != nil {
		t.Fatalf("NormalizeEvent failed: %v", err)
	}
	if len(events) != 1 || events[0].Kind != "issue.opened" || events[0].Project != "group/repo" {
		t.Fatalf("unexpected normalized event: %#v", events)
	}

	result, err := connector.ExecuteAction(context.Background(), ActionRequest{
		Action:      ActionGitLabCreateIssueNote,
		Project:     "group/repo",
		IssueNumber: 11,
		Body:        "Hello from Codencer",
	}, cfg)
	if err != nil {
		t.Fatalf("ExecuteAction failed: %v", err)
	}
	if !result.OK || result.ExternalID != "88" {
		t.Fatalf("unexpected action result: %#v", result)
	}

	issueResult, err := connector.ExecuteAction(context.Background(), ActionRequest{
		Action:      ActionGitLabCreateIssue,
		Project:     "group/repo",
		Title:       "Ship it",
		Description: "Planned from Codencer",
	}, cfg)
	if err != nil {
		t.Fatalf("CreateIssue failed: %v", err)
	}
	if !issueResult.OK || issueResult.ExternalID != "12" || issueResult.URL != "https://gitlab.local/group/repo/-/issues/12" {
		t.Fatalf("unexpected create issue result: %#v", issueResult)
	}

	status := connector.DeriveStatus(validation, verify)
	if !status.Ready || status.State == "" {
		t.Fatalf("unexpected status: %#v", status)
	}
}

func TestGitLabConnectorRejectsInvalidWebhookToken(t *testing.T) {
	connector := NewGitLabConnector(nil)
	hdr := http.Header{}
	hdr.Set("X-Gitlab-Token", "wrong")
	_, err := connector.VerifyWebhook(hdr, []byte("payload"), InstallationConfig{WebhookSecret: "secret"})
	if err == nil {
		t.Fatalf("expected token mismatch error")
	}
}

func TestGitLabConnectorNormalizesMergeRequestAndPush(t *testing.T) {
	connector := NewGitLabConnector(nil)
	mrBody := []byte(`{"object_kind":"merge_request","user":{"username":"bob"},"project":{"path_with_namespace":"group/repo","web_url":"https://gitlab.local/group/repo"},"object_attributes":{"action":"merge","iid":9,"title":"Add feature","url":"https://gitlab.local/group/repo/-/merge_requests/9"}}`)
	mrHeaders := http.Header{}
	mrHeaders.Set("X-Gitlab-Event", "Merge Request Hook")
	mrEvents, err := connector.NormalizeEvent(mrHeaders, mrBody, InstallationConfig{})
	if err != nil {
		t.Fatalf("NormalizeEvent merge request failed: %v", err)
	}
	if len(mrEvents) != 1 || mrEvents[0].Kind != "merge_request.merged" {
		t.Fatalf("unexpected merge request event: %#v", mrEvents)
	}

	pushBody := []byte(`{"object_kind":"push","user_username":"bob","ref":"refs/heads/main","before":"abc","after":"def","project":{"path_with_namespace":"group/repo","web_url":"https://gitlab.local/group/repo"}}`)
	pushHeaders := http.Header{}
	pushHeaders.Set("X-Gitlab-Event", "Push Hook")
	pushEvents, err := connector.NormalizeEvent(pushHeaders, pushBody, InstallationConfig{})
	if err != nil {
		t.Fatalf("NormalizeEvent push failed: %v", err)
	}
	if len(pushEvents) != 1 || pushEvents[0].Kind != "push.pushed" || !strings.Contains(pushEvents[0].Details["ref"], "main") {
		t.Fatalf("unexpected push event: %#v", pushEvents)
	}
}
