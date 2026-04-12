package service_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-bridge/internal/adapters/antigravity"
	"agent-bridge/internal/domain"
	"agent-bridge/internal/service"
	"agent-bridge/internal/storage/sqlite"
	"agent-bridge/internal/workspace"
	_ "github.com/mattn/go-sqlite3"
)

func TestRunService_BrokerDispatch_WorkspaceWorktreePrecedence(t *testing.T) {
	// 1. Setup Mock Broker
	var receivedRepoRoot, receivedWorkspaceRoot string
	mockBroker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/result") && r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"steps":[{"items":[{"message":{"text":"Success!"}}]}]}`))
			return
		}
		if r.URL.Path == "/tasks" && r.Method == "POST" {
			var b struct {
				Prompt        string `json:"prompt"`
				RepoRoot      string `json:"repo_root"`
				WorkspaceRoot string `json:"workspace_root"`
			}
			json.NewDecoder(r.Body).Decode(&b)
			receivedRepoRoot = b.RepoRoot
			receivedWorkspaceRoot = b.WorkspaceRoot
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":"broker-task-123"}`))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/tasks/broker-task-123") && r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":"broker-task-123", "state":"completed", "summary":"Mock success"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockBroker.Close()

	// 2. Setup RunService
	dbPath := filepath.Join(t.TempDir(), "test-broker.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	sqlite.RunMigrations(db)

	runsRepo := sqlite.NewRunsRepo(db)
	phasesRepo := sqlite.NewPhasesRepo(db)
	stepsRepo := sqlite.NewStepsRepo(db)
	attemptsRepo := sqlite.NewAttemptsRepo(db)
	gatesRepo := sqlite.NewGatesRepo(db)
	artifactsRepo := sqlite.NewArtifactsRepo(db)
	validationsRepo := sqlite.NewValidationsRepo(db)
	benchmarksRepo := sqlite.NewBenchmarksRepo(db)

	repoRoot := "/home/user/project"
	brokerAdapter := antigravity.NewBrokerAdapter(mockBroker.URL, repoRoot)

	routingSvc := service.NewRoutingService(benchmarksRepo, map[string]domain.Adapter{
		"antigravity-broker": brokerAdapter,
	})
	policyRegistry := service.NewPolicyRegistry()
	policyRegistry.Register(domain.DefaultPolicy())

	artifactRoot := t.TempDir()
	workspaceRoot := t.TempDir()

	runSvc := service.NewRunService(
		runsRepo, phasesRepo, stepsRepo, attemptsRepo, gatesRepo, artifactsRepo, validationsRepo,
		routingSvc, policyRegistry, workspace.NewNullProvisioner(),
		artifactRoot, workspaceRoot,
	)

	// 3. Dispatch Step
	ctx := context.Background()
	runID := "run-broker-test"
	runSvc.StartRun(ctx, runID, "project", "", "", "")

	step := &domain.Step{
		ID:      "step-broker",
		PhaseID: "phase-execution-" + runID,
		Title:   "Broker Test",
		Goal:    "Verify worktree forwarding",
		Adapter: "antigravity-broker",
		Policy:  "default",
	}

	if err := runSvc.DispatchStep(ctx, runID, step); err != nil {
		t.Fatalf("DispatchStep failed: %v", err)
	}

	// 4. Verify Broker Received Correct Roots
	if receivedRepoRoot != repoRoot {
		t.Errorf("Expected repo_root %s, got %s", repoRoot, receivedRepoRoot)
	}

	// The workspaceRoot passed to the broker should be the isolated worktree for this run
	expectedWorktree := filepath.Join(workspaceRoot, runID)
	if !strings.HasPrefix(receivedWorkspaceRoot, expectedWorktree) {
		t.Errorf("Expected workspace_root to start with %s, got %s", expectedWorktree, receivedWorkspaceRoot)
	}

	// 5. Verify Result & Audit Trail
	res, err := runSvc.GetResultByStep(ctx, step.ID)
	if err != nil {
		t.Fatalf("GetResultByStep failed: %v", err)
	}

	if res.Artifacts["broker_task_id"] != "broker-task-123" {
		t.Errorf("Expected broker_task_id metadata, got %v", res.Artifacts["broker_task_id"])
	}

	if !strings.Contains(res.Summary, "Mock success") {
		t.Errorf("Expected summary to contain broker summary, got %q", res.Summary)
	}

	// Verify trajectory artifact exists
	artifacts, _ := runSvc.GetArtifactsByStep(ctx, step.ID)
	trajFound := false
	for _, art := range artifacts {
		if art.Name == "trajectory.json" {
			trajFound = true
			content, _ := os.ReadFile(art.Path)
			if !strings.Contains(string(content), "Success!") {
				t.Errorf("Trajectory content missing 'Success!': %s", string(content))
			}
			break
		}
	}
	if !trajFound {
		t.Error("trajectory.json artifact not found for broker run")
	}
}
