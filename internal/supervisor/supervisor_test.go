package supervisor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/local"
	projectpkg "agent-bridge/internal/project"
)

func TestRenderersAndDryRunInstall(t *testing.T) {
	home, repo, bin := setupRuntimeFixture(t, "http://127.0.0.1:18085")

	report, err := Service(context.Background(), "install", Options{
		All:     true,
		DryRun:  true,
		Manager: ManagerLaunchd,
		BinDir:  bin,
	})
	if err != nil {
		t.Fatalf("dry-run install: %v", err)
	}
	if len(report.Services) != 3 {
		t.Fatalf("expected three service statuses, got %+v", report.Services)
	}
	if report.Services[0].Name != ServiceDaemon || !strings.Contains(report.Services[0].Rendered, "<key>Label</key>") {
		t.Fatalf("daemon launchd render missing: %+v", report.Services[0])
	}
	if report.Services[1].ObservedState != StateNotConfigured || report.Services[2].ObservedState != StateNotConfigured {
		t.Fatalf("relay/connector should be not_configured without config: %+v", report.Services)
	}

	rendered, err := RenderService(Options{Service: ServiceDaemon, Format: ManagerSystemd, Manager: ManagerManual, BinDir: bin})
	if err != nil {
		t.Fatalf("render systemd: %v", err)
	}
	if !strings.Contains(rendered, "ExecStart=") || !strings.Contains(rendered, "CODENCER_HOME="+home) || !strings.Contains(rendered, repo) {
		t.Fatalf("systemd render missing expected content:\n%s", rendered)
	}
}

func TestWatchdogReportsDaemonDownBlocker(t *testing.T) {
	setupRuntimeFixture(t, "http://127.0.0.1:1")

	report, err := WatchdogOnce(context.Background(), Options{Manager: ManagerManual, Now: func() time.Time {
		return time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	}})
	if err != nil {
		t.Fatalf("watchdog: %v", err)
	}
	if report.OK {
		t.Fatalf("expected watchdog not ok with daemon down: %+v", report)
	}
	if len(report.Blockers) == 0 || report.Blockers[0].Type != ProblemDaemonNotRunning || !report.Blockers[0].PlannerDecisionRequired {
		t.Fatalf("expected daemon_not_running planner blocker: %+v", report.Blockers)
	}
	data, _ := json.Marshal(report)
	if strings.Contains(string(data), "suggested_next_action") {
		t.Fatalf("watchdog must not emit suggested_next_action: %s", data)
	}
}

func TestRecoverLocksDryRunKeepsTerminalOwnerLock(t *testing.T) {
	var repo string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/runs/run-1" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(domain.Run{ID: "run-1", ProjectID: "proj", State: domain.RunStateCompleted})
	}))
	defer server.Close()
	_, repo, _ = setupRuntimeFixture(t, server.URL)
	lockDir := filepath.Join(repo, ".codencer", "workspace")
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(lockDir, ".codencer.lock")
	if err := os.WriteFile(lockPath, []byte("run-1"), 0644); err != nil {
		t.Fatal(err)
	}

	report, err := Recover(context.Background(), RecoveryOptions{Options: Options{DryRun: true, Manager: ManagerManual}, Mode: "locks"})
	if err != nil {
		t.Fatalf("recover locks: %v", err)
	}
	if !report.OK || len(report.Actions) != 1 || report.Actions[0].Type != "remove_stale_lock" || report.Actions[0].Done {
		t.Fatalf("unexpected recovery report: %+v", report)
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("dry-run should keep lock: %v", err)
	}
}

func TestRedactSecrets(t *testing.T) {
	input := `Authorization: Bearer abc123 token="secret" PRIVATE_KEY=very-secret`
	output := redact(input)
	if strings.Contains(output, "abc123") || strings.Contains(output, "very-secret") || strings.Contains(output, `token="secret"`) {
		t.Fatalf("secret was not redacted: %s", output)
	}
}

func setupRuntimeFixture(t *testing.T, daemonURL string) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	repo := filepath.Join(root, "repo")
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(bin, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"orchestratord", "codencer-relayd", "codencer-connectord"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(local.HomeEnvName, home)
	t.Chdir(repo)
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
		DefaultAdapter: "fake",
		AdapterProfile: "fake-success",
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
	return home, repo, bin
}
