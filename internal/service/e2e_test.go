package service_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"agent-bridge/internal/adapters/codex"
	"agent-bridge/internal/domain"
	"agent-bridge/internal/service"
	"agent-bridge/internal/storage/sqlite"
	_ "github.com/mattn/go-sqlite3"
)

// TestE2EFlow validates the orchestrator's state machine and lifecycle loop.
// NOTE: This test uses Simulation Mode and does NOT verify real agent/binary behavior.
func TestE2EFlow(t *testing.T) {
	// 1. Setup Environment
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open DB: %v", err)
	}
	defer db.Close()
	if err := sqlite.RunMigrations(db); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	runsRepo := sqlite.NewRunsRepo(db)
	phasesRepo := sqlite.NewPhasesRepo(db)
	stepsRepo := sqlite.NewStepsRepo(db)
	attemptsRepo := sqlite.NewAttemptsRepo(db)
	gatesRepo := sqlite.NewGatesRepo(db)

	artifactsRepo := sqlite.NewArtifactsRepo(db)
	validationsRepo := sqlite.NewValidationsRepo(db)
	benchmarksRepo := sqlite.NewBenchmarksRepo(db)

	adapters := map[string]domain.Adapter{
		"codex": codex.NewAdapter(),
	}

	routingSvc := service.NewRoutingService(benchmarksRepo, adapters)

	artifactRoot := "/tmp/codencer/artifacts"
	workspaceRoot := "/tmp/codencer/workspace"
	runSvc := service.NewRunService(
		runsRepo,
		phasesRepo,
		stepsRepo,
		attemptsRepo,
		gatesRepo,
		artifactsRepo,
		validationsRepo,
		routingSvc,
		service.NewPolicyRegistry(),
		artifactRoot,
		workspaceRoot,
	)
	gateSvc := service.NewGateService(gatesRepo, runsRepo)

	ctx := context.Background()

	// 2. Start Run
	runId := "test-run-1"
	_, err = runSvc.StartRun(ctx, runId, "test-project")
	if err != nil {
		t.Fatalf("StartRun failed: %v", err)
	}

	// 3. Dispatch Step (Simulated Codex)
	os.Setenv("CODEX_SIMULATION_MODE", "1")
	defer os.Unsetenv("CODEX_SIMULATION_MODE")
	os.Setenv("FORCE_GATE_FOR_TESTING", "1")
	defer os.Unsetenv("FORCE_GATE_FOR_TESTING")
	
	step := &domain.Step{
		ID:      "step-1",
		PhaseID: "phase-01-" + runId,
		Title:   "E2E Step",
		Adapter: "codex",
	}

	// Dispatch is blocking (polls and collects)
	t.Log("Dispatching step...")
	if err := runSvc.DispatchStep(ctx, runId, step); err != nil {
		t.Fatalf("DispatchStep failed: %v", err)
	}

	// 4. Verify Gate was created due to default mock policy
	t.Log("Verifying step gated...")
	s, err := runSvc.GetStep(ctx, "step-1")
	if err != nil {
		t.Fatalf("GetStep failed: %v", err)
	}
	if s.State != domain.StepStateNeedsApproval {
		t.Fatalf("Expected step state NeedsApproval, got %s", s.State)
	}

	gates, err := runSvc.GetGatesByRun(ctx, runId)
	if err != nil || len(gates) == 0 {
		t.Fatalf("Expected gates to be created, got error: %v, count: %d", err, len(gates))
	}
	gateID := gates[0].ID

	// 5. Approve Gate
	t.Log("Approving gate...")
	err = gateSvc.Approve(ctx, gateID)
	if err != nil {
		t.Fatalf("Approve failed: %v", err)
	}

	// Recheck run state
	run, _ := runSvc.GetRun(ctx, runId)
	if run.State != domain.RunStateRunning {
		t.Fatalf("Expected run state Running after approval, got %s", run.State)
	}

	t.Log("Orchestrator Lifecycle Simulation Passed!")
}
