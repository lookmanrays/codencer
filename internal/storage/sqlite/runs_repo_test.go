package sqlite

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"agent-bridge/internal/domain"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := RunMigrations(db); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}
	return db
}

func TestRunsRepo_CreateAndGet(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewRunsRepo(db)

	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	run := &domain.Run{
		ID:        "run-123",
		ProjectID: "proj-123",
		State:     domain.RunStateCreated,
		CreatedAt: now,
		UpdatedAt: now,
	}

	err := repo.Create(ctx, run)
	if err != nil {
		t.Fatalf("failed to create run: %v", err)
	}

	retrieved, err := repo.Get(ctx, run.ID)
	if err != nil {
		t.Fatalf("failed to get run: %v", err)
	}
	if retrieved == nil {
		t.Fatalf("run not found")
	}

	if retrieved.ID != run.ID || retrieved.State != run.State {
		t.Errorf("retrieved run mismatch. Expected state %s, got %s", run.State, retrieved.State)
	}
}

func TestRunsRepo_UpdateState(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewRunsRepo(db)

	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	run := &domain.Run{
		ID:        "run-123",
		ProjectID: "proj-123",
		State:     domain.RunStateCreated,
		CreatedAt: now,
		UpdatedAt: now,
	}

	_ = repo.Create(ctx, run)

	run.State = domain.RunStateRunning
	run.UpdatedAt = now.Add(1 * time.Minute)

	if err := repo.UpdateState(ctx, run); err != nil {
		t.Fatalf("failed to update state: %v", err)
	}

	retrieved, _ := repo.Get(ctx, run.ID)
	if retrieved.State != domain.RunStateRunning {
		t.Errorf("expected state %s, got %s", domain.RunStateRunning, retrieved.State)
	}
}
