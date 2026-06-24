package localexec

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/local"
	projectpkg "agent-bridge/internal/project"
)

func TestSubmitDaemonNotRunningBlocker(t *testing.T) {
	base := setupProject(t, "http://127.0.0.1:1", "fake", "fake-success")
	service := NewService()
	report, err := service.Submit(context.Background(), SubmitOptions{
		BaseOptions: base,
		Goal:        "do a fake task",
		Wait:        true,
	})
	if err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}
	if report.ExitCode != ExitDaemonFailed || report.Blocker == nil || report.Blocker.Type != BlockerDaemonNotRunning {
		t.Fatalf("expected daemon_not_running exit, got %+v", report)
	}
}

func TestRunStartListGetStatusViaHTTP(t *testing.T) {
	cancelled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/runs":
			writeTestJSON(t, w, http.StatusCreated, domain.Run{ID: "run-1", ProjectID: "proj", State: domain.RunStateRunning})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/runs":
			if r.URL.Query().Get("project_id") != "proj" {
				t.Fatalf("project filter = %q", r.URL.Query().Get("project_id"))
			}
			writeTestJSON(t, w, http.StatusOK, []*domain.Run{{ID: "run-1", ProjectID: "proj", State: domain.RunStateCompleted}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/runs/run-1":
			writeTestJSON(t, w, http.StatusOK, domain.Run{ID: "run-1", ProjectID: "proj", State: domain.RunStateCompleted})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/runs/run-1/steps":
			writeTestJSON(t, w, http.StatusOK, []*domain.Step{{ID: "step-1", State: domain.StepStateCompleted}})
		case r.Method == http.MethodPatch && r.URL.Path == "/api/v1/runs/run-1":
			cancelled = true
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	base := setupProject(t, server.URL, "fake", "fake-success")
	service := NewService()
	started, err := service.StartRun(context.Background(), RunOptions{BaseOptions: base})
	if err != nil {
		t.Fatalf("StartRun error: %v", err)
	}
	if !started.OK || started.Run.ID != "run-1" {
		t.Fatalf("unexpected start report: %+v", started)
	}
	listed, err := service.ListRuns(context.Background(), RunOptions{BaseOptions: base})
	if err != nil {
		t.Fatalf("ListRuns error: %v", err)
	}
	if len(listed.Runs) != 1 || listed.Runs[0].ID != "run-1" {
		t.Fatalf("unexpected list report: %+v", listed)
	}
	got, err := service.GetRun(context.Background(), RunOptions{BaseOptions: base, RunID: "run-1"})
	if err != nil {
		t.Fatalf("GetRun error: %v", err)
	}
	if got.Status != string(domain.RunStateCompleted) || len(got.Steps) != 1 {
		t.Fatalf("unexpected get report: %+v", got)
	}
	events, err := service.Events(context.Background(), RunOptions{BaseOptions: base, RunID: "run-1"})
	if err != nil {
		t.Fatalf("Events error: %v", err)
	}
	if len(events.Events) == 0 || events.Events[0].RunID != "run-1" {
		t.Fatalf("unexpected events report: %+v", events)
	}
	cancel, err := service.CancelRun(context.Background(), RunOptions{BaseOptions: base, RunID: "run-1"})
	if err != nil {
		t.Fatalf("CancelRun error: %v", err)
	}
	if !cancelled || !cancel.OK {
		t.Fatalf("expected daemon abort call and ok report, cancelled=%t report=%+v", cancelled, cancel)
	}
	resume, err := service.ResumeRun(context.Background(), RunOptions{BaseOptions: base, RunID: "run-1"})
	if err != nil {
		t.Fatalf("ResumeRun error: %v", err)
	}
	if resume.ExitCode != ExitBlocked || resume.Blocker == nil || resume.Blocker.Type != BlockerUnsupportedOperation {
		t.Fatalf("expected unsupported resume blocker, got %+v", resume)
	}
}

func TestSubmitWaitMapsCompletedAndQuestionBlocker(t *testing.T) {
	tests := []struct {
		name      string
		state     domain.StepState
		questions []string
		exitCode  int
		blocker   string
	}{
		{name: "completed", state: domain.StepStateCompleted, exitCode: ExitSuccess},
		{name: "question", state: domain.StepStateNeedsManualAttention, questions: []string{"choose"}, exitCode: ExitBlocked, blocker: BlockerQuestion},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodPost && r.URL.Path == "/api/v1/runs":
					writeTestJSON(t, w, http.StatusCreated, domain.Run{ID: "run-1", ProjectID: "proj", State: domain.RunStateRunning})
				case r.Method == http.MethodPost && r.URL.Path == "/api/v1/runs/run-1/steps":
					var task domain.TaskSpec
					if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
						t.Fatalf("decode task: %v", err)
					}
					if task.AdapterProfile != "fake-success" {
						t.Fatalf("adapter profile = %q", task.AdapterProfile)
					}
					writeTestJSON(t, w, http.StatusAccepted, domain.Step{ID: "step-1", Title: task.Title, State: domain.StepStateRunning, Adapter: task.AdapterProfile})
				case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-1":
					writeTestJSON(t, w, http.StatusOK, domain.Step{ID: "step-1", State: tc.state, Adapter: "fake-success"})
				case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-1/result":
					writeTestJSON(t, w, http.StatusOK, domain.ResultSpec{StepID: "step-1", State: tc.state, Summary: string(tc.state), Questions: tc.questions, NeedsHumanDecision: len(tc.questions) > 0})
				case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-1/artifacts":
					writeTestJSON(t, w, http.StatusOK, []*domain.Artifact{})
				case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-1/validations":
					writeTestJSON(t, w, http.StatusOK, map[string][]*domain.ValidationResult{})
				case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-1/logs":
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("logs"))
				default:
					t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
				}
			}))
			defer server.Close()

			base := setupProject(t, server.URL, "fake", "fake-success")
			service := NewService()
			service.PollInterval = time.Millisecond
			report, err := service.Submit(context.Background(), SubmitOptions{BaseOptions: base, Goal: "do it", Wait: true})
			if err != nil {
				t.Fatalf("Submit error: %v", err)
			}
			if report.ExitCode != tc.exitCode {
				t.Fatalf("exit = %d, want %d: %+v", report.ExitCode, tc.exitCode, report)
			}
			if tc.blocker != "" && (report.Blocker == nil || report.Blocker.Type != tc.blocker) {
				t.Fatalf("blocker = %+v, want %s", report.Blocker, tc.blocker)
			}
			if tc.blocker == BlockerQuestion {
				if len(report.HumanInterrupts) != 1 {
					t.Fatalf("expected one report human interrupt, got %+v", report.HumanInterrupts)
				}
				interrupt := report.HumanInterrupts[0]
				if interrupt.Type != "clarifying_question_required" || interrupt.Status != "waiting_for_human" || interrupt.RequestedAction != "answer_question" {
					t.Fatalf("unexpected human interrupt: %+v", interrupt)
				}
				if report.Task == nil || len(report.Task.HumanInterrupts) != 1 || report.Task.Blocker == nil || report.Task.Blocker.Interrupt == nil {
					t.Fatalf("expected task and blocker interrupt metadata, got task=%+v", report.Task)
				}
			}
		})
	}
}

func TestSubmitPersistsReportForGetRunPlanReport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/runs":
			writeTestJSON(t, w, http.StatusCreated, domain.Run{ID: "run-submit", ProjectID: "proj", State: domain.RunStateRunning})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/runs/run-submit/steps":
			var task domain.TaskSpec
			if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
				t.Fatalf("decode task: %v", err)
			}
			writeTestJSON(t, w, http.StatusAccepted, domain.Step{ID: "step-submit", Title: task.Title, State: domain.StepStateRunning, Adapter: task.AdapterProfile})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-submit":
			writeTestJSON(t, w, http.StatusOK, domain.Step{ID: "step-submit", State: domain.StepStateCompleted, Adapter: "fake-success"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-submit/result":
			writeTestJSON(t, w, http.StatusOK, domain.ResultSpec{StepID: "step-submit", State: domain.StepStateCompleted, Summary: "simple task completed"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-submit/artifacts":
			writeTestJSON(t, w, http.StatusOK, []*domain.Artifact{})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-submit/validations":
			writeTestJSON(t, w, http.StatusOK, map[string][]*domain.ValidationResult{})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-submit/logs":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("logs"))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	base := setupProject(t, server.URL, "fake", "fake-success")
	service := NewService()
	service.PollInterval = time.Millisecond
	submit, err := service.Submit(context.Background(), SubmitOptions{BaseOptions: base, Goal: "do it", Wait: true})
	if err != nil {
		t.Fatalf("Submit error: %v", err)
	}
	if submit.Run == nil || submit.Run.ID != "run-submit" {
		t.Fatalf("submit missing run id: %+v", submit)
	}
	report, err := service.GetRunPlanReport(context.Background(), RunPlanReportOptions{BaseOptions: base, RunID: submit.Run.ID})
	if err != nil {
		t.Fatalf("GetRunPlanReport error: %v", err)
	}
	if !report.OK || report.Status != string(domain.StepStateCompleted) || len(report.Tasks) != 1 {
		t.Fatalf("unexpected persisted report: %+v", report)
	}
	if report.Tasks[0].StepID != "step-submit" || report.Tasks[0].Summary != "simple task completed" {
		t.Fatalf("unexpected persisted task report: %+v", report.Tasks[0])
	}
}

func TestSubmitNonWaitRunPlanReportRefreshesFromDaemon(t *testing.T) {
	stepFetched := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/runs":
			writeTestJSON(t, w, http.StatusCreated, domain.Run{ID: "run-async", ProjectID: "proj", State: domain.RunStateRunning})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/runs/run-async/steps":
			var task domain.TaskSpec
			if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
				t.Fatalf("decode task: %v", err)
			}
			writeTestJSON(t, w, http.StatusAccepted, domain.Step{ID: "step-async", Title: task.Title, State: domain.StepStateRunning, Adapter: task.AdapterProfile})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-async":
			stepFetched = true
			writeTestJSON(t, w, http.StatusOK, domain.Step{ID: "step-async", Title: "async task", State: domain.StepStateCompleted, Adapter: "fake-success"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-async/result":
			writeTestJSON(t, w, http.StatusOK, domain.ResultSpec{StepID: "step-async", State: domain.StepStateCompleted, Summary: "async task completed"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-async/artifacts":
			writeTestJSON(t, w, http.StatusOK, []*domain.Artifact{})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-async/validations":
			writeTestJSON(t, w, http.StatusOK, map[string][]*domain.ValidationResult{})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-async/logs":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("async logs"))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	base := setupProject(t, server.URL, "fake", "fake-success")
	service := NewService()
	submit, err := service.Submit(context.Background(), SubmitOptions{BaseOptions: base, Goal: "do it", Wait: false})
	if err != nil {
		t.Fatalf("Submit error: %v", err)
	}
	if submit.Status != "submitted" || submit.Run == nil || submit.Run.ID != "run-async" {
		t.Fatalf("unexpected non-wait submit report: %+v", submit)
	}
	report, err := service.GetRunPlanReport(context.Background(), RunPlanReportOptions{BaseOptions: base, RunID: submit.Run.ID})
	if err != nil {
		t.Fatalf("GetRunPlanReport error: %v", err)
	}
	if !stepFetched {
		t.Fatal("expected report read to refresh daemon step state")
	}
	if !report.OK || report.Status != string(domain.StepStateCompleted) || len(report.Tasks) != 1 {
		t.Fatalf("unexpected refreshed report: %+v", report)
	}
	if report.Tasks[0].StepID != "step-async" || report.Tasks[0].Summary != "async task completed" {
		t.Fatalf("unexpected refreshed task report: %+v", report.Tasks[0])
	}
}

func setupProject(t *testing.T, daemonURL, adapter, adapterProfile string) BaseOptions {
	t.Helper()
	home := t.TempDir()
	t.Setenv(local.HomeEnvName, home)
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	paths, err := local.ResolvePaths(repo, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := local.EnsureHome(paths, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	registry := projectpkg.EmptyRegistry()
	project, _, err := projectpkg.NewProject(projectpkg.ProjectOptions{
		ID:             "proj",
		RepoRoot:       repo,
		DefaultAdapter: adapter,
		AdapterProfile: adapterProfile,
		DaemonURL:      daemonURL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := projectpkg.UpsertProject(registry, project, false, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := projectpkg.SaveRegistry(paths.ProjectsFile, registry); err != nil {
		t.Fatal(err)
	}
	return BaseOptions{ProjectID: "proj", RepoRoot: repo}
}

func writeTestJSON(t *testing.T, w http.ResponseWriter, status int, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("write json: %v", err)
	}
}
