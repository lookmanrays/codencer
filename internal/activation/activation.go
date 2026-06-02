package activation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"agent-bridge/internal/local"
	"agent-bridge/internal/localexec"
	"agent-bridge/internal/mcpconfig"
	"agent-bridge/internal/security"
)

const (
	StatusPassed        = "passed"
	StatusFailed        = "failed"
	StatusBlocked       = "blocked"
	StatusSkipped       = "skipped"
	StatusNotConfigured = "not_configured"
)

type Options struct {
	Relay                 string
	MCPURL                string
	TokenEnv              string
	Token                 string
	ProjectID             string
	RunFakeManifest       bool
	CheckOAuth            bool
	CheckChatGPTReadiness bool
	AuthMode              string
	CodencerHome          string
	Now                   func() time.Time
	HTTPClient            *http.Client
}

type Check struct {
	ID            string   `json:"id"`
	Status        string   `json:"status"`
	Detail        string   `json:"detail,omitempty"`
	ObservedFacts []string `json:"observed_facts,omitempty"`
}

type Summary struct {
	Passed        int `json:"passed"`
	Failed        int `json:"failed"`
	Blocked       int `json:"blocked"`
	Skipped       int `json:"skipped"`
	NotConfigured int `json:"not_configured"`
}

type Report struct {
	OK          bool              `json:"ok"`
	Mode        string            `json:"mode"`
	StartedAt   time.Time         `json:"started_at"`
	CompletedAt time.Time         `json:"completed_at"`
	Relay       string            `json:"relay,omitempty"`
	MCPURL      string            `json:"mcp_url,omitempty"`
	ProjectID   string            `json:"project_id,omitempty"`
	AuthMode    string            `json:"auth_mode,omitempty"`
	Checks      []Check           `json:"checks"`
	Summary     Summary           `json:"summary"`
	Output      any               `json:"output,omitempty"`
	PackagePath string            `json:"package_path,omitempty"`
	ReportPath  string            `json:"report_path,omitempty"`
	ExitCode    int               `json:"exit_code"`
	Environment local.Environment `json:"environment"`
}

type packageArtifact struct {
	OK        bool      `json:"ok"`
	CreatedAt time.Time `json:"created_at"`
	Relay     string    `json:"relay,omitempty"`
	MCPURL    string    `json:"mcp_url,omitempty"`
	ProjectID string    `json:"project_id,omitempty"`
	AuthMode  string    `json:"auth_mode,omitempty"`
	Files     []string  `json:"files"`
	States    []string  `json:"states"`
}

var absolutePathPattern = regexp.MustCompile(`(?m)(/Users/|/home/|[A-Za-z]:\\)`)

func CheckActivation(ctx context.Context, opts Options) (Report, error) {
	report := baseReport("check", opts)
	paths, err := local.ResolvePathsForHome("", "", opts.CodencerHome)
	if err != nil {
		return Report{}, err
	}
	if _, err := local.EnsureHome(paths, now(opts)); err != nil {
		report.add("codencer_home", StatusFailed, err.Error())
		return finish(report, opts), nil
	}
	report.add("codencer_home", StatusPassed, "local production home is initialized", "CODENCER_HOME="+paths.Home)
	report.add("activation_package_ready", StatusPassed, "activation artifacts can be written under CODENCER_HOME/artifacts/activation")

	if strings.TrimSpace(opts.Relay) == "" {
		report.add("relay_endpoint", StatusSkipped, "pass --relay to run remote relay/MCP checks")
		return finish(report, opts), nil
	}

	relayURL, mcpURL, err := normalizeRelayAndMCP(opts)
	if err != nil {
		report.add("relay_url", StatusFailed, err.Error())
		return finish(report, opts), nil
	}
	report.Relay = relayURL
	report.MCPURL = mcpURL
	report.AuthMode = authMode(opts)
	report.ProjectID = strings.TrimSpace(opts.ProjectID)

	if strings.HasPrefix(relayURL, "https://") {
		report.add("relay_https_syntax", StatusPassed, "relay endpoint uses HTTPS syntax", relayURL)
	} else {
		report.add("relay_https_syntax", StatusFailed, "relay endpoint must use HTTPS for planner/client activation")
	}

	client := httpClient(opts)
	token := strings.TrimSpace(opts.Token)
	if token == "" && strings.TrimSpace(opts.TokenEnv) != "" {
		token = strings.TrimSpace(os.Getenv(strings.TrimSpace(opts.TokenEnv)))
	}
	if token == "" {
		report.add("relay_status", StatusSkipped, "bearer token not provided; skipping authenticated relay status")
	} else {
		status, body, err := httpJSON(ctx, client, http.MethodGet, relayURL+"/api/v2/status", token, nil)
		if err != nil {
			report.add("relay_status", StatusBlocked, err.Error())
		} else if status != http.StatusOK {
			report.add("relay_status", StatusBlocked, fmt.Sprintf("expected 200, got %d", status), redactBody(body))
		} else {
			report.add("relay_status", StatusPassed, "relay status endpoint is reachable with provided token", redactBody(body))
		}
	}

	checkProtectedResource(ctx, client, &report, relayURL)
	checkMCPUnauth(ctx, client, &report, mcpURL)
	if token != "" {
		checkMCPInitializeAndTools(ctx, client, &report, mcpURL, token)
		if report.ProjectID != "" {
			checkProjectVisible(ctx, client, &report, relayURL, token, report.ProjectID)
		}
		if opts.RunFakeManifest && report.ProjectID != "" {
			checkFakeManifest(ctx, client, &report, mcpURL, token, report.ProjectID)
		} else if opts.RunFakeManifest {
			report.add("fake_manifest_preflight", StatusSkipped, "--project is required for fake manifest preflight")
		}
	} else {
		report.add("mcp_initialize", StatusSkipped, "bearer token not provided")
		report.add("mcp_tools_list", StatusSkipped, "bearer token not provided")
		if opts.ProjectID != "" {
			report.add("project_visibility", StatusSkipped, "bearer token not provided")
		}
	}
	if opts.CheckOAuth || opts.CheckChatGPTReadiness || strings.EqualFold(authMode(opts), "oauth") {
		checkOAuthMetadata(ctx, client, &report, relayURL)
	}
	if opts.CheckChatGPTReadiness {
		checkChatGPTReadiness(&report, relayURL, mcpURL, authMode(opts))
	}
	return finish(report, opts), nil
}

func Package(ctx context.Context, opts Options) (Report, error) {
	report := baseReport("package", opts)
	paths, err := local.ResolvePathsForHome("", "", opts.CodencerHome)
	if err != nil {
		return Report{}, err
	}
	if _, err := local.EnsureHome(paths, now(opts)); err != nil {
		report.add("codencer_home", StatusFailed, err.Error())
		return finish(report, opts), nil
	}
	relayURL, mcpURL, err := normalizeRelayAndMCP(opts)
	if err != nil {
		report.add("relay_url", StatusFailed, err.Error())
		return finish(report, opts), nil
	}
	report.Relay = relayURL
	report.MCPURL = mcpURL
	report.ProjectID = strings.TrimSpace(opts.ProjectID)
	report.AuthMode = authMode(opts)
	root := filepath.Join(paths.ArtifactsDir, "activation", timestamp(now(opts)))
	if err := os.MkdirAll(root, 0755); err != nil {
		return Report{}, err
	}
	files := map[string]string{
		"README.md":              readmeContent(opts, relayURL, mcpURL),
		"curl-smoke.sh":          curlSmokeContent(opts, relayURL, mcpURL),
		"codex-config.toml":      codexConfigContent(opts, mcpURL),
		"claude-code-command.sh": claudeCommandContent(opts, mcpURL),
		"chatgpt-app-setup.md":   chatGPTContent(opts, relayURL, mcpURL),
	}
	written := make([]string, 0, len(files)+1)
	for name, content := range files {
		perm := os.FileMode(0600)
		if strings.HasSuffix(name, ".sh") {
			perm = 0700
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), perm); err != nil {
			report.add("package_write_"+name, StatusFailed, err.Error())
			return finish(report, opts), nil
		}
		written = append(written, name)
	}
	artifact := packageArtifact{
		OK:        true,
		CreatedAt: now(opts),
		Relay:     relayURL,
		MCPURL:    mcpURL,
		ProjectID: strings.TrimSpace(opts.ProjectID),
		AuthMode:  authMode(opts),
		Files:     append([]string(nil), written...),
		States: []string{
			"server_ready",
			"client_config_generated",
			"client_connected",
			"client_used_tool",
			"full_e2e_execution",
		},
	}
	data, _ := json.MarshalIndent(security.RedactJSON(artifact), "", "  ")
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(root, "activation-package.json"), data, 0600); err != nil {
		report.add("package_write_activation_package", StatusFailed, err.Error())
		return finish(report, opts), nil
	}
	written = append(written, "activation-package.json")
	report.add("activation_package", StatusPassed, "activation package generated", written...)
	report.PackagePath = root
	report.Output = security.RedactJSON(map[string]any{
		"package_path": root,
		"files":        written,
		"relay":        relayURL,
		"mcp_url":      mcpURL,
		"auth_mode":    authMode(opts),
	})
	_ = ctx
	return finish(report, opts), nil
}

func ChatGPT(opts Options) (Report, error) {
	report := baseReport("chatgpt", opts)
	relayURL, mcpURL, err := normalizeRelayAndMCP(opts)
	if err != nil {
		report.add("relay_url", StatusFailed, err.Error())
		return finish(report, opts), nil
	}
	report.Relay = relayURL
	report.MCPURL = mcpURL
	report.ProjectID = strings.TrimSpace(opts.ProjectID)
	report.AuthMode = firstNonEmpty(opts.AuthMode, "oauth")
	report.add("chatgpt_server_artifact", StatusPassed, "ChatGPT setup instructions generated; product proof remains pending until exercised in ChatGPT")
	report.Output = security.RedactJSON(chatGPTPayload(opts, relayURL, mcpURL))
	return finish(report, opts), nil
}

func Codex(opts Options) (Report, error) {
	return clientReport("codex", opts)
}

func ClaudeCode(opts Options) (Report, error) {
	return clientReport("claude-code", opts)
}

func clientReport(clientName string, opts Options) (Report, error) {
	report := baseReport(clientName, opts)
	_, mcpURL, err := normalizeRelayAndMCP(opts)
	if err != nil {
		report.add("relay_url", StatusFailed, err.Error())
		return finish(report, opts), nil
	}
	payload, err := mcpconfig.Generate(mcpconfig.Options{
		Client:   clientName,
		Endpoint: mcpURL,
		TokenEnv: tokenEnv(opts),
		Name:     "codencer",
	})
	if err != nil {
		report.add(clientName+"_config", StatusFailed, err.Error())
		return finish(report, opts), nil
	}
	report.MCPURL = mcpURL
	report.ProjectID = strings.TrimSpace(opts.ProjectID)
	report.add(clientName+"_config", StatusPassed, "client setup artifact generated; no user config files were written")
	report.Output = security.RedactJSON(clientPayload(clientName, opts, payload))
	return finish(report, opts), nil
}

func checkProtectedResource(ctx context.Context, client *http.Client, report *Report, relayURL string) {
	status, body, err := httpJSON(ctx, client, http.MethodGet, relayURL+"/.well-known/oauth-protected-resource/mcp", "", nil)
	if err != nil {
		report.add("protected_resource_metadata", StatusBlocked, err.Error())
		return
	}
	if status != http.StatusOK {
		report.add("protected_resource_metadata", StatusBlocked, fmt.Sprintf("expected 200, got %d", status), redactBody(body))
		return
	}
	report.add("protected_resource_metadata", StatusPassed, "MCP protected-resource metadata is available", redactBody(body))
}

func checkMCPUnauth(ctx context.Context, client *http.Client, report *Report, mcpURL string) {
	status, body, err := mcpRequest(ctx, client, mcpURL, "", "", map[string]any{"jsonrpc": "2.0", "id": "unauth", "method": "initialize"})
	if err != nil {
		report.add("mcp_unauth_behavior", StatusBlocked, err.Error())
		return
	}
	switch status {
	case http.StatusUnauthorized:
		report.add("mcp_unauth_behavior", StatusPassed, "unauthenticated MCP request is challenged")
	case http.StatusOK:
		report.add("mcp_unauth_behavior", StatusPassed, "unauthenticated MCP request succeeded; dev-noauth mode appears enabled")
	default:
		report.add("mcp_unauth_behavior", StatusBlocked, fmt.Sprintf("unexpected status %d", status), redactBody(body))
	}
}

func checkMCPInitializeAndTools(ctx context.Context, client *http.Client, report *Report, mcpURL, token string) {
	initReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      "init",
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-11-25",
			"clientInfo": map[string]any{
				"name":    "codencer-activation",
				"version": "v0",
			},
		},
	}
	status, body, err := mcpRequest(ctx, client, mcpURL, token, "", initReq)
	if err != nil {
		report.add("mcp_initialize", StatusBlocked, err.Error())
		return
	}
	if status != http.StatusOK {
		report.add("mcp_initialize", StatusBlocked, fmt.Sprintf("expected 200, got %d", status), redactBody(body))
		return
	}
	sessionID := extractSessionID(body)
	report.add("mcp_initialize", StatusPassed, "MCP initialize returned a structured response", "session_id_present="+fmt.Sprint(sessionID != ""))
	status, toolsBody, err := mcpRequest(ctx, client, mcpURL, token, sessionID, map[string]any{"jsonrpc": "2.0", "id": "tools", "method": "tools/list", "params": map[string]any{}})
	if err != nil {
		report.add("mcp_tools_list", StatusBlocked, err.Error())
		return
	}
	if status != http.StatusOK {
		report.add("mcp_tools_list", StatusBlocked, fmt.Sprintf("expected 200, got %d", status), redactBody(toolsBody))
		return
	}
	bodyText := string(toolsBody)
	required := []string{"codencer.list_projects", "codencer.run_project_manifest", "codencer.submit_project_task_and_wait", "codencer.get_blocker"}
	for _, name := range required {
		if !strings.Contains(bodyText, name) {
			report.add("mcp_tools_list", StatusBlocked, "required project-aware MCP tool is missing", name)
			return
		}
	}
	report.add("mcp_tools_list", StatusPassed, "project-aware MCP tools are listed", required...)
}

func checkProjectVisible(ctx context.Context, client *http.Client, report *Report, relayURL, token, projectID string) {
	status, body, err := httpJSON(ctx, client, http.MethodGet, relayURL+"/api/v2/projects", token, nil)
	if err != nil {
		report.add("project_visibility", StatusBlocked, err.Error())
		return
	}
	if absolutePathPattern.Match(body) {
		report.add("project_path_redaction", StatusFailed, "project list leaked an absolute local path")
	} else {
		report.add("project_path_redaction", StatusPassed, "project list did not expose obvious absolute local paths")
	}
	if status != http.StatusOK {
		report.add("project_visibility", StatusBlocked, fmt.Sprintf("expected 200, got %d", status), redactBody(body))
		return
	}
	if !strings.Contains(string(body), `"`+projectID+`"`) {
		report.add("project_visibility", StatusNotConfigured, "project is not visible through relay", "project="+projectID)
		return
	}
	report.add("project_visibility", StatusPassed, "shared project is visible through relay", "project="+projectID)
}

func checkFakeManifest(ctx context.Context, client *http.Client, report *Report, mcpURL, token, projectID string) {
	manifest := map[string]any{
		"version": "v1",
		"kind":    "codencer.run_plan",
		"metadata": map[string]any{
			"name": "activation-fake-success",
		},
		"tasks": []map[string]any{{
			"id":      "fake-success",
			"goal":    "activation fake manifest preflight",
			"profile": "fake-success",
		}},
	}
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      "fake-manifest",
		"method":  "tools/call",
		"params": map[string]any{
			"name": "codencer.run_project_manifest",
			"arguments": map[string]any{
				"project_id": projectID,
				"manifest":   manifest,
				"wait":       true,
			},
		},
	}
	status, body, err := mcpRequest(ctx, client, mcpURL, token, "", payload)
	if err != nil {
		report.add("fake_manifest_preflight", StatusBlocked, err.Error())
		return
	}
	if status != http.StatusOK {
		report.add("fake_manifest_preflight", StatusBlocked, fmt.Sprintf("expected 200, got %d", status), redactBody(body))
		return
	}
	if strings.Contains(string(body), `"ok":true`) || strings.Contains(string(body), `"ok": true`) {
		report.add("fake_manifest_preflight", StatusPassed, "server preflight fake manifest completed; this is not production live proof")
		return
	}
	report.add("fake_manifest_preflight", StatusBlocked, "fake manifest did not return ok:true", redactBody(body))
}

func checkOAuthMetadata(ctx context.Context, client *http.Client, report *Report, relayURL string) {
	for _, path := range []string{"/.well-known/oauth-authorization-server", "/.well-known/openid-configuration"} {
		status, body, err := httpJSON(ctx, client, http.MethodGet, relayURL+path, "", nil)
		id := strings.Trim(strings.ReplaceAll(path, "/", "_"), "_")
		if err != nil {
			report.add(id, StatusBlocked, err.Error())
			continue
		}
		if status != http.StatusOK {
			report.add(id, StatusNotConfigured, fmt.Sprintf("expected 200, got %d", status), redactBody(body))
			continue
		}
		report.add(id, StatusPassed, "OAuth dev metadata endpoint is available", redactBody(body))
	}
}

func checkChatGPTReadiness(report *Report, relayURL, mcpURL, mode string) {
	if !strings.HasPrefix(relayURL, "https://") || !strings.HasPrefix(mcpURL, "https://") {
		report.add("chatgpt_https_readiness", StatusFailed, "ChatGPT custom MCP activation requires public HTTPS URLs")
		return
	}
	if mode == "" {
		mode = "oauth"
	}
	report.add("chatgpt_readiness", StatusPassed, "server-side ChatGPT setup values are ready; product UI proof remains pending", "auth_mode="+mode)
}

func httpJSON(ctx context.Context, client *http.Client, method, target, token string, payload any) (int, []byte, error) {
	var body io.Reader
	if payload != nil {
		data, _ := json.Marshal(payload)
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, data, nil
}

func mcpRequest(ctx context.Context, client *http.Client, mcpURL, token, sessionID string, payload any) (int, []byte, error) {
	data, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mcpURL, bytes.NewReader(data))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-11-25")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if sessionID != "" {
		req.Header.Set("MCP-Session-Id", sessionID)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, _ = io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if sid := resp.Header.Get("MCP-Session-Id"); sid != "" {
		var decoded map[string]any
		if json.Unmarshal(data, &decoded) == nil {
			decoded["_mcp_session_id"] = sid
			data, _ = json.Marshal(decoded)
		}
	}
	return resp.StatusCode, data, nil
}

func extractSessionID(body []byte) string {
	var payload map[string]any
	if json.Unmarshal(body, &payload) != nil {
		return ""
	}
	value, _ := payload["_mcp_session_id"].(string)
	return value
}

func normalizeRelayAndMCP(opts Options) (string, string, error) {
	relayURL := strings.TrimRight(strings.TrimSpace(opts.Relay), "/")
	mcpURL := strings.TrimRight(strings.TrimSpace(opts.MCPURL), "/")
	if relayURL == "" && mcpURL == "" {
		relayURL = "https://relay.example.com"
	}
	if relayURL == "" && mcpURL != "" {
		relayURL = strings.TrimSuffix(mcpURL, "/mcp")
	}
	if mcpURL == "" {
		mcpURL = relayURL + "/mcp"
	}
	if !strings.HasSuffix(mcpURL, "/mcp") {
		mcpURL += "/mcp"
	}
	parsed, err := url.Parse(relayURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", "", fmt.Errorf("--relay must be an absolute URL")
	}
	parsedMCP, err := url.Parse(mcpURL)
	if err != nil || parsedMCP.Scheme == "" || parsedMCP.Host == "" {
		return "", "", fmt.Errorf("--mcp-url must be an absolute URL")
	}
	return relayURL, mcpURL, nil
}

func baseReport(mode string, opts Options) Report {
	started := now(opts)
	_, mcpURL, _ := normalizeRelayAndMCP(opts)
	return Report{
		OK:          true,
		Mode:        mode,
		StartedAt:   started,
		Relay:       strings.TrimRight(strings.TrimSpace(opts.Relay), "/"),
		MCPURL:      mcpURL,
		ProjectID:   strings.TrimSpace(opts.ProjectID),
		AuthMode:    authMode(opts),
		Environment: local.DetectEnvironment(),
	}
}

func finish(report Report, opts Options) Report {
	report.CompletedAt = now(opts)
	report.Summary = Summary{}
	report.OK = true
	for _, check := range report.Checks {
		switch check.Status {
		case StatusPassed:
			report.Summary.Passed++
		case StatusFailed:
			report.Summary.Failed++
			report.OK = false
		case StatusBlocked:
			report.Summary.Blocked++
			report.OK = false
		case StatusSkipped:
			report.Summary.Skipped++
		case StatusNotConfigured:
			report.Summary.NotConfigured++
		default:
			report.Summary.Blocked++
			report.OK = false
		}
	}
	if report.OK {
		report.ExitCode = localexec.ExitSuccess
	} else if report.Summary.Failed > 0 {
		report.ExitCode = localexec.ExitInvalidInput
	} else {
		report.ExitCode = localexec.ExitDaemonFailed
	}
	report.Output = security.RedactJSON(report.Output)
	return report
}

func (r *Report) add(id, status, detail string, facts ...string) {
	redactedFacts := make([]string, 0, len(facts))
	for _, fact := range facts {
		redactedFacts = append(redactedFacts, security.Redact(fact))
	}
	r.Checks = append(r.Checks, Check{
		ID:            id,
		Status:        status,
		Detail:        security.Redact(detail),
		ObservedFacts: redactedFacts,
	})
}

func now(opts Options) time.Time {
	if opts.Now != nil {
		return opts.Now().UTC()
	}
	return time.Now().UTC()
}

func timestamp(t time.Time) string {
	return t.UTC().Format("20060102T150405Z")
}

func httpClient(opts Options) *http.Client {
	if opts.HTTPClient != nil {
		return opts.HTTPClient
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func authMode(opts Options) string {
	mode := strings.TrimSpace(opts.AuthMode)
	if mode != "" {
		return mode
	}
	if opts.CheckOAuth || opts.CheckChatGPTReadiness {
		return "oauth"
	}
	if opts.Token != "" || opts.TokenEnv != "" {
		return "bearer"
	}
	return ""
}

func tokenEnv(opts Options) string {
	return firstNonEmpty(opts.TokenEnv, "CODENCER_MCP_TOKEN")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func redactBody(body []byte) string {
	return security.Redact(string(body))
}
