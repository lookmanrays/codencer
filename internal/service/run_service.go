package service

import (
	"context"
	"fmt"
	"time"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/state"
	"agent-bridge/internal/storage/sqlite"
)

// RunService orchestrates run capabilities.
type RunService struct {
	repo *sqlite.RunsRepo
}

// NewRunService creates a new RunService.
func NewRunService(repo *sqlite.RunsRepo) *RunService {
	return &RunService{repo: repo}
}

// StartRun begins a new execution.
func (s *RunService) StartRun(ctx context.Context, id, projectID string) (*domain.Run, error) {
	now := time.Now().UTC()
	run := &domain.Run{
		ID:        id,
		ProjectID: projectID,
		State:     domain.RunStateCreated,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.Create(ctx, run); err != nil {
		return nil, err
	}

	// Immediately transition to running if possible
	if err := state.CheckRunTransition(run.State, domain.RunStateRunning); err == nil {
		run.State = domain.RunStateRunning
		run.UpdatedAt = time.Now().UTC()
		_ = s.repo.UpdateState(ctx, run)
	}

	return run, nil
}

// GetStatus returns the current run status.
func (s *RunService) GetStatus(ctx context.Context, id string) (*domain.Run, error) {
	return s.repo.Get(ctx, id)
}

// Abort transitions the run to cancelled if it is not already terminal.
func (s *RunService) Abort(ctx context.Context, id string) error {
	run, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if run == nil {
		return fmt.Errorf("run not found")
	}

	if run.State.IsTerminal() {
		return fmt.Errorf("run is already terminal")
	}

	if err := state.CheckRunTransition(run.State, domain.RunStateCancelled); err != nil {
		return err
	}

	run.State = domain.RunStateCancelled
	run.UpdatedAt = time.Now().UTC()
	return s.repo.UpdateState(ctx, run)
}
