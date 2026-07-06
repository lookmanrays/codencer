package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"agent-bridge/internal/relay"
)

func TestCloudRuntimeRegistryClaimAndList(t *testing.T) {
	cloudStore, relayRuntime, operatorToken, org, workspace, project := newRuntimeCloudHarness(t)
	defer cloudStore.Close()
	defer relayRuntime.Store.Close()

	now := time.Date(2026, time.April, 15, 12, 0, 0, 0, time.UTC)
	if err := relayRuntime.Store.SaveConnectorRecord(context.Background(), relay.ConnectorRecord{
		ConnectorID:         "conn-1",
		MachineID:           "machine-1",
		PublicKey:           "pub-1",
		Label:               "WSL Node",
		MachineMetadataJSON: `{"os":"wsl","host":"dev-box"}`,
		CreatedAt:           now,
		LastSeenAt:          now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := relayRuntime.Store.SaveInstance(context.Background(), relay.InstanceRecord{
		InstanceID:   "inst-1",
		ConnectorID:  "conn-1",
		RepoRoot:     "/repo/core",
		BaseURL:      "http://127.0.0.1:8085",
		InstanceJSON: `{"id":"inst-1","repo_root":"/repo/core"}`,
		LastSeenAt:   now,
	}); err != nil {
		t.Fatal(err)
	}

	server := NewServer(DefaultConfig(), cloudStore, nil, relayRuntime)
	handler := server.Handler()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/cloud/v1/runtime/connectors", jsonBody(t, map[string]any{
		"org_id":       org.ID,
		"workspace_id": workspace.ID,
		"project_id":   project.ID,
		"connector_id": "conn-1",
	}))
	req.Header.Set("Authorization", "Bearer "+operatorToken)
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected runtime connector claim created, got %d body=%s", rr.Code, rr.Body.String())
	}
	var claimed RuntimeConnectorInstallation
	if err := json.NewDecoder(rr.Body).Decode(&claimed); err != nil {
		t.Fatal(err)
	}
	if claimed.ConnectorID != "conn-1" || claimed.OrgID != org.ID {
		t.Fatalf("unexpected claimed runtime connector: %+v", claimed)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/cloud/v1/runtime/connectors?org_id="+org.ID, nil)
	req.Header.Set("Authorization", "Bearer "+operatorToken)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected runtime connector list ok, got %d body=%s", rr.Code, rr.Body.String())
	}
	var connectors []RuntimeConnectorInstallation
	if err := json.NewDecoder(rr.Body).Decode(&connectors); err != nil {
		t.Fatal(err)
	}
	if len(connectors) != 1 || connectors[0].ConnectorID != "conn-1" {
		t.Fatalf("unexpected runtime connectors: %+v", connectors)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/cloud/v1/runtime/instances?org_id="+org.ID, nil)
	req.Header.Set("Authorization", "Bearer "+operatorToken)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected runtime instance list ok, got %d body=%s", rr.Code, rr.Body.String())
	}
	var instances []RuntimeInstance
	if err := json.NewDecoder(rr.Body).Decode(&instances); err != nil {
		t.Fatal(err)
	}
	if len(instances) != 1 || instances[0].ID != "inst-1" || !instances[0].Shared {
		t.Fatalf("unexpected runtime instances: %+v", instances)
	}
}

func TestCloudRuntimeConnectorDisablePropagatesToRelayStore(t *testing.T) {
	cloudStore, relayRuntime, operatorToken, org, workspace, project := newRuntimeCloudHarness(t)
	defer cloudStore.Close()
	defer relayRuntime.Store.Close()

	if err := relayRuntime.Store.SaveConnectorRecord(context.Background(), relay.ConnectorRecord{
		ConnectorID: "conn-2",
		MachineID:   "machine-2",
		PublicKey:   "pub-2",
		Label:       "Laptop",
		CreatedAt:   time.Now().UTC(),
		LastSeenAt:  time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	server := NewServer(DefaultConfig(), cloudStore, nil, relayRuntime)
	handler := server.Handler()

	claim := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/cloud/v1/runtime/connectors", jsonBody(t, map[string]any{
		"org_id":       org.ID,
		"workspace_id": workspace.ID,
		"project_id":   project.ID,
		"connector_id": "conn-2",
	}))
	req.Header.Set("Authorization", "Bearer "+operatorToken)
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(claim, req)
	if claim.Code != http.StatusCreated {
		t.Fatalf("expected claim ok, got %d body=%s", claim.Code, claim.Body.String())
	}
	var runtimeConnector RuntimeConnectorInstallation
	if err := json.NewDecoder(claim.Body).Decode(&runtimeConnector); err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/cloud/v1/runtime/connectors/"+runtimeConnector.ID+"/disable", nil)
	req.Header.Set("Authorization", "Bearer "+operatorToken)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected disable ok, got %d body=%s", rr.Code, rr.Body.String())
	}

	relayConnector, err := relayRuntime.Store.GetConnector(context.Background(), "conn-2")
	if err != nil {
		t.Fatal(err)
	}
	if relayConnector == nil || !relayConnector.Disabled {
		t.Fatalf("expected relay connector to be disabled, got %+v", relayConnector)
	}
}

func TestCloudRuntimeInstanceScopeDeniesOtherOrg(t *testing.T) {
	cloudStore, relayRuntime, operatorToken, org, workspace, project := newRuntimeCloudHarness(t)
	defer cloudStore.Close()
	defer relayRuntime.Store.Close()

	otherOrg, err := cloudStore.CreateOrg(context.Background(), Org{Slug: "other", Name: "Other"})
	if err != nil {
		t.Fatal(err)
	}
	otherTokenRaw, err := GenerateAPIToken()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cloudStore.CreateAPIToken(context.Background(), APIToken{
		OrgID:  otherOrg.ID,
		Name:   "other-operator",
		Scopes: runtimeOperatorScopes(),
	}, otherTokenRaw); err != nil {
		t.Fatal(err)
	}

	if err := relayRuntime.Store.SaveConnectorRecord(context.Background(), relay.ConnectorRecord{
		ConnectorID: "conn-3",
		MachineID:   "machine-3",
		PublicKey:   "pub-3",
		Label:       "WSL",
		CreatedAt:   time.Now().UTC(),
		LastSeenAt:  time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := relayRuntime.Store.SaveInstance(context.Background(), relay.InstanceRecord{
		InstanceID:   "inst-3",
		ConnectorID:  "conn-3",
		RepoRoot:     "/repo/secure",
		BaseURL:      "http://127.0.0.1:8085",
		InstanceJSON: `{"id":"inst-3"}`,
		LastSeenAt:   time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	server := NewServer(DefaultConfig(), cloudStore, nil, relayRuntime)
	handler := server.Handler()
	claimReq := httptest.NewRequest(http.MethodPost, "/api/cloud/v1/runtime/connectors", jsonBody(t, map[string]any{
		"org_id":       org.ID,
		"workspace_id": workspace.ID,
		"project_id":   project.ID,
		"connector_id": "conn-3",
	}))
	claimReq.Header.Set("Authorization", "Bearer "+operatorToken)
	claimReq.Header.Set("Content-Type", "application/json")
	claimRR := httptest.NewRecorder()
	handler.ServeHTTP(claimRR, claimReq)
	if claimRR.Code != http.StatusCreated {
		t.Fatalf("expected claim ok, got %d body=%s", claimRR.Code, claimRR.Body.String())
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/cloud/v1/runtime/instances/inst-3", nil)
	req.Header.Set("Authorization", "Bearer "+otherTokenRaw)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for other org token, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestCloudRuntimeProxySubmitTaskRequiresStepWriteScope(t *testing.T) {
	cloudStore, relayRuntime, operatorToken, org, workspace, project := newRuntimeCloudHarness(t)
	defer cloudStore.Close()
	defer relayRuntime.Store.Close()

	saveRuntimeConnectorAndInstance(t, relayRuntime, "conn-step-scope", "inst-step-scope")

	server := NewServer(DefaultConfig(), cloudStore, nil, relayRuntime)
	handler := server.Handler()
	claimRuntimeConnector(t, handler, operatorToken, org, workspace, project, "conn-step-scope")

	limitedToken := mustCreateRuntimeToken(t, cloudStore, org.ID, workspace.ID, project.ID, []string{
		"cloud:read",
		"runtime_instances:read",
		"runs:write",
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/cloud/v1/runtime/instances/inst-step-scope/runs/run-1/steps", jsonBody(t, map[string]any{
		"version": "v1",
		"goal":    "needs steps:write",
	}))
	req.Header.Set("Authorization", "Bearer "+limitedToken)
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden without steps:write, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestCloudRuntimeProxyRunGatesRequiresGatesReadScope(t *testing.T) {
	cloudStore, relayRuntime, operatorToken, org, workspace, project := newRuntimeCloudHarness(t)
	defer cloudStore.Close()
	defer relayRuntime.Store.Close()

	saveRuntimeConnectorAndInstance(t, relayRuntime, "conn-gates-scope", "inst-gates-scope")

	server := NewServer(DefaultConfig(), cloudStore, nil, relayRuntime)
	handler := server.Handler()
	claimRuntimeConnector(t, handler, operatorToken, org, workspace, project, "conn-gates-scope")

	limitedToken := mustCreateRuntimeToken(t, cloudStore, org.ID, workspace.ID, project.ID, []string{
		"cloud:read",
		"runtime_instances:read",
		"runs:read",
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/cloud/v1/runtime/instances/inst-gates-scope/runs/run-1/gates", nil)
	req.Header.Set("Authorization", "Bearer "+limitedToken)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden without gates:read, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestCloudRuntimeProxyDeniesOtherProjectToken(t *testing.T) {
	cloudStore, relayRuntime, operatorToken, org, workspace, project := newRuntimeCloudHarness(t)
	defer cloudStore.Close()
	defer relayRuntime.Store.Close()

	saveRuntimeConnectorAndInstance(t, relayRuntime, "conn-project-scope", "inst-project-scope")

	ctx := context.Background()
	otherWorkspace, err := cloudStore.CreateWorkspace(ctx, Workspace{OrgID: org.ID, Slug: "other-workspace", Name: "Other Workspace"})
	if err != nil {
		t.Fatal(err)
	}
	otherProject, err := cloudStore.CreateProject(ctx, Project{OrgID: org.ID, WorkspaceID: otherWorkspace.ID, Slug: "other-project", Name: "Other Project"})
	if err != nil {
		t.Fatal(err)
	}

	server := NewServer(DefaultConfig(), cloudStore, nil, relayRuntime)
	handler := server.Handler()
	claimRuntimeConnector(t, handler, operatorToken, org, workspace, project, "conn-project-scope")

	otherProjectToken := mustCreateRuntimeToken(t, cloudStore, org.ID, otherWorkspace.ID, otherProject.ID, runtimeOperatorScopes())

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/cloud/v1/runtime/instances/inst-project-scope/runs", jsonBody(t, map[string]any{
		"project_id": "proj",
	}))
	req.Header.Set("Authorization", "Bearer "+otherProjectToken)
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for other project token, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestCloudRuntimeHTTPProxyStartRunAndSubmitTask(t *testing.T) {
	h := startCloudMCPHarness(t)

	req, err := http.NewRequest(http.MethodGet, h.cloudHTTP.URL+"/api/cloud/v1/runtime/instances", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", h.auth)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected runtime instance list ok, got %d", resp.StatusCode)
	}
	var instances []RuntimeInstance
	if err := json.NewDecoder(resp.Body).Decode(&instances); err != nil {
		t.Fatal(err)
	}
	if len(instances) != 1 {
		t.Fatalf("expected one runtime instance, got %+v", instances)
	}

	instanceID := instances[0].ID
	runReq, err := http.NewRequest(http.MethodPost, h.cloudHTTP.URL+"/api/cloud/v1/runtime/instances/"+instanceID+"/runs", jsonBody(t, map[string]any{
		"project_id": "proj",
	}))
	if err != nil {
		t.Fatal(err)
	}
	runReq.Header.Set("Authorization", h.auth)
	runReq.Header.Set("Content-Type", "application/json")
	runResp, err := http.DefaultClient.Do(runReq)
	if err != nil {
		t.Fatal(err)
	}
	defer runResp.Body.Close()
	if runResp.StatusCode != http.StatusOK {
		t.Fatalf("expected start run ok, got %d", runResp.StatusCode)
	}
	var runBody map[string]any
	if err := json.NewDecoder(runResp.Body).Decode(&runBody); err != nil {
		t.Fatal(err)
	}
	if runBody["id"] != "run-1" {
		t.Fatalf("expected proxied run response, got %+v", runBody)
	}

	stepReq, err := http.NewRequest(http.MethodPost, h.cloudHTTP.URL+"/api/cloud/v1/runtime/instances/"+instanceID+"/runs/run-1/steps", jsonBody(t, map[string]any{
		"version": "v1",
		"goal":    "Ship cloud beta",
	}))
	if err != nil {
		t.Fatal(err)
	}
	stepReq.Header.Set("Authorization", h.auth)
	stepReq.Header.Set("Content-Type", "application/json")
	stepResp, err := http.DefaultClient.Do(stepReq)
	if err != nil {
		t.Fatal(err)
	}
	defer stepResp.Body.Close()
	if stepResp.StatusCode != http.StatusOK {
		t.Fatalf("expected submit task ok, got %d", stepResp.StatusCode)
	}
	var stepBody map[string]any
	if err := json.NewDecoder(stepResp.Body).Decode(&stepBody); err != nil {
		t.Fatal(err)
	}
	if stepBody["id"] != "step-1" {
		t.Fatalf("expected proxied step response, got %+v", stepBody)
	}

	h.mu.Lock()
	gotGoal, _ := h.lastTask["goal"].(string)
	h.mu.Unlock()
	if gotGoal != "Ship cloud beta" {
		t.Fatalf("expected task goal to reach daemon, got %+v", h.lastTask)
	}
}

func TestCloudRuntimeHTTPProxyStepWaitRetryAndGateActions(t *testing.T) {
	h := startCloudMCPHarness(t)
	instanceID := "inst-1"

	waitReq, err := http.NewRequest(http.MethodPost, h.cloudHTTP.URL+"/api/cloud/v1/runtime/instances/"+instanceID+"/steps/step-1/wait", jsonBody(t, map[string]any{
		"timeout_ms":  500,
		"interval_ms": 50,
	}))
	if err != nil {
		t.Fatal(err)
	}
	waitReq.Header.Set("Authorization", h.auth)
	waitReq.Header.Set("Content-Type", "application/json")
	waitResp, err := http.DefaultClient.Do(waitReq)
	if err != nil {
		t.Fatal(err)
	}
	defer waitResp.Body.Close()
	if waitResp.StatusCode != http.StatusOK {
		t.Fatalf("expected wait ok, got %d", waitResp.StatusCode)
	}
	var waitBody map[string]any
	if err := json.NewDecoder(waitResp.Body).Decode(&waitBody); err != nil {
		t.Fatal(err)
	}
	if waitBody["terminal"] != true {
		t.Fatalf("expected terminal wait body, got %+v", waitBody)
	}

	retryReq, err := http.NewRequest(http.MethodPost, h.cloudHTTP.URL+"/api/cloud/v1/runtime/instances/"+instanceID+"/steps/step-1/retry", nil)
	if err != nil {
		t.Fatal(err)
	}
	retryReq.Header.Set("Authorization", h.auth)
	retryResp, err := http.DefaultClient.Do(retryReq)
	if err != nil {
		t.Fatal(err)
	}
	defer retryResp.Body.Close()
	if retryResp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected retry accepted, got %d", retryResp.StatusCode)
	}

	gateReq, err := http.NewRequest(http.MethodPost, h.cloudHTTP.URL+"/api/cloud/v1/runtime/instances/"+instanceID+"/gates/gate-1/approve", nil)
	if err != nil {
		t.Fatal(err)
	}
	gateReq.Header.Set("Authorization", h.auth)
	gateResp, err := http.DefaultClient.Do(gateReq)
	if err != nil {
		t.Fatal(err)
	}
	defer gateResp.Body.Close()
	if gateResp.StatusCode != http.StatusOK {
		t.Fatalf("expected gate approve ok, got %d", gateResp.StatusCode)
	}
}

func newRuntimeCloudHarness(t *testing.T) (*Store, *RelayRuntime, string, *Org, *Workspace, *Project) {
	t.Helper()
	cloudStore, err := OpenStore(filepath.Join(t.TempDir(), "cloud.db"), "cloud-master-key")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	org, err := cloudStore.CreateOrg(ctx, Org{Slug: "acme", Name: "Acme"})
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := cloudStore.CreateWorkspace(ctx, Workspace{OrgID: org.ID, Slug: "platform", Name: "Platform"})
	if err != nil {
		t.Fatal(err)
	}
	project, err := cloudStore.CreateProject(ctx, Project{OrgID: org.ID, WorkspaceID: workspace.ID, Slug: "core", Name: "Core"})
	if err != nil {
		t.Fatal(err)
	}

	operatorToken, err := GenerateAPIToken()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cloudStore.CreateAPIToken(ctx, APIToken{
		OrgID:       org.ID,
		WorkspaceID: workspace.ID,
		ProjectID:   project.ID,
		Name:        "runtime-operator",
		Scopes:      runtimeOperatorScopes(),
	}, operatorToken); err != nil {
		t.Fatal(err)
	}

	relayStore, err := relay.OpenStore(filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatal(err)
	}
	relayServer := relay.NewServer(&relay.Config{
		Host:   "127.0.0.1",
		Port:   0,
		DBPath: filepath.Join(t.TempDir(), "relay-unused.db"),
	}, relayStore)

	return cloudStore, &RelayRuntime{Server: relayServer, Store: relayStore}, operatorToken, org, workspace, project
}

func runtimeOperatorScopes() []string {
	return []string{
		"cloud:read",
		"runtime_connectors:read",
		"runtime_connectors:write",
		"runtime_instances:read",
		"runs:read",
		"runs:write",
		"steps:read",
		"steps:write",
		"artifacts:read",
		"gates:read",
		"gates:write",
	}
}

func claimRuntimeConnector(t *testing.T, handler http.Handler, token string, org *Org, workspace *Workspace, project *Project, connectorID string) RuntimeConnectorInstallation {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/cloud/v1/runtime/connectors", jsonBody(t, map[string]any{
		"org_id":       org.ID,
		"workspace_id": workspace.ID,
		"project_id":   project.ID,
		"connector_id": connectorID,
	}))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected claim ok, got %d body=%s", rr.Code, rr.Body.String())
	}
	var installation RuntimeConnectorInstallation
	if err := json.NewDecoder(rr.Body).Decode(&installation); err != nil {
		t.Fatal(err)
	}
	return installation
}

func saveRuntimeConnectorAndInstance(t *testing.T, relayRuntime *RelayRuntime, connectorID, instanceID string) {
	t.Helper()
	now := time.Now().UTC()
	if err := relayRuntime.Store.SaveConnectorRecord(context.Background(), relay.ConnectorRecord{
		ConnectorID: connectorID,
		MachineID:   connectorID + "-machine",
		PublicKey:   connectorID + "-pub",
		Label:       connectorID + "-label",
		CreatedAt:   now,
		LastSeenAt:  now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := relayRuntime.Store.SaveInstance(context.Background(), relay.InstanceRecord{
		InstanceID:   instanceID,
		ConnectorID:  connectorID,
		RepoRoot:     "/repo/" + instanceID,
		BaseURL:      "http://127.0.0.1:8085",
		InstanceJSON: `{"id":"` + instanceID + `"}`,
		LastSeenAt:   now,
	}); err != nil {
		t.Fatal(err)
	}
}

func mustCreateRuntimeToken(t *testing.T, store *Store, orgID, workspaceID, projectID string, scopes []string) string {
	t.Helper()
	raw, err := GenerateAPIToken()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateAPIToken(context.Background(), APIToken{
		OrgID:       orgID,
		WorkspaceID: workspaceID,
		ProjectID:   projectID,
		Name:        "runtime-test-token",
		Scopes:      scopes,
	}, raw); err != nil {
		t.Fatal(err)
	}
	return raw
}

func jsonBody(t *testing.T, payload any) *bytes.Reader {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(raw)
}
