package cloud

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
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
		"api_tokens",
		"connector_installations",
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
	if appliedCount != 1 {
		t.Fatalf("expected one migration row, got %d", appliedCount)
	}

	store, err = OpenStore(path, "cloud-master-key")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := db.QueryRow(`SELECT COUNT(*) FROM cloud_schema_migrations`).Scan(&appliedCount); err != nil {
		t.Fatal(err)
	}
	if appliedCount != 1 {
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
