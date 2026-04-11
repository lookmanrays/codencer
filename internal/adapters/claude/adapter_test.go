package claude

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-bridge/internal/domain"
)

func TestAdapterSuccessLifecycleAndArtifacts(t *testing.T) {
	a := NewAdapter()
	tmpDir := t.TempDir()
	disableSimulation(t)
	workspaceRoot := filepath.Join(tmpDir, "workspace")
	artifactRoot := filepath.Join(tmpDir, "artifacts")
	if err := os.MkdirAll(workspaceRoot, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(artifactRoot, 0755); err != nil {
		t.Fatal(err)
	}

	setClaudeBinary(t, "fake_claude_success.sh")

	step := &domain.Step{ID: "step-success", Title: "Fix tests", Goal: "Update pkg/foo and rerun tests"}
	attempt := &domain.Attempt{ID: "attempt-success", Adapter: a.Name()}

	if err := a.Start(context.Background(), step, attempt, workspaceRoot, artifactRoot); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	waitForStop(t, a, attempt.ID)

	artifacts, err := a.CollectArtifacts(context.Background(), attempt.ID, artifactRoot)
	if err != nil {
		t.Fatalf("CollectArtifacts failed: %v", err)
	}

	res, err := a.NormalizeResult(context.Background(), attempt.ID, artifacts)
	if err != nil {
		t.Fatalf("NormalizeResult failed: %v", err)
	}

	if res.State != domain.StepStateCompleted {
		t.Fatalf("expected completed, got %s", res.State)
	}
	if res.RawOutputRef == "" {
		t.Fatal("expected RawOutputRef to be populated")
	}
	if _, ok := res.Artifacts["stderr.log"]; !ok {
		t.Fatal("expected stderr.log artifact link")
	}
	if _, ok := res.Artifacts["prompt.txt"]; !ok {
		t.Fatal("expected prompt.txt artifact link")
	}

	stdoutData, err := os.ReadFile(filepath.Join(artifactRoot, "stdout.log"))
	if err != nil {
		t.Fatalf("failed to read stdout.log: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(stdoutData, &payload); err != nil {
		t.Fatalf("expected valid Claude JSON in stdout, got %q: %v", string(stdoutData), err)
	}
	if payload["type"] != "result" {
		t.Fatalf("expected Claude result envelope in stdout, got %v", payload["type"])
	}
	stdoutText := string(stdoutData)
	if !strings.Contains(stdoutText, workspaceRoot) {
		t.Fatalf("expected workspace cwd in stdout, got %s", stdoutText)
	}

	promptData, err := os.ReadFile(filepath.Join(artifactRoot, "prompt.txt"))
	if err != nil {
		t.Fatalf("failed to read prompt.txt: %v", err)
	}
	promptText := string(promptData)
	if !strings.Contains(promptText, "Task Title\nFix tests") {
		t.Fatalf("expected task title in prompt, got %q", promptText)
	}
	if !strings.Contains(promptText, "Goal\nUpdate pkg/foo and rerun tests") {
		t.Fatalf("expected goal in prompt, got %q", promptText)
	}
}

func TestAdapterErrorResultMapping(t *testing.T) {
	a := NewAdapter()
	tmpDir := t.TempDir()
	disableSimulation(t)
	workspaceRoot := filepath.Join(tmpDir, "workspace")
	artifactRoot := filepath.Join(tmpDir, "artifacts")
	if err := os.MkdirAll(workspaceRoot, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(artifactRoot, 0755); err != nil {
		t.Fatal(err)
	}

	setClaudeBinary(t, "fake_claude_error.sh")

	step := &domain.Step{ID: "step-failure", Goal: "Cause an execution failure"}
	attempt := &domain.Attempt{ID: "attempt-failure", Adapter: a.Name()}

	if err := a.Start(context.Background(), step, attempt, workspaceRoot, artifactRoot); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	waitForStop(t, a, attempt.ID)

	artifacts, err := a.CollectArtifacts(context.Background(), attempt.ID, artifactRoot)
	if err != nil {
		t.Fatalf("CollectArtifacts failed: %v", err)
	}

	res, err := a.NormalizeResult(context.Background(), attempt.ID, artifacts)
	if err != nil {
		t.Fatalf("NormalizeResult failed: %v", err)
	}

	if res.State != domain.StepStateFailedAdapter {
		t.Fatalf("expected failed_adapter, got %s", res.State)
	}
	if !strings.Contains(res.Summary, "Rate limit exceeded") {
		t.Fatalf("expected error summary to include Claude error, got %s", res.Summary)
	}
}

func TestAdapterMalformedOutput(t *testing.T) {
	a := NewAdapter()
	tmpDir := t.TempDir()
	disableSimulation(t)
	workspaceRoot := filepath.Join(tmpDir, "workspace")
	artifactRoot := filepath.Join(tmpDir, "artifacts")
	if err := os.MkdirAll(workspaceRoot, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(artifactRoot, 0755); err != nil {
		t.Fatal(err)
	}

	setClaudeBinary(t, "fake_claude_malformed.sh")

	step := &domain.Step{ID: "step-malformed", Goal: "Emit malformed JSON"}
	attempt := &domain.Attempt{ID: "attempt-malformed", Adapter: a.Name()}

	if err := a.Start(context.Background(), step, attempt, workspaceRoot, artifactRoot); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	waitForStop(t, a, attempt.ID)

	artifacts, err := a.CollectArtifacts(context.Background(), attempt.ID, artifactRoot)
	if err != nil {
		t.Fatalf("CollectArtifacts failed: %v", err)
	}

	res, err := a.NormalizeResult(context.Background(), attempt.ID, artifacts)
	if err != nil {
		t.Fatalf("NormalizeResult failed: %v", err)
	}

	if res.State != domain.StepStateFailedTerminal {
		t.Fatalf("expected failed_terminal, got %s", res.State)
	}
	if !strings.Contains(res.Summary, "Malformed or missing Claude result output") {
		t.Fatalf("expected malformed output summary, got %s", res.Summary)
	}
}

func TestAdapterCancelStopsProcess(t *testing.T) {
	a := NewAdapter()
	tmpDir := t.TempDir()
	disableSimulation(t)
	workspaceRoot := filepath.Join(tmpDir, "workspace")
	artifactRoot := filepath.Join(tmpDir, "artifacts")
	if err := os.MkdirAll(workspaceRoot, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(artifactRoot, 0755); err != nil {
		t.Fatal(err)
	}

	setClaudeBinary(t, "fake_claude_sleep.sh")

	step := &domain.Step{ID: "step-cancel", Goal: "Sleep until cancelled"}
	attempt := &domain.Attempt{ID: "attempt-cancel", Adapter: a.Name()}

	if err := a.Start(context.Background(), step, attempt, workspaceRoot, artifactRoot); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	waitForRunning(t, a, attempt.ID)

	if err := a.Cancel(context.Background(), attempt.ID); err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}

	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for cancelled attempt to stop")
		default:
			running, err := a.Poll(context.Background(), attempt.ID)
			if err != nil {
				t.Fatalf("Poll failed: %v", err)
			}
			if !running {
				artifacts, err := a.CollectArtifacts(context.Background(), attempt.ID, artifactRoot)
				if err != nil {
					t.Fatalf("CollectArtifacts failed: %v", err)
				}

				res, err := a.NormalizeResult(context.Background(), attempt.ID, artifacts)
				if err != nil {
					t.Fatalf("NormalizeResult failed: %v", err)
				}
				if res.State != domain.StepStateCancelled {
					t.Fatalf("expected cancelled result, got %s", res.State)
				}
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func TestAdapterStartFailsFastWhenBinaryIsMissing(t *testing.T) {
	a := NewAdapter()
	tmpDir := t.TempDir()
	disableSimulation(t)
	t.Setenv(binaryEnvVar, filepath.Join(tmpDir, "missing-claude"))

	step := &domain.Step{ID: "step-missing", Goal: "This should not start"}
	attempt := &domain.Attempt{ID: "attempt-missing", Adapter: a.Name()}

	err := a.Start(context.Background(), step, attempt, filepath.Join(tmpDir, "workspace"), filepath.Join(tmpDir, "artifacts"))
	if err == nil {
		t.Fatal("expected Start to fail when the Claude binary is missing")
	}
	if !strings.Contains(err.Error(), "not found or not executable") {
		t.Fatalf("expected missing binary error, got %v", err)
	}
}

func waitForStop(t *testing.T, a *Adapter, attemptID string) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for %s to stop", attemptID)
		default:
			running, err := a.Poll(context.Background(), attemptID)
			if err != nil {
				t.Fatalf("Poll failed: %v", err)
			}
			if !running {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func waitForRunning(t *testing.T, a *Adapter, attemptID string) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for %s to start running", attemptID)
		default:
			running, err := a.Poll(context.Background(), attemptID)
			if err != nil {
				t.Fatalf("Poll failed: %v", err)
			}
			if running {
				return
			}
			time.Sleep(25 * time.Millisecond)
		}
	}
}

func setClaudeBinary(t *testing.T, scriptName string) {
	t.Helper()
	scriptPath := filepath.Join("testdata", scriptName)
	absScript, err := filepath.Abs(scriptPath)
	if err != nil {
		t.Fatalf("failed to resolve %s: %v", scriptPath, err)
	}
	t.Setenv(binaryEnvVar, absScript)
}

func disableSimulation(t *testing.T) {
	t.Helper()
	t.Setenv("CLAUDE_SIMULATION_MODE", "0")
	t.Setenv("ALL_ADAPTERS_SIMULATION_MODE", "0")
}
