package qwen

import (
	"context"
	"log/slog"
	"sync"

	"agent-bridge/internal/adapters/common"
	"agent-bridge/internal/domain"
)

// Adapter implements domain.Adapter for the Qwen agent.
type Adapter struct {
	processes map[string]*context.CancelFunc
	mu        sync.Mutex
}

func NewAdapter() *Adapter {
	return &Adapter{
		processes: make(map[string]*context.CancelFunc),
	}
}

func (a *Adapter) Name() string {
	return "qwen"
}

func (a *Adapter) Capabilities() []string {
	return []string{"local_inference", "coding"}
}

func (a *Adapter) Start(ctx context.Context, step *domain.Step, attempt *domain.Attempt, workspaceRoot, attemptArtifactRoot string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.processes[attempt.ID]; exists {
		return nil
	}

	execCtx, cancel := context.WithCancel(context.Background())
	a.processes[attempt.ID] = &cancel

	go func() {
		defer cancel()
		opts := common.ExecutionOptions{
			AdapterName:  a.Name(),
			BinaryName:   "qwen-local",
			BinaryEnvVar: "QWEN_BINARY",
			Args:         []string{"run", "--workspace", workspaceRoot, "--output", attemptArtifactRoot},
			Workspace:    workspaceRoot,
			ArtifactRoot: attemptArtifactRoot,
		}

		if err := common.InvokeLocal(execCtx, step, attempt, opts); err != nil {
			slog.Error("Qwen Adapter: Execution failed", "attemptID", attempt.ID, "error", err)
		}

		a.mu.Lock()
		delete(a.processes, attempt.ID)
		a.mu.Unlock()
	}()

	return nil
}

func (a *Adapter) Poll(ctx context.Context, attemptID string) (bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, running := a.processes[attemptID]
	return running, nil
}

func (a *Adapter) Cancel(ctx context.Context, attemptID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	cancelFunc, exists := a.processes[attemptID]
	if !exists {
		return nil
	}

	(*cancelFunc)()
	delete(a.processes, attemptID)
	return nil
}

func (a *Adapter) CollectArtifacts(ctx context.Context, attemptID string, attemptArtifactRoot string) ([]*domain.Artifact, error) {
	return common.CollectStandardArtifacts(ctx, attemptID, attemptArtifactRoot)
}

func (a *Adapter) NormalizeResult(ctx context.Context, attemptID string, artifacts []*domain.Artifact) (*domain.ResultSpec, error) {
	return common.NormalizeStandardResult(attemptID, artifacts)
}
