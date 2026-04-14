package cloud

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	cloudconnectors "agent-bridge/internal/cloud/connectors"
)

func TestWorkerRunOncePollsJiraAndPersistsSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cloud.db")
	store, err := OpenStore(path, "cloud-master-key")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	org, workspace, project := seedCloudScope(t, store, ctx)
	fixedNow := time.Date(2026, 4, 14, 12, 30, 0, 0, time.UTC)
	searchHit := map[string]any{
		"key":  "PROJ-17",
		"self": "https://jira.example/rest/api/3/issue/PROJ-17",
		"fields": map[string]any{
			"summary":  "Fix the bug",
			"status":   map[string]any{"name": "In Progress"},
			"assignee": map[string]any{"displayName": "Ada Lovelace"},
			"updated":  "2026-04-14T12:34:56.000+0000",
		},
	}
	updatedAt := time.Date(2026, 4, 14, 12, 34, 56, 0, time.UTC)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/rest/api/3/search" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("jql"); !strings.Contains(got, `project = "PROJ"`) || !strings.Contains(got, "ORDER BY updated ASC") {
			t.Fatalf("unexpected jql: %q", got)
		}
		if got := r.URL.Query().Get("maxResults"); got != "50" {
			t.Fatalf("unexpected maxResults: %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Basic amlyYUBleGFtcGxlLmNvbTpqaXJhLXRva2Vu" {
			t.Fatalf("unexpected authorization header: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issues": []any{searchHit},
		})
	}))
	defer srv.Close()

	installation, err := store.CreateConnectorInstallation(ctx, ConnectorInstallation{
		OrgID:        org.ID,
		WorkspaceID:  workspace.ID,
		ProjectID:    project.ID,
		ConnectorKey: string(cloudconnectors.ProviderJira),
		ConfigJSON:   json.RawMessage(`{"api_base_url":"` + srv.URL + `","username":"jira@example.com","project_key":"PROJ"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.PutInstallationSecret(ctx, installation.ID, "token", []byte("jira-token")); err != nil {
		t.Fatal(err)
	}

	worker := NewWorker(store, srv.Client(), 50)
	worker.now = func() time.Time { return fixedNow }

	if err := worker.RunOnce(ctx); err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}

	events, err := store.ListConnectorEvents(ctx, installation.ID, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected one event, got %d", len(events))
	}
	if events[0].EventType != "issue_snapshot" || events[0].Action != "snapshot" {
		t.Fatalf("unexpected event record: %+v", events[0])
	}
	if events[0].SourceEventID != "PROJ-17:"+strconv.FormatInt(updatedAt.UnixMilli(), 10) {
		t.Fatalf("unexpected source event id: %+v", events[0])
	}

	updated, err := store.GetConnectorInstallation(ctx, installation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != "active" || updated.LastSyncAt == nil || !updated.LastSyncAt.Equal(fixedNow) {
		t.Fatalf("expected updated active sync state, got %+v", updated)
	}

	audit, err := store.ListCloudAuditEvents(ctx, 20)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range audit {
		if item.Action == "poll_installation" && item.Outcome == "ok" && item.ResourceID == installation.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected poll_installation audit event, got %+v", audit)
	}
}
