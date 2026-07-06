package local

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestResolvePathsUsesCodencerHomeOverride(t *testing.T) {
	home := t.TempDir()
	t.Setenv(HomeEnvName, home)
	paths, err := ResolvePaths("", "")
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}
	if paths.Home != filepath.Clean(home) {
		t.Fatalf("home = %q want %q", paths.Home, filepath.Clean(home))
	}
	if paths.ProjectsFile != filepath.Join(home, "projects.json") {
		t.Fatalf("projects path = %q", paths.ProjectsFile)
	}

	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.Mkdir(repo, 0755); err != nil {
		t.Fatal(err)
	}
	paths, err = ResolvePaths(repo, filepath.Join(home, "custom-config.json"))
	if err != nil {
		t.Fatalf("resolve paths with overrides: %v", err)
	}
	if paths.RepoRuntimeDir != filepath.Join(repo, ".codencer") {
		t.Fatalf("repo runtime dir = %q", paths.RepoRuntimeDir)
	}
	if paths.ConfigFile != filepath.Join(home, "custom-config.json") {
		t.Fatalf("config path = %q", paths.ConfigFile)
	}
}

func TestEnsureHomeCreatesConfigRegistryAndDirs(t *testing.T) {
	home := filepath.Join(t.TempDir(), "codencer-home")
	t.Setenv(HomeEnvName, home)
	paths, err := ResolvePaths("", "")
	if err != nil {
		t.Fatal(err)
	}
	result, err := EnsureHome(paths, time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("ensure home: %v", err)
	}
	if !result.ConfigCreated || !result.RegistryCreated || !result.MachineCreated {
		t.Fatalf("expected config and registry creation, got %+v", result)
	}
	for _, path := range []string{paths.Home, paths.LogsDir, paths.RuntimeDir, paths.TokensDir, paths.ArtifactsDir, paths.ConfigFile, paths.ProjectsFile, paths.MachineFile} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}

	result, err = EnsureHome(paths, time.Now().UTC())
	if err != nil {
		t.Fatalf("ensure home second pass: %v", err)
	}
	if result.ConfigCreated || result.RegistryCreated || result.MachineCreated {
		t.Fatalf("second pass should be idempotent, got %+v", result)
	}
}

func TestMachineIdentityIsStableAndHostLabelEditable(t *testing.T) {
	home := filepath.Join(t.TempDir(), "codencer-home")
	paths, err := ResolvePathsForHome("", "", home)
	if err != nil {
		t.Fatal(err)
	}
	first, created, err := EnsureMachine(paths.MachineFile, time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !created || first.MachineID == "" || first.HostLabel == "" {
		t.Fatalf("unexpected first machine identity: created=%t machine=%+v", created, first)
	}
	second, created, err := EnsureMachine(paths.MachineFile, time.Date(2026, 6, 1, 13, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if created || second.MachineID != first.MachineID {
		t.Fatalf("machine id should be stable: first=%+v second=%+v created=%t", first, second, created)
	}
	updated, err := SetMachineHostLabel(paths.MachineFile, "MacBook Test", time.Date(2026, 6, 1, 14, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if updated.MachineID != first.MachineID || updated.HostLabel != "macbook-test" {
		t.Fatalf("unexpected host label update: %+v", updated)
	}
}

func TestDoctorAggregationAndStrictMode(t *testing.T) {
	home := t.TempDir()
	t.Setenv(HomeEnvName, home)
	paths, err := ResolvePaths("", "")
	if err != nil {
		t.Fatal(err)
	}
	report := BuildDoctorReport(DoctorOptions{
		Paths:         paths,
		Config:        DefaultConfig(time.Now().UTC()),
		ToolchainOnly: true,
		Strict:        true,
		Probe: func(command string, args ...string) ProbeResult {
			if command == "cc" {
				return ProbeResult{Err: os.ErrNotExist}
			}
			return ProbeResult{Path: "/bin/" + command, Output: command + " version"}
		},
	})
	if report.OK {
		t.Fatalf("strict report with missing required cc should not be ok: %+v", report)
	}
	if report.Summary.Errors != 1 {
		t.Fatalf("expected one error, got %+v", report.Summary)
	}
}

func TestDaemonStatusUsesHealthAndInstanceFallback(t *testing.T) {
	healthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Fatalf("unexpected health request path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer healthServer.Close()
	status := CheckDaemon(healthServer.URL, healthServer.Client())
	if status.Status != RuntimeOK {
		t.Fatalf("expected ok status, got %+v", status)
	}

	instanceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"id":"inst-test"}`))
	}))
	defer instanceServer.Close()
	status = CheckDaemon(instanceServer.URL, instanceServer.Client())
	if status.Status != RuntimeOK || status.Detail != "instance inst-test" {
		t.Fatalf("expected instance fallback, got %+v", status)
	}
}

func TestStatusReportHandlesUnregisteredProjectsWithoutRealHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv(HomeEnvName, home)
	paths, err := ResolvePaths("", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureHome(paths, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	report := BuildStatusReport(StatusOptions{
		Paths:  paths,
		Config: DefaultConfig(time.Now().UTC()),
		Probe: func(command string, args ...string) ProbeResult {
			return ProbeResult{Err: os.ErrNotExist}
		},
	})
	if report.Status != RuntimeNotConfigured {
		t.Fatalf("expected not configured status, got %+v", report)
	}
	if report.ProjectCount != 0 {
		t.Fatalf("project count = %d", report.ProjectCount)
	}
}
