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

// InvokeLocal executes the Qwen adapter.
// NOTE (AUTHENTICATION BLOCKER): Proper local inference requires downloading models
// and setting up Python/llama.cpp environments which exceed MVP host assumptions.
// We simulate the interaction pattern (subprocess -> artifacts) since we cannot guarantee
// a local GPU or Qwen binary.
func InvokeLocal(ctx context.Context, attempt *domain.Attempt, workspaceRoot, artifactRoot string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	if err := os.MkdirAll(artifactRoot, 0755); err != nil {
		return fmt.Errorf("failed to create artifact root: %w", err)
	}

	stdoutPath := filepath.Join(artifactRoot, "qwen_stdout.log")
	resultPath := filepath.Join(artifactRoot, "qwen_result.json")

	script := fmt.Sprintf(`
		echo "Executing Qwen simulator for attempt %s" > "%s"
		cat << 'EOF' > "%s"
{
  "status": "completed",
  "summary": "Qwen simulator executed successfully.",
  "needs_human_decision": false
}
EOF
	`, attempt.ID, stdoutPath, resultPath)

	cmd := exec.CommandContext(ctx, "bash", "-c", script)
	cmd.Dir = workspaceRoot

	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("qwen execution timed out: %w", err)
		}
		if ctx.Err() == context.Canceled {
			return fmt.Errorf("qwen execution cancelled: %w", err)
		}
		return fmt.Errorf("qwen execution failed: %w. Output: %s", err, string(out))
	}

	return nil
}
