package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/storage/sqlite"
	"agent-bridge/internal/workspace"
)

// RecoveryService handles failure recovery, stale run sweeps, and resumability.
type RecoveryService struct {
	runsRepo      *sqlite.RunsRepo
	stepsRepo     *sqlite.StepsRepo
	attemptsRepo  *sqlite.AttemptsRepo
	gatesRepo     *sqlite.GatesRepo
	artifactRoot  string
	workspaceRoot string
	repoRoot      string
}

func NewRecoveryService(
	runsRepo *sqlite.RunsRepo,
	stepsRepo *sqlite.StepsRepo,
	attemptsRepo *sqlite.AttemptsRepo,
	gatesRepo *sqlite.GatesRepo,
	artifactRoot string,
	workspaceRoot string,
	repoRoot ...string,
) *RecoveryService {
	baseRepoRoot := "."
	if len(repoRoot) > 0 && repoRoot[0] != "" {
		baseRepoRoot = repoRoot[0]
	}
	return &RecoveryService{
		runsRepo:      runsRepo,
		stepsRepo:     stepsRepo,
		attemptsRepo:  attemptsRepo,
		gatesRepo:     gatesRepo,
		artifactRoot:  artifactRoot,
		workspaceRoot: workspaceRoot,
		repoRoot:      baseRepoRoot,
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
		steps, stepsErr := s.stepsRepo.ListByRun(ctx, r.ID)
		if stepsErr != nil {
			return fmt.Errorf("failed to reload reconciled run steps: %w", stepsErr)
		}
		r.State = deriveRunState(steps)
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
	salvagedTerminal := 0
	salvagedGate := 0
	manualAttention := 0

	for _, step := range steps {
		if step.State.IsTerminal() || step.State == domain.StepStateNeedsApproval {
			continue
		}

		if gate, err := s.findPendingGate(ctx, run.ID, step.ID); err == nil && gate != nil {
			step.State = domain.StepStateNeedsApproval
			step.StatusReason = "Recovered paused step with a pending gate."
			step.UpdatedAt = time.Now().UTC()
			_ = s.stepsRepo.UpdateState(ctx, step)
			salvagedGate++
			continue
		}

		latestAttempt, err := s.attemptsRepo.GetLatestByStep(ctx, step.ID)
		if err == nil && latestAttempt != nil {
			if latestAttempt.Result != nil {
				switch {
				case latestAttempt.Result.State.IsTerminal():
					step.State = latestAttempt.Result.State
					step.StatusReason = latestAttempt.Result.Summary
					step.UpdatedAt = time.Now().UTC()
					_ = s.stepsRepo.UpdateState(ctx, step)
					salvagedTerminal++
					continue
				case latestAttempt.Result.State == domain.StepStateNeedsApproval:
					step.State = domain.StepStateNeedsApproval
					step.StatusReason = latestAttempt.Result.Summary
					step.UpdatedAt = time.Now().UTC()
					_ = s.stepsRepo.UpdateState(ctx, step)
					if err := s.ensureRecoveryGate(ctx, run.ID, step.ID); err != nil {
						slog.Warn("Failed to ensure recovery gate", "runID", run.ID, "stepID", step.ID, "error", err)
					}
					salvagedGate++
					continue
				}
			}
			if result, err := s.readResultEvidence(run.ID, step.ID, latestAttempt.ID); err == nil && result != nil {
				switch {
				case result.State.IsTerminal():
					step.State = result.State
					step.StatusReason = result.Summary
					step.UpdatedAt = time.Now().UTC()
					_ = s.stepsRepo.UpdateState(ctx, step)
					salvagedTerminal++
					continue
				case result.State == domain.StepStateNeedsApproval:
					step.State = domain.StepStateNeedsApproval
					step.StatusReason = result.Summary
					step.UpdatedAt = time.Now().UTC()
					_ = s.stepsRepo.UpdateState(ctx, step)
					if err := s.ensureRecoveryGate(ctx, run.ID, step.ID); err != nil {
						slog.Warn("Failed to ensure recovery gate", "runID", run.ID, "stepID", step.ID, "error", err)
					}
					salvagedGate++
					continue
				}
			}
		}

		step.State = domain.StepStateNeedsManualAttention
		step.StatusReason = "Bridge restarted while this step was active and could not verify a safe terminal outcome."
		step.UpdatedAt = time.Now().UTC()
		_ = s.stepsRepo.UpdateState(ctx, step)
		manualAttention++
	}

	return fmt.Sprintf(
		"Recovery sweep completed: restored %d terminal steps, restored %d gated steps, marked %d steps needs_manual_attention.",
		salvagedTerminal,
		salvagedGate,
		manualAttention,
	)
}

func (s *RecoveryService) cleanupOrphans(ctx context.Context) {
	worktrees, err := workspace.ListWorktrees(ctx, s.repoRoot)
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
			_ = workspace.RemoveWorktree(ctx, s.repoRoot, wt)
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

func (s *RecoveryService) findPendingGate(ctx context.Context, runID, stepID string) (*domain.Gate, error) {
	gates, err := s.gatesRepo.ListByRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	for _, gate := range gates {
		if gate.StepID == stepID && gate.State == domain.GateStatePending {
			return gate, nil
		}
	}
	return nil, nil
}

func (s *RecoveryService) ensureRecoveryGate(ctx context.Context, runID, stepID string) error {
	gate, err := s.findPendingGate(ctx, runID, stepID)
	if err != nil {
		return err
	}
	if gate != nil {
		return nil
	}
	recoveryGate := &domain.Gate{
		ID:          fmt.Sprintf("gate-recovery-%s", stepID),
		RunID:       runID,
		StepID:      stepID,
		Description: "Recovered stale step requires manual review before the run can continue.",
		State:       domain.GateStatePending,
		CreatedAt:   time.Now().UTC(),
	}
	return s.gatesRepo.Create(ctx, recoveryGate)
}

func (s *RecoveryService) readResultEvidence(runID, stepID, attemptID string) (*domain.ResultSpec, error) {
	resultPath := filepath.Join(s.artifactRoot, runID, stepID, attemptID, "result.json")
	data, err := os.ReadFile(resultPath)
	if err != nil {
		return nil, err
	}
	var result domain.ResultSpec
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
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
