package service_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

type MockProvisioner struct {
	Log []string
}

func (m *MockProvisioner) Provision(ctx context.Context, spec *domain.ProvisioningSpec, baseRepo, workspaceRoot string) (*domain.ProvisioningResult, error) {
	return &domain.ProvisioningResult{
		Success: true,
		Log:     m.Log,
	}, nil
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

func testRepoRoot(t *testing.T) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return string(bytesTrimSpace(out))
}

func bytesTrimSpace(in []byte) []byte {
	for len(in) > 0 && (in[len(in)-1] == '\n' || in[len(in)-1] == '\r' || in[len(in)-1] == ' ' || in[len(in)-1] == '\t') {
		in = in[:len(in)-1]
	}
	return in
}

func createGitRepo(t *testing.T) string {
	t.Helper()
	repoRoot := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s failed: %v\n%s", args, err, string(out))
		}
	}

	run("git", "init")
	run("git", "config", "user.email", "tests@example.com")
	run("git", "config", "user.name", "Codencer Tests")
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("repo"), 0644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", "README.md")
	run("git", "commit", "-m", "init")
	return repoRoot
}

type cancelAwareAdapter struct {
	running      bool
	cancelCalled bool
}

func (a *cancelAwareAdapter) Name() string           { return "cancel-aware" }
func (a *cancelAwareAdapter) Capabilities() []string { return []string{"mock"} }
func (a *cancelAwareAdapter) Start(ctx context.Context, step *domain.Step, attempt *domain.Attempt, workspaceRoot, attemptArtifactRoot string) error {
	a.running = true
	return nil
}
func (a *cancelAwareAdapter) Poll(ctx context.Context, attemptID string) (bool, error) {
	return a.running, nil
}
func (a *cancelAwareAdapter) Cancel(ctx context.Context, attemptID string) error {
	a.cancelCalled = true
	a.running = false
	return nil
}
func (a *cancelAwareAdapter) CollectArtifacts(ctx context.Context, attemptID, attemptArtifactRoot string) ([]*domain.Artifact, error) {
	return nil, nil
}
func (a *cancelAwareAdapter) NormalizeResult(ctx context.Context, attemptID string, artifacts []*domain.Artifact) (*domain.ResultSpec, error) {
	return &domain.ResultSpec{State: domain.StepStateCompleted, Summary: "should not complete"}, nil
}

type stubbornAdapter struct{}

func (a *stubbornAdapter) Name() string           { return "stubborn" }
func (a *stubbornAdapter) Capabilities() []string { return []string{"mock"} }
func (a *stubbornAdapter) Start(ctx context.Context, step *domain.Step, attempt *domain.Attempt, workspaceRoot, attemptArtifactRoot string) error {
	return nil
}
func (a *stubbornAdapter) Poll(ctx context.Context, attemptID string) (bool, error) { return true, nil }
func (a *stubbornAdapter) Cancel(ctx context.Context, attemptID string) error       { return nil }
func (a *stubbornAdapter) CollectArtifacts(ctx context.Context, attemptID, attemptArtifactRoot string) ([]*domain.Artifact, error) {
	return nil, nil
}
func (a *stubbornAdapter) NormalizeResult(ctx context.Context, attemptID string, artifacts []*domain.Artifact) (*domain.ResultSpec, error) {
	return &domain.ResultSpec{State: domain.StepStateCompleted, Summary: "should not complete"}, nil
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
func (m *PollErrorMockAdapter) Capabilities() []string                             { return []string{"mock"} }
func (m *PollErrorMockAdapter) Name() string                                       { return "poll-error-mock" }
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
	repoRoot := createGitRepo(t)
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
		routingSvc, service.NewPolicyRegistry(), workspace.NewNullProvisioner(), artifactRoot, workspaceRoot, repoRoot,
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
		routingSvcIso, &service.PolicyRegistry{}, workspace.NewNullProvisioner(), artifactRoot, workspaceRoot, repoRoot,
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

func TestRunService_ProvisioningResultPersistence(t *testing.T) {
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
	benchmarksRepo := sqlite.NewBenchmarksRepo(db)

	artifactRoot := t.TempDir()
	workspaceRoot := t.TempDir()

	mockAdapter := &MockAdapter{}
	routingSvc := service.NewRoutingService(benchmarksRepo, map[string]domain.Adapter{
		"mock": mockAdapter,
	})

	// Use a mock provisioner that returns a detectable log
	prov := &MockProvisioner{
		Log: []string{"[DONE] Test Setup"},
	}

	runSvc := service.NewRunService(
		runsRepo, phasesRepo, stepsRepo, attemptsRepo, gatesRepo, artifactsRepo, validationsRepo,
		routingSvc, service.NewPolicyRegistry(), prov, artifactRoot, workspaceRoot,
	)

	runID := "run-prov-persist"
	runSvc.StartRun(ctx, runID, "p1", "c1", "pl1", "ex1")

	step := &domain.Step{
		ID:      "step-prov-1",
		Adapter: "mock",
	}

	err := runSvc.DispatchStep(ctx, runID, step)
	if err != nil {
		t.Fatalf("DispatchStep failed: %v", err)
	}

	res, err := runSvc.GetResultByStep(ctx, step.ID)
	if err != nil {
		t.Fatalf("GetResultByStep failed: %v", err)
	}

	if res.Provisioning == nil {
		t.Fatal("Expected Provisioning metadata to be persisted in final result, got nil")
	}

	if len(res.Provisioning.Log) == 0 || res.Provisioning.Log[0] != "[DONE] Test Setup" {
		t.Errorf("Expected provisioning log '[DONE] Test Setup', got %v", res.Provisioning.Log)
	}
}

type startErrAdapter struct {
	MockAdapter
}

func (a *startErrAdapter) Start(ctx context.Context, step *domain.Step, attempt *domain.Attempt, workspaceRoot, attemptArtifactRoot string) error {
	return fmt.Errorf("simulated start error")
}

func (a *startErrAdapter) Name() string { return "start-err" }

func TestRunService_ProvisioningSurvival_OnStartError(t *testing.T) {
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
	benchmarksRepo := sqlite.NewBenchmarksRepo(db)

	artifactRoot := t.TempDir()
	workspaceRoot := t.TempDir()

	adapter := &startErrAdapter{}
	routingSvc := service.NewRoutingService(benchmarksRepo, map[string]domain.Adapter{
		"start-err": adapter,
	})

	prov := &MockProvisioner{
		Log: []string{"Setup succeeded before crash"},
	}

	runSvc := service.NewRunService(
		runsRepo, phasesRepo, stepsRepo, attemptsRepo, gatesRepo, artifactsRepo, validationsRepo,
		routingSvc, service.NewPolicyRegistry(), prov, artifactRoot, workspaceRoot,
	)

	runID := "run-start-fail"
	runSvc.StartRun(ctx, runID, "p1", "", "", "")

	step := &domain.Step{
		ID:      "step-1",
		Adapter: "start-err",
	}

	_ = runSvc.DispatchStep(ctx, runID, step)

	res, _ := runSvc.GetResultByStep(ctx, step.ID)
	if res == nil {
		t.Fatal("Expected result even on start failure")
	}
	if res.Provisioning == nil {
		t.Fatal("Expected provisioning metadata to survive adapter start failure")
	}
	if len(res.Provisioning.Log) == 0 || res.Provisioning.Log[0] != "Setup succeeded before crash" {
		t.Errorf("Wrong provisioning log: %v", res.Provisioning.Log)
	}
}

func TestRunService_Isolation_ProvisionedWorktree(t *testing.T) {
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
	benchmarksRepo := sqlite.NewBenchmarksRepo(db)

	artifactRoot := t.TempDir()
	workspaceRoot := t.TempDir()

	mockAdapter := &MockAdapter{}
	routingSvc := service.NewRoutingService(benchmarksRepo, map[string]domain.Adapter{
		"mock": mockAdapter,
	})

	runSvc := service.NewRunService(
		runsRepo, phasesRepo, stepsRepo, attemptsRepo, gatesRepo, artifactsRepo, validationsRepo,
		routingSvc, service.NewPolicyRegistry(), &fileProv{}, artifactRoot, workspaceRoot,
	)

	runID := "iso-test"
	runSvc.StartRun(ctx, runID, "p1", "", "", "")

	// Validation that checks for the setup.txt file
	step := &domain.Step{
		ID:      "step-check-iso",
		Adapter: "mock",
		Validations: []domain.ValidationCommand{
			{Name: "check-setup", Command: "cat setup.txt"},
		},
	}

	_ = runSvc.DispatchStep(ctx, runID, step)

	res, _ := runSvc.GetResultByStep(ctx, step.ID)
	if res.State != domain.StepStateCompleted {
		t.Errorf("Expected completed, got %s. Summary: %s", res.State, res.Summary)
	}

	// Verify the validation actually passed
	found := false
	for _, v := range res.Validations {
		if v.Name == "check-setup" && v.Passed {
			found = true
		}
	}
	if !found {
		t.Fatal("Validation 'check-setup' failed or was not found, meaning it could not find 'setup.txt' in the worktree")
	}
}

func TestRunService_DispatchStep_UsesConfiguredRepoRootOutsideCWD(t *testing.T) {
	repoRoot := createGitRepo(t)
	otherDir := t.TempDir()
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(prevWD) }()
	if err := os.Chdir(otherDir); err != nil {
		t.Fatal(err)
	}

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
	validationsRepo := sqlite.NewValidationsRepo(db)
	benchmarksRepo := sqlite.NewBenchmarksRepo(db)

	runSvc := service.NewRunService(
		runsRepo, phasesRepo, stepsRepo, attemptsRepo, gatesRepo, artifactsRepo, validationsRepo,
		service.NewRoutingService(benchmarksRepo, map[string]domain.Adapter{"mock-adapter": &MockAdapter{}}),
		service.NewPolicyRegistry(),
		workspace.NewNullProvisioner(),
		filepath.Join(t.TempDir(), "artifacts"),
		filepath.Join(t.TempDir(), "workspace"),
		repoRoot,
	)

	ctx := context.Background()
	runID := "repo-root-run"
	_, _ = runSvc.StartRun(ctx, runID, "project", "", "", "")
	step := &domain.Step{
		ID:      "repo-root-step",
		PhaseID: "phase-execution-" + runID,
		Title:   "Repo Root Step",
		Adapter: "mock-adapter",
	}

	if err := runSvc.DispatchStep(ctx, runID, step); err != nil {
		t.Fatalf("DispatchStep failed outside repo cwd: %v", err)
	}

	result, err := runSvc.GetResultByStep(ctx, step.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.RunID != runID {
		t.Fatalf("expected result run ID %s, got %s", runID, result.RunID)
	}
}

func TestRunService_AbortRunCancelsActiveAttempt(t *testing.T) {
	repoRoot := testRepoRoot(t)
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
	validationsRepo := sqlite.NewValidationsRepo(db)
	benchmarksRepo := sqlite.NewBenchmarksRepo(db)

	adapter := &cancelAwareAdapter{}
	runSvc := service.NewRunService(
		runsRepo, phasesRepo, stepsRepo, attemptsRepo, gatesRepo, artifactsRepo, validationsRepo,
		service.NewRoutingService(benchmarksRepo, map[string]domain.Adapter{"cancel-aware": adapter}),
		service.NewPolicyRegistry(),
		workspace.NewNullProvisioner(),
		filepath.Join(t.TempDir(), "artifacts"),
		filepath.Join(t.TempDir(), "workspace"),
		repoRoot,
	)

	ctx := context.Background()
	runID := "abort-run"
	_, _ = runSvc.StartRun(ctx, runID, "project", "", "", "")
	step := &domain.Step{
		ID:      "abort-step",
		PhaseID: "phase-execution-" + runID,
		Title:   "Abort Step",
		Adapter: "cancel-aware",
	}

	done := make(chan error, 1)
	go func() { done <- runSvc.DispatchStep(context.Background(), runID, step) }()

	time.Sleep(500 * time.Millisecond)
	if err := runSvc.AbortRun(ctx, runID); err != nil {
		t.Fatalf("AbortRun failed: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("DispatchStep failed after abort: %v", err)
	}
	if !adapter.cancelCalled {
		t.Fatal("expected adapter cancel to be invoked")
	}

	run, _ := runSvc.GetRun(ctx, runID)
	if run.State != domain.RunStateCancelled {
		t.Fatalf("expected run state cancelled, got %s", run.State)
	}
	stepState, _ := runSvc.GetStep(ctx, step.ID)
	if stepState.State != domain.StepStateCancelled {
		t.Fatalf("expected step state cancelled, got %s", stepState.State)
	}
}

func TestRunService_DispatchStepAsyncImmediateAbortCancelsBeforeAdapterStart(t *testing.T) {
	repoRoot := testRepoRoot(t)
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
	validationsRepo := sqlite.NewValidationsRepo(db)
	benchmarksRepo := sqlite.NewBenchmarksRepo(db)

	adapter := &cancelAwareAdapter{}
	runSvc := service.NewRunService(
		runsRepo, phasesRepo, stepsRepo, attemptsRepo, gatesRepo, artifactsRepo, validationsRepo,
		service.NewRoutingService(benchmarksRepo, map[string]domain.Adapter{"cancel-aware": adapter}),
		service.NewPolicyRegistry(),
		workspace.NewNullProvisioner(),
		filepath.Join(t.TempDir(), "artifacts"),
		filepath.Join(t.TempDir(), "workspace"),
		repoRoot,
	)

	ctx := context.Background()
	runID := "async-abort-run"
	_, _ = runSvc.StartRun(ctx, runID, "project", "", "", "")
	step := &domain.Step{
		ID:      "async-abort-step",
		PhaseID: "phase-execution-" + runID,
		Title:   "Async Abort Step",
		Adapter: "cancel-aware",
	}

	if err := runSvc.DispatchStepAsync(ctx, runID, step); err != nil {
		t.Fatalf("DispatchStepAsync failed: %v", err)
	}
	if err := runSvc.AbortRun(ctx, runID); err != nil {
		t.Fatalf("AbortRun failed: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		stepState, _ := runSvc.GetStep(ctx, step.ID)
		if stepState != nil && stepState.State.IsTerminal() {
			if stepState.State != domain.StepStateCancelled {
				t.Fatalf("expected step state cancelled, got %s", stepState.State)
			}
			run, _ := runSvc.GetRun(ctx, runID)
			if run.State != domain.RunStateCancelled {
				t.Fatalf("expected run state cancelled, got %s", run.State)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatal("timed out waiting for async abort to reach a terminal state")
}

func TestRunService_AbortRunWithoutConfirmedStopFailsClosed(t *testing.T) {
	repoRoot := testRepoRoot(t)
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
	validationsRepo := sqlite.NewValidationsRepo(db)
	benchmarksRepo := sqlite.NewBenchmarksRepo(db)

	runSvc := service.NewRunService(
		runsRepo, phasesRepo, stepsRepo, attemptsRepo, gatesRepo, artifactsRepo, validationsRepo,
		service.NewRoutingService(benchmarksRepo, map[string]domain.Adapter{"stubborn": &stubbornAdapter{}}),
		service.NewPolicyRegistry(),
		workspace.NewNullProvisioner(),
		filepath.Join(t.TempDir(), "artifacts"),
		filepath.Join(t.TempDir(), "workspace"),
		repoRoot,
	)

	ctx := context.Background()
	runID := "stubborn-run"
	_, _ = runSvc.StartRun(ctx, runID, "project", "", "", "")
	step := &domain.Step{
		ID:      "stubborn-step",
		PhaseID: "phase-execution-" + runID,
		Title:   "Stubborn Step",
		Adapter: "stubborn",
	}

	done := make(chan error, 1)
	go func() { done <- runSvc.DispatchStep(context.Background(), runID, step) }()

	time.Sleep(500 * time.Millisecond)
	err := runSvc.AbortRun(ctx, runID)
	if err == nil {
		t.Fatal("expected AbortRun to report an unconfirmed cancellation outcome")
	}
	if got := err.Error(); !strings.Contains(got, string(domain.StepStateNeedsManualAttention)) {
		t.Fatalf("expected abort error to mention needs_manual_attention, got %q", got)
	}
	if err := <-done; err != nil {
		t.Fatalf("DispatchStep failed after stubborn abort: %v", err)
	}

	run, _ := runSvc.GetRun(ctx, runID)
	if run.State == domain.RunStateCancelled {
		t.Fatalf("expected stubborn abort to avoid reporting cancelled")
	}
	stepState, _ := runSvc.GetStep(ctx, step.ID)
	if stepState.State != domain.StepStateNeedsManualAttention {
		t.Fatalf("expected step state needs_manual_attention, got %s", stepState.State)
	}
}

func TestRunService_AbortRunWithoutRegisteredExecutionMarksManualAttentionAndReturnsError(t *testing.T) {
	repoRoot := testRepoRoot(t)
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
	validationsRepo := sqlite.NewValidationsRepo(db)
	benchmarksRepo := sqlite.NewBenchmarksRepo(db)

	runSvc := service.NewRunService(
		runsRepo, phasesRepo, stepsRepo, attemptsRepo, gatesRepo, artifactsRepo, validationsRepo,
		service.NewRoutingService(benchmarksRepo, map[string]domain.Adapter{"mock-adapter": &MockAdapter{}}),
		service.NewPolicyRegistry(),
		workspace.NewNullProvisioner(),
		filepath.Join(t.TempDir(), "artifacts"),
		filepath.Join(t.TempDir(), "workspace"),
		repoRoot,
	)

	ctx := context.Background()
	runID := "manual-attention-run"
	_, _ = runSvc.StartRun(ctx, runID, "project", "", "", "")
	step := &domain.Step{
		ID:        "manual-attention-step",
		PhaseID:   "phase-execution-" + runID,
		Title:     "Manual Attention Step",
		Adapter:   "mock-adapter",
		State:     domain.StepStateRunning,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := stepsRepo.Create(ctx, step); err != nil {
		t.Fatalf("create step: %v", err)
	}

	err := runSvc.AbortRun(ctx, runID)
	if err == nil {
		t.Fatal("expected AbortRun to report missing execution registration")
	}
	if got := err.Error(); !strings.Contains(got, "no active execution was registered") {
		t.Fatalf("unexpected abort error: %q", got)
	}

	stepState, _ := runSvc.GetStep(ctx, step.ID)
	if stepState.State != domain.StepStateNeedsManualAttention {
		t.Fatalf("expected step state needs_manual_attention, got %s", stepState.State)
	}

	run, _ := runSvc.GetRun(ctx, runID)
	if run.State != domain.RunStateFailed {
		t.Fatalf("expected run state failed after unresolved abort, got %s", run.State)
	}
}

type warningAdapter struct{}

func (a *warningAdapter) Name() string           { return "warning-adapter" }
func (a *warningAdapter) Capabilities() []string { return []string{"mock"} }
func (a *warningAdapter) Start(ctx context.Context, step *domain.Step, attempt *domain.Attempt, workspaceRoot, attemptArtifactRoot string) error {
	return nil
}
func (a *warningAdapter) Poll(ctx context.Context, attemptID string) (bool, error) { return false, nil }
func (a *warningAdapter) Cancel(ctx context.Context, attemptID string) error       { return nil }
func (a *warningAdapter) CollectArtifacts(ctx context.Context, attemptID, attemptArtifactRoot string) ([]*domain.Artifact, error) {
	return nil, nil
}
func (a *warningAdapter) NormalizeResult(ctx context.Context, attemptID string, artifacts []*domain.Artifact) (*domain.ResultSpec, error) {
	return &domain.ResultSpec{Version: "v1", State: domain.StepStateCompletedWithWarnings, Summary: "completed with warnings"}, nil
}

func TestRunService_DispatchStep_PreservesCompletedWithWarningsAndCompletesRun(t *testing.T) {
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
	validationsRepo := sqlite.NewValidationsRepo(db)
	benchmarksRepo := sqlite.NewBenchmarksRepo(db)

	runSvc := service.NewRunService(
		runsRepo, phasesRepo, stepsRepo, attemptsRepo, gatesRepo, artifactsRepo, validationsRepo,
		service.NewRoutingService(benchmarksRepo, map[string]domain.Adapter{"warning": &warningAdapter{}}),
		service.NewPolicyRegistry(),
		workspace.NewNullProvisioner(),
		filepath.Join(t.TempDir(), "artifacts"),
		filepath.Join(t.TempDir(), "workspace"),
	)

	ctx := context.Background()
	runID := "warning-run"
	_, _ = runSvc.StartRun(ctx, runID, "project", "", "", "")
	step := &domain.Step{
		ID:      "warning-step",
		PhaseID: "phase-execution-" + runID,
		Title:   "Warning Step",
		Adapter: "warning",
	}

	if err := runSvc.DispatchStep(ctx, runID, step); err != nil {
		t.Fatalf("DispatchStep failed: %v", err)
	}

	foundStep, _ := runSvc.GetStep(ctx, step.ID)
	if foundStep.State != domain.StepStateCompletedWithWarnings {
		t.Fatalf("expected step state completed_with_warnings, got %s", foundStep.State)
	}

	run, _ := runSvc.GetRun(ctx, runID)
	if run.State != domain.RunStateCompleted {
		t.Fatalf("expected run state completed, got %s", run.State)
	}
}

type fileProv struct{ MockProvisioner }

func (p *fileProv) Provision(ctx context.Context, spec *domain.ProvisioningSpec, baseRepo, workspaceRoot string) (*domain.ProvisioningResult, error) {
	err := os.WriteFile(filepath.Join(workspaceRoot, "setup.txt"), []byte("ok"), 0644)
	if err != nil {
		return nil, err
	}
	return &domain.ProvisioningResult{Success: true, Log: []string{"Setup file created"}}, nil
}
