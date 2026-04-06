package codex

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sync"

	"agent-bridge/internal/adapters/common"
	"agent-bridge/internal/domain"
)

// Adapter implements domain.Adapter for the local Codex agent.
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
	return "codex"
}

func (a *Adapter) Capabilities() []string {
	return []string{"local_cli", "filesystem_read", "filesystem_write"}
}

func (a *Adapter) Start(ctx context.Context, step *domain.Step, attempt *domain.Attempt, workspaceRoot, attemptArtifactRoot string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.processes[attempt.ID]; exists {
		return nil // Already running or just idempotent
	}

	// Fail fast if binary is missing and not in simulation mode
	if !common.IsSimulationEnabled(a.Name()) {
		binary := os.Getenv("CODEX_BINARY")
		if binary == "" {
			binary = "codex-agent"
		}
		if _, err := exec.LookPath(binary); err != nil {
			return fmt.Errorf("codex binary %q not found. Please install it or set CODEX_BINARY to a valid path, or enable simulation with CODEX_SIMULATION_MODE=1", binary)
		}
	}

	execCtx, cancel := context.WithCancel(context.Background())
	a.processes[attempt.ID] = &cancel

	go func() {
		defer cancel()
		opts := common.ExecutionOptions{
			AdapterName:  a.Name(),
			BinaryName:   "codex-agent",
			BinaryEnvVar: "CODEX_BINARY",
			Args:         []string{"run", "--workspace", workspaceRoot, "--output", attemptArtifactRoot, "--title", step.Title, "--goal", step.Goal},
			Workspace:    workspaceRoot,
			ArtifactRoot: attemptArtifactRoot,
		}

		if err := common.InvokeLocal(execCtx, step, attempt, opts); err != nil {
			slog.Error("Codex Adapter: Execution failed", "attemptID", attempt.ID, "error", err)
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
	// NormalizeCore now handles metadata enrichment and artifact linking
	isSimulation := common.IsSimulationEnabled(a.Name())
	return NormalizeCore(attemptID, artifacts, a.Name(), isSimulation)
}
