package antigravity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"agent-bridge/internal/domain"
)

// InstanceProvider is an interface to resolve the bound Antigravity instance.
// This avoids circular dependencies with the service package.
type InstanceProvider interface {
	GetBinding(ctx context.Context) (*domain.AGInstance, error)
}

// Adapter implements domain.Adapter for the Antigravity Language Server.
type Adapter struct {
	client           *Client
	instanceProvider InstanceProvider

	// activeCascades maps attemptID -> cascadeID
	activeCascades map[string]string
	// instanceCache maps attemptID -> AGInstance (pinned for the attempt)
	instanceCache map[string]domain.AGInstance
	// approvedInteractions prevents duplicate approvals if polling observes the same wait twice.
	approvedInteractions map[string]struct{}
	mu                   sync.RWMutex
}

func NewAdapter(provider InstanceProvider) *Adapter {
	return &Adapter{
		client:               NewClient(),
		instanceProvider:     provider,
		activeCascades:       make(map[string]string),
		instanceCache:        make(map[string]domain.AGInstance),
		approvedInteractions: make(map[string]struct{}),
	}
}

func (a *Adapter) Name() string {
	return "antigravity"
}

func (a *Adapter) Capabilities() []string {
	return []string{"remote_rpc", "filesystem_read", "filesystem_write", "trajectory_evidence"}
}

func (a *Adapter) Start(ctx context.Context, step *domain.Step, attempt *domain.Attempt, workspaceRoot, attemptArtifactRoot string) error {
	inst, err := a.instanceProvider.GetBinding(ctx)
	if err != nil {
		return fmt.Errorf("failed to resolve Antigravity binding: %w", err)
	}
	if inst == nil {
		return fmt.Errorf("no Antigravity instance bound to this repository. Use 'orchestratorctl antigravity bind' first")
	}

	workspaceURI := inst.WorkspaceRoot
	if workspaceRoot != "" {
		workspaceURI = fmt.Sprintf("file://%s", workspaceRoot)
	} else if workspaceURI == "" {
		return fmt.Errorf("workspace root is required for Antigravity execution")
	}

	req := &StartCascadeRequest{
		UserPrompt:                 step.Goal,
		WorkspaceFolderAbsoluteUri: workspaceURI,
		Source:                     CortexTrajectorySourceCascadeClient,
		Metadata: CascadeMetadata{
			FileAccessGranted: true,
		},
		CascadeConfig: CascadeConfig{
			PlannerConfig: PlannerConfig{
				PlannerTypeConfig: PlannerTypeConfig{
					Planning: struct{}{},
				},
				RequestedModel: RequestedModel{
					Model: DefaultRequestedModelGoogleGemini25Flash,
				},
			},
		},
	}

	var resp StartCascadeResponse
	if err := a.client.Call(ctx, inst, "StartCascade", req, &resp); err != nil {
		return fmt.Errorf("failed to start cascade: %w", err)
	}

	if resp.CascadeId == "" {
		return fmt.Errorf("Antigravity returned an empty cascadeId")
	}

	inst.WorkspaceRoot = workspaceURI

	a.mu.Lock()
	a.activeCascades[attempt.ID] = resp.CascadeId
	a.instanceCache[attempt.ID] = *inst
	a.mu.Unlock()

	sendReq := &SendUserCascadeMessageRequest{
		CascadeId: resp.CascadeId,
		Items: []CascadeMessageItem{
			{Text: step.Goal},
		},
		CascadeConfig:       req.CascadeConfig,
		WaitForLSClientInit: false,
	}
	if err := a.client.Call(ctx, inst, "SendUserCascadeMessage", sendReq, &struct{}{}); err != nil {
		a.mu.Lock()
		delete(a.activeCascades, attempt.ID)
		delete(a.instanceCache, attempt.ID)
		a.mu.Unlock()
		return fmt.Errorf("failed to send cascade message: %w", err)
	}

	return nil
}

func (a *Adapter) Poll(ctx context.Context, attemptID string) (bool, error) {
	a.mu.RLock()
	cascadeID, exists := a.activeCascades[attemptID]
	inst, instExists := a.instanceCache[attemptID]
	a.mu.RUnlock()

	if !exists || !instExists {
		return false, nil // Not started or already finished
	}

	req := &GetCascadeTrajectoryRequest{
		CascadeId: cascadeID,
	}
	var resp GetCascadeTrajectoryResponse
	if err := a.client.Call(ctx, &inst, "GetCascadeTrajectory", req, &resp); err != nil {
		// If the instance is unreachable, it's an adapter/env failure, not a task failure.
		if errors.Is(err, ErrInstanceUnreachable) || errors.Is(err, ErrAuthFailed) {
			return false, fmt.Errorf("antigravity transport failure: %w", err)
		}
		return false, fmt.Errorf("failed to get trajectory status: %w", err)
	}

	switch resp.Status {
	case StatusRunning:
		if err := a.approvePendingReadOnlyInteractions(ctx, &inst, cascadeID); err != nil {
			return false, err
		}
		return true, nil
	case StatusCompleted, StatusFailed, StatusAborted:
		return false, nil
	case StatusIdle:
		if hasSteps, err := a.inspectAndApprovePendingInteractions(ctx, &inst, cascadeID); err != nil {
			return false, err
		} else if !hasSteps {
			return true, nil
		}
		return false, nil
	default:
		return false, nil
	}
}

func (a *Adapter) approvePendingReadOnlyInteractions(ctx context.Context, inst *domain.AGInstance, cascadeID string) error {
	_, err := a.inspectAndApprovePendingInteractions(ctx, inst, cascadeID)
	return err
}

func (a *Adapter) inspectAndApprovePendingInteractions(ctx context.Context, inst *domain.AGInstance, cascadeID string) (bool, error) {
	req := &GetCascadeTrajectoryStepsRequest{
		CascadeId:  cascadeID,
		StepOffset: 0,
	}
	var resp GetCascadeTrajectoryStepsResponse
	if err := a.client.Call(ctx, inst, "GetCascadeTrajectorySteps", req, &resp); err != nil {
		return false, fmt.Errorf("failed to inspect Antigravity pending interactions: %w", err)
	}

	for _, step := range resp.Steps {
		if step.Status != StepStatusWaiting {
			continue
		}
		approval, ok := a.readOnlyApprovalForStep(inst.WorkspaceRoot, step)
		if !ok {
			continue
		}

		info := step.Metadata.SourceTrajectoryStepInfo
		if info.TrajectoryId == "" {
			continue
		}
		key := fmt.Sprintf("%s:%s:%d", cascadeID, info.TrajectoryId, info.StepIndex)

		a.mu.RLock()
		_, seen := a.approvedInteractions[key]
		a.mu.RUnlock()
		if seen {
			continue
		}

		interactionReq := &HandleCascadeUserInteractionRequest{
			CascadeId: cascadeID,
			Interaction: CascadeUserInteraction{
				TrajectoryId: info.TrajectoryId,
				StepIndex:    info.StepIndex,
				Interaction:  approval,
			},
		}
		if err := a.client.Call(ctx, inst, "HandleCascadeUserInteraction", interactionReq, &struct{}{}); err != nil {
			return false, fmt.Errorf("failed to approve Antigravity read-only interaction: %w", err)
		}

		a.mu.Lock()
		a.approvedInteractions[key] = struct{}{}
		a.mu.Unlock()
	}

	return len(resp.Steps) > 0, nil
}

func (a *Adapter) readOnlyApprovalForStep(workspaceRoot string, step CascadeStep) (map[string]interface{}, bool) {
	info := step.Metadata.SourceTrajectoryStepInfo
	if info.TrajectoryId == "" {
		return nil, false
	}

	if step.RequestedInteraction != nil && step.RequestedInteraction.Permission != nil && step.RequestedInteraction.Permission.Resource != nil {
		resource := step.RequestedInteraction.Permission.Resource
		if isReadOnlyPermissionAction(resource.Action) && pathWithinWorkspace(resource.Target, workspaceRoot) {
			return map[string]interface{}{
				"permission": map[string]interface{}{
					"allow": true,
				},
			}, true
		}
	}

	if step.Metadata.ToolCall != nil && step.Metadata.ToolCall.Name == "ask_permission" {
		args := decodeToolArguments(step.Metadata.ToolCall.ArgumentsJSON)
		if isReadOnlyPermissionAction(args.Action) && pathWithinWorkspace(args.Target, workspaceRoot) {
			return map[string]interface{}{
				"approvalInteraction": map[string]interface{}{
					"confirm": true,
				},
			}, true
		}
	}

	return nil, false
}

func isReadOnlyPermissionAction(action string) bool {
	switch strings.ToLower(action) {
	case "read_file", "list_directory", "read_directory":
		return true
	default:
		return false
	}
}

func pathWithinWorkspace(target, workspaceRoot string) bool {
	if target == "" || workspaceRoot == "" {
		return false
	}

	rootPath, err := fileURIPath(workspaceRoot)
	if err != nil {
		return false
	}
	targetPath := target
	if strings.HasPrefix(target, "file://") {
		targetPath, err = fileURIPath(target)
		if err != nil {
			return false
		}
	}

	rootPath, err = filepath.Abs(rootPath)
	if err != nil {
		return false
	}
	targetPath, err = filepath.Abs(targetPath)
	if err != nil {
		return false
	}

	rel, err := filepath.Rel(rootPath, targetPath)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func fileURIPath(uri string) (string, error) {
	parsed, err := url.Parse(uri)
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "file" {
		return "", fmt.Errorf("unsupported URI scheme %q", parsed.Scheme)
	}
	return parsed.Path, nil
}

func (a *Adapter) Cancel(ctx context.Context, attemptID string) error {
	a.mu.RLock()
	cascadeID, exists := a.activeCascades[attemptID]
	inst, instExists := a.instanceCache[attemptID]
	a.mu.RUnlock()

	if !exists || !instExists {
		return nil
	}

	req := &struct {
		CascadeId string `json:"cascadeId"`
	}{CascadeId: cascadeID}

	var resp struct{}
	if err := a.client.Call(ctx, &inst, "CancelCascadeInvocation", req, &resp); err != nil {
		return fmt.Errorf("failed to cancel cascade: %w", err)
	}

	return nil
}

func (a *Adapter) CollectArtifacts(ctx context.Context, attemptID string, attemptArtifactRoot string) ([]*domain.Artifact, error) {
	a.mu.RLock()
	cascadeID, exists := a.activeCascades[attemptID]
	inst, instExists := a.instanceCache[attemptID]
	a.mu.RUnlock()

	if !exists || !instExists {
		return nil, nil
	}

	req := &GetCascadeTrajectoryStepsRequest{
		CascadeId:  cascadeID,
		StepOffset: 0,
	}
	var resp GetCascadeTrajectoryStepsResponse
	if err := a.client.Call(ctx, &inst, "GetCascadeTrajectorySteps", req, &resp); err != nil {
		return nil, fmt.Errorf("failed to fetch trajectory steps: %w", err)
	}

	// Persist the raw trajectory as a JSON file
	trajectoryPath := filepath.Join(attemptArtifactRoot, "trajectory.json")
	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal trajectory: %w", err)
	}

	if err := os.WriteFile(trajectoryPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write trajectory file: %w", err)
	}

	return []*domain.Artifact{
		{
			ID:        fmt.Sprintf("trajectory-%s", attemptID),
			Name:      "trajectory.json",
			Path:      trajectoryPath,
			Type:      domain.ArtifactTypeResultJSON,
			CreatedAt: time.Now(),
		},
	}, nil
}

func (a *Adapter) NormalizeResult(ctx context.Context, attemptID string, artifacts []*domain.Artifact) (*domain.ResultSpec, error) {
	a.mu.RLock()
	cascadeID, exists := a.activeCascades[attemptID]
	inst, instExists := a.instanceCache[attemptID]
	a.mu.RUnlock()

	if !exists || !instExists {
		return nil, fmt.Errorf("attempt context lost for %s", attemptID)
	}

	req := &GetCascadeTrajectoryRequest{
		CascadeId: cascadeID,
	}
	var resp GetCascadeTrajectoryResponse
	if err := a.client.Call(ctx, &inst, "GetCascadeTrajectory", req, &resp); err != nil {
		return nil, fmt.Errorf("failed to get terminal trajectory status: %w", err)
	}

	state := domain.StepStateCompleted
	summary := "Antigravity execution completed successfully"
	if resp.Status == StatusFailed {
		state = domain.StepStateFailedTerminal
		summary = "Antigravity reported execution failure"
	} else if resp.Status == StatusAborted {
		state = domain.StepStateCancelled
		summary = "Antigravity execution was aborted"
	}

	// Enrich summary from trajectory steps if available
	for _, art := range artifacts {
		if art.Name == "trajectory.json" {
			if content, err := os.ReadFile(art.Path); err == nil {
				var traj GetCascadeTrajectoryStepsResponse
				if err := json.Unmarshal(content, &traj); err == nil && len(traj.Steps) > 0 {
					// Search for a detailed error or the last text message
					detail := ""
					for i := len(traj.Steps) - 1; i >= 0; i-- {
						for _, item := range traj.Steps[i].Items {
							if item.Error != nil {
								detail = fmt.Sprintf("Error: %s", item.Error.Message)
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
						summary = fmt.Sprintf("%s (Details: %s)", summary, detail)
					}
				}
			}
			break
		}
	}

	// Clean up after normalization
	a.mu.Lock()
	delete(a.activeCascades, attemptID)
	delete(a.instanceCache, attemptID)
	for key := range a.approvedInteractions {
		if strings.HasPrefix(key, cascadeID+":") {
			delete(a.approvedInteractions, key)
		}
	}
	a.mu.Unlock()

	return &domain.ResultSpec{
		State:     state,
		Summary:   summary,
		Artifacts: map[string]string{"cascade_id": cascadeID},
		RawOutput: fmt.Sprintf("Antigravity Cascade ID: %s\nStatus: %s", cascadeID, resp.Status),
		UpdatedAt: time.Now(),
	}, nil
}
