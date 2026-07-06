package supervisor

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Err      error
}

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) CommandResult
}

type RealRunner struct {
	Timeout time.Duration
}

func (r RealRunner) Run(ctx context.Context, name string, args ...string) CommandResult {
	timeout := r.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, name, args...)
	stdout, err := cmd.Output()
	stderr := ""
	if exitErr := new(exec.ExitError); errors.As(err, &exitErr) {
		stderr = string(exitErr.Stderr)
	}
	result := CommandResult{
		Stdout: strings.TrimSpace(string(stdout)),
		Stderr: strings.TrimSpace(stderr),
		Err:    err,
	}
	if runCtx.Err() != nil {
		result.Err = runCtx.Err()
	}
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	} else if result.Err != nil {
		result.ExitCode = -1
	}
	return result
}

func runnerOrDefault(runner CommandRunner) CommandRunner {
	if runner != nil {
		return runner
	}
	return RealRunner{}
}
