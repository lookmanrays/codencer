package cloud

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

type cloudMCPHarness struct {
	cloudHTTP *httptest.Server
	relayHTTP *httptest.Server
	daemon    *httptest.Server
	cancel    context.CancelFunc
	waitErr   chan error
	auth      string
	mu        sync.Mutex
	lastTask  map[string]any
}

func startCloudMCPHarness(t *testing.T) *cloudMCPHarness {
	t.Helper()

	h := &cloudMCPHarness{}
	artifact := domain.Artifact{ID: "art-1", AttemptID: "attempt-1", Name: "stdout.log", Path: "/tmp/stdout.log"}

	h.daemon = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/instance":
			_ = json.NewEncoder(w).Encode(domain.InstanceInfo{ID: "inst-1", RepoRoot: "/repo", BaseURL: h.daemon.URL})
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
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-1/artifacts":
			_ = json.NewEncoder(w).Encode([]domain.Artifact{artifact})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/artifacts/art-1/content":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("artifact-content"))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/runs/run-1/gates":
			_ = json.NewEncoder(w).Encode([]domain.Gate{{ID: "gate-1", RunID: "run-1", StepID: "step-1", Description: "pending", State: domain.GateStatePending}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/gates/gate-1":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))

	relayStore, err := relay.OpenStore(filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatal(err)
	}
	relayServer := relay.NewServer(&relay.Config{
		Host:             "127.0.0.1",
		Port:             0,
		DBPath:           filepath.Join(t.TempDir(), "relay-unused.db"),
		PlannerToken:     "planner-token",
		EnrollmentSecret: "enroll-secret",
	}, relayStore)
	h.relayHTTP = httptest.NewServer(relayServer.Handler())

	cfgPath := filepath.Join(t.TempDir(), "connector.json")
	cfg, err := connector.Enroll(context.Background(), h.relayHTTP.URL, h.daemon.URL, "enroll-secret", "test-connector", cfgPath)
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
		req.Header.Set("Authorization", "Bearer planner-token")
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

	cloudStore, err := OpenStore(filepath.Join(t.TempDir(), "cloud.db"), "cloud-master-key")
	if err != nil {
		t.Fatal(err)
	}
	org, workspace, project := seedCloudScope(t, cloudStore, context.Background())
	member, err := cloudStore.CreateMembership(context.Background(), Membership{
		OrgID:       org.ID,
		WorkspaceID: workspace.ID,
		ProjectID:   project.ID,
		Name:        "Operator",
		Role:        RoleOrgOwner,
	})
	if err != nil {
		t.Fatal(err)
	}
	rawToken, err := GenerateAPIToken()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cloudStore.CreateAPIToken(context.Background(), APIToken{
		OrgID:        org.ID,
		WorkspaceID:  workspace.ID,
		ProjectID:    project.ID,
		MembershipID: member.ID,
		Name:         "cloud-operator",
		SubjectType:  "membership",
		SubjectName:  member.Name,
		Scopes: []string{
			"runtime_instances:read",
			"runtime_connectors:read", "runtime_connectors:write",
			"runs:read", "runs:write",
			"steps:read", "steps:write",
			"artifacts:read",
			"gates:read", "gates:write",
		},
	}, rawToken); err != nil {
		t.Fatal(err)
	}
	h.auth = "Bearer " + rawToken

	cloudServer := NewServer(DefaultConfig(), cloudStore, nil, &RelayRuntime{Server: relayServer, Store: relayStore})
	h.cloudHTTP = httptest.NewServer(cloudServer.Handler())

	claimBody, _ := json.Marshal(map[string]any{
		"org_id":       org.ID,
		"workspace_id": workspace.ID,
		"project_id":   project.ID,
		"connector_id": cfg.ConnectorID,
	})
	req, _ := http.NewRequest(http.MethodPost, h.cloudHTTP.URL+"/api/cloud/v1/runtime/connectors", bytes.NewReader(claimBody))
	req.Header.Set("Authorization", h.auth)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected runtime connector claim created, got %d body=%s", resp.StatusCode, string(body))
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
		h.cloudHTTP.Close()
		h.relayHTTP.Close()
		h.daemon.Close()
		_ = cloudStore.Close()
		_ = relayStore.Close()
	})

	return h
}

func (h *cloudMCPHarness) call(t *testing.T, method string, params any, extraHeaders map[string]string) map[string]any {
	t.Helper()
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      "req-1",
		"method":  method,
		"params":  params,
	}
	data, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, h.cloudHTTP.URL+"/api/cloud/v1/mcp", bytes.NewReader(data))
	req.Header.Set("Authorization", h.auth)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for key, value := range extraHeaders {
		req.Header.Set(key, value)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	out := map[string]any{
		"http_status": float64(resp.StatusCode),
	}
	if sessionID := resp.Header.Get("MCP-Session-Id"); sessionID != "" {
		out["session_id"] = sessionID
	}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &out); err != nil {
			out["raw"] = string(body)
		}
		out["http_status"] = float64(resp.StatusCode)
		if sessionID := resp.Header.Get("MCP-Session-Id"); sessionID != "" {
			out["session_id"] = sessionID
		}
	}
	return out
}

func TestCloudMCPSurfaceRuntimeFlow(t *testing.T) {
	h := startCloudMCPHarness(t)

	initResp := h.call(t, "initialize", map[string]any{"protocolVersion": "2025-11-25"}, nil)
	if initResp["http_status"].(float64) != http.StatusOK {
		t.Fatalf("expected initialize ok, got %+v", initResp)
	}
	sessionID, _ := initResp["session_id"].(string)
	if sessionID == "" {
		t.Fatalf("expected session id in initialize response: %+v", initResp)
	}

	toolsResp := h.call(t, "tools/list", map[string]any{}, map[string]string{
		"MCP-Session-Id":       sessionID,
		"MCP-Protocol-Version": "2025-11-25",
	})
	if toolsResp["http_status"].(float64) != http.StatusOK {
		t.Fatalf("expected tools/list ok, got %+v", toolsResp)
	}

	listResp := h.call(t, "tools/call", map[string]any{
		"name":      "codencer.list_instances",
		"arguments": map[string]any{},
	}, map[string]string{
		"MCP-Session-Id":       sessionID,
		"MCP-Protocol-Version": "2025-11-25",
	})
	result := listResp["result"].(map[string]any)
	structured := result["structuredContent"].([]any)
	if len(structured) != 1 {
		t.Fatalf("expected one cloud runtime instance, got %+v", structured)
	}
	instanceID := structured[0].(map[string]any)["instance_id"].(string)

	runResp := h.call(t, "tools/call", map[string]any{
		"name": "codencer.start_run",
		"arguments": map[string]any{
			"instance_id": instanceID,
			"payload": map[string]any{
				"project_id": "proj",
			},
		},
	}, map[string]string{"MCP-Session-Id": sessionID, "MCP-Protocol-Version": "2025-11-25"})
	if runResp["http_status"].(float64) != http.StatusOK {
		t.Fatalf("expected start_run ok, got %+v", runResp)
	}

	submitResp := h.call(t, "tools/call", map[string]any{
		"name": "codencer.submit_task",
		"arguments": map[string]any{
			"instance_id": instanceID,
			"run_id":      "run-1",
			"task": map[string]any{
				"version": "v1",
				"goal":    "Do the thing",
			},
		},
	}, map[string]string{"MCP-Session-Id": sessionID, "MCP-Protocol-Version": "2025-11-25"})
	if submitResp["http_status"].(float64) != http.StatusOK {
		t.Fatalf("expected submit_task ok, got %+v", submitResp)
	}
	h.mu.Lock()
	gotGoal, _ := h.lastTask["goal"].(string)
	h.mu.Unlock()
	if gotGoal != "Do the thing" {
		t.Fatalf("expected task goal to reach daemon, got %+v", h.lastTask)
	}

	waitResp := h.call(t, "tools/call", map[string]any{
		"name":      "codencer.wait_step",
		"arguments": map[string]any{"step_id": "step-1", "timeout_ms": 1000},
	}, map[string]string{"MCP-Session-Id": sessionID, "MCP-Protocol-Version": "2025-11-25"})
	if waitResp["http_status"].(float64) != http.StatusOK {
		t.Fatalf("expected wait_step ok, got %+v", waitResp)
	}

	resultResp := h.call(t, "tools/call", map[string]any{
		"name":      "codencer.get_step_result",
		"arguments": map[string]any{"step_id": "step-1"},
	}, map[string]string{"MCP-Session-Id": sessionID, "MCP-Protocol-Version": "2025-11-25"})
	if resultResp["http_status"].(float64) != http.StatusOK {
		t.Fatalf("expected get_step_result ok, got %+v", resultResp)
	}
}
