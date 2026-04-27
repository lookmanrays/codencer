package workspace

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CreateWorktree creates a new linked working tree at destination from the base repo.
func CreateWorktree(ctx context.Context, baseRepoPath, destPath, branchName string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to ensure worktree parent directory: %w", err)
	}

	// Reusing a run ID should safely reclaim the old workspace path instead of
	// failing on a stale worktree registration from an earlier attempt/test.
	if err := RemoveWorktree(ctx, baseRepoPath, destPath); err != nil {
		return err
	}
	if err := pruneWorktrees(ctx, baseRepoPath); err != nil {
		return err
	}
	if existingPath, err := findWorktreePathForBranch(ctx, baseRepoPath, branchName); err != nil {
		return err
	} else if existingPath != "" && existingPath != destPath {
		// A run-scoped branch should only back one linked worktree. Reclaim any
		// stale path before recreating the workspace at the current destination.
		if err := RemoveWorktree(ctx, baseRepoPath, existingPath); err != nil {
			return err
		}
		if err := pruneWorktrees(ctx, baseRepoPath); err != nil {
			return err
		}
	}

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

func pruneWorktrees(ctx context.Context, baseRepoPath string) error {
	cmd := exec.CommandContext(ctx, "git", "worktree", "prune")
	cmd.Dir = baseRepoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to prune worktrees: %w. output: %s", err, string(out))
	}
	return nil
}

func findWorktreePathForBranch(ctx context.Context, baseRepoPath, branchName string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "worktree", "list", "--porcelain")
	cmd.Dir = baseRepoPath

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to inspect worktrees: %w. output: %s", err, string(out))
	}

	var currentPath string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "worktree "):
			currentPath = strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
		case strings.HasPrefix(line, "branch refs/heads/"):
			currentBranch := strings.TrimSpace(strings.TrimPrefix(line, "branch refs/heads/"))
			if currentBranch == branchName {
				return currentPath, nil
			}
		}
	}

	return "", nil
}

// RemoveWorktree safely removes a linked working tree.
func RemoveWorktree(ctx context.Context, baseRepoPath, destPath string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	remove := func(args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = baseRepoPath
		return cmd.CombinedOutput()
	}

	out, err := remove("worktree", "remove", "--force", destPath)
	if err == nil {
		return nil
	}
	if os.IsNotExist(err) || strings.Contains(string(out), "not a working tree") {
		return nil
	}
	if strings.Contains(string(out), "cannot remove a locked working tree") || strings.Contains(string(out), "use 'remove -f -f'") {
		unlockCmd := exec.CommandContext(ctx, "git", "worktree", "unlock", destPath)
		unlockCmd.Dir = baseRepoPath
		_, _ = unlockCmd.CombinedOutput()

		retryOut, retryErr := remove("worktree", "remove", "-f", "-f", destPath)
		if retryErr == nil || os.IsNotExist(retryErr) || strings.Contains(string(retryOut), "not a working tree") {
			return nil
		}
		return fmt.Errorf("failed to remove locked worktree: %w. output: %s", retryErr, string(retryOut))
	}
	return fmt.Errorf("failed to remove worktree: %w. output: %s", err, string(out))
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
