package validation

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"agent-bridge/internal/domain"
)

// Runner executes validation commands in the context of the workspace.
type Runner struct {
	workspaceRoot string
}

// NewRunner creates a new validation runner mapping to the given root directory.
func NewRunner(workspaceRoot string) *Runner {
	return &Runner{workspaceRoot: workspaceRoot}
}

// Run executes a validation command and returns the structured result.
func (r *Runner) Run(ctx context.Context, cmdSpec domain.ValidationCommand) (*domain.ValidationResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	parts := strings.Fields(cmdSpec.Command)
	if len(parts) == 0 {
		return &domain.ValidationResult{
			Name:   cmdSpec.Name,
			Passed: false,
			Error:  "empty command",
		}, nil
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = r.workspaceRoot

	// Capture outputs
	out, err := cmd.CombinedOutput()

	result := &domain.ValidationResult{
		Name:   cmdSpec.Name,
		Passed: true,
	}

	if err != nil {
		result.Passed = false
		result.Error = fmt.Sprintf("Command failed: %v\nOutput: %s", err, string(out))
		if ctx.Err() == context.DeadlineExceeded {
			result.Error = "Command timed out."
		}
	}

	return result, nil
}
