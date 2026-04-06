package ide

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"agent-bridge/internal/domain"
)

// Adapter implements a PROXY-MEDIATED boundary targeting an IDE chat window (e.g., VS Code + Continue/Copilot).
// It does NOT have native control over the IDE; it communicates via shared file buffers.
type Adapter struct {
	capabilities []string
}

func NewAdapter() *Adapter {
	return &Adapter{
		capabilities: []string{"ide-chat", "gui-automation", "manual-fallback"},
	}
}

func (a *Adapter) Name() string {
	return "ide-chat"
}

func (a *Adapter) Capabilities() []string {
	return a.capabilities
}

// Start performs a proxy-mediated handoff by writing the attempt payload to a shared file buffer. 
// A companion VS Code Extension must watch this file to facilitate ingestion into the active IDE chat.
func (a *Adapter) Start(ctx context.Context, step *domain.Step, attempt *domain.Attempt, workspaceRoot, attemptArtifactRoot string) error {
	slog.Info("IDE Adapter: Starting chat bridge handoff", "attemptID", attempt.ID)

	promptFile := filepath.Join(workspaceRoot, ".codencer", "chat_prompt.txt")
	if err := os.MkdirAll(filepath.Dir(promptFile), 0755); err != nil {
		return fmt.Errorf("failed to create IDE prompt directory: %w", err)
	}

	instruction := fmt.Sprintf("CODENER ATTEMPT [%s]\nPlease complete the task defined in your active step.", attempt.ID)
	if err := os.WriteFile(promptFile, []byte(instruction), 0644); err != nil {
		return fmt.Errorf("failed to write IDE prompt buffer: %w", err)
	}

	slog.Info("IDE Adapter: Prompt buffer written for IDE ingestion", "path", promptFile)
	return nil
}

// Poll checks if the IDE chat session has written the `result.json` payload back to the workspace.
func (a *Adapter) Poll(ctx context.Context, attemptID string) (bool, error) {
	// For IDE plugins, we simulate polling by checking if the extension has materialized
	// a specific .codencer/result.json file signaling chat completion.

	if os.Getenv("IDE_SIMULATION_MODE") == "1" {
		return false, nil // Assume simulate instantaneous completion
	}

	// This is a placeholder for manual/proxy polling logic:
	// Since we lack native IDE control, we rely on the extension to write back a signal
	// or the user to manually trigger completion.
	return false, nil
}

func (a *Adapter) Cancel(ctx context.Context, attemptID string) error {
	slog.Info("IDE Adapter: Cancellation called", "attemptID", attemptID)
	return nil
}

func (a *Adapter) CollectArtifacts(ctx context.Context, attemptID, attemptArtifactRoot string) ([]*domain.Artifact, error) {
	slog.Info("IDE Adapter: Collecting artifacts", "attemptID", attemptID)

	var artifacts []*domain.Artifact
	
	// Create simulate result if in test mode
	if os.Getenv("IDE_SIMULATION_MODE") == "1" {
		simulatedResult := map[string]interface{}{
			"status": "completed",
			"summary": "IDE Chat simulation success",
		}
		data, _ := json.Marshal(simulatedResult)
		_ = os.WriteFile(filepath.Join(attemptArtifactRoot, "result.json"), data, 0644)
	}

	resultPath := filepath.Join(attemptArtifactRoot, "result.json")
	if stat, err := os.Stat(resultPath); err == nil {
		artifacts = append(artifacts, &domain.Artifact{
			ID:        fmt.Sprintf("art-%s-result", attemptID),
			AttemptID: attemptID,
			Type:      "result_json",
			Path:      resultPath,
			Size:      stat.Size(),
			CreatedAt: time.Now().UTC(),
		})
	}

	return artifacts, nil
}

func (a *Adapter) NormalizeResult(ctx context.Context, attemptID string, artifacts []*domain.Artifact) (*domain.ResultSpec, error) {
	var resultFile string
	for _, art := range artifacts {
		if art.Type == "result_json" {
			resultFile = art.Path
			break
		}
	}

	if resultFile == "" {
		return nil, fmt.Errorf("no result.json artifact found for IDE attempt %s", attemptID)
	}

	data, err := os.ReadFile(resultFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read result.json from IDE bounds: %w", err)
	}

	var payload struct {
		Status  string `json:"status"`
		Summary string `json:"summary"`
	}

	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("invalid result.json schema from IDE: %w", err)
	}

	status := domain.StepStateNeedsApproval
	switch payload.Status {
	case "completed":
		status = domain.StepStateCompleted
	case "failed":
		status = domain.StepStateFailedTerminal
	}

	return &domain.ResultSpec{
		State:              status,
		Summary:            payload.Summary,
		NeedsHumanDecision: false,
	}, nil
}
