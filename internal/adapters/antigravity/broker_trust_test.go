package antigravity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"agent-bridge/internal/domain"
)

func TestBrokerAdapter_Trust_FailureMapping(t *testing.T) {
	tests := []struct {
		name          string
		brokerState   string
		expectedState domain.StepState
	}{
		{"Logic Failure", "failed", domain.StepStateFailedTerminal},
		{"Infrastructure Failure", "error", domain.StepStateFailedAdapter},
		{"Cancellation", "cancelled", domain.StepStateCancelled},
		{"Success", "completed", domain.StepStateCompleted},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{
					"state":   tt.brokerState,
					"summary": "Mock summary",
				})
			}))
			defer server.Close()

			adapter := NewBrokerAdapter(server.URL, "/repo")
			adapter.taskCache["att-1"] = "task-1"

			spec, err := adapter.NormalizeResult(context.Background(), "att-1", nil)
			if err != nil {
				t.Fatalf("NormalizeResult failed: %v", err)
			}

			if spec.State != tt.expectedState {
				t.Errorf("Expected state %s, got %s", tt.expectedState, spec.State)
			}
			
			if spec.Artifacts["broker_task_id"] != "task-1" {
				t.Errorf("Expected broker_task_id task-1, got %s", spec.Artifacts["broker_task_id"])
			}
		})
	}
}

func TestBrokerAdapter_Trust_DeepErrorExtraction(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "broker-trust-test")
	defer os.RemoveAll(tmpDir)

	trajectoryPath := filepath.Join(tmpDir, "trajectory.json")
	mockTraj := map[string]any{
		"steps": []any{
			map[string]any{
				"items": []any{
					map[string]any{
						"error": map[string]any{"message": "The system is down!"},
					},
				},
			},
		},
	}
	data, _ := json.Marshal(mockTraj)
	os.WriteFile(trajectoryPath, data, 0644)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"state":   "failed",
			"summary": "Broker reported failure",
		})
	}))
	defer server.Close()

	adapter := NewBrokerAdapter(server.URL, "/repo")
	adapter.taskCache["att-1"] = "task-1"

	arts := []*domain.Artifact{
		{Name: "trajectory.json", Path: trajectoryPath},
	}

	spec, err := adapter.NormalizeResult(context.Background(), "att-1", arts)
	if err != nil {
		t.Fatalf("NormalizeResult failed: %v", err)
	}

	expectedSummary := "Broker reported failure (Detail: Error: The system is down!)"
	if spec.Summary != expectedSummary {
		t.Errorf("Expected summary %q, got %q", expectedSummary, spec.Summary)
	}
}
