package validation

import (
	"strings"
	"testing"

	"agent-bridge/internal/domain"
)

func TestNormalizeTaskInput_MultilineStdin(t *testing.T) {
	multilineGoal := `This is line 1.
This is line 2 with "quotes" and 'single quotes'.
	This line is indented.`

	normalized, err := NormalizeTaskInput(NormalizeTaskInputRequest{
		RunID:      "run-multiline",
		SourceKind: domain.SubmissionSourceStdin,
		SourceName: "stdin",
		Content:    []byte(multilineGoal),
		Direct: DirectTaskOptions{
			Title: "Multiline Test",
		},
	})

	if err != nil {
		t.Fatalf("NormalizeTaskInput failed: %v", err)
	}

	if normalized.Task.Goal != multilineGoal {
		t.Errorf("Goal mismatch.\nExpected:\n%s\nGot:\n%s", multilineGoal, normalized.Task.Goal)
	}

	if normalized.Provenance.OriginalInput != multilineGoal {
		t.Errorf("Provenance mismatch.\nExpected:\n%s\nGot:\n%s", multilineGoal, normalized.Provenance.OriginalInput)
	}

	if normalized.Task.Title != "Multiline Test" {
		t.Errorf("Title mismatch: expected %q, got %q", "Multiline Test", normalized.Task.Title)
	}
}

func TestNormalizeTaskInput_JSONStringStdin(t *testing.T) {
	jsonContent := `{"version":"v1","run_id":"run-json","title":"JSON Stdin","goal":"Process JSON String"}`

	normalized, err := NormalizeTaskInput(NormalizeTaskInputRequest{
		RunID:      "run-json",
		SourceKind: domain.SubmissionSourceTaskJSON,
		SourceName: "-",
		Content:    []byte(jsonContent),
	})

	if err != nil {
		t.Fatalf("NormalizeTaskInput failed: %v", err)
	}

	if normalized.Task.Goal != "Process JSON String" {
		t.Errorf("Goal mismatch: expected %q, got %q", "Process JSON String", normalized.Task.Goal)
	}

	if normalized.Provenance.OriginalFormat != "json" {
		t.Errorf("Format mismatch: expected %q, got %q", "json", normalized.Provenance.OriginalFormat)
	}
}

func TestNormalizeTaskInput_JSONStringStdin_Invalid(t *testing.T) {
	invalidJSON := `{"version":"v1", "goal": "broken`

	_, err := NormalizeTaskInput(NormalizeTaskInputRequest{
		RunID:      "run-json",
		SourceKind: domain.SubmissionSourceTaskJSON,
		SourceName: "-",
		Content:    []byte(invalidJSON),
	})

	if err == nil {
		t.Fatal("Expected error for invalid JSON, got nil")
	}
	if !strings.Contains(err.Error(), "parse task spec json") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestNormalizeTaskInput_JSONStringStdin_RunIDHandling(t *testing.T) {
	// 1. Omitted RunID in JSON -> Autofilled
	jsonOmitted := `{"version":"v1","goal":"Autofill RunID"}`
	normalized, err := NormalizeTaskInput(NormalizeTaskInputRequest{
		RunID:      "run-autofill",
		SourceKind: domain.SubmissionSourceTaskJSON,
		SourceName: "-",
		Content:    []byte(jsonOmitted),
	})
	if err != nil {
		t.Fatalf("Autofill case failed: %v", err)
	}
	if normalized.Task.RunID != "run-autofill" {
		t.Errorf("Expected run-autofill, got %q", normalized.Task.RunID)
	}

	// 2. Conflicting RunID in JSON -> Rejected
	jsonConflict := `{"version":"v1","run_id":"run-wrong","goal":"Conflict RunID"}`
	_, err = NormalizeTaskInput(NormalizeTaskInputRequest{
		RunID:      "run-correct",
		SourceKind: domain.SubmissionSourceTaskJSON,
		SourceName: "-",
		Content:    []byte(jsonConflict),
	})
	if err == nil || !strings.Contains(err.Error(), "does not match submit run ID") {
		t.Fatalf("Conflict case should have failed with mismatch error, got %v", err)
	}
}

func TestNormalizeTaskInput_BrokerAdapterSelection(t *testing.T) {
	normalized, err := NormalizeTaskInput(NormalizeTaskInputRequest{
		RunID:      "run-broker",
		SourceKind: domain.SubmissionSourceGoal,
		SourceName: "inline-goal",
		Content:    []byte("Check UI"),
		Direct: DirectTaskOptions{
			Adapter: "antigravity-broker",
		},
	})

	if err != nil {
		t.Fatalf("NormalizeTaskInput failed: %v", err)
	}

	if normalized.Task.AdapterProfile != "antigravity-broker" {
		t.Errorf("Adapter mismatch: expected %q, got %q", "antigravity-broker", normalized.Task.AdapterProfile)
	}
}
