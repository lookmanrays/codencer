package common

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"agent-bridge/internal/domain"
)

// ExecutionOptions defines how a local process should be invoked.
type ExecutionOptions struct {
	AdapterName  string
	BinaryName   string
	BinaryEnvVar string
	Args         []string
	Timeout      time.Duration
	Workspace    string
	ArtifactRoot string
}

// InvokeLocal manages the lifecycle of a local adapter process.
func InvokeLocal(ctx context.Context, attempt *domain.Attempt, opts ExecutionOptions) error {
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	if err := os.MkdirAll(opts.ArtifactRoot, 0755); err != nil {
		return fmt.Errorf("failed to create artifact root: %w", err)
	}

	stdoutPath := filepath.Join(opts.ArtifactRoot, "stdout.log")

	// 1. Check for Simulation Mode
	if IsSimulationEnabled(opts.AdapterName) {
		return RunSimulation(ctx, attempt, opts.ArtifactRoot, opts.Workspace)
	}

	// 2. Resolve Binary
	binary := os.Getenv(opts.BinaryEnvVar)
	if binary == "" {
		binary = opts.BinaryName
	}

	binaryPath, err := exec.LookPath(binary)
	if err != nil {
		return fmt.Errorf("%s binary %q not found or not executable. Set %s to a valid path: %w", opts.BinaryName, binary, opts.BinaryEnvVar, err)
	}

	// 3. Prepare Command
	cmd := exec.CommandContext(ctx, binaryPath, opts.Args...)
	cmd.Dir = opts.Workspace
	
	outFd, err := os.Create(stdoutPath)
	if err != nil {
		return fmt.Errorf("failed to create stdout log file: %w", err)
	}
	defer outFd.Close()

	cmd.Stdout = outFd
	cmd.Stderr = outFd

	// 4. Run & Handle Result
	slog.Info("Adapter Execution: Starting process", "adapter", opts.AdapterName, "attemptID", attempt.ID, "binary", binaryPath)
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("%s execution timed out: %w", opts.BinaryName, err)
		}
		if ctx.Err() == context.Canceled {
			return fmt.Errorf("%s execution cancelled: %w", opts.BinaryName, err)
		}
		return fmt.Errorf("%s process exited with error: %w (see stdout.log for details)", opts.BinaryName, err)
	}

	return nil
}
