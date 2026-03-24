package qwen

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"agent-bridge/internal/domain"
)

// Adapter implements domain.Adapter for the Qwen agent.
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
	return "qwen"
}

func (a *Adapter) Capabilities() []string {
	return []string{"local_inference", "coding"}
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
		slog.Info("Qwen Adapter: Starting process", "attemptID", attempt.ID)
		
		err := InvokeLocal(execCtx, attempt, workspaceRoot, artifactRoot)
		if err != nil {
			slog.Error("Qwen Adapter: Process failed", "attemptID", attempt.ID, "error", err)
		} else {
			slog.Info("Qwen Adapter: Process finished", "attemptID", attempt.ID)
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

	slog.Info("Qwen Adapter: Cancelling process", "attemptID", attemptID)
	(*cancelFunc)()
	delete(a.processes, attemptID)
	return nil
}

func (a *Adapter) CollectArtifacts(ctx context.Context, attemptID string, artifactRoot string) ([]*domain.Artifact, error) {
	slog.Info("Qwen Adapter: Collecting artifacts", "attemptID", attemptID)
	now := time.Now()
	
	artifacts := []*domain.Artifact{}
	_ = artifactRoot

	artifacts = append(artifacts, &domain.Artifact{
		ID:        fmt.Sprintf("art-qwen-stdout-%s", attemptID),
		AttemptID: attemptID,
		Type:      domain.ArtifactTypeStdout,
		Path:      fmt.Sprintf("%s/qwen_stdout.log", artifactRoot),
		Size:      1024,
		CreatedAt: now,
	})
	
	artifacts = append(artifacts, &domain.Artifact{
		ID:        fmt.Sprintf("art-qwen-result-%s", attemptID),
		AttemptID: attemptID,
		Type:      domain.ArtifactType("result_json"),
		Path:      fmt.Sprintf("%s/qwen_result.json", artifactRoot),
		Size:      500,
		CreatedAt: now,
	})
	
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
