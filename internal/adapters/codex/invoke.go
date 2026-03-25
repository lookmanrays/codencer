package codex

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"agent-bridge/internal/domain"
)

// InvokeLocal executes the Codex adapter as a local child process.
func InvokeLocal(ctx context.Context, attempt *domain.Attempt, workspaceRoot, artifactRoot string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	if err := os.MkdirAll(artifactRoot, 0755); err != nil {
		return fmt.Errorf("failed to create artifact root: %w", err)
	}

	stdoutPath := filepath.Join(artifactRoot, "stdout.log")
	resultPath := filepath.Join(artifactRoot, "result.json")

	// 1. Get binary from environment or use fallback
	codexBinary := os.Getenv("CODEX_BINARY")
	if codexBinary == "" {
		codexBinary = "codex-agent" // Fallback expected CLI name
	}

	// 2. We use a shell wrapper to easily redirect the standard output, 
	// or we can execute directly and attach stdout.
	// Since Codex might not be installed in all test environments, we allow 
	// CODEX_SIMULATION_MODE to fallback to the stub for E2E tests if necessary,
	// but strictly log it.
	
	if os.Getenv("CODEX_SIMULATION_MODE") == "1" {
		// Honest simulation logging
		script := fmt.Sprintf(`
			echo "Executing Simulated Codex for attempt %s" > "%s"
			cat << 'EOF' > "%s"
{
  "status": "completed",
  "summary": "Simulated successful task.",
  "needs_human_decision": false
}
EOF
		`, attempt.ID, stdoutPath, resultPath)
		cmd := exec.CommandContext(ctx, "bash", "-c", script)
		cmd.Dir = workspaceRoot
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("simulated codex execution failed: %w. Output: %s", err, string(out))
		}
		return nil
	}

	// Genuine Execution
	binaryPath, err := exec.LookPath(codexBinary)
	if err != nil {
		return fmt.Errorf("codex binary %q not found or not executable. Set CODEX_BINARY to a valid path: %w", codexBinary, err)
	}

	cmd := exec.CommandContext(ctx, binaryPath, "run", "--workspace", workspaceRoot, "--output", artifactRoot)
	cmd.Dir = workspaceRoot
	
	outFd, err := os.Create(stdoutPath)
	if err != nil {
		return fmt.Errorf("failed to create stdout log file: %w", err)
	}
	defer outFd.Close()

	cmd.Stdout = outFd
	cmd.Stderr = outFd

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("codex execution timed out after 30 minutes: %w", err)
		}
		if ctx.Err() == context.Canceled {
			return fmt.Errorf("codex execution cancelled by orchestrator: %w", err)
		}
		// If it failed, the actual error (like non-zero exit code) and context are in stdout/stderr log
		return fmt.Errorf("codex process exited with error: %w (see artifacts for details)", err)
	}

	return nil
}
