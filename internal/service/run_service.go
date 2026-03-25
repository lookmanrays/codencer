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

type RunService struct {
	runsRepo      *sqlite.RunsRepo
	phasesRepo    *sqlite.PhasesRepo
	stepsRepo     *sqlite.StepsRepo
	attemptsRepo  *sqlite.AttemptsRepo
	gatesRepo     *sqlite.GatesRepo
	artifactsRepo *sqlite.ArtifactsRepo
	validationsRepo *sqlite.ValidationsRepo
	routingSvc    *RoutingService
	artifactRoot  string
	workspaceRoot string
}

// NewRunService creates a new RunService.
func NewRunService(
	runsRepo *sqlite.RunsRepo,
	phasesRepo *sqlite.PhasesRepo,
	stepsRepo *sqlite.StepsRepo,
	attemptsRepo *sqlite.AttemptsRepo,
	gatesRepo *sqlite.GatesRepo,
	artifactsRepo *sqlite.ArtifactsRepo,
	validationsRepo *sqlite.ValidationsRepo,
	routingSvc *RoutingService,
	artifactRoot string,
	workspaceRoot string,
) *RunService {
	return &RunService{
		runsRepo:      runsRepo,
		phasesRepo:    phasesRepo,
		stepsRepo:     stepsRepo,
		attemptsRepo:  attemptsRepo,
		gatesRepo:     gatesRepo,
		artifactsRepo: artifactsRepo,
		validationsRepo: validationsRepo,
		routingSvc:    routingSvc,
		artifactRoot:  artifactRoot,
		workspaceRoot: workspaceRoot,
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
	return s.artifactsRepo.ListByStep(ctx, stepID)
}

// GetValidationsByStep returns all validations for all attempts of a step.
func (s *RunService) GetValidationsByStep(ctx context.Context, stepID string) (map[string][]*domain.ValidationResult, error) {
	return s.validationsRepo.ListByStep(ctx, stepID)
}

// GetValidationsByAttempt returns all validations for a specific attempt.
func (s *RunService) GetValidationsByAttempt(ctx context.Context, attemptID string) ([]*domain.ValidationResult, error) {
	return s.validationsRepo.ListByAttempt(ctx, attemptID)
}

// GetResultByStep returns the result of the latest attempt for a step.
func (s *RunService) GetResultByStep(ctx context.Context, stepID string) (*domain.Result, error) {
	step, err := s.stepsRepo.Get(ctx, stepID)
	if err != nil {
		return nil, err
	}
	if step == nil {
		return nil, fmt.Errorf("step %s not found", stepID)
	}

	attempts, err := s.attemptsRepo.ListByStep(ctx, stepID)
	if err != nil {
		return nil, err
	}
	if len(attempts) == 0 {
		// Return a pending result structure if no attempts yet
		return &domain.Result{
			Status:  step.State,
			Summary: "No attempts executed for this step yet.",
		}, nil
	}

	// ListByStep orders by number ASC. The last one is the latest attempt.
	latest := attempts[len(attempts)-1]
	if latest.Result == nil {
		return &domain.Result{
			Status:  step.State,
			Summary: fmt.Sprintf("Latest attempt %s is still in progress or failed before result normalization.", latest.ID),
		}, nil
	}
	return latest.Result, nil
}

// GetPhase returns a specific phase.
func (s *RunService) GetPhase(ctx context.Context, id string) (*domain.Phase, error) {
	return s.phasesRepo.Get(ctx, id)
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
func (s *RunService) DispatchStep(ctx context.Context, runID string, step *domain.Step) error {
	if err := s.initializeStep(ctx, step); err != nil {
		return err
	}

	fallbackChain, err := s.routingSvc.BuildFallbackChain(ctx, step.Adapter)
	if err != nil || len(fallbackChain) == 0 {
		return s.failStep(ctx, step, fmt.Sprintf("no viable adapters found for profile '%s'", step.Adapter))
	}

	step.State = domain.StepStateRunning
	_ = s.stepsRepo.UpdateState(ctx, step)

	finalResult, finalEval, err := s.runAttemptLoop(ctx, runID, step, fallbackChain)
	if err != nil {
		return err
	}

	return s.finalizeStep(ctx, runID, step, finalResult, finalEval)
}

func (s *RunService) initializeStep(ctx context.Context, step *domain.Step) error {
	existing, err := s.stepsRepo.Get(ctx, step.ID)
	if err == nil && existing != nil {
		// Step already exists, just reset state for a new attempt cycle
		step.State = domain.StepStateDispatching
		step.UpdatedAt = time.Now().UTC()
		return s.stepsRepo.UpdateState(ctx, step)
	}

	now := time.Now().UTC()
	step.State = domain.StepStateDispatching
	step.CreatedAt = now
	step.UpdatedAt = now

	if err := s.stepsRepo.Create(ctx, step); err != nil {
		return fmt.Errorf("failed to create step in store: %w", err)
	}
	return nil
}

func (s *RunService) runAttemptLoop(ctx context.Context, runID string, step *domain.Step, fallbackChain []string) (*domain.Result, PolicyEvaluation, error) {
	const maxAttempts = 3
	var finalResult *domain.Result
	var finalEval PolicyEvaluation
	policy := domain.DefaultPolicy()

	for attemptNum := 1; attemptNum <= maxAttempts; attemptNum++ {
		adapterProfile := s.selectAdapterProfile(fallbackChain, attemptNum)
		adapter, ok := s.routingSvc.GetAdapter(adapterProfile)
		if !ok {
			return nil, PolicyEvaluation{}, s.failStep(ctx, step, fmt.Sprintf("adapter %s out of bounds", adapterProfile))
		}

		attempt, err := s.createAttempt(ctx, step, attemptNum, adapterProfile)
		if err != nil {
			return nil, PolicyEvaluation{}, err
		}

		startTime := time.Now()
		res, eval, err := s.executeAttempt(ctx, runID, step, attempt, adapter, policy)
		duration := time.Since(startTime).Milliseconds()

		if err != nil {
			return nil, PolicyEvaluation{}, fmt.Errorf("system error executing attempt %d: %w", attemptNum, err)
		}

		s.logBenchmark(ctx, step.PhaseID, attempt.ID, adapterProfile, res, duration)

		finalResult = res
		finalEval = eval

		if eval.ShouldFail {
			break
		}

		if s.shouldRetry(res, policy, attemptNum, maxAttempts) {
			time.Sleep(2 * time.Second)
			continue
		}
		break
	}
	return finalResult, finalEval, nil
}

func (s *RunService) selectAdapterProfile(fallbackChain []string, attemptNum int) string {
	if attemptNum-1 < len(fallbackChain) {
		return fallbackChain[attemptNum-1]
	}
	return fallbackChain[0]
}

func (s *RunService) createAttempt(ctx context.Context, step *domain.Step, attemptNum int, adapterProfile string) (*domain.Attempt, error) {
	attempt := &domain.Attempt{
		ID:        fmt.Sprintf("%s-a%d", step.ID, attemptNum),
		StepID:    step.ID,
		Number:    attemptNum,
		Adapter:   adapterProfile,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if err := s.attemptsRepo.Create(ctx, attempt); err != nil {
		return nil, fmt.Errorf("failed to persist attempt: %w", err)
	}
	return attempt, nil
}

func (s *RunService) logBenchmark(ctx context.Context, phaseID, attemptID, adapterID string, res *domain.Result, durationMs int64) {
	benchScore := &domain.BenchmarkScore{
		ID:             fmt.Sprintf("bench-%s", attemptID),
		Adapter:        adapterID,
		PhaseID:        phaseID,
		DurationMs:     durationMs,
		ValidationsHit: 1,
		ValidationsMax: 1,
		CostCents:      0.0,
		CreatedAt:      time.Now().UTC(),
	}
	if res != nil && res.Status != domain.StepStateCompleted {
		benchScore.FailureReason = res.Summary
		benchScore.ValidationsHit = 0
	}
	_ = s.routingSvc.benchmarksRepo.Save(context.Background(), benchScore)
}

func (s *RunService) shouldRetry(res *domain.Result, policy *domain.Policy, attemptNum, maxAttempts int) bool {
	if res == nil {
		return false
	}
	isFailed := res.Status == domain.StepStateFailedRetryable || res.Status == domain.StepStateFailedTerminal
	return isFailed && policy.RetryWhen.AdapterProcessFailed && attemptNum < maxAttempts
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
	policy *domain.Policy,
) (*domain.Result, PolicyEvaluation, error) {
	// 1. Setup Environment
	workspaceRoot := fmt.Sprintf("%s/%s", s.workspaceRoot, runID)
	baseRepo := "."
	branchName := "codencer-" + runID
	
	if err := workspace.CreateWorktree(ctx, baseRepo, workspaceRoot, branchName); err != nil {
		slog.Error("Failed to create worktree", "runID", runID, "error", err)
		attempt.Result = &domain.Result{Status: domain.StepStateFailedRetryable, Summary: "Environment setup failed: " + err.Error()}
		s.updateAttemptResult(ctx, attempt)
		return attempt.Result, PolicyEvaluation{}, nil
	}
	defer func() {
		// Ensure cleanup happens even if subsequent steps fail
		_ = workspace.RemoveWorktree(context.Background(), baseRepo, workspaceRoot)
	}()

	// 2. Start Execution
	if err := adapter.Start(ctx, attempt, workspaceRoot, s.artifactRoot); err != nil {
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
	artifacts, err := adapter.CollectArtifacts(ctx, attempt.ID, s.artifactRoot)
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
