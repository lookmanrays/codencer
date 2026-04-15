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

func jsonBody(t *testing.T, payload any) *bytes.Reader {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(raw)
}
