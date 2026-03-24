package workspace

import (
	"fmt"
	"os"
	"path/filepath"
)

// Lock represents an active lock claim on a repository.
type Lock struct {
	path string
}

// AcquireLock attempts to claim an exclusive file lock on a repo.
func AcquireLock(repoPath, id string) (*Lock, error) {
	lockPath := filepath.Join(repoPath, ".codencer.lock")

	// Atomically try to create the lock file
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("repository %s is already locked by another run", repoPath)
		}
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}

	_, _ = file.WriteString(id)
	_ = file.Close()

	return &Lock{path: lockPath}, nil
}

// Release removes the lock.
func (l *Lock) Release() error {
	if err := os.Remove(l.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to release lock: %w", err)
	}
	return nil
}
