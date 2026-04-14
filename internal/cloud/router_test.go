package cloud

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cloudconnectors "agent-bridge/internal/cloud/connectors"
)

func TestServerAdminAndConnectorFlows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cloud.db")
	store, err := OpenStore(path, "cloud-master-key")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	org, workspace, project := seedCloudScope(t, store, ctx)

	bootstrapRaw, err := GenerateAPIToken()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateAPIToken(ctx, APIToken{
		OrgID:       org.ID,
		WorkspaceID: workspace.ID,
		ProjectID:   project.ID,
		Name:        "bootstrap",
		Kind:        "bootstrap",
		Scopes: []string{
			"cloud:read",
			"tokens:read",
			"tokens:write",
			"installations:read",
			"installations:write",
			"events:read",
			"audit:read",
		},
	}, bootstrapRaw); err != nil {
		t.Fatal(err)
	}

	slackAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/auth.test":
			if got := r.Header.Get("Authorization"); got != "Bearer slack-token" {
				t.Fatalf("unexpected auth header: %q", got)
			}
			_, _ = w.Write([]byte(`{"ok":true,"team":"Acme","user":"codencer"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer slackAPI.Close()

	server := NewServer(DefaultConfig(), store, cloudconnectors.NewRegistry(), nil)
	handler := server.Handler()

	t.Run("auth and status", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/cloud/v1/status", nil)
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected unauthorized status, got %d body=%s", rr.Code, rr.Body.String())
		}

		rr = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/api/cloud/v1/status", nil)
		req.Header.Set("Authorization", "Bearer "+bootstrapRaw)
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected status ok, got %d body=%s", rr.Code, rr.Body.String())
		}

		var status cloudStatusResponse
		if err := json.NewDecoder(rr.Body).Decode(&status); err != nil {
			t.Fatal(err)
		}
		if status.RelayComposed {
			t.Fatalf("expected relay handler to be absent in this router-only test")
		}
		if !containsProvider(status.ConnectorProviders, cloudconnectors.ProviderSlack) {
			t.Fatalf("expected slack connector provider in %v", status.ConnectorProviders)
		}
	})

	tokenResp := createAPITokenViaHTTP(t, handler, bootstrapRaw, org.ID, workspace.ID, project.ID)
	if tokenResp.Record.ID == "" || tokenResp.Token == "" {
		t.Fatalf("expected token creation response, got %+v", tokenResp)
	}

	installation := createInstallationViaHTTP(t, handler, tokenResp.Token, org.ID, workspace.ID, project.ID, slackAPI.URL)
	if installation.ID == "" {
		t.Fatal("expected installation id")
	}

	t.Run("disable and enable installation", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/cloud/v1/installations/"+installation.ID+"/disable", nil)
		req.Header.Set("Authorization", "Bearer "+tokenResp.Token)
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected disable ok, got %d body=%s", rr.Code, rr.Body.String())
		}
		var disabled ConnectorInstallation
		if err := json.NewDecoder(rr.Body).Decode(&disabled); err != nil {
			t.Fatal(err)
		}
		if disabled.Enabled || disabled.Status != "disabled" {
			t.Fatalf("expected disabled installation, got %+v", disabled)
		}

		rr = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/api/cloud/v1/installations/"+installation.ID+"/webhook", strings.NewReader(string([]byte(`{"type":"event_callback","event_id":"EvDisabled","event":{"type":"app_mention","user":"U1","channel":"C1","text":"please approve","ts":"1713096000.000100"}}`))))
		req.Header.Set("Content-Type", "application/json")
		ts := time.Now().Unix()
		req.Header.Set("X-Slack-Request-Timestamp", fmt.Sprintf("%d", ts))
		req.Header.Set("X-Slack-Signature", slackSignature("slack-secret", []byte(`{"type":"event_callback","event_id":"EvDisabled","event":{"type":"app_mention","user":"U1","channel":"C1","text":"please approve","ts":"1713096000.000100"}}`), ts))
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusConflict {
			t.Fatalf("expected webhook conflict when disabled, got %d body=%s", rr.Code, rr.Body.String())
		}

		rr = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/api/cloud/v1/installations/"+installation.ID+"/enable", nil)
		req.Header.Set("Authorization", "Bearer "+tokenResp.Token)
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected enable ok, got %d body=%s", rr.Code, rr.Body.String())
		}
		var enabled ConnectorInstallation
		if err := json.NewDecoder(rr.Body).Decode(&enabled); err != nil {
			t.Fatal(err)
		}
		if !enabled.Enabled || enabled.Status != "created" {
			t.Fatalf("expected re-enabled installation to reset to created, got %+v", enabled)
		}
	})

	t.Run("validate installation", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/cloud/v1/installations/"+installation.ID+"/validate", nil)
		req.Header.Set("Authorization", "Bearer "+tokenResp.Token)
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected validation ok, got %d body=%s", rr.Code, rr.Body.String())
		}

		var payload struct {
			Validation cloudconnectors.ValidationResult `json:"validation"`
			Status     cloudconnectors.ConnectorStatus  `json:"status"`
			Error      string                           `json:"error"`
		}
		if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if !payload.Validation.OK || !payload.Status.Ready {
			t.Fatalf("expected ready validation status, got %+v", payload)
		}
	})

	webhookBody := []byte(`{"type":"event_callback","event_id":"Ev123","event":{"type":"app_mention","user":"U1","channel":"C1","text":"please approve","ts":"1713096000.000100"}}`)
	webhookReq := httptest.NewRequest(http.MethodPost, "/api/cloud/v1/installations/"+installation.ID+"/webhook", strings.NewReader(string(webhookBody)))
	webhookReq.Header.Set("Content-Type", "application/json")
	ts := time.Now().Unix()
	webhookReq.Header.Set("X-Slack-Request-Timestamp", fmt.Sprintf("%d", ts))
	webhookReq.Header.Set("X-Slack-Signature", slackSignature("slack-secret", webhookBody, ts))

	webhookRR := httptest.NewRecorder()
	handler.ServeHTTP(webhookRR, webhookReq)
	if webhookRR.Code != http.StatusAccepted {
		t.Fatalf("expected webhook accepted, got %d body=%s", webhookRR.Code, webhookRR.Body.String())
	}

	var webhookPayload struct {
		Verification cloudconnectors.WebhookVerification `json:"verification"`
		Events       []cloudconnectors.Event             `json:"events"`
	}
	if err := json.NewDecoder(webhookRR.Body).Decode(&webhookPayload); err != nil {
		t.Fatal(err)
	}
	if !webhookPayload.Verification.Verified {
		t.Fatalf("expected verified webhook, got %+v", webhookPayload.Verification)
	}
	if len(webhookPayload.Events) != 1 || webhookPayload.Events[0].Kind != "app_mention" {
		t.Fatalf("unexpected webhook events: %+v", webhookPayload.Events)
	}

	t.Run("events and audit", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/cloud/v1/events?installation_id="+installation.ID, nil)
		req.Header.Set("Authorization", "Bearer "+tokenResp.Token)
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected events ok, got %d body=%s", rr.Code, rr.Body.String())
		}
		var events []ConnectorEvent
		if err := json.NewDecoder(rr.Body).Decode(&events); err != nil {
			t.Fatal(err)
		}
		if len(events) != 1 {
			t.Fatalf("expected one event, got %d", len(events))
		}
		if events[0].EventType != "app_mention" || events[0].Action != "mention" {
			t.Fatalf("unexpected stored event: %+v", events[0])
		}
		if events[0].SourceEventID != "Ev123" {
			t.Fatalf("expected event_id to win over delivery id, got %+v", events[0])
		}

		rr = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/api/cloud/v1/audit?limit=20", nil)
		req.Header.Set("Authorization", "Bearer "+tokenResp.Token)
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected audit ok, got %d body=%s", rr.Code, rr.Body.String())
		}
		var audit []CloudAuditEvent
		if err := json.NewDecoder(rr.Body).Decode(&audit); err != nil {
			t.Fatal(err)
		}
		gotActions := make([]string, 0, len(audit))
		for _, item := range audit {
			gotActions = append(gotActions, item.Action)
		}
		for _, want := range []string{"create_api_token", "create_installation", "disable_installation", "enable_installation", "validate_installation", "webhook_ingest"} {
			if !containsString(gotActions, want) {
				t.Fatalf("expected audit action %q in %v", want, gotActions)
			}
		}
	})
}

func seedCloudScope(t *testing.T, store *Store, ctx context.Context) (*Org, *Workspace, *Project) {
	t.Helper()
	org, err := store.CreateOrg(ctx, Org{Slug: "acme", Name: "Acme"})
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := store.CreateWorkspace(ctx, Workspace{OrgID: org.ID, Slug: "platform", Name: "Platform"})
	if err != nil {
		t.Fatal(err)
	}
	project, err := store.CreateProject(ctx, Project{OrgID: org.ID, WorkspaceID: workspace.ID, Slug: "core", Name: "Core"})
	if err != nil {
		t.Fatal(err)
	}
	return org, workspace, project
}

type tokenCreateResponse struct {
	Token  string   `json:"token"`
	Record APIToken `json:"record"`
}

func createAPITokenViaHTTP(t *testing.T, handler http.Handler, bearer, orgID, workspaceID, projectID string) tokenCreateResponse {
	t.Helper()
	body := map[string]any{
		"org_id":       orgID,
		"workspace_id": workspaceID,
		"project_id":   projectID,
		"name":         "installation-bootstrap",
		"kind":         "project",
		"scopes": []string{
			"installations:read",
			"installations:write",
			"events:read",
			"audit:read",
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/cloud/v1/tokens", strings.NewReader(string(raw)))
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected token creation success, got %d body=%s", rr.Code, rr.Body.String())
	}
	var resp tokenCreateResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	return resp
}

func createInstallationViaHTTP(t *testing.T, handler http.Handler, bearer, orgID, workspaceID, projectID, apiBaseURL string) ConnectorInstallation {
	t.Helper()
	body := map[string]any{
		"org_id":        orgID,
		"workspace_id":  workspaceID,
		"project_id":    projectID,
		"connector_key": "slack",
		"name":          "Slack CI",
		"config": map[string]string{
			"api_base_url": apiBaseURL,
		},
		"secrets": map[string]string{
			"token":          "slack-token",
			"webhook_secret": "slack-secret",
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/cloud/v1/installations", strings.NewReader(string(raw)))
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected installation creation success, got %d body=%s", rr.Code, rr.Body.String())
	}
	var installation ConnectorInstallation
	if err := json.NewDecoder(rr.Body).Decode(&installation); err != nil {
		t.Fatal(err)
	}
	return installation
}

func slackSignature(secret string, body []byte, ts int64) string {
	base := fmt.Sprintf("v0:%d:%s", ts, string(body))
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(base))
	return "v0=" + hex.EncodeToString(mac.Sum(nil))
}

func containsProvider(providers []cloudconnectors.Provider, want cloudconnectors.Provider) bool {
	for _, provider := range providers {
		if provider == want {
			return true
		}
	}
	return false
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
