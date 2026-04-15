package cloud

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestRuntimeRegistryTablesAreMigrated(t *testing.T) {
	store := mustOpenCloudStore(t)
	defer store.Close()

	tables := []string{
		"runtime_connector_installations",
		"runtime_instances",
	}
	for _, table := range tables {
		if !tableExists(t, store, table) {
			t.Fatalf("expected table %s to exist", table)
		}
	}
}

func TestRuntimeConnectorInstallationCRUDAndScope(t *testing.T) {
	store := mustOpenCloudStore(t)
	defer store.Close()

	ctx := context.Background()
	orgA, wsA, projA := mustTenantScope(t, store, "acme", "platform", "core")
	orgB, wsB, projB := mustTenantScope(t, store, "bravo", "ops", "infra")
	wsA2, err := store.CreateWorkspace(ctx, Workspace{OrgID: orgA.ID, Slug: "platform-2", Name: "platform-2 workspace"})
	if err != nil {
		t.Fatal(err)
	}
	projA2, err := store.CreateProject(ctx, Project{OrgID: orgA.ID, WorkspaceID: wsA2.ID, Slug: "core-2", Name: "core-2 project"})
	if err != nil {
		t.Fatal(err)
	}

	seen := time.Date(2026, time.March, 14, 10, 0, 0, 0, time.UTC)
	created, err := store.CreateRuntimeConnectorInstallation(ctx, RuntimeConnectorInstallation{
		OrgID:        orgA.ID,
		WorkspaceID:  wsA.ID,
		ProjectID:    projA.ID,
		ConnectorID:  "connector-acme-1",
		MachineID:    "machine-acme-1",
		Label:        "WSL Node",
		PublicKey:    "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIruntime",
		Status:       "online",
		Enabled:      true,
		Health:       "healthy",
		MetadataJSON: json.RawMessage(`{"shared":true,"platform":"wsl"}`),
		LastSeenAt:   &seen,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" {
		t.Fatal("expected runtime connector installation id")
	}
	loaded, err := store.GetRuntimeConnectorInstallation(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ConnectorID != "connector-acme-1" || loaded.MachineID != "machine-acme-1" || loaded.Label != "WSL Node" {
		t.Fatalf("unexpected connector installation: %+v", loaded)
	}
	if loaded.Status != "online" || !loaded.Enabled || loaded.Health != "healthy" {
		t.Fatalf("unexpected connector installation state: %+v", loaded)
	}
	if loaded.LastSeenAt == nil || !loaded.LastSeenAt.Equal(seen) {
		t.Fatalf("expected last seen to be preserved, got %+v", loaded.LastSeenAt)
	}
	if string(loaded.MetadataJSON) != `{"shared":true,"platform":"wsl"}` {
		t.Fatalf("unexpected metadata json: %s", string(loaded.MetadataJSON))
	}

	updatedRecord := *loaded
	updatedRecord.Label = "WSL Node Updated"
	updatedRecord.Status = "disabled"
	updatedRecord.Enabled = false
	updatedRecord.Health = "degraded"
	updatedRecord.LastError = "heartbeat missed"
	updatedRecord.MetadataJSON = json.RawMessage(`{"shared":false,"platform":"wsl","reason":"maintenance"}`)
	updated, err := store.UpdateRuntimeConnectorInstallation(ctx, updatedRecord)
	if err != nil {
		t.Fatal(err)
	}
	if !updated.CreatedAt.Equal(loaded.CreatedAt) {
		t.Fatalf("expected created_at to remain stable, got %s vs %s", updated.CreatedAt, loaded.CreatedAt)
	}
	if updated.Label != "WSL Node Updated" || updated.Status != "disabled" || updated.Enabled || updated.Health != "degraded" || updated.LastError != "heartbeat missed" {
		t.Fatalf("unexpected updated connector installation: %+v", updated)
	}

	tenantScoped, err := store.ListRuntimeConnectorInstallations(ctx, orgA.ID, wsA.ID, projA.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tenantScoped) != 1 {
		t.Fatalf("expected one scoped runtime connector, got %d", len(tenantScoped))
	}
	if tenantScoped[0].ID != updated.ID {
		t.Fatalf("expected scoped runtime connector %s, got %s", updated.ID, tenantScoped[0].ID)
	}

	secondTenantConnector, err := store.CreateRuntimeConnectorInstallation(ctx, RuntimeConnectorInstallation{
		OrgID:       orgA.ID,
		WorkspaceID: wsA2.ID,
		ProjectID:   projA2.ID,
		ConnectorID: "connector-acme-2",
		MachineID:   "machine-acme-2",
		Label:       "Laptop",
		Status:      "online",
		Enabled:     true,
		Health:      "healthy",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateRuntimeConnectorInstallation(ctx, RuntimeConnectorInstallation{
		OrgID:       orgB.ID,
		WorkspaceID: wsB.ID,
		ProjectID:   projB.ID,
		ConnectorID: "connector-bravo-1",
		MachineID:   "machine-bravo-1",
		Label:       "Remote",
		Status:      "online",
		Enabled:     true,
		Health:      "healthy",
	}); err != nil {
		t.Fatal(err)
	}

	allAcme, err := store.ListRuntimeConnectorInstallations(ctx, orgA.ID, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(allAcme) != 2 {
		t.Fatalf("expected two runtime connectors in acme org, got %d", len(allAcme))
	}
	workspaceScoped, err := store.ListRuntimeConnectorInstallations(ctx, orgA.ID, wsA2.ID, projA2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(workspaceScoped) != 1 || workspaceScoped[0].ID != secondTenantConnector.ID {
		t.Fatalf("unexpected workspace scoped connectors: %+v", workspaceScoped)
	}
	orgScoped, err := store.ListRuntimeConnectorInstallations(ctx, orgB.ID, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(orgScoped) != 1 || orgScoped[0].ConnectorID != "connector-bravo-1" {
		t.Fatalf("unexpected org scoped runtime connectors: %+v", orgScoped)
	}
}

func TestRuntimeInstanceCRUDAndScope(t *testing.T) {
	store := mustOpenCloudStore(t)
	defer store.Close()

	ctx := context.Background()
	orgA, wsA, projA := mustTenantScope(t, store, "acme", "platform", "core")
	orgB, wsB, projB := mustTenantScope(t, store, "bravo", "ops", "infra")

	connectorA, err := store.CreateRuntimeConnectorInstallation(ctx, RuntimeConnectorInstallation{
		OrgID:       orgA.ID,
		WorkspaceID: wsA.ID,
		ProjectID:   projA.ID,
		ConnectorID: "connector-acme",
		MachineID:   "machine-acme",
		Label:       "Primary WSL",
		Status:      "online",
		Enabled:     true,
		Health:      "healthy",
	})
	if err != nil {
		t.Fatal(err)
	}
	connectorB, err := store.CreateRuntimeConnectorInstallation(ctx, RuntimeConnectorInstallation{
		OrgID:       orgB.ID,
		WorkspaceID: wsB.ID,
		ProjectID:   projB.ID,
		ConnectorID: "connector-bravo",
		MachineID:   "machine-bravo",
		Label:       "Secondary WSL",
		Status:      "online",
		Enabled:     true,
		Health:      "healthy",
	})
	if err != nil {
		t.Fatal(err)
	}

	createdAlpha, err := store.CreateRuntimeInstance(ctx, RuntimeInstance{
		ID:                             "inst-alpha",
		OrgID:                          orgA.ID,
		WorkspaceID:                    wsA.ID,
		ProjectID:                      projA.ID,
		RuntimeConnectorInstallationID: connectorA.ID,
		RepoRoot:                       "/repo/acme/core",
		InstanceJSON:                   json.RawMessage(`{"platform":"wsl","adapters":["git","shell"]}`),
		Status:                         "online",
		Enabled:                        true,
		Health:                         "healthy",
		Shared:                         true,
		LastSeenAt:                     ptrTime(time.Date(2026, time.March, 14, 11, 0, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.CreateRuntimeInstance(ctx, RuntimeInstance{
		ID:                             "inst-beta",
		OrgID:                          orgA.ID,
		WorkspaceID:                    wsA.ID,
		ProjectID:                      projA.ID,
		RuntimeConnectorInstallationID: connectorA.ID,
		RepoRoot:                       "/repo/acme/platform",
		InstanceJSON:                   json.RawMessage(`{"platform":"wsl","adapters":["git"]}`),
		Status:                         "offline",
		Enabled:                        true,
		Health:                         "degraded",
		Shared:                         false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateRuntimeInstance(ctx, RuntimeInstance{
		ID:                             "inst-bravo",
		OrgID:                          orgB.ID,
		WorkspaceID:                    wsB.ID,
		ProjectID:                      projB.ID,
		RuntimeConnectorInstallationID: connectorB.ID,
		RepoRoot:                       "/repo/bravo/infra",
		Status:                         "online",
		Enabled:                        true,
		Health:                         "healthy",
		Shared:                         true,
	}); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.GetRuntimeInstance(ctx, createdAlpha.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.RepoRoot != "/repo/acme/core" || loaded.RuntimeConnectorInstallationID != connectorA.ID || !loaded.Shared {
		t.Fatalf("unexpected runtime instance: %+v", loaded)
	}
	if string(loaded.InstanceJSON) != `{"platform":"wsl","adapters":["git","shell"]}` {
		t.Fatalf("unexpected instance json: %s", string(loaded.InstanceJSON))
	}
	if loaded.LastSeenAt == nil || !loaded.LastSeenAt.Equal(time.Date(2026, time.March, 14, 11, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected last seen to be preserved, got %+v", loaded.LastSeenAt)
	}

	updatedRecord := *loaded
	updatedRecord.Status = "disabled"
	updatedRecord.Enabled = false
	updatedRecord.Health = "degraded"
	updatedRecord.Shared = false
	updatedRecord.LastError = "connector paused"
	updatedRecord.InstanceJSON = json.RawMessage(`{"platform":"wsl","adapters":["git","shell"],"paused":true}`)
	updated, err := store.UpdateRuntimeInstance(ctx, updatedRecord)
	if err != nil {
		t.Fatal(err)
	}
	if !updated.CreatedAt.Equal(loaded.CreatedAt) {
		t.Fatalf("expected created_at to remain stable, got %s vs %s", updated.CreatedAt, loaded.CreatedAt)
	}
	if updated.Status != "disabled" || updated.Enabled || updated.Health != "degraded" || updated.Shared || updated.LastError != "connector paused" {
		t.Fatalf("unexpected updated runtime instance: %+v", updated)
	}

	scoped, err := store.ListRuntimeInstances(ctx, orgA.ID, wsA.ID, projA.ID, connectorA.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(scoped) != 2 {
		t.Fatalf("expected two runtime instances in acme scope, got %d", len(scoped))
	}
	for _, instance := range scoped {
		if instance.OrgID != orgA.ID || instance.RuntimeConnectorInstallationID != connectorA.ID {
			t.Fatalf("instance escaped scope: %+v", instance)
		}
	}

	orgScoped, err := store.ListRuntimeInstances(ctx, orgA.ID, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(orgScoped) != 2 {
		t.Fatalf("expected two runtime instances in acme org, got %d", len(orgScoped))
	}

	otherOrg, err := store.ListRuntimeInstances(ctx, orgB.ID, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(otherOrg) != 1 || otherOrg[0].ID != "inst-bravo" {
		t.Fatalf("unexpected other-org runtime instances: %+v", otherOrg)
	}
}

func mustOpenCloudStore(t *testing.T) *Store {
	t.Helper()
	store, err := OpenStore(filepath.Join(t.TempDir(), "cloud.db"), "cloud-master-key")
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func mustTenantScope(t *testing.T, store *Store, orgSlug, workspaceSlug, projectSlug string) (Org, Workspace, Project) {
	t.Helper()
	ctx := context.Background()

	org, err := store.CreateOrg(ctx, Org{Slug: orgSlug, Name: orgSlug + " org"})
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := store.CreateWorkspace(ctx, Workspace{OrgID: org.ID, Slug: workspaceSlug, Name: workspaceSlug + " workspace"})
	if err != nil {
		t.Fatal(err)
	}
	project, err := store.CreateProject(ctx, Project{OrgID: org.ID, WorkspaceID: workspace.ID, Slug: projectSlug, Name: projectSlug + " project"})
	if err != nil {
		t.Fatal(err)
	}
	return *org, *workspace, *project
}

func ptrTime(t time.Time) *time.Time {
	return &t
}

func tableExists(t *testing.T, store *Store, table string) bool {
	t.Helper()
	var name string
	err := store.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name = ?`, table).Scan(&name)
	return err == nil
}
