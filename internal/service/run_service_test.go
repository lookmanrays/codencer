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

func (m *MockAdapter) Start(ctx context.Context, attempt *domain.Attempt, workspaceRoot, artifactsRoot string) error {
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

func (m *MockAdapter) NormalizeResult(ctx context.Context, attemptID string, artifacts []*domain.Artifact) (*domain.Result, error) {
	// Provide a successful domain result
	return &domain.Result{
		Status:             domain.StepStateNeedsApproval,
		Summary:            "MockAdapter executed successfully",
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

	mockAdapter := &MockAdapter{}
	adapters := map[string]domain.Adapter{
		"mock-adapter": mockAdapter,
	}

	runSvc := service.NewRunService(runsRepo, phasesRepo, stepsRepo, attemptsRepo, gatesRepo, artifactsRepo, adapters)

	ctx := context.Background()

	runId := "isolated-run-1"
	_, err = runSvc.StartRun(ctx, runId, "isolated-project")
	if err != nil {
		t.Fatalf("StartRun failed: %v", err)
	}

	step := &domain.Step{
		ID:      "step-test-1",
		PhaseID: "phase-01-" + runId,
		Title:   "Isolated Step",
		Adapter: "mock-adapter",
	}

	artifactRoot := filepath.Join(t.TempDir(), "artifacts")

	err = runSvc.DispatchStep(ctx, runId, step, artifactRoot)
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
