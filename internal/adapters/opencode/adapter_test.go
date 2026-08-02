package opencode

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-bridge/internal/domain"
)

func TestCommandArgsUsesNonInteractiveStructuredAutoApprovedRun(t *testing.T) {
	args := commandArgs(&domain.Step{Title: "Fix tests"}, "do the work")
	want := []string{"run", "--format", "json", "--dangerously-skip-permissions", "--title", "Fix tests", "do the work"}
	if len(args) != len(want) {
		t.Fatalf("unexpected args: %#v", args)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("args[%d] = %q, want %q", i, args[i], want[i])
		}
	}
}

func TestResultFromOutputUsesLastCompletedTextEvent(t *testing.T) {
	dir := t.TempDir()
	stdout := filepath.Join(dir, stdoutLogFile)
	stderr := filepath.Join(dir, stderrLogFile)
	writeTestFile(t, stdout, "not json\n{\"type\":\"text\",\"part\":{\"type\":\"text\",\"text\":\"first\",\"time\":{\"end\":1}}}\n{\"type\":\"text\",\"part\":{\"type\":\"text\",\"text\":\"final outcome\",\"time\":{\"end\":2}}}\n")
	writeTestFile(t, stderr, "")

	result := resultFromOutput(&domain.Attempt{ID: "attempt-1", Adapter: AdapterID}, stdout, stderr, nil, nil)
	if result.State != domain.StepStateCompleted || result.Summary != "final outcome" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestResultFromOutputCompletesWithoutText(t *testing.T) {
	dir := t.TempDir()
	stdout := filepath.Join(dir, stdoutLogFile)
	stderr := filepath.Join(dir, stderrLogFile)
	writeTestFile(t, stdout, "{\"type\":\"step_finish\",\"part\":{}}\n")
	writeTestFile(t, stderr, "")

	result := resultFromOutput(&domain.Attempt{ID: "attempt-2", Adapter: AdapterID}, stdout, stderr, nil, nil)
	if result.State != domain.StepStateCompleted || result.Summary != "OpenCode task completed successfully." {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestResultFromOutputMapsEventAndProcessErrors(t *testing.T) {
	dir := t.TempDir()
	stdout := filepath.Join(dir, stdoutLogFile)
	stderr := filepath.Join(dir, stderrLogFile)
	writeTestFile(t, stdout, "{\"type\":\"error\",\"error\":{\"data\":{\"message\":\"rate limited\"}}}\n")
	writeTestFile(t, stderr, "process failed")

	result := resultFromOutput(&domain.Attempt{ID: "attempt-3", Adapter: AdapterID}, stdout, stderr, errTestProcess, nil)
	if result.State != domain.StepStateFailedAdapter || result.Summary != "rate limited" {
		t.Fatalf("unexpected event-error result: %+v", result)
	}

	writeTestFile(t, stdout, "malformed output\n")
	result = resultFromOutput(&domain.Attempt{ID: "attempt-4", Adapter: AdapterID}, stdout, stderr, errTestProcess, nil)
	if result.State != domain.StepStateFailedAdapter || result.Summary != "process failed" {
		t.Fatalf("unexpected process-error result: %+v", result)
	}
}

func TestAdapterSimulationLifecycle(t *testing.T) {
	t.Setenv("OPENCODE_SIMULATION_MODE", "1")
	t.Setenv("ALL_ADAPTERS_SIMULATION_MODE", "0")
	adapter := NewAdapter()
	workspace := filepath.Join(t.TempDir(), "workspace")
	artifactsRoot := filepath.Join(t.TempDir(), "artifacts")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	attempt := &domain.Attempt{ID: "simulation", Adapter: AdapterID}
	if err := adapter.Start(context.Background(), &domain.Step{Goal: "simulate"}, attempt, workspace, artifactsRoot); err != nil {
		t.Fatalf("start simulation: %v", err)
	}
	waitForStop(t, adapter, attempt.ID)
	artifacts, err := adapter.CollectArtifacts(context.Background(), attempt.ID, artifactsRoot)
	if err != nil {
		t.Fatalf("collect artifacts: %v", err)
	}
	result, err := adapter.NormalizeResult(context.Background(), attempt.ID, artifacts)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if result.State != domain.StepStateCompleted || !result.IsSimulation {
		t.Fatalf("unexpected simulation result: %+v", result)
	}
}

func TestAdapterRealLifecycleWritesNormalizedArtifacts(t *testing.T) {
	t.Setenv("OPENCODE_SIMULATION_MODE", "0")
	t.Setenv("ALL_ADAPTERS_SIMULATION_MODE", "0")
	binary := writeExecutable(t, "#!/bin/sh\nprintf '%s\\n' '{\"type\":\"text\",\"part\":{\"type\":\"text\",\"text\":\"workspace updated\",\"time\":{\"end\":1}}}'\n")
	t.Setenv(binaryEnvVar, binary)
	adapter := NewAdapter()
	workspace := filepath.Join(t.TempDir(), "workspace")
	artifactsRoot := filepath.Join(t.TempDir(), "artifacts")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	attempt := &domain.Attempt{ID: "real", Adapter: AdapterID}
	if err := adapter.Start(context.Background(), &domain.Step{Title: "Update workspace", Goal: "make a change"}, attempt, workspace, artifactsRoot); err != nil {
		t.Fatalf("start real adapter: %v", err)
	}
	waitForStop(t, adapter, attempt.ID)
	artifacts, err := adapter.CollectArtifacts(context.Background(), attempt.ID, artifactsRoot)
	if err != nil {
		t.Fatal(err)
	}
	result, err := adapter.NormalizeResult(context.Background(), attempt.ID, artifacts)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != domain.StepStateCompleted || result.Summary != "workspace updated" {
		t.Fatalf("unexpected result: %+v", result)
	}
	for _, name := range []string{stdoutLogFile, stderrLogFile, resultFile, "prompt.txt"} {
		if _, err := os.Stat(filepath.Join(artifactsRoot, name)); err != nil {
			t.Fatalf("expected %s: %v", name, err)
		}
	}
}

func TestAdapterCancelProducesCancelledResult(t *testing.T) {
	t.Setenv("OPENCODE_SIMULATION_MODE", "0")
	t.Setenv("ALL_ADAPTERS_SIMULATION_MODE", "0")
	t.Setenv(binaryEnvVar, writeExecutable(t, "#!/bin/sh\nsleep 30\n"))
	adapter := NewAdapter()
	workspace := filepath.Join(t.TempDir(), "workspace")
	artifactsRoot := filepath.Join(t.TempDir(), "artifacts")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	attempt := &domain.Attempt{ID: "cancel", Adapter: AdapterID}
	if err := adapter.Start(context.Background(), &domain.Step{Goal: "wait"}, attempt, workspace, artifactsRoot); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Cancel(context.Background(), attempt.ID); err != nil {
		t.Fatal(err)
	}
	waitForStop(t, adapter, attempt.ID)
	artifacts, err := adapter.CollectArtifacts(context.Background(), attempt.ID, artifactsRoot)
	if err != nil {
		t.Fatal(err)
	}
	result, err := adapter.NormalizeResult(context.Background(), attempt.ID, artifacts)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != domain.StepStateCancelled {
		t.Fatalf("expected cancelled, got %+v", result)
	}
}

func TestStartFailsFastWhenBinaryIsMissing(t *testing.T) {
	t.Setenv("OPENCODE_SIMULATION_MODE", "0")
	t.Setenv("ALL_ADAPTERS_SIMULATION_MODE", "0")
	t.Setenv(binaryEnvVar, filepath.Join(t.TempDir(), "missing-opencode"))
	err := NewAdapter().Start(context.Background(), &domain.Step{}, &domain.Attempt{ID: "missing", Adapter: AdapterID}, t.TempDir(), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "not found or not executable") {
		t.Fatalf("expected missing-binary error, got %v", err)
	}
}

var errTestProcess = testError("process exited")

type testError string

func (e testError) Error() string { return string(e) }

func writeTestFile(t *testing.T, path, value string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeExecutable(t *testing.T, script string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "opencode")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func waitForStop(t *testing.T, adapter *Adapter, attemptID string) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for %s", attemptID)
		default:
			running, err := adapter.Poll(context.Background(), attemptID)
			if err != nil {
				t.Fatal(err)
			}
			if !running {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
}
