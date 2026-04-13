package relay_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-bridge/internal/connector"
	"agent-bridge/internal/domain"
	"agent-bridge/internal/relay"
)

type mcpHarness struct {
	relayHTTP *httptest.Server
	daemon    *httptest.Server
	cancel    context.CancelFunc
	waitErr   chan error
	auth      string
	mu        sync.Mutex
	lastTask  map[string]any
}

func startMCPHarness(t *testing.T) *mcpHarness {
	t.Helper()

	h := &mcpHarness{auth: "Bearer planner-token"}
	artifact := domain.Artifact{ID: "art-1", AttemptID: "attempt-1", Name: "stdout.log", Path: "/tmp/stdout.log"}

	h.daemon = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/instance":
			_ = json.NewEncoder(w).Encode(domain.InstanceInfo{
				ID:       "inst-1",
				RepoRoot: "/repo",
				BaseURL:  h.daemon.URL,
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/runs":
			_, _ = w.Write([]byte(`{"id":"run-1","project_id":"proj","state":"running"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/runs/run-1":
			_, _ = w.Write([]byte(`{"id":"run-1","project_id":"proj","state":"running"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/runs/run-1/steps":
			var payload map[string]any
			_ = json.NewDecoder(r.Body).Decode(&payload)
			h.mu.Lock()
			h.lastTask = payload
			h.mu.Unlock()
			_, _ = w.Write([]byte(`{"id":"step-1","phase_id":"phase-1","state":"running"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-1":
			_, _ = w.Write([]byte(`{"id":"step-1","phase_id":"phase-1","state":"completed"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/steps/step-1/wait":
			_, _ = w.Write([]byte(`{"step_id":"step-1","state":"completed","terminal":true,"timed_out":false}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-1/result":
			_, _ = w.Write([]byte(`{"version":"v1","run_id":"run-1","step_id":"step-1","state":"completed","summary":"done"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-1/validations":
			_, _ = w.Write([]byte(`[{"name":"tests","status":"passed"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-1/logs":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("step-log-output"))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/artifacts/art-1":
			_ = json.NewEncoder(w).Encode(artifact)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-1/artifacts":
			_ = json.NewEncoder(w).Encode([]domain.Artifact{artifact})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/gates/gate-1":
			_ = json.NewEncoder(w).Encode(domain.Gate{ID: "gate-1", RunID: "run-1", StepID: "step-1", Description: "pending", State: domain.GateStatePending})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/artifacts/art-1/content":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("artifact-content"))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/runs/run-1/gates":
			_ = json.NewEncoder(w).Encode([]domain.Gate{{ID: "gate-1", RunID: "run-1", StepID: "step-1", Description: "pending", State: domain.GateStatePending}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/gates/gate-1":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPatch && r.URL.Path == "/api/v1/runs/run-1":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/steps/step-1/retry":
			w.WriteHeader(http.StatusAccepted)
		default:
			http.NotFound(w, r)
		}
	}))

	store, err := relay.OpenStore(filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	server := relay.NewServer(&relay.Config{
		Host:             "127.0.0.1",
		Port:             0,
		DBPath:           filepath.Join(t.TempDir(), "relay-unused.db"),
		PlannerToken:     "planner-token",
		EnrollmentSecret: "enroll-secret",
	}, store)
	h.relayHTTP = httptest.NewServer(server.Handler())

	configPath := filepath.Join(t.TempDir(), "connector.json")
	cfg, err := connector.Enroll(context.Background(), h.relayHTTP.URL, h.daemon.URL, "enroll-secret", "test-connector", configPath)
	if err != nil {
		t.Fatal(err)
	}
	client := connector.NewClient(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	h.waitErr = make(chan error, 1)
	go func() { h.waitErr <- client.Run(ctx) }()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		req, _ := http.NewRequest(http.MethodGet, h.relayHTTP.URL+"/api/v2/instances", nil)
		req.Header.Set("Authorization", h.auth)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			var instances []map[string]any
			if json.Unmarshal(body, &instances) == nil && len(instances) == 1 {
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Cleanup(func() {
		cancel()
		select {
		case err := <-h.waitErr:
			if err != nil && err != context.Canceled && !strings.Contains(err.Error(), "closed network connection") {
				t.Fatalf("connector run failed: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("connector did not stop")
		}
		h.relayHTTP.Close()
		h.daemon.Close()
	})

	return h
}

func (h *mcpHarness) call(t *testing.T, auth string, method string, params any) map[string]any {
	t.Helper()
	return h.callPath(t, auth, http.MethodPost, "/mcp", map[string]string{
		"Content-Type": "application/json",
	}, map[string]any{
		"jsonrpc": "2.0",
		"id":      "req-1",
		"method":  method,
		"params":  params,
	})
}

func (h *mcpHarness) callPath(t *testing.T, auth, httpMethod, path string, headers map[string]string, payload any) map[string]any {
	t.Helper()

	var body io.Reader
	if payload != nil {
		data, _ := json.Marshal(payload)
		body = bytes.NewReader(data)
	}
	req, _ := http.NewRequest(httpMethod, h.relayHTTP.URL+path, body)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	payloadMap := map[string]any{
		"http_status": float64(resp.StatusCode),
	}
	if sessionID := resp.Header.Get("MCP-Session-Id"); sessionID != "" {
		payloadMap["session_id"] = sessionID
	}
	if protocolVersion := resp.Header.Get("MCP-Protocol-Version"); protocolVersion != "" {
		payloadMap["protocol_version"] = protocolVersion
	}
	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		payloadMap["content_type"] = contentType
	}
	if len(data) == 0 {
		return payloadMap
	}
	if err := json.Unmarshal(data, &payloadMap); err != nil {
		payloadMap["raw_body"] = string(data)
		return payloadMap
	}
	payloadMap["http_status"] = float64(resp.StatusCode)
	if sessionID := resp.Header.Get("MCP-Session-Id"); sessionID != "" {
		payloadMap["session_id"] = sessionID
	}
	if protocolVersion := resp.Header.Get("MCP-Protocol-Version"); protocolVersion != "" {
		payloadMap["protocol_version"] = protocolVersion
	}
	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		payloadMap["content_type"] = contentType
	}
	return payloadMap
}

func TestMCPToolsListIncludesRequiredCodencerTools(t *testing.T) {
	t.Parallel()

	h := startMCPHarness(t)
	response := h.call(t, h.auth, "tools/list", map[string]any{})
	result := response["result"].(map[string]any)
	tools := result["tools"].([]any)
	names := make(map[string]struct{})
	for _, item := range tools {
		tool := item.(map[string]any)
		names[tool["name"].(string)] = struct{}{}
	}
	required := []string{
		"codencer.list_instances",
		"codencer.get_instance",
		"codencer.start_run",
		"codencer.get_run",
		"codencer.list_run_gates",
		"codencer.submit_task",
		"codencer.get_step",
		"codencer.wait_step",
		"codencer.get_step_result",
		"codencer.list_step_artifacts",
		"codencer.get_step_logs",
		"codencer.get_artifact_content",
		"codencer.get_step_validations",
		"codencer.approve_gate",
		"codencer.reject_gate",
		"codencer.abort_run",
		"codencer.retry_step",
	}
	for _, name := range required {
		if _, ok := names[name]; !ok {
			t.Fatalf("expected tool %s in tools/list, got %v", name, names)
		}
	}
}

func TestMCPSubmitTaskUsesTaskSpecContract(t *testing.T) {
	t.Parallel()

	h := startMCPHarness(t)
	response := h.call(t, h.auth, "tools/call", map[string]any{
		"name": "codencer.submit_task",
		"arguments": map[string]any{
			"instance_id": "inst-1",
			"run_id":      "run-1",
			"task": map[string]any{
				"version":         "v1",
				"goal":            "Ship the fix",
				"adapter_profile": "codex",
				"allowed_paths":   []string{"internal/relay"},
				"timeout_seconds": 120,
			},
		},
	})
	result := response["result"].(map[string]any)
	if isError, _ := result["isError"].(bool); isError {
		t.Fatalf("unexpected MCP tool error: %+v", result)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.lastTask["goal"] != "Ship the fix" || h.lastTask["version"] != "v1" {
		t.Fatalf("expected TaskSpec payload to reach daemon, got %+v", h.lastTask)
	}
}

func TestMCPWaitStepAndArtifactContent(t *testing.T) {
	t.Parallel()

	h := startMCPHarness(t)
	waitResponse := h.call(t, h.auth, "tools/call", map[string]any{
		"name": "codencer.wait_step",
		"arguments": map[string]any{
			"step_id":        "step-1",
			"timeout_ms":     500,
			"interval_ms":    50,
			"include_result": false,
		},
	})
	waitResult := waitResponse["result"].(map[string]any)
	if isError, _ := waitResult["isError"].(bool); isError {
		t.Fatalf("unexpected wait_step error: %+v", waitResult)
	}
	waitStructured := waitResult["structuredContent"].(map[string]any)
	if waitStructured["terminal"] != true {
		t.Fatalf("expected terminal wait response, got %+v", waitStructured)
	}

	artifactResponse := h.call(t, h.auth, "tools/call", map[string]any{
		"name": "codencer.get_artifact_content",
		"arguments": map[string]any{
			"artifact_id": "art-1",
		},
	})
	artifactResult := artifactResponse["result"].(map[string]any)
	if isError, _ := artifactResult["isError"].(bool); isError {
		t.Fatalf("unexpected get_artifact_content error: %+v", artifactResult)
	}
	structured := artifactResult["structuredContent"].(map[string]any)
	if structured["encoding"] != "utf-8" || structured["text"] != "artifact-content" {
		t.Fatalf("unexpected artifact payload: %+v", structured)
	}
}

func TestMCPRunGatesAndStepLogs(t *testing.T) {
	t.Parallel()

	h := startMCPHarness(t)
	gatesResponse := h.call(t, h.auth, "tools/call", map[string]any{
		"name": "codencer.list_run_gates",
		"arguments": map[string]any{
			"instance_id": "inst-1",
			"run_id":      "run-1",
		},
	})
	gatesResult := gatesResponse["result"].(map[string]any)
	if isError, _ := gatesResult["isError"].(bool); isError {
		t.Fatalf("unexpected list_run_gates error: %+v", gatesResult)
	}
	gates := gatesResult["structuredContent"].([]any)
	if len(gates) != 1 {
		t.Fatalf("expected one gate, got %+v", gates)
	}

	logsResponse := h.call(t, h.auth, "tools/call", map[string]any{
		"name": "codencer.get_step_logs",
		"arguments": map[string]any{
			"step_id": "step-1",
		},
	})
	logsResult := logsResponse["result"].(map[string]any)
	if isError, _ := logsResult["isError"].(bool); isError {
		t.Fatalf("unexpected get_step_logs error: %+v", logsResult)
	}
	logs := logsResult["structuredContent"].(map[string]any)
	if logs["encoding"] != "utf-8" || logs["text"] != "step-log-output" {
		t.Fatalf("unexpected step logs payload: %+v", logs)
	}
}

func TestMCPInitializeStreamAndCompatibilityPath(t *testing.T) {
	t.Parallel()

	h := startMCPHarness(t)
	initialize := h.callPath(t, h.auth, http.MethodPost, "/mcp", map[string]string{
		"Content-Type":         "application/json",
		"MCP-Protocol-Version": "2025-11-25",
	}, map[string]any{
		"jsonrpc": "2.0",
		"id":      "req-init",
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-11-25",
		},
	})
	if initialize["protocol_version"] != "2025-11-25" {
		t.Fatalf("expected negotiated protocol version, got %+v", initialize)
	}
	sessionID, _ := initialize["session_id"].(string)
	if sessionID == "" {
		t.Fatalf("expected initialize to return session id, got %+v", initialize)
	}

	stream := h.callPath(t, h.auth, http.MethodGet, "/mcp", map[string]string{
		"MCP-Session-Id":       sessionID,
		"MCP-Protocol-Version": "2025-11-25",
	}, nil)
	if status := int(stream["http_status"].(float64)); status != http.StatusOK {
		t.Fatalf("expected GET /mcp success, got %+v", stream)
	}
	contentType, _ := stream["content_type"].(string)
	if !strings.Contains(contentType, "text/event-stream") {
		t.Fatalf("expected SSE content type, got %+v", stream)
	}
	rawBody, _ := stream["raw_body"].(string)
	if !strings.Contains(rawBody, "codencer-relay-mcp-stream") {
		t.Fatalf("expected SSE bootstrap payload, got %+v", stream)
	}

	compat := h.callPath(t, h.auth, http.MethodPost, "/mcp/call", map[string]string{
		"Content-Type":         "application/json",
		"MCP-Session-Id":       sessionID,
		"MCP-Protocol-Version": "2025-11-25",
	}, map[string]any{
		"jsonrpc": "2.0",
		"id":      "req-tools",
		"method":  "tools/list",
		"params":  map[string]any{},
	})
	if status := int(compat["http_status"].(float64)); status != http.StatusOK {
		t.Fatalf("expected /mcp/call compatibility success, got %+v", compat)
	}

	deleted := h.callPath(t, h.auth, http.MethodDelete, "/mcp", map[string]string{
		"MCP-Session-Id":       sessionID,
		"MCP-Protocol-Version": "2025-11-25",
	}, nil)
	if status := int(deleted["http_status"].(float64)); status != http.StatusNoContent {
		t.Fatalf("expected session delete success, got %+v", deleted)
	}
}

func TestMCPOriginHandling(t *testing.T) {
	t.Parallel()

	store, err := relay.OpenStore(filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	server := relay.NewServer(&relay.Config{
		DBPath:         filepath.Join(t.TempDir(), "relay-auth.db"),
		PlannerToken:   "planner-token",
		AllowedOrigins: []string{"https://planner.example"},
	}, store)
	relayHTTP := httptest.NewServer(server.Handler())
	defer relayHTTP.Close()

	body := map[string]any{
		"jsonrpc": "2.0",
		"id":      "req-1",
		"method":  "tools/list",
		"params":  map[string]any{},
	}
	allowed := map[string]string{
		"Authorization": "Bearer planner-token",
		"Content-Type":  "application/json",
		"Origin":        "https://planner.example",
	}
	allowedResp := (&mcpHarness{relayHTTP: relayHTTP}).callPath(t, "", http.MethodPost, "/mcp", allowed, body)
	if status := int(allowedResp["http_status"].(float64)); status != http.StatusOK {
		t.Fatalf("expected allowed origin success, got %+v", allowedResp)
	}

	blocked := map[string]string{
		"Authorization": "Bearer planner-token",
		"Content-Type":  "application/json",
		"Origin":        "https://blocked.example",
	}
	blockedResp := (&mcpHarness{relayHTTP: relayHTTP}).callPath(t, "", http.MethodPost, "/mcp", blocked, body)
	if status := int(blockedResp["http_status"].(float64)); status != http.StatusForbidden {
		t.Fatalf("expected blocked origin failure, got %+v", blockedResp)
	}
}

func TestMCPAuthAndMalformedInput(t *testing.T) {
	t.Parallel()

	h := startMCPHarness(t)

	unauthorized := h.call(t, "", "tools/list", map[string]any{})
	if status := int(unauthorized["http_status"].(float64)); status != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing auth, got %+v", unauthorized)
	}

	store, err := relay.OpenStore(filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server := relay.NewServer(&relay.Config{
		DBPath: filepath.Join(t.TempDir(), "relay-auth.db"),
		PlannerTokens: []relay.PlannerTokenConfig{{
			Name:   "read-only",
			Token:  "read-token",
			Scopes: []string{"instances:read"},
		}},
	}, store)
	relayHTTP := httptest.NewServer(server.Handler())
	defer relayHTTP.Close()

	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      "req-1",
		"method":  "tools/call",
		"params": map[string]any{
			"name": "codencer.start_run",
			"arguments": map[string]any{
				"instance_id": "inst-1",
				"payload": map[string]any{
					"project_id": "proj",
				},
			},
		},
	})
	req, _ := http.NewRequest(http.MethodPost, relayHTTP.URL+"/mcp", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer read-token")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var scoped map[string]any
	data, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(data, &scoped)
	result := scoped["result"].(map[string]any)
	if result["isError"] != true {
		t.Fatalf("expected scope-denied MCP tool result, got %+v", scoped)
	}

	malformed := h.call(t, h.auth, "tools/call", map[string]any{
		"name":      "codencer.submit_task",
		"arguments": map[string]any{"instance_id": "inst-1", "run_id": "run-1"},
	})
	malformedResult := malformed["result"].(map[string]any)
	if malformedResult["isError"] != true {
		t.Fatalf("expected malformed_request result, got %+v", malformed)
	}
}

func TestMCPRetryStepRejectsWrongInstance(t *testing.T) {
	t.Parallel()

	h := startMCPHarness(t)
	h.call(t, h.auth, "tools/call", map[string]any{
		"name": "codencer.submit_task",
		"arguments": map[string]any{
			"instance_id": "inst-1",
			"run_id":      "run-1",
			"task": map[string]any{
				"version": "v1",
				"goal":    "Seed step route",
			},
		},
	})

	response := h.call(t, h.auth, "tools/call", map[string]any{
		"name": "codencer.retry_step",
		"arguments": map[string]any{
			"instance_id": "inst-2",
			"step_id":     "step-1",
		},
	})

	result := response["result"].(map[string]any)
	if result["isError"] != true {
		t.Fatalf("expected retry_step to reject wrong instance, got %+v", response)
	}
	structured := result["structuredContent"].(map[string]any)
	errPayload := structured["error"].(map[string]any)
	if errPayload["code"] != "instance_denied" && errPayload["code"] != "instance_not_found" {
		t.Fatalf("expected instance_denied, got %+v", errPayload)
	}
}
