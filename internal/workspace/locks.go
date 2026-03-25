package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Lock represents an active lock claim on a repository.
type Lock struct {
	path string
}

// AcquireLock attempts to claim an exclusive file lock on a repo.
// Returns the lock if successful, or an error containing the current owner ID if already locked.
func AcquireLock(repoPath, id string) (*Lock, error) {
	lockPath := filepath.Join(repoPath, ".codencer.lock")

	// Atomically try to create the lock file
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsExist(err) {
			owner, _ := os.ReadFile(lockPath)
			return nil, fmt.Errorf("repository %s is already locked by run %s", repoPath, string(owner))
		}
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer file.Close()

	_, _ = file.WriteString(id)
	return &Lock{path: lockPath}, nil
}

// CheckLock returns the ID of the run currently holding the lock at repoPath, or empty if unlocked.
func CheckLock(repoPath string) string {
	lockPath := filepath.Join(repoPath, ".codencer.lock")
	data, err := os.ReadFile(lockPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// Release removes the lock.
func (l *Lock) Release() error {
	if err := os.Remove(l.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to release lock: %w", err)
	}
	return nil
}
