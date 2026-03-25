package service

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/storage/sqlite"
)

func TestRoutingService_BuildHeuristicChain(t *testing.T) {
	adapters := map[string]domain.Adapter{
		"qwen":   nil,
		"claude": nil,
		"codex":  nil,
	}
	rs := NewRoutingService(nil, adapters)

	// Test 1: Requested adapter exists
	chain, err := rs.BuildHeuristicChain(context.Background(), "qwen")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chain) == 0 || chain[0] != "qwen" {
		t.Errorf("expected qwen first, got %v", chain)
	}

	// Test 2: Requested adapter missing
	_, err = rs.BuildHeuristicChain(context.Background(), "missing")
	if err == nil {
		t.Error("expected error for missing adapter")
	}

	// Test 3: Default chain
	chain, _ = rs.BuildHeuristicChain(context.Background(), "")
	expectedOrder := []string{"qwen", "claude", "codex"}
	for i, id := range expectedOrder {
		if i < len(chain) && chain[i] != id {
			t.Errorf("at index %d: expected %s, got %s", i, id, chain[i])
		}
	}
}

func TestRunService_LogBenchmark(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()
	
	sqlite.RunMigrations(db)
	repo := sqlite.NewBenchmarksRepo(db)
	rs := &RoutingService{benchmarksRepo: repo}
	svc := &RunService{routingSvc: rs}

	attemptID := "test-attempt"
	res := &domain.Result{State: domain.StepStateCompleted, Summary: "OK"}
	
	// Log real benchmark
	svc.logBenchmark(context.Background(), "p1", attemptID, "codex", res, 100, false)
	
	scores, _ := repo.GetScoresByAdapter(context.Background(), "codex")
	if len(scores) != 1 {
		t.Fatalf("expected 1 score, got %d", len(scores))
	}
	if scores[0].IsSimulation {
		t.Error("expected IsSimulation=false")
	}
	if scores[0].AttemptID != attemptID {
		t.Errorf("expected AttemptID=%s, got %s", attemptID, scores[0].AttemptID)
	}

	// Log simulated benchmark
	svc.logBenchmark(context.Background(), "p1", "sim-attempt", "claude", res, 50, true)
	simScores, _ := repo.GetScoresByAdapter(context.Background(), "claude")
	if len(simScores) != 1 || !simScores[0].IsSimulation {
		t.Error("expected 1 simulated score")
	}
}
