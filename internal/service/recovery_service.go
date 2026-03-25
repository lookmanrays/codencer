package service

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/storage/sqlite"
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

// SweepStaleRuns looks for runs stuck in "running" state across agent daemon restarts,
// marking them as paused or failed based on artifact footprint presence.
func (s *RecoveryService) SweepStaleRuns(ctx context.Context) error {
	slog.Info("Running recovery sweep for stale runs")

	runs, err := s.runsRepo.ListByState(ctx, domain.RunStateRunning)
	if err != nil {
		return fmt.Errorf("failed to list running tasks for recovery sweep: %w", err)
	}

	for _, r := range runs {
		// Reconcile leftover lock files.
		lockPath := fmt.Sprintf("%s/%s/.codencer.lock", s.workspaceRoot, r.ID)
		_ = os.Remove(lockPath)

		// Check all steps to salvage process output and intelligently resume.
		steps, _ := s.stepsRepo.ListByRun(ctx, r.ID)
		for _, step := range steps {
			if step.State == domain.StepStateRunning {
				artifactsPath := fmt.Sprintf("%s/%s/result.json", s.artifactRoot, step.ID)
				if _, err := os.Stat(artifactsPath); err == nil {
					// Artifact exists. The adapter completed writing results, but orchestrator died before finalizing.
					step.State = domain.StepStateNeedsApproval
					step.UpdatedAt = time.Now().UTC()
					_ = s.stepsRepo.UpdateState(ctx, step)
					slog.Info("Salvaged interrupted step results", "stepID", step.ID)
				} else {
					step.State = domain.StepStateFailedRetryable
					step.UpdatedAt = time.Now().UTC()
					_ = s.stepsRepo.UpdateState(ctx, step)
					slog.Warn("Marked interrupted step failed retryable", "stepID", step.ID)
				}
			}
		}

		// Re-attach the run into resumable Paused state rather than terminal Failed.
		r.State = domain.RunStatePausedForGate
		r.UpdatedAt = time.Now().UTC()
		if updateErr := s.runsRepo.UpdateState(ctx, r); updateErr != nil {
			slog.Error("Failed to update stale run state", "runID", r.ID, "error", updateErr)
			continue
		}
		slog.Warn("Reconciled stale run to PausedForGate for resumability", "runID", r.ID)
	}

	return nil
}

// Resume Run attempts to pick up a run that is in a valid resumable state (PausedForGate).
func (s *RecoveryService) ResumeRun(ctx context.Context, runID string) error {
	run, err := s.runsRepo.Get(ctx, runID)
	if err != nil {
		return err
	}
	
	if run.State != domain.RunStatePausedForGate {
		return fmt.Errorf("run %s is not in a resumable state", runID)
	}
	
	slog.Info("Resuming run", "runID", runID)
	
	run.State = domain.RunStateRunning
	run.UpdatedAt = time.Now().UTC()
	return s.runsRepo.UpdateState(ctx, run)
}
