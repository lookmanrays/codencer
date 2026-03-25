package service

import (
	"context"
	"fmt"
	"time"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/storage/sqlite"
)

// GateService handles the approval and rejection lifecycle of gates.
type GateService struct {
	repo     *sqlite.GatesRepo
	runsRepo *sqlite.RunsRepo
}

func NewGateService(repo *sqlite.GatesRepo, runsRepo *sqlite.RunsRepo) *GateService {
	return &GateService{repo: repo, runsRepo: runsRepo}
}

// Approve unlocks a paused run and transitions it back to running.
func (s *GateService) Approve(ctx context.Context, gateID string) error {
	return s.resolve(ctx, gateID, domain.GateStateApproved)
}

// Reject cancels a paused run explicitly due to gate rejection.
func (s *GateService) Reject(ctx context.Context, gateID string) error {
	return s.resolve(ctx, gateID, domain.GateStateRejected)
}

func (s *GateService) resolve(ctx context.Context, gateID string, state domain.GateState) error {
	gate, err := s.repo.Get(ctx, gateID)
	if err != nil {
		return err
	}
	if gate == nil {
		return fmt.Errorf("gate not found")
	}
	if gate.State != domain.GateStatePending {
		return fmt.Errorf("gate is already resolved")
	}

	if err := s.repo.Resolve(ctx, gateID, state); err != nil {
		return err
	}

	// Update associated run state
	run, err := s.runsRepo.Get(ctx, gate.RunID)
	if err != nil || run == nil {
		return fmt.Errorf("failed to fetch associated run: %w", err)
	}

	if state == domain.GateStateApproved {
		run.State = domain.RunStateRunning
	} else {
		run.State = domain.RunStateCancelled
	}
	run.UpdatedAt = time.Now().UTC()

	return s.runsRepo.UpdateState(ctx, run)
}
