package cloud

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenStoreRunsMigrationsAndIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cloud.db")

	store, err := OpenStore(path, "cloud-master-key")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	tables := []string{
		"orgs",
		"workspaces",
		"projects",
		"memberships",
		"api_tokens",
		"connector_installations",
		"runtime_connector_installations",
		"runtime_instances",
		"installation_secrets",
		"connector_events",
		"connector_action_logs",
		"cloud_audit_events",
		"cloud_schema_migrations",
	}
	for _, table := range tables {
		var name string
		if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name = ?`, table).Scan(&name); err != nil {
			t.Fatalf("expected table %s to exist: %v", table, err)
		}
	}

	var appliedCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM cloud_schema_migrations`).Scan(&appliedCount); err != nil {
		t.Fatal(err)
	}
	if appliedCount != len(migrations) {
		t.Fatalf("expected %d migration rows, got %d", len(migrations), appliedCount)
	}

	store, err = OpenStore(path, "cloud-master-key")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := db.QueryRow(`SELECT COUNT(*) FROM cloud_schema_migrations`).Scan(&appliedCount); err != nil {
		t.Fatal(err)
	}
	if appliedCount != len(migrations) {
		t.Fatalf("expected migrations to remain idempotent, got %d rows", appliedCount)
	}
}

func TestStoreCreatesOrgWorkspaceProjectAndInstallation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cloud.db")
	store, err := OpenStore(path, "cloud-master-key")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()

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
	installation, err := store.CreateConnectorInstallation(ctx, ConnectorInstallation{
		OrgID:        org.ID,
		WorkspaceID:  workspace.ID,
		ProjectID:    project.ID,
		ConnectorKey: "github",
		Name:         "GitHub",
	})
	if err != nil {
		t.Fatal(err)
	}
	if installation.ID == "" {
		t.Fatal("expected installation id")
	}
	loaded, err := store.GetConnectorInstallation(ctx, installation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ConnectorKey != "github" || loaded.Name != "GitHub" {
		t.Fatalf("unexpected installation loaded: %+v", loaded)
	}
}

func TestStoreCreateConnectorEventPreservesRepeatedSourceEventHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cloud.db")
	store, err := OpenStore(path, "cloud-master-key")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
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
	installation, err := store.CreateConnectorInstallation(ctx, ConnectorInstallation{
		OrgID:        org.ID,
		WorkspaceID:  workspace.ID,
		ProjectID:    project.ID,
		ConnectorKey: "github",
		Name:         "GitHub",
		Status:       "active",
		Enabled:      true,
		Health:       "healthy",
	})
	if err != nil {
		t.Fatal(err)
	}

	base := time.Now().UTC()
	for idx := 0; idx < 2; idx++ {
		if _, err := store.CreateConnectorEvent(ctx, ConnectorEvent{
			ID:             "",
			InstallationID: installation.ID,
			SourceEventID:  "issue-17",
			EventType:      "issue.opened",
			Action:         "opened",
			Status:         "received",
			OccurredAt:     base.Add(time.Duration(idx) * time.Second),
			ReceivedAt:     base.Add(time.Duration(idx) * time.Second),
		}); err != nil {
			t.Fatalf("CreateConnectorEvent(%d) failed: %v", idx, err)
		}
	}

	events, err := store.ListConnectorEvents(ctx, installation.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("expected repeated source_event_id rows to be preserved, got %+v", events)
	}
	if events[0].SourceEventID != "issue-17" || events[1].SourceEventID != "issue-17" {
		t.Fatalf("unexpected source event ids: %+v", events)
	}
	if !events[0].ReceivedAt.After(events[1].ReceivedAt) {
		t.Fatalf("expected newest event first, got %+v", events)
	}
}
