package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/service"
	"agent-bridge/internal/storage/sqlite"
	_ "github.com/mattn/go-sqlite3"
)

func TestAPI_Endpoints(t *testing.T) {
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
	artsRepo := sqlite.NewArtifactsRepo(db)
	valsRepo := sqlite.NewValidationsRepo(db)
	benchRepo := sqlite.NewBenchmarksRepo(db)
	routingSvc := service.NewRoutingService(benchRepo, nil)
	policyReg := service.NewPolicyRegistry()

	runSvc := service.NewRunService(runsRepo, phasesRepo, stepsRepo, attemptsRepo, gatesRepo, artsRepo, valsRepo, routingSvc, policyReg, "/tmp", "/tmp")
	gateSvc := service.NewGateService(gatesRepo, runsRepo)

	handler := &APIHandler{
		RunSvc:  runSvc,
		GateSvc: gateSvc,
	}

	ctx := context.Background()
	runID := "api-test-run"
	_, _ = runSvc.StartRun(ctx, runID, "api-project")

	t.Run("GET /api/v1/runs", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/runs", nil)
		w := httptest.NewRecorder()
		handler.handleRuns(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var resp []*domain.Run
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}
		if len(resp) != 1 || resp[0].ID != runID {
			t.Errorf("unexpected runs response: %+v", resp)
		}
	})

	t.Run("GET /api/v1/benchmarks", func(t *testing.T) {
		// Insert mock benchmark
		bench := &domain.BenchmarkScore{
			ID: "bench-1",
			Adapter: "codex",
			AttemptID: "att-1",
			DurationMs: 100,
		}
		_ = benchRepo.Save(ctx, bench)

		req := httptest.NewRequest("GET", "/api/v1/benchmarks?adapter=codex", nil)
		w := httptest.NewRecorder()
		handler.handleBenchmarks(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var resp []*domain.BenchmarkScore
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}
		if len(resp) != 1 || resp[0].ID != "bench-1" {
			t.Errorf("unexpected benchmark response: %+v", resp)
		}
	})

	t.Run("GET /api/v1/routing", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/routing", nil)
		w := httptest.NewRecorder()
		handler.handleRouting(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}
		if resp["mode"] != "Heuristic Static Fallback" {
			t.Errorf("unexpected routing mode: %v", resp["mode"])
		}
	})
}
