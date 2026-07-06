package validation

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"agent-bridge/internal/domain"
)

// Runner executes validation commands in the context of the workspace.
type Runner struct {
}

// NewRunner creates a new validation runner.
func NewRunner() *Runner {
	return &Runner{}
}

// Run executes a validation command and returns the structured result.
func (r *Runner) Run(ctx context.Context, cmdSpec domain.ValidationCommand, workspaceRoot string) (*domain.ValidationResult, error) {
	return r.RunWithArtifacts(ctx, cmdSpec, workspaceRoot, "")
}

func (r *Runner) RunWithArtifacts(ctx context.Context, cmdSpec domain.ValidationCommand, workspaceRoot, artifactRoot string) (*domain.ValidationResult, error) {
	timeout := 10 * time.Minute
	if cmdSpec.TimeoutSeconds > 0 {
		timeout = time.Duration(cmdSpec.TimeoutSeconds) * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	started := time.Now()
	result := &domain.ValidationResult{
		Name:      firstNonEmpty(cmdSpec.Name, "validation"),
		Command:   cmdSpec.Command,
		State:     domain.ValidationStateRunning,
		Passed:    false,
		ExitCode:  -1,
		UpdatedAt: started.UTC(),
	}

	if strings.TrimSpace(cmdSpec.Command) == "" {
		result.State = domain.ValidationStateErrored
		result.Error = "empty command"
		result.DurationMs = time.Since(started).Milliseconds()
		return &domain.ValidationResult{
			Name:       result.Name,
			Command:    result.Command,
			State:      result.State,
			Passed:     result.Passed,
			ExitCode:   result.ExitCode,
			Error:      result.Error,
			DurationMs: result.DurationMs,
			UpdatedAt:  result.UpdatedAt,
		}, nil
	}

	cmd := shellCommand(ctx, cmdSpec.Command)
	cmd.Dir = workspaceRoot

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(started)
	result.DurationMs = duration.Milliseconds()
	result.UpdatedAt = time.Now().UTC()
	result.StdoutRef, result.StderrRef = writeValidationOutput(artifactRoot, result.Name, stdout.Bytes(), stderr.Bytes())

	if err != nil {
		result.State = domain.ValidationStateFailed
		result.Passed = false
		result.Error = fmt.Sprintf("command failed: %v", err)
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		}
		if ctx.Err() == context.DeadlineExceeded {
			result.State = domain.ValidationStateTimeout
			result.Error = fmt.Sprintf("command timed out after %s", timeout)
		}
		return result, nil
	}

	result.State = domain.ValidationStatePassed
	result.Passed = true
	result.ExitCode = 0
	return result, nil
}

func shellCommand(ctx context.Context, command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", "/C", command)
	}
	return exec.CommandContext(ctx, "sh", "-c", command)
}

func writeValidationOutput(artifactRoot, name string, stdout, stderr []byte) (string, string) {
	if artifactRoot == "" {
		return "", ""
	}
	if err := os.MkdirAll(artifactRoot, 0755); err != nil {
		return "", ""
	}
	safeName := sanitizeValidationName(name)
	stdoutPath := filepath.Join(artifactRoot, "validation-"+safeName+"-stdout.log")
	stderrPath := filepath.Join(artifactRoot, "validation-"+safeName+"-stderr.log")
	if err := os.WriteFile(stdoutPath, stdout, 0644); err != nil {
		stdoutPath = ""
	}
	if err := os.WriteFile(stderrPath, stderr, 0644); err != nil {
		stderrPath = ""
	}
	return stdoutPath, stderrPath
}

func sanitizeValidationName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "validation"
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteString("-")
		}
	}
	if b.Len() == 0 {
		return "validation"
	}
	return b.String()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
