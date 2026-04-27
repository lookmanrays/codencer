package service_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/service"
	"agent-bridge/internal/storage/sqlite"
	_ "github.com/mattn/go-sqlite3"
)

func TestInstanceServiceCurrentDegradesBrokerErrors(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "instance.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := sqlite.RunMigrations(db); err != nil {
		t.Fatal(err)
	}

	settingsRepo := sqlite.NewSettingsRepo(db)
	repoRoot := filepath.Join(t.TempDir(), "repo")
	instanceSvc := service.NewInstanceService(
		settingsRepo,
		service.NewAntigravityService(settingsRepo, "http://127.0.0.1:1", repoRoot),
		"test-version",
		time.Unix(1700000000, 0).UTC(),
		repoRoot,
		filepath.Join(t.TempDir(), ".codencer"),
		filepath.Join(t.TempDir(), "workspace"),
		"127.0.0.1",
		7777,
		func() map[string]domain.Adapter { return map[string]domain.Adapter{} },
	)

	if err := settingsRepo.Set(context.Background(), "daemon_instance_id", "inst-degraded"); err != nil {
		t.Fatal(err)
	}

	info, err := instanceSvc.Current(context.Background())
	if err != nil {
		t.Fatalf("expected degraded instance info, got error %v", err)
	}
	if info.ID != "inst-degraded" {
		t.Fatalf("expected stable instance id, got %s", info.ID)
	}
	if !info.Broker.Enabled || info.Broker.Mode != "broker" {
		t.Fatalf("expected broker configuration to remain visible, got %+v", info.Broker)
	}
	if info.Broker.BoundInstance != nil {
		t.Fatalf("expected bound instance to be omitted on broker failure, got %+v", info.Broker.BoundInstance)
	}
}
