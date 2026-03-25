package claude

import (
	"context"
	"log/slog"
	"sync"

	"agent-bridge/internal/adapters/common"
	"agent-bridge/internal/domain"
)

// Adapter implements domain.Adapter for the Claude agent.
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
	return "claude"
}

func (a *Adapter) Capabilities() []string {
	return []string{"mcp_client", "planning"}
}

func (a *Adapter) Start(ctx context.Context, attempt *domain.Attempt, workspaceRoot, artifactRoot string) error {
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
			BinaryName:   "claude-code",
			BinaryEnvVar: "CLAUDE_BINARY",
			Args:         []string{"run", "--workspace", workspaceRoot, "--output", artifactRoot},
			Workspace:    workspaceRoot,
			ArtifactRoot: artifactRoot,
		}

		if err := common.InvokeLocal(execCtx, attempt, opts); err != nil {
			slog.Error("Claude Adapter: Execution failed", "attemptID", attempt.ID, "error", err)
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

func (a *Adapter) CollectArtifacts(ctx context.Context, attemptID string, artifactRoot string) ([]*domain.Artifact, error) {
	return common.CollectStandardArtifacts(ctx, attemptID, artifactRoot)
}

func (a *Adapter) NormalizeResult(ctx context.Context, attemptID string, artifacts []*domain.Artifact) (*domain.ResultSpec, error) {
	return common.NormalizeStandardResult(attemptID, artifacts)
}
