package workspace

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"agent-bridge/internal/domain"
)

// Provisioner handles preparing the attempt worktree.
type Provisioner interface {
	// Provision prepares the worktree at the given location using the spec.
	// baseRepo is the absolute path to the stable repository root.
	// workspaceRoot is the absolute path to the attempt-specific worktree.
	Provision(ctx context.Context, spec *domain.ProvisioningSpec, baseRepo, workspaceRoot string) (*domain.ProvisioningResult, error)
}

// LocalProvisioner is the standard engine for preparing attempt worktrees.
type LocalProvisioner struct{}

func NewLocalProvisioner() *LocalProvisioner {
	return &LocalProvisioner{}
}

func (p *LocalProvisioner) Provision(ctx context.Context, spec *domain.ProvisioningSpec, baseRepo, workspaceRoot string) (*domain.ProvisioningResult, error) {
	start := time.Now()
	res := &domain.ProvisioningResult{
		Success: true,
		Log:     []string{},
	}

	if spec == nil {
		res.DurationMs = time.Since(start).Milliseconds()
		return res, nil
	}

	// 1. Copy Files
	for _, relPath := range spec.Copy {
		if err := p.validatePath(relPath); err != nil {
			return p.fail(res, start, "Invalid copy path: "+relPath, err)
		}

		src := filepath.Join(baseRepo, relPath)
		dst := filepath.Join(workspaceRoot, relPath)
		
		res.Log = append(res.Log, fmt.Sprintf("[COPY] %s", relPath))
		if err := p.copyFile(src, dst); err != nil {
			return p.fail(res, start, "Copy failed for "+relPath, err)
		}
	}

	// 2. Symlink Directories
	for _, relPath := range spec.Symlinks {
		if err := p.validatePath(relPath); err != nil {
			return p.fail(res, start, "Invalid symlink path: "+relPath, err)
		}

		src := filepath.Join(baseRepo, relPath)
		dst := filepath.Join(workspaceRoot, relPath)
		
		res.Log = append(res.Log, fmt.Sprintf("[SYMLINK] %s", relPath))
		// Remove existing to allow idempotency/retries in the same folder if needed
		_ = os.Remove(dst)
		if err := os.Symlink(src, dst); err != nil {
			return p.fail(res, start, "Symlink failed for "+relPath, err)
		}
	}

	// 3. PostCreate Hook
	if spec.Hooks.PostCreate != "" {
		res.Log = append(res.Log, fmt.Sprintf("[HOOK] %s", spec.Hooks.PostCreate))
		slog.Info("Provision: executing PostCreate hook", "hook", spec.Hooks.PostCreate)
		
		cmd := exec.CommandContext(ctx, "sh", "-c", spec.Hooks.PostCreate)
		cmd.Dir = workspaceRoot
		
		output, err := cmd.CombinedOutput()
		if len(output) > 0 {
			res.Log = append(res.Log, string(output))
		}
		
		if err != nil {
			return p.fail(res, start, "Post-create hook failed", err)
		}
	}

	res.DurationMs = time.Since(start).Milliseconds()
	return res, nil
}

func (p *LocalProvisioner) validatePath(relPath string) error {
	if filepath.IsAbs(relPath) {
		return fmt.Errorf("absolute paths are not allowed: %s", relPath)
	}
	// Clean and ensure no parent traversal
	clean := filepath.Clean(relPath)
	if strings.Contains(clean, "..") {
		return fmt.Errorf("parent traversal is not allowed: %s", relPath)
	}
	return nil
}

func (p *LocalProvisioner) fail(res *domain.ProvisioningResult, start time.Time, summary string, err error) (*domain.ProvisioningResult, error) {
	res.Success = false
	res.Summary = fmt.Sprintf("%s: %v", summary, err)
	res.DurationMs = time.Since(start).Milliseconds()
	return res, err
}

func (p *LocalProvisioner) copyFile(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	d, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer d.Close()

	_, err = io.Copy(d, s)
	return err
}

// NullProvisioner is a pass-through provisioner that does nothing.
type NullProvisioner struct{}

func (p *NullProvisioner) Provision(ctx context.Context, spec *domain.ProvisioningSpec, baseRepo, workspaceRoot string) (*domain.ProvisioningResult, error) {
	return &domain.ProvisioningResult{Success: true}, nil
}

func NewNullProvisioner() *NullProvisioner {
	return &NullProvisioner{}
}
