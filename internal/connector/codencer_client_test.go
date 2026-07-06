package connector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/relayproto"
)

func TestCodencerClientWaitStepReturnsAtGateDecision(t *testing.T) {
	daemon := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-gated":
			_ = json.NewEncoder(w).Encode(domain.Step{
				ID:    "step-gated",
				State: domain.StepStateNeedsApproval,
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-gated/result":
			_ = json.NewEncoder(w).Encode(domain.ResultSpec{
				Version:            "v1",
				StepID:             "step-gated",
				State:              domain.StepStateNeedsApproval,
				Summary:            "Policy enforced gate: review required.",
				NeedsHumanDecision: true,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer daemon.Close()

	client := NewCodencerClient(daemon.URL)
	body, _ := json.Marshal(map[string]any{
		"timeout_ms":     5000,
		"interval_ms":    1000,
		"include_result": true,
	})
	start := time.Now()
	resp := client.Proxy(context.Background(), relayproto.CommandRequest{
		Type:      "request",
		RequestID: "req-1",
		Method:    http.MethodPost,
		Path:      "/api/v1/steps/step-gated/wait",
		Body:      body,
	})
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("wait_step should return immediately at a gate, took %s", elapsed)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected wait response ok, got %d error=%s", resp.StatusCode, resp.Error)
	}
	var payload map[string]any
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		t.Fatalf("decode wait payload: %v", err)
	}
	if payload["state"] != string(domain.StepStateNeedsApproval) {
		t.Fatalf("expected needs_approval payload, got %+v", payload)
	}
	if payload["terminal"] != false || payload["needs_decision"] != true || payload["decision_kind"] != "gate" {
		t.Fatalf("expected non-terminal gate decision payload, got %+v", payload)
	}
	if _, ok := payload["result"].(map[string]any); !ok {
		t.Fatalf("expected included result payload, got %+v", payload)
	}
}
