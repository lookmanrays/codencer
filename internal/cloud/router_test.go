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

	bridgeapp "agent-bridge/internal/app"
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
		if status.Version != bridgeapp.Version {
			t.Fatalf("expected cloud status version %q, got %q", bridgeapp.Version, status.Version)
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

func TestTokenRevokeRequiresAuthorizedScopeAndRevokedTokenFailsAcrossCloudSurfaces(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "cloud.db"), "cloud-master-key")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	org, err := store.CreateOrg(ctx, Org{Slug: "acme", Name: "Acme"})
	if err != nil {
		t.Fatal(err)
	}
	workspaceA, err := store.CreateWorkspace(ctx, Workspace{OrgID: org.ID, Slug: "platform", Name: "Platform"})
	if err != nil {
		t.Fatal(err)
	}
	projectA, err := store.CreateProject(ctx, Project{OrgID: org.ID, WorkspaceID: workspaceA.ID, Slug: "core", Name: "Core"})
	if err != nil {
		t.Fatal(err)
	}
	workspaceB, err := store.CreateWorkspace(ctx, Workspace{OrgID: org.ID, Slug: "ops", Name: "Ops"})
	if err != nil {
		t.Fatal(err)
	}
	projectB, err := store.CreateProject(ctx, Project{OrgID: org.ID, WorkspaceID: workspaceB.ID, Slug: "infra", Name: "Infra"})
	if err != nil {
		t.Fatal(err)
	}

	adminRaw, err := GenerateAPIToken()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateAPIToken(ctx, APIToken{
		OrgID:  org.ID,
		Name:   "org-admin",
		Scopes: []string{"cloud:read", "tokens:write"},
	}, adminRaw); err != nil {
		t.Fatal(err)
	}

	workspaceRaw, err := GenerateAPIToken()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateAPIToken(ctx, APIToken{
		OrgID:       org.ID,
		WorkspaceID: workspaceA.ID,
		ProjectID:   projectA.ID,
		Name:        "workspace-admin",
		Scopes:      []string{"cloud:read", "tokens:write"},
	}, workspaceRaw); err != nil {
		t.Fatal(err)
	}

	targetRaw, err := GenerateAPIToken()
	if err != nil {
		t.Fatal(err)
	}
	targetRecord, err := store.CreateAPIToken(ctx, APIToken{
		OrgID:       org.ID,
		WorkspaceID: workspaceB.ID,
		ProjectID:   projectB.ID,
		Name:        "target",
		Scopes:      []string{"cloud:read", "runtime_instances:read"},
	}, targetRaw)
	if err != nil {
		t.Fatal(err)
	}

	server := NewServer(DefaultConfig(), store, nil, nil)
	handler := server.Handler()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/cloud/v1/tokens/"+targetRecord.ID+"/revoke", nil)
	req.Header.Set("Authorization", "Bearer "+workspaceRaw)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected workspace-scoped revoke to be forbidden, got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/cloud/v1/status", nil)
	req.Header.Set("Authorization", "Bearer "+targetRaw)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected target token to remain active after denied revoke, got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/cloud/v1/tokens/"+targetRecord.ID+"/revoke", nil)
	req.Header.Set("Authorization", "Bearer "+adminRaw)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected org-scoped revoke to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/cloud/v1/status", nil)
	req.Header.Set("Authorization", "Bearer "+targetRaw)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected revoked token to fail cloud HTTP auth, got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/cloud/v1/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":"1","method":"initialize","params":{"protocolVersion":"2025-11-25"}}`))
	req.Header.Set("Authorization", "Bearer "+targetRaw)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected revoked token to fail cloud MCP auth, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestEventsListingRespectsTokenTenantScope(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "cloud.db"), "cloud-master-key")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	org, err := store.CreateOrg(ctx, Org{Slug: "acme", Name: "Acme"})
	if err != nil {
		t.Fatal(err)
	}
	workspaceA, err := store.CreateWorkspace(ctx, Workspace{OrgID: org.ID, Slug: "platform", Name: "Platform"})
	if err != nil {
		t.Fatal(err)
	}
	projectA, err := store.CreateProject(ctx, Project{OrgID: org.ID, WorkspaceID: workspaceA.ID, Slug: "core", Name: "Core"})
	if err != nil {
		t.Fatal(err)
	}
	workspaceB, err := store.CreateWorkspace(ctx, Workspace{OrgID: org.ID, Slug: "ops", Name: "Ops"})
	if err != nil {
		t.Fatal(err)
	}
	projectB, err := store.CreateProject(ctx, Project{OrgID: org.ID, WorkspaceID: workspaceB.ID, Slug: "infra", Name: "Infra"})
	if err != nil {
		t.Fatal(err)
	}

	readerRaw, err := GenerateAPIToken()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateAPIToken(ctx, APIToken{
		OrgID:       org.ID,
		WorkspaceID: workspaceA.ID,
		ProjectID:   projectA.ID,
		Name:        "project-reader",
		Scopes:      []string{"events:read"},
	}, readerRaw); err != nil {
		t.Fatal(err)
	}

	installA, err := store.CreateConnectorInstallation(ctx, ConnectorInstallation{
		OrgID:        org.ID,
		WorkspaceID:  workspaceA.ID,
		ProjectID:    projectA.ID,
		ConnectorKey: "slack",
		Name:         "Project A",
		Status:       "active",
		Enabled:      true,
		Health:       "healthy",
	})
	if err != nil {
		t.Fatal(err)
	}
	installB, err := store.CreateConnectorInstallation(ctx, ConnectorInstallation{
		OrgID:        org.ID,
		WorkspaceID:  workspaceB.ID,
		ProjectID:    projectB.ID,
		ConnectorKey: "slack",
		Name:         "Project B",
		Status:       "active",
		Enabled:      true,
		Health:       "healthy",
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.CreateConnectorEvent(ctx, ConnectorEvent{
		ID:             "evt-a",
		InstallationID: installA.ID,
		SourceEventID:  "A-1",
		EventType:      "app_mention",
		Status:         "accepted",
		ReceivedAt:     time.Now().UTC(),
		OccurredAt:     time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateConnectorEvent(ctx, ConnectorEvent{
		ID:             "evt-b",
		InstallationID: installB.ID,
		SourceEventID:  "B-1",
		EventType:      "push",
		Status:         "accepted",
		ReceivedAt:     time.Now().UTC().Add(1 * time.Second),
		OccurredAt:     time.Now().UTC().Add(1 * time.Second),
	}); err != nil {
		t.Fatal(err)
	}

	server := NewServer(DefaultConfig(), store, nil, nil)
	handler := server.Handler()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/cloud/v1/events?limit=20", nil)
	req.Header.Set("Authorization", "Bearer "+readerRaw)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected scoped events listing ok, got %d body=%s", rr.Code, rr.Body.String())
	}
	var events []ConnectorEvent
	if err := json.NewDecoder(rr.Body).Decode(&events); err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].InstallationID != installA.ID {
		t.Fatalf("expected only project-scoped events, got %+v", events)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/cloud/v1/events?installation_id="+installB.ID, nil)
	req.Header.Set("Authorization", "Bearer "+readerRaw)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected scoped token to be denied on foreign installation events, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestAuditListingRespectsProjectScope(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "cloud.db"), "cloud-master-key")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	org, workspace, project := seedCloudScope(t, store, ctx)
	siblingProject, err := store.CreateProject(ctx, Project{OrgID: org.ID, WorkspaceID: workspace.ID, Slug: "sibling", Name: "Sibling"})
	if err != nil {
		t.Fatal(err)
	}

	readerRaw, err := GenerateAPIToken()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateAPIToken(ctx, APIToken{
		OrgID:       org.ID,
		WorkspaceID: workspace.ID,
		ProjectID:   project.ID,
		Name:        "audit-reader",
		Scopes:      []string{"audit:read"},
	}, readerRaw); err != nil {
		t.Fatal(err)
	}

	for _, event := range []CloudAuditEvent{
		{ID: "audit-org", ActorType: "service", Action: "org_only", OrgID: org.ID, Outcome: "ok", CreatedAt: time.Now().UTC()},
		{ID: "audit-project", ActorType: "service", Action: "project_only", OrgID: org.ID, WorkspaceID: workspace.ID, ProjectID: project.ID, Outcome: "ok", CreatedAt: time.Now().UTC().Add(1 * time.Second)},
		{ID: "audit-sibling", ActorType: "service", Action: "sibling_project", OrgID: org.ID, WorkspaceID: workspace.ID, ProjectID: siblingProject.ID, Outcome: "ok", CreatedAt: time.Now().UTC().Add(2 * time.Second)},
	} {
		if _, err := store.CreateCloudAuditEvent(ctx, event); err != nil {
			t.Fatal(err)
		}
	}

	server := NewServer(DefaultConfig(), store, nil, nil)
	handler := server.Handler()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/cloud/v1/audit?limit=20", nil)
	req.Header.Set("Authorization", "Bearer "+readerRaw)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected scoped audit listing ok, got %d body=%s", rr.Code, rr.Body.String())
	}
	var audit []CloudAuditEvent
	if err := json.NewDecoder(rr.Body).Decode(&audit); err != nil {
		t.Fatal(err)
	}
	if len(audit) != 1 || audit[0].Action != "project_only" {
		t.Fatalf("expected only project-scoped audit rows, got %+v", audit)
	}
}

func TestWebhookHistoryPreservesRepeatedSourceEventIDs(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "cloud.db"), "cloud-master-key")
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
			_, _ = w.Write([]byte(`{"ok":true,"team":"Acme","user":"codencer"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer slackAPI.Close()

	server := NewServer(DefaultConfig(), store, cloudconnectors.NewRegistry(), nil)
	handler := server.Handler()

	tokenResp := createAPITokenViaHTTP(t, handler, bootstrapRaw, org.ID, workspace.ID, project.ID)
	installation := createInstallationViaHTTP(t, handler, tokenResp.Token, org.ID, workspace.ID, project.ID, slackAPI.URL)

	webhookBodies := [][]byte{
		[]byte(`{"type":"event_callback","event_id":"EvDup","event":{"type":"app_mention","user":"U1","channel":"C1","text":"first approval","ts":"1713096000.000100"}}`),
		[]byte(`{"type":"event_callback","event_id":"EvDup","event":{"type":"app_mention","user":"U1","channel":"C1","text":"second approval","ts":"1713096001.000100"}}`),
	}
	for idx, body := range webhookBodies {
		timestamp := time.Now().Unix() + int64(idx)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/cloud/v1/installations/"+installation.ID+"/webhook", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Slack-Request-Timestamp", fmt.Sprintf("%d", timestamp))
		req.Header.Set("X-Slack-Signature", slackSignature("slack-secret", body, timestamp))
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusAccepted {
			t.Fatalf("webhook %d: expected accepted, got %d body=%s", idx, rr.Code, rr.Body.String())
		}
	}

	events, err := store.ListConnectorEvents(ctx, installation.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("expected repeated source_event_id rows to be preserved, got %+v", events)
	}
	if events[0].SourceEventID != "EvDup" || events[1].SourceEventID != "EvDup" {
		t.Fatalf("expected repeated shared source_event_id EvDup, got %+v", events)
	}
	payloads := []string{string(events[0].PayloadJSON), string(events[1].PayloadJSON)}
	if !(containsSubstring(payloads, "first approval") && containsSubstring(payloads, "second approval")) {
		t.Fatalf("expected both webhook payloads to survive history append, got %+v", payloads)
	}
}

func TestJiraWebhookRouteReturnsDeferredWithoutPersistingEvents(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "cloud.db"), "cloud-master-key")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	org, workspace, project := seedCloudScope(t, store, ctx)
	installation, err := store.CreateConnectorInstallation(ctx, ConnectorInstallation{
		OrgID:        org.ID,
		WorkspaceID:  workspace.ID,
		ProjectID:    project.ID,
		ConnectorKey: string(cloudconnectors.ProviderJira),
		Name:         "Jira",
		Status:       "active",
		Enabled:      true,
		Health:       "healthy",
		ConfigJSON:   json.RawMessage(`{"api_base_url":"https://jira.example","username":"jira@example.com","project_key":"PROJ"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.PutInstallationSecret(ctx, installation.ID, "token", []byte("jira-token")); err != nil {
		t.Fatal(err)
	}

	server := NewServer(DefaultConfig(), store, cloudconnectors.NewRegistry(), nil)
	handler := server.Handler()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/cloud/v1/installations/"+installation.ID+"/webhook", strings.NewReader(`{"issue":{"key":"PROJ-17","fields":{"summary":"Fix bug","status":{"name":"Done"}}}}`))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("expected jira webhook route to be deferred, got %d body=%s", rr.Code, rr.Body.String())
	}

	events, err := store.ListConnectorEvents(ctx, installation.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("expected jira webhook deferment to skip event persistence, got %+v", events)
	}

	audit, err := store.ListCloudAuditEvents(ctx, 20)
	if err != nil {
		t.Fatal(err)
	}
	foundDeferred := false
	for _, item := range audit {
		if item.Action == "webhook_ingest" && item.ResourceID == installation.ID && item.Outcome == "deferred" {
			foundDeferred = true
		}
		if item.Action == "webhook_ingest" && item.ResourceID == installation.ID && item.Outcome == "ok" {
			t.Fatalf("expected jira webhook deferment to avoid success audit rows, got %+v", audit)
		}
	}
	if !foundDeferred {
		t.Fatalf("expected deferred jira webhook audit row, got %+v", audit)
	}
}

func TestConnectorActionLogsCaptureRequestCompletionAndAuditDetails(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "cloud.db"), "cloud-master-key")
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
			_, _ = w.Write([]byte(`{"ok":true,"team":"Acme","user":"codencer"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/chat.postMessage":
			_, _ = w.Write([]byte(`{"ok":true,"channel":"C123","ts":"1713096000.000100","text":"hello beta"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer slackAPI.Close()

	server := NewServer(DefaultConfig(), store, cloudconnectors.NewRegistry(), nil)
	handler := server.Handler()

	tokenResp := createAPITokenViaHTTP(t, handler, bootstrapRaw, org.ID, workspace.ID, project.ID)
	installation := createInstallationViaHTTP(t, handler, tokenResp.Token, org.ID, workspace.ID, project.ID, slackAPI.URL)

	actionBody := `{"action":"post_message","channel":"C123","body":"hello beta"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/cloud/v1/installations/"+installation.ID+"/actions", strings.NewReader(actionBody))
	req.Header.Set("Authorization", "Bearer "+tokenResp.Token)
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected connector action ok, got %d body=%s", rr.Code, rr.Body.String())
	}

	logs, err := store.ListConnectorActionLogs(ctx, installation.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected one action log, got %+v", logs)
	}
	log := logs[0]
	if log.ActionName != "post_message" || log.Status != "completed" {
		t.Fatalf("unexpected action log: %+v", log)
	}
	if log.CompletedAt == nil || log.CompletedAt.Before(log.StartedAt) {
		t.Fatalf("expected completed action timestamps, got %+v", log)
	}
	if !strings.Contains(string(log.RequestJSON), `"channel":"C123"`) || !strings.Contains(string(log.RequestJSON), `"body":"hello beta"`) {
		t.Fatalf("expected request payload in action log, got %s", string(log.RequestJSON))
	}
	if !strings.Contains(string(log.ResponseJSON), `"external_id":"1713096000.000100"`) {
		t.Fatalf("expected response payload in action log, got %s", string(log.ResponseJSON))
	}

	audit, err := store.ListCloudAuditEvents(ctx, 20)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range audit {
		if item.Action != "connector_action" || item.ResourceID != installation.ID {
			continue
		}
		found = true
		details := map[string]any{}
		if err := json.Unmarshal(item.DetailsJSON, &details); err != nil {
			t.Fatalf("decode action audit details: %v", err)
		}
		if details["action"] != "post_message" || details["provider"] != "slack" {
			t.Fatalf("unexpected action audit details: %+v", details)
		}
		if details["status"] != "completed" || details["external_id"] != "1713096000.000100" {
			t.Fatalf("expected richer connector action audit details, got %+v", details)
		}
	}
	if !found {
		t.Fatalf("expected connector_action audit row, got %+v", audit)
	}
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

func containsSubstring(values []string, want string) bool {
	for _, value := range values {
		if strings.Contains(value, want) {
			return true
		}
	}
	return false
}
