package validation

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// CaptureDiff runs git diff in the workspace and returns the patch content.
func CaptureDiff(ctx context.Context, workspaceRoot string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "diff")
	cmd.Dir = workspaceRoot

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	return string(out), nil
}

// CaptureChangedFiles runs git diff --name-only and returns the list.
func CaptureChangedFiles(ctx context.Context, workspaceRoot string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "diff", "--name-only")
	cmd.Dir = workspaceRoot

	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(out), "\n")
	var result []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result, nil
}
