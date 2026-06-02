package acceptance

import (
	"os"
	"path/filepath"
	"testing"

	"agent-bridge/internal/live"
	"agent-bridge/internal/supervisor"
)

func TestVerdictReadyWithSkips(t *testing.T) {
	report := Report{
		Gates: []Gate{
			{ID: "required", Status: live.StatusPassed, Required: true},
			{ID: "codex", Status: live.StatusSkipped, Required: false, Live: true},
		},
	}
	report.computeVerdict()
	if !report.OK || report.Verdict != VerdictReadyWithSkips {
		t.Fatalf("unexpected verdict: %+v", report)
	}
	if len(report.SkippedLiveGates) != 1 || report.SkippedLiveGates[0] != "codex" {
		t.Fatalf("expected skipped live gate: %+v", report.SkippedLiveGates)
	}
}

func TestVerdictBlocksOnRequiredFailure(t *testing.T) {
	report := Report{Gates: []Gate{{ID: "local", Status: live.StatusFailed, Required: true}}}
	report.computeVerdict()
	if report.OK || report.Verdict != VerdictNotReady {
		t.Fatalf("unexpected verdict: %+v", report)
	}
	if len(report.BlockingGates) != 1 || report.BlockingGates[0] != "local" {
		t.Fatalf("expected blocking gate: %+v", report.BlockingGates)
	}
}

func TestWatchdogAndRecoveryHealthStatusesBlockOnBlockers(t *testing.T) {
	watchdog := supervisor.WatchdogReport{
		OK:       false,
		Blockers: []supervisor.RuntimeBlocker{{Type: supervisor.ProblemDaemonNotRunning, Message: "daemon down"}},
	}
	if got := watchdogHealthStatus(watchdog); got != live.StatusBlocked {
		t.Fatalf("expected blocked watchdog health, got %s", got)
	}
	recovery := supervisor.RecoveryReport{
		OK:       false,
		Blockers: []supervisor.RuntimeBlocker{{Type: supervisor.ProblemUnknownRuntimeState, Message: "unknown"}},
	}
	if got := recoverySafetyStatus(recovery); got != live.StatusBlocked {
		t.Fatalf("expected blocked recovery safety, got %s", got)
	}
}

func TestReleaseArtifactsStatusDetectsMissingBuiltArtifacts(t *testing.T) {
	repo := t.TempDir()
	dist := filepath.Join(repo, "dist")
	if err := os.MkdirAll(dist, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, "manifest.json"), []byte(`{
  "version": "v-test",
  "artifacts": [
    {"name":"missing.tar.gz","os":"darwin","arch":"arm64","status":"built","sha256":"abc"}
  ]
}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, "checksums.txt"), []byte("abc  missing.tar.gz\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := releaseArtifactsStatus(repo); got != live.StatusFailed {
		t.Fatalf("expected release artifacts gate to fail, got %s", got)
	}
}

func TestReleaseArtifactsStatusSkipsWhenNoManifest(t *testing.T) {
	if got := releaseArtifactsStatus(t.TempDir()); got != live.StatusSkipped {
		t.Fatalf("expected missing release manifest to skip artifact gate, got %s", got)
	}
}
