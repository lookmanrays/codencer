package relay

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/relayproto"
)

func TestHandleAdvertiseReplacesSharedInstancesAndPrunesRoutes(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	server := NewServer(&Config{
		Host:              "127.0.0.1",
		Port:              0,
		DBPath:            filepath.Join(t.TempDir(), "unused.db"),
		PlannerToken:      "planner-token",
		SessionTTLSeconds: 60,
	}, store)
	session := &session{
		connectorID: "connector-1",
		instanceIDs: map[string]struct{}{},
		pending:     make(map[string]chan relayproto.CommandResponse),
	}
	server.hub.RegisterConnector(session)

	first := mustAdvertiseMessage(t, "inst-1", "/repo-a", "http://127.0.0.1:8085")
	if err := server.handleAdvertise(context.Background(), session, first); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveResourceRoute(context.Background(), "step", "step-1", "inst-1"); err != nil {
		t.Fatal(err)
	}
	if got := server.hub.Get("inst-1"); got == nil {
		t.Fatal("expected inst-1 to be routable after initial advertise")
	}

	second := mustAdvertiseMessage(t, "inst-2", "/repo-b", "http://127.0.0.1:8086")
	if err := server.handleAdvertise(context.Background(), session, second); err != nil {
		t.Fatal(err)
	}

	inst1, err := store.GetInstance(context.Background(), "inst-1")
	if err != nil {
		t.Fatal(err)
	}
	if inst1 != nil {
		t.Fatalf("expected inst-1 to be pruned from store, got %+v", inst1)
	}
	inst2, err := store.GetInstance(context.Background(), "inst-2")
	if err != nil {
		t.Fatal(err)
	}
	if inst2 == nil || inst2.ConnectorID != "connector-1" {
		t.Fatalf("expected inst-2 to remain stored, got %+v", inst2)
	}
	route, err := store.LookupResourceRoute(context.Background(), "step", "step-1")
	if err != nil {
		t.Fatal(err)
	}
	if route != "" {
		t.Fatalf("expected step route hint to be pruned, got %q", route)
	}
	if got := server.hub.Get("inst-1"); got != nil {
		t.Fatalf("expected inst-1 to be removed from live hub, got %+v", got)
	}
	if got := server.hub.Get("inst-2"); got != session {
		t.Fatalf("expected inst-2 to remain live, got %+v", got)
	}
}

func TestHandleAdvertiseStoresSharedProjects(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	server := NewServer(&Config{
		Host:         "127.0.0.1",
		Port:         0,
		DBPath:       filepath.Join(t.TempDir(), "unused.db"),
		PlannerToken: "planner-token",
	}, store)
	session := &session{
		connectorID: "connector-1",
		instanceIDs: map[string]struct{}{},
		projectIDs:  map[string]struct{}{},
		pending:     make(map[string]chan relayproto.CommandResponse),
	}
	server.hub.RegisterConnector(session)

	instanceJSON, _ := json.Marshal(domain.InstanceInfo{ID: "inst-1", RepoRoot: "/repo", BaseURL: "http://127.0.0.1:8085"})
	projectJSON := []byte(`{"id":"proj","repo_root":"/repo","default_adapter":"fake","adapter_profile":"fake-success","shared_to_relay":true}`)
	message, err := json.Marshal(relayproto.AdvertiseMessage{
		Type:      "advertise",
		Instances: []relayproto.InstanceAdvertisement{{Instance: instanceJSON}},
		Projects: []relayproto.ProjectAdvertisement{{
			ProjectID:  "proj",
			InstanceID: "inst-1",
			MachineID:  "mach_local",
			HostLabel:  "macbook",
			Hostname:   "host.local",
			Status:     "available",
			Project:    projectJSON,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.handleAdvertise(context.Background(), session, message); err != nil {
		t.Fatal(err)
	}
	projects, err := store.ListProjectsByID(context.Background(), "proj")
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects[0].ConnectorID != "connector-1" || projects[0].InstanceID != "inst-1" {
		t.Fatalf("expected stored project advertisement, got %+v", projects)
	}
	if projects[0].MachineID != "mach_local" || projects[0].HostLabel != "macbook" || projects[0].Hostname != "host.local" {
		t.Fatalf("expected stored project machine metadata, got %+v", projects[0])
	}
}

func TestRelayProjectLocationsRoutingAndMCPOutput(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	server := NewServer(&Config{
		Host:         "127.0.0.1",
		Port:         0,
		DBPath:       filepath.Join(t.TempDir(), "unused.db"),
		PlannerToken: "planner-token",
	}, store)
	seedProjectLocation(t, server, store, "connector-mac", "inst-mac", "mach_mac", "macbook", "/Users/example/private/codencer")
	seedProjectLocation(t, server, store, "connector-wsl", "inst-wsl", "mach_wsl", "pc-wsl", "/home/example/private/codencer")
	principal := &plannerPrincipal{Name: "operator", Scopes: []string{"*"}}

	byMachine, apiErr := server.resolveProjectRecord(context.Background(), principal, "codencer", "runs:write", projectSelector{MachineID: "mach_mac"})
	if apiErr != nil {
		t.Fatalf("route by machine_id failed: %+v", apiErr)
	}
	if byMachine.InstanceID != "inst-mac" {
		t.Fatalf("machine selector routed to %s", byMachine.InstanceID)
	}

	byLabel, apiErr := server.resolveProjectRecord(context.Background(), principal, "codencer", "runs:write", projectSelector{HostLabel: "pc-wsl"})
	if apiErr != nil {
		t.Fatalf("route by host_label failed: %+v", apiErr)
	}
	if byLabel.InstanceID != "inst-wsl" {
		t.Fatalf("host label selector routed to %s", byLabel.InstanceID)
	}

	_, apiErr = server.resolveProjectRecord(context.Background(), principal, "codencer", "runs:write", projectSelector{})
	if apiErr == nil || apiErr.Code != "ambiguous_project_location" {
		t.Fatalf("expected ambiguous project location, got %+v", apiErr)
	}
	if apiErr.Blocker["type"] != "ambiguous_project_location" || apiErr.Blocker["planner_decision_required"] != true {
		t.Fatalf("expected structured ambiguity blocker, got %+v", apiErr.Blocker)
	}

	listResult, apiErr := server.mcp.tools["codencer.list_projects"].Invoke(context.Background(), principal, map[string]any{})
	if apiErr != nil {
		t.Fatalf("mcp list projects failed: %+v", apiErr)
	}
	encoded, _ := json.Marshal(listResult.StructuredContent)
	if strings.Contains(string(encoded), "/Users/example") || strings.Contains(string(encoded), "/home/example") {
		t.Fatalf("MCP list_projects leaked absolute paths: %s", encoded)
	}
	projects, ok := listResult.StructuredContent.([]any)
	if !ok || len(projects) != 1 {
		t.Fatalf("expected grouped project locations in MCP output, got %#v", listResult.StructuredContent)
	}
	firstProject := projects[0].(map[string]any)
	locations := firstProject["locations"].([]any)
	if len(locations) != 2 {
		t.Fatalf("expected two project locations in MCP output, got %+v", firstProject)
	}

	_, apiErr = server.mcp.tools["codencer.submit_project_task_and_wait"].Invoke(context.Background(), principal, map[string]any{
		"project_id": "codencer",
		"goal":       "do it",
	})
	if apiErr == nil {
		t.Fatal("expected ambiguous project location api error")
	}
	ambiguousResult := apiErrorToolResult(apiErr)
	if !ambiguousResult.IsError {
		t.Fatalf("expected MCP ambiguity result to be an error, got %+v", ambiguousResult)
	}
	structured := ambiguousResult.StructuredContent.(map[string]any)
	if structured["type"] != "ambiguous_project_location" || structured["planner_decision_required"] != true {
		t.Fatalf("unexpected MCP ambiguity payload: %+v", structured)
	}
}

func seedProjectLocation(t *testing.T, server *Server, store *Store, connectorID, instanceID, machineID, hostLabel, repoRoot string) {
	t.Helper()
	if err := store.SaveConnector(context.Background(), connectorID, "relay-"+machineID, "pub", hostLabel); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReplaceConnectorProjects(context.Background(), connectorID, []ProjectRecord{{
		ProjectID:      "codencer",
		ConnectorID:    connectorID,
		InstanceID:     instanceID,
		MachineID:      machineID,
		HostLabel:      hostLabel,
		Hostname:       hostLabel + ".local",
		RepoRoot:       repoRoot,
		DefaultAdapter: "fake",
		AdapterProfile: "fake-success",
		ProjectJSON:    `{"id":"codencer","name":"Codencer","repo_root":"` + repoRoot + `","default_adapter":"fake","adapter_profile":"fake-success"}`,
		LastSeenAt:     time.Now().UTC(),
	}}); err != nil {
		t.Fatal(err)
	}
	server.hub.RegisterConnector(&session{
		connectorID: connectorID,
		instanceIDs: map[string]struct{}{instanceID: {}},
		projectIDs:  map[string]struct{}{"codencer": {}},
		pending:     make(map[string]chan relayproto.CommandResponse),
		lastSeenAt:  time.Now().UTC(),
	})
}

func TestResolveResourceRouteIgnoresOfflineHint(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.SaveConnector(context.Background(), "connector-1", "machine-1", "pub", "offline"); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveInstance(context.Background(), InstanceRecord{
		InstanceID:   "inst-offline",
		ConnectorID:  "connector-1",
		RepoRoot:     "/repo",
		BaseURL:      "http://127.0.0.1:8085",
		InstanceJSON: `{"id":"inst-offline"}`,
		LastSeenAt:   time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveResourceRoute(context.Background(), "step", "step-1", "inst-offline"); err != nil {
		t.Fatal(err)
	}

	server := NewServer(&Config{
		Host:         "127.0.0.1",
		Port:         0,
		DBPath:       filepath.Join(t.TempDir(), "unused.db"),
		PlannerToken: "planner-token",
	}, store)

	instanceID, apiErr := server.resolveResourceRoute(context.Background(), &plannerPrincipal{
		Name:   "operator",
		Scopes: []string{"*"},
	}, "step", "step-1", "steps:read", "")
	if apiErr == nil {
		t.Fatalf("expected offline route hint to fail closed, got instance %s", instanceID)
	}
	if instanceID != "" {
		t.Fatalf("expected no routed instance for offline hint, got %s", instanceID)
	}
	if apiErr.Code != "connector_offline" {
		t.Fatalf("expected connector_offline, got %+v", apiErr)
	}
}

func mustAdvertiseMessage(t *testing.T, instanceID, repoRoot, baseURL string) []byte {
	t.Helper()
	info, err := json.Marshal(domain.InstanceInfo{
		ID:       instanceID,
		RepoRoot: repoRoot,
		BaseURL:  baseURL,
	})
	if err != nil {
		t.Fatal(err)
	}
	message, err := json.Marshal(relayproto.AdvertiseMessage{
		Type:      "advertise",
		Instances: []relayproto.InstanceAdvertisement{{Instance: info}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return message
}
