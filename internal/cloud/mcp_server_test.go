package cloud

import (
	"bufio"
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
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type cloudMCPHarness struct {
	cloudHTTP  *httptest.Server
	relayHTTP  *httptest.Server
	daemon     *httptest.Server
	cloudStore *Store
	cancel     context.CancelFunc
	waitErr    chan error
	auth       string
	rawToken   string
	token      APIToken
	org        *Org
	workspace  *Workspace
	project    *Project
	mu         sync.Mutex
	lastTask   map[string]any
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
	h.cloudStore = cloudStore

	org, workspace, project := seedCloudScope(t, cloudStore, context.Background())
	h.org = org
	h.workspace = workspace
	h.project = project
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
	record, err := cloudStore.CreateAPIToken(context.Background(), APIToken{
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
	}, rawToken)
	if err != nil {
		t.Fatal(err)
	}
	h.token = *record
	h.rawToken = rawToken
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

func (h *cloudMCPHarness) createScopedToken(t *testing.T, orgID, workspaceID, projectID string, scopes []string) (APIToken, string) {
	t.Helper()
	rawToken, err := GenerateAPIToken()
	if err != nil {
		t.Fatal(err)
	}
	record, err := h.cloudStore.CreateAPIToken(context.Background(), APIToken{
		OrgID:       orgID,
		WorkspaceID: workspaceID,
		ProjectID:   projectID,
		Name:        "scoped-mcp-token",
		Kind:        "project",
		Scopes:      scopes,
	}, rawToken)
	if err != nil {
		t.Fatal(err)
	}
	return *record, rawToken
}

func (h *cloudMCPHarness) call(t *testing.T, auth, method string, params any, extraHeaders map[string]string) map[string]any {
	t.Helper()
	return h.callPath(t, auth, http.MethodPost, "/api/cloud/v1/mcp", mergeHeaders(map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json, text/event-stream",
	}, extraHeaders), map[string]any{
		"jsonrpc": "2.0",
		"id":      "req-1",
		"method":  method,
		"params":  params,
	})
}

func (h *cloudMCPHarness) callPath(t *testing.T, auth, httpMethod, path string, headers map[string]string, payload any) map[string]any {
	t.Helper()

	var body io.Reader
	if payload != nil {
		data, _ := json.Marshal(payload)
		body = bytes.NewReader(data)
	}
	req, _ := http.NewRequest(httpMethod, h.cloudHTTP.URL+path, body)
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
	response := map[string]any{
		"http_status": float64(resp.StatusCode),
	}
	if sessionID := resp.Header.Get(mcpHeaderSessionID); sessionID != "" {
		response["session_id"] = sessionID
	}
	if protocolVersion := resp.Header.Get(mcpHeaderProtocolVersion); protocolVersion != "" {
		response["protocol_version"] = protocolVersion
	}
	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		response["content_type"] = contentType
	}
	if allowOrigin := resp.Header.Get("Access-Control-Allow-Origin"); allowOrigin != "" {
		response["allow_origin"] = allowOrigin
	}
	if len(data) == 0 {
		return response
	}
	if err := json.Unmarshal(data, &response); err != nil {
		response["raw_body"] = string(data)
		return response
	}
	response["http_status"] = float64(resp.StatusCode)
	if sessionID := resp.Header.Get(mcpHeaderSessionID); sessionID != "" {
		response["session_id"] = sessionID
	}
	if protocolVersion := resp.Header.Get(mcpHeaderProtocolVersion); protocolVersion != "" {
		response["protocol_version"] = protocolVersion
	}
	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		response["content_type"] = contentType
	}
	if allowOrigin := resp.Header.Get("Access-Control-Allow-Origin"); allowOrigin != "" {
		response["allow_origin"] = allowOrigin
	}
	return response
}

func (h *cloudMCPHarness) openStream(t *testing.T, auth, sessionID string, extraHeaders map[string]string) (*http.Response, *bufio.Reader) {
	t.Helper()

	req, _ := http.NewRequest(http.MethodGet, h.cloudHTTP.URL+"/api/cloud/v1/mcp", nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	req.Header.Set(mcpHeaderSessionID, sessionID)
	req.Header.Set(mcpHeaderProtocolVersion, "2025-11-25")
	req.Header.Set("Accept", "text/event-stream")
	for key, value := range extraHeaders {
		req.Header.Set(key, value)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp, bufio.NewReader(resp.Body)
}

func TestCloudMCPSurfaceRuntimeFlow(t *testing.T) {
	t.Parallel()

	h := startCloudMCPHarness(t)
	initResp := h.call(t, h.auth, "initialize", map[string]any{"protocolVersion": "2025-11-25"}, map[string]string{
		mcpHeaderProtocolVersion: "2025-11-25",
	})
	if mcpHTTPStatus(initResp) != http.StatusOK {
		t.Fatalf("expected initialize ok, got %+v", initResp)
	}
	sessionID, _ := initResp["session_id"].(string)
	if sessionID == "" {
		t.Fatalf("expected session id in initialize response: %+v", initResp)
	}

	listResp := h.callPath(t, h.auth, http.MethodPost, "/api/cloud/v1/mcp/call", map[string]string{
		"Content-Type":           "application/json",
		"Accept":                 "application/json, text/event-stream",
		mcpHeaderSessionID:       sessionID,
		mcpHeaderProtocolVersion: "2025-11-25",
	}, map[string]any{
		"jsonrpc":   "2.0",
		"id":        "req-list",
		"name":      "codencer.list_instances",
		"arguments": map[string]any{},
	})
	if mcpHTTPStatus(listResp) != http.StatusOK {
		t.Fatalf("expected list_instances alias ok, got %+v", listResp)
	}
	listResult := requireMCPToolSuccess(t, listResp)
	structured := listResult["structuredContent"].([]any)
	if len(structured) != 1 {
		t.Fatalf("expected one cloud runtime instance, got %+v", structured)
	}
	instanceID := structured[0].(map[string]any)["instance_id"].(string)

	runResp := h.call(t, h.auth, "tools/call", map[string]any{
		"name": "codencer.start_run",
		"arguments": map[string]any{
			"instance_id": instanceID,
			"payload": map[string]any{
				"project_id": "proj",
			},
		},
	}, map[string]string{
		mcpHeaderSessionID:       sessionID,
		mcpHeaderProtocolVersion: "2025-11-25",
	})
	if mcpHTTPStatus(runResp) != http.StatusOK {
		t.Fatalf("expected start_run ok, got %+v", runResp)
	}

	submitResp := h.call(t, h.auth, "tools/call", map[string]any{
		"name": "codencer.submit_task",
		"arguments": map[string]any{
			"instance_id": instanceID,
			"run_id":      "run-1",
			"task": map[string]any{
				"version": "v1",
				"goal":    "Do the thing",
			},
		},
	}, map[string]string{
		mcpHeaderSessionID:       sessionID,
		mcpHeaderProtocolVersion: "2025-11-25",
	})
	if mcpHTTPStatus(submitResp) != http.StatusOK {
		t.Fatalf("expected submit_task ok, got %+v", submitResp)
	}
	h.mu.Lock()
	gotGoal, _ := h.lastTask["goal"].(string)
	h.mu.Unlock()
	if gotGoal != "Do the thing" {
		t.Fatalf("expected task goal to reach daemon, got %+v", h.lastTask)
	}

	waitResp := h.call(t, h.auth, "tools/call", map[string]any{
		"name":      "codencer.wait_step",
		"arguments": map[string]any{"step_id": "step-1", "timeout_ms": 1000},
	}, map[string]string{
		mcpHeaderSessionID:       sessionID,
		mcpHeaderProtocolVersion: "2025-11-25",
	})
	if mcpHTTPStatus(waitResp) != http.StatusOK {
		t.Fatalf("expected wait_step ok, got %+v", waitResp)
	}

	resultResp := h.call(t, h.auth, "tools/call", map[string]any{
		"name":      "codencer.get_step_result",
		"arguments": map[string]any{"step_id": "step-1"},
	}, map[string]string{
		mcpHeaderSessionID:       sessionID,
		mcpHeaderProtocolVersion: "2025-11-25",
	})
	if mcpHTTPStatus(resultResp) != http.StatusOK {
		t.Fatalf("expected get_step_result ok, got %+v", resultResp)
	}

	logsResp := h.call(t, h.auth, "tools/call", map[string]any{
		"name":      "codencer.get_step_logs",
		"arguments": map[string]any{"step_id": "step-1"},
	}, map[string]string{
		mcpHeaderSessionID:       sessionID,
		mcpHeaderProtocolVersion: "2025-11-25",
	})
	logs := requireMCPToolSuccess(t, logsResp)["structuredContent"].(map[string]any)
	if logs["encoding"] != "utf-8" || logs["text"] != "step-log-output" {
		t.Fatalf("unexpected logs payload: %+v", logs)
	}

	artifactListResp := h.call(t, h.auth, "tools/call", map[string]any{
		"name":      "codencer.list_step_artifacts",
		"arguments": map[string]any{"step_id": "step-1"},
	}, map[string]string{
		mcpHeaderSessionID:       sessionID,
		mcpHeaderProtocolVersion: "2025-11-25",
	})
	artifacts := requireMCPToolSuccess(t, artifactListResp)["structuredContent"].([]any)
	if len(artifacts) != 1 {
		t.Fatalf("expected one artifact, got %+v", artifacts)
	}

	artifactResp := h.call(t, h.auth, "tools/call", map[string]any{
		"name": "codencer.get_artifact_content",
		"arguments": map[string]any{
			"artifact_id": "art-1",
		},
	}, map[string]string{
		mcpHeaderSessionID:       sessionID,
		mcpHeaderProtocolVersion: "2025-11-25",
	})
	artifactPayload := requireMCPToolSuccess(t, artifactResp)["structuredContent"].(map[string]any)
	if artifactPayload["encoding"] != "utf-8" || artifactPayload["text"] != "artifact-content" {
		t.Fatalf("unexpected artifact payload: %+v", artifactPayload)
	}
}

func TestCloudMCPInitializeStreamAndCompatibilityPath(t *testing.T) {
	t.Parallel()

	h := startCloudMCPHarness(t)
	initialize := h.callPath(t, h.auth, http.MethodPost, "/api/cloud/v1/mcp", map[string]string{
		"Content-Type":           "application/json",
		mcpHeaderProtocolVersion: "2025-11-25",
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

	resp, reader := h.openStream(t, h.auth, sessionID, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected GET /api/cloud/v1/mcp success, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("expected SSE content type, got %q", got)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("expected bootstrap SSE line, got error: %v", err)
	}
	if !strings.Contains(line, "codencer-cloud-mcp-stream") {
		t.Fatalf("expected cloud SSE bootstrap payload, got %q", line)
	}
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("expected SSE separator line, got error: %v", err)
	}

	compat := h.callPath(t, h.auth, http.MethodPost, "/api/cloud/v1/mcp/call", map[string]string{
		"Content-Type":           "application/json",
		"Accept":                 "application/json, text/event-stream",
		mcpHeaderSessionID:       sessionID,
		mcpHeaderProtocolVersion: "2025-11-25",
	}, map[string]any{
		"jsonrpc": "2.0",
		"id":      "req-tools",
		"method":  "tools/list",
		"params":  map[string]any{},
	})
	if mcpHTTPStatus(compat) != http.StatusOK {
		t.Fatalf("expected /api/cloud/v1/mcp/call compatibility success, got %+v", compat)
	}

	deleted := h.callPath(t, h.auth, http.MethodDelete, "/api/cloud/v1/mcp", map[string]string{
		mcpHeaderSessionID:       sessionID,
		mcpHeaderProtocolVersion: "2025-11-25",
	}, nil)
	if mcpHTTPStatus(deleted) != http.StatusNoContent {
		t.Fatalf("expected session delete success, got %+v", deleted)
	}
	done := make(chan error, 1)
	go func() {
		_, err := io.Copy(io.Discard, resp.Body)
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected stream to close cleanly after DELETE, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SSE stream to close after DELETE")
	}
	_ = resp.Body.Close()
}

func TestCloudMCPOriginHandling(t *testing.T) {
	t.Parallel()

	h := startCloudMCPHarness(t)
	body := map[string]any{
		"jsonrpc": "2.0",
		"id":      "req-1",
		"method":  "tools/list",
		"params":  map[string]any{},
	}

	allowed := h.callPath(t, h.auth, http.MethodPost, "/api/cloud/v1/mcp", map[string]string{
		"Content-Type": "application/json",
		"Origin":       "http://127.0.0.1:3000",
	}, body)
	if mcpHTTPStatus(allowed) != http.StatusOK {
		t.Fatalf("expected allowed origin success, got %+v", allowed)
	}
	if allowed["allow_origin"] != "http://127.0.0.1:3000" {
		t.Fatalf("expected allow-origin header, got %+v", allowed)
	}

	blocked := h.callPath(t, h.auth, http.MethodPost, "/api/cloud/v1/mcp", map[string]string{
		"Content-Type": "application/json",
		"Origin":       "https://blocked.example",
	}, body)
	if mcpHTTPStatus(blocked) != http.StatusForbidden {
		t.Fatalf("expected blocked origin failure, got %+v", blocked)
	}
}

func TestCloudMCPScopedStepReadParityAndSessionBinding(t *testing.T) {
	t.Parallel()

	h := startCloudMCPHarness(t)
	_, scopedRaw := h.createScopedToken(t, h.org.ID, h.workspace.ID, h.project.ID, []string{"steps:read"})
	scopedAuth := "Bearer " + scopedRaw

	httpReq, _ := http.NewRequest(http.MethodGet, h.cloudHTTP.URL+"/api/cloud/v1/runtime/instances/inst-1/steps/step-1", nil)
	httpReq.Header.Set("Authorization", scopedAuth)
	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		t.Fatal(err)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		t.Fatalf("expected scoped HTTP get_step success, got %d body=%s", httpResp.StatusCode, string(body))
	}

	initResp := h.call(t, scopedAuth, "initialize", map[string]any{"protocolVersion": "2025-11-25"}, map[string]string{
		mcpHeaderProtocolVersion: "2025-11-25",
	})
	if mcpHTTPStatus(initResp) != http.StatusOK {
		t.Fatalf("expected scoped MCP initialize success, got %+v", initResp)
	}
	sessionID, _ := initResp["session_id"].(string)
	if sessionID == "" {
		t.Fatalf("expected scoped MCP session id, got %+v", initResp)
	}

	listResp := h.call(t, scopedAuth, "tools/call", map[string]any{
		"name":      "codencer.list_instances",
		"arguments": map[string]any{},
	}, map[string]string{
		mcpHeaderSessionID:       sessionID,
		mcpHeaderProtocolVersion: "2025-11-25",
	})
	listErr := requireMCPToolError(t, listResp)
	if listErr["code"] != "scope_denied" {
		t.Fatalf("expected scope_denied for list_instances, got %+v", listErr)
	}

	getStepResp := h.call(t, scopedAuth, "tools/call", map[string]any{
		"name":      "codencer.get_step",
		"arguments": map[string]any{"step_id": "step-1"},
	}, map[string]string{
		mcpHeaderSessionID:       sessionID,
		mcpHeaderProtocolVersion: "2025-11-25",
	})
	if mcpHTTPStatus(getStepResp) != http.StatusOK {
		t.Fatalf("expected scoped MCP get_step success, got %+v", getStepResp)
	}

	_, otherRaw := h.createScopedToken(t, h.org.ID, h.workspace.ID, h.project.ID, []string{"steps:read"})
	deleteResp := h.callPath(t, "Bearer "+otherRaw, http.MethodDelete, "/api/cloud/v1/mcp", map[string]string{
		mcpHeaderSessionID:       sessionID,
		mcpHeaderProtocolVersion: "2025-11-25",
	}, nil)
	if mcpHTTPStatus(deleteResp) != http.StatusNotFound {
		t.Fatalf("expected session-bound DELETE rejection, got %+v", deleteResp)
	}
}

func TestCloudMCPRevokedTokenRejected(t *testing.T) {
	t.Parallel()

	h := startCloudMCPHarness(t)
	initResp := h.call(t, h.auth, "initialize", map[string]any{"protocolVersion": "2025-11-25"}, nil)
	if mcpHTTPStatus(initResp) != http.StatusOK {
		t.Fatalf("expected initialize ok before revoke, got %+v", initResp)
	}
	sessionID, _ := initResp["session_id"].(string)
	if err := h.cloudStore.RevokeAPIToken(context.Background(), h.token.ID); err != nil {
		t.Fatal(err)
	}

	revokedInit := h.call(t, h.auth, "initialize", map[string]any{"protocolVersion": "2025-11-25"}, nil)
	if mcpHTTPStatus(revokedInit) != http.StatusUnauthorized {
		t.Fatalf("expected revoked initialize to fail, got %+v", revokedInit)
	}

	streamResp, _ := h.openStream(t, h.auth, sessionID, nil)
	defer streamResp.Body.Close()
	if streamResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected revoked stream to fail, got %d", streamResp.StatusCode)
	}

	deleteResp := h.callPath(t, h.auth, http.MethodDelete, "/api/cloud/v1/mcp", map[string]string{
		mcpHeaderSessionID:       sessionID,
		mcpHeaderProtocolVersion: "2025-11-25",
	}, nil)
	if mcpHTTPStatus(deleteResp) != http.StatusUnauthorized {
		t.Fatalf("expected revoked delete to fail, got %+v", deleteResp)
	}
}

func TestCloudMCPOfficialGoSDKInterop(t *testing.T) {
	t.Parallel()

	h := startCloudMCPHarness(t)
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "codencer-cloud-sdk-smoke",
		Version: "1.0.0",
	}, nil)
	httpClient := &http.Client{
		Transport: cloudMCPAuthRoundTripper{authorization: h.auth},
	}
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint:   h.cloudHTTP.URL + "/api/cloud/v1/mcp",
		HTTPClient: httpClient,
	}, nil)
	if err != nil {
		t.Fatalf("client.Connect() failed: %v", err)
	}
	defer func() {
		if err := session.Close(); err != nil {
			t.Fatalf("session.Close() failed: %v", err)
		}
	}()

	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools() failed: %v", err)
	}
	if len(tools.Tools) == 0 {
		t.Fatal("expected official SDK client to see cloud tools")
	}

	instancesResult, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "codencer.list_instances",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("list_instances failed: %v", err)
	}
	instances, ok := instancesResult.StructuredContent.([]any)
	if !ok || len(instances) == 0 {
		t.Fatalf("expected list_instances structured content, got %+v", instancesResult.StructuredContent)
	}
	firstInstance, ok := instances[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected instance payload: %+v", instances[0])
	}
	instanceID, _ := firstInstance["instance_id"].(string)
	if instanceID == "" {
		t.Fatalf("missing instance_id in %+v", firstInstance)
	}

	if _, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "codencer.start_run",
		Arguments: map[string]any{
			"instance_id": instanceID,
			"payload": map[string]any{
				"id":         "run-1",
				"project_id": "sdk-project",
			},
		},
	}); err != nil {
		t.Fatalf("start_run failed: %v", err)
	}

	submitResult, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "codencer.submit_task",
		Arguments: map[string]any{
			"instance_id": instanceID,
			"run_id":      "run-1",
			"task": map[string]any{
				"version": "v1",
				"goal":    "Verify cloud SDK interoperability",
			},
		},
	})
	if err != nil {
		t.Fatalf("submit_task failed: %v", err)
	}
	submitted, ok := submitResult.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("unexpected submit_task payload: %+v", submitResult.StructuredContent)
	}
	stepID, _ := submitted["id"].(string)
	if stepID == "" {
		t.Fatalf("missing step id in submit_task payload: %+v", submitted)
	}

	waitResult, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "codencer.wait_step",
		Arguments: map[string]any{
			"step_id":     stepID,
			"timeout_ms":  750,
			"interval_ms": 50,
		},
	})
	if err != nil {
		t.Fatalf("wait_step failed: %v", err)
	}
	waitPayload, ok := waitResult.StructuredContent.(map[string]any)
	if !ok || waitPayload["terminal"] != true {
		t.Fatalf("unexpected wait_step payload: %+v", waitResult.StructuredContent)
	}

	stepResult, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "codencer.get_step_result",
		Arguments: map[string]any{
			"step_id": stepID,
		},
	})
	if err != nil {
		t.Fatalf("get_step_result failed: %v", err)
	}
	resultPayload, ok := stepResult.StructuredContent.(map[string]any)
	if !ok || resultPayload["summary"] != "done" {
		t.Fatalf("unexpected get_step_result payload: %+v", stepResult.StructuredContent)
	}

	logsResult, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "codencer.get_step_logs",
		Arguments: map[string]any{
			"step_id": stepID,
		},
	})
	if err != nil {
		t.Fatalf("get_step_logs failed: %v", err)
	}
	logsPayload, ok := logsResult.StructuredContent.(map[string]any)
	if !ok || logsPayload["text"] != "step-log-output" {
		t.Fatalf("unexpected logs payload: %+v", logsResult.StructuredContent)
	}

	artifactsResult, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "codencer.list_step_artifacts",
		Arguments: map[string]any{
			"step_id": stepID,
		},
	})
	if err != nil {
		t.Fatalf("list_step_artifacts failed: %v", err)
	}
	artifacts, ok := artifactsResult.StructuredContent.([]any)
	if !ok || len(artifacts) != 1 {
		t.Fatalf("unexpected artifacts payload: %+v", artifactsResult.StructuredContent)
	}
	firstArtifact, ok := artifacts[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected artifact payload: %+v", artifacts[0])
	}
	artifactID, _ := firstArtifact["id"].(string)
	if artifactID == "" {
		t.Fatalf("missing artifact id in %+v", firstArtifact)
	}

	artifactContent, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "codencer.get_artifact_content",
		Arguments: map[string]any{
			"artifact_id": artifactID,
		},
	})
	if err != nil {
		t.Fatalf("get_artifact_content failed: %v", err)
	}
	artifactPayload, ok := artifactContent.StructuredContent.(map[string]any)
	if !ok || artifactPayload["text"] != "artifact-content" {
		t.Fatalf("unexpected artifact content payload: %+v", artifactContent.StructuredContent)
	}
}

type cloudMCPAuthRoundTripper struct {
	base          http.RoundTripper
	authorization string
	origin        string
}

func (rt cloudMCPAuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	base := rt.base
	if base == nil {
		base = http.DefaultTransport
	}
	cloned := req.Clone(req.Context())
	cloned.Header = req.Header.Clone()
	if rt.authorization != "" {
		cloned.Header.Set("Authorization", rt.authorization)
	}
	if rt.origin != "" {
		cloned.Header.Set("Origin", rt.origin)
	}
	return base.RoundTrip(cloned)
}

func mergeHeaders(base, extra map[string]string) map[string]string {
	if len(extra) == 0 {
		return base
	}
	out := make(map[string]string, len(base)+len(extra))
	for key, value := range base {
		out[key] = value
	}
	for key, value := range extra {
		out[key] = value
	}
	return out
}

func mcpHTTPStatus(response map[string]any) int {
	status, _ := response["http_status"].(float64)
	return int(status)
}

func requireMCPToolSuccess(t *testing.T, response map[string]any) map[string]any {
	t.Helper()
	result, ok := response["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected MCP result payload, got %+v", response)
	}
	if isError, _ := result["isError"].(bool); isError {
		t.Fatalf("unexpected MCP tool error: %+v", response)
	}
	return result
}

func requireMCPToolError(t *testing.T, response map[string]any) map[string]any {
	t.Helper()
	result, ok := response["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected MCP result payload, got %+v", response)
	}
	if isError, _ := result["isError"].(bool); !isError {
		t.Fatalf("expected MCP tool error, got %+v", response)
	}
	structured, ok := result["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("expected structured error payload, got %+v", result)
	}
	errPayload, ok := structured["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected structured error details, got %+v", structured)
	}
	return errPayload
}
