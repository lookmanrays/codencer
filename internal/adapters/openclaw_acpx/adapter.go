package openclaw_acpx

import (
	"context"
	"log/slog"

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
}

func NewAdapter() *Adapter {
	return &Adapter{
		binaryName: "acpx",
		envVar:     "OPENCLAW_ACPX_BINARY",
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
	// 1. Check for Simulation Mode
	if common.IsSimulationEnabled(a.Name()) {
		return common.RunSimulation(ctx, step, attempt, attemptArtifactRoot, workspaceRoot)
	}

	// 2. Prepare acpx command
	// Conceptual: acpx prompt --session <attemptID> --wait-for-completion "<goal>"
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

	// 3. Initial implementation uses InvokeLocal (background) for consistency with Codex/Claude
	go func() {
		if err := common.InvokeLocal(context.Background(), step, attempt, opts); err != nil {
			slog.Error("OpenClaw Execution failed", "error", err, "attemptID", attempt.ID)
		}
	}()

	return nil
}

func (a *Adapter) Poll(ctx context.Context, attemptID string) (bool, error) {
	// Initial implementation relies on process exit (InvokeLocal completes).
	// Future optimization: call 'acpx status --session <attemptID>'
	return false, nil
}

func (a *Adapter) Cancel(ctx context.Context, attemptID string) error {
	// Conceptual: acpx stop --session <attemptID>
	slog.Info("OpenClaw Execution: Cancellation requested", "attemptID", attemptID)
	return nil
}

func (a *Adapter) CollectArtifacts(ctx context.Context, attemptID string, attemptArtifactRoot string) ([]*domain.Artifact, error) {
	return common.CollectStandardArtifacts(ctx, attemptID, attemptArtifactRoot)
}

func (a *Adapter) NormalizeResult(ctx context.Context, attemptID string, artifacts []*domain.Artifact) (*domain.ResultSpec, error) {
	return common.NormalizeStandardResult(attemptID, artifacts)
}
