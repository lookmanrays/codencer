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

// MockAdapter specifically avoids network and os.exec calls.
type MockAdapter struct{}

func (m *MockAdapter) Start(ctx context.Context, step *domain.Step, attempt *domain.Attempt, workspaceRoot, artifactRoot string) error {
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

func (m *MockAdapter) CollectArtifacts(ctx context.Context, attemptID, artifactRoot string) ([]*domain.Artifact, error) {
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

func (m *FailingMockAdapter) Start(ctx context.Context, step *domain.Step, attempt *domain.Attempt, workspaceRoot, artifactRoot string) error {
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

func (m *FailingMockAdapter) CollectArtifacts(ctx context.Context, attemptID, artifactRoot string) ([]*domain.Artifact, error) {
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
				routingSvc, service.NewPolicyRegistry(), t.TempDir(), t.TempDir())

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
