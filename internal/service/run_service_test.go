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

// MockAdapter specifically avoids network and os.exec calls.
type MockAdapter struct{}

func (m *MockAdapter) Start(ctx context.Context, step *domain.Step, attempt *domain.Attempt, workspaceRoot, attemptArtifactRoot string) error {
	return nil
}

func (m *MockAdapter) Poll(ctx context.Context, attemptID string) (bool, error) {
	return false, nil // Return false to indicate the process has exited
}

func (m *MockAdapter) Cancel(ctx context.Context, attemptID string) error {
	return nil
}

func (m *MockAdapter) Capabilities() []string {
	return []string{"mock"}
}

func (m *MockAdapter) Name() string {
	return "mock-adapter"
}

func (m *MockAdapter) CollectArtifacts(ctx context.Context, attemptID, attemptArtifactRoot string) ([]*domain.Artifact, error) {
	arts := []*domain.Artifact{
		{
			ID:        "art-1",
			AttemptID: attemptID,
			Type:      domain.ArtifactType("result.json"),
			Path:      "result.json",
			Size:      120,
			CreatedAt: time.Now(),
		},
	}
	return arts, nil
}

func (m *MockAdapter) NormalizeResult(ctx context.Context, attemptID string, artifacts []*domain.Artifact) (*domain.ResultSpec, error) {
	// Provide a successful domain result
	return &domain.ResultSpec{
		State:              domain.StepStateCompleted,
		Summary:            "Test Result",
		NeedsHumanDecision: true,
		Questions:          []string{"Mock adapter isolated gate test question"},
	}, nil
}

func TestRunService_DispatchStep_Isolated(t *testing.T) {
	// File-based sqlite for isolation to avoid connection pool deadlocks.
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open DB: %v", err)
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
	benchmarksRepo := sqlite.NewBenchmarksRepo(db)

	mockAdapter := &MockAdapter{}
	adapters := map[string]domain.Adapter{
		"mock-adapter": mockAdapter,
	}

	routingSvc := service.NewRoutingService(benchmarksRepo, adapters)

	artifactRoot := t.TempDir()
	workspaceRoot := t.TempDir()
	runSvc := service.NewRunService(runsRepo, phasesRepo, stepsRepo, attemptsRepo,
		gatesRepo,
		artifactsRepo,
		sqlite.NewValidationsRepo(db),
		routingSvc,
		service.NewPolicyRegistry(),
		workspace.NewNullProvisioner(),
		artifactRoot, workspaceRoot)

	ctx := context.Background()

	runId := "isolated-run-1"
	_, err = runSvc.StartRun(ctx, runId, "isolated-project", "", "", "")
	if err != nil {
		t.Fatalf("StartRun failed: %v", err)
	}

	step := &domain.Step{
		ID:      "step-test-1",
		PhaseID: "phase-01-" + runId,
		Title:   "Isolated Step",
		Adapter: "mock-adapter",
	}


	err = runSvc.DispatchStep(ctx, runId, step)
	if err != nil {
		t.Fatalf("DispatchStep failed: %v", err)
	}

	s, err := runSvc.GetStep(ctx, step.ID)
	if err != nil {
		t.Fatalf("GetStep failed: %v", err)
	}

	if s.State != domain.StepStateNeedsApproval {
		t.Fatalf("Expected step state NeedsApproval, got %s", s.State)
	}

	arts, err := runSvc.GetArtifactsByStep(ctx, step.ID)
	if err != nil {
		t.Fatalf("GetArtifactsByStep failed: %v", err)
	}
	if len(arts) == 0 {
		t.Fatalf("Expected artifacts to be persisted")
	}
}

// FailingMockAdapter simulates various terminal failure states
type FailingMockAdapter struct {
	FailState domain.StepState
}

func (m *FailingMockAdapter) Start(ctx context.Context, step *domain.Step, attempt *domain.Attempt, workspaceRoot, attemptArtifactRoot string) error {
	return nil
}

func (m *FailingMockAdapter) Poll(ctx context.Context, attemptID string) (bool, error) {
	return false, nil
}

func (m *FailingMockAdapter) Cancel(ctx context.Context, attemptID string) error {
	return nil
}

func (m *FailingMockAdapter) Capabilities() []string {
	return []string{"mock"}
}

func (m *FailingMockAdapter) Name() string {
	return "failing-mock"
}

func (m *FailingMockAdapter) CollectArtifacts(ctx context.Context, attemptID, attemptArtifactRoot string) ([]*domain.Artifact, error) {
	return nil, nil
}

func (m *FailingMockAdapter) NormalizeResult(ctx context.Context, attemptID string, artifacts []*domain.Artifact) (*domain.ResultSpec, error) {
	return &domain.ResultSpec{
		State:   m.FailState,
		Summary: "Simulated failure",
	}, nil
}

func TestRunService_DispatchStep_GranularFailurePreservation(t *testing.T) {
	tests := []struct {
		name          string
		failState     domain.StepState
		expectedState domain.StepState
	}{
		{
			name:          "Preserve-FailedBridge",
			failState:     domain.StepStateFailedBridge,
			expectedState: domain.StepStateFailedBridge,
		},
		{
			name:          "Preserve-FailedValidation",
			failState:     domain.StepStateFailedValidation,
			expectedState: domain.StepStateFailedValidation,
		},
		{
			name:          "Preserve-Timeout",
			failState:     domain.StepStateTimeout,
			expectedState: domain.StepStateTimeout,
		},
		{
			name:          "Collapse-Unknown-to-FailedAdapter",
			failState:     domain.StepState("unknown_failure"),
			expectedState: domain.StepStateFailedAdapter,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "test.db")
			db, _ := sql.Open("sqlite3", dbPath)
			defer db.Close()
			_ = sqlite.RunMigrations(db)

			runsRepo := sqlite.NewRunsRepo(db)
			phasesRepo := sqlite.NewPhasesRepo(db)
			stepsRepo := sqlite.NewStepsRepo(db)
			attemptsRepo := sqlite.NewAttemptsRepo(db)
			gatesRepo := sqlite.NewGatesRepo(db)
			artifactsRepo := sqlite.NewArtifactsRepo(db)
			benchmarksRepo := sqlite.NewBenchmarksRepo(db)

			failingAdapter := &FailingMockAdapter{FailState: tt.failState}
			adapters := map[string]domain.Adapter{
				"failing-mock": failingAdapter,
			}

			routingSvc := service.NewRoutingService(benchmarksRepo, adapters)
			runSvc := service.NewRunService(runsRepo, phasesRepo, stepsRepo, attemptsRepo,
				gatesRepo, artifactsRepo, sqlite.NewValidationsRepo(db),
				routingSvc, service.NewPolicyRegistry(), workspace.NewNullProvisioner(), t.TempDir(), t.TempDir())

			ctx := context.Background()
			runId := "fail-test-" + tt.name
			_, _ = runSvc.StartRun(ctx, runId, "fail-project", "", "", "")

			step := &domain.Step{
				ID:      "step-" + tt.name,
				PhaseID: "phase-01-" + runId,
				Title:   "Failing Step",
				Adapter: "failing-mock",
			}

			_ = runSvc.DispatchStep(ctx, runId, step)

			s, _ := runSvc.GetStep(ctx, step.ID)
			if s.State != tt.expectedState {
				t.Errorf("Expected state %s, got %s", tt.expectedState, s.State)
			}
			if s.StatusReason != "Simulated failure" {
				t.Errorf("Expected status reason 'Simulated failure', got %s", s.StatusReason)
			}
		})
	}
}

// PollErrorMockAdapter simulates a transport failure during polling
type PollErrorMockAdapter struct{}

func (m *PollErrorMockAdapter) Start(ctx context.Context, step *domain.Step, attempt *domain.Attempt, workspaceRoot, attemptArtifactRoot string) error {
	return nil
}
func (m *PollErrorMockAdapter) Poll(ctx context.Context, attemptID string) (bool, error) {
	return false, errors.New("Poll failed") // Just some error
}
func (m *PollErrorMockAdapter) Cancel(ctx context.Context, attemptID string) error { return nil }
func (m *PollErrorMockAdapter) Capabilities() []string                           { return []string{"mock"} }
func (m *PollErrorMockAdapter) Name() string                                     { return "poll-error-mock" }
func (m *PollErrorMockAdapter) CollectArtifacts(ctx context.Context, attemptID, attemptArtifactRoot string) ([]*domain.Artifact, error) {
	return nil, nil
}
func (m *PollErrorMockAdapter) NormalizeResult(ctx context.Context, attemptID string, artifacts []*domain.Artifact) (*domain.ResultSpec, error) {
	return nil, nil
}

func TestRunService_DispatchStep_PollErrorMapping(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, _ := sql.Open("sqlite3", dbPath)
	defer db.Close()
	_ = sqlite.RunMigrations(db)

	runsRepo := sqlite.NewRunsRepo(db)
	phasesRepo := sqlite.NewPhasesRepo(db)
	stepsRepo := sqlite.NewStepsRepo(db)
	attemptsRepo := sqlite.NewAttemptsRepo(db)
	gatesRepo := sqlite.NewGatesRepo(db)
	artifactsRepo := sqlite.NewArtifactsRepo(db)
	benchmarksRepo := sqlite.NewBenchmarksRepo(db)

	adapters := map[string]domain.Adapter{
		"poll-error-mock": &PollErrorMockAdapter{},
	}

	routingSvc := service.NewRoutingService(benchmarksRepo, adapters)
	runSvc := service.NewRunService(runsRepo, phasesRepo, stepsRepo, attemptsRepo,
		gatesRepo, artifactsRepo, sqlite.NewValidationsRepo(db),
		routingSvc, service.NewPolicyRegistry(), workspace.NewNullProvisioner(), t.TempDir(), t.TempDir())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runId := "poll-error-run"
	_, _ = runSvc.StartRun(ctx, runId, "project", "", "", "")

	step := &domain.Step{
		ID:      "poll-error-step",
		PhaseID: "phase-01-" + runId,
		Title:   "Poll Error Test",
		Adapter: "poll-error-mock",
	}

	_ = runSvc.DispatchStep(ctx, runId, step)

	s, _ := runSvc.GetStep(ctx, step.ID)
	if s.State != domain.StepStateFailedAdapter {
		t.Errorf("Expected state %s for poll error, got %s", domain.StepStateFailedAdapter, s.State)
	}
}

func TestRunService_DispatchStep_ImmutableNamespacing(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, _ := sql.Open("sqlite3", dbPath)
	defer db.Close()
	_ = sqlite.RunMigrations(db)

	runsRepo := sqlite.NewRunsRepo(db)
	phasesRepo := sqlite.NewPhasesRepo(db)
	stepsRepo := sqlite.NewStepsRepo(db)
	attemptsRepo := sqlite.NewAttemptsRepo(db)
	gatesRepo := sqlite.NewGatesRepo(db)
	artifactsRepo := sqlite.NewArtifactsRepo(db)
	benchmarksRepo := sqlite.NewBenchmarksRepo(db)

	// Custom MockAdapter that writes unique content to stdout.log
	mockAdapter := &MockAdapter{} // We can reuse but we'll manually check paths
	adapters := map[string]domain.Adapter{
		"mock": mockAdapter,
	}

	artifactRoot := t.TempDir()
	workspaceRoot := t.TempDir()
	routingSvc := service.NewRoutingService(benchmarksRepo, adapters)
	runSvc := service.NewRunService(runsRepo, phasesRepo, stepsRepo, attemptsRepo,
		gatesRepo, artifactsRepo, sqlite.NewValidationsRepo(db),
		routingSvc, service.NewPolicyRegistry(), workspace.NewNullProvisioner(),
		artifactRoot, workspaceRoot)

	ctx := context.Background()
	runID := "namespace-run"
	_, _ = runSvc.StartRun(ctx, runID, "project", "", "", "")

	step := &domain.Step{
		ID:      "namespace-step",
		PhaseID: "phase-execution-" + runID,
		Title:   "Namespace Test",
		Adapter: "mock",
	}

	// Dispatch Attempt 1
	t.Log("Dispatching Attempt 1")
	if err := runSvc.DispatchStep(ctx, runID, step); err != nil {
		t.Fatalf("DispatchStep 1 failed: %v", err)
	}
	t.Log("Attempt 1 finished")
	attempts, _ := attemptsRepo.ListByStep(ctx, step.ID)
	if len(attempts) != 1 {
		t.Fatalf("Expected 1 attempt, got %d", len(attempts))
	}
	attempt1 := attempts[0]
	path1 := filepath.Join(artifactRoot, runID, step.ID, attempt1.ID)
	if _, err := os.Stat(path1); err != nil {
		t.Errorf("Expected artifact dir for Attempt 1 to exist: %v", err)
	}

	// Dispatch Attempt 2 (simulate same step being picked up again or retry)
	// Reset step state to pending for redispatch simulation
	step.State = domain.StepStatePending
	t.Log("Dispatching Attempt 2")
	if err := runSvc.DispatchStep(ctx, runID, step); err != nil {
		t.Fatalf("DispatchStep 2 failed: %v", err)
	}
	t.Log("Attempt 2 finished")
	
	attempts, _ = attemptsRepo.ListByStep(ctx, step.ID)
	if len(attempts) != 2 {
		t.Fatalf("Expected 2 attempts, got %d", len(attempts))
	}
	attempt2 := attempts[1]
	path2 := filepath.Join(artifactRoot, runID, step.ID, attempt2.ID)
	
	if _, err := os.Stat(path2); err != nil {
		t.Errorf("Expected artifact dir for Attempt 2 to exist: %v", err)
	}

	if path1 == path2 {
		t.Errorf("Artifact paths for Attempt 1 and 2 must be different, both got %s", path1)
	}
}

// Local mock for validation tests
type valMockAdapter struct {
	res            *domain.ResultSpec
	WorkspaceFiles map[string]string
}

func (a *valMockAdapter) Start(ctx context.Context, step *domain.Step, attempt *domain.Attempt, workspaceRoot, attemptArtifactRoot string) error {
	for name, content := range a.WorkspaceFiles {
		path := filepath.Join(workspaceRoot, name)
		_ = os.MkdirAll(filepath.Dir(path), 0755)
		_ = os.WriteFile(path, []byte(content), 0644)
	}
	return nil
}
func (m *valMockAdapter) Poll(ctx context.Context, attemptID string) (bool, error) {
	return false, nil
}
func (m *valMockAdapter) Cancel(ctx context.Context, attemptID string) error {
	return nil
}
func (m *valMockAdapter) Capabilities() []string {
	return []string{"mock"}
}
func (m *valMockAdapter) Name() string {
	return "val-mock"
}
func (m *valMockAdapter) CollectArtifacts(ctx context.Context, attemptID, attemptArtifactRoot string) ([]*domain.Artifact, error) {
	return nil, nil
}
func (m *valMockAdapter) NormalizeResult(ctx context.Context, attemptID string, artifacts []*domain.Artifact) (*domain.ResultSpec, error) {
	return m.res, nil
}

func TestRunService_DispatchStep_WithValidations(t *testing.T) {
	ctx := context.Background()
	db, _ := sql.Open("sqlite3", ":memory:")
	sqlite.RunMigrations(db)

	runsRepo := sqlite.NewRunsRepo(db)
	phasesRepo := sqlite.NewPhasesRepo(db)
	stepsRepo := sqlite.NewStepsRepo(db)
	attemptsRepo := sqlite.NewAttemptsRepo(db)
	gatesRepo := sqlite.NewGatesRepo(db)
	artifactsRepo := sqlite.NewArtifactsRepo(db)
	validationsRepo := sqlite.NewValidationsRepo(db)

	artifactRoot := t.TempDir()
	workspaceRoot := t.TempDir()

	// Inject mock adapter via RoutingService
	mock := &valMockAdapter{
		res: &domain.ResultSpec{State: domain.StepStateCompleted, Summary: "Success"},
	}
	benchmarksRepo := sqlite.NewBenchmarksRepo(db)
	routingSvc := service.NewRoutingService(benchmarksRepo, map[string]domain.Adapter{
		"mock": mock,
	})

	// Use service. prefix for package-level types and functions
	runSvc := service.NewRunService(
		runsRepo, phasesRepo, stepsRepo, attemptsRepo, gatesRepo, artifactsRepo, validationsRepo,
		routingSvc, service.NewPolicyRegistry(), workspace.NewNullProvisioner(), artifactRoot, workspaceRoot,
	)

	runID := "run-val-1"
	runSvc.StartRun(ctx, runID, "p1", "c1", "pl1", "ex1")

	// Case 1: Multiple validations, all pass
	step := &domain.Step{
		ID:      "step-val-pass",
		Adapter: "mock",
		Validations: []domain.ValidationCommand{
			{Name: "v1", Command: "true"},
			{Name: "v2", Command: "ls"},
		},
	}

	_ = runSvc.DispatchStep(ctx, runID, step)

	if step.State != domain.StepStateCompleted {
		t.Errorf("Expected StepStateCompleted, got %s", step.State)
	}

	res, _ := runSvc.GetResultByStep(ctx, step.ID)
	if len(res.Validations) != 2 {
		t.Errorf("Expected 2 validation results, got %d", len(res.Validations))
	}

	for _, v := range res.Validations {
		if !v.Passed {
			t.Errorf("Validation %s failed but should have passed", v.Name)
		}
	}

	// Case 2: One validation fails
	stepFail := &domain.Step{
		ID:      "step-val-fail",
		Adapter: "mock",
		Validations: []domain.ValidationCommand{
			{Name: "pass", Command: "true"},
			{Name: "fail", Command: "false"},
		},
	}

	_ = runSvc.DispatchStep(ctx, runID, stepFail)

	if stepFail.State != domain.StepStateFailedValidation {
		t.Errorf("Expected StepStateFailedValidation, got %s", stepFail.State)
	}

	resFail, _ := runSvc.GetResultByStep(ctx, stepFail.ID)
	if resFail.State != domain.StepStateFailedValidation {
		t.Errorf("ResultSpec.State should be failed_validation, got %s", resFail.State)
	}

	failedFound := false
	for _, v := range resFail.Validations {
		if v.Name == "fail" && !v.Passed {
			failedFound = true
		}
	}
	if !failedFound {
		t.Error("Expected failing validation result in ResultSpec")
	}

	// Manually create the isolator file in the expected workspace path
	// We'll use the adapter to inject it during Start
	isoAdapter := &valMockAdapter{
		res: &domain.ResultSpec{State: domain.StepStateCompleted},
		WorkspaceFiles: map[string]string{
			"isolator.txt": "isolated-truth",
		},
	}
	routingSvcIso := service.NewRoutingService(benchmarksRepo, map[string]domain.Adapter{"mock-iso": isoAdapter})
	runSvcIso := service.NewRunService(
		runsRepo, phasesRepo, stepsRepo, attemptsRepo, gatesRepo, artifactsRepo, validationsRepo,
		routingSvcIso, &service.PolicyRegistry{}, workspace.NewNullProvisioner(), artifactRoot, workspaceRoot,
	)

	stepIso := &domain.Step{
		ID:      "step-val-iso",
		Adapter: "mock-iso",
		Validations: []domain.ValidationCommand{
			{Name: "check-iso", Command: "cat isolator.txt"},
		},
	}

	_ = runSvcIso.DispatchStep(ctx, runID, stepIso)

	if stepIso.State != domain.StepStateCompleted {
		t.Errorf("Expected StepStateCompleted for isolation check, got %s", stepIso.State)
	}

	resIso, _ := runSvc.GetResultByStep(ctx, stepIso.ID)
	foundIso := false
	for _, v := range resIso.Validations {
		if v.Name == "check-iso" && v.Passed {
			foundIso = true
		}
	}
	if !foundIso {
		t.Error("Validation failed to find isolator.txt in attempt workspace")
	}
}
