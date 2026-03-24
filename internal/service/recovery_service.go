package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/storage/sqlite"
)

// RecoveryService handles failure recovery, stale run sweeps, and resumability.
type RecoveryService struct {
	runsRepo *sqlite.RunsRepo
}

func NewRecoveryService(runsRepo *sqlite.RunsRepo) *RecoveryService {
	return &RecoveryService{runsRepo: runsRepo}
}

// SweepStaleRuns looks for runs stuck in "running" state across agent daemon restarts,
// marking them as cancelled or failed based on policy.
func (s *RecoveryService) SweepStaleRuns(ctx context.Context) error {
	slog.Info("Running recovery sweep for stale runs")

	// For MVP, we pretend we pull all runs with state == Running and update them.
	// Since we don't have a ListRunning query in MVP runs_repo yet, this is a conceptual stub.
	
	// Real implementation would look like:
	// runs, err := s.runsRepo.ListByState(ctx, domain.RunStateRunning)
	// for _, r := range runs {
	//    r.State = domain.RunStateFailed
	//    r.UpdatedAt = time.Now()
	//    _ = s.runsRepo.UpdateState(ctx, r)
	//    slog.Info("Marked stale run as failed", "runID", r.ID)
	// }

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
