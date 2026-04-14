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

func TestGateService_ApproveAndRejectLifecycle(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := sqlite.RunMigrations(db); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	runsRepo := sqlite.NewRunsRepo(db)
	phasesRepo := sqlite.NewPhasesRepo(db)
	stepsRepo := sqlite.NewStepsRepo(db)
	attemptsRepo := sqlite.NewAttemptsRepo(db)
	gatesRepo := sqlite.NewGatesRepo(db)

	run := &domain.Run{ID: "run-1", ProjectID: "p1", State: domain.RunStatePausedForGate, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := runsRepo.Create(ctx, run); err != nil {
		t.Fatal(err)
	}
	if err := phasesRepo.Create(ctx, &domain.Phase{ID: "phase-1", RunID: run.ID, Name: "Execution", SeqOrder: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	step := &domain.Step{ID: "step-1", PhaseID: "phase-1", Title: "Step", Goal: "Goal", Adapter: "mock", State: domain.StepStateNeedsApproval, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := stepsRepo.Create(ctx, step); err != nil {
		t.Fatal(err)
	}
	attempt := &domain.Attempt{
		ID:        "attempt-1",
		StepID:    step.ID,
		Number:    1,
		Adapter:   "mock",
		Result:    &domain.ResultSpec{Version: "v1", State: domain.StepStateCompleted, Summary: "approved"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := attemptsRepo.Create(ctx, attempt); err != nil {
		t.Fatal(err)
	}
	gate := &domain.Gate{ID: "gate-1", RunID: run.ID, StepID: step.ID, Description: "review", State: domain.GateStatePending, CreatedAt: time.Now()}
	if err := gatesRepo.Create(ctx, gate); err != nil {
		t.Fatal(err)
	}

	gateSvc := service.NewGateService(gatesRepo, runsRepo, stepsRepo, attemptsRepo)
	if err := gateSvc.Approve(ctx, gate.ID); err != nil {
		t.Fatalf("Approve failed: %v", err)
	}
	approvedGate, _ := gatesRepo.Get(ctx, gate.ID)
	if approvedGate.State != domain.GateStateApproved || approvedGate.ResolvedAt == nil {
		t.Fatalf("expected gate to be resolved as approved, got %+v", approvedGate)
	}

	approvedStep, _ := stepsRepo.Get(ctx, step.ID)
	if approvedStep.State != domain.StepStateCompleted {
		t.Fatalf("expected step state completed after approval, got %s", approvedStep.State)
	}
	approvedRun, _ := runsRepo.Get(ctx, run.ID)
	if approvedRun.State != domain.RunStateCompleted {
		t.Fatalf("expected run state completed after approval, got %s", approvedRun.State)
	}

	rejectRun := &domain.Run{ID: "run-2", ProjectID: "p2", State: domain.RunStatePausedForGate, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := runsRepo.Create(ctx, rejectRun); err != nil {
		t.Fatal(err)
	}
	if err := phasesRepo.Create(ctx, &domain.Phase{ID: "phase-2", RunID: rejectRun.ID, Name: "Execution", SeqOrder: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	rejectStep := &domain.Step{ID: "step-2", PhaseID: "phase-2", Title: "Reject Step", Goal: "Goal", Adapter: "mock", State: domain.StepStateNeedsApproval, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := stepsRepo.Create(ctx, rejectStep); err != nil {
		t.Fatal(err)
	}
	rejectAttempt := &domain.Attempt{
		ID:        "attempt-2",
		StepID:    rejectStep.ID,
		Number:    1,
		Adapter:   "mock",
		Result:    &domain.ResultSpec{Version: "v1", State: domain.StepStateCompletedWithWarnings, Summary: "reject"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := attemptsRepo.Create(ctx, rejectAttempt); err != nil {
		t.Fatal(err)
	}
	rejectGate := &domain.Gate{ID: "gate-2", RunID: rejectRun.ID, StepID: rejectStep.ID, Description: "review", State: domain.GateStatePending, CreatedAt: time.Now()}
	if err := gatesRepo.Create(ctx, rejectGate); err != nil {
		t.Fatal(err)
	}

	if err := gateSvc.Reject(ctx, rejectGate.ID); err != nil {
		t.Fatalf("Reject failed: %v", err)
	}
	rejectedGate, _ := gatesRepo.Get(ctx, rejectGate.ID)
	if rejectedGate.State != domain.GateStateRejected || rejectedGate.ResolvedAt == nil {
		t.Fatalf("expected gate to be resolved as rejected, got %+v", rejectedGate)
	}
	rejectedStep, _ := stepsRepo.Get(ctx, rejectStep.ID)
	if rejectedStep.State != domain.StepStateCancelled {
		t.Fatalf("expected step state cancelled after rejection, got %s", rejectedStep.State)
	}
	rejectedRun, _ := runsRepo.Get(ctx, rejectRun.ID)
	if rejectedRun.State != domain.RunStateCancelled {
		t.Fatalf("expected run state cancelled after rejection, got %s", rejectedRun.State)
	}
}
