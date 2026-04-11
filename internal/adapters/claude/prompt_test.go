package claude

import (
	"strings"
	"testing"

	"agent-bridge/internal/domain"
)

func TestBuildPromptIncludesTitleAndGoal(t *testing.T) {
	step := &domain.Step{
		Title: "Fix tests",
		Goal:  "Update pkg/foo and rerun tests",
	}

	got := buildPrompt(step)

	expected := strings.TrimSpace(`Task Title
Fix tests

Goal
Update pkg/foo and rerun tests`)
	if got != expected {
		t.Fatalf("unexpected prompt\nexpected:\n%s\n\nactual:\n%s", expected, got)
	}
}

func TestBuildPromptUsesTaskSpecSnapshot(t *testing.T) {
	step := &domain.Step{
		Title:          "fallback title",
		Goal:           "fallback goal",
		Policy:         "fallback-policy",
		TimeoutSeconds: 11,
		Validations: []domain.ValidationCommand{
			{Name: "fallback", Command: "echo fallback"},
		},
		TaskSpecSnapshot: &domain.TaskSpec{
			Title:          "Snapshot Title",
			Goal:           "Snapshot Goal",
			Context:        domain.TaskContext{Summary: "Snapshot context"},
			Constraints:    []string{"Stay inside repo", "Do not edit generated files"},
			AllowedPaths:   []string{"internal/adapters/claude", "docs/"},
			ForbiddenPaths: []string{"vendor/"},
			Acceptance:     []string{"All Claude adapter tests pass"},
			Validations:    []domain.ValidationCommand{{Name: "unit", Command: "go test ./internal/adapters/claude"}, {Command: "go test ./internal/adapters/..."}},
			StopConditions: []string{"Stop if repository is dirty outside attempt workspace"},
			PolicyBundle:   "snapshot-policy",
			TimeoutSeconds: 45,
		},
	}

	got := buildPrompt(step)
	expected := strings.TrimSpace(`Task Title
Snapshot Title

Goal
Snapshot Goal

Context
Snapshot context

Constraints
- Stay inside repo
- Do not edit generated files

Allowed Paths
- internal/adapters/claude
- docs/

Forbidden Paths
- vendor/

Acceptance Criteria
- All Claude adapter tests pass

Validations
- unit: go test ./internal/adapters/claude
- go test ./internal/adapters/...

Stop Conditions
- Stop if repository is dirty outside attempt workspace

Policy
snapshot-policy

Timeout Seconds
45`)
	if got != expected {
		t.Fatalf("unexpected prompt\nexpected:\n%s\n\nactual:\n%s", expected, got)
	}
	if strings.Contains(got, "fallback") {
		t.Fatalf("expected snapshot fields to win over step fields, got %q", got)
	}
}

func TestBuildPromptFallsBackWithoutSnapshot(t *testing.T) {
	step := &domain.Step{
		Title:          "Fallback Title",
		Goal:           "Fallback Goal",
		Policy:         "local-policy",
		TimeoutSeconds: 22,
		Validations: []domain.ValidationCommand{
			{Name: "unit", Command: "go test ./..."},
		},
	}

	got := buildPrompt(step)
	if !strings.Contains(got, "Task Title\nFallback Title") {
		t.Fatalf("expected fallback title in prompt, got %q", got)
	}
	if !strings.Contains(got, "Goal\nFallback Goal") {
		t.Fatalf("expected fallback goal in prompt, got %q", got)
	}
	if !strings.Contains(got, "Policy\nlocal-policy") {
		t.Fatalf("expected fallback policy in prompt, got %q", got)
	}
	if !strings.Contains(got, "Timeout Seconds\n22") {
		t.Fatalf("expected fallback timeout in prompt, got %q", got)
	}
}

func TestBuildPromptIncludesValidationsAndConstraintsOnlyWhenPresent(t *testing.T) {
	step := &domain.Step{
		TaskSpecSnapshot: &domain.TaskSpec{
			Goal:        "Audit adapter",
			Constraints: []string{"Do not change public APIs", ""},
			Validations: []domain.ValidationCommand{
				{Name: "unit", Command: "go test ./internal/adapters/claude"},
				{Command: "go test ./internal/adapters/..."},
				{Name: "skip", Command: ""},
			},
		},
	}

	got := buildPrompt(step)
	if !strings.Contains(got, "Constraints\n- Do not change public APIs") {
		t.Fatalf("expected constraints section, got %q", got)
	}
	if !strings.Contains(got, "Validations\n- unit: go test ./internal/adapters/claude\n- go test ./internal/adapters/...") {
		t.Fatalf("expected validations section, got %q", got)
	}
	if strings.Contains(got, "Allowed Paths") || strings.Contains(got, "Forbidden Paths") || strings.Contains(got, "Acceptance Criteria") {
		t.Fatalf("expected empty sections to be omitted, got %q", got)
	}
}
