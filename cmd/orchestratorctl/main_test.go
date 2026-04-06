package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"agent-bridge/internal/domain"
)

var testBinaryPath string

func TestMain(m *testing.M) {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	tmpDir, err := os.MkdirTemp("", "orchestratorctl-test-bin")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	testBinaryPath = filepath.Join(tmpDir, "orchestratorctl")
	buildCmd := exec.Command("go", "build", "-o", testBinaryPath, ".")
	buildCmd.Dir = cwd
	buildCmd.Env = os.Environ()
	buildOutput, err := buildCmd.CombinedOutput()
	if err != nil {
		panic(string(buildOutput))
	}

	os.Exit(m.Run())
}

func TestSubmitWaitJSONEmitsSingleTerminalPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/runs/run-123/steps":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"id":"step-123","phase_id":"phase-execution-run-123","title":"T","goal":"G","adapter":"codex","created_at":"2026-04-06T00:00:00Z","updated_at":"2026-04-06T00:00:00Z"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-123/result":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"v1","run_id":"run-123","phase_id":"phase-execution-run-123","step_id":"step-123","state":"completed","summary":"done"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	taskFile := writeTempTaskFile(t, `
version: "1.1"
run_id: "run-123"
title: "Task"
goal: "Do the thing"
adapter_profile: "codex"
`)

	result := runBinary(t, server.URL, "submit", "run-123", taskFile, "--wait", "--json")
	if result.exitCode != exitCodeSuccess {
		t.Fatalf("exit code = %d, stderr = %s", result.exitCode, result.stderr)
	}
	assertSingleJSONDocument(t, result.stdout)
	if strings.Contains(result.stdout, "\"id\"") {
		t.Fatalf("stdout should contain only the terminal payload, got %s", result.stdout)
	}
	if !strings.Contains(result.stdout, "\"step_id\": \"step-123\"") {
		t.Fatalf("expected terminal result payload, got %s", result.stdout)
	}
}

func TestSubmitGoalJSONNormalizesDirectInput(t *testing.T) {
	var received domain.TaskSpec
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/runs/run-123/steps" {
			http.NotFound(w, r)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"id":"step-123","phase_id":"phase-execution-run-123","title":"Fix tests","goal":"Fix the failing tests in package X","adapter":"codex","created_at":"2026-04-06T00:00:00Z","updated_at":"2026-04-06T00:00:00Z"}`))
	}))
	defer server.Close()

	result := runBinary(t, server.URL,
		"submit", "run-123",
		"--goal", "Fix the failing tests in package X",
		"--title", "Fix tests",
		"--adapter", "codex",
		"--timeout", "90",
		"--policy", "default_safe_refactor",
		"--acceptance", "tests pass",
		"--validation", "go test ./pkg/x",
		"--json",
	)
	if result.exitCode != exitCodeSuccess {
		t.Fatalf("exit code = %d stderr=%s", result.exitCode, result.stderr)
	}
	assertSingleJSONDocument(t, result.stdout)

	if received.Version != "v1" || received.RunID != "run-123" {
		t.Fatalf("unexpected normalized task: %+v", received)
	}
	if received.Title != "Fix tests" || received.Goal != "Fix the failing tests in package X" {
		t.Fatalf("unexpected normalized direct task: %+v", received)
	}
	if received.AdapterProfile != "codex" || received.TimeoutSeconds != 90 || received.PolicyBundle != "default_safe_refactor" {
		t.Fatalf("unexpected direct metadata: %+v", received)
	}
	if len(received.Validations) != 1 || received.Validations[0].Name != "validation-1" || received.Validations[0].Command != "go test ./pkg/x" {
		t.Fatalf("unexpected validations: %+v", received.Validations)
	}
	if received.SubmissionProvenance == nil || received.SubmissionProvenance.SourceKind != domain.SubmissionSourceGoal {
		t.Fatalf("expected submission provenance, got %+v", received.SubmissionProvenance)
	}
}

func TestSubmitPromptFileJSONUsesPromptBasename(t *testing.T) {
	var received domain.TaskSpec
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/runs/run-123/steps" {
			http.NotFound(w, r)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"id":"step-123","phase_id":"phase-execution-run-123","title":"fix-tests","goal":"Fix the failing tests","created_at":"2026-04-06T00:00:00Z","updated_at":"2026-04-06T00:00:00Z"}`))
	}))
	defer server.Close()

	promptFile := filepath.Join(t.TempDir(), "fix-tests.md")
	if err := os.WriteFile(promptFile, []byte("Fix the failing tests"), 0644); err != nil {
		t.Fatalf("write prompt file: %v", err)
	}

	result := runBinary(t, server.URL, "submit", "run-123", "--prompt-file", promptFile, "--json")
	if result.exitCode != exitCodeSuccess {
		t.Fatalf("exit code = %d stderr=%s", result.exitCode, result.stderr)
	}
	if received.Title != "fix-tests" {
		t.Fatalf("expected basename-derived title, got %q", received.Title)
	}
	if received.SubmissionProvenance == nil || received.SubmissionProvenance.OriginalFormat != "md" {
		t.Fatalf("expected md provenance, got %+v", received.SubmissionProvenance)
	}
}

func TestSubmitTaskJSONFromFileAndStdin(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		stdin   string
		wantRun string
	}{
		{
			name:    "file",
			args:    []string{"submit", "run-123"},
			wantRun: "run-123",
		},
		{
			name:    "stdin",
			args:    []string{"submit", "run-123", "--task-json", "-", "--json"},
			stdin:   `{"version":"v1","title":"Task","goal":"Ship it","adapter_profile":"codex"}`,
			wantRun: "run-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var received domain.TaskSpec
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/api/v1/runs/run-123/steps" {
					http.NotFound(w, r)
					return
				}
				if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
					t.Fatalf("decode request: %v", err)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusAccepted)
				_, _ = w.Write([]byte(`{"id":"step-123","phase_id":"phase-execution-run-123","title":"Task","goal":"Ship it","created_at":"2026-04-06T00:00:00Z","updated_at":"2026-04-06T00:00:00Z"}`))
			}))
			defer server.Close()

			args := append([]string{}, tt.args...)
			if tt.name == "file" {
				taskFile := filepath.Join(t.TempDir(), "task.json")
				if err := os.WriteFile(taskFile, []byte(`{"version":"v1","title":"Task","goal":"Ship it","adapter_profile":"codex"}`), 0644); err != nil {
					t.Fatalf("write task json: %v", err)
				}
				args = append(args, "--task-json", taskFile, "--json")
			}

			result := runBinaryWithStdin(t, server.URL, tt.stdin, args...)
			if result.exitCode != exitCodeSuccess {
				t.Fatalf("exit code = %d stderr=%s", result.exitCode, result.stderr)
			}
			if received.RunID != tt.wantRun {
				t.Fatalf("expected run_id %q, got %+v", tt.wantRun, received)
			}
			if received.SubmissionProvenance == nil || received.SubmissionProvenance.OriginalFormat != "json" {
				t.Fatalf("expected json provenance, got %+v", received.SubmissionProvenance)
			}
		})
	}
}

func TestSubmitStdinJSONNormalizesInput(t *testing.T) {
	var received domain.TaskSpec
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/runs/run-123/steps" {
			http.NotFound(w, r)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"id":"step-123","phase_id":"phase-execution-run-123","title":"Direct task","goal":"Fix from stdin","created_at":"2026-04-06T00:00:00Z","updated_at":"2026-04-06T00:00:00Z"}`))
	}))
	defer server.Close()

	result := runBinaryWithStdin(t, server.URL, "Fix from stdin", "submit", "run-123", "--stdin", "--json")
	if result.exitCode != exitCodeSuccess {
		t.Fatalf("exit code = %d stderr=%s", result.exitCode, result.stderr)
	}
	if received.Goal != "Fix from stdin" || received.Title != "Direct task" {
		t.Fatalf("unexpected stdin normalization: %+v", received)
	}
}

func TestSubmitRejectsInvalidSourceCombinations(t *testing.T) {
	taskFile := writeTempTaskFile(t, `
version: "v1"
title: "Task"
goal: "Do the thing"
`)

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing source",
			args: []string{"submit", "run-123", "--json"},
		},
		{
			name: "multiple sources",
			args: []string{"submit", "run-123", taskFile, "--goal", "Do the thing", "--json"},
		},
		{
			name: "metadata with canonical source",
			args: []string{"submit", "run-123", taskFile, "--title", "Nope", "--json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runBinary(t, "http://127.0.0.1:1", tt.args...)
			if result.exitCode != exitCodeUsage {
				t.Fatalf("exit code = %d want %d stderr=%s stdout=%s", result.exitCode, exitCodeUsage, result.stderr, result.stdout)
			}
			if strings.TrimSpace(result.stdout) != "" {
				assertSingleJSONDocument(t, result.stdout)
			}
		})
	}
}

func TestSubmitRejectsCanonicalRunIDMismatch(t *testing.T) {
	taskFile := filepath.Join(t.TempDir(), "task.json")
	if err := os.WriteFile(taskFile, []byte(`{"version":"v1","run_id":"run-other","title":"Task","goal":"Ship it"}`), 0644); err != nil {
		t.Fatalf("write task json: %v", err)
	}

	result := runBinary(t, "http://127.0.0.1:1", "submit", "run-123", "--task-json", taskFile, "--json")
	if result.exitCode != exitCodeUsage {
		t.Fatalf("exit code = %d want %d stderr=%s stdout=%s", result.exitCode, exitCodeUsage, result.stderr, result.stdout)
	}
	assertSingleJSONDocument(t, result.stdout)
}

func TestSubmitGoalWaitJSONEmitsSingleTerminalPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/runs/run-123/steps":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"id":"step-123","phase_id":"phase-execution-run-123","title":"Direct task","goal":"Fix it","created_at":"2026-04-06T00:00:00Z","updated_at":"2026-04-06T00:00:00Z"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-123/result":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"v1","run_id":"run-123","phase_id":"phase-execution-run-123","step_id":"step-123","state":"completed","summary":"done"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	result := runBinary(t, server.URL, "submit", "run-123", "--goal", "Fix it", "--wait", "--json")
	if result.exitCode != exitCodeSuccess {
		t.Fatalf("exit code = %d, stderr = %s", result.exitCode, result.stderr)
	}
	assertSingleJSONDocument(t, result.stdout)
	if strings.Contains(result.stdout, "\"id\"") {
		t.Fatalf("stdout should contain only the terminal payload, got %s", result.stdout)
	}
}

func TestStepLogsJSONNoStdoutPollution(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/steps/step-123/logs" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("hello from logs"))
	}))
	defer server.Close()

	result := runBinary(t, server.URL, "step", "logs", "step-123", "--json")
	if result.exitCode != exitCodeSuccess {
		t.Fatalf("exit code = %d, stderr = %s", result.exitCode, result.stderr)
	}
	assertSingleJSONDocument(t, result.stdout)
	if strings.Contains(result.stdout, "Fetching logs") {
		t.Fatalf("stdout was polluted: %s", result.stdout)
	}
	if !strings.Contains(result.stdout, "\"available\": true") || !strings.Contains(result.stdout, "\"content\": \"hello from logs\"") {
		t.Fatalf("unexpected JSON payload: %s", result.stdout)
	}
}

func TestStepWaitJSONExitCodes(t *testing.T) {
	tests := []struct {
		name     string
		state    string
		wantCode int
	}{
		{name: "completed", state: "completed", wantCode: exitCodeSuccess},
		{name: "failed_terminal", state: "failed_terminal", wantCode: exitCodeTerminalFailed},
		{name: "timeout", state: "timeout", wantCode: exitCodeTimeout},
		{name: "needs_approval", state: "needs_approval", wantCode: exitCodeIntervention},
		{name: "failed_adapter", state: "failed_adapter", wantCode: exitCodeInfrastructure},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/steps/step-123/result" {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"run_id":"run-123","phase_id":"phase-1","step_id":"step-123","state":"` + tt.state + `","summary":"state test"}`))
			}))
			defer server.Close()

			result := runBinary(t, server.URL, "step", "wait", "step-123", "--json", "--interval", "1ms")
			if result.exitCode != tt.wantCode {
				t.Fatalf("exit code = %d want %d stderr=%s", result.exitCode, tt.wantCode, result.stderr)
			}
			assertSingleJSONDocument(t, result.stdout)
			if strings.Contains(result.stdout, "Waiting for step") {
				t.Fatalf("stdout was polluted: %s", result.stdout)
			}
		})
	}
}

func TestRunWaitJSONExitCodes(t *testing.T) {
	tests := []struct {
		name     string
		state    string
		wantCode int
	}{
		{name: "completed", state: "completed", wantCode: exitCodeSuccess},
		{name: "failed", state: "failed", wantCode: exitCodeTerminalFailed},
		{name: "cancelled", state: "cancelled", wantCode: exitCodeIntervention},
		{name: "paused_for_gate", state: "paused_for_gate", wantCode: exitCodeIntervention},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/runs/run-123" {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"id":"run-123","project_id":"proj","state":"` + tt.state + `","created_at":"2026-04-06T00:00:00Z","updated_at":"2026-04-06T00:00:00Z"}`))
			}))
			defer server.Close()

			result := runBinary(t, server.URL, "run", "wait", "run-123", "--json", "--interval", "1ms")
			if result.exitCode != tt.wantCode {
				t.Fatalf("exit code = %d want %d stderr=%s", result.exitCode, tt.wantCode, result.stderr)
			}
			assertSingleJSONDocument(t, result.stdout)
		})
	}
}

func TestRunWaitJSONTimeoutExitCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/runs/run-123" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"run-123","project_id":"proj","state":"running","created_at":"2026-04-06T00:00:00Z","updated_at":"2026-04-06T00:00:00Z"}`))
	}))
	defer server.Close()

	result := runBinary(t, server.URL, "run", "wait", "run-123", "--json", "--interval", "1ms", "--timeout", "5ms")
	if result.exitCode != exitCodeTimeout {
		t.Fatalf("exit code = %d want %d stderr=%s stdout=%s", result.exitCode, exitCodeTimeout, result.stderr, result.stdout)
	}
	assertSingleJSONDocument(t, result.stdout)
}

type binaryResult struct {
	stdout   string
	stderr   string
	exitCode int
}

func runBinary(t *testing.T, serverURL string, args ...string) binaryResult {
	return runBinaryWithStdin(t, serverURL, "", args...)
}

func runBinaryWithStdin(t *testing.T, serverURL string, stdin string, args ...string) binaryResult {
	t.Helper()

	cmd := exec.Command(testBinaryPath, args...)
	cmd.Env = append(os.Environ(), "ORCHESTRATORD_URL="+serverURL)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("running binary: %v", err)
		}
		exitCode = exitErr.ExitCode()
	}

	return binaryResult{
		stdout:   stdout.String(),
		stderr:   stderr.String(),
		exitCode: exitCode,
	}
}

func writeTempTaskFile(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "task.yaml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(contents)+"\n"), 0644); err != nil {
		t.Fatalf("write task file: %v", err)
	}
	return path
}

func assertSingleJSONDocument(t *testing.T, stdout string) {
	t.Helper()

	dec := json.NewDecoder(strings.NewReader(stdout))
	var payload any
	if err := dec.Decode(&payload); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout=%s", err, stdout)
	}

	var extra any
	if err := dec.Decode(&extra); err == nil {
		t.Fatalf("stdout contained multiple JSON documents: %s", stdout)
	}
}
