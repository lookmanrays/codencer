package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"agent-bridge/internal/domain"
)

func runAttempt(ctx context.Context, state *processState, step *domain.Step, attempt *domain.Attempt, workspaceRoot, attemptArtifactRoot string) error {
	if err := os.MkdirAll(attemptArtifactRoot, 0755); err != nil {
		return fmt.Errorf("failed to create artifact root: %w", err)
	}

	stdoutPath := filepath.Join(attemptArtifactRoot, "stdout.log")
	stderrPath := filepath.Join(attemptArtifactRoot, "stderr.log")
	resultPath := filepath.Join(attemptArtifactRoot, "result.json")
	promptPath := filepath.Join(attemptArtifactRoot, "prompt.txt")

	prompt, err := writePromptArtifact(promptPath, step)
	if err != nil {
		return fmt.Errorf("failed to write prompt artifact: %w", err)
	}

	stdoutFile, err := os.Create(stdoutPath)
	if err != nil {
		return fmt.Errorf("failed to create stdout log: %w", err)
	}
	defer stdoutFile.Close()

	stderrFile, err := os.Create(stderrPath)
	if err != nil {
		return fmt.Errorf("failed to create stderr log: %w", err)
	}
	defer stderrFile.Close()

	binaryPath, err := resolveBinary()
	if err != nil {
		return err
	}

	cmd := newCommand(ctx, binaryPath)
	cmd.Dir = workspaceRoot
	cmd.Stdin = strings.NewReader(prompt)
	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}

	state.mu.Lock()
	state.cmd = cmd
	state.mu.Unlock()

	runErr := cmd.Run()

	state.mu.Lock()
	state.cmd = nil
	state.mu.Unlock()

	synth := synthesizeResult(attempt.ID, stdoutPath, stderrPath, runErr)
	if err := writeResult(resultPath, synth); err != nil {
		return fmt.Errorf("failed to write synthesized result: %w", err)
	}

	if runErr != nil && ctx.Err() == nil {
		return fmt.Errorf("claude process exited with error: %w", runErr)
	}

	return nil
}

func stopProcess(state *processState) error {
	state.mu.Lock()
	cmd := state.cmd
	state.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return nil
	}

	if runtime.GOOS == "windows" {
		if err := cmd.Process.Kill(); err != nil && !isProcessDoneError(err) {
			return err
		}
		return nil
	}

	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM); err != nil && !isProcessDoneError(err) && err != syscall.ESRCH {
		return err
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil && !isProcessDoneError(err) {
		return err
	}

	return nil
}

func isProcessDoneError(err error) bool {
	return err == os.ErrProcessDone || strings.Contains(err.Error(), "process already finished")
}

func writeResult(path string, res *domain.ResultSpec) error {
	data, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func synthesizeResult(attemptID, stdoutPath, stderrPath string, runErr error) *domain.ResultSpec {
	parsed, parseErr := parseClaudeResultFromStdout(stdoutPath)
	if parseErr == nil {
		parsed.AttemptID = attemptID
		parsed.Version = "v1"
		parsed.RawOutputRef = stdoutPath
		parsed.UpdatedAt = time.Now().UTC()
		if runErr != nil && parsed.State == domain.StepStateCompleted {
			parsed.State = domain.StepStateFailedAdapter
			parsed.Summary = fmt.Sprintf("Claude exited non-zero after reporting success: %v", runErr)
		}
		return parsed
	}

	if runErr != nil && errorsContainContextCancellation(runErr) {
		return &domain.ResultSpec{
			Version:      "v1",
			AttemptID:    attemptID,
			State:        domain.StepStateCancelled,
			Summary:      "Claude execution cancelled.",
			RawOutputRef: stdoutPath,
			UpdatedAt:    time.Now().UTC(),
		}
	}

	summary := fmt.Sprintf("Malformed or missing Claude result output: %v", parseErr)
	if runErr != nil {
		summary = fmt.Sprintf("%s (process error: %v)", summary, runErr)
	}
	if stderrText, err := readTrimmed(stderrPath); err == nil && stderrText != "" {
		summary = fmt.Sprintf("%s. stderr: %s", summary, stderrText)
	}

	return &domain.ResultSpec{
		Version:      "v1",
		AttemptID:    attemptID,
		State:        domain.StepStateFailedTerminal,
		Summary:      summary,
		RawOutputRef: stdoutPath,
		UpdatedAt:    time.Now().UTC(),
	}
}

func readTrimmed(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func errorsContainContextCancellation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "context canceled") || strings.Contains(msg, "signal: killed")
}
