package qwen

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"agent-bridge/internal/domain"
)

// InvokeLocal executes the Qwen adapter as a local child process.
func InvokeLocal(ctx context.Context, attempt *domain.Attempt, workspaceRoot, artifactRoot string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	if err := os.MkdirAll(artifactRoot, 0755); err != nil {
		return fmt.Errorf("failed to create artifact root: %w", err)
	}

	stdoutPath := filepath.Join(artifactRoot, "stdout.log")
	resultPath := filepath.Join(artifactRoot, "result.json")

	qwenBinary := os.Getenv("QWEN_BINARY")
	if qwenBinary == "" {
		qwenBinary = "qwen-local" // Fallback expected CLI name
	}

	if os.Getenv("QWEN_SIMULATION_MODE") == "1" {
		script := fmt.Sprintf(`
			echo "Executing Simulated Qwen for attempt %s" > "%s"
			cat << 'EOF' > "%s"
{
  "status": "completed",
  "summary": "Simulated successful qwen task.",
  "needs_human_decision": false
}
EOF
		`, attempt.ID, stdoutPath, resultPath)
		cmd := exec.CommandContext(ctx, "bash", "-c", script)
		cmd.Dir = workspaceRoot
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("simulated qwen execution failed: %w. Output: %s", err, string(out))
		}
		return nil
	}

	// Genuine Execution
	binaryPath, err := exec.LookPath(qwenBinary)
	if err != nil {
		return fmt.Errorf("qwen binary %q not found or not executable. Set QWEN_BINARY to a valid path: %w", qwenBinary, err)
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
			return fmt.Errorf("qwen execution timed out after 30 minutes: %w", err)
		}
		if ctx.Err() == context.Canceled {
			return fmt.Errorf("qwen execution cancelled by orchestrator: %w", err)
		}
		// If it failed, the actual error (like non-zero exit code) and context are in stdout/stderr log
		return fmt.Errorf("qwen process exited with error: %w (see artifacts for details)", err)
	}

	return nil
}
