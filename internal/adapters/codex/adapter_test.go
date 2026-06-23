package codex

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-bridge/internal/adapters/common"
	"agent-bridge/internal/domain"
)

func TestAdapterExecModeUsesCodexCLIAndSynthesizesResult(t *testing.T) {
	tmp := t.TempDir()
	workspaceRoot := filepath.Join(tmp, "workspace")
	artifactRoot := filepath.Join(tmp, "artifacts")
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(artifactRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	argsPath := filepath.Join(tmp, "codex-args.txt")
	stdinPath := filepath.Join(tmp, "codex-stdin.txt")
	fakeCodex := filepath.Join(tmp, "fake-codex")
	script := `#!/bin/sh
printf '%s\n' "$@" > "$FAKE_CODEX_ARGS_PATH"
cat > "$FAKE_CODEX_STDIN_PATH"
last_message=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --output-last-message)
      shift
      last_message="$1"
      ;;
  esac
  shift
done
echo "fake codex stdout"
if [ -n "$last_message" ]; then
  printf 'fake codex final message\n' > "$last_message"
fi
`
	if err := os.WriteFile(fakeCodex, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv(codexBinaryEnv, fakeCodex)
	t.Setenv("FAKE_CODEX_ARGS_PATH", argsPath)
	t.Setenv("FAKE_CODEX_STDIN_PATH", stdinPath)
	t.Setenv("CODEX_SIMULATION_MODE", "0")
	t.Setenv("ALL_ADAPTERS_SIMULATION_MODE", "0")
	t.Setenv(codexAdapterModeEnv, "")

	adapter := NewAdapter()
	attempt := &domain.Attempt{ID: "attempt-1", Adapter: "codex"}
	step := &domain.Step{
		ID:      "step-1",
		Title:   "Flagship Codex",
		Goal:    "Return a short summary.",
		Adapter: "codex",
		TaskSpecSnapshot: &domain.TaskSpec{
			Version:        "v1",
			Goal:           "Return a short summary.",
			AdapterProfile: "codex",
		},
	}

	if err := adapter.Start(context.Background(), step, attempt, workspaceRoot, artifactRoot); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	waitForCodexAdapterStop(t, adapter, attempt.ID)

	argsData, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("fake codex did not capture args: %v", err)
	}
	args := string(argsData)
	argLines := strings.Split(strings.TrimSpace(args), "\n")
	if len(argLines) < 3 || argLines[0] != "--ask-for-approval" || argLines[1] != "never" || argLines[2] != "exec" {
		t.Fatalf("expected approval policy before exec for current Codex CLI compatibility, got:\n%s", args)
	}
	for _, want := range []string{"exec", "--cd", workspaceRoot, "--output-last-message", "-"} {
		if !strings.Contains(args, want) {
			t.Fatalf("expected codex args/prompt to contain %q, got:\n%s", want, args)
		}
	}
	stdinData, err := os.ReadFile(stdinPath)
	if err != nil {
		t.Fatalf("fake codex did not capture stdin: %v", err)
	}
	stdin := string(stdinData)
	for _, want := range []string{"Flagship Codex", "TaskSpec", artifactRoot} {
		if !strings.Contains(stdin, want) {
			t.Fatalf("expected codex stdin prompt to contain %q, got:\n%s", want, stdin)
		}
	}

	artifacts, err := adapter.CollectArtifacts(context.Background(), attempt.ID, artifactRoot)
	if err != nil {
		t.Fatalf("CollectArtifacts failed: %v", err)
	}
	result, err := adapter.NormalizeResult(context.Background(), attempt.ID, artifacts)
	if err != nil {
		t.Fatalf("NormalizeResult failed: %v", err)
	}
	if result.State != domain.StepStateCompleted {
		t.Fatalf("expected synthesized completed result, got %s", result.State)
	}
	if !strings.Contains(result.Summary, "fake codex final message") {
		t.Fatalf("expected synthesized summary from last message, got %q", result.Summary)
	}
	if result.Adapter != "codex" || result.IsSimulation {
		t.Fatalf("expected real codex adapter metadata, got adapter=%s simulation=%v", result.Adapter, result.IsSimulation)
	}
}

func TestAdapterExecModeIgnoresCwdDotEnvSimulation(t *testing.T) {
	tmp := t.TempDir()
	workspaceRoot := filepath.Join(tmp, "workspace")
	artifactRoot := filepath.Join(tmp, "artifacts")
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(artifactRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".env"), []byte("ALL_ADAPTERS_SIMULATION_MODE=1\nCODEX_SIMULATION_MODE=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeCodex := filepath.Join(tmp, "fake-codex")
	script := `#!/bin/sh
last_message=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --output-last-message)
      shift
      last_message="$1"
      ;;
  esac
  shift
done
echo "real codex cli stdout from cwd dotenv regression"
if [ -n "$last_message" ]; then
  printf 'real codex cli final message from cwd dotenv regression\n' > "$last_message"
fi
`
	if err := os.WriteFile(fakeCodex, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv(codexBinaryEnv, fakeCodex)
	t.Setenv("ALL_ADAPTERS_SIMULATION_MODE", "")
	t.Setenv("CODEX_SIMULATION_MODE", "")
	t.Setenv(codexAdapterModeEnv, "")
	previousCWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previousCWD); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})

	adapter := NewAdapter()
	attempt := &domain.Attempt{ID: "attempt-dotenv", Adapter: "codex"}
	step := &domain.Step{
		ID:      "step-dotenv",
		Title:   "CWD dotenv regression",
		Goal:    "Return a short summary.",
		Adapter: "codex",
		TaskSpecSnapshot: &domain.TaskSpec{
			Version:        "v1",
			Goal:           "Return a short summary.",
			AdapterProfile: "codex",
		},
	}

	if err := adapter.Start(context.Background(), step, attempt, workspaceRoot, artifactRoot); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	waitForCodexAdapterStop(t, adapter, attempt.ID)
	artifacts, err := adapter.CollectArtifacts(context.Background(), attempt.ID, artifactRoot)
	if err != nil {
		t.Fatalf("CollectArtifacts failed: %v", err)
	}
	result, err := adapter.NormalizeResult(context.Background(), attempt.ID, artifacts)
	if err != nil {
		t.Fatalf("NormalizeResult failed: %v", err)
	}
	if result.IsSimulation || strings.Contains(result.Summary, "Simulated") {
		t.Fatalf("expected cwd .env to be ignored for real Codex execution, got simulation=%v summary=%q", result.IsSimulation, result.Summary)
	}
	if !strings.Contains(result.Summary, "real codex cli final message") {
		t.Fatalf("expected fake Codex CLI output, got %q", result.Summary)
	}
}

func TestCodexFallbackWithoutResultOrLastMessageIsFailure(t *testing.T) {
	artifactRoot := t.TempDir()
	attempt := &domain.Attempt{ID: "attempt-empty", Adapter: "codex"}
	step := &domain.Step{ID: "step-empty", Adapter: "codex"}

	writeCodexFallbackResult(step, attempt, artifactRoot, "codex", nil)

	artifacts, err := common.CollectStandardArtifacts(context.Background(), attempt.ID, artifactRoot)
	if err != nil {
		t.Fatalf("collect artifacts: %v", err)
	}
	result, err := NormalizeCore(attempt.ID, artifacts, "codex", false)
	if err != nil {
		t.Fatalf("NormalizeCore failed: %v", err)
	}
	if result.State != domain.StepStateFailedTerminal {
		t.Fatalf("expected missing evidence fallback to fail terminal, got %s", result.State)
	}
	if !strings.Contains(result.Summary, "did not write result.json") {
		t.Fatalf("expected missing evidence summary, got %q", result.Summary)
	}
	if _, ok := result.Artifacts["codex_last_message_ref"]; ok {
		t.Fatalf("did not expect missing codex_last_message_ref artifact, got %+v", result.Artifacts)
	}
}

func TestNormalizeCoreRejectsUnknownResultState(t *testing.T) {
	artifactRoot := t.TempDir()
	resultPath := filepath.Join(artifactRoot, "result.json")
	if err := os.WriteFile(resultPath, []byte(`{"version":"v1","state":"success","summary":"not a domain state"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	artifacts, err := common.CollectStandardArtifacts(context.Background(), "attempt-unknown", artifactRoot)
	if err != nil {
		t.Fatalf("collect artifacts: %v", err)
	}
	result, err := NormalizeCore("attempt-unknown", artifacts, "codex", false)
	if err != nil {
		t.Fatalf("NormalizeCore failed: %v", err)
	}
	if result.State != domain.StepStateFailedTerminal {
		t.Fatalf("expected unsupported state to fail terminal, got %s", result.State)
	}
	if !strings.Contains(result.Summary, `unsupported state "success"`) {
		t.Fatalf("expected unsupported state summary, got %q", result.Summary)
	}
}

func TestAdapterLegacyModeKeepsCodexAgentArguments(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	artifactRoot := filepath.Join(t.TempDir(), "artifacts")
	step := &domain.Step{Title: "Legacy Title", Goal: "Legacy goal"}

	t.Setenv(codexAdapterModeEnv, codexLegacyMode)
	opts := codexExecutionOptions(step, workspaceRoot, artifactRoot)

	if opts.BinaryName != codexLegacyBinary {
		t.Fatalf("expected legacy binary %q, got %q", codexLegacyBinary, opts.BinaryName)
	}
	got := strings.Join(opts.Args, " ")
	for _, want := range []string{"run", "--workspace " + workspaceRoot, "--output " + artifactRoot, "--title Legacy Title", "--goal Legacy goal"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected legacy args to contain %q, got %q", want, got)
		}
	}
}

func waitForCodexAdapterStop(t *testing.T, adapter *Adapter, attemptID string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		running, err := adapter.Poll(context.Background(), attemptID)
		if err != nil {
			t.Fatalf("Poll failed: %v", err)
		}
		if !running {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for codex adapter attempt %s to stop", attemptID)
}
