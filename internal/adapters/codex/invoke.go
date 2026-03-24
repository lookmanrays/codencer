package codex

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"agent-bridge/internal/domain"
)

// InvokeLocal executes the Codex adapter as a local child process.
func InvokeLocal(ctx context.Context, attempt *domain.Attempt) error {
	// For MVP, we simulate a local cli invocation:
	// exec.CommandContext(ctx, "codex", "run", "--prompt", ...)

	// Simulate timeout handling
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "echo", fmt.Sprintf("Simulating Codex execution for attempt %s", attempt.ID))
	
	// In reality we would capture stdout/stderr to files in the artifactRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("codex execution timed out: %w", err)
		}
		if ctx.Err() == context.Canceled {
			return fmt.Errorf("codex execution cancelled: %w", err)
		}
		return fmt.Errorf("codex execution failed: %w. Output: %s", err, string(out))
	}

	return nil
}
