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
	cfg.RelayRequestTimeoutSeconds = 0
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
	if cfg.RelayRequestTimeoutSeconds != DefaultRelayRequestTimeoutSeconds {
		t.Fatalf("expected default relay request timeout %d, got %d", DefaultRelayRequestTimeoutSeconds, cfg.RelayRequestTimeoutSeconds)
	}
	redacted := RedactedConfig(cfg)
	if redacted.Auth.TokenFile != "<redacted-token-file>" {
		t.Fatalf("gateway token file not redacted: %#v", redacted.Auth.TokenFile)
	}
	if redacted.RelayProfiles[0].TokenFile != "<redacted-token-file>" {
		t.Fatalf("relay token file not redacted: %#v", redacted.RelayProfiles[0].TokenFile)
	}

	cfg.RelayRequestTimeoutSeconds = 240
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected custom relay request timeout to validate: %v", err)
	}
	if cfg.RelayRequestTimeoutSeconds != 240 {
		t.Fatalf("custom relay request timeout was not preserved: %d", cfg.RelayRequestTimeoutSeconds)
	}
}

func TestNewServerUsesConfiguredRelayRequestTimeout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PublicBaseURL = "http://127.0.0.1:19090"
	cfg.MCPURL = "http://127.0.0.1:19090/mcp"
	cfg.RelayRequestTimeoutSeconds = 240
	server, err := NewServer(cfg, ServerOptions{})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if server.client.Timeout != 240*time.Second {
		t.Fatalf("expected configured client timeout, got %s", server.client.Timeout)
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
	assertNoGatewayMCPLeak(t, runBody)
	if !strings.Contains(runBody, `"artifact_id":"artifact-run"`) || !strings.Contains(runBody, `"hash":"hash-run"`) {
		t.Fatalf("safe artifact metadata was not preserved in run response: %s", runBody)
	}
	if strings.Contains(runBody, `"state":"running"`) || !strings.Contains(runBody, `"state":"completed"`) {
		t.Fatalf("run response exposed contradictory state: %s", runBody)
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
	assertNoGatewayMCPLeak(t, reportBody)
	if !strings.Contains(reportBody, `"artifact_id":"artifact-report"`) || !strings.Contains(reportBody, `"mime_type":"text/plain"`) {
		t.Fatalf("safe artifact metadata was not preserved in run report: %s", reportBody)
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
		"adapter":          "claude",
		"profile":          "claude-default",
		"adapter_profile":  "claude-default",
		"goal":             "Return deterministic evidence.",
	})
	submitBody := mustJSON(t, submit)
	if !strings.Contains(submitBody, `"step_id":"step-gateway-test"`) {
		t.Fatalf("expected forwarded submit response, got %s", submitBody)
	}
	assertNoGatewayMCPLeak(t, submitBody)
	if relay.lastHostLabel != "macbook" {
		t.Fatalf("expected host_label selector to reach relay, got %q", relay.lastHostLabel)
	}
	if relay.lastSubmitRequest["adapter"] != "claude" || relay.lastSubmitRequest["profile"] != "claude-default" || relay.lastSubmitRequest["adapter_profile"] != "claude-default" {
		t.Fatalf("expected selected executor metadata to reach relay, got %+v", relay.lastSubmitRequest)
	}
	submitRunID := runIDFromPayload(mcpStructuredContent(t, submit))
	if submitRunID == "" {
		t.Fatalf("submit response did not expose a run_id for report lookup: %s", submitBody)
	}
	submitReport := mcpToolCall(t, server.URL, session, "codencer.get_run_report", map[string]any{
		"relay_profile_id": "personal",
		"project_id":       "codencer",
		"run_id":           submitRunID,
	})
	submitReportBody := mustJSON(t, submitReport)
	if !strings.Contains(submitReportBody, `"status":"completed"`) || !strings.Contains(submitReportBody, `"run_id":"`+submitRunID+`"`) {
		t.Fatalf("expected simple submit run report, got %s", submitReportBody)
	}
	assertNoGatewayMCPLeak(t, submitReportBody)

	blocker := mcpToolCall(t, server.URL, session, "codencer.get_blocker", map[string]any{
		"relay_profile_id": "personal",
		"project_id":       "codencer",
		"run_id":           "run-blocked",
	})
	blockerBody := mustJSON(t, blocker)
	if !strings.Contains(blockerBody, `"type":"needs_planner_decision"`) {
		t.Fatalf("expected blocker report, got %s", blockerBody)
	}
	assertNoGatewayMCPLeak(t, blockerBody)
}

func TestGatewayMCPAsyncLifecycleTools(t *testing.T) {
	relay := newFakeRelay(t, fakeRelayOptions{})
	defer relay.Close()
	server, _ := newGatewayStoreAPIServer(t, relay.URL)
	defer server.Close()

	session := initializeMCP(t, server.URL)
	toolsResp := doMCPRequest(t, server.URL, session, map[string]any{
		"jsonrpc": "2.0",
		"id":      "tools",
		"method":  "tools/list",
		"params":  map[string]any{},
	})
	var toolsList map[string]any
	if err := json.NewDecoder(toolsResp.Body).Decode(&toolsList); err != nil {
		t.Fatalf("decode tools/list: %v", err)
	}
	toolsBody := mustJSON(t, toolsList)
	for _, want := range []string{
		"codencer.start_project_run",
		"codencer.submit_project_task",
		"codencer.list_project_runs",
		"codencer.get_project_run",
		"codencer.get_project_run_status",
		"codencer.get_gateway_run_events",
		"codencer.respond_to_human_interrupt",
		"codencer.cancel_project_run",
		"codencer.resume_project_run",
	} {
		if !strings.Contains(toolsBody, want) {
			t.Fatalf("tools/list missing %s: %s", want, toolsBody)
		}
	}

	started := mcpToolCall(t, server.URL, session, "codencer.start_project_run", map[string]any{
		"relay_profile_id": "default",
		"project_id":       "codencer",
		"machine_id":       "mach-1",
	})
	startedBody := mustJSON(t, started)
	if !strings.Contains(startedBody, `"run_id":"run-async-gateway-test"`) || !strings.Contains(startedBody, `"status":"running"`) {
		t.Fatalf("expected async start response, got %s", startedBody)
	}
	assertNoGatewayMCPLeak(t, startedBody)
	startedPayload, _ := mcpStructuredContent(t, started).(map[string]any)
	startedRunHistoryID := stringValueFromAny(startedPayload["run_history_id"])
	if startedRunHistoryID == "" {
		t.Fatalf("start response did not return run_history_id: %s", startedBody)
	}

	asyncSubmit := mcpToolCall(t, server.URL, session, "codencer.submit_project_task", map[string]any{
		"relay_profile_id": "default",
		"project_id":       "codencer",
		"host_label":       "macbook",
		"title":            "Async Gateway task",
		"goal":             "Return after submission without waiting.",
	})
	asyncSubmitBody := mustJSON(t, asyncSubmit)
	if !strings.Contains(asyncSubmitBody, `"status":"submitted"`) || strings.Contains(asyncSubmitBody, `"status":"completed"`) {
		t.Fatalf("expected non-blocking submitted task response, got %s", asyncSubmitBody)
	}
	assertNoGatewayMCPLeak(t, asyncSubmitBody)
	asyncPayload, _ := mcpStructuredContent(t, asyncSubmit).(map[string]any)
	runHistoryID := stringValueFromAny(asyncPayload["run_history_id"])
	if runHistoryID == "" {
		t.Fatalf("async submit did not return run_history_id: %s", asyncSubmitBody)
	}

	runs := mcpToolCall(t, server.URL, session, "codencer.list_project_runs", map[string]any{
		"relay_profile_id": "default",
		"project_id":       "codencer",
		"machine_id":       "mach-1",
	})
	runsBody := mustJSON(t, runs)
	if !strings.Contains(runsBody, `"run_id":"run-async-gateway-test"`) {
		t.Fatalf("expected async run listing, got %s", runsBody)
	}
	assertNoGatewayMCPLeak(t, runsBody)

	status := mcpToolCall(t, server.URL, session, "codencer.get_project_run_status", map[string]any{
		"relay_profile_id": "default",
		"project_id":       "codencer",
		"run_id":           "run-async-gateway-test",
		"machine_id":       "mach-1",
	})
	statusBody := mustJSON(t, status)
	if !strings.Contains(statusBody, `"state":"running"`) {
		t.Fatalf("expected async run status, got %s", statusBody)
	}
	assertNoGatewayMCPLeak(t, statusBody)

	events := mcpToolCall(t, server.URL, session, "codencer.get_gateway_run_events", map[string]any{
		"run_history_id": runHistoryID,
		"limit":          10,
	})
	eventsBody := mustJSON(t, events)
	if !strings.Contains(eventsBody, `"task_submitted"`) || !strings.Contains(eventsBody, `"run_started"`) || !strings.Contains(eventsBody, `"pagination"`) {
		t.Fatalf("expected Gateway run events, got %s", eventsBody)
	}
	assertNoGatewayMCPLeak(t, eventsBody)

	blocked := mcpToolCall(t, server.URL, session, "codencer.submit_project_task", map[string]any{
		"relay_profile_id": "default",
		"project_id":       "codencer",
		"machine_id":       "mach-1",
		"title":            "Blocked Gateway task",
		"goal":             "Ask for a safe operator decision.",
	})
	blockedBody := mustJSON(t, blocked)
	blockedPayload, _ := mcpStructuredContent(t, blocked).(map[string]any)
	blockedRunHistoryID := stringValueFromAny(blockedPayload["run_history_id"])
	if blockedRunHistoryID == "" || !strings.Contains(blockedBody, `"status":"blocked"`) || !strings.Contains(blockedBody, `"type":"question"`) {
		t.Fatalf("expected blocked run response with history id, got %s", blockedBody)
	}
	assertNoGatewayMCPLeak(t, blockedBody)

	response := mcpToolCall(t, server.URL, session, "codencer.respond_to_human_interrupt", map[string]any{
		"run_history_id": blockedRunHistoryID,
		"response_type":  "answer",
		"response":       "Use the documented safe path, not /Users/example/private token=relay-secret.",
		"follow_up":      "resume",
	})
	responseBody := mustJSON(t, response)
	if !strings.Contains(responseBody, `"status":"human_interrupt_response_recorded"`) ||
		!strings.Contains(responseBody, `"resume_supported":true`) ||
		!strings.Contains(responseBody, `"resume_attempted":true`) ||
		!strings.Contains(responseBody, `"follow_up":"resume"`) ||
		!strings.Contains(responseBody, `"follow_up_result"`) ||
		!strings.Contains(responseBody, `"run_resumed"`) ||
		!strings.Contains(responseBody, `"operator_response"`) {
		t.Fatalf("expected recorded human interrupt response, got %s", responseBody)
	}
	assertNoGatewayMCPLeak(t, responseBody)

	blockedEvents := mcpToolCall(t, server.URL, session, "codencer.get_gateway_run_events", map[string]any{
		"run_history_id": blockedRunHistoryID,
		"limit":          20,
	})
	blockedEventsBody := mustJSON(t, blockedEvents)
	if !strings.Contains(blockedEventsBody, `"human_interrupt_created"`) ||
		!strings.Contains(blockedEventsBody, `"human_interrupt_responded"`) ||
		!strings.Contains(blockedEventsBody, `"resume_project_run_requested"`) ||
		!strings.Contains(blockedEventsBody, `"run_resumed"`) ||
		!strings.Contains(blockedEventsBody, `"operator_response"`) {
		t.Fatalf("expected human interrupt response audit event, got %s", blockedEventsBody)
	}
	assertNoGatewayMCPLeak(t, blockedEventsBody)

	blockedCancel := mcpToolCall(t, server.URL, session, "codencer.submit_project_task", map[string]any{
		"relay_profile_id": "default",
		"project_id":       "codencer",
		"machine_id":       "mach-1",
		"title":            "Blocked Gateway cancel task",
		"goal":             "Ask for a safe operator cancellation.",
	})
	blockedCancelBody := mustJSON(t, blockedCancel)
	blockedCancelPayload, _ := mcpStructuredContent(t, blockedCancel).(map[string]any)
	blockedCancelRunHistoryID := stringValueFromAny(blockedCancelPayload["run_history_id"])
	if blockedCancelRunHistoryID == "" || !strings.Contains(blockedCancelBody, `"status":"blocked"`) || !strings.Contains(blockedCancelBody, `"type":"question"`) {
		t.Fatalf("expected blocked cancel run response with history id, got %s", blockedCancelBody)
	}
	assertNoGatewayMCPLeak(t, blockedCancelBody)

	cancelResponse := mcpToolCall(t, server.URL, session, "codencer.respond_to_human_interrupt", map[string]any{
		"run_history_id": blockedCancelRunHistoryID,
		"response_type":  "deny",
		"response":       "Cancel this run without using /Users/example/private token=relay-secret.",
		"follow_up":      "cancel",
	})
	cancelResponseBody := mustJSON(t, cancelResponse)
	if !strings.Contains(cancelResponseBody, `"status":"human_interrupt_response_recorded"`) ||
		!strings.Contains(cancelResponseBody, `"cancel_supported":true`) ||
		!strings.Contains(cancelResponseBody, `"cancel_attempted":true`) ||
		!strings.Contains(cancelResponseBody, `"follow_up":"cancel"`) ||
		!strings.Contains(cancelResponseBody, `"follow_up_result"`) ||
		!strings.Contains(cancelResponseBody, `"run_cancelled"`) ||
		!strings.Contains(cancelResponseBody, `"operator_response"`) {
		t.Fatalf("expected recorded cancel human interrupt response, got %s", cancelResponseBody)
	}
	assertNoGatewayMCPLeak(t, cancelResponseBody)

	blockedCancelEvents := mcpToolCall(t, server.URL, session, "codencer.get_gateway_run_events", map[string]any{
		"run_history_id": blockedCancelRunHistoryID,
		"limit":          20,
	})
	blockedCancelEventsBody := mustJSON(t, blockedCancelEvents)
	if !strings.Contains(blockedCancelEventsBody, `"human_interrupt_created"`) ||
		!strings.Contains(blockedCancelEventsBody, `"human_interrupt_responded"`) ||
		!strings.Contains(blockedCancelEventsBody, `"cancel_project_run_requested"`) ||
		!strings.Contains(blockedCancelEventsBody, `"run_cancelled"`) ||
		!strings.Contains(blockedCancelEventsBody, `"operator_response"`) {
		t.Fatalf("expected human interrupt cancel audit event, got %s", blockedCancelEventsBody)
	}
	assertNoGatewayMCPLeak(t, blockedCancelEventsBody)

	blockedStart := mcpToolCall(t, server.URL, session, "codencer.submit_project_task", map[string]any{
		"relay_profile_id": "default",
		"project_id":       "codencer",
		"machine_id":       "mach-1",
		"title":            "Blocked Gateway start task",
		"goal":             "Ask for a safe replacement task.",
	})
	blockedStartBody := mustJSON(t, blockedStart)
	blockedStartPayload, _ := mcpStructuredContent(t, blockedStart).(map[string]any)
	blockedStartRunHistoryID := stringValueFromAny(blockedStartPayload["run_history_id"])
	if blockedStartRunHistoryID == "" || !strings.Contains(blockedStartBody, `"status":"blocked"`) || !strings.Contains(blockedStartBody, `"type":"question"`) {
		t.Fatalf("expected blocked start-new-task run response with history id, got %s", blockedStartBody)
	}
	assertNoGatewayMCPLeak(t, blockedStartBody)

	startResponse := mcpToolCall(t, server.URL, session, "codencer.respond_to_human_interrupt", map[string]any{
		"run_history_id": blockedStartRunHistoryID,
		"response_type":  "decision",
		"response":       "Start a new safe task without using /Users/example/private token=relay-secret.",
		"follow_up":      "start_new_task",
		"new_task_title": "Gateway follow-up task",
		"new_task_goal":  "Run a safe follow-up task that does not expose local paths.",
	})
	startResponseBody := mustJSON(t, startResponse)
	if !strings.Contains(startResponseBody, `"status":"human_interrupt_response_recorded"`) ||
		!strings.Contains(startResponseBody, `"start_new_task_supported":true`) ||
		!strings.Contains(startResponseBody, `"start_new_task_attempted":true`) ||
		!strings.Contains(startResponseBody, `"follow_up":"start_new_task"`) ||
		!strings.Contains(startResponseBody, `"follow_up_result"`) ||
		!strings.Contains(startResponseBody, `"run-followup"`) ||
		!strings.Contains(startResponseBody, `"operator_response"`) {
		t.Fatalf("expected recorded start_new_task human interrupt response, got %s", startResponseBody)
	}
	assertNoGatewayMCPLeak(t, startResponseBody)

	blockedStartEvents := mcpToolCall(t, server.URL, session, "codencer.get_gateway_run_events", map[string]any{
		"run_history_id": blockedStartRunHistoryID,
		"limit":          20,
	})
	blockedStartEventsBody := mustJSON(t, blockedStartEvents)
	if !strings.Contains(blockedStartEventsBody, `"human_interrupt_created"`) ||
		!strings.Contains(blockedStartEventsBody, `"human_interrupt_responded"`) ||
		!strings.Contains(blockedStartEventsBody, `"start_new_task_requested"`) ||
		!strings.Contains(blockedStartEventsBody, `"operator_response"`) {
		t.Fatalf("expected human interrupt start_new_task audit event, got %s", blockedStartEventsBody)
	}
	assertNoGatewayMCPLeak(t, blockedStartEventsBody)

	cancel := mcpToolCall(t, server.URL, session, "codencer.cancel_project_run", map[string]any{
		"relay_profile_id": "default",
		"project_id":       "codencer",
		"run_id":           "run-async-gateway-test",
		"run_history_id":   startedRunHistoryID,
	})
	cancelBody := mustJSON(t, cancel)
	if !strings.Contains(cancelBody, `"status":"cancelled"`) || !strings.Contains(cancelBody, `"run_id":"run-async-gateway-test"`) {
		t.Fatalf("expected forwarded cancel response, got %s", cancelBody)
	}
	assertNoGatewayMCPLeak(t, cancelBody)
	cancelEvents := mcpToolCall(t, server.URL, session, "codencer.get_gateway_run_events", map[string]any{
		"run_history_id": startedRunHistoryID,
		"limit":          20,
	})
	cancelEventsBody := mustJSON(t, cancelEvents)
	if !strings.Contains(cancelEventsBody, `"cancel_project_run_requested"`) || !strings.Contains(cancelEventsBody, `"run_cancelled"`) {
		t.Fatalf("expected cancel lifecycle audit events, got %s", cancelEventsBody)
	}
	assertNoGatewayMCPLeak(t, cancelEventsBody)

	resume := mcpToolCall(t, server.URL, session, "codencer.resume_project_run", map[string]any{
		"relay_profile_id": "default",
		"project_id":       "codencer",
		"run_id":           "run-async-gateway-test",
		"run_history_id":   startedRunHistoryID,
	})
	resumeBody := mustJSON(t, resume)
	if !strings.Contains(resumeBody, `"run_resumed"`) || !strings.Contains(resumeBody, `"state":"running"`) {
		t.Fatalf("expected forwarded resume response, got %s", resumeBody)
	}
	assertNoGatewayMCPLeak(t, resumeBody)
	resumeEvents := mcpToolCall(t, server.URL, session, "codencer.get_gateway_run_events", map[string]any{
		"run_history_id": startedRunHistoryID,
		"limit":          20,
	})
	resumeEventsBody := mustJSON(t, resumeEvents)
	for _, want := range []string{`"resume_project_run_requested"`, `"run_resumed"`} {
		if !strings.Contains(resumeEventsBody, want) {
			t.Fatalf("resume run events missing %s: %s", want, resumeEventsBody)
		}
	}
	assertNoGatewayMCPLeak(t, resumeEventsBody)
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

func TestGatewayConsoleCollectionResponsesUseEmptyArrays(t *testing.T) {
	relay := newProjectListRelay(t, []map[string]any{})
	defer relay.Close()
	httpServer, token := newGatewayStoreAPIServer(t, relay.URL)
	defer httpServer.Close()

	for _, tc := range []struct {
		path string
		keys []string
	}{
		{path: "/api/gateway/v1/relays", keys: []string{"relays"}},
		{path: "/api/gateway/v1/machines", keys: []string{"machines"}},
		{path: "/api/gateway/v1/connectors", keys: []string{"connectors"}},
		{path: "/api/gateway/v1/executors", keys: []string{"executors"}},
		{path: "/api/gateway/v1/projects", keys: []string{"projects", "relay_errors"}},
		{path: "/api/gateway/v1/runs", keys: []string{"runs"}},
		{path: "/api/gateway/v1/audit-events", keys: []string{"audit_events", "events"}},
		{path: "/api/gateway/v1/activation/commands", keys: []string{"activation_commands", "commands"}},
	} {
		t.Run(tc.path, func(t *testing.T) {
			payload := apiGet[map[string]any](t, httpServer.URL+tc.path, token.AccessToken)
			body := mustJSON(t, payload)
			if strings.Contains(body, ":null") {
				t.Fatalf("collection response serialized null: %s", body)
			}
			for _, key := range tc.keys {
				values := requireJSONArray(t, payload, key)
				if tc.path != "/api/gateway/v1/activation/commands" && tc.path != "/api/gateway/v1/relays" && tc.path != "/api/gateway/v1/executors" && len(values) != 0 {
					t.Fatalf("%s expected empty array for %q, got %s", tc.path, key, body)
				}
				if tc.path == "/api/gateway/v1/activation/commands" && len(values) == 0 {
					t.Fatalf("%s expected activation commands for %q, got %s", tc.path, key, body)
				}
				if tc.path == "/api/gateway/v1/executors" {
					if len(values) == 0 {
						t.Fatalf("%s expected executor profiles for %q, got %s", tc.path, key, body)
					}
					for _, want := range []string{"codex-workspace", "codex-full", "claude-default", "fake-success"} {
						if !strings.Contains(body, `"`+want+`"`) {
							t.Fatalf("%s missing executor %s: %s", tc.path, want, body)
						}
					}
					assertNoGatewayConsoleSensitiveLeak(t, body)
				}
			}
		})
	}
}

func TestGatewayConsoleCollectionsSynthesizeRelayLocationMetadata(t *testing.T) {
	relay := newFakeRelay(t, fakeRelayOptions{})
	defer relay.Close()
	httpServer, token := newGatewayStoreAPIServer(t, relay.URL)
	defer httpServer.Close()

	projects := apiGet[map[string]any](t, httpServer.URL+"/api/gateway/v1/projects", token.AccessToken)
	projectBody := mustJSON(t, projects)
	if !strings.Contains(projectBody, `"connector_id":"connector-1"`) || !strings.Contains(projectBody, `"machine_id":"mach-1"`) {
		t.Fatalf("project location did not include live relay metadata: %s", projectBody)
	}

	machines := apiGet[map[string]any](t, httpServer.URL+"/api/gateway/v1/machines", token.AccessToken)
	machineBody := mustJSON(t, machines)
	if !strings.Contains(machineBody, `"id":"mach-1"`) || !strings.Contains(machineBody, `"host_label":"macbook"`) || !strings.Contains(machineBody, `"hostname":"dev-host"`) || !strings.Contains(machineBody, `"status":"online"`) {
		t.Fatalf("machines endpoint did not synthesize safe relay metadata: %s", machineBody)
	}
	assertNoGatewayConsoleSensitiveLeak(t, machineBody)

	connectors := apiGet[map[string]any](t, httpServer.URL+"/api/gateway/v1/connectors", token.AccessToken)
	connectorBody := mustJSON(t, connectors)
	if !strings.Contains(connectorBody, `"id":"connector-1"`) || !strings.Contains(connectorBody, `"machine_id":"mach-1"`) || !strings.Contains(connectorBody, `"relay_profile_id":"default"`) || !strings.Contains(connectorBody, `"status":"online"`) {
		t.Fatalf("connectors endpoint did not synthesize safe relay metadata: %s", connectorBody)
	}
	assertNoGatewayConsoleSensitiveLeak(t, connectorBody)
}

func TestGatewayStoreDeviceLoginRelayRegistryAndConnectorBinding(t *testing.T) {
	relay := newFakeRelay(t, fakeRelayOptions{allowEnrollment: true, defaultAdapter: "codex", adapterProfile: "codex-workspace"})
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
	runResult := apiPost[map[string]any](t, httpServer.URL+"/api/gateway/v1/projects/codencer/runs", token.AccessToken, map[string]any{
		"adapter":          "claude",
		"profile":          "claude-default",
		"adapter_profile":  "claude-default",
		"relay_profile_id": "default",
		"machine_id":       "mach-1",
		"title":            "Gateway Console task",
		"goal":             "Return deterministic evidence.",
		"timeout_seconds":  30,
	})
	runResultBody := mustJSON(t, runResult)
	if !strings.Contains(runResultBody, `"step_id":"step-gateway-test"`) {
		t.Fatalf("project run endpoint did not submit through relay: %s", runResultBody)
	}
	if relay.lastSubmitRequest["adapter"] != "claude" || relay.lastSubmitRequest["profile"] != "claude-default" || relay.lastSubmitRequest["adapter_profile"] != "claude-default" {
		t.Fatalf("project run endpoint did not forward selected executor metadata: %+v", relay.lastSubmitRequest)
	}
	runHistoryID, _ := runResult["run_history_id"].(string)
	if runHistoryID == "" {
		t.Fatalf("project run endpoint did not return run_history_id: %s", runResultBody)
	}
	assertNoGatewayMCPLeak(t, runResultBody)
	asyncRunResult := apiPost[map[string]any](t, httpServer.URL+"/api/gateway/v1/projects/codencer/runs", token.AccessToken, map[string]any{
		"relay_profile_id": "default",
		"machine_id":       "mach-1",
		"title":            "Async Gateway Console task",
		"goal":             "Return immediately and refresh by report polling.",
		"timeout_seconds":  30,
		"wait":             false,
	})
	asyncRunBody := mustJSON(t, asyncRunResult)
	if !strings.Contains(asyncRunBody, `"status":"submitted"`) || strings.Contains(asyncRunBody, `"status":"completed"`) || !strings.Contains(asyncRunBody, `"run_id":"run-async-console"`) {
		t.Fatalf("project run endpoint did not honor wait=false: %s", asyncRunBody)
	}
	asyncRunHistoryID, _ := asyncRunResult["run_history_id"].(string)
	if asyncRunHistoryID == "" {
		t.Fatalf("async project run endpoint did not return run_history_id: %s", asyncRunBody)
	}
	assertNoGatewayMCPLeak(t, asyncRunBody)
	asyncReport := apiGet[map[string]any](t, httpServer.URL+"/api/gateway/v1/projects/codencer/runs/run-async-console/report?relay_profile_id=default&machine_id=mach-1", token.AccessToken)
	asyncReportBody := mustJSON(t, asyncReport)
	if !strings.Contains(asyncReportBody, `"status":"completed"`) || !strings.Contains(asyncReportBody, `"run_id":"run-async-console"`) {
		t.Fatalf("async project run report endpoint did not return terminal report: %s", asyncReportBody)
	}
	if asyncReport["run_history_id"] != asyncRunHistoryID {
		t.Fatalf("async report endpoint returned run_history_id=%v want %s: %s", asyncReport["run_history_id"], asyncRunHistoryID, asyncReportBody)
	}
	assertNoGatewayMCPLeak(t, asyncReportBody)
	asyncManifestResult := apiPost[map[string]any](t, httpServer.URL+"/api/gateway/v1/projects/codencer/runs", token.AccessToken, map[string]any{
		"relay_profile_id": "default",
		"machine_id":       "mach-1",
		"manifest_name":    "Async Gateway Console manifest",
		"manifest_text":    "version: codencer.io/v1alpha1\nkind: RunManifest\n",
		"mode":             "manifest",
		"wait":             false,
	})
	asyncManifestBody := mustJSON(t, asyncManifestResult)
	if !strings.Contains(asyncManifestBody, `"status":"submitted"`) || strings.Contains(asyncManifestBody, `"status":"completed"`) || !strings.Contains(asyncManifestBody, `"run_id":"run-async-manifest"`) {
		t.Fatalf("manifest project run endpoint did not honor wait=false: %s", asyncManifestBody)
	}
	asyncManifestHistoryID, _ := asyncManifestResult["run_history_id"].(string)
	if asyncManifestHistoryID == "" {
		t.Fatalf("async manifest endpoint did not return run_history_id: %s", asyncManifestBody)
	}
	assertNoGatewayMCPLeak(t, asyncManifestBody)
	asyncManifestReport := apiGet[map[string]any](t, httpServer.URL+"/api/gateway/v1/projects/codencer/runs/run-async-manifest/report?relay_profile_id=default&machine_id=mach-1", token.AccessToken)
	asyncManifestReportBody := mustJSON(t, asyncManifestReport)
	if !strings.Contains(asyncManifestReportBody, `"status":"completed"`) || !strings.Contains(asyncManifestReportBody, `"run_id":"run-async-manifest"`) {
		t.Fatalf("async manifest report endpoint did not return terminal report: %s", asyncManifestReportBody)
	}
	if asyncManifestReport["run_history_id"] != asyncManifestHistoryID {
		t.Fatalf("async manifest report endpoint returned run_history_id=%v want %s: %s", asyncManifestReport["run_history_id"], asyncManifestHistoryID, asyncManifestReportBody)
	}
	assertNoGatewayMCPLeak(t, asyncManifestReportBody)
	cancelResult := apiPost[map[string]any](t, httpServer.URL+"/api/gateway/v1/projects/codencer/runs/run-async-console/cancel?relay_profile_id=default&machine_id=mach-1", token.AccessToken, map[string]any{
		"reason": "operator requested stop",
	})
	cancelResultBody := mustJSON(t, cancelResult)
	if !strings.Contains(cancelResultBody, `"status":"cancelled"`) || !strings.Contains(cancelResultBody, `"run_id":"run-async-console"`) {
		t.Fatalf("project run cancel endpoint did not cancel through relay: %s", cancelResultBody)
	}
	if cancelResult["run_history_id"] != asyncRunHistoryID {
		t.Fatalf("cancel endpoint returned run_history_id=%v want %s: %s", cancelResult["run_history_id"], asyncRunHistoryID, cancelResultBody)
	}
	assertNoGatewayMCPLeak(t, cancelResultBody)
	report := apiGet[map[string]any](t, httpServer.URL+"/api/gateway/v1/projects/codencer/runs/run-gateway-test/report?relay_profile_id=default&machine_id=mach-1", token.AccessToken)
	reportBody := mustJSON(t, report)
	if !strings.Contains(reportBody, `"status":"completed"`) || !strings.Contains(reportBody, `"run_id":"run-gateway-test"`) {
		t.Fatalf("project run report endpoint did not fetch report: %s", reportBody)
	}
	if !strings.Contains(reportBody, `"adapter":"claude"`) || !strings.Contains(reportBody, `"profile":"claude-default"`) || strings.Contains(reportBody, `"profile":"codex-workspace"`) {
		t.Fatalf("project run report endpoint did not preserve selected Claude report metadata: %s", reportBody)
	}
	if report["run_history_id"] != runHistoryID {
		t.Fatalf("report endpoint returned run_history_id=%v want %s: %s", report["run_history_id"], runHistoryID, reportBody)
	}
	assertNoGatewayMCPLeak(t, reportBody)
	conflict := apiRaw(t, httpServer.URL+"/api/gateway/v1/projects/codencer/runs", token.AccessToken, map[string]any{
		"adapter":          "codex",
		"profile":          "claude-default",
		"relay_profile_id": "default",
		"machine_id":       "mach-1",
		"title":            "Conflicting executor metadata",
		"goal":             "This should not execute.",
		"wait":             true,
	})
	conflictBody := readBody(t, conflict)
	conflict.Body.Close()
	if conflict.StatusCode != http.StatusBadRequest || !strings.Contains(conflictBody, "invalid_input") || strings.Contains(conflictBody, "step-gateway-test") {
		t.Fatalf("executor adapter/profile conflict should return invalid_input before execution, status=%d body=%s", conflict.StatusCode, conflictBody)
	}
	syncResult := apiPost[map[string]any](t, httpServer.URL+"/api/gateway/v1/sync/runs", token.AccessToken, map[string]any{
		"mode":  "metadata_only",
		"scope": "local",
		"projects": []map[string]any{{
			"id":         "codencer",
			"name":       "Codencer",
			"profile":    "codex-workspace",
			"machine_id": "mach-sync",
			"host_label": "sync-host",
		}},
		"runs": []map[string]any{{
			"run_id":             "run-synced-extra",
			"project_id":         "codencer",
			"title":              "Synced metadata-only run",
			"status":             "completed",
			"report_status":      "local",
			"summary":            "Synced from /Users/example/private token=secret-token",
			"executor_profile":   "codex-workspace",
			"mode":               "task",
			"scope":              "local",
			"execution_mode":     "real",
			"safe_artifact_refs": []string{"stdout.log", "/Users/example/private/report.json", "http://127.0.0.1:18085/raw.log"},
		}},
	})
	syncResultBody := mustJSON(t, syncResult)
	if !strings.Contains(syncResultBody, `"scope":"synced"`) || !strings.Contains(syncResultBody, `"synced_runs":1`) {
		t.Fatalf("sync ingest response wrong: %s", syncResultBody)
	}
	assertNoGatewayConsoleSensitiveLeak(t, syncResultBody)
	syncedIDs, _ := syncResult["run_history_ids"].([]any)
	if len(syncedIDs) != 1 {
		t.Fatalf("sync ingest missing run history id: %s", syncResultBody)
	}
	syncedRunHistoryID, _ := syncedIDs[0].(string)
	if syncedRunHistoryID == "" {
		t.Fatalf("sync ingest returned blank run history id: %s", syncResultBody)
	}
	syncPublishAudit := apiGet[map[string]any](t, httpServer.URL+"/api/gateway/v1/audit-events?type=sync.publish", token.AccessToken)
	syncPublishAuditBody := mustJSON(t, syncPublishAudit)
	if !strings.Contains(syncPublishAuditBody, `"type":"sync.publish"`) ||
		!strings.Contains(syncPublishAuditBody, `"scope":"synced"`) ||
		!strings.Contains(syncPublishAuditBody, `"mode":"metadata_only"`) ||
		!strings.Contains(syncPublishAuditBody, `"run_count":1`) {
		t.Fatalf("sync publish audit missing aggregate metadata: %s", syncPublishAuditBody)
	}
	assertNoGatewayConsoleSensitiveLeak(t, syncPublishAuditBody)
	syncRunAudit := apiGet[map[string]any](t, httpServer.URL+"/api/gateway/v1/audit-events?type=sync.run_published&run_history_id="+syncedRunHistoryID, token.AccessToken)
	syncRunAuditBody := mustJSON(t, syncRunAudit)
	if !strings.Contains(syncRunAuditBody, `"type":"sync.run_published"`) ||
		!strings.Contains(syncRunAuditBody, `"run_history_id":"`+syncedRunHistoryID+`"`) ||
		!strings.Contains(syncRunAuditBody, `"run_id":"run-synced-extra"`) ||
		!strings.Contains(syncRunAuditBody, `"scope":"synced"`) {
		t.Fatalf("sync run audit missing per-run metadata: %s", syncRunAuditBody)
	}
	assertNoGatewayConsoleSensitiveLeak(t, syncRunAuditBody)
	rawSync := apiRaw(t, httpServer.URL+"/api/gateway/v1/sync/runs", token.AccessToken, map[string]any{
		"mode":          "metadata_only",
		"scope":         "local",
		"projects":      []map[string]any{},
		"runs":          []map[string]any{},
		"raw_artifacts": []string{"must-not-upload"},
	})
	rawSyncBody := readBody(t, rawSync)
	rawSync.Body.Close()
	if rawSync.StatusCode != http.StatusBadRequest || !strings.Contains(rawSyncBody, "unknown field") {
		t.Fatalf("sync ingest should reject unknown raw artifact fields, status=%d body=%s", rawSync.StatusCode, rawSyncBody)
	}
	runs := apiGet[map[string]any](t, httpServer.URL+"/api/gateway/v1/runs", token.AccessToken)
	runsBody := mustJSON(t, runs)
	if !strings.Contains(runsBody, `"id":"`+runHistoryID+`"`) ||
		!strings.Contains(runsBody, `"run_id":"run-gateway-test"`) ||
		!strings.Contains(runsBody, `"title":"Gateway Console task"`) ||
		!strings.Contains(runsBody, `"goal":"Return deterministic evidence."`) ||
		!strings.Contains(runsBody, `"executor_adapter":"claude"`) ||
		!strings.Contains(runsBody, `"executor_profile":"claude-default"`) ||
		!strings.Contains(runsBody, `"scope":"gateway_submitted"`) ||
		!strings.Contains(runsBody, `"relay_profile_id":"default"`) ||
		!strings.Contains(runsBody, `"connector_id":"connector-1"`) ||
		!strings.Contains(runsBody, `"machine_id":"mach-1"`) ||
		!strings.Contains(runsBody, `"result_summary":"Artifacts: stdout.log"`) {
		t.Fatalf("run history list missing expected safe metadata: %s", runsBody)
	}
	if !strings.Contains(runsBody, `"pagination":`) || !strings.Contains(runsBody, `"has_more":false`) {
		t.Fatalf("run history list missing pagination metadata: %s", runsBody)
	}
	assertNoGatewayConsoleSensitiveLeak(t, runsBody)
	pagedRuns := apiGet[map[string]any](t, httpServer.URL+"/api/gateway/v1/runs?limit=1", token.AccessToken)
	pagedRunsBody := mustJSON(t, pagedRuns)
	if !strings.Contains(pagedRunsBody, `"has_more":true`) || !strings.Contains(pagedRunsBody, `"next_offset":1`) {
		t.Fatalf("run history pagination did not expose next page: %s", pagedRunsBody)
	}
	syncedRuns := apiGet[map[string]any](t, httpServer.URL+"/api/gateway/v1/runs?scope=synced", token.AccessToken)
	if syncedRunsBody := mustJSON(t, syncedRuns); !strings.Contains(syncedRunsBody, `"id":"`+syncedRunHistoryID+`"`) || !strings.Contains(syncedRunsBody, `"run_id":"run-synced-extra"`) || !strings.Contains(syncedRunsBody, `redacted-local-path`) || strings.Contains(syncedRunsBody, `"id":"`+runHistoryID+`"`) || strings.Contains(syncedRunsBody, `/Users/example`) || strings.Contains(syncedRunsBody, `secret-token`) || strings.Contains(syncedRunsBody, `127.0.0.1:18085`) {
		t.Fatalf("run history scope filter returned wrong runs: %s", syncedRunsBody)
	}
	scopedRuns := apiGet[map[string]any](t, httpServer.URL+"/api/gateway/v1/runs?scope=gateway_submitted", token.AccessToken)
	if scopedRunsBody := mustJSON(t, scopedRuns); !strings.Contains(scopedRunsBody, `"id":"`+runHistoryID+`"`) {
		t.Fatalf("run history scope filter missing expected run: %s", scopedRunsBody)
	}
	runDetail := apiGet[map[string]any](t, httpServer.URL+"/api/gateway/v1/runs/"+runHistoryID, token.AccessToken)
	runDetailBody := mustJSON(t, runDetail)
	if !strings.Contains(runDetailBody, `"run_id":"run-gateway-test"`) ||
		!strings.Contains(runDetailBody, `"scope":"gateway_submitted"`) ||
		!strings.Contains(runDetailBody, `"executor_adapter":"claude"`) ||
		!strings.Contains(runDetailBody, `"executor_profile":"claude-default"`) ||
		!strings.Contains(runDetailBody, `"report_status":"completed"`) ||
		!strings.Contains(runDetailBody, `"result_details":"Artifacts: stdout.log"`) {
		t.Fatalf("run history detail missing expected result metadata: %s", runDetailBody)
	}
	assertNoGatewayConsoleSensitiveLeak(t, runDetailBody)
	runEvents := apiGet[map[string]any](t, httpServer.URL+"/api/gateway/v1/runs/"+runHistoryID+"/events", token.AccessToken)
	runEventsBody := mustJSON(t, runEvents)
	for _, eventType := range []string{"task_submitted", "route_resolved", "relay_selected", "connector_selected", "executor_selected", "run_started", "run_completed", "report_read"} {
		if !strings.Contains(runEventsBody, `"`+eventType+`"`) {
			t.Fatalf("run event history missing %s event: %s", eventType, runEventsBody)
		}
	}
	if !strings.Contains(runEventsBody, `"run_history_id":"`+runHistoryID+`"`) {
		t.Fatalf("run event history missing run metadata: %s", runEventsBody)
	}
	if !strings.Contains(runEventsBody, `"executor_adapter":"claude"`) || !strings.Contains(runEventsBody, `"executor_profile":"claude-default"`) || strings.Contains(runEventsBody, `"executor_profile":"codex-workspace"`) {
		t.Fatalf("run event history did not preserve selected Claude executor metadata: %s", runEventsBody)
	}
	if !strings.Contains(runEventsBody, `"groups":`) || !strings.Contains(runEventsBody, `"event_count":8`) || !strings.Contains(runEventsBody, `"pagination":`) {
		t.Fatalf("run event history missing grouped audit or pagination metadata: %s", runEventsBody)
	}
	assertNoGatewayConsoleSensitiveLeak(t, runEventsBody)
	blockedRun := apiPost[map[string]any](t, httpServer.URL+"/api/gateway/v1/projects/codencer/runs", token.AccessToken, map[string]any{
		"relay_profile_id": "default",
		"machine_id":       "mach-1",
		"title":            "Blocked Gateway task",
		"goal":             "Ask for a safe operator decision.",
		"timeout_seconds":  30,
	})
	blockedBody := mustJSON(t, blockedRun)
	blockedRunHistoryID, _ := blockedRun["run_history_id"].(string)
	if blockedRunHistoryID == "" || !strings.Contains(blockedBody, `"status":"blocked"`) || !strings.Contains(blockedBody, `"type":"question"`) {
		t.Fatalf("blocked run did not return blocker and history id: %s", blockedBody)
	}
	assertNoGatewayConsoleSensitiveLeak(t, blockedBody)
	blockedEvents := apiGet[map[string]any](t, httpServer.URL+"/api/gateway/v1/runs/"+blockedRunHistoryID+"/events", token.AccessToken)
	blockedEventsBody := mustJSON(t, blockedEvents)
	for _, want := range []string{`"type":"blocker"`, `"type":"human_interrupt_created"`, `"interrupt_type":"clarifying_question_required"`, `"status":"waiting_for_human"`, `"requested_action":"answer_question"`} {
		if !strings.Contains(blockedEventsBody, want) {
			t.Fatalf("blocked run events missing %s: %s", want, blockedEventsBody)
		}
	}
	if strings.Contains(blockedEventsBody, "/Users/example") || strings.Contains(blockedEventsBody, "relay-secret") || strings.Contains(blockedEventsBody, "/tmp/codencer/secret") {
		t.Fatalf("blocked run events leaked sensitive data: %s", blockedEventsBody)
	}
	interruptResponse := apiPost[map[string]any](t, httpServer.URL+"/api/gateway/v1/runs/"+blockedRunHistoryID+"/human-interrupts/respond", token.AccessToken, map[string]any{
		"response_type": "answer",
		"response":      "Use the documented safe path, not /Users/example/private token=relay-secret.",
		"follow_up":     "resume",
		"reason":        "Operator reviewed the blocker.",
	})
	interruptResponseBody := mustJSON(t, interruptResponse)
	if !strings.Contains(interruptResponseBody, `"status":"human_interrupt_response_recorded"`) ||
		!strings.Contains(interruptResponseBody, `"resume_supported":true`) ||
		!strings.Contains(interruptResponseBody, `"resume_attempted":true`) ||
		!strings.Contains(interruptResponseBody, `"follow_up":"resume"`) ||
		!strings.Contains(interruptResponseBody, `"follow_up_result"`) ||
		!strings.Contains(interruptResponseBody, `"run_resumed"`) ||
		!strings.Contains(interruptResponseBody, `"operator_response"`) {
		t.Fatalf("human interrupt response endpoint missing expected payload: %s", interruptResponseBody)
	}
	assertNoGatewayConsoleSensitiveLeak(t, interruptResponseBody)
	blockedEventsAfterResponse := apiGet[map[string]any](t, httpServer.URL+"/api/gateway/v1/runs/"+blockedRunHistoryID+"/events", token.AccessToken)
	blockedEventsAfterResponseBody := mustJSON(t, blockedEventsAfterResponse)
	if !strings.Contains(blockedEventsAfterResponseBody, `"human_interrupt_responded"`) ||
		!strings.Contains(blockedEventsAfterResponseBody, `"resume_project_run_requested"`) ||
		!strings.Contains(blockedEventsAfterResponseBody, `"run_resumed"`) ||
		!strings.Contains(blockedEventsAfterResponseBody, `"operator_response"`) {
		t.Fatalf("blocked run events missing human interrupt response audit: %s", blockedEventsAfterResponseBody)
	}
	assertNoGatewayConsoleSensitiveLeak(t, blockedEventsAfterResponseBody)
	blockedCancelRun := apiPost[map[string]any](t, httpServer.URL+"/api/gateway/v1/projects/codencer/runs", token.AccessToken, map[string]any{
		"relay_profile_id": "default",
		"machine_id":       "mach-1",
		"title":            "Blocked Gateway cancel task",
		"goal":             "Ask for a safe operator cancellation.",
		"timeout_seconds":  30,
	})
	blockedCancelBody := mustJSON(t, blockedCancelRun)
	blockedCancelRunHistoryID, _ := blockedCancelRun["run_history_id"].(string)
	if blockedCancelRunHistoryID == "" || !strings.Contains(blockedCancelBody, `"status":"blocked"`) || !strings.Contains(blockedCancelBody, `"type":"question"`) {
		t.Fatalf("blocked cancel run did not return blocker and history id: %s", blockedCancelBody)
	}
	assertNoGatewayConsoleSensitiveLeak(t, blockedCancelBody)
	cancelInterruptResponse := apiPost[map[string]any](t, httpServer.URL+"/api/gateway/v1/runs/"+blockedCancelRunHistoryID+"/human-interrupts/respond", token.AccessToken, map[string]any{
		"response_type": "deny",
		"response":      "Cancel this run without using /Users/example/private token=relay-secret.",
		"follow_up":     "cancel",
		"reason":        "Operator rejected the blocked run.",
	})
	cancelInterruptResponseBody := mustJSON(t, cancelInterruptResponse)
	if !strings.Contains(cancelInterruptResponseBody, `"status":"human_interrupt_response_recorded"`) ||
		!strings.Contains(cancelInterruptResponseBody, `"cancel_supported":true`) ||
		!strings.Contains(cancelInterruptResponseBody, `"cancel_attempted":true`) ||
		!strings.Contains(cancelInterruptResponseBody, `"follow_up":"cancel"`) ||
		!strings.Contains(cancelInterruptResponseBody, `"follow_up_result"`) ||
		!strings.Contains(cancelInterruptResponseBody, `"run_cancelled"`) ||
		!strings.Contains(cancelInterruptResponseBody, `"operator_response"`) {
		t.Fatalf("human interrupt cancel response endpoint missing expected payload: %s", cancelInterruptResponseBody)
	}
	assertNoGatewayConsoleSensitiveLeak(t, cancelInterruptResponseBody)
	blockedCancelEventsAfterResponse := apiGet[map[string]any](t, httpServer.URL+"/api/gateway/v1/runs/"+blockedCancelRunHistoryID+"/events", token.AccessToken)
	blockedCancelEventsAfterResponseBody := mustJSON(t, blockedCancelEventsAfterResponse)
	if !strings.Contains(blockedCancelEventsAfterResponseBody, `"human_interrupt_responded"`) ||
		!strings.Contains(blockedCancelEventsAfterResponseBody, `"cancel_project_run_requested"`) ||
		!strings.Contains(blockedCancelEventsAfterResponseBody, `"run_cancelled"`) ||
		!strings.Contains(blockedCancelEventsAfterResponseBody, `"operator_response"`) {
		t.Fatalf("blocked cancel run events missing human interrupt cancel audit: %s", blockedCancelEventsAfterResponseBody)
	}
	assertNoGatewayConsoleSensitiveLeak(t, blockedCancelEventsAfterResponseBody)
	blockedStartRun := apiPost[map[string]any](t, httpServer.URL+"/api/gateway/v1/projects/codencer/runs", token.AccessToken, map[string]any{
		"relay_profile_id": "default",
		"machine_id":       "mach-1",
		"title":            "Blocked Gateway start task",
		"goal":             "Ask for a safe replacement task.",
		"timeout_seconds":  30,
	})
	blockedStartBody := mustJSON(t, blockedStartRun)
	blockedStartRunHistoryID, _ := blockedStartRun["run_history_id"].(string)
	if blockedStartRunHistoryID == "" || !strings.Contains(blockedStartBody, `"status":"blocked"`) || !strings.Contains(blockedStartBody, `"type":"question"`) {
		t.Fatalf("blocked start-new-task run did not return blocker and history id: %s", blockedStartBody)
	}
	assertNoGatewayConsoleSensitiveLeak(t, blockedStartBody)
	startMissingGoalResponse := apiPost[map[string]any](t, httpServer.URL+"/api/gateway/v1/runs/"+blockedStartRunHistoryID+"/human-interrupts/respond", token.AccessToken, map[string]any{
		"response_type": "decision",
		"response":      "Start a different task, but no task goal was provided.",
		"follow_up":     "start_new_task",
		"reason":        "Operator requested a replacement task.",
	})
	startMissingGoalResponseBody := mustJSON(t, startMissingGoalResponse)
	if !strings.Contains(startMissingGoalResponseBody, `"status":"human_interrupt_response_recorded"`) ||
		!strings.Contains(startMissingGoalResponseBody, `"start_new_task_supported":true`) ||
		!strings.Contains(startMissingGoalResponseBody, `"start_new_task_attempted":true`) ||
		!strings.Contains(startMissingGoalResponseBody, `"follow_up":"start_new_task"`) ||
		!strings.Contains(startMissingGoalResponseBody, `"new_task_goal_required"`) ||
		!strings.Contains(startMissingGoalResponseBody, `"operator_response"`) {
		t.Fatalf("human interrupt start_new_task missing-goal response missing expected blocker: %s", startMissingGoalResponseBody)
	}
	assertNoGatewayConsoleSensitiveLeak(t, startMissingGoalResponseBody)
	blockedStartEventsAfterResponse := apiGet[map[string]any](t, httpServer.URL+"/api/gateway/v1/runs/"+blockedStartRunHistoryID+"/events", token.AccessToken)
	blockedStartEventsAfterResponseBody := mustJSON(t, blockedStartEventsAfterResponse)
	if !strings.Contains(blockedStartEventsAfterResponseBody, `"human_interrupt_responded"`) ||
		!strings.Contains(blockedStartEventsAfterResponseBody, `"start_new_task_blocked"`) ||
		!strings.Contains(blockedStartEventsAfterResponseBody, `"new_task_goal_required"`) ||
		!strings.Contains(blockedStartEventsAfterResponseBody, `"operator_response"`) {
		t.Fatalf("blocked start-new-task events missing structured blocker audit: %s", blockedStartEventsAfterResponseBody)
	}
	assertNoGatewayConsoleSensitiveLeak(t, blockedStartEventsAfterResponseBody)
	audit := apiGet[map[string]any](t, httpServer.URL+"/api/gateway/v1/audit-events", token.AccessToken)
	auditBody := mustJSON(t, audit)
	for _, eventType := range []string{
		"connector.login",
		"task_submitted",
		"route_resolved",
		"relay_selected",
		"connector_selected",
		"executor_selected",
		"run_started",
		"run_completed",
		"sync.publish",
		"sync.run_published",
		"human_interrupt_created",
		"human_interrupt_responded",
		"report_read",
	} {
		if !strings.Contains(auditBody, `"`+eventType+`"`) {
			t.Fatalf("audit endpoint missing %s event: %s", eventType, auditBody)
		}
	}
	assertNoGatewayConsoleSensitiveLeak(t, auditBody)
	if !strings.Contains(auditBody, `"run_history_id":"`+runHistoryID+`"`) {
		t.Fatalf("audit endpoint missing run history metadata: %s", auditBody)
	}
	if !strings.Contains(auditBody, `"executor_adapter":"claude"`) || !strings.Contains(auditBody, `"executor_profile":"claude-default"`) {
		t.Fatalf("audit endpoint did not preserve selected Claude executor metadata: %s", auditBody)
	}
	if !strings.Contains(auditBody, `"groups":`) || !strings.Contains(auditBody, `"event_count":8`) || !strings.Contains(auditBody, `"pagination":`) {
		t.Fatalf("audit endpoint missing grouped audit or pagination metadata: %s", auditBody)
	}
	pagedAudit := apiGet[map[string]any](t, httpServer.URL+"/api/gateway/v1/audit-events?limit=2", token.AccessToken)
	if pagedAuditBody := mustJSON(t, pagedAudit); !strings.Contains(pagedAuditBody, `"has_more":true`) || !strings.Contains(pagedAuditBody, `"next_offset":2`) {
		t.Fatalf("audit pagination did not expose next page: %s", pagedAuditBody)
	}
	filteredAudit := apiGet[map[string]any](t, httpServer.URL+"/api/gateway/v1/audit-events?type=run_completed&project_id=codencer&run_id=run-gateway-test&run_history_id="+runHistoryID, token.AccessToken)
	filteredAuditBody := mustJSON(t, filteredAudit)
	if !strings.Contains(filteredAuditBody, `"type":"run_completed"`) ||
		strings.Contains(filteredAuditBody, `"type":"run_started"`) ||
		!strings.Contains(filteredAuditBody, `"event_count":1`) {
		t.Fatalf("audit filters did not isolate the requested lifecycle event: %s", filteredAuditBody)
	}
	if strings.Contains(auditBody, "relay-secret") || strings.Contains(auditBody, relay.URL) {
		t.Fatalf("audit endpoint leaked relay secret or daemon URL: %s", auditBody)
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
	defaultAdapter    string
	adapterProfile    string
}

type fakeRelay struct {
	*httptest.Server
	lastMachineID     string
	lastHostLabel     string
	lastSubmitRequest map[string]any
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
			"project_id":      "codencer",
			"name":            "Codencer",
			"online":          true,
			"status":          "available",
			"default_adapter": firstNonEmpty(opts.defaultAdapter, "fake"),
			"adapter_profile": firstNonEmpty(opts.adapterProfile, "fake-success"),
			"locations":       locations,
			"repo_root":       "/Users/example/codencer",
			"token":           "relay-secret",
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
	mux.HandleFunc("/api/v2/projects/codencer/runs", func(w http.ResponseWriter, r *http.Request) {
		requireRelayAuth(t, r)
		relay.lastMachineID = r.URL.Query().Get("machine_id")
		switch r.Method {
		case http.MethodPost:
			writeTestJSON(t, w, map[string]any{
				"ok":     true,
				"status": "running",
				"run_id": "run-async-gateway-test",
				"run": map[string]any{
					"id":    "run-async-gateway-test",
					"state": "running",
				},
				"repo_root":   "/Users/example/codencer",
				"report_path": "/tmp/codencer/run-plans/run-async-gateway-test.json",
			})
		case http.MethodGet:
			writeTestJSON(t, w, map[string]any{
				"runs": []map[string]any{{
					"run_id": "run-async-gateway-test",
					"status": "running",
					"run": map[string]any{
						"id":    "run-async-gateway-test",
						"state": "running",
					},
					"repo_root": "/Users/example/codencer",
				}},
			})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/v2/projects/codencer/runs/run-async-gateway-test", func(w http.ResponseWriter, r *http.Request) {
		requireRelayAuth(t, r)
		writeTestJSON(t, w, map[string]any{
			"run_id": "run-async-gateway-test",
			"status": "running",
			"run": map[string]any{
				"id":    "run-async-gateway-test",
				"state": "running",
			},
			"repo_root":   "/Users/example/codencer",
			"report_path": "/tmp/codencer/run-plans/run-async-gateway-test.json",
		})
	})
	mux.HandleFunc("/api/v2/projects/codencer/runs/run-async-gateway-test/cancel", func(w http.ResponseWriter, r *http.Request) {
		requireRelayAuth(t, r)
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeTestJSON(t, w, map[string]any{
			"ok":     true,
			"run_id": "run-async-gateway-test",
			"status": "cancelled",
			"run": map[string]any{
				"id":    "run-async-gateway-test",
				"state": "cancelled",
			},
			"repo_root":   "/Users/example/codencer",
			"report_path": "/tmp/codencer/run-plans/run-async-gateway-test.json",
		})
	})
	mux.HandleFunc("/api/v2/projects/codencer/runs/run-async-gateway-test/resume", func(w http.ResponseWriter, r *http.Request) {
		requireRelayAuth(t, r)
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeTestJSON(t, w, map[string]any{
			"ok":     true,
			"run_id": "run-async-gateway-test",
			"status": "running",
			"run": map[string]any{
				"id":    "run-async-gateway-test",
				"state": "running",
			},
			"events": []map[string]any{{
				"type":   "run_resumed",
				"run_id": "run-async-gateway-test",
				"state":  "running",
			}},
			"repo_root":   "/Users/example/codencer",
			"report_path": "/tmp/codencer/run-plans/run-async-gateway-test.json",
		})
	})
	mux.HandleFunc("/api/v2/projects/codencer/runs/run-blocked/resume", func(w http.ResponseWriter, r *http.Request) {
		requireRelayAuth(t, r)
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeTestJSON(t, w, map[string]any{
			"ok":     true,
			"run_id": "run-blocked",
			"status": "running",
			"run": map[string]any{
				"id":    "run-blocked",
				"state": "running",
			},
			"events": []map[string]any{{
				"type":   "run_resumed",
				"run_id": "run-blocked",
				"state":  "running",
			}},
			"repo_root":   "/Users/example/codencer",
			"report_path": "/tmp/codencer/run-plans/run-blocked.json",
		})
	})
	mux.HandleFunc("/api/v2/projects/codencer/runs/run-blocked-cancel/cancel", func(w http.ResponseWriter, r *http.Request) {
		requireRelayAuth(t, r)
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeTestJSON(t, w, map[string]any{
			"ok":     true,
			"run_id": "run-blocked-cancel",
			"status": "cancelled",
			"run": map[string]any{
				"id":    "run-blocked-cancel",
				"state": "cancelled",
			},
			"events": []map[string]any{{
				"type":   "run_cancelled",
				"run_id": "run-blocked-cancel",
				"state":  "cancelled",
			}},
			"repo_root":   "/Users/example/codencer",
			"report_path": "/tmp/codencer/run-plans/run-blocked-cancel.json",
		})
	})
	mux.HandleFunc("/api/v2/projects/codencer/run-plan", func(w http.ResponseWriter, r *http.Request) {
		requireRelayAuth(t, r)
		relay.lastMachineID = r.URL.Query().Get("machine_id")
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req["manifest_name"] == "Async Gateway Console manifest" {
			if wait, _ := req["wait"].(bool); wait {
				t.Fatalf("expected async console manifest to forward wait=false")
			}
			writeTestJSON(t, w, map[string]any{
				"ok":     true,
				"status": "submitted",
				"run_id": "run-async-manifest",
				"run": map[string]any{
					"id":    "run-async-manifest",
					"state": "submitted",
				},
				"report_path": "/tmp/codencer/run-plans/run-async-manifest.json",
			})
			return
		}
		writeTestJSON(t, w, map[string]any{
			"ok":          true,
			"status":      "completed",
			"run_id":      "run-gateway-test",
			"repo_root":   "/Users/example/codencer",
			"report_path": "/tmp/codencer/run-plans/run-gateway-test.json",
			"run":         map[string]any{"id": "run-gateway-test", "state": "running"},
			"evidence": map[string]any{
				"logs_ref": "/var/folders/test/codencer/stdout.log",
				"artifacts": []map[string]any{{
					"id":        "artifact-run",
					"name":      "stdout.log",
					"type":      "stdout",
					"mime_type": "text/plain",
					"size":      12,
					"hash":      "hash-run",
					"path":      "/Users/example/.codencer-live-test/artifacts/stdout.log",
				}},
			},
			"token": "relay-secret",
		})
	})
	mux.HandleFunc("/api/v2/projects/codencer/submit", func(w http.ResponseWriter, r *http.Request) {
		requireRelayAuth(t, r)
		relay.lastHostLabel = r.URL.Query().Get("host_label")
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		relay.lastSubmitRequest = req
		if req["title"] == "Blocked Gateway task" || req["title"] == "Blocked Gateway cancel task" || req["title"] == "Blocked Gateway start task" {
			runID := "run-blocked"
			stepID := "step-blocked"
			if req["title"] == "Blocked Gateway cancel task" {
				runID = "run-blocked-cancel"
				stepID = "step-blocked-cancel"
			} else if req["title"] == "Blocked Gateway start task" {
				runID = "run-blocked-start"
				stepID = "step-blocked-start"
			}
			writeTestJSON(t, w, map[string]any{
				"ok":      false,
				"status":  "blocked",
				"run_id":  runID,
				"step_id": stepID,
				"blocker": map[string]any{
					"type":      "question",
					"message":   "Need operator choice from /Users/example/private token=relay-secret",
					"questions": []string{"Choose a safe path without exposing /tmp/codencer/secret"},
				},
				"task": map[string]any{
					"run_id": runID,
					"status": "blocked",
				},
			})
			return
		}
		if req["title"] == "Gateway follow-up task" {
			if wait, _ := req["wait"].(bool); wait {
				t.Fatalf("expected follow-up task to forward wait=false")
			}
			writeTestJSON(t, w, map[string]any{
				"ok":      true,
				"status":  "submitted",
				"run_id":  "run-followup",
				"step_id": "step-followup",
				"task": map[string]any{
					"run_id":  "run-followup",
					"status":  "submitted",
					"step_id": "step-followup",
				},
				"report_path": "/tmp/codencer/run-plans/run-followup.json",
			})
			return
		}
		if req["title"] == "Async Gateway Console task" {
			if wait, _ := req["wait"].(bool); wait {
				t.Fatalf("expected async console task to forward wait=false")
			}
			writeTestJSON(t, w, map[string]any{
				"ok":      true,
				"status":  "submitted",
				"run_id":  "run-async-console",
				"step_id": "step-async-console",
				"task": map[string]any{
					"run_id":  "run-async-console",
					"status":  "submitted",
					"step_id": "step-async-console",
				},
				"report_path": "/tmp/codencer/run-plans/run-async-console.json",
			})
			return
		}
		if wait, _ := req["wait"].(bool); !wait {
			writeTestJSON(t, w, map[string]any{
				"ok":      true,
				"status":  "submitted",
				"run_id":  "run-gateway-test",
				"step_id": "step-gateway-test",
				"task": map[string]any{
					"run_id": "run-gateway-test",
					"status": "submitted",
					"evidence": map[string]any{
						"logs_ref": "/Users/example/.codencer-live-test/runtime/daemon/state/artifacts/task.log",
					},
				},
				"report_path": "/tmp/codencer/run-plans/run-gateway-test.json",
			})
			return
		}
		writeTestJSON(t, w, map[string]any{
			"ok":      true,
			"status":  "completed",
			"step_id": "step-gateway-test",
			"task": map[string]any{
				"run_id": "run-gateway-test",
				"evidence": map[string]any{
					"logs_ref": "/Users/example/.codencer-live-test/runtime/daemon/state/artifacts/task.log",
				},
			},
		})
	})
	mux.HandleFunc("/api/v2/projects/codencer/reports/run-plans/run-async-console", func(w http.ResponseWriter, r *http.Request) {
		requireRelayAuth(t, r)
		writeTestJSON(t, w, map[string]any{
			"run_id":      "run-async-console",
			"status":      "completed",
			"report_path": "/Users/example/.codencer-live-test/artifacts/run-plans/run-async-console.json",
			"run":         map[string]any{"id": "run-async-console", "state": "completed"},
			"tasks": []map[string]any{{
				"task_id": "async",
				"status":  "completed",
				"summary": "async console task completed",
			}},
		})
	})
	mux.HandleFunc("/api/v2/projects/codencer/reports/run-plans/run-async-manifest", func(w http.ResponseWriter, r *http.Request) {
		requireRelayAuth(t, r)
		writeTestJSON(t, w, map[string]any{
			"run_id":      "run-async-manifest",
			"status":      "completed",
			"report_path": "/Users/example/.codencer-live-test/artifacts/run-plans/run-async-manifest.json",
			"run":         map[string]any{"id": "run-async-manifest", "state": "completed"},
			"tasks": []map[string]any{{
				"task_id": "async-manifest",
				"status":  "completed",
				"summary": "async manifest completed",
			}},
		})
	})
	mux.HandleFunc("/api/v2/projects/codencer/runs/run-async-console/cancel", func(w http.ResponseWriter, r *http.Request) {
		requireRelayAuth(t, r)
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeTestJSON(t, w, map[string]any{
			"ok":     true,
			"run_id": "run-async-console",
			"status": "cancelled",
			"run": map[string]any{
				"id":    "run-async-console",
				"state": "cancelled",
			},
			"report_path": "/Users/example/.codencer-live-test/artifacts/run-plans/run-async-console.json",
		})
	})
	mux.HandleFunc("/api/v2/projects/codencer/reports/run-plans/run-gateway-test", func(w http.ResponseWriter, r *http.Request) {
		requireRelayAuth(t, r)
		writeTestJSON(t, w, map[string]any{
			"run_id":      "run-gateway-test",
			"status":      "completed",
			"report_path": "/Users/example/.codencer-live-test/artifacts/run-plans/run-gateway-test.json",
			"run":         map[string]any{"id": "run-gateway-test", "state": "running"},
			"tasks": []map[string]any{{
				"task_id":          "fake",
				"adapter":          "claude",
				"profile":          "claude-default",
				"adapter_profile":  "claude-default",
				"executor_profile": "claude-default",
				"evidence": map[string]any{
					"logs_ref": "/tmp/codencer/stdout.log",
					"artifacts": []map[string]any{{
						"id":        "artifact-report",
						"name":      "stdout.log",
						"type":      "stdout",
						"mime_type": "text/plain",
						"size":      24,
						"hash":      "hash-report",
						"path":      "/var/folders/test/codencer/stdout.log",
					}},
					"result": map[string]any{"artifacts": map[string]any{
						"normalized_task_ref": "/Users/example/.codencer-live-test/normalized-task.json",
						"original_input_ref":  "/Users/example/.codencer-live-test/original-input.txt",
					}},
				},
			}},
		})
	})
	mux.HandleFunc("/api/v2/projects/codencer/reports/run-plans/run-blocked", func(w http.ResponseWriter, r *http.Request) {
		requireRelayAuth(t, r)
		writeTestJSON(t, w, map[string]any{
			"run_id":      "run-blocked",
			"status":      "blocked",
			"report_path": "/Users/example/.codencer-live-test/artifacts/run-plans/run-blocked.json",
			"blocker": map[string]any{
				"type":                      "needs_planner_decision",
				"planner_decision_required": true,
				"evidence_refs":             []string{"/tmp/codencer/blocker.json"},
			},
		})
	})
	relay.Server = httptest.NewServer(mux)
	return relay
}

func newProjectListRelay(t *testing.T, projects []map[string]any) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/status", func(w http.ResponseWriter, r *http.Request) {
		requireRelayAuth(t, r)
		writeTestJSON(t, w, map[string]any{"ok": true})
	})
	mux.HandleFunc("/api/v2/projects", func(w http.ResponseWriter, r *http.Request) {
		requireRelayAuth(t, r)
		writeTestJSON(t, w, projects)
	})
	return httptest.NewServer(mux)
}

func newGatewayStoreAPIServer(t *testing.T, relayURL string) (*httptest.Server, TokenResponse) {
	t.Helper()
	t.Setenv("CODENCER_TEST_GATEWAY_TOKEN", "gateway-secret")
	t.Setenv("CODENCER_DEFAULT_RELAY_TOKEN", "relay-secret")

	cfg := DefaultConfig()
	cfg.PublicBaseURL = "http://127.0.0.1:19090"
	cfg.MCPURL = "http://127.0.0.1:19090/mcp"
	cfg.Auth.TokenEnv = "CODENCER_TEST_GATEWAY_TOKEN"
	cfg.Store.Path = filepath.Join(t.TempDir(), "gateway.db")
	cfg.DefaultRelay.URL = relayURL
	cfg.DefaultRelay.TokenEnv = "CODENCER_DEFAULT_RELAY_TOKEN"
	cfg.OAuthDev.Issuer = "http://127.0.0.1:19090"
	cfg.OAuthDev.OperatorCodeHash = sha256Hex("operator-code")
	server, err := NewServer(cfg, ServerOptions{})
	if err != nil {
		t.Fatalf("new gateway server: %v", err)
	}
	httpServer := httptest.NewServer(server.Handler())
	return httpServer, TokenResponse{
		AccessToken: "gateway-secret",
		UserID:      server.devUserID,
		WorkspaceID: server.devWorkspaceID,
	}
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

func mcpStructuredContent(t *testing.T, response map[string]any) any {
	t.Helper()
	result, ok := response["result"].(map[string]any)
	if !ok {
		t.Fatalf("MCP response missing result object: %s", mustJSON(t, response))
	}
	payload, ok := result["structuredContent"]
	if !ok {
		t.Fatalf("MCP response missing structuredContent: %s", mustJSON(t, response))
	}
	return payload
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

func assertNoGatewayMCPLeak(t *testing.T, body string) {
	t.Helper()
	for _, forbidden := range []string{
		"/Users/",
		"/tmp/",
		"/var/folders/",
		".codencer-live-test",
		"relay-secret",
		"report_path",
		"logs_ref",
		"normalized_task_ref",
		"original_input_ref",
		`"path"`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("Gateway MCP output leaked %q: %s", forbidden, body)
		}
	}
}

func assertNoGatewayConsoleSensitiveLeak(t *testing.T, body string) {
	t.Helper()
	for _, forbidden := range []string{
		"/Users/",
		"/tmp/",
		"/var/folders/",
		".codencer-live-test",
		"relay-secret",
		"planner_token",
		"daemon_url",
		"public_key",
		`"repo_root"`,
		`"path"`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("Gateway Console output leaked %q: %s", forbidden, body)
		}
	}
}

func requireJSONArray(t *testing.T, payload map[string]any, key string) []any {
	t.Helper()
	value, ok := payload[key]
	if !ok {
		t.Fatalf("missing %q in payload: %s", key, mustJSON(t, payload))
	}
	values, ok := value.([]any)
	if !ok {
		t.Fatalf("%q is %T, want JSON array in payload: %s", key, value, mustJSON(t, payload))
	}
	return values
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
