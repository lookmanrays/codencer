package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-bridge/internal/domain"
)

func TestParseClaudeResultFromStdoutSuccess(t *testing.T) {
	stdoutPath := writeTempStdout(t, `{"type":"result","subtype":"success","is_error":false,"result":"done"}`)

	res, err := parseClaudeResultFromStdout(stdoutPath)
	if err != nil {
		t.Fatalf("parseClaudeResultFromStdout failed: %v", err)
	}
	if res.State != domain.StepStateCompleted {
		t.Fatalf("expected completed, got %s", res.State)
	}
	if res.Summary != "done" {
		t.Fatalf("expected success summary, got %q", res.Summary)
	}
}

func TestParseClaudeResultFromStdoutExecutionError(t *testing.T) {
	stdoutPath := writeTempStdout(t, `{"type":"result","subtype":"error_during_execution","is_error":true,"errors":["Rate limit exceeded"],"result":"adapter failed"}`)

	res, err := parseClaudeResultFromStdout(stdoutPath)
	if err != nil {
		t.Fatalf("parseClaudeResultFromStdout failed: %v", err)
	}
	if res.State != domain.StepStateFailedAdapter {
		t.Fatalf("expected failed_adapter, got %s", res.State)
	}
	if res.Summary != "Rate limit exceeded" {
		t.Fatalf("expected execution error summary, got %q", res.Summary)
	}
}

func TestParseClaudeResultFromStdoutMalformedJSON(t *testing.T) {
	stdoutPath := writeTempStdout(t, `not-json`)

	_, err := parseClaudeResultFromStdout(stdoutPath)
	if err == nil {
		t.Fatal("expected parseClaudeResultFromStdout to fail on malformed JSON")
	}
	if !strings.Contains(err.Error(), "stdout was not valid JSON") {
		t.Fatalf("expected malformed JSON error, got %v", err)
	}
}

func TestParseClaudeResultFromStdoutMissingResultFields(t *testing.T) {
	stdoutPath := writeTempStdout(t, `{"subtype":"success","is_error":false,"result":"done"}`)

	_, err := parseClaudeResultFromStdout(stdoutPath)
	if err == nil {
		t.Fatal("expected parseClaudeResultFromStdout to fail on missing type field")
	}
	if !strings.Contains(err.Error(), `expected Claude result event, got type ""`) {
		t.Fatalf("expected missing type error, got %v", err)
	}
}

func TestMapClaudeResultUnexpectedSubtype(t *testing.T) {
	res := mapClaudeResult(claudeResultEnvelope{
		Type:    "result",
		Subtype: "mystery_failure",
		Result:  "mystery happened",
	})

	if res.State != domain.StepStateFailedTerminal {
		t.Fatalf("expected failed_terminal, got %s", res.State)
	}
	if res.Summary != "mystery happened" {
		t.Fatalf("expected unexpected subtype summary to use result text, got %q", res.Summary)
	}
}

func TestNormalizeCoreMissingStateFieldFallsBackToFailedTerminal(t *testing.T) {
	tmpDir := t.TempDir()
	resultPath := filepath.Join(tmpDir, "result.json")
	stdoutPath := filepath.Join(tmpDir, "stdout.log")

	if err := os.WriteFile(resultPath, []byte(`{"summary":"missing state"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stdoutPath, []byte(`{"type":"result","subtype":"success","is_error":false,"result":"done"}`), 0644); err != nil {
		t.Fatal(err)
	}

	res, err := NormalizeCore("attempt-normalize", []*domain.Artifact{
		{Name: "result.json", Path: resultPath, Type: domain.ArtifactTypeResultJSON},
		{Name: "stdout.log", Path: stdoutPath, Type: domain.ArtifactTypeStdout},
	}, "claude", false)
	if err != nil {
		t.Fatalf("NormalizeCore failed: %v", err)
	}
	if res.State != domain.StepStateFailedTerminal {
		t.Fatalf("expected failed_terminal, got %s", res.State)
	}
	if !strings.Contains(res.Summary, "missing 'state' field") {
		t.Fatalf("expected missing state summary, got %q", res.Summary)
	}
	if res.RawOutputRef != stdoutPath {
		t.Fatalf("expected RawOutputRef to point to stdout, got %q", res.RawOutputRef)
	}
}

func writeTempStdout(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stdout.log")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write stdout fixture: %v", err)
	}
	return path
}
