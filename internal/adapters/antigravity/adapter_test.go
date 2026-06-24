package antigravity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"agent-bridge/internal/domain"
)

type mockProvider struct {
	inst *domain.AGInstance
}

func (m *mockProvider) GetBinding(ctx context.Context) (*domain.AGInstance, error) {
	return m.inst, nil
}

func TestAdapter_Start(t *testing.T) {
	var capturedURI string
	var capturedSource int
	var capturedMessage string
	var capturedModel int
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "StartCascade") {
			var req StartCascadeRequest
			json.NewDecoder(r.Body).Decode(&req)
			capturedURI = req.WorkspaceFolderAbsoluteUri
			capturedSource = req.Source
			capturedModel = req.CascadeConfig.PlannerConfig.RequestedModel.Model

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(StartCascadeResponse{CascadeId: "test-cascade"})
			return
		}
		if strings.HasSuffix(r.URL.Path, "SendUserCascadeMessage") {
			var req SendUserCascadeMessageRequest
			json.NewDecoder(r.Body).Decode(&req)
			if req.CascadeId != "test-cascade" {
				t.Errorf("Expected cascade id test-cascade, got %s", req.CascadeId)
			}
			if len(req.Items) == 1 {
				capturedMessage = req.Items[0].Text
			}
			if req.WaitForLSClientInit {
				t.Errorf("Expected waitForLsClientInit=false")
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	portStr := strings.Split(server.Listener.Addr().String(), ":")[1]
	port, _ := strconv.Atoi(portStr)

	// Case 1: No authoritative WorkspaceRoot
	inst := &domain.AGInstance{
		HTTPSPort: port,
		CSRFToken: "test-token",
	}

	adapter := NewAdapter(&mockProvider{inst: inst})
	step := &domain.Step{Goal: "test goal"}
	attempt := &domain.Attempt{ID: "attempt-1"}

	err := adapter.Start(context.Background(), step, attempt, "/tmp/ws", "/tmp/artifacts")
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if capturedURI != "file:///tmp/ws" {
		t.Errorf("Expected fallback URI file:///tmp/ws, got %s", capturedURI)
	}
	if capturedSource != CortexTrajectorySourceCascadeClient {
		t.Errorf("Expected source %d, got %d", CortexTrajectorySourceCascadeClient, capturedSource)
	}
	if capturedModel != DefaultRequestedModelGoogleGemini25Flash {
		t.Errorf("Expected model %d, got %d", DefaultRequestedModelGoogleGemini25Flash, capturedModel)
	}
	if capturedMessage != "test goal" {
		t.Errorf("Expected sent message %q, got %q", "test goal", capturedMessage)
	}

	// Case 2: Instance WorkspaceRoot is available, but execution stays scoped to the provisioned workspace.
	inst.WorkspaceRoot = "file:///authoritative/path"
	err = adapter.Start(context.Background(), step, attempt, "/tmp/ws", "/tmp/artifacts")
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if capturedURI != "file:///tmp/ws" {
		t.Errorf("Expected provisioned URI file:///tmp/ws, got %s", capturedURI)
	}
	if capturedSource != CortexTrajectorySourceCascadeClient {
		t.Errorf("Expected source %d, got %d", CortexTrajectorySourceCascadeClient, capturedSource)
	}
}

func TestAdapter_Poll(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "GetCascadeTrajectory") {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(GetCascadeTrajectoryResponse{
				Status:    StatusCompleted,
				CascadeId: "test-cascade",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	portStr := strings.Split(server.Listener.Addr().String(), ":")[1]
	port, _ := strconv.Atoi(portStr)

	inst := &domain.AGInstance{
		HTTPSPort: port,
		CSRFToken: "test-token",
	}

	adapter := NewAdapter(&mockProvider{inst: inst})
	adapter.activeCascades["attempt-1"] = "test-cascade"
	adapter.instanceCache["attempt-1"] = *inst

	running, err := adapter.Poll(context.Background(), "attempt-1")
	if err != nil {
		t.Fatalf("Poll failed: %v", err)
	}

	if running {
		t.Errorf("Expected running=false for StatusCompleted")
	}
}

func TestAdapter_PollApprovesReadOnlyPermission(t *testing.T) {
	var approved bool
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "GetCascadeTrajectory"):
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(GetCascadeTrajectoryResponse{
				Status:    StatusRunning,
				CascadeId: "test-cascade",
			})
		case strings.HasSuffix(r.URL.Path, "GetCascadeTrajectorySteps"):
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(GetCascadeTrajectoryStepsResponse{
				Steps: []CascadeStep{
					{
						Type:   "CORTEX_STEP_TYPE_LIST_DIRECTORY",
						Status: StepStatusWaiting,
						Metadata: CascadeStepMetadata{
							SourceTrajectoryStepInfo: CascadeSourceTrajectoryStepInfo{
								TrajectoryId: "trajectory-1",
								StepIndex:    3,
								CascadeId:    "test-cascade",
							},
						},
						RequestedInteraction: &CascadeRequestedInteraction{
							Permission: &CascadePermissionInteraction{
								Resource: &CascadePermissionResource{
									Action: "list_directory",
									Target: "/tmp/ws",
								},
							},
						},
					},
				},
			})
		case strings.HasSuffix(r.URL.Path, "HandleCascadeUserInteraction"):
			var req HandleCascadeUserInteractionRequest
			json.NewDecoder(r.Body).Decode(&req)
			if req.CascadeId != "test-cascade" {
				t.Errorf("Expected cascade id test-cascade, got %s", req.CascadeId)
			}
			if req.Interaction.TrajectoryId != "trajectory-1" || req.Interaction.StepIndex != 3 {
				t.Errorf("Unexpected interaction target: %+v", req.Interaction)
			}
			permission, ok := req.Interaction.Interaction["permission"].(map[string]interface{})
			if !ok || permission["allow"] != true {
				t.Errorf("Expected permission allow interaction, got %#v", req.Interaction.Interaction)
			}
			approved = true
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	portStr := strings.Split(server.Listener.Addr().String(), ":")[1]
	port, _ := strconv.Atoi(portStr)

	inst := &domain.AGInstance{
		HTTPSPort:     port,
		CSRFToken:     "test-token",
		WorkspaceRoot: "file:///tmp/ws",
	}

	adapter := NewAdapter(&mockProvider{inst: inst})
	adapter.activeCascades["attempt-1"] = "test-cascade"
	adapter.instanceCache["attempt-1"] = *inst

	running, err := adapter.Poll(context.Background(), "attempt-1")
	if err != nil {
		t.Fatalf("Poll failed: %v", err)
	}
	if !running {
		t.Errorf("Expected running=true for StatusRunning")
	}
	if !approved {
		t.Errorf("Expected read-only permission approval")
	}
}

func TestAdapter_PollKeepsEmptyIdleCascadePending(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "GetCascadeTrajectory"):
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(GetCascadeTrajectoryResponse{
				Status:    StatusIdle,
				CascadeId: "test-cascade",
			})
		case strings.HasSuffix(r.URL.Path, "GetCascadeTrajectorySteps"):
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(GetCascadeTrajectoryStepsResponse{Steps: []CascadeStep{}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	portStr := strings.Split(server.Listener.Addr().String(), ":")[1]
	port, _ := strconv.Atoi(portStr)

	inst := &domain.AGInstance{
		HTTPSPort:     port,
		CSRFToken:     "test-token",
		WorkspaceRoot: "file:///tmp/ws",
	}

	adapter := NewAdapter(&mockProvider{inst: inst})
	adapter.activeCascades["attempt-1"] = "test-cascade"
	adapter.instanceCache["attempt-1"] = *inst

	running, err := adapter.Poll(context.Background(), "attempt-1")
	if err != nil {
		t.Fatalf("Poll failed: %v", err)
	}
	if !running {
		t.Errorf("Expected empty idle cascade to remain pending")
	}
}

func TestAdapter_DoesNotApproveOutsideWorkspace(t *testing.T) {
	adapter := NewAdapter(&mockProvider{})
	approval, ok := adapter.readOnlyApprovalForStep("file:///tmp/ws", CascadeStep{
		Status: StepStatusWaiting,
		Metadata: CascadeStepMetadata{
			SourceTrajectoryStepInfo: CascadeSourceTrajectoryStepInfo{
				TrajectoryId: "trajectory-1",
				StepIndex:    1,
			},
		},
		RequestedInteraction: &CascadeRequestedInteraction{
			Permission: &CascadePermissionInteraction{
				Resource: &CascadePermissionResource{
					Action: "read_file",
					Target: "/tmp/other",
				},
			},
		},
	})
	if ok || approval != nil {
		t.Fatalf("Expected no approval outside workspace, got %#v", approval)
	}
}

func TestAdapter_NormalizeResult_WithTrajectory(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "GetCascadeTrajectory") {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(GetCascadeTrajectoryResponse{
				Status:    StatusFailed,
				CascadeId: "test-cascade",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	portStr := strings.Split(server.Listener.Addr().String(), ":")[1]
	port, _ := strconv.Atoi(portStr)

	inst := &domain.AGInstance{
		HTTPSPort: port,
		CSRFToken: "test-token",
	}

	adapter := NewAdapter(&mockProvider{inst: inst})
	adapter.activeCascades["attempt-1"] = "test-cascade"
	adapter.instanceCache["attempt-1"] = *inst

	// Prepare mock trajectory file
	tmpDir := t.TempDir()
	trajPath := filepath.Join(tmpDir, "trajectory.json")
	traj := GetCascadeTrajectoryStepsResponse{
		Steps: []CascadeStep{
			{
				StepIndex: 0,
				Items: []CascadeItem{
					{
						Message: &CascadeMessage{Text: "Initial thought"},
					},
				},
			},
			{
				StepIndex: 1,
				Items: []CascadeItem{
					{
						Error: &CascadeError{Message: "Some tool failed"},
					},
				},
			},
		},
	}
	data, _ := json.Marshal(traj)
	os.WriteFile(trajPath, data, 0644)

	artifacts := []*domain.Artifact{
		{
			Name: "trajectory.json",
			Path: trajPath,
			Type: domain.ArtifactTypeResultJSON,
		},
	}

	res, err := adapter.NormalizeResult(context.Background(), "attempt-1", artifacts)
	if err != nil {
		t.Fatalf("NormalizeResult failed: %v", err)
	}

	if res.State != domain.StepStateFailedTerminal {
		t.Errorf("Expected state FailedTerminal, got %s", res.State)
	}

	expectedSummary := "Antigravity reported execution failure (Details: Error: Some tool failed)"
	if res.Summary != expectedSummary {
		t.Errorf("Expected summary %q, got %q", expectedSummary, res.Summary)
	}

	if res.Artifacts["cascade_id"] != "test-cascade" {
		t.Errorf("Expected cascade_id metadata, got %s", res.Artifacts["cascade_id"])
	}
}
