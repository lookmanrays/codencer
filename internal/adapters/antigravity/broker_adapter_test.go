package antigravity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"agent-bridge/internal/domain"
)

func TestBrokerAdapter_Start(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/tasks" && r.Method == "POST" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"id": "broker-task-123"})
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	adapter := NewBrokerAdapter(server.URL, "/repo")
	step := &domain.Step{Goal: "test goal"}
	attempt := &domain.Attempt{ID: "att-1"}

	err := adapter.Start(context.Background(), step, attempt, "/tmp", "/tmp/artifacts")
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if adapter.taskCache["att-1"] != "broker-task-123" {
		t.Errorf("Expected broker task ID broker-task-123, got %s", adapter.taskCache["att-1"])
	}
}

func TestBrokerAdapter_PollMapping(t *testing.T) {
	tests := []struct {
		brokerState   string
		expectedRunning bool
	}{
		{"running",   true},
		{"completed", false},
		{"failed",    false},
		{"error",     false},
	}

	for _, tt := range tests {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"state": tt.brokerState})
		}))
		
		adapter := NewBrokerAdapter(server.URL, "/repo")
		adapter.taskCache["att-1"] = "task-1"
		
		running, err := adapter.Poll(context.Background(), "att-1")
		if err != nil {
			t.Errorf("Poll failed for %s: %v", tt.brokerState, err)
		}
		if running != tt.expectedRunning {
			t.Errorf("For %s, expected running=%v, got %v", tt.brokerState, tt.expectedRunning, running)
		}
		server.Close()
	}
}
