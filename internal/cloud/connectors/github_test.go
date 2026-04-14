package connectors

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGitHubConnectorValidateVerifyNormalizeAndWrite(t *testing.T) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/user":
			if got := r.Header.Get("Authorization"); got != "Bearer token-123" {
				t.Fatalf("unexpected auth header: %q", got)
			}
			_, _ = w.Write([]byte(`{"login":"octocat","id":42,"html_url":"https://github.com/octocat"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/repos/acme/widgets/issues/12/comments":
			if got := r.Header.Get("Authorization"); got != "Bearer token-123" {
				t.Fatalf("unexpected auth header: %q", got)
			}
			if got := r.Header.Get("Accept"); !strings.Contains(got, "github") {
				t.Fatalf("unexpected accept header: %q", got)
			}
			if got := r.URL.RawPath; got != "" {
				t.Fatalf("unexpected raw path: %q", got)
			}
			_, _ = w.Write([]byte(`{"id":99,"html_url":"https://github.local/comment/99"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	connector := NewGitHubConnector(server.Client())
	cfg := InstallationConfig{
		APIBaseURL:    server.URL,
		Token:         "token-123",
		WebhookSecret: "webhook-secret",
	}

	validation, err := connector.ValidateInstallation(context.Background(), cfg)
	if err != nil {
		t.Fatalf("ValidateInstallation failed: %v", err)
	}
	if !validation.OK || validation.Identity != "octocat" {
		t.Fatalf("unexpected validation: %#v", validation)
	}

	body := []byte(`{"action":"opened","issue":{"id":1,"number":12,"title":"Bug","html_url":"https://github.local/issues/12"},"repository":{"full_name":"acme/widgets"},"sender":{"login":"alice"}}`)
	sig := githubSignature(cfg.WebhookSecret, body)
	hdr := http.Header{}
	hdr.Set("X-Hub-Signature-256", sig)
	hdr.Set("X-GitHub-Event", "issues")
	hdr.Set("X-GitHub-Delivery", "delivery-1")
	verify, err := connector.VerifyWebhook(hdr, body, cfg)
	if err != nil || !verify.Verified {
		t.Fatalf("unexpected webhook verification: %#v err=%v", verify, err)
	}

	eventHeaders := http.Header{}
	eventHeaders.Set("X-GitHub-Event", "issues")
	events, err := connector.NormalizeEvent(eventHeaders, body, cfg)
	if err != nil {
		t.Fatalf("NormalizeEvent failed: %v", err)
	}
	if len(events) != 1 || events[0].Kind != "issue.opened" || events[0].Repository != "acme/widgets" {
		t.Fatalf("unexpected normalized event: %#v", events)
	}

	result, err := connector.ExecuteAction(context.Background(), ActionRequest{
		Action:      ActionGitHubCreateIssueComment,
		Repository:  "acme/widgets",
		IssueNumber: 12,
		Body:        "Hello from Codencer",
	}, cfg)
	if err != nil {
		t.Fatalf("ExecuteAction failed: %v", err)
	}
	if !result.OK || result.ExternalID != "99" {
		t.Fatalf("unexpected action result: %#v", result)
	}

	status := connector.DeriveStatus(validation, verify)
	if !status.Ready || status.State == "" {
		t.Fatalf("unexpected status: %#v", status)
	}
}

func TestGitHubConnectorRejectsInvalidWebhookSignature(t *testing.T) {
	connector := NewGitHubConnector(nil)
	hdr := http.Header{}
	hdr.Set("X-Hub-Signature-256", "sha256=deadbeef")
	_, err := connector.VerifyWebhook(hdr, []byte("payload"), InstallationConfig{WebhookSecret: "secret"})
	if err == nil {
		t.Fatalf("expected signature mismatch error")
	}
}

func githubSignature(secret string, body []byte) string {
	sum := hmac.New(sha256.New, []byte(secret))
	_, _ = sum.Write(body)
	return "sha256=" + hex.EncodeToString(sum.Sum(nil))
}
