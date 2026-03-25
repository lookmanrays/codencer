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
	
	recoverySvc := service.NewRecoveryService(runsRepo, stepsRepo, attemptsRepo, artifactRoot, workspaceRoot)
	ctx := context.Background()

	// 1. Setup Stale State
	runID := "stale-run-1"
	phaseID := "stale-phase-1"
	stepID := "stale-step-1"
	runsRepo.Create(ctx, &domain.Run{ID: runID, State: domain.RunStateRunning, CreatedAt: time.Now(), UpdatedAt: time.Now()})
	phasesRepo.Create(ctx, &domain.Phase{ID: phaseID, RunID: runID})
	stepsRepo.Create(ctx, &domain.Step{ID: stepID, PhaseID: phaseID, State: domain.StepStateRunning})
	
	// Simulate process completion on disk but crash before DB update
	stepArtDir := filepath.Join(artifactRoot, stepID)
	os.MkdirAll(stepArtDir, 0755)
	os.WriteFile(filepath.Join(stepArtDir, "result.json"), []byte("{}"), 0644)

	// 2. Run Sweep
	err := recoverySvc.SweepStaleRuns(ctx)
	if err != nil {
		t.Fatalf("Sweep failed: %v", err)
	}

	// 3. Verify
	run, _ := runsRepo.Get(ctx, runID)
	if run.State != domain.RunStatePausedForGate {
		t.Errorf("Expected run state PausedForGate, got %s", run.State)
	}
	if run.RecoveryNotes == "" {
		t.Errorf("Expected recovery notes to be populated")
	}

	step, _ := stepsRepo.Get(ctx, stepID)
	if step.State != domain.StepStateNeedsApproval {
		t.Errorf("Expected step state NeedsApproval (salvaged), got %s", step.State)
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
	
	recoverySvc := service.NewRecoveryService(runsRepo, stepsRepo, attemptsRepo, artifactRoot, workspaceRoot)
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
	if step.State != domain.StepStateFailedRetryable {
		t.Errorf("Expected step state FailedRetryable, got %s", step.State)
	}
}
