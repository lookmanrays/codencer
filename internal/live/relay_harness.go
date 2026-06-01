package live

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"agent-bridge/internal/localexec"
	manifestpkg "agent-bridge/internal/manifest"
	"agent-bridge/internal/supervisor"
)

func runRelayMCPHarness(ctx context.Context, opts Options, includeRestart bool) (Report, error) {
	report, _, err := NewReport("relay-mcp-harness", opts)
	if err != nil {
		return Report{}, err
	}
	h, err := newWorkspaceHarness(opts)
	if err != nil {
		report.Add(failed("relay_workspace", "relay", BlockerUnknown, err.Error(), true))
		report.Finish(opts)
		return report, err
	}
	defer h.Cleanup()
	report.Workspace = h.Root

	daemon, daemonURL, err := startDaemon(ctx, h, opts)
	if err != nil {
		report.Add(blocked("relay_daemon", "runtime", BlockerDaemonNotRunning, err.Error(), true))
		report.Finish(opts)
		return report, err
	}
	defer daemon.Stop()
	report.Add(passed("relay_daemon", "runtime", true, daemonURL))

	if err := h.registerProject("codencer", "fake", "fake-success", daemonURL, true); err != nil {
		report.Add(failed("relay_project_registry", "relay", BlockerUnknown, err.Error(), true))
		report.Finish(opts)
		return report, err
	}

	relay, relayURL, token, relayConfig, err := startRelay(ctx, h, opts)
	if err != nil {
		report.Add(blocked("relay_start", "relay", BlockerRelayUnreachable, err.Error(), true))
		report.Finish(opts)
		return report, err
	}
	defer relay.Stop()
	report.Add(passed("relay_start", "relay", true, relayURL))

	secret, err := createEnrollmentSecret(ctx, opts, relayConfig)
	if err != nil {
		report.Add(blocked("relay_enrollment", "relay", BlockerRelayUnreachable, err.Error(), true))
		report.Finish(opts)
		return report, err
	}
	connectorConfig := filepath.Join(h.Root, "connector.json")
	connector, err := startConnector(ctx, h, opts, relayURL, daemonURL, secret, connectorConfig)
	if err != nil {
		report.Add(blocked("connector_start", "relay", BlockerConnectorOffline, err.Error(), true))
		report.Finish(opts)
		return report, err
	}
	defer connector.Stop()
	if err := waitProjectOnline(ctx, relayURL, token, true); err != nil {
		report.Add(blocked("project_advertisement", "relay", BlockerConnectorOffline, err.Error(), true))
		report.Finish(opts)
		return report, err
	}
	report.Add(passed("project_advertisement", "relay", true, "project=codencer"))

	endpoint := relayURL + "/mcp"
	if err := mcpInitializeAndList(ctx, endpoint, token); err != nil {
		report.Add(blocked("mcp_tools_list", "mcp", BlockerMCPConfigInvalid, err.Error(), true))
		report.Finish(opts)
		return report, err
	}
	report.Add(passed("mcp_tools_list", "mcp", true, endpoint))

	submit, err := mcpCall(ctx, endpoint, token, "codencer.submit_project_task_and_wait", map[string]any{
		"project_id": "codencer",
		"goal":       "relay fake success",
		"profile":    "fake-success",
	})
	if err != nil {
		report.Add(blocked("mcp_submit_project_task", "mcp", BlockerRelayUnreachable, err.Error(), true))
		report.Finish(opts)
		return report, err
	}
	stepID := nestedString(submit, "result", "structuredContent", "task", "step_id")
	if nestedBool(submit, "result", "structuredContent", "ok") != true || stepID == "" {
		report.Add(failed("mcp_submit_project_task", "mcp", BlockerUnknown, "MCP submit did not return ok task evidence", true))
		report.Finish(opts)
		return report, fmt.Errorf("MCP submit did not return ok task evidence")
	}
	report.Add(passed("mcp_submit_project_task", "mcp", true, "step_id="+stepID))

	if _, err := mcpCall(ctx, endpoint, token, "codencer.get_project_step_result", map[string]any{"project_id": "codencer", "step_id": stepID}); err != nil {
		report.Add(blocked("mcp_step_result", "mcp", BlockerRelayUnreachable, err.Error(), true))
	} else {
		report.Add(passed("mcp_step_result", "mcp", true, "step_id="+stepID))
	}

	success, err := mcpRunManifest(ctx, endpoint, token, filepath.Join(repoRootForOpts(opts), "testdata", "manifests", "fake-success.yaml"))
	if err != nil {
		report.Add(blocked("mcp_run_manifest_success", "mcp", BlockerRelayUnreachable, err.Error(), true))
		report.Finish(opts)
		return report, err
	}
	runID := nestedString(success, "result", "structuredContent", "run", "id")
	if nestedBool(success, "result", "structuredContent", "ok") != true || runID == "" {
		report.Add(failed("mcp_run_manifest_success", "mcp", BlockerUnknown, "fake-success manifest did not complete", true))
		report.Finish(opts)
		return report, fmt.Errorf("fake-success manifest did not complete")
	}
	report.Add(passed("mcp_run_manifest_success", "mcp", true, "run_id="+runID))

	if _, err := mcpCall(ctx, endpoint, token, "codencer.get_run_report", map[string]any{"project_id": "codencer", "run_id": runID}); err != nil {
		report.Add(blocked("mcp_get_run_report", "mcp", BlockerRelayUnreachable, err.Error(), true))
	} else {
		report.Add(passed("mcp_get_run_report", "mcp", true, "run_id="+runID))
	}

	blocker, err := mcpRunManifest(ctx, endpoint, token, filepath.Join(repoRootForOpts(opts), "testdata", "manifests", "fake-blocker.yaml"))
	if err != nil {
		report.Add(blocked("mcp_run_manifest_blocker", "mcp", BlockerRelayUnreachable, err.Error(), true))
	} else if nestedNumber(blocker, "result", "structuredContent", "exit_code") == 10 {
		report.Add(passed("mcp_run_manifest_blocker", "mcp", true, "exit_code=10"))
	} else {
		report.Add(failed("mcp_run_manifest_blocker", "mcp", BlockerUnknown, "fake-blocker manifest did not return exit_code 10", true))
	}

	validation, err := mcpRunManifest(ctx, endpoint, token, filepath.Join(repoRootForOpts(opts), "testdata", "manifests", "fake-validation-failure.yaml"))
	if err != nil {
		report.Add(blocked("mcp_run_manifest_validation_failure", "mcp", BlockerRelayUnreachable, err.Error(), true))
	} else if nestedNumber(validation, "result", "structuredContent", "exit_code") == 21 {
		report.Add(passed("mcp_run_manifest_validation_failure", "mcp", true, "exit_code=21"))
	} else {
		report.Add(failed("mcp_run_manifest_validation_failure", "mcp", BlockerValidationFailed, "validation-failure manifest did not return exit_code 21", true))
	}

	if includeRestart {
		connector.Stop()
		if err := waitProjectOnline(ctx, relayURL, token, false); err != nil {
			report.Add(blocked("connector_offline_observed", "relay", BlockerConnectorOffline, err.Error(), true))
		} else {
			report.Add(passed("connector_offline_observed", "relay", true))
		}
		connector, err = startConnectorRunOnly(ctx, opts, connectorConfig, h.Root)
		if err != nil {
			report.Add(blocked("connector_restart", "relay", BlockerConnectorOffline, err.Error(), true))
		} else if err := waitProjectOnline(ctx, relayURL, token, true); err != nil {
			report.Add(blocked("connector_reconnected", "relay", BlockerConnectorOffline, err.Error(), true))
		} else {
			report.Add(passed("connector_reconnected", "relay", true))
		}

		daemon.Stop()
		if stoppedPayload, err := mcpCall(ctx, endpoint, token, "codencer.submit_project_task_and_wait", map[string]any{"project_id": "codencer", "goal": "daemon stopped", "profile": "fake-success"}); err != nil {
			report.Add(passed("daemon_stop_observed", "runtime", true, Redact(err.Error())))
		} else if nestedBool(stoppedPayload, "result", "structuredContent", "ok") == false {
			blockerType := nestedString(stoppedPayload, "result", "structuredContent", "blocker", "type")
			if blockerType == localexec.BlockerDaemonNotRunning || blockerType == localexec.BlockerBridgeError {
				report.Add(passed("daemon_stop_observed", "runtime", true, "blocker="+blockerType))
			} else {
				report.Add(failed("daemon_stop_observed", "runtime", BlockerUnknown, "unexpected daemon stopped blocker "+blockerType, true))
			}
		} else {
			report.Add(failed("daemon_stop_observed", "runtime", BlockerUnknown, "expected daemon stopped call to return a structured blocker", true))
		}
		daemon, daemonURL, err = startDaemon(ctx, h, opts)
		if err != nil {
			report.Add(blocked("daemon_restart", "runtime", BlockerDaemonNotRunning, err.Error(), true))
		} else {
			_ = h.registerProject("codencer", "fake", "fake-success", daemonURL, true)
			_ = daemonURL
			report.Add(passed("daemon_restart", "runtime", true))
		}

		service := localexec.NewService()
		manifestPath := filepath.Join(repoRootForOpts(opts), "testdata", "manifests", "fake-success.yaml")
		successManifest, _, loadErr := manifestpkg.Load(manifestPath)
		var manifest *manifestpkg.Manifest
		if loadErr == nil {
			manifest = successManifest
		}
		runReport, runErr := service.RunPlan(ctx, localexec.RunPlanOptions{
			BaseOptions:  localexec.BaseOptions{ProjectID: "codencer", RepoRoot: h.RepoRoot, CodencerHome: h.Home},
			Manifest:     manifest,
			ManifestPath: manifestPath,
			Wait:         true,
		})
		report.Add(checkFromRunPlan("post_restart_fake_run_plan", "runtime", runReport, runErr))
		watchdog, watchErr := supervisor.WatchdogOnce(ctx, supervisor.Options{ProjectID: "codencer", RepoRoot: h.RepoRoot, BinDir: opts.BinDir, Manager: supervisor.ManagerManual})
		if watchErr != nil {
			report.Add(blocked("watchdog_once", "runtime", BlockerUnknown, watchErr.Error(), true))
		} else {
			report.Add(passed("watchdog_once", "runtime", true, fmt.Sprintf("ok=%t", watchdog.OK)))
		}
	}

	report.Finish(opts)
	return report, nil
}

func startRelay(ctx context.Context, h *workspaceHarness, opts Options) (*processHandle, string, string, string, error) {
	port, err := freePort()
	if err != nil {
		return nil, "", "", "", err
	}
	binary, err := resolveBinary(opts, "codencer-relayd")
	if err != nil {
		return nil, "", "", "", err
	}
	relayURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	token := "planner-token"
	configPath := filepath.Join(h.Root, "relay.json")
	config := map[string]any{
		"host":            "127.0.0.1",
		"port":            port,
		"db_path":         filepath.Join(h.Root, "relay.db"),
		"public_base_url": relayURL,
		"planner_tokens": []map[string]any{{
			"name":        "operator",
			"token":       token,
			"scopes":      []string{"admin:read", "admin:write", "connectors:enroll", "projects:read", "projects:write", "reports:read", "runs:read", "runs:write", "steps:read", "steps:write", "artifacts:read"},
			"project_ids": []string{"codencer"},
		}},
		"proxy_timeout_seconds": 60,
		"allowed_origins":       []string{relayURL},
	}
	if err := writeJSONFile(configPath, config, 0600); err != nil {
		return nil, "", "", "", err
	}
	cmd := exec.CommandContext(ctx, binary, "serve", "--config", configPath)
	logPath := filepath.Join(h.Root, "relay.log")
	if err := startProcess(cmd, logPath); err != nil {
		return nil, "", "", "", err
	}
	handle := &processHandle{cmd: cmd, logPath: logPath}
	if err := waitHTTP(ctx, relayURL+"/api/v2/status", token, 15*time.Second); err != nil {
		handle.Stop()
		return nil, "", "", "", fmt.Errorf("relay did not become healthy: %w; log: %s", err, readSmall(logPath))
	}
	return handle, relayURL, token, configPath, nil
}

func createEnrollmentSecret(ctx context.Context, opts Options, relayConfig string) (string, error) {
	binary, err := resolveBinary(opts, "codencer-relayd")
	if err != nil {
		return "", err
	}
	out, err := commandOutput(ctx, nil, binary, "enrollment-token", "create", "--config", relayConfig, "--label", "live-relay-mcp", "--expires-in-seconds", "600", "--json")
	if err != nil {
		return "", fmt.Errorf("create enrollment token: %w: %s", err, Redact(string(out)))
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		return "", err
	}
	secret, _ := payload["secret"].(string)
	if secret == "" {
		return "", fmt.Errorf("enrollment token response missing secret")
	}
	return secret, nil
}

func startConnector(ctx context.Context, h *workspaceHarness, opts Options, relayURL, daemonURL, secret, configPath string) (*processHandle, error) {
	binary, err := resolveBinary(opts, "codencer-connectord")
	if err != nil {
		return nil, err
	}
	out, err := commandOutput(ctx, nil, binary, "enroll", "--relay-url", relayURL, "--daemon-url", daemonURL, "--enrollment-token", secret, "--config", configPath, "--codencer-home", h.Home)
	if err != nil {
		return nil, fmt.Errorf("connector enroll: %w: %s", err, Redact(string(out)))
	}
	return startConnectorRunOnly(ctx, opts, configPath, h.Root)
}

func startConnectorRunOnly(ctx context.Context, opts Options, configPath, root string) (*processHandle, error) {
	binary, err := resolveBinary(opts, "codencer-connectord")
	if err != nil {
		return nil, err
	}
	logPath := filepath.Join(root, fmt.Sprintf("connector-%d.log", time.Now().UnixNano()))
	cmd := exec.CommandContext(ctx, binary, "run", "--config", configPath)
	if err := startProcess(cmd, logPath); err != nil {
		return nil, err
	}
	return &processHandle{cmd: cmd, logPath: logPath}, nil
}

func waitProjectOnline(ctx context.Context, relayURL, token string, online bool) error {
	client := &http.Client{Timeout: 1200 * time.Millisecond}
	deadline := time.Now().Add(20 * time.Second)
	var last string
	for time.Now().Before(deadline) {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, relayURL+"/api/v2/projects", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := client.Do(req)
		if err == nil {
			data, _ := ioReadAllAndClose(resp)
			last = string(data)
			var projects []map[string]any
			if json.Unmarshal(data, &projects) == nil {
				for _, p := range projects {
					if p["project_id"] == "codencer" {
						gotOnline, _ := p["online"].(bool)
						if gotOnline == online {
							return nil
						}
					}
				}
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return fmt.Errorf("project online=%t not observed; last=%s", online, Redact(last))
}

func mcpRunManifest(ctx context.Context, endpoint, token, manifestPath string) (map[string]any, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}
	return mcpCall(ctx, endpoint, token, "codencer.run_project_manifest", map[string]any{
		"project_id":    "codencer",
		"manifest_text": string(data),
		"manifest_name": filepath.Base(manifestPath),
		"wait":          true,
	})
}

func repoRootForOpts(opts Options) string {
	if opts.RepoRoot != "" {
		return opts.RepoRoot
	}
	wd, _ := os.Getwd()
	return wd
}

func nestedString(payload map[string]any, path ...string) string {
	value := nestedValue(payload, path...)
	out, _ := value.(string)
	return out
}

func nestedBool(payload map[string]any, path ...string) bool {
	value := nestedValue(payload, path...)
	out, _ := value.(bool)
	return out
}

func nestedNumber(payload map[string]any, path ...string) int {
	value := nestedValue(payload, path...)
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	default:
		return 0
	}
}

func nestedValue(payload map[string]any, path ...string) any {
	var current any = payload
	for _, key := range path {
		next, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = next[key]
	}
	return current
}

func ioReadAllAndClose(resp *http.Response) ([]byte, error) {
	if resp == nil || resp.Body == nil {
		return nil, nil
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(resp.Body)
	return buf.Bytes(), err
}
