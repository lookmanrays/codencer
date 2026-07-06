package live

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/local"
	"agent-bridge/internal/localexec"
	manifestpkg "agent-bridge/internal/manifest"
	"agent-bridge/internal/mcpconfig"
	"agent-bridge/internal/supervisor"
)

func Matrix(ctx context.Context, opts Options) (Report, error) {
	profile := firstNonEmpty(opts.Profile, "local")
	report, paths, err := NewReport(profile, opts)
	if err != nil {
		return Report{}, err
	}
	if _, err := local.EnsureHome(paths, now(opts)); err != nil {
		report.Add(failed("codencer_home", "local", BlockerUnknown, err.Error(), false))
	} else {
		report.Add(passed("codencer_home", "local", false, paths.Home))
	}
	report.Add(binaryCapabilityCheck("codex_cli", "executor", "CODEX_BINARY", "codex"))
	report.Add(binaryCapabilityCheck("claude_cli", "executor", "CLAUDE_BINARY", "claude"))
	report.Add(toolBinaryCheck(opts, "orchestratord", "runtime"))
	report.Add(toolBinaryCheck(opts, "codencer-relayd", "relay"))
	report.Add(toolBinaryCheck(opts, "codencer-connectord", "relay"))

	switch profile {
	case "local":
		addLocalLiveChecks(ctx, &report, opts)
	case "relay":
		addRelayLiveChecks(ctx, &report, opts)
	case "wsl":
		report.Add(RunWSLCheck(ctx, opts))
	case "all":
		addLocalLiveChecks(ctx, &report, opts)
		addRelayLiveChecks(ctx, &report, opts)
		report.Add(RunWSLCheck(ctx, opts))
	default:
		report.Add(failed("live_profile", "input", BlockerUnknown, "profile must be local, relay, wsl, or all", false))
	}

	report.Finish(opts)
	if err := PersistReport(paths, "live-matrix", &report); err != nil {
		return report, err
	}
	return report, nil
}

func addLocalLiveChecks(ctx context.Context, report *Report, opts Options) {
	if opts.IsEnabled("codex") {
		child, err := RunCodex(ctx, opts)
		report.Add(reportToCheck("codex_live_execution", "executor", child, err))
	} else {
		report.Add(skipped("codex_live_execution", "executor", "set CODENCER_LIVE_CODEX=1 or --enable-codex to run live Codex", true))
	}
	if opts.IsEnabled("claude") {
		child, err := RunClaude(ctx, opts)
		report.Add(reportToCheck("claude_live_execution", "executor", child, err))
	} else {
		report.Add(skipped("claude_live_execution", "executor", "set CODENCER_LIVE_CLAUDE=1 or --enable-claude to run live Claude", true))
	}
}

func addRelayLiveChecks(ctx context.Context, report *Report, opts Options) {
	if opts.IsEnabled("relay-mcp") {
		child, err := RunRelayMCP(ctx, opts)
		report.Add(reportToCheck("relay_mcp_live", "relay", child, err))
	} else {
		report.Add(skipped("relay_mcp_live", "relay", "set CODENCER_LIVE_RELAY_MCP=1 or --enable-relay-mcp to run live Relay/MCP", true))
	}
	if opts.IsEnabled("restart-reconnect") {
		child, err := RunRestartReconnect(ctx, opts)
		report.Add(reportToCheck("restart_reconnect_live", "runtime", child, err))
	} else {
		report.Add(skipped("restart_reconnect_live", "runtime", "set CODENCER_LIVE_SERVICE_RESTART=1 or --enable-service-restart to run restart/reconnect smoke", true))
	}
}

func RunCodex(ctx context.Context, opts Options) (Report, error) {
	return runAgent(ctx, opts, "codex", "CODEX_BINARY", "codex", "codex-workspace")
}

func RunClaude(ctx context.Context, opts Options) (Report, error) {
	return runAgent(ctx, opts, "claude", "CLAUDE_BINARY", "claude", "claude-default")
}

func runAgent(ctx context.Context, opts Options, id, envName, binaryName, profile string) (Report, error) {
	report, paths, err := NewReport(id, opts)
	if err != nil {
		return Report{}, err
	}
	if !opts.IsEnabled(id) {
		report.Add(skipped(id+"_live_execution", "executor", "live "+id+" smoke is not enabled", true))
		report.Finish(opts)
		_ = PersistReport(paths, "live-matrix", &report)
		return report, nil
	}
	path, version, err := probeBinary(envName, binaryName)
	if err != nil {
		report.Add(skipped(id+"_cli", "executor", fmt.Sprintf("%s not installed or not on PATH: %v", binaryName, err), false))
		report.Finish(opts)
		_ = PersistReport(paths, "live-matrix", &report)
		return report, nil
	}
	report.Add(passed(id+"_cli", "executor", false, path, firstLine(version)))

	h, err := newWorkspaceHarness(opts)
	if err != nil {
		report.Add(failed(id+"_workspace", "executor", BlockerUnknown, err.Error(), true))
		report.Finish(opts)
		_ = PersistReport(paths, "live-matrix", &report)
		return report, nil
	}
	defer h.Cleanup()
	report.Workspace = h.Root

	daemon, daemonURL, err := startDaemon(ctx, h, opts)
	if err != nil {
		report.Add(blocked(id+"_daemon", "runtime", BlockerDaemonNotRunning, err.Error(), true))
		report.Finish(opts)
		_ = PersistReport(paths, "live-matrix", &report)
		return report, nil
	}
	defer daemon.Stop()

	if err := h.registerProject("live", id, profile, daemonURL, false); err != nil {
		report.Add(failed(id+"_project", "local", BlockerUnknown, err.Error(), true))
		report.Finish(opts)
		_ = PersistReport(paths, "live-matrix", &report)
		return report, nil
	}
	service := localexec.NewService()
	service.WaitTimeout = 15 * time.Minute
	manifest := agentManifest(profile)
	runReport, err := service.RunPlan(ctx, localexec.RunPlanOptions{
		BaseOptions: localexec.BaseOptions{
			ProjectID:    "live",
			RepoRoot:     h.RepoRoot,
			CodencerHome: h.Home,
		},
		Manifest:     manifest,
		ManifestName: id + "-live.yaml",
		Wait:         true,
	})
	check := checkFromRunPlan(id+"_task", "executor", runReport, err)
	check.Live = true
	report.Add(check)
	report.Finish(opts)
	_ = PersistReport(paths, "live-matrix", &report)
	return report, nil
}

func RunRelayMCP(ctx context.Context, opts Options) (Report, error) {
	report, paths, err := NewReport("relay-mcp", opts)
	if err != nil {
		return Report{}, err
	}
	if !opts.IsEnabled("relay-mcp") {
		report.Add(skipped("relay_mcp_live", "relay", "live Relay/MCP smoke is not enabled", true))
		report.Finish(opts)
		_ = PersistReport(paths, "live-matrix", &report)
		return report, nil
	}
	child, err := runRelayMCPHarness(ctx, opts, false)
	for _, check := range child.Checks {
		report.Add(check)
	}
	if child.Workspace != "" {
		report.Workspace = child.Workspace
	}
	if err != nil {
		report.Add(failed("relay_mcp_harness", "relay", BlockerRelayUnreachable, err.Error(), true))
	}
	report.Finish(opts)
	_ = PersistReport(paths, "live-matrix", &report)
	return report, nil
}

func RunRestartReconnect(ctx context.Context, opts Options) (Report, error) {
	report, paths, err := NewReport("restart-reconnect", opts)
	if err != nil {
		return Report{}, err
	}
	if !opts.IsEnabled("restart-reconnect") {
		report.Add(skipped("restart_reconnect_live", "runtime", "restart/reconnect smoke is not enabled", true))
		report.Finish(opts)
		_ = PersistReport(paths, "live-matrix", &report)
		return report, nil
	}
	child, err := runRelayMCPHarness(ctx, opts, true)
	for _, check := range child.Checks {
		report.Add(check)
	}
	if child.Workspace != "" {
		report.Workspace = child.Workspace
	}
	if err != nil {
		report.Add(failed("restart_reconnect_harness", "runtime", BlockerServiceRestartFailed, err.Error(), true))
	}
	report.Finish(opts)
	_ = PersistReport(paths, "live-matrix", &report)
	return report, nil
}

func RunCodexMCP(ctx context.Context, opts Options) (Report, error) {
	return runMCPClientProof(ctx, opts, "codex-mcp", "codex", EnvLiveCodexMCP, "CODENCER_CODEX_CONFIG_WRITE")
}

func RunClaudeMCP(ctx context.Context, opts Options) (Report, error) {
	return runMCPClientProof(ctx, opts, "claude-mcp", "claude-code", EnvLiveClaudeMCP, "CODENCER_CLAUDE_CONFIG_WRITE")
}

func runMCPClientProof(ctx context.Context, opts Options, profile, client, envName, writeEnv string) (Report, error) {
	report, paths, err := NewReport(profile, opts)
	if err != nil {
		return Report{}, err
	}
	endpoint := firstNonEmpty(opts.Endpoint, "https://relay.example.com/mcp")
	payload, err := mcpconfig.Generate(mcpconfig.Options{Client: client, Endpoint: endpoint, TokenEnv: "CODENCER_PLANNER_TOKEN", Name: "codencer"})
	if err != nil {
		report.Add(failed(profile+"_config_generated", "mcp", BlockerMCPConfigInvalid, err.Error(), false))
	} else {
		report.Add(passed(profile+"_config_generated", "mcp", false, fmt.Sprintf("endpoint=%s", payload["endpoint"])))
	}
	if !envEnabled(envName) && !opts.IsEnabled(profile) {
		report.Add(skipped(profile+"_endpoint_verified", "mcp", "live MCP client proof is not enabled", true))
		report.Add(skipped(profile+"_client_verified", "mcp", "manual_client_proof_required; no client config was written", false))
		report.Finish(opts)
		_ = PersistReport(paths, "live-matrix", &report)
		return report, nil
	}
	if token := strings.TrimSpace(os.Getenv("CODENCER_PLANNER_TOKEN")); token != "" {
		err := waitHTTP(ctx, strings.TrimSuffix(endpoint, "/mcp")+"/.well-known/oauth-protected-resource/mcp", token, 5*time.Second)
		if err != nil {
			report.Add(blocked(profile+"_endpoint_verified", "mcp", BlockerRelayUnreachable, err.Error(), true))
		} else {
			report.Add(passed(profile+"_endpoint_verified", "mcp", true, endpoint))
		}
	} else {
		report.Add(blocked(profile+"_endpoint_verified", "mcp", BlockerAuthRequired, "CODENCER_PLANNER_TOKEN is not set", true))
	}
	if envEnabled(writeEnv) {
		report.Add(skipped(profile+"_client_verified", "mcp", "client config write requested but product client automation is not implemented in Sprint 5", true))
	} else {
		report.Add(blocked(profile+"_client_verified", "mcp", BlockerManualProofRequired, "manual client proof required; set "+writeEnv+"=1 only for a reversible operator-run proof", false))
	}
	report.Finish(opts)
	_ = PersistReport(paths, "live-matrix", &report)
	return report, nil
}

func RunWSL(ctx context.Context, opts Options) (Report, error) {
	report, paths, err := NewReport("wsl", opts)
	if err != nil {
		return Report{}, err
	}
	report.Add(RunWSLCheck(ctx, opts))
	report.Finish(opts)
	_ = PersistReport(paths, "live-matrix", &report)
	return report, nil
}

func RunWSLCheck(ctx context.Context, opts Options) Check {
	_ = ctx
	platform := supervisor.DetectPlatform("auto", nil)
	if !platform.WSL {
		return Check{ID: "wsl_environment", Category: "wsl", Status: StatusSkipped, Reason: "not_wsl", ObservedFacts: []string{platform.OS + "/" + platform.Arch}}
	}
	if platform.ServiceManager != supervisor.ManagerSystemd || !platform.SystemdUser {
		return Check{ID: "wsl_systemd", Category: "wsl", Status: StatusUnsupported, Reason: "WSL systemd user manager is unavailable", Blocker: &Blocker{Type: BlockerWSLSystemdDisabled, Message: platform.UnsupportedNote}}
	}
	if !opts.IsEnabled("wsl") {
		return Check{ID: "wsl_live", Category: "wsl", Status: StatusSkipped, Reason: "set CODENCER_LIVE_WSL=1 to run WSL live smoke", ObservedFacts: []string{"systemd_user_available=true"}}
	}
	return Check{ID: "wsl_live", Category: "wsl", Status: StatusPassed, Live: true, ObservedFacts: []string{"systemd_user_available=true"}}
}

func binaryCapabilityCheck(id, category, envName, fallback string) Check {
	return timedCheck(id, category, false, func() Check {
		path, version, err := probeBinary(envName, fallback)
		if err != nil {
			return Check{Status: StatusSkipped, Reason: fmt.Sprintf("%s not installed: %v", fallback, err), Blocker: &Blocker{Type: BlockerExecutorMissing, Message: err.Error()}}
		}
		return Check{Status: StatusPassed, ObservedFacts: []string{path, firstLine(version)}}
	})
}

func toolBinaryCheck(opts Options, name, category string) Check {
	return timedCheck(name+"_binary", category, false, func() Check {
		path, err := resolveBinary(opts, name)
		if err != nil {
			return Check{Status: StatusNotConfigured, Reason: err.Error(), Blocker: &Blocker{Type: BlockerExecutorMissing, Message: err.Error()}}
		}
		return Check{Status: StatusPassed, ObservedFacts: []string{path}}
	})
}

func reportToCheck(id, category string, report Report, err error) Check {
	if err != nil {
		return failed(id, category, BlockerUnknown, err.Error(), true)
	}
	if report.OK {
		return passed(id, category, true, "report="+report.ReportPath)
	}
	for _, check := range report.Checks {
		if check.Status == StatusBlocked || check.Status == StatusFailed {
			check.ID = id
			check.Category = category
			check.Live = true
			return check
		}
	}
	return failed(id, category, BlockerUnknown, "live report failed without a specific blocker", true)
}

func checkFromRunPlan(id, category string, report localexec.RunPlanReport, err error) Check {
	if err != nil {
		if localErr := localexec.ErrorReportFor(err); localErr.Blocker != nil {
			return localexecBlockerCheck(id, category, localErr.Blocker.Type, localErr.Blocker.Message, localErr.ExitCode)
		}
		return failed(id, category, BlockerUnknown, err.Error(), true)
	}
	if report.OK {
		facts := []string{"status=" + report.Status, "report=" + report.ReportPath}
		if report.Run != nil {
			facts = append(facts, "run_id="+report.Run.ID)
		}
		return passed(id, category, true, facts...)
	}
	if report.Blocker != nil {
		return localexecBlockerCheck(id, category, report.Blocker.Type, report.Blocker.Message, report.ExitCode)
	}
	for _, task := range report.Tasks {
		if task.Blocker != nil {
			return localexecBlockerCheck(id, category, task.Blocker.Type, task.Blocker.Message, task.ExitCode)
		}
		if !task.OK && task.Summary != "" {
			return classifiedCheck(id, category, task.Summary, task.ExitCode)
		}
	}
	return classifiedCheck(id, category, report.Status, report.ExitCode)
}

func localexecBlockerCheck(id, category, blockerType, message string, exitCode int) Check {
	if mapped := classifyMessage(message); mapped != "" {
		blockerType = mapped
	}
	switch blockerType {
	case localexec.BlockerValidationFailed:
		return failed(id, category, BlockerValidationFailed, message, true)
	case localexec.BlockerDaemonNotRunning, localexec.BlockerBridgeError:
		return blocked(id, category, BlockerDaemonNotRunning, message, true)
	case localexec.BlockerTimeout:
		return blocked(id, category, BlockerTimeout, message, true)
	case localexec.BlockerAdapterError:
		return failed(id, category, "adapter_error", message, true)
	default:
		if exitCode == localexec.ExitBlocked {
			return blocked(id, category, firstNonEmpty(blockerType, BlockerUnknown), message, true)
		}
		return failed(id, category, firstNonEmpty(blockerType, BlockerUnknown), message, true)
	}
}

func classifiedCheck(id, category, message string, exitCode int) Check {
	if mapped := classifyMessage(message); mapped != "" {
		return blocked(id, category, mapped, message, true)
	}
	switch exitCode {
	case localexec.ExitValidationFailed:
		return failed(id, category, BlockerValidationFailed, message, true)
	case localexec.ExitTimeout:
		return blocked(id, category, BlockerTimeout, message, true)
	case localexec.ExitDaemonFailed:
		return blocked(id, category, BlockerDaemonNotRunning, message, true)
	case localexec.ExitBlocked:
		return blocked(id, category, BlockerUnknown, message, true)
	default:
		return failed(id, category, BlockerUnknown, message, true)
	}
}

func classifyMessage(message string) string {
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "auth") || strings.Contains(lower, "login") || strings.Contains(lower, "not logged in") || strings.Contains(lower, "api key"):
		return BlockerAuthRequired
	case strings.Contains(lower, "rate limit") || strings.Contains(lower, "429") || strings.Contains(lower, "too many requests"):
		return BlockerRateLimit
	case strings.Contains(lower, "validation"):
		return BlockerValidationFailed
	case strings.Contains(lower, "timed out") || strings.Contains(lower, "timeout"):
		return BlockerTimeout
	default:
		return ""
	}
}

func agentManifest(profile string) *manifestpkg.Manifest {
	return &manifestpkg.Manifest{
		Version: manifestpkg.APIVersion,
		Kind:    manifestpkg.Kind,
		Execution: manifestpkg.Execution{
			Profile: profile,
			Timeout: "10m",
		},
		Tasks: []manifestpkg.Task{{
			ID:    "live-smoke",
			Title: "Live executor smoke",
			Goal:  "Create or update codencer-live-result.txt in the workspace with exactly the text CODENCER_LIVE_SMOKE_OK. Do not modify files outside the workspace.",
			Validations: []domain.ValidationCommand{{
				Name:           "live-result",
				Command:        "test -f codencer-live-result.txt && grep -q CODENCER_LIVE_SMOKE_OK codencer-live-result.txt",
				TimeoutSeconds: 30,
			}},
		}},
	}
}

func firstLine(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if idx := strings.IndexByte(value, '\n'); idx >= 0 {
		return strings.TrimSpace(value[:idx])
	}
	return value
}

func mcpCall(ctx context.Context, endpoint, token, name string, arguments map[string]any) (map[string]any, error) {
	body, _ := json.Marshal(map[string]any{
		"jsonrpc":   "2.0",
		"id":        time.Now().UnixNano(),
		"name":      name,
		"arguments": arguments,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(endpoint, "/")+"/call", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("MCP call %s returned %s: %s", name, resp.Status, Redact(string(data)))
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func mcpInitializeAndList(ctx context.Context, endpoint, token string) error {
	initBody := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(initBody))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("MCP-Protocol-Version", "2025-11-25")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	sessionID := resp.Header.Get("MCP-Session-Id")
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("initialize returned %s", resp.Status)
	}
	body := []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	req, err = http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("MCP-Protocol-Version", "2025-11-25")
	if sessionID != "" {
		req.Header.Set("MCP-Session-Id", sessionID)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("tools/list returned %s: %s", resp.Status, Redact(string(data)))
	}
	if !bytes.Contains(data, []byte("codencer.list_projects")) {
		return fmt.Errorf("tools/list did not include codencer.list_projects")
	}
	return nil
}

func writeTempFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0600)
}

func commandOutput(ctx context.Context, env []string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	return cmd.CombinedOutput()
}
