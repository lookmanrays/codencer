package relay_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-bridge/internal/connector"
	"agent-bridge/internal/domain"
	"agent-bridge/internal/relay"
)

func TestRelayConnectorProxyFlow(t *testing.T) {
	t.Parallel()

	artifact := domain.Artifact{ID: "art-1", AttemptID: "attempt-1", Name: "stdout.log", Path: "/tmp/stdout.log"}
	slowStepReadyAt := time.Now().Add(16 * time.Second)
	var fakeDaemon *httptest.Server
	fakeDaemon = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/instance":
			_ = json.NewEncoder(w).Encode(domain.InstanceInfo{
				ID:       "inst-1",
				RepoRoot: "/repo",
				BaseURL:  fakeDaemon.URL,
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/compatibility":
			_, _ = w.Write([]byte(`{"tier":2,"adapters":[],"environment":{"os":"test","vscode_detected":false}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/runs":
			_, _ = w.Write([]byte(`{"id":"run-1","project_id":"proj","state":"running"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/runs/run-1":
			_, _ = w.Write([]byte(`{"id":"run-1","project_id":"proj","state":"running"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/runs/run-1/gates":
			_ = json.NewEncoder(w).Encode([]domain.Gate{{
				ID:          "gate-1",
				RunID:       "run-1",
				StepID:      "step-1",
				Description: "pending",
				State:       domain.GateStatePending,
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/runs/run-1/steps":
			_, _ = w.Write([]byte(`{"id":"step-1","phase_id":"phase-1","state":"running"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-1":
			_, _ = w.Write([]byte(`{"id":"step-1","phase_id":"phase-1","state":"completed"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-slow":
			state := "running"
			if time.Now().After(slowStepReadyAt) {
				state = "completed"
			}
			_, _ = w.Write([]byte(fmt.Sprintf(`{"id":"step-slow","phase_id":"phase-1","state":"%s"}`, state)))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/steps/step-1/wait":
			_, _ = w.Write([]byte(`{"step_id":"step-1","state":"completed","terminal":true,"timed_out":false}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/steps/step-1/retry":
			w.WriteHeader(http.StatusAccepted)
		case r.Method == http.MethodPatch && r.URL.Path == "/api/v1/runs/run-1":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-1/result":
			_, _ = w.Write([]byte(`{"version":"v1","run_id":"run-1","step_id":"step-1","state":"completed","summary":"done"}`))
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
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/gates/gate-1":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer fakeDaemon.Close()

	store, err := relay.OpenStore(filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	server := relay.NewServer(&relay.Config{
		Host:                "127.0.0.1",
		Port:                0,
		DBPath:              filepath.Join(t.TempDir(), "relay-unused.db"),
		PlannerToken:        "planner-token",
		EnrollmentSecret:    "enroll-secret",
		ProxyTimeoutSeconds: 20,
	}, store)
	relayHTTP := httptest.NewServer(server.Handler())
	defer relayHTTP.Close()

	plannerClient := &http.Client{Timeout: 25 * time.Second}
	auth := "Bearer planner-token"

	var enrollSecret string
	req, _ := http.NewRequest(http.MethodPost, relayHTTP.URL+"/api/v2/connectors/enrollment-tokens", bytes.NewReader([]byte(`{"label":"local-dev"}`)))
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Type", "application/json")
	resp, err := plannerClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var tokenResp map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode >= 400 {
		t.Fatalf("token creation failed: %+v", tokenResp)
	}
	enrollSecret, _ = tokenResp["secret"].(string)
	if enrollSecret == "" {
		t.Fatalf("expected enrollment secret from relay token endpoint, got %+v", tokenResp)
	}

	configPath := filepath.Join(t.TempDir(), "connector.json")
	cfg, err := connector.Enroll(context.Background(), relayHTTP.URL, fakeDaemon.URL, enrollSecret, "test-connector", configPath)
	if err != nil {
		t.Fatal(err)
	}
	client := connector.NewClient(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- client.Run(ctx) }()

	var instances []relay.InstanceRecord
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		req, _ := http.NewRequest(http.MethodGet, relayHTTP.URL+"/api/v2/instances", nil)
		req.Header.Set("Authorization", auth)
		resp, err := plannerClient.Do(req)
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if json.Unmarshal(body, &instances) == nil && len(instances) == 1 {
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	if len(instances) != 1 {
		t.Fatalf("expected one connected instance, got %+v", instances)
	}

	do := func(method, path string, payload []byte) []byte {
		req, _ := http.NewRequest(method, relayHTTP.URL+path, bytes.NewReader(payload))
		req.Header.Set("Authorization", auth)
		if len(payload) > 0 {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := plannerClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 400 {
			t.Fatalf("%s %s failed: %s", method, path, string(body))
		}
		return body
	}

	statusBody := do(http.MethodGet, "/api/v2/status", nil)
	if !bytes.Contains(statusBody, []byte(`"planner_auth_mode":"static_bearer_tokens"`)) {
		t.Fatalf("unexpected relay status payload: %s", string(statusBody))
	}
	connectorsBody := do(http.MethodGet, "/api/v2/connectors", nil)
	if !bytes.Contains(connectorsBody, []byte(`"connector_id":"`+cfg.ConnectorID+`"`)) {
		t.Fatalf("unexpected connectors payload: %s", string(connectorsBody))
	}
	do(http.MethodGet, "/api/v2/steps/step-1", nil)
	do(http.MethodGet, "/api/v2/artifacts/art-1/content", nil)
	do(http.MethodPost, "/api/v2/gates/gate-1/approve", nil)
	gatesBody := do(http.MethodGet, "/api/v2/instances/inst-1/runs/run-1/gates", nil)
	if !bytes.Contains(gatesBody, []byte(`"id":"gate-1"`)) {
		t.Fatalf("unexpected run gates payload: %s", string(gatesBody))
	}
	do(http.MethodPost, "/api/v2/instances/inst-1/runs", []byte(`{"id":"run-1","project_id":"proj"}`))
	do(http.MethodGet, "/api/v2/instances/inst-1", nil)
	do(http.MethodGet, "/api/v2/instances/inst-1/runs/run-1", nil)
	do(http.MethodPost, "/api/v2/instances/inst-1/runs/run-1/steps", []byte(`{"goal":"hello"}`))
	resultBody := do(http.MethodGet, "/api/v2/steps/step-1/result", nil)
	if !bytes.Contains(resultBody, []byte(`"summary":"done"`)) {
		t.Fatalf("unexpected result payload: %s", string(resultBody))
	}
	logsBody := do(http.MethodGet, "/api/v2/steps/step-1/logs", nil)
	if string(logsBody) != "step-log-output" {
		t.Fatalf("unexpected step logs payload: %s", string(logsBody))
	}
	waitBody := do(http.MethodPost, "/api/v2/steps/step-1/wait", []byte(`{"timeout_ms":500}`))
	if !bytes.Contains(waitBody, []byte(`"terminal":true`)) {
		t.Fatalf("unexpected wait payload: %s", string(waitBody))
	}
	longWaitBody := do(http.MethodPost, "/api/v2/steps/step-slow/wait", []byte(`{"timeout_ms":17000}`))
	if !bytes.Contains(longWaitBody, []byte(`"terminal":true`)) {
		t.Fatalf("unexpected long wait payload: %s", string(longWaitBody))
	}
	do(http.MethodPost, "/api/v2/steps/step-1/retry", nil)
	do(http.MethodGet, "/api/v2/steps/step-1/artifacts", nil)
	content := do(http.MethodGet, "/api/v2/artifacts/art-1/content", nil)
	if string(content) != "artifact-content" {
		t.Fatalf("unexpected artifact content: %s", string(content))
	}
	disabledBody := do(http.MethodPost, "/api/v2/connectors/"+cfg.ConnectorID+"/disable", nil)
	if !bytes.Contains(disabledBody, []byte(`"disabled":true`)) {
		t.Fatalf("unexpected connector disable payload: %s", string(disabledBody))
	}
	reqDisabled, _ := http.NewRequest(http.MethodGet, relayHTTP.URL+"/api/v2/steps/step-1/result", nil)
	reqDisabled.Header.Set("Authorization", auth)
	respDisabled, err := plannerClient.Do(reqDisabled)
	if err != nil {
		t.Fatal(err)
	}
	disabledResultBody, _ := io.ReadAll(respDisabled.Body)
	_ = respDisabled.Body.Close()
	if respDisabled.StatusCode != http.StatusForbidden {
		t.Fatalf("expected disabled connector to deny routing, got %d body=%s", respDisabled.StatusCode, string(disabledResultBody))
	}
	enabledBody := do(http.MethodPost, "/api/v2/connectors/"+cfg.ConnectorID+"/enable", nil)
	if !bytes.Contains(enabledBody, []byte(`"disabled":false`)) {
		t.Fatalf("unexpected connector enable payload: %s", string(enabledBody))
	}
	recoveredBody := do(http.MethodGet, "/api/v2/steps/step-1/result", nil)
	if !bytes.Contains(recoveredBody, []byte(`"summary":"done"`)) {
		t.Fatalf("unexpected recovered result payload: %s", string(recoveredBody))
	}
	do(http.MethodPost, "/api/v2/instances/inst-1/runs/run-1/abort", nil)
	auditBody := do(http.MethodGet, "/api/v2/audit?limit=5", nil)
	if !bytes.Contains(auditBody, []byte(`"action":"abort_run"`)) || !bytes.Contains(auditBody, []byte(`"action":"disable_connector"`)) {
		t.Fatalf("unexpected audit payload: %s", string(auditBody))
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil && err != context.Canceled && !strings.Contains(err.Error(), "closed network connection") {
			t.Fatalf("connector run failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("connector did not stop")
	}
}
