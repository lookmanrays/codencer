package antigravity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"agent-bridge/internal/domain"
)

// BrokerAdapter implements domain.Adapter by proxying calls to an Antigravity Broker.
type BrokerAdapter struct {
	baseURL string
	client  *http.Client
	
	// baseRepoRoot is the fixed repository path used for binding lookup
	baseRepoRoot string
	
	// taskCache maps attemptID -> broker_task_id
	taskCache map[string]string
	mu        sync.RWMutex
}

func NewBrokerAdapter(baseURL, baseRepoRoot string) *BrokerAdapter {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8088"
	}
	return &BrokerAdapter{
		baseURL:      baseURL,
		client:       &http.Client{Timeout: 10 * time.Second},
		baseRepoRoot: baseRepoRoot,
		taskCache:    make(map[string]string),
	}
}

func (a *BrokerAdapter) Name() string {
	return "antigravity-broker"
}

func (a *BrokerAdapter) Capabilities() []string {
	return []string{"remote_proxy", "filesystem_read", "filesystem_write", "trajectory_evidence"}
}

func (a *BrokerAdapter) Start(ctx context.Context, step *domain.Step, attempt *domain.Attempt, workspaceRoot, attemptArtifactRoot string) error {
	url := fmt.Sprintf("%s/tasks", a.baseURL)
	reqBody := map[string]string{
		"prompt":         step.Goal,
		"repo_root":      a.baseRepoRoot, // Binding Identity
		"workspace_root": workspaceRoot,  // Execution Context
	}
	data, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("Broker Transport Error - broker unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("broker start failed (%d): %s", resp.StatusCode, string(body))
	}

	var task struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return fmt.Errorf("failed to decode broker task ID: %w", err)
	}

	a.mu.Lock()
	a.taskCache[attempt.ID] = task.ID
	a.mu.Unlock()

	return nil
}

func (a *BrokerAdapter) Poll(ctx context.Context, attemptID string) (bool, error) {
	a.mu.RLock()
	taskID, exists := a.taskCache[attemptID]
	a.mu.RUnlock()
	if !exists {
		return false, nil
	}

	url := fmt.Sprintf("%s/tasks/%s", a.baseURL, taskID)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := a.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("Broker Transport Error - broker poll failure: %w", err)
	}
	defer resp.Body.Close()

	var task struct {
		State   string `json:"state"`
		Summary string `json:"summary"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return false, err
	}

	switch task.State {
	case "completed", "failed", "cancelled", "error":
		return false, nil
	default:
		return true, nil
	}
}

func (a *BrokerAdapter) Cancel(ctx context.Context, attemptID string) error {
	a.mu.RLock()
	taskID, exists := a.taskCache[attemptID]
	a.mu.RUnlock()
	if !exists {
		return nil
	}

	url := fmt.Sprintf("%s/tasks/%s", a.baseURL, taskID)
	req, _ := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	_, _ = a.client.Do(req)
	return nil
}

func (a *BrokerAdapter) CollectArtifacts(ctx context.Context, attemptID, attemptArtifactRoot string) ([]*domain.Artifact, error) {
	a.mu.RLock()
	taskID, exists := a.taskCache[attemptID]
	a.mu.RUnlock()
	if !exists {
		return nil, nil
	}

	// Fetch raw trajectory from broker
	url := fmt.Sprintf("%s/tasks/%s/result", a.baseURL, taskID)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	trajectoryPath := filepath.Join(attemptArtifactRoot, "trajectory.json")
	f, err := os.Create(trajectoryPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	_, _ = io.Copy(f, resp.Body)

	return []*domain.Artifact{
		{
			ID:        fmt.Sprintf("broker-traj-%s", attemptID),
			Name:      "trajectory.json",
			Path:      trajectoryPath,
			Type:      domain.ArtifactTypeResultJSON,
			CreatedAt: time.Now(),
		},
	}, nil
}

func (a *BrokerAdapter) NormalizeResult(ctx context.Context, attemptID string, artifacts []*domain.Artifact) (*domain.ResultSpec, error) {
	a.mu.RLock()
	taskID, exists := a.taskCache[attemptID]
	a.mu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("task context lost for attempt %s", attemptID)
	}

	url := fmt.Sprintf("%s/tasks/%s", a.baseURL, taskID)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Broker Transport Error - broker unreachable during normalization: %w", err)
	}
	defer resp.Body.Close()

	var task struct {
		State   string `json:"state"`
		Summary string `json:"summary"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return nil, fmt.Errorf("failed to decode broker task status: %w", err)
	}

	finalState := domain.StepStateCompleted
	finalSummary := task.Summary

	if task.State == "failed" {
		finalState = domain.StepStateFailedTerminal
	} else if task.State == "cancelled" {
		finalState = domain.StepStateCancelled
	} else if task.State == "error" {
		finalState = domain.StepStateFailedAdapter
	}

	// Deep Error Extraction from trajectory.json (if available)
	for _, art := range artifacts {
		if art.Name == "trajectory.json" {
			if content, err := os.ReadFile(art.Path); err == nil {
				// The trajectory format is the same as the direct LS Go binding
				var traj struct {
					Steps []struct {
						Items []struct {
							Message *struct{ Text string `json:"text"` } `json:"message"`
							Error   *struct{ Message string `json:"message"` } `json:"error"`
						} `json:"items"`
					} `json:"steps"`
				}
				if err := json.Unmarshal(content, &traj); err == nil && len(traj.Steps) > 0 {
					detail := ""
					for i := len(traj.Steps) - 1; i >= 0; i-- {
						for _, item := range traj.Steps[i].Items {
							if item.Error != nil {
								detail = "Error: " + item.Error.Message
								break
							}
							if item.Message != nil && detail == "" {
								detail = item.Message.Text
							}
						}
						if detail != "" && strings.HasPrefix(detail, "Error:") {
							break
						}
					}
					if detail != "" {
						finalSummary = fmt.Sprintf("%s (Detail: %s)", finalSummary, detail)
					}
				}
			}
			break
		}
	}

	return &domain.ResultSpec{
		State:     finalState,
		Summary:   finalSummary,
		Artifacts: map[string]string{"broker_task_id": taskID},
		RawOutput: fmt.Sprintf("Broker Task ID: %s\nBroker State: %s", taskID, task.State),
		UpdatedAt: time.Now(),
	}, nil
}
