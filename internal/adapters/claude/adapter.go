package claude

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

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
		return fmt.Errorf("attempt %s is already running", attempt.ID)
	}

	execCtx, cancel := context.WithCancel(context.Background())
	a.processes[attempt.ID] = &cancel

	go func() {
		defer cancel()
		slog.Info("Claude Adapter: Starting process", "attemptID", attempt.ID)
		
		err := InvokeLocal(execCtx, attempt, workspaceRoot, artifactRoot)
		if err != nil {
			slog.Error("Claude Adapter: Process failed", "attemptID", attempt.ID, "error", err)
		} else {
			slog.Info("Claude Adapter: Process finished", "attemptID", attempt.ID)
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
		return fmt.Errorf("attempt %s is not running", attemptID)
	}

	slog.Info("Claude Adapter: Cancelling process", "attemptID", attemptID)
	(*cancelFunc)()
	delete(a.processes, attemptID)
	return nil
}

func (a *Adapter) CollectArtifacts(ctx context.Context, attemptID string, artifactRoot string) ([]*domain.Artifact, error) {
	slog.Info("Claude Adapter: Collecting artifacts", "attemptID", attemptID)
	
	var artifacts []*domain.Artifact
	
	entries, err := os.ReadDir(artifactRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return artifacts, nil
		}
		return nil, fmt.Errorf("failed to read artifact root %s: %w", artifactRoot, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		info, err := entry.Info()
		if err != nil {
			continue
		}

		path := filepath.Join(artifactRoot, entry.Name())
		artType := domain.ArtifactType("file")
		
		if entry.Name() == "stdout.log" {
			artType = domain.ArtifactTypeStdout
		} else if entry.Name() == "result.json" {
			artType = domain.ArtifactType("result_json")
		} else if filepath.Ext(entry.Name()) == ".patch" {
			artType = domain.ArtifactType("diff")
		}

		artifacts = append(artifacts, &domain.Artifact{
			ID:        fmt.Sprintf("art-%s-%s", entry.Name(), attemptID),
			AttemptID: attemptID,
			Type:      artType,
			Path:      path,
			Size:      info.Size(),
			CreatedAt: info.ModTime(),
		})
	}
	
	return artifacts, nil
}

func (a *Adapter) NormalizeResult(ctx context.Context, attemptID string, artifacts []*domain.Artifact) (*domain.Result, error) {
	var resultPath string
	for _, art := range artifacts {
		if art.Type == "result_json" {
			resultPath = art.Path
		}
	}
	return NormalizeCore(attemptID, resultPath)
}
