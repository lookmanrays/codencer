package validation

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-bridge/internal/domain"
)

func TestRunnerCapturesPassingValidationOutput(t *testing.T) {
	workspace := t.TempDir()
	artifacts := t.TempDir()
	runner := NewRunner()
	result, err := runner.RunWithArtifacts(context.Background(), domain.ValidationCommand{
		Name:    "unit-tests",
		Command: "echo hello && echo err >&2",
	}, workspace, artifacts)
	if err != nil {
		t.Fatalf("run validation: %v", err)
	}
	if !result.Passed || result.State != domain.ValidationStatePassed || result.ExitCode != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	assertFileContains(t, result.StdoutRef, "hello")
	assertFileContains(t, result.StderrRef, "err")
}

func TestRunnerReportsFailedValidation(t *testing.T) {
	workspace := t.TempDir()
	artifacts := t.TempDir()
	result, err := NewRunner().RunWithArtifacts(context.Background(), domain.ValidationCommand{
		Name:    "fail",
		Command: "echo nope; exit 7",
	}, workspace, artifacts)
	if err != nil {
		t.Fatalf("run validation: %v", err)
	}
	if result.Passed || result.State != domain.ValidationStateFailed || result.ExitCode != 7 {
		t.Fatalf("unexpected failed result: %+v", result)
	}
	assertFileContains(t, result.StdoutRef, "nope")
}

func TestRunnerReportsTimeout(t *testing.T) {
	workspace := t.TempDir()
	result, err := NewRunner().RunWithArtifacts(context.Background(), domain.ValidationCommand{
		Name:           "timeout",
		Command:        "sleep 2",
		TimeoutSeconds: 1,
	}, workspace, "")
	if err != nil {
		t.Fatalf("run validation: %v", err)
	}
	if result.Passed || result.State != domain.ValidationStateTimeout {
		t.Fatalf("expected timeout result, got %+v", result)
	}
}

func assertFileContains(t *testing.T, path, want string) {
	t.Helper()
	if path == "" {
		t.Fatal("expected artifact ref")
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("read artifact %s: %v", path, err)
	}
	if string(data) == "" || !strings.Contains(string(data), want) {
		t.Fatalf("artifact %s = %q, want contains %q", path, string(data), want)
	}
}
