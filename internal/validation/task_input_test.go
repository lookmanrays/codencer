package validation

import (
	"strings"
	"testing"

	"agent-bridge/internal/domain"
)

func TestParseTaskSpecBytesAcceptsYAMLAndJSON(t *testing.T) {
	yamlSpec, err := ParseTaskSpecBytes([]byte(`
version: "v1"
run_id: "run-123"
title: "Task"
goal: "Ship it"
adapter_profile: "codex"
`))
	if err != nil {
		t.Fatalf("ParseTaskSpecBytes YAML failed: %v", err)
	}
	if yamlSpec.RunID != "run-123" || yamlSpec.AdapterProfile != "codex" {
		t.Fatalf("unexpected YAML parse result: %+v", yamlSpec)
	}

	jsonSpec, err := ParseTaskSpecBytes([]byte(`{"version":"v1","run_id":"run-456","title":"Task","goal":"Ship it","adapter_profile":"qwen"}`))
	if err != nil {
		t.Fatalf("ParseTaskSpecBytes JSON failed: %v", err)
	}
	if jsonSpec.RunID != "run-456" || jsonSpec.AdapterProfile != "qwen" {
		t.Fatalf("unexpected JSON parse result: %+v", jsonSpec)
	}
}

func TestParseTaskSpecJSONBytesIsStrict(t *testing.T) {
	if _, err := ParseTaskSpecJSONBytes([]byte("goal: nope")); err == nil {
		t.Fatal("expected strict JSON parser to reject YAML input")
	}
}

func TestNormalizeTaskInputDirectGoal(t *testing.T) {
	normalized, err := NormalizeTaskInput(NormalizeTaskInputRequest{
		RunID:      "run-123",
		SourceKind: domain.SubmissionSourceGoal,
		SourceName: "inline-goal",
		Content:    []byte("Fix the failing tests"),
		Direct: DirectTaskOptions{
			Adapter:            "codex",
			TimeoutSeconds:     120,
			Policy:             "default_safe_refactor",
			AcceptanceCriteria: []string{"tests pass"},
			ValidationCommands: []string{"go test ./pkg/foo", "go test ./pkg/bar"},
		},
	})
	if err != nil {
		t.Fatalf("NormalizeTaskInput failed: %v", err)
	}

	if normalized.Task.Version != "v1" {
		t.Fatalf("expected version v1, got %q", normalized.Task.Version)
	}
	if normalized.Task.RunID != "run-123" {
		t.Fatalf("expected run_id run-123, got %q", normalized.Task.RunID)
	}
	if normalized.Task.Title != "Direct task" {
		t.Fatalf("expected default title, got %q", normalized.Task.Title)
	}
	if normalized.Task.Goal != "Fix the failing tests" {
		t.Fatalf("unexpected goal %q", normalized.Task.Goal)
	}
	if normalized.Task.AdapterProfile != "codex" || normalized.Task.PolicyBundle != "default_safe_refactor" {
		t.Fatalf("unexpected direct metadata: %+v", normalized.Task)
	}
	if len(normalized.Task.Validations) != 2 {
		t.Fatalf("expected 2 validations, got %d", len(normalized.Task.Validations))
	}
	if normalized.Task.Validations[0].Name != "validation-1" || normalized.Task.Validations[0].Command != "go test ./pkg/foo" {
		t.Fatalf("unexpected first validation: %+v", normalized.Task.Validations[0])
	}
	if normalized.Provenance.SourceKind != domain.SubmissionSourceGoal || normalized.Provenance.OriginalInput != "Fix the failing tests" {
		t.Fatalf("unexpected provenance: %+v", normalized.Provenance)
	}
	if !contains(normalized.Provenance.DefaultsApplied, "version") || !contains(normalized.Provenance.DefaultsApplied, "title") {
		t.Fatalf("expected defaults to include version and title, got %v", normalized.Provenance.DefaultsApplied)
	}
}

func TestNormalizeTaskInputPromptFileUsesBasenameTitle(t *testing.T) {
	normalized, err := NormalizeTaskInput(NormalizeTaskInputRequest{
		RunID:      "run-123",
		SourceKind: domain.SubmissionSourcePromptFile,
		SourceName: "/tmp/prompts/fix-tests.md",
		Content:    []byte("Fix the failing tests"),
	})
	if err != nil {
		t.Fatalf("NormalizeTaskInput failed: %v", err)
	}
	if normalized.Task.Title != "fix-tests" {
		t.Fatalf("expected prompt title fallback from basename, got %q", normalized.Task.Title)
	}
	if normalized.Provenance.OriginalFormat != "md" {
		t.Fatalf("expected md provenance, got %q", normalized.Provenance.OriginalFormat)
	}
}

func TestNormalizeTaskInputRejectsEmptyDirectInput(t *testing.T) {
	_, err := NormalizeTaskInput(NormalizeTaskInputRequest{
		RunID:      "run-123",
		SourceKind: domain.SubmissionSourceStdin,
		SourceName: "stdin",
		Content:    []byte("   \n\t"),
	})
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("expected empty direct input error, got %v", err)
	}
}

func TestNormalizeTaskInputCanonicalRunIDBehavior(t *testing.T) {
	normalized, err := NormalizeTaskInput(NormalizeTaskInputRequest{
		RunID:      "run-123",
		SourceKind: domain.SubmissionSourceTaskFile,
		SourceName: "task.yaml",
		Content: []byte(`
version: "v1"
title: "Task"
goal: "Ship it"
`),
	})
	if err != nil {
		t.Fatalf("NormalizeTaskInput failed: %v", err)
	}
	if normalized.Task.RunID != "run-123" {
		t.Fatalf("expected run_id to be autofilled, got %q", normalized.Task.RunID)
	}
	if !contains(normalized.Provenance.DefaultsApplied, "run_id") {
		t.Fatalf("expected run_id default to be tracked, got %v", normalized.Provenance.DefaultsApplied)
	}

	_, err = NormalizeTaskInput(NormalizeTaskInputRequest{
		RunID:      "run-123",
		SourceKind: domain.SubmissionSourceTaskJSON,
		SourceName: "task.json",
		Content:    []byte(`{"version":"v1","run_id":"run-999","title":"Task","goal":"Ship it"}`),
	})
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected run_id mismatch error, got %v", err)
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
