package service_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"agent-bridge/internal/adapters/codex"
	"agent-bridge/internal/domain"
	"agent-bridge/internal/service"
	"agent-bridge/internal/storage/sqlite"
	_ "github.com/mattn/go-sqlite3"
)

// TestCodexValidationScenario simulates the "Internal Version Bump" scenario.
// It proves the bridge can harvest, hash, and report artifacts correctly.
func TestCodexValidationScenario(t *testing.T) {
	// 1. Setup
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	artifactRoot := filepath.Join(tmpDir, "artifacts")
	workspaceRoot := filepath.Join(tmpDir, "workspace")
	_ = os.MkdirAll(artifactRoot, 0755)
	_ = os.MkdirAll(workspaceRoot, 0755)

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open DB: %v", err)
	}
	defer db.Close()
	if err := sqlite.RunMigrations(db); err != nil {
		t.Fatalf("Migrations failed: %v", err)
	}

	// 2. Initialize Repos and Service
	runsRepo := sqlite.NewRunsRepo(db)
	phasesRepo := sqlite.NewPhasesRepo(db)
	stepsRepo := sqlite.NewStepsRepo(db)
	attemptsRepo := sqlite.NewAttemptsRepo(db)
	gatesRepo := sqlite.NewGatesRepo(db)
	artifactsRepo := sqlite.NewArtifactsRepo(db)
	validationsRepo := sqlite.NewValidationsRepo(db)
	benchmarksRepo := sqlite.NewBenchmarksRepo(db)

	// Use a wrapper that injects files but uses real Codex normalization
	adapter := &ValidationTestAdapter{
		real:         codex.NewAdapter(),
		artifactRoot: artifactRoot,
	}

	adapters := map[string]domain.Adapter{"codex": adapter}
	routingSvc := service.NewRoutingService(benchmarksRepo, adapters)

	runSvc := service.NewRunService(
		runsRepo, phasesRepo, stepsRepo, attemptsRepo,
		gatesRepo, artifactsRepo, validationsRepo,
		routingSvc, service.NewPolicyRegistry(),
		artifactRoot, workspaceRoot,
	)

	// 3. Start Run and Step
	runID := "val-run-01"
	_, _ = runSvc.StartRun(ctx, runID, "val-project", "", "", "")

	step := &domain.Step{
		ID:      "val-step-01",
		PhaseID: "phase-01-" + runID,
		Title:   "Internal Version Bump",
		Adapter: "codex",
		Goal:    "Update internal/app/version.go to v0.1.0-alpha",
	}

	// 4. Dispatch (Execution)
	t.Logf("Dispatching step with artifactRoot: %s", artifactRoot)
	if err := runSvc.DispatchStep(ctx, runID, step); err != nil {
		t.Fatalf("DispatchStep failed: %v", err)
	}

	// 5. Verify Bridge Outcomes
	s, _ := runSvc.GetStep(ctx, step.ID)
	t.Logf("Step state: %s", s.State)
	if s.State != domain.StepStateCompleted {
		// List files for debugging
		files, _ := os.ReadDir(artifactRoot)
		t.Logf("Files in artifactRoot %s:", artifactRoot)
		for _, f := range files {
			t.Logf(" - %s", f.Name())
		}
		t.Fatalf("Expected step state completed, got %s", s.State)
	}

	// 6. Verify Artifact Evidence (Harden Check)
	arts, err := runSvc.GetArtifactsByStep(ctx, step.ID)
	if err != nil || len(arts) == 0 {
		t.Fatalf("Expected artifacts to be persisted, found %d", len(arts))
	}

	foundStdout := false
	foundResult := false
	for _, a := range arts {
		if a.Hash == "" {
			t.Errorf("Artifact %s missing hash", a.Name)
		}
		if a.MimeType == "" {
			t.Errorf("Artifact %s missing mime_type", a.Name)
		}
		if a.Name == "stdout.log" {
			foundStdout = true
		}
		if a.Name == "result.json" {
			foundResult = true
		}
	}
	if !foundStdout || !foundResult {
		t.Errorf("Missing expected artifacts: stdout=%v, result=%v", foundStdout, foundResult)
	}

	// 7. Verify Result Spec (Contract Check)
	res, err := runSvc.GetResultByStep(ctx, step.ID)
	if err != nil {
		t.Fatalf("GetResultByStep failed: %v", err)
	}
	if res.Version != "v1" {
		t.Errorf("Expected result version v1, got %s", res.Version)
	}
	if res.RawOutputRef == "" {
		t.Error("Result missing raw_output_ref")
	}
}

// ValidationTestAdapter mocks the execution but produces real harvested files.
type ValidationTestAdapter struct {
	real         domain.Adapter
	artifactRoot string
}

func (v *ValidationTestAdapter) Name() string         { return "codex" }
func (v *ValidationTestAdapter) Capabilities() []string { return v.real.Capabilities() }

func (v *ValidationTestAdapter) Start(ctx context.Context, step *domain.Step, attempt *domain.Attempt, workspaceRoot, artifactRoot string) error {
	// Simulate Codex execution by writing files to the actual artifactRoot provided by RunService
	if err := os.WriteFile(filepath.Join(artifactRoot, "stdout.log"), []byte("Updating version string...\nDone."), 0644); err != nil {
		return err
	}
	
	res := domain.ResultSpec{
		State:   domain.StepStateCompleted,
		Summary: "Updated internal/app/version.go to v0.1.0-alpha",
	}
	data, _ := json.Marshal(res)
	if err := os.WriteFile(filepath.Join(artifactRoot, "result.json"), data, 0644); err != nil {
		return err
	}
	
	return nil
}

func (v *ValidationTestAdapter) Poll(ctx context.Context, attemptID string) (bool, error) {
	return false, nil // "Exited" immediately
}

func (v *ValidationTestAdapter) Cancel(ctx context.Context, attemptID string) error {
	return nil
}

func (v *ValidationTestAdapter) CollectArtifacts(ctx context.Context, attemptID, artifactRoot string) ([]*domain.Artifact, error) {
	return v.real.CollectArtifacts(ctx, attemptID, artifactRoot)
}

func (v *ValidationTestAdapter) NormalizeResult(ctx context.Context, attemptID string, artifacts []*domain.Artifact) (*domain.ResultSpec, error) {
	// Call NormalizeCore directly from the codex package to ensure version is set correctly
	isSim := false 
	return codex.NormalizeCore(attemptID, artifacts, "codex", isSim)
}
