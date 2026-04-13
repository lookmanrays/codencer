package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/state"
	"agent-bridge/internal/storage/sqlite"
	"agent-bridge/internal/validation"
	"agent-bridge/internal/workspace"
	sqlite3driver "github.com/mattn/go-sqlite3"
)

type activeExecution struct {
	stepID         string
	attemptID      string
	adapter        domain.Adapter
	cancel         context.CancelFunc
	done           chan struct{}
	abortRequested bool
}

type RunService struct {
	runsRepo         *sqlite.RunsRepo
	phasesRepo       *sqlite.PhasesRepo
	stepsRepo        *sqlite.StepsRepo
	attemptsRepo     *sqlite.AttemptsRepo
	gatesRepo        *sqlite.GatesRepo
	artifactsRepo    *sqlite.ArtifactsRepo
	validationsRepo  *sqlite.ValidationsRepo
	routingSvc       *RoutingService
	policyRegistry   *PolicyRegistry
	validationRunner *validation.Runner
	provisioner      workspace.Provisioner
	artifactRoot     string
	workspaceRoot    string
	repoRoot         string
	execMu           sync.Mutex
	executions       map[string]*activeExecution
}

var (
	ErrConflict             = errors.New("conflict")
	ErrInvalidTaskSpec      = errors.New("invalid task spec")
	ErrArtifactAccessDenied = errors.New("artifact access denied")
)

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
	policyReg *PolicyRegistry,
	provisioner workspace.Provisioner,
	artifactRoot string,
	workspaceRoot string,
	repoRoot ...string,
) *RunService {
	baseRepoRoot := "."
	if len(repoRoot) > 0 && repoRoot[0] != "" {
		baseRepoRoot = repoRoot[0]
	}
	return &RunService{
		runsRepo:         runsRepo,
		phasesRepo:       phasesRepo,
		stepsRepo:        stepsRepo,
		attemptsRepo:     attemptsRepo,
		gatesRepo:        gatesRepo,
		artifactsRepo:    artifactsRepo,
		validationsRepo:  validationsRepo,
		routingSvc:       routingSvc,
		policyRegistry:   policyReg,
		validationRunner: validation.NewRunner(),
		provisioner:      provisioner,
		artifactRoot:     artifactRoot,
		workspaceRoot:    workspaceRoot,
		repoRoot:         baseRepoRoot,
		executions:       make(map[string]*activeExecution),
	}
}

// StartRun initiates an execution session (Run) as requested by the planner.
// It creates the necessary sequence containers (Phases) to house upcoming steps.
func (s *RunService) StartRun(ctx context.Context, id, projectID, conversationID, plannerID, executorID string) (*domain.Run, error) {
	now := time.Now().UTC()
	run := &domain.Run{
		ID:             id,
		ProjectID:      projectID,
		ConversationID: conversationID,
		PlannerID:      plannerID,
		ExecutorID:     executorID,
		State:          domain.RunStateCreated,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.runsRepo.Create(ctx, run); err != nil {
		if isUniqueConstraint(err) {
			return nil, fmt.Errorf("%w: run %s already exists", ErrConflict, id)
		}
		return nil, err
	}

	// Create default phase
	phase := &domain.Phase{
		ID:        fmt.Sprintf("phase-execution-%s", id),
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

// List returns runs based on optional filters.
func (s *RunService) List(ctx context.Context, filters map[string]string) ([]*domain.Run, error) {
	return s.runsRepo.List(ctx, filters)
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

// GetArtifact returns a single artifact by ID.
func (s *RunService) GetArtifact(ctx context.Context, artifactID string) (*domain.Artifact, error) {
	return s.artifactsRepo.Get(ctx, artifactID)
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
func (s *RunService) GetResultByStep(ctx context.Context, stepID string) (*domain.ResultSpec, error) {
	step, err := s.stepsRepo.Get(ctx, stepID)
	if err != nil {
		return nil, err
	}
	if step == nil {
		return nil, fmt.Errorf("step %s not found", stepID)
	}
	phase, _ := s.phasesRepo.Get(ctx, step.PhaseID)
	runID := ""
	if phase != nil {
		runID = phase.RunID
	}

	attempts, err := s.attemptsRepo.ListByStep(ctx, stepID)
	if err != nil {
		return nil, err
	}
	if len(attempts) == 0 {
		return s.newResultSpec(runID, step, nil, step.State, "No attempts executed for this step yet."), nil
	}

	// ListByStep orders by number ASC. The last one is the latest attempt.
	latest := attempts[len(attempts)-1]
	if latest.Result == nil {
		return s.newResultSpec(runID, step, latest, step.State, fmt.Sprintf("Latest attempt %s is still in progress or failed before result normalization.", latest.ID)), nil
	}

	// Fetch validations for this attempt
	if vals, verr := s.validationsRepo.ListByAttempt(ctx, latest.ID); verr == nil {
		latest.Result.Validations = make([]domain.ValidationResult, len(vals))
		for i, v := range vals {
			latest.Result.Validations[i] = *v
		}
	}

	// Enrichment for terminal consumers
	s.ensureResultEnvelope(latest.Result, runID, step, latest)

	return latest.Result, nil
}

// GetPhase returns a specific phase.
func (s *RunService) GetPhase(ctx context.Context, id string) (*domain.Phase, error) {
	return s.phasesRepo.Get(ctx, id)
}

// GetBenchmarks returns recent benchmark scores.
func (s *RunService) GetBenchmarks(ctx context.Context, adapter string) ([]*domain.BenchmarkScore, error) {
	return s.routingSvc.benchmarksRepo.GetScoresByAdapter(ctx, adapter)
}

// GetRoutingConfig returns the current static fallback chain.
func (s *RunService) GetRoutingConfig(ctx context.Context) map[string]interface{} {
	chain, _ := s.routingSvc.BuildHeuristicChain(ctx, "")
	return map[string]interface{}{
		"mode":       "Heuristic Static Fallback",
		"chain":      chain,
		"disclaimer": "Benchmark data is currently logged but NOT used for dynamic routing decisions.",
	}
}

const abortGracePeriod = 5 * time.Second

// AbortRun is a best-effort cancellation request.
// It reports success only after the active step actually reaches the cancelled state.
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

	execution := s.getExecution(id)
	if execution == nil {
		steps, serr := s.stepsRepo.ListByRun(ctx, id)
		if serr != nil {
			return serr
		}
		hasNonTerminal := false
		hasPendingGate := false
		for _, step := range steps {
			if step.State == domain.StepStateNeedsApproval {
				hasPendingGate = true
				continue
			}
			if step.State.IsTerminal() {
				continue
			}
			hasNonTerminal = true
			step.State = domain.StepStateNeedsManualAttention
			step.StatusReason = "Abort requested, but no active execution was registered for this run."
			step.UpdatedAt = time.Now().UTC()
			if err := s.stepsRepo.UpdateState(ctx, step); err != nil {
				return err
			}
		}
		if err := s.reconcileRunState(ctx, id); err != nil {
			return err
		}
		switch {
		case hasPendingGate:
			return fmt.Errorf("abort requested for run %s, but the run is paused_for_gate; resolve the pending gate explicitly before retrying or abandoning the run", id)
		case hasNonTerminal:
			return fmt.Errorf("abort requested for run %s, but no active execution was registered; non-terminal steps were moved to needs_manual_attention", id)
		default:
			return fmt.Errorf("abort requested for run %s, but no active execution was registered", id)
		}
	}

	s.requestAbort(id)
	if execution.cancel != nil {
		execution.cancel()
	}

	waitCtx, cancel := context.WithTimeout(ctx, abortGracePeriod+2*time.Second)
	defer cancel()

	select {
	case <-execution.done:
		return s.abortOutcome(ctx, id, execution.stepID)
	case <-waitCtx.Done():
		return fmt.Errorf("abort requested for run %s, but the active execution did not stop within %s", id, abortGracePeriod+2*time.Second)
	}
}

// DispatchStepAsync persists dispatch state synchronously, then continues execution in the background.
func (s *RunService) DispatchStepAsync(ctx context.Context, runID string, step *domain.Step) error {
	return s.dispatchStepAsync(ctx, runID, step, false)
}

func (s *RunService) dispatchStepAsync(ctx context.Context, runID string, step *domain.Step, allowExisting bool) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	if err := s.initializeStep(context.Background(), runID, step, allowExisting); err != nil {
		return err
	}

	dispatchCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	s.registerExecution(runID, step.ID, cancel, done)

	go func() {
		if err := s.dispatchPreparedStep(context.Background(), dispatchCtx, runID, step, done); err != nil {
			slog.Error("Failed to dispatch step asynchronously", "runID", runID, "stepID", step.ID, "error", err)
		}
	}()

	return nil
}

// DispatchStep handles the tactical execution of a planner-issued Step.
// It manages adapter selection, environment setup, and terminal state reporting.
func (s *RunService) DispatchStep(ctx context.Context, runID string, step *domain.Step) error {
	if err := s.initializeStep(ctx, runID, step, true); err != nil {
		return err
	}

	baseCtx := ctx
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	dispatchCtx, cancel := context.WithCancel(baseCtx)
	done := make(chan struct{})
	s.registerExecution(runID, step.ID, cancel, done)

	return s.dispatchPreparedStep(ctx, dispatchCtx, runID, step, done)
}

func (s *RunService) dispatchPreparedStep(persistCtx, dispatchCtx context.Context, runID string, step *domain.Step, done chan struct{}) error {
	defer s.clearExecution(runID)
	defer close(done)

	finalizeEarlyAbort := func() error {
		finalizationCtx := context.Background()
		if err := s.finalizeStep(finalizationCtx, runID, step, nil, PolicyEvaluation{}, ""); err != nil {
			return err
		}
		return s.reconcileRunState(finalizationCtx, runID)
	}

	if dispatchCtx.Err() != nil || s.isAbortRequested(runID) {
		return finalizeEarlyAbort()
	}

	fallbackChain, err := s.routingSvc.BuildHeuristicChain(dispatchCtx, step.Adapter)
	if err != nil || len(fallbackChain) == 0 {
		if dispatchCtx.Err() != nil || s.isAbortRequested(runID) {
			return finalizeEarlyAbort()
		}
		return s.failStep(dispatchCtx, step, fmt.Sprintf("no viable adapters found for profile '%s'", step.Adapter))
	}

	step.State = domain.StepStateRunning
	step.UpdatedAt = time.Now().UTC()
	if err := s.stepsRepo.UpdateState(dispatchCtx, step); err != nil {
		if dispatchCtx.Err() != nil || s.isAbortRequested(runID) {
			return finalizeEarlyAbort()
		}
		return err
	}

	finalResult, finalEval, finalAttemptID, err := s.runAttemptLoop(dispatchCtx, runID, step, fallbackChain)
	if err != nil {
		return err
	}

	finalizationCtx := persistCtx
	if finalizationCtx == nil || finalizationCtx.Err() != nil {
		finalizationCtx = context.Background()
	}
	if err := s.finalizeStep(finalizationCtx, runID, step, finalResult, finalEval, finalAttemptID); err != nil {
		return err
	}
	return s.reconcileRunState(finalizationCtx, runID)
}

// RetryStep re-dispatches an existing step.
func (s *RunService) RetryStep(ctx context.Context, stepID string) error {
	step, err := s.stepsRepo.Get(ctx, stepID)
	if err != nil {
		return err
	}
	if step == nil {
		return fmt.Errorf("step %s not found", stepID)
	}

	phase, err := s.phasesRepo.Get(ctx, step.PhaseID)
	if err != nil {
		return err
	}
	if phase == nil {
		return fmt.Errorf("phase %s for step %s not found", step.PhaseID, stepID)
	}

	return s.dispatchStepAsync(ctx, phase.RunID, step, true)
}

func (s *RunService) initializeStep(ctx context.Context, runID string, step *domain.Step, allowExisting bool) error {
	if step != nil && step.PhaseID == "" {
		step.PhaseID = fmt.Sprintf("phase-execution-%s", runID)
	}
	if err := s.validateTaskSpecSnapshot(runID, step); err != nil {
		return err
	}

	// Ensure the phase exists before creating the step to prevent orphan references
	if err := s.ensurePhaseExists(ctx, runID, step.PhaseID); err != nil {
		return fmt.Errorf("failed to ensure phase consistency: %w", err)
	}

	existing, err := s.stepsRepo.Get(ctx, step.ID)
	if err != nil {
		return err
	}
	if existing != nil {
		if err := s.ensureExistingStepBelongsToRun(ctx, runID, existing); err != nil {
			return err
		}
		if !allowExisting {
			return fmt.Errorf("%w: step %s already exists", ErrConflict, step.ID)
		}
		// Step already exists, just reset state for a new attempt cycle.
		step.State = domain.StepStateDispatching
		step.UpdatedAt = time.Now().UTC()
		return s.stepsRepo.UpdateState(ctx, step)
	}

	now := time.Now().UTC()
	step.State = domain.StepStateDispatching
	step.CreatedAt = now
	step.UpdatedAt = now

	if err := s.stepsRepo.Create(ctx, step); err != nil {
		if isUniqueConstraint(err) {
			return fmt.Errorf("%w: step %s already exists", ErrConflict, step.ID)
		}
		return fmt.Errorf("failed to create step in store: %w", err)
	}
	return nil
}

func (s *RunService) ensurePhaseExists(ctx context.Context, runID, phaseID string) error {
	existing, err := s.phasesRepo.Get(ctx, phaseID)
	if err != nil {
		return err
	}
	if existing != nil {
		if existing.RunID != runID {
			return fmt.Errorf("%w: phase %s belongs to run %s, not %s", ErrInvalidTaskSpec, phaseID, existing.RunID, runID)
		}
		return nil
	}

	// Phase doesn't exist, auto-create it under this run
	now := time.Now().UTC()
	phase := &domain.Phase{
		ID:        phaseID,
		RunID:     runID,
		Name:      "Tactical Phase",
		SeqOrder:  99,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.phasesRepo.Create(ctx, phase); err != nil {
		if !isUniqueConstraint(err) {
			return err
		}
		existing, getErr := s.phasesRepo.Get(ctx, phaseID)
		if getErr != nil {
			return getErr
		}
		if existing == nil {
			return err
		}
		if existing.RunID != runID {
			return fmt.Errorf("%w: phase %s belongs to run %s, not %s", ErrInvalidTaskSpec, phaseID, existing.RunID, runID)
		}
	}
	return nil
}

func (s *RunService) runAttemptLoop(ctx context.Context, runID string, step *domain.Step, fallbackChain []string) (*domain.ResultSpec, PolicyEvaluation, string, error) {
	const maxAttempts = 3
	var finalResult *domain.ResultSpec
	var finalEval PolicyEvaluation
	var finalAttemptID string
	policy := s.policyRegistry.Lookup(step.Policy)

	for attemptNum := 1; attemptNum <= maxAttempts; attemptNum++ {
		if s.isAbortRequested(runID) {
			break
		}

		adapterProfile := s.selectAdapterProfile(fallbackChain, attemptNum)
		adapter, ok := s.routingSvc.GetAdapter(adapterProfile)
		if !ok {
			return nil, PolicyEvaluation{}, "", s.failStep(ctx, step, fmt.Sprintf("adapter %s out of bounds", adapterProfile))
		}

		attempt, err := s.createAttempt(ctx, step, attemptNum, adapterProfile)
		if err != nil {
			return nil, PolicyEvaluation{}, "", err
		}

		startTime := time.Now()
		res, eval, err := s.executeAttempt(ctx, runID, step, attempt, adapter, policy)
		duration := time.Since(startTime).Milliseconds()

		if err != nil {
			return nil, PolicyEvaluation{}, "", fmt.Errorf("system error executing attempt %d: %w", attemptNum, err)
		}

		isSim := os.Getenv(strings.ToUpper(adapterProfile)+"_SIMULATION_MODE") == "1" || os.Getenv("ALL_ADAPTERS_SIMULATION_MODE") == "1"
		s.logBenchmark(ctx, step.PhaseID, attempt.ID, adapterProfile, res, duration, isSim)

		finalResult = res
		finalEval = eval
		finalAttemptID = attempt.ID

		if s.isAbortRequested(runID) {
			break
		}

		if eval.ShouldFail {
			break
		}

		if s.shouldRetry(res, policy, attemptNum, maxAttempts) {
			time.Sleep(2 * time.Second)
			continue
		}
		break
	}
	return finalResult, finalEval, finalAttemptID, nil
}

func (s *RunService) selectAdapterProfile(fallbackChain []string, attemptNum int) string {
	if attemptNum-1 < len(fallbackChain) {
		return fallbackChain[attemptNum-1]
	}
	return fallbackChain[0]
}

func (s *RunService) createAttempt(ctx context.Context, step *domain.Step, attemptNum int, adapterProfile string) (*domain.Attempt, error) {
	attempt := &domain.Attempt{
		ID:        fmt.Sprintf("%s-a%d-%d", step.ID, attemptNum, time.Now().UnixNano()),
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

func (s *RunService) logBenchmark(ctx context.Context, phaseID, attemptID, adapterID string, res *domain.ResultSpec, durationMs int64, isSimulation bool) {
	benchScore := &domain.BenchmarkScore{
		ID:         fmt.Sprintf("bench-%s", attemptID),
		Adapter:    adapterID,
		PhaseID:    phaseID,
		AttemptID:  attemptID,
		DurationMs: durationMs,
		// ValidationsHit/Max are currently binary success markers (1/1 or 0/1)
		// until the domain.Result model is expanded to include count metadata.
		ValidationsHit: 1,
		ValidationsMax: 1,
		CostCents:      0.0,
		IsSimulation:   isSimulation,
		CreatedAt:      time.Now().UTC(),
	}
	if res != nil && res.State == domain.StepStateNeedsApproval {
		benchScore.FailureReason = res.Summary
		benchScore.ValidationsHit = 0
	}
	_ = s.routingSvc.benchmarksRepo.Save(context.Background(), benchScore)
}

func (s *RunService) shouldRetry(res *domain.ResultSpec, policy *domain.Policy, attemptNum, maxAttempts int) bool {
	if res == nil {
		return false
	}
	isFailed := res.State == domain.StepStateFailedRetryable || res.State == domain.StepStateFailedTerminal
	return isFailed && policy.RetryWhen.AdapterProcessFailed && attemptNum < maxAttempts
}

func (s *RunService) failStep(ctx context.Context, step *domain.Step, reason string) error {
	step.State = domain.StepStateFailedBridge
	step.StatusReason = reason
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
) (*domain.ResultSpec, PolicyEvaluation, error) {
	// 1. Setup Environment
	workspaceRoot := filepath.Join(s.workspaceRoot, runID)
	attemptArtifactRoot := filepath.Join(s.artifactRoot, runID, step.ID, attempt.ID)
	baseRepo := s.repoRoot
	branchName := "codencer-" + runID
	s.setExecutionAttempt(runID, attempt.ID, adapter)

	// Create workspace dir if not exists (parent of worktree)
	_ = os.MkdirAll(s.workspaceRoot, 0755)
	if err := os.MkdirAll(attemptArtifactRoot, 0755); err != nil {
		reason := fmt.Sprintf("Failed to create attempt artifact root: %v", err)
		attempt.Result = s.newResultSpec(runID, step, attempt, domain.StepStateFailedBridge, reason)
		step.StatusReason = reason
		s.updateAttemptResult(ctx, attempt)
		return attempt.Result, PolicyEvaluation{}, nil
	}
	if err := s.writeSubmissionProvenance(attemptArtifactRoot, step); err != nil {
		reason := fmt.Sprintf("Failed to persist submission provenance: %v", err)
		attempt.Result = s.newResultSpec(runID, step, attempt, domain.StepStateFailedBridge, reason)
		step.StatusReason = reason
		_, _ = s.persistProvenanceArtifacts(ctx, attempt, attemptArtifactRoot)
		s.updateAttemptResult(ctx, attempt)
		return attempt.Result, PolicyEvaluation{}, nil
	}

	// Acquire exclusive lock for this run's workspace
	lock, err := workspace.AcquireLock(s.workspaceRoot, runID)
	if err != nil {
		attempt.Result = s.newResultSpec(runID, step, attempt, domain.StepStateFailedRetryable, "Workspace lock conflict: "+err.Error())
		_, _ = s.persistProvenanceArtifacts(ctx, attempt, attemptArtifactRoot)
		s.updateAttemptResult(ctx, attempt)
		return attempt.Result, PolicyEvaluation{}, nil
	}
	defer func() {
		_ = lock.Release()
	}()

	if err := workspace.CreateWorktree(ctx, baseRepo, workspaceRoot, branchName); err != nil {
		slog.Error("Failed to create worktree", "runID", runID, "error", err)
		reason := fmt.Sprintf("Workspace creation failed: %v", err)
		attempt.Result = s.newResultSpec(runID, step, attempt, domain.StepStateFailedBridge, reason)
		step.StatusReason = reason // Propagate to step state
		_, _ = s.persistProvenanceArtifacts(ctx, attempt, attemptArtifactRoot)
		s.updateAttemptResult(ctx, attempt)
		return attempt.Result, PolicyEvaluation{}, nil
	}
	defer func() {
		// Ensure cleanup happens even if subsequent steps fail
		_ = workspace.RemoveWorktree(context.Background(), baseRepo, workspaceRoot)
	}()

	// 2. Provision Workspace
	slog.Info("Provisioning workspace", "runID", runID, "workspace", workspaceRoot)

	// Load repo-local provisioning config
	spec, err := workspace.LoadWorkspaceConfig(baseRepo)
	if err != nil {
		slog.Warn("Failed to load provisioning config (skipping)", "error", err)
	}

	provRes, err := s.provisioner.Provision(ctx, spec, baseRepo, workspaceRoot)
	// provRes is attached to any ResultSpec created below.

	if err != nil {
		slog.Error("Failed to provision workspace", "runID", runID, "error", err)
		reason := fmt.Sprintf("Provisioning failed: %v", err)
		if attempt.Result == nil {
			attempt.Result = s.newResultSpec(runID, step, attempt, domain.StepStateFailedBridge, reason)
			attempt.Result.Provisioning = provRes
		} else {
			attempt.Result.State = domain.StepStateFailedBridge
			attempt.Result.Summary = reason
		}
		step.StatusReason = reason
		_, _ = s.persistProvenanceArtifacts(ctx, attempt, attemptArtifactRoot)
		s.updateAttemptResult(ctx, attempt)
		return attempt.Result, PolicyEvaluation{}, nil
	}

	// 3. Start Execution
	if err := adapter.Start(ctx, step, attempt, workspaceRoot, attemptArtifactRoot); err != nil {
		attempt.Result = s.newResultSpec(runID, step, attempt, domain.StepStateFailedRetryable, "Adapter failed to start: "+err.Error())
		attempt.Result.Provisioning = provRes
		_, _ = s.persistProvenanceArtifacts(ctx, attempt, attemptArtifactRoot)
		s.updateAttemptResult(ctx, attempt)
		return attempt.Result, PolicyEvaluation{}, nil
	}

	// 3. Poll
	if !s.pollAdapter(ctx, runID, adapter, attempt, s.attemptsRepo, step, provRes) {
		persistCtx := ctx
		if ctx.Err() != nil {
			persistCtx = context.Background()
		}
		_, _ = s.persistProvenanceArtifacts(persistCtx, attempt, attemptArtifactRoot)
		s.updateAttemptResult(persistCtx, attempt)
		return attempt.Result, PolicyEvaluation{}, nil
	}

	step.State = domain.StepStateCollectingArtifacts
	step.UpdatedAt = time.Now().UTC()
	_ = s.stepsRepo.UpdateState(ctx, step)

	// 4. Collect & Finalize
	artifacts, err := adapter.CollectArtifacts(ctx, attempt.ID, attemptArtifactRoot)
	if err != nil {
		reason := "Failed to collect artifacts: " + err.Error()
		attempt.Result = s.newResultSpec(runID, step, attempt, domain.StepStateFailedBridge, reason)
		attempt.Result.Provisioning = provRes
		step.StatusReason = reason
		_, _ = s.persistProvenanceArtifacts(ctx, attempt, attemptArtifactRoot)
		s.updateAttemptResult(ctx, attempt)
		return attempt.Result, PolicyEvaluation{}, nil
	}

	// Persist artifacts
	for _, art := range artifacts {
		_ = s.artifactsRepo.Create(ctx, art)
	}

	res, err := adapter.NormalizeResult(ctx, attempt.ID, artifacts)
	if err != nil {
		reason := "Normalization failed: " + err.Error()
		attempt.Result = s.newResultSpec(runID, step, attempt, domain.StepStateFailedBridge, reason)
		attempt.Result.Provisioning = provRes
		step.StatusReason = reason
	} else {
		res.Provisioning = provRes
		attempt.Result = res
		if res.State == domain.StepStateFailedTerminal || res.State == domain.StepStateFailedAdapter {
			step.StatusReason = res.Summary
		}
	}

	// Always persist provenance as artifacts on the terminal path
	_, _ = s.persistProvenanceArtifacts(ctx, attempt, attemptArtifactRoot)

	s.ensureResultEnvelope(attempt.Result, runID, step, attempt)

	s.updateAttemptResult(ctx, attempt)

	// 4.5 Run Validations (3B)
	if len(step.Validations) > 0 {
		step.State = domain.StepStateValidating
		s.stepsRepo.UpdateState(ctx, step)

		passed := 0
		failed := 0
		anyFailed := false
		for _, cmd := range step.Validations {
			vres, verr := s.validationRunner.Run(ctx, cmd, workspaceRoot)
			if verr != nil {
				slog.Error("Validation runner system error", "error", verr, "cmd", cmd.Name)
			}
			if vres != nil {
				if !vres.Passed {
					failed++
					anyFailed = true
				} else {
					passed++
				}
				_ = s.validationsRepo.Create(ctx, attempt.ID, vres)
			}
		}

		if anyFailed {
			attempt.Result.State = domain.StepStateFailedValidation
			attempt.Result.Summary = fmt.Sprintf("Validation failed: %d passed, %d failed.", passed, failed)
			s.ensureResultEnvelope(attempt.Result, runID, step, attempt)
			s.updateAttemptResult(ctx, attempt)
		} else {
			// Even if passing, it's useful to know validations ran
			attempt.Result.Summary += fmt.Sprintf(" (Validations: %d passed)", passed)
			s.ensureResultEnvelope(attempt.Result, runID, step, attempt)
			s.updateAttemptResult(ctx, attempt)
		}
	}

	// 5. Evaluate Policy
	var changedFiles []string
	if os.Getenv("FORCE_GATE_FOR_TESTING") == "1" {
		changedFiles = append(changedFiles, "migrations/fake.sql")
	} else {
		if files, err := workspace.CaptureChangedFiles(ctx, workspaceRoot); err == nil {
			changedFiles = files
		}
	}
	attempt.Result.FilesChanged = append([]string(nil), changedFiles...)
	s.ensureResultEnvelope(attempt.Result, runID, step, attempt)

	eval := Evaluate(policy, attempt.Result, changedFiles)
	return attempt.Result, eval, nil
}

func (s *RunService) updateAttemptResult(ctx context.Context, attempt *domain.Attempt) {
	if attempt.Result == nil {
		return
	}
	attempt.UpdatedAt = time.Now().UTC()
	if err := s.attemptsRepo.UpdateResult(ctx, attempt); err != nil {
		slog.Error("Failed to persist attempt result", "attemptID", attempt.ID, "error", err)
	}
}

func (s *RunService) writeSubmissionProvenance(attemptArtifactRoot string, step *domain.Step) error {
	normalizedTask := s.snapshotTaskSpec(step)
	normalizedBytes, err := json.MarshalIndent(normalizedTask, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal normalized task: %w", err)
	}
	if err := os.WriteFile(filepath.Join(attemptArtifactRoot, "normalized-task.json"), normalizedBytes, 0644); err != nil {
		return fmt.Errorf("write normalized-task.json: %w", err)
	}

	if step.SubmissionProvenance == nil || step.SubmissionProvenance.OriginalInput == "" {
		return nil
	}

	ext := sanitizeSubmissionExtension(step.SubmissionProvenance.OriginalFormat)
	originalPath := filepath.Join(attemptArtifactRoot, "original-input."+ext)
	if err := os.WriteFile(originalPath, []byte(step.SubmissionProvenance.OriginalInput), 0644); err != nil {
		return fmt.Errorf("write original input: %w", err)
	}
	return nil
}

func (s *RunService) snapshotTaskSpec(step *domain.Step) *domain.TaskSpec {
	if step.TaskSpecSnapshot != nil {
		snapshot := *step.TaskSpecSnapshot
		return &snapshot
	}

	return &domain.TaskSpec{
		RunID:                "",
		PhaseID:              step.PhaseID,
		StepID:               step.ID,
		Title:                step.Title,
		Goal:                 step.Goal,
		PolicyBundle:         step.Policy,
		AdapterProfile:       step.Adapter,
		TimeoutSeconds:       step.TimeoutSeconds,
		Validations:          append([]domain.ValidationCommand(nil), step.Validations...),
		SubmissionProvenance: step.SubmissionProvenance,
	}
}

func sanitizeSubmissionExtension(format string) string {
	switch strings.ToLower(strings.TrimPrefix(format, ".")) {
	case "yaml", "yml":
		return "yaml"
	case "json":
		return "json"
	case "md":
		return "md"
	case "txt":
		return "txt"
	default:
		return "txt"
	}
}

func (s *RunService) persistProvenanceArtifacts(ctx context.Context, attempt *domain.Attempt, attemptArtifactRoot string) ([]*domain.Artifact, error) {
	filenames := []string{"normalized-task.json"}
	if originalName, ok := findOriginalInputName(attemptArtifactRoot); ok {
		filenames = append(filenames, originalName)
	}

	artifacts := make([]*domain.Artifact, 0, len(filenames))
	for _, name := range filenames {
		artifact, err := buildSubmissionArtifact(attempt.ID, filepath.Join(attemptArtifactRoot, name))
		if err != nil {
			continue
		}
		if err := s.artifactsRepo.Create(ctx, artifact); err != nil {
			slog.Warn("Failed to persist provenance artifact", "attemptID", attempt.ID, "name", artifact.Name, "error", err)
			continue
		}
		artifacts = append(artifacts, artifact)
	}
	s.attachSubmissionArtifactRefs(attempt.Result, artifacts)
	return artifacts, nil
}

func buildSubmissionArtifact(attemptID, path string) (*domain.Artifact, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	sample := make([]byte, 512)
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	n, _ := file.Read(sample)
	if _, err := file.Seek(0, 0); err != nil {
		return nil, err
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return nil, err
	}
	artifactType := domain.ArtifactType("file")
	if filepath.Base(path) == "normalized-task.json" {
		artifactType = domain.ArtifactTypeInputJSON
	}

	return &domain.Artifact{
		ID:        fmt.Sprintf("art-%s-%s", attemptID, strings.ReplaceAll(filepath.Base(path), ".", "-")),
		AttemptID: attemptID,
		Type:      artifactType,
		Name:      filepath.Base(path),
		Path:      path,
		Size:      info.Size(),
		Hash:      hex.EncodeToString(hasher.Sum(nil)),
		MimeType:  http.DetectContentType(sample[:n]),
		CreatedAt: info.ModTime(),
		UpdatedAt: time.Now().UTC(),
	}, nil
}

func findOriginalInputName(attemptArtifactRoot string) (string, bool) {
	entries, err := os.ReadDir(attemptArtifactRoot)
	if err != nil {
		return "", false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), "original-input.") {
			return entry.Name(), true
		}
	}
	return "", false
}

func (s *RunService) attachSubmissionArtifactRefs(result *domain.ResultSpec, artifacts []*domain.Artifact) {
	if result == nil {
		return
	}
	if result.Artifacts == nil {
		result.Artifacts = make(map[string]string)
	}
	for _, artifact := range artifacts {
		switch {
		case artifact.Name == "normalized-task.json":
			result.Artifacts["normalized_task_ref"] = artifact.Path
		case strings.HasPrefix(artifact.Name, "original-input."):
			result.Artifacts["original_input_ref"] = artifact.Path
		}
	}
}

func (s *RunService) finalizeStep(
	ctx context.Context,
	runID string,
	step *domain.Step,
	finalResult *domain.ResultSpec,
	eval PolicyEvaluation,
	attemptID string,
) error {
	if s.isAbortRequested(runID) {
		if finalResult == nil {
			step.State = domain.StepStateCancelled
			step.StatusReason = "Execution cancelled before adapter start."
		} else {
			step.State = finalResult.State
			step.StatusReason = finalResult.Summary
		}
	} else if eval.ShouldGate {
		step.State = domain.StepStateNeedsApproval
		gate := &domain.Gate{
			ID:          "gate-" + attemptID,
			RunID:       runID,
			StepID:      step.ID,
			Description: "Policy enforced gate: " + strings.Join(eval.GateReasons, ", "),
			State:       domain.GateStatePending,
			CreatedAt:   time.Now().UTC(),
		}

		if gerr := s.gatesRepo.Create(ctx, gate); gerr != nil {
			slog.Error("Failed to create gate", "error", gerr)
			step.State = domain.StepStateFailedBridge
			step.StatusReason = "Infrastructure error: failed to create gate"
		}
	} else if eval.ShouldFail {
		step.State = domain.StepStateFailedValidation
		step.StatusReason = "Policy enforced failure: " + strings.Join(eval.FailReasons, ", ")
	} else if finalResult != nil && (finalResult.State != domain.StepStateCompleted && finalResult.State != domain.StepStateCompletedWithWarnings) {
		// Preserve granular failure states if reported by the adapter/system
		switch finalResult.State {
		case domain.StepStateFailedBridge, domain.StepStateFailedValidation, domain.StepStateTimeout, domain.StepStateNeedsManualAttention, domain.StepStateFailedTerminal, domain.StepStateFailedAdapter:
			step.State = finalResult.State
		default:
			step.State = domain.StepStateFailedAdapter
		}
		step.StatusReason = finalResult.Summary
	} else {
		step.State = domain.StepStateCompleted
		if finalResult != nil {
			step.StatusReason = finalResult.Summary
		}
		if finalResult != nil && finalResult.State == domain.StepStateCompletedWithWarnings {
			step.State = domain.StepStateCompletedWithWarnings
			step.StatusReason = finalResult.Summary
		}
	}

	step.UpdatedAt = time.Now().UTC()
	return s.stepsRepo.UpdateState(ctx, step)
}
func (s *RunService) pollAdapter(ctx context.Context, runID string, adapter domain.Adapter, attempt *domain.Attempt, repo *sqlite.AttemptsRepo, step *domain.Step, prov *domain.ProvisioningResult) bool {
	interval := 2 * time.Second
	pollTicker := time.NewTicker(interval)
	defer pollTicker.Stop()

	var timeoutChan <-chan time.Time
	if step != nil && step.TimeoutSeconds > 0 {
		timer := time.NewTimer(time.Duration(step.TimeoutSeconds) * time.Second)
		defer timer.Stop()
		timeoutChan = timer.C
	}

	for {
		select {
		case <-ctx.Done():
			_ = adapter.Cancel(context.Background(), attempt.ID)
			if s.waitForAdapterStop(adapter, attempt.ID, abortGracePeriod) {
				attempt.Result = s.newResultSpec(runID, step, attempt, domain.StepStateCancelled, "Execution cancelled by operator request.")
			} else {
				attempt.Result = s.newResultSpec(runID, step, attempt, domain.StepStateNeedsManualAttention, "Abort requested, but the adapter did not confirm cancellation before the grace period elapsed.")
			}
			attempt.Result.Provisioning = prov
			return false
		case <-timeoutChan:
			slog.Warn("Attempt timed out", "attemptID", attempt.ID, "timeoutSeconds", step.TimeoutSeconds)
			_ = adapter.Cancel(context.Background(), attempt.ID)
			attempt.Result = s.newResultSpec(runID, step, attempt, domain.StepStateTimeout, fmt.Sprintf("Execution timed out after %d seconds", step.TimeoutSeconds))
			attempt.Result.Provisioning = prov
			_ = repo.UpdateResult(context.Background(), attempt)
			return false
		case <-pollTicker.C:
			running, err := adapter.Poll(ctx, attempt.ID)
			if err != nil {
				attempt.Result = s.newResultSpec(runID, step, attempt, domain.StepStateFailedAdapter, fmt.Sprintf("Poll error: %v", err))
				attempt.Result.Provisioning = prov
				_ = repo.UpdateResult(context.Background(), attempt)
				return false
			}
			if !running {
				return true
			}
		}
	}
}

func (s *RunService) registerExecution(runID, stepID string, cancel context.CancelFunc, done chan struct{}) {
	s.execMu.Lock()
	defer s.execMu.Unlock()
	s.executions[runID] = &activeExecution{
		stepID: stepID,
		cancel: cancel,
		done:   done,
	}
}

func (s *RunService) setExecutionAttempt(runID, attemptID string, adapter domain.Adapter) {
	s.execMu.Lock()
	defer s.execMu.Unlock()
	if execution, ok := s.executions[runID]; ok {
		execution.attemptID = attemptID
		execution.adapter = adapter
	}
}

func (s *RunService) clearExecution(runID string) {
	s.execMu.Lock()
	defer s.execMu.Unlock()
	delete(s.executions, runID)
}

func (s *RunService) getExecution(runID string) *activeExecution {
	s.execMu.Lock()
	defer s.execMu.Unlock()
	execution, ok := s.executions[runID]
	if !ok {
		return nil
	}
	return execution
}

func (s *RunService) requestAbort(runID string) {
	s.execMu.Lock()
	defer s.execMu.Unlock()
	if execution, ok := s.executions[runID]; ok {
		execution.abortRequested = true
	}
}

func (s *RunService) isAbortRequested(runID string) bool {
	s.execMu.Lock()
	defer s.execMu.Unlock()
	if execution, ok := s.executions[runID]; ok {
		return execution.abortRequested
	}
	return false
}

func (s *RunService) waitForAdapterStop(adapter domain.Adapter, attemptID string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		running, err := adapter.Poll(context.Background(), attemptID)
		if err == nil && !running {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func (s *RunService) abortOutcome(ctx context.Context, runID, stepID string) error {
	outcomeCtx := ctx
	if outcomeCtx == nil || outcomeCtx.Err() != nil {
		outcomeCtx = context.Background()
	}
	step, err := s.stepsRepo.Get(outcomeCtx, stepID)
	if err != nil {
		return fmt.Errorf("abort requested for run %s, but the final step outcome could not be loaded: %w", runID, err)
	}
	if step == nil {
		return fmt.Errorf("abort requested for run %s, but step %s was not found after the execution stopped", runID, stepID)
	}
	if step.State == domain.StepStateCancelled {
		return nil
	}
	return fmt.Errorf("abort requested for run %s, but cancellation was not confirmed; step %s ended in state %s", runID, stepID, step.State)
}

func (s *RunService) ensureResultEnvelope(result *domain.ResultSpec, runID string, step *domain.Step, attempt *domain.Attempt) {
	if result == nil || step == nil {
		return
	}
	if result.Version == "" {
		result.Version = "v1"
	}
	if result.RunID == "" {
		result.RunID = runID
	}
	if result.PhaseID == "" {
		result.PhaseID = step.PhaseID
	}
	if result.StepID == "" {
		result.StepID = step.ID
	}
	if attempt != nil {
		if result.AttemptID == "" {
			result.AttemptID = attempt.ID
		}
		if result.Adapter == "" {
			result.Adapter = attempt.Adapter
		}
	}
	if result.RequestedAdapter == "" {
		result.RequestedAdapter = step.Adapter
	}
	if result.Summary == "" {
		result.Summary = string(result.State)
	}
	if result.CreatedAt.IsZero() {
		result.CreatedAt = time.Now().UTC()
	}
	result.UpdatedAt = time.Now().UTC()
}

func (s *RunService) newResultSpec(runID string, step *domain.Step, attempt *domain.Attempt, state domain.StepState, summary string) *domain.ResultSpec {
	result := &domain.ResultSpec{
		State:   state,
		Summary: summary,
	}
	s.ensureResultEnvelope(result, runID, step, attempt)
	return result
}

func (s *RunService) reconcileRunState(ctx context.Context, runID string) error {
	run, err := s.runsRepo.Get(ctx, runID)
	if err != nil {
		return err
	}
	if run == nil {
		return fmt.Errorf("run not found")
	}

	steps, err := s.stepsRepo.ListByRun(ctx, runID)
	if err != nil {
		return err
	}

	nextState := deriveRunState(steps)
	run.State = nextState
	run.UpdatedAt = time.Now().UTC()
	return s.runsRepo.UpdateState(ctx, run)
}

func (s *RunService) GetArtifactContent(ctx context.Context, artifactID string) (*domain.Artifact, []byte, error) {
	artifact, err := s.artifactsRepo.Get(ctx, artifactID)
	if err != nil {
		return nil, nil, err
	}
	if artifact == nil {
		return nil, nil, fmt.Errorf("artifact %s not found", artifactID)
	}
	artifactPath, err := s.resolveArtifactPath(artifact.Path)
	if err != nil {
		return nil, nil, err
	}
	content, err := os.ReadFile(artifactPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read artifact content: %w", err)
	}
	return artifact, content, nil
}

func (s *RunService) GetLogsByStep(ctx context.Context, stepID string) (*domain.Artifact, []byte, error) {
	attempt, err := s.attemptsRepo.GetLatestByStep(ctx, stepID)
	if err != nil {
		return nil, nil, err
	}
	if attempt == nil {
		return nil, nil, fmt.Errorf("no attempts found for step %s", stepID)
	}

	artifacts, err := s.artifactsRepo.ListByAttempt(ctx, attempt.ID)
	if err != nil {
		return nil, nil, err
	}

	var candidates []*domain.Artifact
	for _, artifact := range artifacts {
		if artifact.Type == domain.ArtifactTypeStdout || artifact.Type == domain.ArtifactTypeStderr {
			candidates = append(candidates, artifact)
		}
	}
	if len(candidates) == 0 {
		return nil, nil, fmt.Errorf("no log artifact found for step %s", stepID)
	}

	sort.Slice(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		if logArtifactPriority(left.Type) != logArtifactPriority(right.Type) {
			return logArtifactPriority(left.Type) < logArtifactPriority(right.Type)
		}
		if !left.CreatedAt.Equal(right.CreatedAt) {
			return left.CreatedAt.After(right.CreatedAt)
		}
		if !left.UpdatedAt.Equal(right.UpdatedAt) {
			return left.UpdatedAt.After(right.UpdatedAt)
		}
		return left.ID > right.ID
	})

	selected := candidates[0]
	return s.GetArtifactContent(ctx, selected.ID)
}

func logArtifactPriority(artifactType domain.ArtifactType) int {
	switch artifactType {
	case domain.ArtifactTypeStdout:
		return 0
	case domain.ArtifactTypeStderr:
		return 1
	default:
		return 2
	}
}

func (s *RunService) validateTaskSpecSnapshot(runID string, step *domain.Step) error {
	if step == nil {
		return fmt.Errorf("%w: step is required", ErrInvalidTaskSpec)
	}
	if step.ID == "" {
		return fmt.Errorf("%w: step id is required", ErrInvalidTaskSpec)
	}
	if step.TaskSpecSnapshot == nil {
		return nil
	}
	if step.TaskSpecSnapshot.RunID != "" && step.TaskSpecSnapshot.RunID != runID {
		return fmt.Errorf("%w: task run_id %q does not match target run %q", ErrInvalidTaskSpec, step.TaskSpecSnapshot.RunID, runID)
	}
	if step.TaskSpecSnapshot.PhaseID != "" && step.TaskSpecSnapshot.PhaseID != step.PhaseID {
		return fmt.Errorf("%w: task phase_id %q does not match target phase %q", ErrInvalidTaskSpec, step.TaskSpecSnapshot.PhaseID, step.PhaseID)
	}
	if step.TaskSpecSnapshot.StepID != "" && step.TaskSpecSnapshot.StepID != step.ID {
		return fmt.Errorf("%w: task step_id %q does not match target step %q", ErrInvalidTaskSpec, step.TaskSpecSnapshot.StepID, step.ID)
	}
	return nil
}

func (s *RunService) ensureExistingStepBelongsToRun(ctx context.Context, runID string, step *domain.Step) error {
	phase, err := s.phasesRepo.Get(ctx, step.PhaseID)
	if err != nil {
		return err
	}
	if phase == nil {
		return fmt.Errorf("%w: existing step %s references missing phase %s", ErrConflict, step.ID, step.PhaseID)
	}
	if phase.RunID != runID {
		return fmt.Errorf("%w: step %s already belongs to run %s", ErrConflict, step.ID, phase.RunID)
	}
	return nil
}

func (s *RunService) resolveArtifactPath(storedPath string) (string, error) {
	rootAbs, err := filepath.Abs(s.artifactRoot)
	if err != nil {
		return "", fmt.Errorf("resolve artifact root: %w", err)
	}
	rootClean := filepath.Clean(rootAbs)
	if rootResolved, err := filepath.EvalSymlinks(rootAbs); err == nil {
		rootClean = filepath.Clean(rootResolved)
	}

	candidate := storedPath
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(rootClean, candidate)
	}
	candidateAbs, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve artifact path: %w", err)
	}
	resolved := filepath.Clean(candidateAbs)
	if symlinkResolved, err := filepath.EvalSymlinks(candidateAbs); err == nil {
		resolved = filepath.Clean(symlinkResolved)
	}

	rel, err := filepath.Rel(rootClean, resolved)
	if err != nil {
		return "", fmt.Errorf("check artifact path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("%w: artifact path %q escapes artifact root", ErrArtifactAccessDenied, storedPath)
	}
	return resolved, nil
}

func isUniqueConstraint(err error) bool {
	var sqliteErr sqlite3driver.Error
	if errors.As(err, &sqliteErr) {
		return sqliteErr.Code == sqlite3driver.ErrConstraint ||
			sqliteErr.ExtendedCode == sqlite3driver.ErrConstraintPrimaryKey ||
			sqliteErr.ExtendedCode == sqlite3driver.ErrConstraintUnique
	}
	return false
}
