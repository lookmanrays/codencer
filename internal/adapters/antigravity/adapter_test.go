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
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "StartCascade") {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(StartCascadeResponse{CascadeId: "test-cascade"})
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
	step := &domain.Step{Goal: "test goal"}
	attempt := &domain.Attempt{ID: "attempt-1"}

	err := adapter.Start(context.Background(), step, attempt, "/tmp/ws", "/tmp/artifacts")
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if adapter.activeCascades["attempt-1"] != "test-cascade" {
		t.Errorf("Expected cascadeId test-cascade, got %s", adapter.activeCascades["attempt-1"])
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
