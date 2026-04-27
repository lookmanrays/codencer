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
	repo         *sqlite.GatesRepo
	runsRepo     *sqlite.RunsRepo
	stepsRepo    *sqlite.StepsRepo
	attemptsRepo *sqlite.AttemptsRepo
}

func NewGateService(repo *sqlite.GatesRepo, runsRepo *sqlite.RunsRepo, stepsRepo *sqlite.StepsRepo, attemptsRepo *sqlite.AttemptsRepo) *GateService {
	return &GateService{repo: repo, runsRepo: runsRepo, stepsRepo: stepsRepo, attemptsRepo: attemptsRepo}
}

// Get returns a single gate by ID.
func (s *GateService) Get(ctx context.Context, gateID string) (*domain.Gate, error) {
	return s.repo.Get(ctx, gateID)
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

	step, err := s.stepsRepo.Get(ctx, gate.StepID)
	if err != nil {
		return err
	}
	if step == nil {
		return fmt.Errorf("step %s for gate %s not found", gate.StepID, gateID)
	}

	switch state {
	case domain.GateStateApproved:
		latestAttempt, err := s.attemptsRepo.GetLatestByStep(ctx, gate.StepID)
		if err != nil {
			return err
		}
		if latestAttempt == nil || latestAttempt.Result == nil {
			step.State = domain.StepStateNeedsManualAttention
			step.StatusReason = "Gate approved, but no terminal attempt result was available to restore step state."
		} else {
			step.State = latestAttempt.Result.State
			if latestAttempt.Result.State == domain.StepStateCancelled {
				step.StatusReason = "Gate approved after a cancelled attempt; manual retry may be required."
			} else {
				step.StatusReason = latestAttempt.Result.Summary
			}
		}
	case domain.GateStateRejected:
		step.State = domain.StepStateCancelled
		step.StatusReason = "Gate rejected by operator."
	}
	step.UpdatedAt = time.Now().UTC()
	if err := s.stepsRepo.UpdateState(ctx, step); err != nil {
		return err
	}

	run, err := s.runsRepo.Get(ctx, gate.RunID)
	if err != nil {
		return err
	}
	if run == nil {
		return fmt.Errorf("run %s for gate %s not found", gate.RunID, gateID)
	}

	steps, err := s.stepsRepo.ListByRun(ctx, gate.RunID)
	if err != nil {
		return err
	}
	run.State = deriveRunState(steps)
	run.UpdatedAt = time.Now().UTC()

	return s.runsRepo.UpdateState(ctx, run)
}
