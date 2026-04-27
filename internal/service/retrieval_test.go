package service_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/service"
	"agent-bridge/internal/storage/sqlite"
	"agent-bridge/internal/workspace"
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
	policyReg := service.NewPolicyRegistry()
	artifactRoot := t.TempDir()
	workspaceRoot := t.TempDir()

	svc := service.NewRunService(runsRepo, phasesRepo, stepsRepo, attemptsRepo, gatesRepo, artifactsRepo, validationsRepo, routingSvc, policyReg, workspace.NewNullProvisioner(), artifactRoot, workspaceRoot)

	ctx := context.Background()
	runID := "test-run"
	stepID := "test-step"

	// 1. Setup minimal run/phase/step
	_, _ = svc.StartRun(ctx, runID, "test-proj", "", "", "")
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

	t.Run("GetResultByStep - Overlays NeedsApproval After Terminal Attempt", func(t *testing.T) {
		step.State = domain.StepStateNeedsApproval
		step.StatusReason = "Policy enforced gate: review required."
		step.UpdatedAt = time.Now().UTC()
		if err := stepsRepo.UpdateState(ctx, step); err != nil {
			t.Fatal(err)
		}

		attempt := &domain.Attempt{
			ID:      stepID + "-a2",
			StepID:  stepID,
			Number:  2,
			Adapter: "mock",
			Result: &domain.ResultSpec{
				Version: "v1",
				State:   domain.StepStateCompleted,
				Summary: "attempt completed before gate",
			},
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if err := attemptsRepo.Create(ctx, attempt); err != nil {
			t.Fatal(err)
		}

		res, err := svc.GetResultByStep(ctx, stepID)
		if err != nil {
			t.Fatal(err)
		}
		if res.State != domain.StepStateNeedsApproval {
			t.Fatalf("expected needs_approval overlay, got %s", res.State)
		}
		if !res.NeedsHumanDecision {
			t.Fatal("expected needs_human_decision to be true for gated step")
		}
		if res.Summary != step.StatusReason {
			t.Fatalf("expected gated summary %q, got %q", step.StatusReason, res.Summary)
		}
	})

	t.Run("GetResultByStep - Overlays Cancelled After Gate Rejection", func(t *testing.T) {
		step.State = domain.StepStateCancelled
		step.StatusReason = "Gate rejected by operator."
		step.UpdatedAt = time.Now().UTC()
		if err := stepsRepo.UpdateState(ctx, step); err != nil {
			t.Fatal(err)
		}

		attempt := &domain.Attempt{
			ID:      stepID + "-a3",
			StepID:  stepID,
			Number:  3,
			Adapter: "mock",
			Result: &domain.ResultSpec{
				Version:   "v1",
				State:     domain.StepStateCompletedWithWarnings,
				Summary:   "completed before rejection",
				Retryable: true,
			},
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if err := attemptsRepo.Create(ctx, attempt); err != nil {
			t.Fatal(err)
		}

		res, err := svc.GetResultByStep(ctx, stepID)
		if err != nil {
			t.Fatal(err)
		}
		if res.State != domain.StepStateCancelled {
			t.Fatalf("expected cancelled overlay, got %s", res.State)
		}
		if res.Retryable {
			t.Fatal("expected cancelled overlay to clear retryable")
		}
		if res.Summary != step.StatusReason {
			t.Fatalf("expected cancelled summary %q, got %q", step.StatusReason, res.Summary)
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

	t.Run("Artifact Content Retrieval", func(t *testing.T) {
		contentPath := filepath.Join(artifactRoot, "stdout.log")
		if err := os.MkdirAll(filepath.Dir(contentPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(contentPath, []byte("hello world"), 0644); err != nil {
			t.Fatal(err)
		}
		artifact := &domain.Artifact{
			ID:        "art-content",
			AttemptID: stepID + "-a1",
			Type:      domain.ArtifactTypeStdout,
			Name:      "stdout.log",
			Path:      contentPath,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := artifactsRepo.Create(ctx, artifact); err != nil {
			t.Fatal(err)
		}

		foundArtifact, content, err := svc.GetArtifactContent(ctx, artifact.ID)
		if err != nil {
			t.Fatal(err)
		}
		if foundArtifact.ID != artifact.ID {
			t.Fatalf("expected artifact %s, got %s", artifact.ID, foundArtifact.ID)
		}
		if string(content) != "hello world" {
			t.Fatalf("unexpected artifact content: %s", string(content))
		}
	})

	t.Run("Artifact Content Retrieval Rejects Escapes", func(t *testing.T) {
		outsidePath := filepath.Join(t.TempDir(), "outside.log")
		if err := os.WriteFile(outsidePath, []byte("secret"), 0644); err != nil {
			t.Fatal(err)
		}
		artifact := &domain.Artifact{
			ID:        "art-escape",
			AttemptID: stepID + "-a1",
			Type:      domain.ArtifactTypeStdout,
			Name:      "outside.log",
			Path:      outsidePath,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := artifactsRepo.Create(ctx, artifact); err != nil {
			t.Fatal(err)
		}

		_, _, err := svc.GetArtifactContent(ctx, artifact.ID)
		if !errors.Is(err, service.ErrArtifactAccessDenied) {
			t.Fatalf("expected artifact access denial, got %v", err)
		}
	})

	t.Run("Log Retrieval Uses Artifact Lookup", func(t *testing.T) {
		logStepID := "log-step"
		logAttemptID := logStepID + "-a1"
		logStep := &domain.Step{
			ID:      logStepID,
			PhaseID: "phase-01-" + runID,
			Title:   "Log Step",
			Adapter: "mock",
			State:   domain.StepStateCompleted,
		}
		if err := stepsRepo.Create(ctx, logStep); err != nil {
			t.Fatal(err)
		}
		if err := attemptsRepo.Create(ctx, &domain.Attempt{
			ID:        logAttemptID,
			StepID:    logStepID,
			Number:    1,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}); err != nil {
			t.Fatal(err)
		}

		logPath := filepath.Join(artifactRoot, "latest.log")
		if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(logPath, []byte("latest logs"), 0644); err != nil {
			t.Fatal(err)
		}
		artifact := &domain.Artifact{
			ID:        "art-log",
			AttemptID: logAttemptID,
			Type:      domain.ArtifactTypeStdout,
			Name:      "stdout.log",
			Path:      logPath,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := artifactsRepo.Create(ctx, artifact); err != nil {
			t.Fatal(err)
		}

		foundArtifact, content, err := svc.GetLogsByStep(ctx, logStepID)
		if err != nil {
			t.Fatal(err)
		}
		if foundArtifact.ID != artifact.ID {
			t.Fatalf("expected log artifact %s, got %s", artifact.ID, foundArtifact.ID)
		}
		if string(content) != "latest logs" {
			t.Fatalf("unexpected logs content: %s", string(content))
		}
	})

	t.Run("Log Retrieval deterministically prefers latest stdout", func(t *testing.T) {
		logStepID := "deterministic-log-step"
		logAttemptID := logStepID + "-a1"
		logStep := &domain.Step{
			ID:      logStepID,
			PhaseID: "phase-01-" + runID,
			Title:   "Deterministic Log Step",
			Adapter: "mock",
			State:   domain.StepStateCompleted,
		}
		if err := stepsRepo.Create(ctx, logStep); err != nil {
			t.Fatal(err)
		}
		if err := attemptsRepo.Create(ctx, &domain.Attempt{
			ID:        logAttemptID,
			StepID:    logStepID,
			Number:    1,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}); err != nil {
			t.Fatal(err)
		}

		baseTime := time.Now().Add(-3 * time.Minute)
		for _, spec := range []struct {
			id      string
			kind    domain.ArtifactType
			content string
			when    time.Time
		}{
			{id: "stdout-old", kind: domain.ArtifactTypeStdout, content: "stdout-old", when: baseTime},
			{id: "stderr-newer", kind: domain.ArtifactTypeStderr, content: "stderr-newer", when: baseTime.Add(1 * time.Minute)},
			{id: "stdout-new", kind: domain.ArtifactTypeStdout, content: "stdout-new", when: baseTime.Add(2 * time.Minute)},
		} {
			logPath := filepath.Join(artifactRoot, spec.id+".log")
			if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(logPath, []byte(spec.content), 0644); err != nil {
				t.Fatal(err)
			}
			if err := artifactsRepo.Create(ctx, &domain.Artifact{
				ID:        spec.id,
				AttemptID: logAttemptID,
				Type:      spec.kind,
				Name:      spec.id + ".log",
				Path:      logPath,
				CreatedAt: spec.when,
				UpdatedAt: spec.when,
			}); err != nil {
				t.Fatal(err)
			}
		}

		foundArtifact, content, err := svc.GetLogsByStep(ctx, logStepID)
		if err != nil {
			t.Fatal(err)
		}
		if foundArtifact.ID != "stdout-new" {
			t.Fatalf("expected latest stdout artifact, got %s", foundArtifact.ID)
		}
		if string(content) != "stdout-new" {
			t.Fatalf("unexpected logs content: %s", string(content))
		}
	})
}
