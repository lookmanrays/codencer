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
	return s.resolve(ctx, gateID, domain.GateStatusApproved)
}

// Reject cancels a paused run explicitly due to gate rejection.
func (s *GateService) Reject(ctx context.Context, gateID string) error {
	return s.resolve(ctx, gateID, domain.GateStatusRejected)
}

func (s *GateService) resolve(ctx context.Context, gateID string, status domain.GateStatus) error {
	gate, err := s.repo.Get(ctx, gateID)
	if err != nil {
		return err
	}
	if gate == nil {
		return fmt.Errorf("gate not found")
	}
	if gate.Status != domain.GateStatusPending {
		return fmt.Errorf("gate is already resolved")
	}

	now := time.Now().UTC()
	gate.Status = status
	gate.ResolvedAt = &now

	if err := s.repo.UpdateStatus(ctx, gate); err != nil {
		return err
	}

	// Update associated run state
	run, err := s.runsRepo.Get(ctx, gate.RunID)
	if err != nil || run == nil {
		return fmt.Errorf("failed to fetch associated run: %w", err)
	}

	if status == domain.GateStatusApproved {
		run.State = domain.RunStateRunning
	} else {
		run.State = domain.RunStateCancelled
	}
	run.UpdatedAt = now

	return s.runsRepo.UpdateState(ctx, run)
}
