package gateway

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConfigValidationAndRedaction(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PublicBaseURL = "http://gateway.example.com"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected non-local http public URL to be rejected")
	}

	cfg = DefaultConfig()
	cfg.PublicBaseURL = "http://127.0.0.1:19090"
	cfg.MCPURL = "http://127.0.0.1:19090/mcp"
	cfg.Auth.TokenFile = "/tmp/gateway-token"
	cfg.RelayProfiles = []RelayProfile{{
		ID:        "personal",
		URL:       "http://127.0.0.1:19091",
		TokenFile: "/tmp/relay-token",
		Enabled:   true,
	}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected localhost config to validate: %v", err)
	}
	redacted := RedactedConfig(cfg)
	if redacted.Auth.TokenFile != "<redacted-token-file>" {
		t.Fatalf("gateway token file not redacted: %#v", redacted.Auth.TokenFile)
	}
	if redacted.RelayProfiles[0].TokenFile != "<redacted-token-file>" {
		t.Fatalf("relay token file not redacted: %#v", redacted.RelayProfiles[0].TokenFile)
	}
}

func TestGatewayMCPToolsAggregateForwardAndSanitize(t *testing.T) {
	relay := newFakeRelay(t, fakeRelayOptions{})
	defer relay.Close()
	server := newGatewayTestServer(t, []RelayProfile{{
		ID:       "personal",
		Name:     "Personal",
		URL:      relay.URL,
		TokenEnv: "CODENCER_TEST_RELAY_TOKEN",
		Enabled:  true,
	}})
	defer server.Close()

	session := initializeMCP(t, server.URL)
	tools := mcpToolCall(t, server.URL, session, "codencer.list_relays", nil)
	body := mustJSON(t, tools)
	if !strings.Contains(body, "codencer") && !strings.Contains(body, "personal") {
		t.Fatalf("expected relay listing, got %s", body)
	}
	if strings.Contains(body, "relay-secret") {
		t.Fatalf("relay token leaked in list_relays: %s", body)
	}

	projects := mcpToolCall(t, server.URL, session, "codencer.list_projects", nil)
	projectBody := mustJSON(t, projects)
	if !strings.Contains(projectBody, `"project_id":"codencer"`) {
		t.Fatalf("expected codencer project, got %s", projectBody)
	}
	if strings.Contains(projectBody, "/Users/") || strings.Contains(projectBody, "/home/") {
		t.Fatalf("absolute path leaked in list_projects: %s", projectBody)
	}

	run := mcpToolCall(t, server.URL, session, "codencer.run_project_manifest", map[string]any{
		"relay_profile_id": "personal",
		"project_id":       "codencer",
		"machine_id":       "mach-1",
		"manifest":         map[string]any{"version": "v0.3", "tasks": []any{}},
		"wait":             true,
	})
	runBody := mustJSON(t, run)
	if !strings.Contains(runBody, `"run_id":"run-gateway-test"`) {
		t.Fatalf("expected forwarded run id, got %s", runBody)
	}
	if strings.Contains(runBody, "/Users/") || strings.Contains(runBody, "relay-secret") {
		t.Fatalf("sensitive relay output leaked: %s", runBody)
	}
	if relay.lastMachineID != "mach-1" {
		t.Fatalf("expected machine_id selector to reach relay, got %q", relay.lastMachineID)
	}

	report := mcpToolCall(t, server.URL, session, "codencer.get_run_report", map[string]any{
		"relay_profile_id": "personal",
		"project_id":       "codencer",
		"run_id":           "run-gateway-test",
	})
	reportBody := mustJSON(t, report)
	if !strings.Contains(reportBody, `"status":"completed"`) {
		t.Fatalf("expected run report, got %s", reportBody)
	}

	locations := mcpToolCall(t, server.URL, session, "codencer.list_project_locations", map[string]any{
		"relay_profile_id": "personal",
		"project_id":       "codencer",
		"host_label":       "macbook",
	})
	locationsBody := mustJSON(t, locations)
	if !strings.Contains(locationsBody, `"host_label":"macbook"`) {
		t.Fatalf("expected filtered project locations, got %s", locationsBody)
	}
	if strings.Contains(locationsBody, "/Users/") || strings.Contains(locationsBody, "relay-secret") {
		t.Fatalf("sensitive location output leaked: %s", locationsBody)
	}

	submit := mcpToolCall(t, server.URL, session, "codencer.submit_project_task_and_wait", map[string]any{
		"relay_profile_id": "personal",
		"project_id":       "codencer",
		"host_label":       "macbook",
		"goal":             "Return deterministic evidence.",
	})
	submitBody := mustJSON(t, submit)
	if !strings.Contains(submitBody, `"step_id":"step-gateway-test"`) {
		t.Fatalf("expected forwarded submit response, got %s", submitBody)
	}
	if relay.lastHostLabel != "macbook" {
		t.Fatalf("expected host_label selector to reach relay, got %q", relay.lastHostLabel)
	}

	blocker := mcpToolCall(t, server.URL, session, "codencer.get_blocker", map[string]any{
		"relay_profile_id": "personal",
		"project_id":       "codencer",
		"run_id":           "run-blocked",
	})
	blockerBody := mustJSON(t, blocker)
	if !strings.Contains(blockerBody, `"type":"needs_planner_decision"`) {
		t.Fatalf("expected blocker report, got %s", blockerBody)
	}
}

func TestGatewayAuthMetadataAndChallenge(t *testing.T) {
	server := newGatewayTestServer(t, []RelayProfile{})
	defer server.Close()

	protected, err := http.Get(server.URL + "/.well-known/oauth-protected-resource/mcp")
	if err != nil {
		t.Fatal(err)
	}
	defer protected.Body.Close()
	if protected.StatusCode != http.StatusOK {
		t.Fatalf("protected resource status=%d body=%s", protected.StatusCode, readBody(t, protected))
	}
	protectedBody := readBody(t, protected)
	if !strings.Contains(protectedBody, "Codencer Gateway MCP") || !strings.Contains(protectedBody, "/mcp") {
		t.Fatalf("protected resource metadata missing Gateway MCP details: %s", protectedBody)
	}

	authorization, err := http.Get(server.URL + "/.well-known/oauth-authorization-server")
	if err != nil {
		t.Fatal(err)
	}
	defer authorization.Body.Close()
	if authorization.StatusCode != http.StatusOK {
		t.Fatalf("authorization server status=%d body=%s", authorization.StatusCode, readBody(t, authorization))
	}
	authorizationBody := readBody(t, authorization)
	if !strings.Contains(authorizationBody, "/oauth/authorize") || !strings.Contains(authorizationBody, "/oauth/token") {
		t.Fatalf("authorization server metadata missing OAuth endpoints: %s", authorizationBody)
	}

	req, err := http.NewRequest(http.MethodPost, server.URL+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":"init","method":"initialize","params":{}}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized MCP response, got status=%d body=%s", resp.StatusCode, readBody(t, resp))
	}
	challenge := resp.Header.Get("WWW-Authenticate")
	if !strings.Contains(challenge, "Bearer") || !strings.Contains(challenge, "resource_metadata") {
		t.Fatalf("missing Gateway bearer challenge metadata: %q", challenge)
	}
}

func TestGatewayAmbiguityAndRelayUnavailableBlockers(t *testing.T) {
	relayA := newFakeRelay(t, fakeRelayOptions{})
	defer relayA.Close()
	relayB := newFakeRelay(t, fakeRelayOptions{})
	defer relayB.Close()
	server := newGatewayTestServer(t, []RelayProfile{
		{ID: "a", URL: relayA.URL, TokenEnv: "CODENCER_TEST_RELAY_TOKEN", Enabled: true},
		{ID: "b", URL: relayB.URL, TokenEnv: "CODENCER_TEST_RELAY_TOKEN", Enabled: true},
	})
	defer server.Close()

	session := initializeMCP(t, server.URL)
	ambiguousRelay := mcpToolCall(t, server.URL, session, "codencer.run_project_manifest", map[string]any{
		"project_id": "codencer",
		"manifest":   map[string]any{"version": "v0.3"},
	})
	assertToolErrorContains(t, ambiguousRelay, "ambiguous_relay_profile")

	multiLocationRelay := newFakeRelay(t, fakeRelayOptions{multipleLocations: true})
	defer multiLocationRelay.Close()
	locationServer := newGatewayTestServer(t, []RelayProfile{{
		ID:       "personal",
		URL:      multiLocationRelay.URL,
		TokenEnv: "CODENCER_TEST_RELAY_TOKEN",
		Enabled:  true,
	}})
	defer locationServer.Close()
	locationSession := initializeMCP(t, locationServer.URL)
	ambiguousLocation := mcpToolCall(t, locationServer.URL, locationSession, "codencer.run_project_manifest", map[string]any{
		"relay_profile_id": "personal",
		"project_id":       "codencer",
		"manifest":         map[string]any{"version": "v0.3"},
	})
	assertToolErrorContains(t, ambiguousLocation, "ambiguous_project_location")

	downServer := newGatewayTestServer(t, []RelayProfile{{
		ID:       "down",
		URL:      "http://127.0.0.1:1",
		TokenEnv: "CODENCER_TEST_RELAY_TOKEN",
		Enabled:  true,
	}})
	defer downServer.Close()
	downSession := initializeMCP(t, downServer.URL)
	relayDown := mcpToolCall(t, downServer.URL, downSession, "codencer.run_project_manifest", map[string]any{
		"relay_profile_id": "down",
		"project_id":       "codencer",
		"manifest":         map[string]any{"version": "v0.3"},
	})
	assertToolErrorContains(t, relayDown, "relay_unavailable")
}

func TestGatewayStoreDeviceLoginRelayRegistryAndConnectorBinding(t *testing.T) {
	relay := newFakeRelay(t, fakeRelayOptions{allowEnrollment: true})
	defer relay.Close()
	t.Setenv("CODENCER_DEFAULT_RELAY_TOKEN", "relay-secret")
	t.Setenv("CODENCER_SELFHOST_RELAY_TOKEN", "relay-secret")

	cfg := DefaultConfig()
	cfg.PublicBaseURL = "http://127.0.0.1:19090"
	cfg.MCPURL = "http://127.0.0.1:19090/mcp"
	cfg.Store.Path = filepath.Join(t.TempDir(), "gateway.db")
	cfg.DefaultRelay.URL = relay.URL
	cfg.DefaultRelay.TokenEnv = "CODENCER_DEFAULT_RELAY_TOKEN"
	cfg.OAuthDev.Issuer = "http://127.0.0.1:19090"
	cfg.OAuthDev.OperatorCodeHash = sha256Hex("operator-code")
	server, err := NewServer(cfg, ServerOptions{})
	if err != nil {
		t.Fatalf("new gateway server: %v", err)
	}
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	auth := apiPost[DeviceAuthorization](t, httpServer.URL+"/api/gateway/v1/device/authorize", "", map[string]any{
		"email":        "dev@example.com",
		"display_name": "Developer",
	})
	if auth.DeviceCode == "" || auth.UserCode == "" {
		t.Fatalf("device authorization missing codes: %+v", auth)
	}
	apiPost[map[string]any](t, httpServer.URL+"/api/gateway/v1/device/approve", "", map[string]any{"user_code": auth.UserCode})
	token := apiPost[TokenResponse](t, httpServer.URL+"/api/gateway/v1/device/token", "", map[string]any{"device_code": auth.DeviceCode})
	if token.AccessToken == "" || token.UserID == "" || token.WorkspaceID == "" {
		t.Fatalf("token was not workspace-bound: %+v", token)
	}

	who := apiGet[map[string]any](t, httpServer.URL+"/api/gateway/v1/whoami", token.AccessToken)
	if who["workspace_id"] != token.WorkspaceID {
		t.Fatalf("whoami workspace = %v want %s", who["workspace_id"], token.WorkspaceID)
	}
	relays := apiGet[map[string]any](t, httpServer.URL+"/api/gateway/v1/relays", token.AccessToken)
	relayBody := mustJSON(t, relays)
	if !strings.Contains(relayBody, `"id":"default"`) || strings.Contains(relayBody, "relay-secret") {
		t.Fatalf("default relay missing or token leaked: %s", relayBody)
	}
	selfHost := apiPost[map[string]any](t, httpServer.URL+"/api/gateway/v1/relays", token.AccessToken, map[string]any{
		"name":      "Personal",
		"url":       relay.URL,
		"token_env": "CODENCER_SELFHOST_RELAY_TOKEN",
	})
	selfHostBody := mustJSON(t, selfHost)
	if !strings.Contains(selfHostBody, `"token_configured":true`) || strings.Contains(selfHostBody, "relay-secret") {
		t.Fatalf("self-host relay response unsafe: %s", selfHostBody)
	}

	connectorLogin := apiPost[map[string]any](t, httpServer.URL+"/api/gateway/v1/connectors/login", token.AccessToken, map[string]any{
		"relay": "default",
		"machine": map[string]any{
			"machine_id": "mach-local",
			"hostname":   "dev-host",
			"host_label": "macbook",
			"os":         "darwin",
			"arch":       "arm64",
		},
	})
	if connectorLogin["enrollment_token"] != "enroll-secret" {
		t.Fatalf("connector login did not return enrollment secret to authenticated CLI: %s", mustJSON(t, connectorLogin))
	}
	apiPost[map[string]any](t, httpServer.URL+"/api/gateway/v1/connectors/complete", token.AccessToken, map[string]any{
		"binding_id":         connectorLogin["binding_id"],
		"relay_connector_id": "conn-relay",
		"relay_machine_id":   "mach-relay",
		"public_key":         "public-key",
	})

	workspace := apiGet[map[string]any](t, httpServer.URL+"/api/gateway/v1/workspace", token.AccessToken)
	workspaceBody := mustJSON(t, workspace)
	if !strings.Contains(workspaceBody, `"mcp_url"`) || strings.Contains(workspaceBody, "relay-secret") {
		t.Fatalf("workspace response missing metadata or leaked token: %s", workspaceBody)
	}
	health := apiGet[map[string]any](t, httpServer.URL+"/api/gateway/v1/relays/default/health", token.AccessToken)
	if !strings.Contains(mustJSON(t, health), `"status":"available"`) {
		t.Fatalf("expected default relay health to be available, got %s", mustJSON(t, health))
	}
	machines := apiGet[map[string]any](t, httpServer.URL+"/api/gateway/v1/machines", token.AccessToken)
	if !strings.Contains(mustJSON(t, machines), `"host_label":"macbook"`) {
		t.Fatalf("machines endpoint missing connector machine: %s", mustJSON(t, machines))
	}
	connectors := apiGet[map[string]any](t, httpServer.URL+"/api/gateway/v1/connectors", token.AccessToken)
	if !strings.Contains(mustJSON(t, connectors), `"status":"online"`) || strings.Contains(mustJSON(t, connectors), "public-key") {
		t.Fatalf("connectors endpoint missing online binding or leaked public key: %s", mustJSON(t, connectors))
	}
	projects := apiGet[map[string]any](t, httpServer.URL+"/api/gateway/v1/projects", token.AccessToken)
	projectBody := mustJSON(t, projects)
	if !strings.Contains(projectBody, `"project_id":"codencer"`) {
		t.Fatalf("projects endpoint missing relay project: %s", projectBody)
	}
	if strings.Contains(projectBody, "/Users/") || strings.Contains(projectBody, "relay-secret") {
		t.Fatalf("projects endpoint leaked sensitive relay fields: %s", projectBody)
	}
	project := apiGet[map[string]any](t, httpServer.URL+"/api/gateway/v1/projects/codencer", token.AccessToken)
	if !strings.Contains(mustJSON(t, project), `"relay_profile_id":"default"`) {
		t.Fatalf("project detail missing relay profile: %s", mustJSON(t, project))
	}
	audit := apiGet[map[string]any](t, httpServer.URL+"/api/gateway/v1/audit-events", token.AccessToken)
	if !strings.Contains(mustJSON(t, audit), `"connector.login"`) {
		t.Fatalf("audit endpoint missing connector event: %s", mustJSON(t, audit))
	}
	activation := apiGet[map[string]any](t, httpServer.URL+"/api/gateway/v1/activation/commands", token.AccessToken)
	activationBody := mustJSON(t, activation)
	if !strings.Contains(activationBody, "codencer login --gateway") || strings.Contains(activationBody, "relay-secret") {
		t.Fatalf("activation commands missing login or leaked token: %s", activationBody)
	}
	verifier := "test-code-verifier"
	oauthQuery := "?response_type=code&client_id=codencer-chatgpt-dev&redirect_uri=http%3A%2F%2F127.0.0.1%2Fcallback&scope=projects%3Aread+projects%3Awrite&state=state-1&code_challenge=" + codeChallengeS256(verifier) + "&code_challenge_method=S256&resource=http%3A%2F%2F127.0.0.1%3A19090%2Fmcp"
	oauthRequest := apiGet[map[string]any](t, httpServer.URL+"/api/gateway/v1/oauth/consent"+oauthQuery, "")
	if !strings.Contains(mustJSON(t, oauthRequest), `"client_id":"codencer-chatgpt-dev"`) {
		t.Fatalf("oauth consent request missing client: %s", mustJSON(t, oauthRequest))
	}
	oauthDecision := apiPost[map[string]any](t, httpServer.URL+"/api/gateway/v1/oauth/consent", "", map[string]any{
		"decision":              "approve",
		"operator_code":         "operator-code",
		"response_type":         "code",
		"client_id":             "codencer-chatgpt-dev",
		"redirect_uri":          "http://127.0.0.1/callback",
		"scope":                 "projects:read projects:write",
		"state":                 "state-1",
		"code_challenge":        codeChallengeS256(verifier),
		"code_challenge_method": "S256",
		"resource":              "http://127.0.0.1:19090/mcp",
	})
	if !strings.Contains(mustJSON(t, oauthDecision), `"authorization_code_issued":true`) {
		t.Fatalf("oauth consent did not issue authorization code: %s", mustJSON(t, oauthDecision))
	}

	readOnlyToken, err := server.store.CreateAccessToken(context.Background(), token.UserID, token.WorkspaceID, "test-readonly", cfg.MCPURL, []string{"projects:read"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	resp := apiRaw(t, httpServer.URL+"/api/gateway/v1/relays", readOnlyToken.AccessToken, map[string]any{"name": "denied", "url": relay.URL, "token_env": "CODENCER_SELFHOST_RELAY_TOKEN"})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected read-only relay add to be forbidden, status=%d body=%s", resp.StatusCode, readBody(t, resp))
	}
}

type fakeRelayOptions struct {
	multipleLocations bool
	allowEnrollment   bool
}

type fakeRelay struct {
	*httptest.Server
	lastMachineID string
	lastHostLabel string
}

func newFakeRelay(t *testing.T, opts fakeRelayOptions) *fakeRelay {
	t.Helper()
	relay := &fakeRelay{}
	mux := http.NewServeMux()
	project := func() map[string]any {
		locations := []map[string]any{{
			"machine_id":     "mach-1",
			"host_label":     "macbook",
			"hostname":       "dev-host",
			"connector_id":   "connector-1",
			"repo_root":      "/Users/example/codencer",
			"online":         true,
			"status":         "available",
			"daemon_url":     "http://127.0.0.1:18085",
			"planner_token":  "relay-secret",
			"absolute_extra": "/Users/example/codencer",
		}}
		if opts.multipleLocations {
			locations = append(locations, map[string]any{
				"machine_id": "mach-2",
				"host_label": "linux",
				"online":     true,
				"status":     "available",
			})
		}
		return map[string]any{
			"project_id": "codencer",
			"name":       "Codencer",
			"online":     true,
			"status":     "available",
			"locations":  locations,
			"repo_root":  "/Users/example/codencer",
			"token":      "relay-secret",
		}
	}
	mux.HandleFunc("/api/v2/status", func(w http.ResponseWriter, r *http.Request) {
		requireRelayAuth(t, r)
		writeTestJSON(t, w, map[string]any{"ok": true})
	})
	mux.HandleFunc("/api/v2/connectors/enrollment-tokens", func(w http.ResponseWriter, r *http.Request) {
		requireRelayAuth(t, r)
		if !opts.allowEnrollment {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeTestJSON(t, w, map[string]any{"secret": "enroll-secret"})
	})
	mux.HandleFunc("/api/v2/projects", func(w http.ResponseWriter, r *http.Request) {
		requireRelayAuth(t, r)
		writeTestJSON(t, w, []map[string]any{project()})
	})
	mux.HandleFunc("/api/v2/projects/codencer", func(w http.ResponseWriter, r *http.Request) {
		requireRelayAuth(t, r)
		writeTestJSON(t, w, project())
	})
	mux.HandleFunc("/api/v2/projects/codencer/run-plan", func(w http.ResponseWriter, r *http.Request) {
		requireRelayAuth(t, r)
		relay.lastMachineID = r.URL.Query().Get("machine_id")
		writeTestJSON(t, w, map[string]any{
			"ok":        true,
			"run_id":    "run-gateway-test",
			"repo_root": "/Users/example/codencer",
			"token":     "relay-secret",
		})
	})
	mux.HandleFunc("/api/v2/projects/codencer/submit", func(w http.ResponseWriter, r *http.Request) {
		requireRelayAuth(t, r)
		relay.lastHostLabel = r.URL.Query().Get("host_label")
		writeTestJSON(t, w, map[string]any{"ok": true, "step_id": "step-gateway-test"})
	})
	mux.HandleFunc("/api/v2/projects/codencer/reports/run-plans/run-gateway-test", func(w http.ResponseWriter, r *http.Request) {
		requireRelayAuth(t, r)
		writeTestJSON(t, w, map[string]any{"run_id": "run-gateway-test", "status": "completed"})
	})
	mux.HandleFunc("/api/v2/projects/codencer/reports/run-plans/run-blocked", func(w http.ResponseWriter, r *http.Request) {
		requireRelayAuth(t, r)
		writeTestJSON(t, w, map[string]any{
			"run_id": "run-blocked",
			"status": "blocked",
			"blocker": map[string]any{
				"type":                      "needs_planner_decision",
				"planner_decision_required": true,
			},
		})
	})
	relay.Server = httptest.NewServer(mux)
	return relay
}

func requireRelayAuth(t *testing.T, r *http.Request) {
	t.Helper()
	if got := r.Header.Get("Authorization"); got != "Bearer relay-secret" {
		t.Fatalf("expected relay bearer token, got %q", got)
	}
}

func newGatewayTestServer(t *testing.T, profiles []RelayProfile) *httptest.Server {
	t.Helper()
	t.Setenv("CODENCER_TEST_GATEWAY_TOKEN", "gateway-secret")
	t.Setenv("CODENCER_TEST_RELAY_TOKEN", "relay-secret")
	cfg := DefaultConfig()
	cfg.PublicBaseURL = "http://127.0.0.1:19090"
	cfg.MCPURL = "http://127.0.0.1:19090/mcp"
	cfg.Auth.TokenEnv = "CODENCER_TEST_GATEWAY_TOKEN"
	cfg.OAuthDev.Enabled = true
	cfg.OAuthDev.Issuer = "http://127.0.0.1:19090"
	cfg.RelayProfiles = profiles
	server, err := NewServer(cfg, ServerOptions{})
	if err != nil {
		t.Fatalf("new gateway server: %v", err)
	}
	return httptest.NewServer(server.Handler())
}

func initializeMCP(t *testing.T, baseURL string) string {
	t.Helper()
	resp := doMCPRequest(t, baseURL, "", map[string]any{
		"jsonrpc": "2.0",
		"id":      "init",
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-11-25",
			"clientInfo":      map[string]any{"name": "gateway-test", "version": "v0"},
		},
	})
	sessionID := resp.Header.Get(headerSessionID)
	if sessionID == "" {
		t.Fatalf("initialize did not return session id: %s", readBody(t, resp))
	}
	return sessionID
}

func mcpToolCall(t *testing.T, baseURL, sessionID, name string, args map[string]any) map[string]any {
	t.Helper()
	if args == nil {
		args = map[string]any{}
	}
	resp := doMCPRequest(t, baseURL, sessionID, map[string]any{
		"jsonrpc": "2.0",
		"id":      name,
		"method":  "tools/call",
		"params":  map[string]any{"name": name, "arguments": args},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("tool call status=%d body=%s", resp.StatusCode, readBody(t, resp))
	}
	var decoded map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode MCP response: %v", err)
	}
	return decoded
}

func doMCPRequest(t *testing.T, baseURL, sessionID string, payload map[string]any) *http.Response {
	t.Helper()
	data, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, baseURL+"/mcp", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+os.Getenv("CODENCER_TEST_GATEWAY_TOKEN"))
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerProtocolVersion, "2025-11-25")
	if sessionID != "" {
		req.Header.Set(headerSessionID, sessionID)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("MCP request status=%d body=%s", resp.StatusCode, readBody(t, resp))
	}
	return resp
}

func assertToolErrorContains(t *testing.T, response map[string]any, want string) {
	t.Helper()
	body := mustJSON(t, response)
	if !strings.Contains(body, `"isError":true`) || !strings.Contains(body, want) {
		t.Fatalf("expected tool error containing %q, got %s", want, body)
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func codeChallengeS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func writeTestJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatal(err)
	}
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(resp.Body)
	return buf.String()
}

func apiGet[T any](t *testing.T, url, token string) T {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		t.Fatalf("GET %s status=%d body=%s", url, resp.StatusCode, readBody(t, resp))
	}
	var out T
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode GET %s: %v", url, err)
	}
	return out
}

func apiPost[T any](t *testing.T, url, token string, payload map[string]any) T {
	t.Helper()
	resp := apiRaw(t, url, token, payload)
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		t.Fatalf("POST %s status=%d body=%s", url, resp.StatusCode, readBody(t, resp))
	}
	var out T
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode POST %s: %v", url, err)
	}
	return out
}

func apiRaw(t *testing.T, url, token string, payload map[string]any) *http.Response {
	t.Helper()
	data, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}
