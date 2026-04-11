package claude

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

const (
	binaryName   = "claude"
	binaryEnvVar = "CLAUDE_BINARY"
)

type processState struct {
	cancel context.CancelFunc
	done   chan struct{}

	mu  sync.Mutex
	cmd *exec.Cmd
}

// Adapter implements domain.Adapter for the Claude agent.
type Adapter struct {
	mu        sync.Mutex
	processes map[string]*processState
}

func NewAdapter() *Adapter {
	return &Adapter{
		processes: make(map[string]*processState),
	}
}

func (a *Adapter) Name() string {
	return "claude"
}

func (a *Adapter) Capabilities() []string {
	return []string{"mcp_client", "planning"}
}

func (a *Adapter) Start(ctx context.Context, step *domain.Step, attempt *domain.Attempt, workspaceRoot, attemptArtifactRoot string) error {
	a.mu.Lock()
	if _, exists := a.processes[attempt.ID]; exists {
		a.mu.Unlock()
		return nil
	}

	state := &processState{done: make(chan struct{})}
	a.processes[attempt.ID] = state
	a.mu.Unlock()

	isSimulation := common.IsSimulationEnabled(a.Name())
	if !isSimulation {
		if _, err := resolveBinary(); err != nil {
			a.mu.Lock()
			delete(a.processes, attempt.ID)
			a.mu.Unlock()
			return err
		}
	}

	execCtx, cancel := context.WithCancel(context.Background())
	state.cancel = cancel

	go func() {
		defer close(state.done)
		defer cancel()

		var err error
		if isSimulation {
			err = common.RunSimulation(execCtx, step, attempt, attemptArtifactRoot, workspaceRoot)
		} else {
			err = runAttempt(execCtx, state, step, attempt, workspaceRoot, attemptArtifactRoot)
		}

		if err != nil {
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
	state, exists := a.processes[attemptID]
	a.mu.Unlock()
	if !exists {
		return false, nil
	}

	select {
	case <-state.done:
		return false, nil
	default:
		return true, nil
	}
}

func (a *Adapter) Cancel(ctx context.Context, attemptID string) error {
	a.mu.Lock()
	state, exists := a.processes[attemptID]
	a.mu.Unlock()
	if !exists {
		return nil
	}

	if state.cancel != nil {
		state.cancel()
	}
	if err := stopProcess(state); err != nil {
		return err
	}
	return nil
}

func (a *Adapter) CollectArtifacts(ctx context.Context, attemptID string, attemptArtifactRoot string) ([]*domain.Artifact, error) {
	return common.CollectStandardArtifacts(ctx, attemptID, attemptArtifactRoot)
}

func (a *Adapter) NormalizeResult(ctx context.Context, attemptID string, artifacts []*domain.Artifact) (*domain.ResultSpec, error) {
	isSimulation := common.IsSimulationEnabled(a.Name())
	return NormalizeCore(attemptID, artifacts, a.Name(), isSimulation)
}

func resolveBinary() (string, error) {
	binary := os.Getenv(binaryEnvVar)
	if binary == "" {
		binary = binaryName
	}

	binaryPath, err := exec.LookPath(binary)
	if err != nil {
		return "", fmt.Errorf("claude binary %q not found or not executable. Set %s to a valid path or enable simulation with %s_SIMULATION_MODE=1: %w", binary, binaryEnvVar, "CLAUDE", err)
	}

	return binaryPath, nil
}
