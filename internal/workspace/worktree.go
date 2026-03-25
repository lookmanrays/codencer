package workspace

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// CreateWorktree creates a new linked working tree at destination from the base repo.
func CreateWorktree(ctx context.Context, baseRepoPath, destPath, branchName string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Check if branch exists
	checkCmd := exec.CommandContext(ctx, "git", "rev-parse", "--verify", branchName)
	checkCmd.Dir = baseRepoPath
	if err := checkCmd.Run(); err == nil {
		// Branch exists, just add worktree
		cmd := exec.CommandContext(ctx, "git", "worktree", "add", destPath, branchName)
		cmd.Dir = baseRepoPath
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to add worktree for existing branch: %w. output: %s", err, string(out))
		}
		return nil
	}

	// Branch does not exist, create it
	cmd := exec.CommandContext(ctx, "git", "worktree", "add", "-b", branchName, destPath)
	cmd.Dir = baseRepoPath

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create worktree with new branch: %w. output: %s", err, string(out))
	}
	return nil
}

// RemoveWorktree safely removes a linked working tree.
func RemoveWorktree(ctx context.Context, baseRepoPath, destPath string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "worktree", "remove", "--force", destPath)
	cmd.Dir = baseRepoPath

	out, err := cmd.CombinedOutput()
	if err != nil {
		if !os.IsNotExist(err) && !strings.Contains(string(out), "not a working tree") {
			return fmt.Errorf("failed to remove worktree: %w. output: %s", err, string(out))
		}
	}
	return nil
}

// CaptureChangedFiles returns a list of files modified strictly within the given worktree.
func CaptureChangedFiles(ctx context.Context, worktreePath string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = worktreePath

	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to capture changed files: %w. output: %s", err, string(out))
	}

	var changed []string
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) < 3 {
			continue
		}
		// Git porcelain format typically is: " M path/to/file" or "?? path/to/file"
		filePath := strings.TrimSpace(line[2:])
		changed = append(changed, filePath)
	}

	return changed, nil
}

// ListWorktrees returns a list of paths for all active linked worktrees.
func ListWorktrees(ctx context.Context, baseRepoPath string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "worktree", "list", "--porcelain")
	cmd.Dir = baseRepoPath

	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w. output: %s", err, string(out))
	}

	var worktrees []string
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "worktree ") {
			path := strings.TrimPrefix(line, "worktree ")
			worktrees = append(worktrees, strings.TrimSpace(path))
		}
	}
	return worktrees, nil
}
