package fake

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"agent-bridge/internal/adapters/common"
	"agent-bridge/internal/domain"
)

const (
	Success = "fake-success"
	Failure = "fake-failure"
	Blocker = "fake-blocker"
	Timeout = "fake-timeout"
)

// Adapter is a deterministic adapter used only for local execution verification.
// It still runs through the daemon's normal run/step/attempt lifecycle.
type Adapter struct {
	id string

	mu      sync.Mutex
	running map[string]bool
}

func New(id string) *Adapter {
	return &Adapter{
		id:      id,
		running: map[string]bool{},
	}
}

func (a *Adapter) Name() string { return a.id }

func (a *Adapter) Capabilities() []string {
	return []string{"local-verification", "deterministic"}
}

func (a *Adapter) Start(ctx context.Context, step *domain.Step, attempt *domain.Attempt, workspaceRoot, attemptArtifactRoot string) error {
	if err := os.MkdirAll(attemptArtifactRoot, 0755); err != nil {
		return fmt.Errorf("create fake artifact root: %w", err)
	}

	a.setRunning(attempt.ID, true)
	if a.id == Timeout {
		return nil
	}

	state, summary := a.resultState()
	result := domain.ResultSpec{
		Version:            "v1",
		RunID:              stepRunID(step),
		PhaseID:            step.PhaseID,
		StepID:             step.ID,
		AttemptID:          attempt.ID,
		Adapter:            a.id,
		RequestedAdapter:   step.Adapter,
		State:              state,
		Summary:            summary,
		NeedsHumanDecision: a.id == Blocker,
		IsSimulation:       true,
		Artifacts:          map[string]string{},
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
	}
	if a.id == Blocker {
		result.Questions = []string{"Fake blocker requires a planner decision."}
	}

	if err := writeFakeArtifacts(attemptArtifactRoot, a.id, result); err != nil {
		a.setRunning(attempt.ID, false)
		return err
	}
	a.setRunning(attempt.ID, false)
	return nil
}

func (a *Adapter) Poll(ctx context.Context, attemptID string) (bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.running[attemptID], nil
}

func (a *Adapter) Cancel(ctx context.Context, attemptID string) error {
	a.setRunning(attemptID, false)
	return nil
}

func (a *Adapter) CollectArtifacts(ctx context.Context, attemptID string, attemptArtifactRoot string) ([]*domain.Artifact, error) {
	return common.CollectStandardArtifacts(ctx, attemptID, attemptArtifactRoot)
}

func (a *Adapter) NormalizeResult(ctx context.Context, attemptID string, artifacts []*domain.Artifact) (*domain.ResultSpec, error) {
	result, err := common.NormalizeStandardResult(attemptID, artifacts)
	if err != nil {
		return nil, err
	}
	if result.Adapter == "" {
		result.Adapter = a.id
	}
	if result.AttemptID == "" {
		result.AttemptID = attemptID
	}
	result.IsSimulation = true
	result.UpdatedAt = time.Now().UTC()
	if result.CreatedAt.IsZero() {
		result.CreatedAt = result.UpdatedAt
	}
	return result, nil
}

func (a *Adapter) setRunning(attemptID string, running bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if running {
		a.running[attemptID] = true
		return
	}
	delete(a.running, attemptID)
}

func (a *Adapter) resultState() (domain.StepState, string) {
	switch a.id {
	case Success:
		return domain.StepStateCompleted, "Fake adapter completed successfully."
	case Failure:
		return domain.StepStateFailedTerminal, "Fake adapter reported a terminal failure."
	case Blocker:
		return domain.StepStateNeedsManualAttention, "Fake adapter requires planner attention."
	default:
		return domain.StepStateFailedAdapter, "Unknown fake adapter mode."
	}
}

func writeFakeArtifacts(root, adapterID string, result domain.ResultSpec) error {
	stdout := fmt.Sprintf("%s executed at %s\n", adapterID, time.Now().UTC().Format(time.RFC3339))
	if err := os.WriteFile(filepath.Join(root, "stdout.log"), []byte(stdout), 0644); err != nil {
		return fmt.Errorf("write fake stdout: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("encode fake result: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(root, "result.json"), data, 0644); err != nil {
		return fmt.Errorf("write fake result: %w", err)
	}
	return nil
}

func stepRunID(step *domain.Step) string {
	if step != nil && step.TaskSpecSnapshot != nil {
		return step.TaskSpecSnapshot.RunID
	}
	return ""
}
