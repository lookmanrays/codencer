package service

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/storage/sqlite"
	"agent-bridge/internal/workspace"
	"path/filepath"
	"strings"
)

// RecoveryService handles failure recovery, stale run sweeps, and resumability.
type RecoveryService struct {
	runsRepo      *sqlite.RunsRepo
	stepsRepo     *sqlite.StepsRepo
	attemptsRepo  *sqlite.AttemptsRepo
	artifactRoot  string
	workspaceRoot string
}

func NewRecoveryService(
	runsRepo *sqlite.RunsRepo,
	stepsRepo *sqlite.StepsRepo,
	attemptsRepo *sqlite.AttemptsRepo,
	artifactRoot string,
	workspaceRoot string,
) *RecoveryService {
	return &RecoveryService{
		runsRepo:      runsRepo,
		stepsRepo:     stepsRepo,
		attemptsRepo:  attemptsRepo,
		artifactRoot:  artifactRoot,
		workspaceRoot: workspaceRoot,
	}
}

// SweepStaleRuns looks for runs stuck in inconsistent states and reconciles them.
func (s *RecoveryService) SweepStaleRuns(ctx context.Context) error {
	slog.Info("Running enhanced recovery sweep")

	// 1. Reconcile Running Runs
	runs, err := s.runsRepo.ListByState(ctx, domain.RunStateRunning)
	if err != nil {
		return fmt.Errorf("failed to list running runs: %w", err)
	}

	for _, r := range runs {
		currentOwner := workspace.CheckLock(s.workspaceRoot)
		if currentOwner == r.ID {
			// Still locked by the current process? 
			// In a local bridge, if we are in Sweep, we usually assume we are the only orchestrator.
			// If it's locked and we are here, it's either a concurrent orchestrator or a stale lock.
			// For now, we assume if we are sweeping, we OWN the workspace root.
		}

		notes := s.reconcileRunSteps(ctx, r)
		r.RecoveryNotes = notes
		r.State = domain.RunStatePausedForGate
		r.UpdatedAt = time.Now().UTC()
		_ = s.runsRepo.UpdateState(ctx, r)
		slog.Info("Reconciled stale run", "runID", r.ID, "notes", notes)
	}

	// 2. Clean up Orphaned Locks and Worktrees
	s.cleanupOrphans(ctx)

	return nil
}

func (s *RecoveryService) reconcileRunSteps(ctx context.Context, run *domain.Run) string {
	steps, _ := s.stepsRepo.ListByRun(ctx, run.ID)
	salvaged := 0
	failed := 0

	for _, step := range steps {
		if step.State.IsTerminal() || step.State == domain.StepStateNeedsApproval {
			continue
		}

		// Check for result footprint on disk in the latest attempt's namespaced folder
		latestAttempt, err := s.attemptsRepo.GetLatestByStep(ctx, step.ID)
		if err == nil && latestAttempt != nil {
			resultPath := filepath.Join(s.artifactRoot, run.ID, step.ID, latestAttempt.ID, "result.json")
			if _, err := os.Stat(resultPath); err == nil {
				// Result exists but DB state is not terminal. Salvage it.
				step.State = domain.StepStateNeedsApproval
				step.UpdatedAt = time.Now().UTC()
				_ = s.stepsRepo.UpdateState(ctx, step)
				salvaged++
				continue
			}
		}

		// No result found or no latest attempt. Mark as failed-retryable.
		step.State = domain.StepStateFailedRetryable
		step.UpdatedAt = time.Now().UTC()
		_ = s.stepsRepo.UpdateState(ctx, step)
		failed++
	}

	return fmt.Sprintf("Recovery sweep completed: salvaged %d steps, marked %d steps failed_retryable. Run paused for inspection.", salvaged, failed)
}

func (s *RecoveryService) cleanupOrphans(ctx context.Context) {
	worktrees, err := workspace.ListWorktrees(ctx, ".")
	if err != nil {
		slog.Warn("Failed to list worktrees for orphan cleanup", "error", err)
		return
	}

	cleanRoot, _ := filepath.Abs(filepath.Clean(s.workspaceRoot))
	for _, wt := range worktrees {
		absWt, _ := filepath.Abs(filepath.Clean(wt))
		// If worktree is inside our workspace root but not locked, it's an orphan
		if !strings.HasPrefix(absWt, cleanRoot) {
			continue
		}

		runID := filepath.Base(wt)
		// Heuristic: if runID is not the current lock owner, and run is not running, cleanup
		lockOwner := workspace.CheckLock(s.workspaceRoot)
		if lockOwner != runID {
			slog.Info("Cleaning up orphaned worktree", "path", wt, "runID", runID)
			_ = workspace.RemoveWorktree(ctx, ".", wt)
		}
	}

	// Also cleanup stale lock if no run is actually using it
	lockOwner := workspace.CheckLock(s.workspaceRoot)
	if lockOwner != "" {
		run, _ := s.runsRepo.Get(ctx, lockOwner)
		if run == nil || run.State.IsTerminal() {
			slog.Info("Removing stale lock file", "owner", lockOwner)
			lockPath := filepath.Join(s.workspaceRoot, ".codencer.lock")
			_ = os.Remove(lockPath)
		}
	}
}

// Resume Run attempts to pick up a run that is in a valid resumable state (PausedForGate).
func (s *RecoveryService) ResumeRun(ctx context.Context, runID string) error {
	run, err := s.runsRepo.Get(ctx, runID)
	if err != nil {
		return err
	}
	
	if run.State != domain.RunStatePausedForGate && run.State != domain.RunStateCreated {
		return fmt.Errorf("run %s is not in a resumable state (must be paused_for_gate or created)", runID)
	}

	// Ensure we can acquire the lock (it might be held by a sweep or another concurrent dispatch)
	owner := workspace.CheckLock(s.workspaceRoot)
	if owner != "" && owner != runID {
		return fmt.Errorf("cannot resume run %s: workspace is currently locked by run %s", runID, owner)
	}
	
	slog.Info("Resuming run", "runID", runID)
	
	run.State = domain.RunStateRunning
	run.UpdatedAt = time.Now().UTC()
	return s.runsRepo.UpdateState(ctx, run)
}
