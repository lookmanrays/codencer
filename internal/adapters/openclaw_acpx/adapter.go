package openclaw_acpx

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"agent-bridge/internal/adapters/common"
	"agent-bridge/internal/domain"
)

const (
	AdapterID = "openclaw-acpx"
)

// Adapter implements the OpenClaw integration via the acpx (Agent Client Protocol) bridge.
type Adapter struct {
	binaryName string
	envVar     string
	processes  map[string]*context.CancelFunc
	mu         sync.Mutex
}

func NewAdapter() *Adapter {
	return &Adapter{
		binaryName: "acpx",
		envVar:     "OPENCLAW_ACPX_BINARY",
		processes:  make(map[string]*context.CancelFunc),
	}
}

func (a *Adapter) Name() string {
	return AdapterID
}

func (a *Adapter) Capabilities() []string {
	return []string{
		"persistent_session",
		"structured_result",
		"acp_compliance",
	}
}

func (a *Adapter) Start(ctx context.Context, step *domain.Step, attempt *domain.Attempt, workspaceRoot, attemptArtifactRoot string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.processes[attempt.ID]; exists {
		return nil // Idempotent start
	}

	// 1. Check for Simulation Mode (Sim is always ready)
	isSim := common.IsSimulationEnabled(a.Name())

	// Fail fast if binary is missing and NOT in simulation mode
	if !isSim {
		binary := os.Getenv(a.envVar)
		if binary == "" {
			binary = a.binaryName
		}
		if _, err := exec.LookPath(binary); err != nil {
			return fmt.Errorf("openclaw acpx binary %q not found. Please install it or set %s to a valid path, or enable simulation with %s_SIMULATION_MODE=1", binary, a.envVar, a.Name())
		}
	}

	execCtx, cancel := context.WithCancel(context.Background())
	a.processes[attempt.ID] = &cancel

	// 2. Execution wrapper
	go func() {
		defer cancel()
		var err error

		if isSim {
			err = common.RunSimulation(execCtx, step, attempt, attemptArtifactRoot, workspaceRoot)
		} else {
			// acpx prompt --session <attemptID> --wait-for-completion "<goal>"
			// We ensure we anchor to the workspaceRoot implicitly via common.InvokeLocal's Dir setting.
			args := []string{
				"prompt",
				"--session", attempt.ID,
				"--wait-for-completion",
				step.Goal,
			}
			opts := common.ExecutionOptions{
				AdapterName:  a.Name(),
				BinaryName:   a.binaryName,
				BinaryEnvVar: a.envVar,
				Args:         args,
				Workspace:    workspaceRoot,
				ArtifactRoot: attemptArtifactRoot,
			}
			err = common.InvokeLocal(execCtx, step, attempt, opts)
		}

		if err != nil {
			slog.Error("OpenClaw Execution failed", "error", err, "attemptID", attempt.ID)
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

	// 1. Check in-memory process map (Primary for local session)
	_, running := a.processes[attemptID]
	if running {
		return true, nil
	}

	// 2. Proactive check via acpx status if not in sim mode
	// This ensures we can recover state if the daemon restarts/crashes.
	if !common.IsSimulationEnabled(a.Name()) {
		binary := os.Getenv(a.envVar)
		if binary == "" {
			binary = a.binaryName
		}

		// Check 'acpx status --session <id> --json'
		statusCtx, statusCancel := context.WithTimeout(ctx, 3*time.Second)
		defer statusCancel()

		cmd := exec.CommandContext(statusCtx, binary, "status", "--session", attemptID, "--json")
		output, err := cmd.Output()
		if err == nil {
			type acpStatus struct {
				Running bool   `json:"running"`
				Status  string `json:"status"`
			}
			var s acpStatus
			if json.Unmarshal(output, &s) == nil {
				// If status is terminal (success/failed), it's definitely not running anymore.
				if s.Status == "success" || s.Status == "failed" || s.Status == "cancelled" {
					return false, nil
				}
				return s.Running, nil
			}
		}
	}

	return false, nil
}

func (a *Adapter) Cancel(ctx context.Context, attemptID string) error {
	a.mu.Lock()
	cancelFunc, exists := a.processes[attemptID]
	a.mu.Unlock()

	// 1. Context cancellation triggers local process SIGTERM (if we own it)
	if exists {
		(*cancelFunc)()
	}

	// 2. Best-effort explicit ACP stop via acpx CLI
	if !common.IsSimulationEnabled(a.Name()) {
		binary := os.Getenv(a.envVar)
		if binary == "" {
			binary = a.binaryName
		}
		
		// Run acpx stop with short timeout
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()
		
		cmd := exec.CommandContext(stopCtx, binary, "stop", "--session", attemptID)
		if err := cmd.Run(); err != nil {
			slog.Warn("OpenClaw: Explicit acpx stop failed", "error", err, "attemptID", attemptID)
		} else {
			slog.Info("OpenClaw: Explicit acpx stop sent", "attemptID", attemptID)
		}
	}

	a.mu.Lock()
	delete(a.processes, attemptID)
	a.mu.Unlock()

	slog.Info("OpenClaw Execution: Cancellation processed", "attemptID", attemptID)
	return nil
}

func (a *Adapter) CollectArtifacts(ctx context.Context, attemptID string, attemptArtifactRoot string) ([]*domain.Artifact, error) {
	// 1. Gather files from standard artifact root (stdout.log etc)
	artifacts, err := common.CollectStandardArtifacts(ctx, attemptID, attemptArtifactRoot)
	if err != nil {
		return nil, err
	}

	// 2. Proactive artifact harvesting from workspace-anchored session directories.
	// Standard ACP evidence often lives in .acp/sessions/<attemptID>/
	// Future: search for common ACP names directly in the attempt artifact root 
	// because acpx is often called with redirection there by common.InvokeLocal.
	evidenceNames := []string{"acp-status.json", "result.json", "session.log"}
	for _, name := range evidenceNames {
		path := filepath.Join(attemptArtifactRoot, name)
		if _, err := os.Stat(path); err == nil {
			artifacts = append(artifacts, &domain.Artifact{
				AttemptID: attemptID,
				Name:      name,
				Path:      path,
				Type:      domain.ArtifactTypeResultJSON,
				MimeType:  "application/json",
			})
		}
	}

	return artifacts, nil
}

func (a *Adapter) NormalizeResult(ctx context.Context, attemptID string, artifacts []*domain.Artifact) (*domain.ResultSpec, error) {
	isSimulation := common.IsSimulationEnabled(a.Name())
	return NormalizeResult(attemptID, artifacts, isSimulation)
}
