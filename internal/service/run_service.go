package service

import (
	"context"
	"fmt"
	"log/slog"
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
	runsRepo      *sqlite.RunsRepo
	phasesRepo    *sqlite.PhasesRepo
	stepsRepo     *sqlite.StepsRepo
	attemptsRepo  *sqlite.AttemptsRepo
	gatesRepo     *sqlite.GatesRepo
	artifactsRepo *sqlite.ArtifactsRepo
	adapters      map[string]domain.Adapter
}

// NewRunService creates a new RunService.
func NewRunService(
	runsRepo *sqlite.RunsRepo,
	phasesRepo *sqlite.PhasesRepo,
	stepsRepo *sqlite.StepsRepo,
	attemptsRepo *sqlite.AttemptsRepo,
	gatesRepo *sqlite.GatesRepo,
	artifactsRepo *sqlite.ArtifactsRepo,
	adapters map[string]domain.Adapter,
) *RunService {
	return &RunService{
		runsRepo:      runsRepo,
		phasesRepo:    phasesRepo,
		stepsRepo:     stepsRepo,
		attemptsRepo:  attemptsRepo,
		gatesRepo:     gatesRepo,
		artifactsRepo: artifactsRepo,
		adapters:      adapters,
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

// GetArtifactsByAttempt returns all artifacts for an attempt.
func (s *RunService) GetArtifactsByAttempt(ctx context.Context, attemptID string) ([]*domain.Artifact, error) {
	return s.artifactsRepo.ListByAttempt(ctx, attemptID)
}

// GetGatesByRun returns all gates for a run.
func (s *RunService) GetGatesByRun(ctx context.Context, id string) ([]*domain.Gate, error) {
	return s.gatesRepo.ListByRun(ctx, id)
}

// GetArtifactsByStep returns all artifacts for all attempts of a step.
func (s *RunService) GetArtifactsByStep(ctx context.Context, stepID string) ([]*domain.Artifact, error) {
	attempts, err := s.attemptsRepo.ListByStep(ctx, stepID)
	if err != nil {
		return nil, err
	}
	var allArtifacts []*domain.Artifact
	for _, a := range attempts {
		arts, err := s.artifactsRepo.ListByAttempt(ctx, a.ID)
		if err == nil {
			allArtifacts = append(allArtifacts, arts...)
		}
	}
	return allArtifacts, nil
}

// GetResultByStep returns the result of the latest attempt for a step.
func (s *RunService) GetResultByStep(ctx context.Context, stepID string) (*domain.Result, error) {
	attempts, err := s.attemptsRepo.ListByStep(ctx, stepID)
	if err != nil {
		return nil, err
	}
	if len(attempts) == 0 {
		return nil, fmt.Errorf("no attempts found for step %s", stepID)
	}
	// ListByStep orders by number ASC. The last one is the latest attempt.
	latest := attempts[len(attempts)-1]
	if latest.Result == nil {
		return nil, fmt.Errorf("latest attempt %s has no result yet", latest.ID)
	}
	return latest.Result, nil
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
		return s.failStep(ctx, step, fmt.Sprintf("adapter %s not found", step.Adapter))
	}

	step.State = domain.StepStateRunning
	_ = s.stepsRepo.UpdateState(ctx, step)

	const maxAttempts = 3
	var finalResult *domain.Result
	var finalEval PolicyEvaluation

	// Load formal execution boundary definitions
	policy := domain.DefaultPolicy()

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

		res, eval, err := s.executeAttempt(ctx, runID, step, attempt, adapter, artifactRoot, policy)
		if err != nil {
			// System failure, not just adapter failure
			return fmt.Errorf("system error executing attempt: %w", err)
		}

		finalResult = res
		finalEval = eval

		if eval.ShouldFail {
			break
		}
		
		isFailed := res != nil && (res.Status == domain.StepStateFailedRetryable || res.Status == domain.StepStateFailedTerminal)
		if isFailed && policy.RetryWhen.AdapterProcessFailed && attemptNum < maxAttempts {
			time.Sleep(2 * time.Second)
			continue
		}
		
		break
	}

	return s.finalizeStep(ctx, runID, step, finalResult, finalEval)
}

func (s *RunService) failStep(ctx context.Context, step *domain.Step, reason string) error {
	step.State = domain.StepStateFailedTerminal
	step.UpdatedAt = time.Now().UTC()
	_ = s.stepsRepo.UpdateState(ctx, step)
	return fmt.Errorf(reason)
}

func (s *RunService) executeAttempt(
	ctx context.Context,
	runID string,
	step *domain.Step,
	attempt *domain.Attempt,
	adapter domain.Adapter,
	artifactRoot string,
	policy *domain.Policy,
) (*domain.Result, PolicyEvaluation, error) {
	// 1. Setup Environment
	workspaceRoot := "/tmp/codencer/workspace/" + runID
	baseRepo := "."
	branchName := "codencer-" + runID
	_ = workspace.CreateWorktree(ctx, baseRepo, workspaceRoot, branchName)
	defer workspace.RemoveWorktree(context.Background(), baseRepo, workspaceRoot)

	// 2. Start Execution
	if err := adapter.Start(ctx, attempt, workspaceRoot, artifactRoot); err != nil {
		attempt.Result = &domain.Result{Status: domain.StepStateFailedRetryable, Summary: "Adapter failed to start: " + err.Error()}
		s.updateAttemptResult(ctx, attempt)
		return attempt.Result, PolicyEvaluation{}, nil
	}

	// 3. Poll
	if !s.pollAdapter(ctx, adapter, attempt, s.attemptsRepo) {
		s.updateAttemptResult(ctx, attempt)
		return attempt.Result, PolicyEvaluation{}, nil
	}

	step.State = domain.StepStateCollectingArtifacts
	s.stepsRepo.UpdateState(ctx, step)

	// 4. Collect & Normalize
	artifacts, err := adapter.CollectArtifacts(ctx, attempt.ID, artifactRoot)
	if err != nil {
		attempt.Result = &domain.Result{Status: domain.StepStateFailedRetryable, Summary: "Artifact collection failed: " + err.Error()}
	} else {
		for _, art := range artifacts {
			_ = s.artifactsRepo.Create(ctx, art)
		}
		
		res, err := adapter.NormalizeResult(ctx, attempt.ID, artifacts)
		if err != nil {
			attempt.Result = &domain.Result{Status: domain.StepStateFailedRetryable, Summary: "Normalization failed: " + err.Error()}
		} else {
			attempt.Result = res
		}
	}

	s.updateAttemptResult(ctx, attempt)

	// 5. Evaluate Policy
	var changedFiles []string
	if os.Getenv("FORCE_GATE_FOR_TESTING") == "1" {
		changedFiles = append(changedFiles, "migrations/fake.sql")
	} else {
		if files, err := workspace.CaptureChangedFiles(ctx, workspaceRoot); err == nil {
			changedFiles = files
		}
	}

	eval := Evaluate(policy, attempt.Result, changedFiles)
	return attempt.Result, eval, nil
}

func (s *RunService) updateAttemptResult(ctx context.Context, attempt *domain.Attempt) {
	attempt.UpdatedAt = time.Now().UTC()
	s.attemptsRepo.UpdateResult(ctx, attempt)
}

func (s *RunService) finalizeStep(
	ctx context.Context,
	runID string,
	step *domain.Step,
	finalResult *domain.Result,
	eval PolicyEvaluation,
) error {
	fmt.Printf("DEBUG finalizeStep: ShouldGate=%v, ShouldFail=%v, finalResult=%+v\n", eval.ShouldGate, eval.ShouldFail, finalResult)
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
			fmt.Printf("CRITICAL ISOLATED GATE DB ERROR: %v\n", err)
			slog.Error("Failed to create isolated gate", "error", err)
			step.State = domain.StepStateFailedTerminal
		}
	} else if eval.ShouldFail || (finalResult != nil && finalResult.Status != domain.StepStateCompleted) {
		step.State = domain.StepStateFailedTerminal
	} else {
		step.State = domain.StepStateCompleted
	}

	step.UpdatedAt = time.Now().UTC()
	return s.stepsRepo.UpdateState(ctx, step)
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
