package workspace

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// IsDirty checks whether the given repository path has uncommitted changes.
func IsDirty(ctx context.Context, repoPath string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = repoPath

	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check git status: %w", err)
	}

	if len(strings.TrimSpace(string(out))) > 0 {
		return true, nil
	}

	return false, nil
}
