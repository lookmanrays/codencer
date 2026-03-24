package claude

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"agent-bridge/internal/domain"
)

// InvokeLocal executes the Claude adapter.
// NOTE (AUTHENTICATION BLOCKER): Claude Code requires an active auth session and API keys.
// For the local-first MVP, we simulate the interaction pattern (subprocess -> artifacts) since
// we cannot guarantee headless auth. When run for real, this would execute `npx @anthropic-ai/claude-code`.
func InvokeLocal(ctx context.Context, attempt *domain.Attempt, workspaceRoot, artifactRoot string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	if err := os.MkdirAll(artifactRoot, 0755); err != nil {
		return fmt.Errorf("failed to create artifact root: %w", err)
	}

	stdoutPath := filepath.Join(artifactRoot, "claude_stdout.log")
	resultPath := filepath.Join(artifactRoot, "claude_result.json")

	script := fmt.Sprintf(`
		echo "Executing Claude Code simulator for attempt %s" > "%s"
		cat << 'EOF' > "%s"
{
  "status": "completed",
  "summary": "Claude simulator executed successfully without API key.",
  "needs_human_decision": false
}
EOF
	`, attempt.ID, stdoutPath, resultPath)

	cmd := exec.CommandContext(ctx, "bash", "-c", script)
	cmd.Dir = workspaceRoot

	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("claude execution timed out: %w", err)
		}
		if ctx.Err() == context.Canceled {
			return fmt.Errorf("claude execution cancelled: %w", err)
		}
		return fmt.Errorf("claude execution failed: %w. Output: %s", err, string(out))
	}

	return nil
}
