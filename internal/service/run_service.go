package service

import (
	"context"
	"fmt"
	"os"
	"time"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/state"
	"agent-bridge/internal/storage/sqlite"
	"agent-bridge/internal/workspace"
	"strings"
)

// RunService orchestrates run capabilities.
// RunService orchestrates run capabilities.
type RunService struct {
	runsRepo     *sqlite.RunsRepo
	phasesRepo   *sqlite.PhasesRepo
	stepsRepo    *sqlite.StepsRepo
	attemptsRepo *sqlite.AttemptsRepo
	gatesRepo    *sqlite.GatesRepo
	adapters     map[string]domain.Adapter
}

// NewRunService creates a new RunService.
func NewRunService(
	runsRepo *sqlite.RunsRepo,
	phasesRepo *sqlite.PhasesRepo,
	stepsRepo *sqlite.StepsRepo,
	attemptsRepo *sqlite.AttemptsRepo,
	gatesRepo *sqlite.GatesRepo,
	adapters map[string]domain.Adapter,
) *RunService {
	return &RunService{
		runsRepo:     runsRepo,
		phasesRepo:   phasesRepo,
		stepsRepo:    stepsRepo,
		attemptsRepo: attemptsRepo,
		gatesRepo:    gatesRepo,
		adapters:     adapters,
	}
}

// StartRun begins a new execution and creates a default phase.
func (s *RunService) StartRun(ctx context.Context, id, projectID string) (*domain.Run, error) {
	now := time.Now().UTC()
	run := &domain.Run{
		ID:        id,
		ProjectID: projectID,
		State:     domain.RunStateCreated,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.runsRepo.Create(ctx, run); err != nil {
		return nil, err
	}

	// Create default phase
	phase := &domain.Phase{
		ID:        fmt.Sprintf("phase-01-%s", id),
		RunID:     id,
		Name:      "Execution",
		SeqOrder:  1,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.phasesRepo.Create(ctx, phase); err != nil {
		return nil, fmt.Errorf("failed to auto-create default phase for run %s: %w", id, err)
	}

	// Immediately transition to running if possible
	if err := state.CheckRunTransition(run.State, domain.RunStateRunning); err == nil {
		run.State = domain.RunStateRunning
		run.UpdatedAt = time.Now().UTC()
		_ = s.runsRepo.UpdateState(ctx, run)
	}

	run.Phases = []*domain.Phase{phase}
	return run, nil
}

// GetRun returns the current run status.
func (s *RunService) GetRun(ctx context.Context, id string) (*domain.Run, error) {
	return s.runsRepo.Get(ctx, id)
}

// GetStep returns a specific step.
func (s *RunService) GetStep(ctx context.Context, id string) (*domain.Step, error) {
	return s.stepsRepo.Get(ctx, id)
}

// List returns all runs.
func (s *RunService) List(ctx context.Context) ([]*domain.Run, error) {
	return s.runsRepo.List(ctx)
}

// GetStepsByRun returns all steps for a run.
func (s *RunService) GetStepsByRun(ctx context.Context, id string) ([]*domain.Step, error) {
	return s.stepsRepo.ListByRun(ctx, id)
}

// GetGatesByRun returns all gates for a run.
func (s *RunService) GetGatesByRun(ctx context.Context, id string) ([]*domain.Gate, error) {
	return s.gatesRepo.ListByRun(ctx, id)
}

// Abort transitions the run to cancelled if it is not already terminal.
func (s *RunService) AbortRun(ctx context.Context, id string) error {
	run, err := s.runsRepo.Get(ctx, id)
	if err != nil {
		return err
	}
	if run == nil {
		return fmt.Errorf("run not found")
	}

	if run.State.IsTerminal() {
		return fmt.Errorf("run is already terminal in state %s", run.State)
	}

	run.State = domain.RunStateCancelled
	run.UpdatedAt = time.Now().UTC()
	return s.runsRepo.UpdateState(ctx, run)
}

// DispatchStep manages the core lifecycle of executing a step natively
func (s *RunService) DispatchStep(ctx context.Context, runID string, step *domain.Step, artifactRoot string) error {
	now := time.Now().UTC()
	step.State = domain.StepStateDispatching
	step.CreatedAt = now
	step.UpdatedAt = now

	if err := s.stepsRepo.Create(ctx, step); err != nil {
		return fmt.Errorf("failed to create step: %w", err)
	}

	adapter, ok := s.adapters[step.Adapter]
	if !ok {
		step.State = domain.StepStateFailedTerminal
		_ = s.stepsRepo.UpdateState(ctx, step)
		return fmt.Errorf("adapter %s not found", step.Adapter)
	}

	step.State = domain.StepStateRunning
	_ = s.stepsRepo.UpdateState(ctx, step)

	const maxAttempts = 3
	var finalResult *domain.Result
	var eval PolicyEvaluation

	// 4. Validation & Policy evaluate (Mock Policy)
	policy := &domain.Policy{}
	policy.GateWhen.ChangedFilesOver = -1
	policy.GateWhen.DependencyFilesChanged = true
	policy.GateWhen.MigrationsDetected = true
	policy.GateWhen.UnresolvedQuestionsPresent = true
	policy.FailWhen.ArtifactPersistenceFailed = true
	policy.RetryWhen.AdapterProcessFailed = true // Allow retry on failure

	for attemptNum := 1; attemptNum <= maxAttempts; attemptNum++ {
		attempt := &domain.Attempt{
			ID:        fmt.Sprintf("%s-a%d", step.ID, attemptNum),
			StepID:    step.ID,
			Number:    attemptNum,
			Adapter:   step.Adapter,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}

		if err := s.attemptsRepo.Create(ctx, attempt); err != nil {
			return fmt.Errorf("failed to create attempt: %w", err)
		}

		// 1. Start execution
		workspaceRoot := "/tmp/codencer/workspace/" + runID
		
		// Attempt to create isolated worktree, fallback to standard execution gracefully
		baseRepo := "."
		branchName := "codencer-" + runID
		_ = workspace.CreateWorktree(ctx, baseRepo, workspaceRoot, branchName)
		defer workspace.RemoveWorktree(context.Background(), baseRepo, workspaceRoot)

		if err := adapter.Start(ctx, attempt, workspaceRoot, artifactRoot); err != nil {
			attempt.Result = &domain.Result{Status: domain.StepStateFailedRetryable, Summary: "Adapter failed to start: " + err.Error()}
			_ = s.attemptsRepo.UpdateResult(ctx, attempt)
			goto EvaluatePh
		}

		// 2. Poll cycle
		if !s.pollAdapter(ctx, adapter, attempt, s.attemptsRepo) {
			goto EvaluatePh
		}

		step.State = domain.StepStateCollectingArtifacts
		s.stepsRepo.UpdateState(ctx, step)

		// 3. Artifact collection and Normalization
		if artifacts, err := adapter.CollectArtifacts(ctx, attempt.ID, artifactRoot); err == nil {
			if res, err := adapter.NormalizeResult(ctx, attempt.ID, artifacts); err == nil {
				attempt.Result = res
			} else {
				attempt.Result = &domain.Result{Status: domain.StepStateFailedRetryable, Summary: "Normalization failed: " + err.Error()}
			}
		} else {
			attempt.Result = &domain.Result{Status: domain.StepStateFailedRetryable, Summary: "Artifact collection failed: " + err.Error()}
		}

	EvaluatePh:
		attempt.UpdatedAt = time.Now().UTC()
		s.attemptsRepo.UpdateResult(ctx, attempt)
		finalResult = attempt.Result

		var changedFiles []string
		if os.Getenv("FORCE_GATE_FOR_TESTING") == "1" {
			changedFiles = append(changedFiles, "migrations/fake.sql")
		}

		eval = Evaluate(policy, attempt.Result, changedFiles)

		// Simple retry condition
		if eval.ShouldFail {
			break
		}
		if (attempt.Result.Status == domain.StepStateFailedRetryable || attempt.Result.Status == domain.StepStateFailedTerminal) && policy.RetryWhen.AdapterProcessFailed {
			if attemptNum < maxAttempts {
				time.Sleep(2 * time.Second)
				continue
			}
		}
		
		break
	}

	if eval.ShouldGate {
		step.State = domain.StepStateNeedsApproval
		gate := &domain.Gate{
			ID:          fmt.Sprintf("gate-%s", step.ID),
			RunID:       runID,
			StepID:      step.ID,
			Description: "Policy enforced gate: " + strings.Join(eval.GateReasons, ", "),
			Status:      domain.GateStatusPending,
			CreatedAt:   time.Now().UTC(),
		}
		
		if err := s.gatesRepo.Create(ctx, gate); err == nil {
			if run, err := s.runsRepo.Get(ctx, runID); err == nil && run != nil {
				run.State = domain.RunStatePausedForGate
				run.UpdatedAt = time.Now().UTC()
				_ = s.runsRepo.UpdateState(ctx, run)
			}
		} else {
			step.State = domain.StepStateFailedTerminal // Fallback if gate fails
		}
	} else if eval.ShouldFail || (finalResult != nil && finalResult.Status != domain.StepStateCompleted) {
		step.State = domain.StepStateFailedTerminal
	} else {
		step.State = domain.StepStateCompleted
	}

	step.UpdatedAt = time.Now().UTC()
	s.stepsRepo.UpdateState(ctx, step)

	return nil
}

func (s *RunService) pollAdapter(ctx context.Context, adapter domain.Adapter, attempt *domain.Attempt, repo *sqlite.AttemptsRepo) bool {
	pollTicker := time.NewTicker(2 * time.Second)
	defer pollTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = adapter.Cancel(context.Background(), attempt.ID)
			return false
		case <-pollTicker.C:
			running, err := adapter.Poll(ctx, attempt.ID)
			if err != nil {
				attempt.Result = &domain.Result{Status: domain.StepStateFailedTerminal, Summary: err.Error()}
				_ = repo.UpdateResult(ctx, attempt)
				return false
			}
			if !running {
				return true
			}
		}
	}
}
