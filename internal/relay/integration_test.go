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
	"testing"
	"time"

	"agent-bridge/internal/connector"
	"agent-bridge/internal/domain"
	"agent-bridge/internal/relay"
)

func TestRelayConnectorProxyFlow(t *testing.T) {
	t.Parallel()

	artifact := domain.Artifact{ID: "art-1", AttemptID: "attempt-1", Name: "stdout.log", Path: "/tmp/stdout.log"}
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
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/runs/run-1/steps":
			_, _ = w.Write([]byte(`{"id":"step-1","phase_id":"phase-1","state":"running"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-1":
			_, _ = w.Write([]byte(`{"id":"step-1","phase_id":"phase-1","state":"completed"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/steps/step-1/wait":
			_, _ = w.Write([]byte(`{"step_id":"step-1","state":"completed","terminal":true,"timed_out":false}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/steps/step-1/retry":
			w.WriteHeader(http.StatusAccepted)
		case r.Method == http.MethodPatch && r.URL.Path == "/api/v1/runs/run-1":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-1/result":
			_, _ = w.Write([]byte(`{"version":"v1","run_id":"run-1","step_id":"step-1","state":"completed","summary":"done"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/steps/step-1/artifacts":
			_ = json.NewEncoder(w).Encode([]domain.Artifact{artifact})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/artifacts/art-1/content":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("artifact-content"))
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
		Host:             "127.0.0.1",
		Port:             0,
		DBPath:           filepath.Join(t.TempDir(), "relay-unused.db"),
		PlannerToken:     "planner-token",
		EnrollmentSecret: "enroll-secret",
	}, store)
	relayHTTP := httptest.NewServer(server.Handler())
	defer relayHTTP.Close()

	plannerClient := &http.Client{Timeout: 5 * time.Second}
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

	do(http.MethodPost, "/api/v2/instances/inst-1/runs", []byte(`{"id":"run-1","project_id":"proj"}`))
	do(http.MethodGet, "/api/v2/instances/inst-1", nil)
	do(http.MethodGet, "/api/v2/instances/inst-1/runs/run-1", nil)
	do(http.MethodPost, "/api/v2/instances/inst-1/runs/run-1/steps", []byte(`{"goal":"hello"}`))
	resultBody := do(http.MethodGet, "/api/v2/steps/step-1/result", nil)
	if !bytes.Contains(resultBody, []byte(`"summary":"done"`)) {
		t.Fatalf("unexpected result payload: %s", string(resultBody))
	}
	waitBody := do(http.MethodPost, "/api/v2/steps/step-1/wait", []byte(`{"timeout_ms":500}`))
	if !bytes.Contains(waitBody, []byte(`"terminal":true`)) {
		t.Fatalf("unexpected wait payload: %s", string(waitBody))
	}
	do(http.MethodPost, "/api/v2/steps/step-1/retry", nil)
	do(http.MethodGet, "/api/v2/steps/step-1/artifacts", nil)
	content := do(http.MethodGet, "/api/v2/artifacts/art-1/content", nil)
	if string(content) != "artifact-content" {
		t.Fatalf("unexpected artifact content: %s", string(content))
	}
	do(http.MethodPost, "/api/v2/instances/inst-1/runs/run-1/abort", nil)

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
