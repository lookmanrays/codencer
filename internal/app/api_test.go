package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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

type blockingAPIAdapter struct {
	mu            sync.Mutex
	running       bool
	started       chan struct{}
	cancelled     chan struct{}
	startedOnce   sync.Once
	cancelledOnce sync.Once
}

func newBlockingAPIAdapter() *blockingAPIAdapter {
	return &blockingAPIAdapter{
		started:   make(chan struct{}),
		cancelled: make(chan struct{}),
	}
}

func (a *blockingAPIAdapter) Name() string           { return "blocking-api" }
func (a *blockingAPIAdapter) Capabilities() []string { return []string{"mock"} }
func (a *blockingAPIAdapter) Start(ctx context.Context, step *domain.Step, attempt *domain.Attempt, workspaceRoot, attemptArtifactRoot string) error {
	a.mu.Lock()
	a.running = true
	a.mu.Unlock()
	a.startedOnce.Do(func() { close(a.started) })
	return nil
}
func (a *blockingAPIAdapter) Poll(ctx context.Context, attemptID string) (bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.running, nil
}
func (a *blockingAPIAdapter) Cancel(ctx context.Context, attemptID string) error {
	a.mu.Lock()
	a.running = false
	a.mu.Unlock()
	a.cancelledOnce.Do(func() { close(a.cancelled) })
	return nil
}
func (a *blockingAPIAdapter) CollectArtifacts(ctx context.Context, attemptID, attemptArtifactRoot string) ([]*domain.Artifact, error) {
	return nil, nil
}
func (a *blockingAPIAdapter) NormalizeResult(ctx context.Context, attemptID string, artifacts []*domain.Artifact) (*domain.ResultSpec, error) {
	return &domain.ResultSpec{Version: "v1", State: domain.StepStateCompleted, Summary: "completed"}, nil
}

type apiRouteProofEnv struct {
	handler         *APIHandler
	mux             *http.ServeMux
	runSvc          *service.RunService
	gateSvc         *service.GateService
	runsRepo        *sqlite.RunsRepo
	phasesRepo      *sqlite.PhasesRepo
	stepsRepo       *sqlite.StepsRepo
	attemptsRepo    *sqlite.AttemptsRepo
	gatesRepo       *sqlite.GatesRepo
	artifactsRepo   *sqlite.ArtifactsRepo
	validationsRepo *sqlite.ValidationsRepo
	artifactRoot    string
	workspaceRoot   string
	repoRoot        string
}

func newAPIRouteProofEnv(t *testing.T, adapters map[string]domain.Adapter) *apiRouteProofEnv {
	t.Helper()
	if adapters == nil {
		adapters = map[string]domain.Adapter{}
	}

	repoRoot := createGitRepo(t)
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "api-route-proof.db")
	workspaceRoot := filepath.Join(tmpDir, "workspace")
	artifactRoot := filepath.Join(tmpDir, "artifacts")

	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(artifactRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	if err := sqlite.RunMigrations(db); err != nil {
		t.Fatal(err)
	}

	runsRepo := sqlite.NewRunsRepo(db)
	phasesRepo := sqlite.NewPhasesRepo(db)
	stepsRepo := sqlite.NewStepsRepo(db)
	attemptsRepo := sqlite.NewAttemptsRepo(db)
	gatesRepo := sqlite.NewGatesRepo(db)
	artifactsRepo := sqlite.NewArtifactsRepo(db)
	validationsRepo := sqlite.NewValidationsRepo(db)
	benchRepo := sqlite.NewBenchmarksRepo(db)
	routingSvc := service.NewRoutingService(benchRepo, adapters)
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
		workspace.NewNullProvisioner(),
		artifactRoot,
		workspaceRoot,
		repoRoot,
	)
	gateSvc := service.NewGateService(gatesRepo, runsRepo, stepsRepo, attemptsRepo)

	handler := &APIHandler{
		RunSvc:  runSvc,
		GateSvc: gateSvc,
		AppCtx: &AppContext{
			Config: &Config{
				Host:          "127.0.0.1",
				Port:          8085,
				DBPath:        dbPath,
				WorkspaceRoot: workspaceRoot,
			},
			RepoRoot:    repoRoot,
			Adapters:    adapters,
			StartedAt:   time.Unix(1700000000, 0).UTC(),
			InstanceID:  "api-route-proof",
			InstanceSvc: nil,
		},
	}

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &apiRouteProofEnv{
		handler:         handler,
		mux:             mux,
		runSvc:          runSvc,
		gateSvc:         gateSvc,
		runsRepo:        runsRepo,
		phasesRepo:      phasesRepo,
		stepsRepo:       stepsRepo,
		attemptsRepo:    attemptsRepo,
		gatesRepo:       gatesRepo,
		artifactsRepo:   artifactsRepo,
		validationsRepo: validationsRepo,
		artifactRoot:    artifactRoot,
		workspaceRoot:   workspaceRoot,
		repoRoot:        repoRoot,
	}
}

func waitForChannel(t *testing.T, ch <-chan struct{}, name string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", name)
	}
}

func createGitRepo(t *testing.T) string {
	t.Helper()
	repoRoot := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s failed: %v\n%s", strings.Join(args, " "), err, string(out))
		}
	}

	run("git", "init")
	run("git", "config", "user.email", "tests@example.com")
	run("git", "config", "user.name", "Codencer Tests")
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("repo"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", "README.md")
	run("git", "commit", "-m", "init")
	return repoRoot
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

func TestAPI_RouteProofs(t *testing.T) {
	t.Run("POST /api/v1/gates/{id} approves and rejects gates", func(t *testing.T) {
		env := newAPIRouteProofEnv(t, nil)
		ctx := context.Background()

		runApprove := "gate-route-run-approve"
		if _, err := env.runSvc.StartRun(ctx, runApprove, "gate-proj", "", "", ""); err != nil {
			t.Fatal(err)
		}
		approvePhaseID := "phase-execution-" + runApprove
		approveStep := &domain.Step{
			ID:        "gate-step-approve",
			PhaseID:   approvePhaseID,
			Title:     "Gate approval step",
			Goal:      "Validate gate approval",
			Adapter:   "mock",
			State:     domain.StepStateNeedsApproval,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if err := env.stepsRepo.Create(ctx, approveStep); err != nil {
			t.Fatal(err)
		}
		if err := env.attemptsRepo.Create(ctx, &domain.Attempt{
			ID:        "gate-attempt-approve",
			StepID:    approveStep.ID,
			Number:    1,
			Adapter:   "mock",
			Result:    &domain.ResultSpec{Version: "v1", State: domain.StepStateCompleted, Summary: "approved"},
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}); err != nil {
			t.Fatal(err)
		}
		approveGate := &domain.Gate{
			ID:          "gate-route-approve",
			RunID:       runApprove,
			StepID:      approveStep.ID,
			Description: "approval required",
			State:       domain.GateStatePending,
			CreatedAt:   time.Now().UTC(),
		}
		if err := env.gatesRepo.Create(ctx, approveGate); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodPost, "/api/v1/gates/"+approveGate.ID, strings.NewReader(`{"action":"approve"}`))
		w := httptest.NewRecorder()
		env.mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
		}

		updatedGate, err := env.gatesRepo.Get(ctx, approveGate.ID)
		if err != nil {
			t.Fatal(err)
		}
		if updatedGate.State != domain.GateStateApproved {
			t.Fatalf("expected approved gate state, got %s", updatedGate.State)
		}
		updatedStep, err := env.stepsRepo.Get(ctx, approveStep.ID)
		if err != nil {
			t.Fatal(err)
		}
		if updatedStep.State != domain.StepStateCompleted {
			t.Fatalf("expected step state completed after approval, got %s", updatedStep.State)
		}

		runReject := "gate-route-run-reject"
		if _, err := env.runSvc.StartRun(ctx, runReject, "gate-proj", "", "", ""); err != nil {
			t.Fatal(err)
		}
		rejectPhaseID := "phase-execution-" + runReject
		rejectStep := &domain.Step{
			ID:        "gate-step-reject",
			PhaseID:   rejectPhaseID,
			Title:     "Gate rejection step",
			Goal:      "Validate gate rejection",
			Adapter:   "mock",
			State:     domain.StepStateNeedsApproval,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if err := env.stepsRepo.Create(ctx, rejectStep); err != nil {
			t.Fatal(err)
		}
		if err := env.attemptsRepo.Create(ctx, &domain.Attempt{
			ID:        "gate-attempt-reject",
			StepID:    rejectStep.ID,
			Number:    1,
			Adapter:   "mock",
			Result:    &domain.ResultSpec{Version: "v1", State: domain.StepStateCompletedWithWarnings, Summary: "reject"},
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}); err != nil {
			t.Fatal(err)
		}
		rejectGate := &domain.Gate{
			ID:          "gate-route-reject",
			RunID:       runReject,
			StepID:      rejectStep.ID,
			Description: "rejection required",
			State:       domain.GateStatePending,
			CreatedAt:   time.Now().UTC(),
		}
		if err := env.gatesRepo.Create(ctx, rejectGate); err != nil {
			t.Fatal(err)
		}

		req = httptest.NewRequest(http.MethodPost, "/api/v1/gates/"+rejectGate.ID, strings.NewReader(`{"action":"reject"}`))
		w = httptest.NewRecorder()
		env.mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
		}

		updatedGate, err = env.gatesRepo.Get(ctx, rejectGate.ID)
		if err != nil {
			t.Fatal(err)
		}
		if updatedGate.State != domain.GateStateRejected {
			t.Fatalf("expected rejected gate state, got %s", updatedGate.State)
		}
		updatedStep, err = env.stepsRepo.Get(ctx, rejectStep.ID)
		if err != nil {
			t.Fatal(err)
		}
		if updatedStep.State != domain.StepStateCancelled {
			t.Fatalf("expected step state cancelled after rejection, got %s", updatedStep.State)
		}
	})

	t.Run("PATCH /api/v1/runs/{id} aborts an active step", func(t *testing.T) {
		adapter := newBlockingAPIAdapter()
		env := newAPIRouteProofEnv(t, map[string]domain.Adapter{"blocking": adapter})
		ctx := context.Background()
		runID := "abort-route-run"

		req := httptest.NewRequest(http.MethodPost, "/api/v1/runs", strings.NewReader(`{"id":"`+runID+`","project_id":"abort-proj"}`))
		w := httptest.NewRecorder()
		env.mux.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d body=%s", w.Code, w.Body.String())
		}

		stepPayload := strings.NewReader(`{
			"version":"1.1",
			"run_id":"` + runID + `",
			"step_id":"abort-step-1",
			"title":"Abort route step",
			"goal":"Keep the adapter running until aborted",
			"adapter_profile":"blocking"
		}`)
		req = httptest.NewRequest(http.MethodPost, "/api/v1/runs/"+runID+"/steps", stepPayload)
		w = httptest.NewRecorder()
		env.mux.ServeHTTP(w, req)
		if w.Code != http.StatusAccepted {
			t.Fatalf("expected 202, got %d body=%s", w.Code, w.Body.String())
		}

		waitForChannel(t, adapter.started, "adapter start")

		abortReq := httptest.NewRequest(http.MethodPatch, "/api/v1/runs/"+runID, strings.NewReader(`{"action":"abort"}`))
		abortW := httptest.NewRecorder()
		env.mux.ServeHTTP(abortW, abortReq)
		if abortW.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", abortW.Code, abortW.Body.String())
		}

		waitForChannel(t, adapter.cancelled, "adapter cancel")

		run, err := env.runsRepo.Get(ctx, runID)
		if err != nil {
			t.Fatal(err)
		}
		if run.State != domain.RunStateCancelled {
			t.Fatalf("expected run to be cancelled, got %s", run.State)
		}
		step, err := env.stepsRepo.Get(ctx, "abort-step-1")
		if err != nil {
			t.Fatal(err)
		}
		if step.State != domain.StepStateCancelled {
			t.Fatalf("expected step to be cancelled, got %s", step.State)
		}
	})

	t.Run("GET /api/v1/steps/{id}/result, validations, logs return persisted evidence", func(t *testing.T) {
		env := newAPIRouteProofEnv(t, nil)
		ctx := context.Background()
		runID := "evidence-route-run"
		stepID := "evidence-step-1"
		attemptID := "evidence-attempt-1"

		if _, err := env.runSvc.StartRun(ctx, runID, "evidence-proj", "", "", ""); err != nil {
			t.Fatal(err)
		}
		phaseID := "phase-execution-" + runID
		step := &domain.Step{
			ID:        stepID,
			PhaseID:   phaseID,
			Title:     "Evidence step",
			Goal:      "Persist result, validations, and logs",
			Adapter:   "mock",
			State:     domain.StepStateCompleted,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if err := env.stepsRepo.Create(ctx, step); err != nil {
			t.Fatal(err)
		}
		if err := env.attemptsRepo.Create(ctx, &domain.Attempt{
			ID:      attemptID,
			StepID:  stepID,
			Number:  1,
			Adapter: "mock",
			Result: &domain.ResultSpec{
				Version: "v1",
				State:   domain.StepStateCompleted,
				Summary: "task complete",
				RunID:   runID,
				PhaseID: phaseID,
				StepID:  stepID,
				Adapter: "mock",
			},
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}); err != nil {
			t.Fatal(err)
		}
		if err := env.validationsRepo.Create(ctx, attemptID, &domain.ValidationResult{
			Name:       "lint",
			Command:    "go test ./...",
			State:      domain.ValidationStatePassed,
			Passed:     true,
			ExitCode:   0,
			DurationMs: 123,
		}); err != nil {
			t.Fatal(err)
		}

		logPath := filepath.Join(env.artifactRoot, "logs", "stdout.log")
		if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(logPath, []byte("step logs"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := env.artifactsRepo.Create(ctx, &domain.Artifact{
			ID:        "artifact-log-1",
			AttemptID: attemptID,
			Type:      domain.ArtifactTypeStdout,
			Name:      "stdout.log",
			Path:      logPath,
			MimeType:  "text/plain; charset=utf-8",
			Size:      int64(len("step logs")),
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/steps/"+stepID+"/result", nil)
		w := httptest.NewRecorder()
		env.mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 for result route, got %d body=%s", w.Code, w.Body.String())
		}

		var result domain.ResultSpec
		if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
			t.Fatal(err)
		}
		if result.State != domain.StepStateCompleted || result.Summary != "task complete" {
			t.Fatalf("unexpected result payload: %+v", result)
		}
		if result.RunID != runID || result.StepID != stepID || result.AttemptID != attemptID {
			t.Fatalf("unexpected result envelope: %+v", result)
		}
		if len(result.Validations) != 1 || result.Validations[0].Name != "lint" {
			t.Fatalf("expected validations to be attached to result, got %+v", result.Validations)
		}

		req = httptest.NewRequest(http.MethodGet, "/api/v1/steps/"+stepID+"/validations", nil)
		w = httptest.NewRecorder()
		env.mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 for validations route, got %d body=%s", w.Code, w.Body.String())
		}

		var byAttempt map[string][]domain.ValidationResult
		if err := json.NewDecoder(w.Body).Decode(&byAttempt); err != nil {
			t.Fatal(err)
		}
		validations := byAttempt[attemptID]
		if len(validations) != 1 || validations[0].Name != "lint" {
			t.Fatalf("unexpected validations payload: %+v", byAttempt)
		}

		req = httptest.NewRequest(http.MethodGet, "/api/v1/steps/"+stepID+"/logs", nil)
		w = httptest.NewRecorder()
		env.mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 for logs route, got %d body=%s", w.Code, w.Body.String())
		}
		if got := w.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
			t.Fatalf("unexpected logs content type: %s", got)
		}
		if body := w.Body.String(); body != "step logs" {
			t.Fatalf("unexpected logs body: %q", body)
		}
	})
}
