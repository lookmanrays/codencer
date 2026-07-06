package codex

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"agent-bridge/internal/adapters/common"
	"agent-bridge/internal/domain"
)

const (
	codexAdapterModeEnv = "CODEX_ADAPTER_MODE"
	codexExecMode       = "codex-exec"
	codexLegacyMode     = "legacy-agent"
	codexBinaryEnv      = "CODEX_BINARY"
	codexDefaultBinary  = "codex"
	codexLegacyBinary   = "codex-agent"
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

	opts := codexExecutionOptions(step, workspaceRoot, attemptArtifactRoot)

	// Fail fast if binary is missing and not in simulation mode.
	if !common.IsSimulationEnabled(a.Name()) {
		binary := os.Getenv(codexBinaryEnv)
		if binary == "" {
			binary = opts.BinaryName
		}
		if _, err := exec.LookPath(binary); err != nil {
			return fmt.Errorf("codex binary %q not found. Please install it or set CODEX_BINARY to a valid path, or enable simulation with CODEX_SIMULATION_MODE=1", binary)
		}
	}

	execCtx, cancel := context.WithCancel(context.Background())
	a.processes[attempt.ID] = &cancel

	go func() {
		defer cancel()
		if err := common.InvokeLocal(execCtx, step, attempt, opts); err != nil {
			slog.Error("Codex Adapter: Execution failed", "attemptID", attempt.ID, "error", err)
			writeCodexFallbackResult(step, attempt, attemptArtifactRoot, a.Name(), err)
		} else {
			writeCodexFallbackResult(step, attempt, attemptArtifactRoot, a.Name(), nil)
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

func codexExecutionOptions(step *domain.Step, workspaceRoot, attemptArtifactRoot string) common.ExecutionOptions {
	mode := strings.TrimSpace(os.Getenv(codexAdapterModeEnv))
	if mode == "" {
		mode = codexExecMode
	}
	if mode == codexLegacyMode {
		return common.ExecutionOptions{
			AdapterName:  "codex",
			BinaryName:   codexLegacyBinary,
			BinaryEnvVar: codexBinaryEnv,
			Args:         []string{"run", "--workspace", workspaceRoot, "--output", attemptArtifactRoot, "--title", step.Title, "--goal", step.Goal},
			Workspace:    workspaceRoot,
			ArtifactRoot: attemptArtifactRoot,
		}
	}

	return common.ExecutionOptions{
		AdapterName:  "codex",
		BinaryName:   codexDefaultBinary,
		BinaryEnvVar: codexBinaryEnv,
		Args: []string{
			"--ask-for-approval", "never",
			"exec",
			"--cd", workspaceRoot,
			"--sandbox", "workspace-write",
			"--output-last-message", filepath.Join(attemptArtifactRoot, "codex-last-message.txt"),
			"--color", "never",
			"-",
		},
		Workspace:    workspaceRoot,
		ArtifactRoot: attemptArtifactRoot,
		Stdin:        buildCodexPrompt(step, attemptArtifactRoot),
	}
}

func buildCodexPrompt(step *domain.Step, attemptArtifactRoot string) string {
	var b strings.Builder
	b.WriteString("You are the local Codex executor running under Codencer.\n\n")
	b.WriteString("Codencer is the bridge and source of record. Execute only the task below, write evidence under the artifact root, and do not choose follow-up tasks.\n\n")
	b.WriteString("Codencer runs TaskSpec validations after you return. Do not rerun broad validations unless the task explicitly asks you to; keep the executor pass narrow and report what you changed or inspected.\n\n")
	b.WriteString("Artifact root:\n")
	b.WriteString(attemptArtifactRoot)
	b.WriteString("\n\n")
	b.WriteString("When finished, write a JSON object to result.json in the artifact root. Use this shape:\n")
	b.WriteString(`{"version":"v1","state":"completed","summary":"short outcome","files_changed":[],"warnings":[],"questions":[]}`)
	b.WriteString("\n\n")
	if step != nil && strings.TrimSpace(step.Title) != "" {
		b.WriteString("Title:\n")
		b.WriteString(step.Title)
		b.WriteString("\n\n")
	}
	if step != nil && step.TaskSpecSnapshot != nil {
		if data, err := json.MarshalIndent(step.TaskSpecSnapshot, "", "  "); err == nil {
			b.WriteString("TaskSpec:\n")
			b.Write(data)
			b.WriteString("\n")
			return b.String()
		}
	}
	if step != nil {
		b.WriteString("Goal:\n")
		b.WriteString(step.Goal)
		b.WriteString("\n")
	}
	return b.String()
}

func writeCodexFallbackResult(step *domain.Step, attempt *domain.Attempt, attemptArtifactRoot, adapterName string, execErr error) {
	resultPath := filepath.Join(attemptArtifactRoot, "result.json")
	if _, err := os.Stat(resultPath); err == nil {
		return
	}
	_ = os.MkdirAll(attemptArtifactRoot, 0o755)

	state := domain.StepStateCompleted
	summary := "Codex execution completed. The adapter synthesized this result because Codex did not write result.json."
	if execErr != nil {
		state = domain.StepStateFailedAdapter
		summary = "Codex execution failed: " + execErr.Error()
	} else if text := readCodexLastMessage(attemptArtifactRoot); text != "" {
		summary = text
	} else {
		state = domain.StepStateFailedTerminal
		summary = "Bridge Interface Error: Codex exited successfully but did not write result.json or codex-last-message.txt."
	}

	now := time.Now().UTC()
	requestedAdapter := adapterName
	if step != nil && step.Adapter != "" {
		requestedAdapter = step.Adapter
	}
	attemptID := ""
	if attempt != nil {
		attemptID = attempt.ID
	}
	artifactRefs := map[string]string{}
	for name, path := range map[string]string{
		"codex_last_message_ref": filepath.Join(attemptArtifactRoot, "codex-last-message.txt"),
		"stdout_ref":             filepath.Join(attemptArtifactRoot, "stdout.log"),
	} {
		if _, err := os.Stat(path); err == nil {
			artifactRefs[name] = path
		}
	}

	result := domain.ResultSpec{
		Version:          "v1",
		AttemptID:        attemptID,
		Adapter:          adapterName,
		RequestedAdapter: requestedAdapter,
		State:            state,
		Summary:          summary,
		Artifacts:        artifactRefs,
		IsSimulation:     false,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if stdoutRef, ok := artifactRefs["stdout_ref"]; ok {
		result.RawOutputRef = stdoutRef
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(resultPath, data, 0o644)
}

func readCodexLastMessage(attemptArtifactRoot string) string {
	data, err := os.ReadFile(filepath.Join(attemptArtifactRoot, "codex-last-message.txt"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
