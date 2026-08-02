package opencode

import (
	"context"
	"encoding/json"
	"fmt"
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
	AdapterID     = "opencode"
	binaryName    = "opencode"
	binaryEnvVar  = "OPENCODE_BINARY"
	resultFile    = "result.json"
	stdoutLogFile = "stdout.log"
	stderrLogFile = "stderr.log"
)

type processState struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// Adapter implements domain.Adapter for the OpenCode non-interactive CLI.
type Adapter struct {
	mu        sync.Mutex
	processes map[string]*processState
}

func NewAdapter() *Adapter {
	return &Adapter{processes: make(map[string]*processState)}
}

func (a *Adapter) Name() string { return AdapterID }

func (a *Adapter) Capabilities() []string {
	return []string{"local_cli", "filesystem_read", "filesystem_write", "structured_events"}
}

func (a *Adapter) Start(ctx context.Context, step *domain.Step, attempt *domain.Attempt, workspaceRoot, attemptArtifactRoot string) error {
	if attempt == nil || strings.TrimSpace(attempt.ID) == "" {
		return fmt.Errorf("opencode requires an attempt id")
	}

	a.mu.Lock()
	if _, exists := a.processes[attempt.ID]; exists {
		a.mu.Unlock()
		return nil
	}
	if !common.IsSimulationEnabled(a.Name()) {
		if _, err := resolveBinary(); err != nil {
			a.mu.Unlock()
			return err
		}
	}
	state := &processState{done: make(chan struct{})}
	execCtx, cancel := context.WithCancel(context.Background())
	state.cancel = cancel
	a.processes[attempt.ID] = state
	a.mu.Unlock()

	go func() {
		defer close(state.done)
		defer cancel()
		defer func() {
			a.mu.Lock()
			delete(a.processes, attempt.ID)
			a.mu.Unlock()
		}()

		if common.IsSimulationEnabled(a.Name()) {
			_ = common.RunSimulation(execCtx, step, attempt, attemptArtifactRoot, workspaceRoot)
			return
		}
		_ = runAttempt(execCtx, step, attempt, workspaceRoot, attemptArtifactRoot)
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
	state.cancel()
	return nil
}

func (a *Adapter) CollectArtifacts(ctx context.Context, attemptID, attemptArtifactRoot string) ([]*domain.Artifact, error) {
	return common.CollectStandardArtifacts(ctx, attemptID, attemptArtifactRoot)
}

func (a *Adapter) NormalizeResult(ctx context.Context, attemptID string, artifacts []*domain.Artifact) (*domain.ResultSpec, error) {
	return normalizeResult(attemptID, artifacts, common.IsSimulationEnabled(a.Name()))
}

func resolveBinary() (string, error) {
	binary := strings.TrimSpace(os.Getenv(binaryEnvVar))
	if binary == "" {
		binary = binaryName
	}
	path, err := exec.LookPath(binary)
	if err != nil {
		return "", fmt.Errorf("opencode binary %q not found or not executable. Set %s to a valid path or enable simulation with OPENCODE_SIMULATION_MODE=1: %w", binary, binaryEnvVar, err)
	}
	return path, nil
}

func commandArgs(step *domain.Step, prompt string) []string {
	title := "Codencer task"
	if step != nil && strings.TrimSpace(step.Title) != "" {
		title = strings.TrimSpace(step.Title)
	}
	return []string{"run", "--format", "json", "--dangerously-skip-permissions", "--title", title, prompt}
}

func runAttempt(ctx context.Context, step *domain.Step, attempt *domain.Attempt, workspaceRoot, artifactRoot string) error {
	if err := os.MkdirAll(artifactRoot, 0o755); err != nil {
		return fmt.Errorf("create artifact root: %w", err)
	}
	prompt := buildPrompt(step, artifactRoot)
	if err := os.WriteFile(filepath.Join(artifactRoot, "prompt.txt"), []byte(prompt), 0o644); err != nil {
		return fmt.Errorf("write prompt artifact: %w", err)
	}

	stdout, err := os.Create(filepath.Join(artifactRoot, stdoutLogFile))
	if err != nil {
		return fmt.Errorf("create stdout log: %w", err)
	}
	defer stdout.Close()
	stderr, err := os.Create(filepath.Join(artifactRoot, stderrLogFile))
	if err != nil {
		return fmt.Errorf("create stderr log: %w", err)
	}
	defer stderr.Close()

	binary, err := resolveBinary()
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, binary, commandArgs(step, prompt)...)
	cmd.Dir = workspaceRoot
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	runErr := cmd.Run()

	result := resultFromOutput(attempt, filepath.Join(artifactRoot, stdoutLogFile), filepath.Join(artifactRoot, stderrLogFile), runErr, ctx.Err())
	if err := writeResult(filepath.Join(artifactRoot, resultFile), result); err != nil {
		return fmt.Errorf("write normalized result: %w", err)
	}
	return runErr
}

func writeResult(path string, result *domain.ResultSpec) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func resultFromOutput(attempt *domain.Attempt, stdoutPath, stderrPath string, runErr error, ctxErr error) *domain.ResultSpec {
	now := time.Now().UTC()
	result := &domain.ResultSpec{
		Version:      "v1",
		Adapter:      AdapterID,
		State:        domain.StepStateCompleted,
		Summary:      "OpenCode task completed successfully.",
		RawOutputRef: stdoutPath,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if attempt != nil {
		result.AttemptID = attempt.ID
		result.RequestedAdapter = attempt.Adapter
	}

	summary, eventErr := parseEvents(stdoutPath)
	if summary != "" {
		result.Summary = summary
	}
	if ctxErr == context.Canceled {
		result.State = domain.StepStateCancelled
		result.Summary = "OpenCode execution cancelled."
		return result
	}
	if ctxErr == context.DeadlineExceeded {
		result.State = domain.StepStateTimeout
		result.Summary = "OpenCode execution timed out."
		return result
	}
	if eventErr != "" || runErr != nil {
		result.State = domain.StepStateFailedAdapter
		result.Summary = firstNonEmpty(eventErr, stderrSummary(stderrPath), errorSummary(runErr), "OpenCode execution failed.")
	}
	return result
}

func errorSummary(err error) string {
	if err == nil {
		return ""
	}
	return "OpenCode process exited with error: " + err.Error()
}

func stderrSummary(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
