package app

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
	"time"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/service"
	"agent-bridge/internal/storage/sqlite"
	"agent-bridge/internal/workspace"
	_ "github.com/mattn/go-sqlite3"
)

type apiTestAdapter struct{}

func (a apiTestAdapter) Name() string           { return "api-test" }
func (a apiTestAdapter) Capabilities() []string { return []string{"mock"} }
func (a apiTestAdapter) Start(ctx context.Context, step *domain.Step, attempt *domain.Attempt, workspaceRoot, attemptArtifactRoot string) error {
	return nil
}
func (a apiTestAdapter) Poll(ctx context.Context, attemptID string) (bool, error) { return false, nil }
func (a apiTestAdapter) Cancel(ctx context.Context, attemptID string) error       { return nil }
func (a apiTestAdapter) CollectArtifacts(ctx context.Context, attemptID, attemptArtifactRoot string) ([]*domain.Artifact, error) {
	return nil, nil
}
func (a apiTestAdapter) NormalizeResult(ctx context.Context, attemptID string, artifacts []*domain.Artifact) (*domain.ResultSpec, error) {
	return &domain.ResultSpec{Version: "v1", State: domain.StepStateCompleted, Summary: "ok"}, nil
}

func TestAPI_Endpoints(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "api-test.db")
	repoRoot := filepath.Join(tmpDir, "repo")
	stateDir := filepath.Join(tmpDir, ".codencer")
	workspaceRoot := filepath.Join(tmpDir, "workspace")
	artifactRoot := filepath.Join(tmpDir, "artifacts")
	if err := os.MkdirAll(repoRoot, 0755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", dbPath)
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
	settingsRepo := sqlite.NewSettingsRepo(db)
	routingSvc := service.NewRoutingService(benchRepo, nil)
	policyReg := service.NewPolicyRegistry()
	agSvc := service.NewAntigravityService(settingsRepo, "", repoRoot)

	runSvc := service.NewRunService(runsRepo, phasesRepo, stepsRepo, attemptsRepo, gatesRepo, artsRepo, valsRepo, routingSvc, policyReg, workspace.NewNullProvisioner(), artifactRoot, workspaceRoot)
	gateSvc := service.NewGateService(gatesRepo, runsRepo, stepsRepo, attemptsRepo)

	appCtx := &AppContext{
		Config:     &Config{Host: "127.0.0.1", Port: 8085, DBPath: dbPath, WorkspaceRoot: workspaceRoot},
		RepoRoot:   repoRoot,
		InstanceID: "inst-test",
		Adapters: map[string]domain.Adapter{
			"mock": apiTestAdapter{},
		},
		StartedAt: time.Unix(1700000000, 0).UTC(),
	}
	instanceSvc := service.NewInstanceService(
		settingsRepo,
		agSvc,
		Version,
		appCtx.StartedAt,
		appCtx.RepoRoot,
		stateDir,
		appCtx.Config.WorkspaceRoot,
		appCtx.Config.Host,
		appCtx.Config.Port,
		func() map[string]domain.Adapter { return appCtx.Adapters },
	)
	if err := settingsRepo.Set(context.Background(), "daemon_instance_id", "inst-test"); err != nil {
		t.Fatal(err)
	}
	if _, err := instanceSvc.EnsureStableInstanceID(context.Background()); err != nil {
		t.Fatal(err)
	}
	appCtx.InstanceSvc = instanceSvc

	handler := &APIHandler{
		RunSvc:  runSvc,
		GateSvc: gateSvc,
		AGSvc:   agSvc,
		AppCtx:  appCtx,
	}

	ctx := context.Background()
	runID := "api-test-run"
	_, _ = runSvc.StartRun(ctx, runID, "api-project", "", "", "")

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
			ID:         "bench-1",
			Adapter:    "codex",
			AttemptID:  "att-1",
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

	t.Run("POST /api/v1/runs/{id}/steps autofills phase_id and step_id", func(t *testing.T) {
		payload := strings.NewReader(`{
			"version":"1.1",
			"run_id":"api-test-run",
			"title":"Autofill IDs",
			"goal":"Verify the daemon fills missing IDs",
			"adapter_profile":"codex"
		}`)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/runs/"+runID+"/steps", payload)
		w := httptest.NewRecorder()
		handler.handleRunByID(w, req)

		if w.Code != http.StatusAccepted {
			t.Fatalf("expected 202, got %d body=%s", w.Code, w.Body.String())
		}

		var resp domain.Step
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}
		if resp.ID == "" {
			t.Fatal("expected step ID to be auto-filled")
		}
		if resp.PhaseID == "" {
			t.Fatal("expected phase ID to be auto-filled")
		}
	})

	t.Run("POST /api/v1/runs/{id}/steps rejects mismatched run_id", func(t *testing.T) {
		payload := strings.NewReader(`{
			"version":"1.1",
			"run_id":"different-run",
			"step_id":"step-mismatch",
			"title":"Mismatch",
			"goal":"Verify clean rejection",
			"adapter_profile":"codex"
		}`)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/runs/"+runID+"/steps", payload)
		w := httptest.NewRecorder()
		handler.handleRunByID(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("POST /api/v1/runs conflicts on duplicate run id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/runs", strings.NewReader(`{
			"id":"`+runID+`",
			"project_id":"api-project"
		}`))
		w := httptest.NewRecorder()
		handler.handleRuns(w, req)

		if w.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("POST /api/v1/runs/{id}/steps conflicts on duplicate step id", func(t *testing.T) {
		stepID := "step-duplicate"
		payload := `{
			"version":"1.1",
			"run_id":"` + runID + `",
			"step_id":"` + stepID + `",
			"title":"Duplicate step",
			"goal":"Create once",
			"adapter_profile":"codex"
		}`

		firstReq := httptest.NewRequest(http.MethodPost, "/api/v1/runs/"+runID+"/steps", strings.NewReader(payload))
		firstW := httptest.NewRecorder()
		handler.handleRunByID(firstW, firstReq)
		if firstW.Code != http.StatusAccepted {
			t.Fatalf("expected first request to succeed, got %d body=%s", firstW.Code, firstW.Body.String())
		}

		req := httptest.NewRequest(http.MethodPost, "/api/v1/runs/"+runID+"/steps", strings.NewReader(payload))
		w := httptest.NewRecorder()
		handler.handleRunByID(w, req)

		if w.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("GET /api/v1/instance includes stable identity", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/instance", nil)
		w := httptest.NewRecorder()
		handler.handleInstance(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var info domain.InstanceInfo
		if err := json.NewDecoder(w.Body).Decode(&info); err != nil {
			t.Fatal(err)
		}
		if info.ID != "inst-test" {
			t.Fatalf("expected stable instance ID, got %s", info.ID)
		}
		if info.RepoName != "repo" {
			t.Fatalf("expected repo name repo, got %s", info.RepoName)
		}
		if info.ManifestPath != filepath.Join(stateDir, "instance.json") {
			t.Fatalf("expected manifest path to be set, got %s", info.ManifestPath)
		}
		if len(info.Adapters) != 1 || info.Adapters[0].ID != "mock" {
			t.Fatalf("expected instance adapters to be included, got %+v", info.Adapters)
		}
	})

	t.Run("GET /api/v1/instance degrades broker lookup failures", func(t *testing.T) {
		prev := handler.AppCtx.InstanceSvc
		brokerSvc := service.NewInstanceService(
			settingsRepo,
			service.NewAntigravityService(settingsRepo, "http://127.0.0.1:1", repoRoot),
			Version,
			appCtx.StartedAt,
			repoRoot,
			stateDir,
			workspaceRoot,
			appCtx.Config.Host,
			appCtx.Config.Port,
			func() map[string]domain.Adapter { return appCtx.Adapters },
		)
		handler.AppCtx.InstanceSvc = brokerSvc
		t.Cleanup(func() {
			handler.AppCtx.InstanceSvc = prev
		})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/instance", nil)
		w := httptest.NewRecorder()
		handler.handleInstance(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
		}

		var info domain.InstanceInfo
		if err := json.NewDecoder(w.Body).Decode(&info); err != nil {
			t.Fatal(err)
		}
		if !info.Broker.Enabled || info.Broker.Mode != "broker" {
			t.Fatalf("expected degraded broker info to preserve broker mode, got %+v", info.Broker)
		}
		if info.Broker.BoundInstance != nil {
			t.Fatalf("expected degraded broker info to omit bound instance, got %+v", info.Broker.BoundInstance)
		}
	})

	t.Run("GET /api/v1/compatibility derives runtime surface", func(t *testing.T) {
		handler.AppCtx.Adapters = map[string]domain.Adapter{
			"ide-chat": apiTestAdapter{},
			"custom":   apiTestAdapter{},
		}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/compatibility", nil)
		w := httptest.NewRecorder()
		handler.handleCompatibility(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var info domain.CompatibilityInfo
		if err := json.NewDecoder(w.Body).Decode(&info); err != nil {
			t.Fatal(err)
		}
		if len(info.Adapters) != 2 || info.Adapters[0].ID != "custom" || info.Adapters[1].ID != "ide-chat" {
			t.Fatalf("unexpected compatibility payload: %+v", info)
		}
		if !info.Adapters[0].Available || info.Adapters[0].Status != "registered" {
			t.Fatalf("expected unknown registered adapter to surface as registered, got %+v", info.Adapters[0])
		}
	})

	t.Run("GET /api/v1/artifacts/{id}/content returns binary-safe bytes", func(t *testing.T) {
		contentPath := filepath.Join(artifactRoot, "artifact.bin")
		payload := []byte{0x00, 0x01, 0x02, 0x03}
		if err := os.MkdirAll(filepath.Dir(contentPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(contentPath, payload, 0644); err != nil {
			t.Fatal(err)
		}

		attempt := &domain.Attempt{
			ID:        "attempt-1",
			StepID:    "step-artifact",
			Number:    1,
			Adapter:   "mock",
			State:     domain.StepStateCompleted,
			Result:    &domain.ResultSpec{Version: "v1", State: domain.StepStateCompleted, Summary: "done"},
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if err := attemptsRepo.Create(ctx, attempt); err != nil {
			t.Fatal(err)
		}
		artifact := &domain.Artifact{
			ID:        "artifact-1",
			AttemptID: attempt.ID,
			Type:      domain.ArtifactTypeStdout,
			Name:      "artifact.bin",
			Path:      contentPath,
			Size:      int64(len(payload)),
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if err := artsRepo.Create(ctx, artifact); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/artifacts/"+artifact.ID+"/content", nil)
		w := httptest.NewRecorder()
		handler.handleArtifactByID(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if got := w.Header().Get("Content-Type"); got != "application/octet-stream" {
			t.Fatalf("expected octet-stream content type, got %s", got)
		}
		if got := w.Header().Get("Content-Length"); got != "4" {
			t.Fatalf("expected content length 4, got %s", got)
		}
		if body := w.Body.Bytes(); string(body) != string(payload) {
			t.Fatalf("unexpected artifact body %v", body)
		}
	})

	t.Run("GET /api/v1/artifacts/{id}/content rejects paths outside artifact root", func(t *testing.T) {
		outsidePath := filepath.Join(t.TempDir(), "outside.bin")
		if err := os.WriteFile(outsidePath, []byte("nope"), 0644); err != nil {
			t.Fatal(err)
		}

		attempt := &domain.Attempt{
			ID:        "attempt-escape-1",
			StepID:    "step-escape",
			Number:    1,
			Adapter:   "mock",
			State:     domain.StepStateCompleted,
			Result:    &domain.ResultSpec{Version: "v1", State: domain.StepStateCompleted, Summary: "done"},
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if err := attemptsRepo.Create(ctx, attempt); err != nil {
			t.Fatal(err)
		}
		artifact := &domain.Artifact{
			ID:        "artifact-escape-1",
			AttemptID: attempt.ID,
			Type:      domain.ArtifactTypeStdout,
			Name:      "outside.bin",
			Path:      outsidePath,
			Size:      4,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if err := artsRepo.Create(ctx, artifact); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/artifacts/"+artifact.ID+"/content", nil)
		w := httptest.NewRecorder()
		handler.handleArtifactByID(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("GET /api/v1/artifacts/{id} returns artifact metadata", func(t *testing.T) {
		attempt := &domain.Attempt{
			ID:        "attempt-meta-1",
			StepID:    "step-meta",
			Number:    1,
			Adapter:   "mock",
			State:     domain.StepStateCompleted,
			Result:    &domain.ResultSpec{Version: "v1", State: domain.StepStateCompleted, Summary: "done"},
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if err := attemptsRepo.Create(ctx, attempt); err != nil {
			t.Fatal(err)
		}
		artifact := &domain.Artifact{
			ID:        "artifact-meta-1",
			AttemptID: attempt.ID,
			Type:      domain.ArtifactTypeStdout,
			Name:      "stdout.log",
			Path:      "/tmp/stdout.log",
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if err := artsRepo.Create(ctx, artifact); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/artifacts/"+artifact.ID, nil)
		w := httptest.NewRecorder()
		handler.handleArtifactByID(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var found domain.Artifact
		if err := json.NewDecoder(w.Body).Decode(&found); err != nil {
			t.Fatal(err)
		}
		if found.ID != artifact.ID {
			t.Fatalf("expected artifact %s, got %s", artifact.ID, found.ID)
		}
	})

	t.Run("GET /api/v1/gates/{id} returns gate metadata", func(t *testing.T) {
		gate := &domain.Gate{
			ID:          "gate-api-1",
			RunID:       runID,
			StepID:      "step-api-gate",
			Description: "approval required",
			State:       domain.GateStatePending,
			CreatedAt:   time.Now().UTC(),
		}
		if err := gatesRepo.Create(ctx, gate); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/gates/"+gate.ID, nil)
		w := httptest.NewRecorder()
		handler.handleGateByID(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var found domain.Gate
		if err := json.NewDecoder(w.Body).Decode(&found); err != nil {
			t.Fatal(err)
		}
		if found.ID != gate.ID {
			t.Fatalf("expected gate %s, got %s", gate.ID, found.ID)
		}
	})
}
