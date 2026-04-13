package service_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/service"
	"agent-bridge/internal/storage/sqlite"
	_ "github.com/mattn/go-sqlite3"
)

func TestRecovery_StaleAttempt_Salvage(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, _ := sql.Open("sqlite3", dbPath)
	sqlite.RunMigrations(db)

	artifactRoot := filepath.Join(tmpDir, "artifacts")
	workspaceRoot := filepath.Join(tmpDir, "workspace")
	os.MkdirAll(artifactRoot, 0755)
	os.MkdirAll(workspaceRoot, 0755)

	runsRepo := sqlite.NewRunsRepo(db)
	stepsRepo := sqlite.NewStepsRepo(db)
	attemptsRepo := sqlite.NewAttemptsRepo(db)
	phasesRepo := sqlite.NewPhasesRepo(db)
	gatesRepo := sqlite.NewGatesRepo(db)

	recoverySvc := service.NewRecoveryService(runsRepo, stepsRepo, attemptsRepo, gatesRepo, artifactRoot, workspaceRoot)
	ctx := context.Background()

	// 1. Setup Stale State
	runID := "stale-run-1"
	phaseID := "stale-phase-1"
	stepID := "stale-step-1"
	runsRepo.Create(ctx, &domain.Run{ID: runID, State: domain.RunStateRunning, CreatedAt: time.Now(), UpdatedAt: time.Now()})
	phasesRepo.Create(ctx, &domain.Phase{ID: phaseID, RunID: runID})
	stepsRepo.Create(ctx, &domain.Step{ID: stepID, PhaseID: phaseID, State: domain.StepStateRunning})
	attemptsRepo.Create(ctx, &domain.Attempt{
		ID:        "stale-attempt-1",
		StepID:    stepID,
		Number:    1,
		State:     domain.StepStateRunning,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	// Simulate process completion on disk but crash before DB update
	// New namespaced path: <artifactRoot>/<runID>/<stepID>/<attemptID>
	stepArtDir := filepath.Join(artifactRoot, runID, stepID, "stale-attempt-1")
	os.MkdirAll(stepArtDir, 0755)
	os.WriteFile(filepath.Join(stepArtDir, "result.json"), []byte(`{
		"version":"v1",
		"run_id":"stale-run-1",
		"step_id":"stale-step-1",
		"state":"completed",
		"summary":"Recovered result"
	}`), 0644)

	// 2. Run Sweep
	err := recoverySvc.SweepStaleRuns(ctx)
	if err != nil {
		t.Fatalf("Sweep failed: %v", err)
	}

	// 3. Verify
	run, _ := runsRepo.Get(ctx, runID)
	if run.State != domain.RunStateCompleted {
		t.Errorf("Expected run state Completed, got %s", run.State)
	}
	if run.RecoveryNotes == "" {
		t.Errorf("Expected recovery notes to be populated")
	}

	step, _ := stepsRepo.Get(ctx, stepID)
	if step.State != domain.StepStateCompleted {
		t.Errorf("Expected step state Completed (salvaged), got %s", step.State)
	}
}

func TestRecovery_StaleAttempt_Fail(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, _ := sql.Open("sqlite3", dbPath)
	sqlite.RunMigrations(db)

	artifactRoot := filepath.Join(tmpDir, "artifacts")
	workspaceRoot := filepath.Join(tmpDir, "workspace")
	os.MkdirAll(artifactRoot, 0755)
	os.MkdirAll(workspaceRoot, 0755)

	runsRepo := sqlite.NewRunsRepo(db)
	stepsRepo := sqlite.NewStepsRepo(db)
	attemptsRepo := sqlite.NewAttemptsRepo(db)
	phasesRepo := sqlite.NewPhasesRepo(db)
	gatesRepo := sqlite.NewGatesRepo(db)

	recoverySvc := service.NewRecoveryService(runsRepo, stepsRepo, attemptsRepo, gatesRepo, artifactRoot, workspaceRoot)
	ctx := context.Background()

	// 1. Setup Stale State (no result on disk)
	runID := "stale-run-2"
	phaseID := "stale-phase-2"
	stepID := "stale-step-2"
	runsRepo.Create(ctx, &domain.Run{ID: runID, State: domain.RunStateRunning, CreatedAt: time.Now(), UpdatedAt: time.Now()})
	phasesRepo.Create(ctx, &domain.Phase{ID: phaseID, RunID: runID})
	stepsRepo.Create(ctx, &domain.Step{ID: stepID, PhaseID: phaseID, State: domain.StepStateRunning})

	// 2. Run Sweep
	_ = recoverySvc.SweepStaleRuns(ctx)

	// 3. Verify
	step, _ := stepsRepo.Get(ctx, stepID)
	if step.State != domain.StepStateNeedsManualAttention {
		t.Errorf("Expected step state NeedsManualAttention, got %s", step.State)
	}
	run, _ := runsRepo.Get(ctx, runID)
	if run.State != domain.RunStateFailed {
		t.Errorf("Expected run state Failed, got %s", run.State)
	}
}

func TestRecovery_StaleAttempt_PendingGatePreserved(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, _ := sql.Open("sqlite3", dbPath)
	sqlite.RunMigrations(db)

	artifactRoot := filepath.Join(tmpDir, "artifacts")
	workspaceRoot := filepath.Join(tmpDir, "workspace")
	os.MkdirAll(artifactRoot, 0755)
	os.MkdirAll(workspaceRoot, 0755)

	runsRepo := sqlite.NewRunsRepo(db)
	stepsRepo := sqlite.NewStepsRepo(db)
	attemptsRepo := sqlite.NewAttemptsRepo(db)
	phasesRepo := sqlite.NewPhasesRepo(db)
	gatesRepo := sqlite.NewGatesRepo(db)

	recoverySvc := service.NewRecoveryService(runsRepo, stepsRepo, attemptsRepo, gatesRepo, artifactRoot, workspaceRoot)
	ctx := context.Background()

	runID := "stale-run-gate"
	phaseID := "stale-phase-gate"
	stepID := "stale-step-gate"
	now := time.Now()
	runsRepo.Create(ctx, &domain.Run{ID: runID, State: domain.RunStateRunning, CreatedAt: now, UpdatedAt: now})
	phasesRepo.Create(ctx, &domain.Phase{ID: phaseID, RunID: runID})
	stepsRepo.Create(ctx, &domain.Step{ID: stepID, PhaseID: phaseID, State: domain.StepStateRunning})
	gatesRepo.Create(ctx, &domain.Gate{
		ID:          "gate-1",
		RunID:       runID,
		StepID:      stepID,
		Description: "pending",
		State:       domain.GateStatePending,
		CreatedAt:   now,
	})

	if err := recoverySvc.SweepStaleRuns(ctx); err != nil {
		t.Fatalf("Sweep failed: %v", err)
	}

	step, _ := stepsRepo.Get(ctx, stepID)
	if step.State != domain.StepStateNeedsApproval {
		t.Fatalf("expected step state needs_approval, got %s", step.State)
	}
	run, _ := runsRepo.Get(ctx, runID)
	if run.State != domain.RunStatePausedForGate {
		t.Fatalf("expected run state paused_for_gate, got %s", run.State)
	}
}
