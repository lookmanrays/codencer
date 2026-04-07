package service_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-bridge/internal/adapters/common"
	"agent-bridge/internal/domain"
	"agent-bridge/internal/service"
	"agent-bridge/internal/storage/sqlite"
	"agent-bridge/internal/workspace"
	_ "github.com/mattn/go-sqlite3"
)

type earlyFailingProvenanceAdapter struct{}

func (a *earlyFailingProvenanceAdapter) Start(ctx context.Context, step *domain.Step, attempt *domain.Attempt, workspaceRoot, attemptArtifactRoot string) error {
	return errors.New("adapter start failed")
}

func (a *earlyFailingProvenanceAdapter) Poll(ctx context.Context, attemptID string) (bool, error) {
	return false, nil
}

func (a *earlyFailingProvenanceAdapter) Cancel(ctx context.Context, attemptID string) error {
	return nil
}

func (a *earlyFailingProvenanceAdapter) Capabilities() []string {
	return []string{"mock"}
}

func (a *earlyFailingProvenanceAdapter) Name() string {
	return "early-failing-provenance"
}

func (a *earlyFailingProvenanceAdapter) CollectArtifacts(ctx context.Context, attemptID, attemptArtifactRoot string) ([]*domain.Artifact, error) {
	return common.CollectStandardArtifacts(ctx, attemptID, attemptArtifactRoot)
}

func (a *earlyFailingProvenanceAdapter) NormalizeResult(ctx context.Context, attemptID string, artifacts []*domain.Artifact) (*domain.ResultSpec, error) {
	return nil, errors.New("normalize should not be called for start failure")
}

func TestRunService_PersistsSubmissionProvenanceOnEarlyFailure(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	if err := sqlite.RunMigrations(db); err != nil {
		t.Fatalf("migrations failed: %v", err)
	}

	runsRepo := sqlite.NewRunsRepo(db)
	phasesRepo := sqlite.NewPhasesRepo(db)
	stepsRepo := sqlite.NewStepsRepo(db)
	attemptsRepo := sqlite.NewAttemptsRepo(db)
	gatesRepo := sqlite.NewGatesRepo(db)
	artifactsRepo := sqlite.NewArtifactsRepo(db)
	validationsRepo := sqlite.NewValidationsRepo(db)
	benchmarksRepo := sqlite.NewBenchmarksRepo(db)

	adapter := &earlyFailingProvenanceAdapter{}
	routingSvc := service.NewRoutingService(benchmarksRepo, map[string]domain.Adapter{
		"provenance-fail": adapter,
	})
	policyRegistry := service.NewPolicyRegistry()
	noRetryPolicy := domain.DefaultPolicy()
	noRetryPolicy.Name = "no_retry"
	noRetryPolicy.RetryWhen.AdapterProcessFailed = false
	policyRegistry.Register(noRetryPolicy)

	artifactRoot := t.TempDir()
	workspaceRoot := t.TempDir()
	runSvc := service.NewRunService(
		runsRepo,
		phasesRepo,
		stepsRepo,
		attemptsRepo,
		gatesRepo,
		artifactsRepo,
		validationsRepo,
		routingSvc,
		policyRegistry,
		workspace.NewNullProvisioner(),
		artifactRoot,
		workspaceRoot,
	)

	ctx := context.Background()
	runID := "run-provenance-" + strings.ReplaceAll(strings.ToLower(t.Name()), "/", "-")
	if _, err := runSvc.StartRun(ctx, runID, "project", "", "", ""); err != nil {
		t.Fatalf("StartRun failed: %v", err)
	}

	provenance := &domain.SubmissionProvenance{
		SourceKind:      domain.SubmissionSourceGoal,
		SourceName:      "inline-goal",
		OriginalFormat:  "txt",
		OriginalInput:   "Fix the failing tests in package X",
		DefaultsApplied: []string{"version", "title"},
	}
	taskSnapshot := &domain.TaskSpec{
		Version:              "v1",
		RunID:                runID,
		PhaseID:              "phase-execution-" + runID,
		StepID:               "step-provenance",
		Title:                "Direct task",
		Goal:                 "Fix the failing tests in package X",
		AdapterProfile:       "provenance-fail",
		SubmissionProvenance: provenance,
	}
	step := &domain.Step{
		ID:                   "step-provenance",
		PhaseID:              "phase-execution-" + runID,
		Title:                "Direct task",
		Goal:                 "Fix the failing tests in package X",
		Adapter:              "provenance-fail",
		Policy:               "no_retry",
		TaskSpecSnapshot:     taskSnapshot,
		SubmissionProvenance: provenance,
	}

	if err := runSvc.DispatchStep(ctx, runID, step); err != nil {
		t.Fatalf("DispatchStep failed: %v", err)
	}

	artifacts, err := runSvc.GetArtifactsByStep(ctx, step.ID)
	if err != nil {
		t.Fatalf("GetArtifactsByStep failed: %v", err)
	}
	if len(artifacts) < 2 {
		t.Fatalf("expected provenance artifacts, got %d", len(artifacts))
	}

	result, err := runSvc.GetResultByStep(ctx, step.ID)
	if err != nil {
		t.Fatalf("GetResultByStep failed: %v", err)
	}
	if result.Artifacts["normalized_task_ref"] == "" || result.Artifacts["original_input_ref"] == "" {
		t.Fatalf("expected provenance artifact refs, got %+v", result.Artifacts)
	}

	normalizedBytes, err := os.ReadFile(result.Artifacts["normalized_task_ref"])
	if err != nil {
		t.Fatalf("read normalized task: %v", err)
	}
	if !strings.Contains(string(normalizedBytes), "\"submission_provenance\"") || !strings.Contains(string(normalizedBytes), "\"goal\": \"Fix the failing tests in package X\"") {
		t.Fatalf("unexpected normalized task content: %s", string(normalizedBytes))
	}

	originalBytes, err := os.ReadFile(result.Artifacts["original_input_ref"])
	if err != nil {
		t.Fatalf("read original input: %v", err)
	}
	if string(originalBytes) != "Fix the failing tests in package X" {
		t.Fatalf("unexpected original input content: %q", string(originalBytes))
	}
}

type successProvenanceAdapter struct{}

func (a *successProvenanceAdapter) Start(ctx context.Context, step *domain.Step, attempt *domain.Attempt, workspaceRoot, attemptArtifactRoot string) error {
	for _, name := range []string{"normalized-task.json", "original-input.txt"} {
		path := filepath.Join(attemptArtifactRoot, name)
		if _, err := os.Stat(path); err != nil {
			return errors.New("missing provenance file before adapter start")
		}
	}
	return nil
}

func (a *successProvenanceAdapter) Poll(ctx context.Context, attemptID string) (bool, error) {
	return true, nil
}

func (a *successProvenanceAdapter) Cancel(ctx context.Context, attemptID string) error {
	return nil
}

func (a *successProvenanceAdapter) Capabilities() []string {
	return []string{"mock"}
}

func (a *successProvenanceAdapter) Name() string {
	return "success-provenance"
}

func (a *successProvenanceAdapter) CollectArtifacts(ctx context.Context, attemptID, attemptArtifactRoot string) ([]*domain.Artifact, error) {
	return common.CollectStandardArtifacts(ctx, attemptID, attemptArtifactRoot)
}

func (a *successProvenanceAdapter) NormalizeResult(ctx context.Context, attemptID string, artifacts []*domain.Artifact) (*domain.ResultSpec, error) {
	return &domain.ResultSpec{
		State:   domain.StepStateCompleted,
		Summary: "Task succeeded",
	}, nil
}

func TestRunService_PersistsSubmissionProvenanceOnSuccess(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test-success.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	if err := sqlite.RunMigrations(db); err != nil {
		t.Fatalf("migrations failed: %v", err)
	}

	runsRepo := sqlite.NewRunsRepo(db)
	phasesRepo := sqlite.NewPhasesRepo(db)
	stepsRepo := sqlite.NewStepsRepo(db)
	attemptsRepo := sqlite.NewAttemptsRepo(db)
	gatesRepo := sqlite.NewGatesRepo(db)
	artifactsRepo := sqlite.NewArtifactsRepo(db)
	validationsRepo := sqlite.NewValidationsRepo(db)
	benchmarksRepo := sqlite.NewBenchmarksRepo(db)

	adapter := &successProvenanceAdapter{}
	routingSvc := service.NewRoutingService(benchmarksRepo, map[string]domain.Adapter{
		"provenance-success": adapter,
	})
	policyRegistry := service.NewPolicyRegistry()
	policyRegistry.Register(domain.DefaultPolicy())

	artifactRoot := t.TempDir()
	workspaceRoot := t.TempDir()
	runSvc := service.NewRunService(
		runsRepo,
		phasesRepo,
		stepsRepo,
		attemptsRepo,
		gatesRepo,
		artifactsRepo,
		validationsRepo,
		routingSvc,
		policyRegistry,
		workspace.NewNullProvisioner(),
		artifactRoot,
		workspaceRoot,
	)

	ctx := context.Background()
	runID := "run-success-" + strings.ReplaceAll(strings.ToLower(t.Name()), "/", "-")
	if _, err := runSvc.StartRun(ctx, runID, "project", "", "", ""); err != nil {
		t.Fatalf("StartRun failed: %v", err)
	}

	provenance := &domain.SubmissionProvenance{
		SourceKind:     domain.SubmissionSourceGoal,
		SourceName:     "inline-goal",
		OriginalFormat: "txt",
		OriginalInput:  "Success goal",
	}
	step := &domain.Step{
		ID:                   "step-success",
		PhaseID:              "phase-execution-" + runID,
		Title:                "Success task",
		Goal:                 "Success goal",
		Adapter:              "provenance-success",
		Policy:               "default",
		SubmissionProvenance: provenance,
	}

	if err := runSvc.DispatchStep(ctx, runID, step); err != nil {
		t.Fatalf("DispatchStep failed: %v", err)
	}

	result, err := runSvc.GetResultByStep(ctx, step.ID)
	if err != nil {
		t.Fatalf("GetResultByStep failed: %v", err)
	}

	if result.Artifacts["normalized_task_ref"] == "" || result.Artifacts["original_input_ref"] == "" {
		t.Fatalf("expected provenance artifact refs on success, got %+v", result.Artifacts)
	}

	// Verify result enrichment
	if result.RunID != runID {
		t.Errorf("expected RunID %s, got %s", runID, result.RunID)
	}
	if result.PhaseID != step.PhaseID {
		t.Errorf("expected PhaseID %s, got %s", step.PhaseID, result.PhaseID)
	}
	if result.StepID != step.ID {
		t.Errorf("expected StepID %s, got %s", step.ID, result.StepID)
	}
	if result.Adapter != "provenance-success" {
		t.Errorf("expected Adapter provenance-success, got %s", result.Adapter)
	}
}

func TestRunService_PersistsMultilineProvenance(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test-multiline.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	if err := sqlite.RunMigrations(db); err != nil {
		t.Fatalf("migrations failed: %v", err)
	}

	runsRepo := sqlite.NewRunsRepo(db)
	phasesRepo := sqlite.NewPhasesRepo(db)
	stepsRepo := sqlite.NewStepsRepo(db)
	attemptsRepo := sqlite.NewAttemptsRepo(db)
	gatesRepo := sqlite.NewGatesRepo(db)
	artifactsRepo := sqlite.NewArtifactsRepo(db)
	validationsRepo := sqlite.NewValidationsRepo(db)
	benchmarksRepo := sqlite.NewBenchmarksRepo(db)

	adapter := &successProvenanceAdapter{}
	routingSvc := service.NewRoutingService(benchmarksRepo, map[string]domain.Adapter{
		"success": adapter,
	})
	policyRegistry := service.NewPolicyRegistry()
	policyRegistry.Register(domain.DefaultPolicy())

	artifactRoot := t.TempDir()
	workspaceRoot := t.TempDir()
	runSvc := service.NewRunService(
		runsRepo,
		phasesRepo,
		stepsRepo,
		attemptsRepo,
		gatesRepo,
		artifactsRepo,
		validationsRepo,
		routingSvc,
		policyRegistry,
		workspace.NewNullProvisioner(),
		artifactRoot,
		workspaceRoot,
	)

	ctx := context.Background()
	runID := "run-multiline-" + strings.ReplaceAll(strings.ToLower(t.Name()), "/", "-")
	if _, err := runSvc.StartRun(ctx, runID, "project", "", "", ""); err != nil {
		t.Fatalf("StartRun failed: %v", err)
	}

	multilineGoal := "Line 1\nLine 2\nLine 3"
	provenance := &domain.SubmissionProvenance{
		SourceKind:     domain.SubmissionSourceStdin,
		SourceName:     "stdin",
		OriginalFormat: "txt",
		OriginalInput:  multilineGoal,
	}
	step := &domain.Step{
		ID:                   "step-multiline",
		PhaseID:              "phase-execution-" + runID,
		Title:                "Multiline task",
		Goal:                 multilineGoal,
		Adapter:              "success",
		Policy:               "default",
		SubmissionProvenance: provenance,
	}

	if err := runSvc.DispatchStep(ctx, runID, step); err != nil {
		t.Fatalf("DispatchStep failed: %v", err)
	}

	result, err := runSvc.GetResultByStep(ctx, step.ID)
	if err != nil {
		t.Fatalf("GetResultByStep failed: %v", err)
	}

	originalBytes, err := os.ReadFile(result.Artifacts["original_input_ref"])
	if err != nil {
		t.Fatalf("read original input: %v", err)
	}
	if string(originalBytes) != multilineGoal {
		t.Fatalf("unexpected original input content: %q", string(originalBytes))
	}
}
