package service_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/service"
	"agent-bridge/internal/storage/sqlite"
	_ "github.com/mattn/go-sqlite3"
)

func TestRunService_Retrieval(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := sqlite.RunMigrations(db); err != nil {
		t.Fatal(err)
	}

	runsRepo := sqlite.NewRunsRepo(db)
	phasesRepo := sqlite.NewPhasesRepo(db)
	stepsRepo := sqlite.NewStepsRepo(db)
	attemptsRepo := sqlite.NewAttemptsRepo(db)
	gatesRepo := sqlite.NewGatesRepo(db)
	artifactsRepo := sqlite.NewArtifactsRepo(db)
	validationsRepo := sqlite.NewValidationsRepo(db)
	routingSvc := service.NewRoutingService(nil, nil)

	svc := service.NewRunService(runsRepo, phasesRepo, stepsRepo, attemptsRepo, gatesRepo, artifactsRepo, validationsRepo, routingSvc, service.NewPolicyRegistry(), t.TempDir(), t.TempDir())

	ctx := context.Background()
	runID := "test-run"
	stepID := "test-step"

	// 1. Setup minimal run/phase/step
	_, _ = svc.StartRun(ctx, runID, "test-proj")
	step := &domain.Step{
		ID:      stepID,
		PhaseID: "phase-01-" + runID,
		Title:   "Test Step",
		Adapter: "mock",
		State:   domain.StepStateRunning,
	}
	_ = stepsRepo.Create(ctx, step)

	t.Run("GetResultByStep - No Attempts", func(t *testing.T) {
		res, err := svc.GetResultByStep(ctx, stepID)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if res.Summary != "No attempts executed for this step yet." {
			t.Errorf("unexpected summary: %s", res.Summary)
		}
	})

	t.Run("GetResultByStep - Incomplete Attempt", func(t *testing.T) {
		attempt := &domain.Attempt{
			ID:        stepID + "-a1",
			StepID:    stepID,
			Number:    1,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		_ = attemptsRepo.Create(ctx, attempt)

		res, err := svc.GetResultByStep(ctx, stepID)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if res.State != domain.StepStateRunning {
			t.Errorf("Expected State running, got %s", res.State)
		}
	})

	t.Run("Validation Retrieval", func(t *testing.T) {
		vRes := &domain.ValidationResult{
			Name:    "lint",
			Command: "make lint",
			State:   domain.ValidationStatePassed,
			Passed:  true,
		}
		_ = validationsRepo.Create(ctx, stepID+"-a1", vRes)

		byStep, err := svc.GetValidationsByStep(ctx, stepID)
		if err != nil {
			t.Fatal(err)
		}
		if len(byStep[stepID+"-a1"]) != 1 {
			t.Errorf("expected 1 validation outcome, got %d", len(byStep[stepID+"-a1"]))
		}
	})

	t.Run("Artifact Retrieval", func(t *testing.T) {
		art := &domain.Artifact{
			ID:        "art-1",
			AttemptID: stepID + "-a1",
			Type:      domain.ArtifactTypeDiff,
			Name:      "changes.diff",
			Path:      "/tmp/changes.diff",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		_ = artifactsRepo.Create(ctx, art)

		arts, err := svc.GetArtifactsByStep(ctx, stepID)
		if err != nil {
			t.Fatal(err)
		}
		if len(arts) != 1 {
			t.Errorf("expected 1 artifact, got %d", len(arts))
		}
		if arts[0].Name != "changes.diff" {
			t.Errorf("expected name changes.diff, got %s", arts[0].Name)
		}
	})
}
