package codex

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"agent-bridge/internal/domain"
)

// Adapter implements domain.Adapter for the local Codex agent.
type Adapter struct {
	processes map[string]*context.CancelFunc // Map of attemptID to cancel functions
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

func (a *Adapter) Start(ctx context.Context, attempt *domain.Attempt) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.processes[attempt.ID]; exists {
		return fmt.Errorf("attempt %s is already running", attempt.ID)
	}

	execCtx, cancel := context.WithCancel(context.Background())
	a.processes[attempt.ID] = &cancel

	// Run invocation async as requested by typical adapter spec
	go func() {
		defer cancel()
		slog.Info("Codex Adapter: Starting process", "attemptID", attempt.ID)
		
		err := InvokeLocal(execCtx, attempt)
		if err != nil {
			slog.Error("Codex Adapter: Process failed", "attemptID", attempt.ID, "error", err)
		} else {
			slog.Info("Codex Adapter: Process finished", "attemptID", attempt.ID)
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

	slog.Info("Codex Adapter: Cancelling process", "attemptID", attemptID)
	(*cancelFunc)()
	delete(a.processes, attemptID)
	return nil
}

func (a *Adapter) CollectArtifacts(ctx context.Context, attemptID string, artifactRoot string) ([]*domain.Artifact, error) {
	slog.Info("Codex Adapter: Collecting artifacts", "attemptID", attemptID)
	// For MVP, just return stub records or basic text logs
	now := time.Now()
	return []*domain.Artifact{
		{
			ID:        fmt.Sprintf("art-stdout-%s", attemptID),
			AttemptID: attemptID,
			Type:      domain.ArtifactTypeStdout,
			Path:      fmt.Sprintf("%s/stdout.log", artifactRoot),
			Size:      1024,
			CreatedAt: now,
		},
	}, nil
}

func (a *Adapter) NormalizeResult(ctx context.Context, attemptID string, artifacts []*domain.Artifact) (*domain.Result, error) {
	// MVP normalization
	return NormalizeCore(attemptID, artifacts)
}
