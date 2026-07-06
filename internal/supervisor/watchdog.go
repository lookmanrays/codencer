package supervisor

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent-bridge/internal/connector"
	"agent-bridge/internal/domain"
	"agent-bridge/internal/local"
	"agent-bridge/internal/localexec"
	"agent-bridge/internal/relay"
)

func WatchdogOnce(ctx context.Context, opts Options) (WatchdogReport, error) {
	rt, err := resolveRuntimeContext(opts)
	if err != nil {
		return WatchdogReport{}, err
	}
	report := WatchdogReport{
		OK:          true,
		Platform:    rt.platform,
		Environment: local.DetectEnvironment(),
		ExitCode:    localexec.ExitSuccess,
	}
	addCheck := func(check Check) {
		check.Message = redact(check.Message)
		for i := range check.Observed {
			check.Observed[i] = redact(check.Observed[i])
		}
		report.Checks = append(report.Checks, check)
		if check.Status == local.CheckError {
			report.OK = false
			if check.BlockerType != "" {
				report.Blockers = append(report.Blockers, RuntimeBlocker{
					Type:                    check.BlockerType,
					Message:                 check.Message,
					PlannerDecisionRequired: true,
					ObservedFacts:           check.Observed,
				})
			}
		}
	}

	addCheck(pathCheck("codencer_home", rt.paths.Home))
	addCheck(pathCheck("artifacts_dir", rt.paths.ArtifactsDir))
	addCheck(pathCheck("runtime_dir", rt.paths.RuntimeDir))
	addCheck(registryWatchdogCheck(rt))
	for _, spec := range rt.specs {
		status, _ := performServiceAction(ctx, managerFor(rt.platform, opts.Runner), "status", spec, opts)
		status = enrichHealth(ctx, status, spec, rt.config, opts)
		check := Check{Name: "service_" + spec.Name, Status: local.CheckOK, Message: status.ObservedState}
		if status.ObservedState == StateNotConfigured {
			check.Status = local.CheckSkipped
		} else if status.ObservedState == StateFailed || status.Health == HealthError {
			check.Status = local.CheckError
			check.BlockerType = ProblemUnknownRuntimeState
			check.Message = firstNonEmpty(status.LastError, status.ObservedState)
		}
		addCheck(check)
	}
	addCheck(daemonWatchdogCheck(rt, opts))
	addCheck(relayWatchdogCheck(ctx, rt, opts))
	addCheck(connectorWatchdogCheck(rt))
	for _, check := range executorChecks(rt) {
		addCheck(check)
	}
	for _, check := range staleExecutionChecks(ctx, rt, opts) {
		addCheck(check)
	}
	for _, check := range lockChecks(ctx, rt, opts) {
		addCheck(check)
	}
	return finalizeWatchdog(report), nil
}

func finalizeWatchdog(report WatchdogReport) WatchdogReport {
	if !report.OK {
		report.ExitCode = localexec.ExitBlocked
	}
	return report
}

func pathCheck(name, path string) Check {
	if path == "" {
		return Check{Name: name, Status: local.CheckError, BlockerType: ProblemUnknownRuntimeState, Message: "path is empty"}
	}
	if err := os.MkdirAll(path, 0755); err != nil {
		return Check{Name: name, Status: local.CheckError, BlockerType: ProblemUnknownRuntimeState, Message: err.Error()}
	}
	if err := canWriteDir(path); err != nil {
		return Check{Name: name, Status: local.CheckError, BlockerType: ProblemUnknownRuntimeState, Message: err.Error()}
	}
	return Check{Name: name, Status: local.CheckOK, Message: path}
}

func registryWatchdogCheck(rt *runtimeContext) Check {
	if rt.registry == nil {
		return Check{Name: "project_registry", Status: local.CheckError, BlockerType: ProblemUnknownRuntimeState, Message: "registry unavailable"}
	}
	if rt.project == nil {
		return Check{Name: "project_resolution", Status: local.CheckError, BlockerType: ProblemUnknownRuntimeState, Message: strings.Join(rt.warnings, "; ")}
	}
	return Check{Name: "project_resolution", Status: local.CheckOK, Message: rt.project.ID, Observed: []string{"source=" + rt.resolution, "repo_root=" + rt.project.RepoRoot}}
}

func daemonWatchdogCheck(rt *runtimeContext, opts Options) Check {
	status := local.CheckDaemon(daemonURL(rt), opts.HTTPClient)
	if status.Status == local.RuntimeOK {
		return Check{Name: "daemon_health", Status: local.CheckOK, Message: status.Detail, Observed: []string{status.URL}}
	}
	checkStatus := local.CheckWarning
	if status.Status == local.RuntimeNotRunning || status.Status == local.RuntimeError {
		checkStatus = local.CheckError
	}
	return Check{Name: "daemon_health", Status: checkStatus, BlockerType: ProblemDaemonNotRunning, Message: status.Detail, Observed: []string{status.URL}}
}

func relayWatchdogCheck(ctx context.Context, rt *runtimeContext, opts Options) Check {
	if strings.TrimSpace(rt.config.RelayConfigPath) == "" {
		return Check{Name: "relay_health", Status: local.CheckSkipped, Message: "relay config not configured"}
	}
	cfg, err := relay.LoadConfig(rt.config.RelayConfigPath)
	if err != nil {
		return Check{Name: "relay_health", Status: local.CheckError, BlockerType: ProblemRelayUnreachable, Message: err.Error()}
	}
	baseURL := cfg.PublicBaseURL
	if baseURL == "" {
		scheme := "http"
		if cfg.TLSCertFile != "" {
			scheme = "https"
		}
		baseURL = fmt.Sprintf("%s://%s:%d", scheme, cfg.Host, cfg.Port)
	}
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: healthTimeout(rt.config)}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/api/v2/status", nil)
	if err != nil {
		return Check{Name: "relay_health", Status: local.CheckError, BlockerType: ProblemRelayUnreachable, Message: err.Error()}
	}
	if token := firstPlannerToken(cfg); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return Check{Name: "relay_health", Status: local.CheckError, BlockerType: ProblemRelayUnreachable, Message: err.Error(), Observed: []string{baseURL}}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return Check{Name: "relay_health", Status: local.CheckOK, Message: resp.Status, Observed: []string{baseURL}}
	}
	return Check{Name: "relay_health", Status: local.CheckError, BlockerType: ProblemRelayUnreachable, Message: resp.Status, Observed: []string{baseURL}}
}

func connectorWatchdogCheck(rt *runtimeContext) Check {
	if strings.TrimSpace(rt.config.ConnectorConfigPath) == "" {
		return Check{Name: "connector_status", Status: local.CheckSkipped, Message: "connector config not configured"}
	}
	status, err := connector.LoadStatus(rt.config.ConnectorConfigPath)
	if err != nil {
		return Check{Name: "connector_status", Status: local.CheckError, BlockerType: ProblemConnectorOffline, Message: err.Error()}
	}
	if status.SessionState != connector.SessionStateConnected {
		return Check{Name: "connector_status", Status: local.CheckError, BlockerType: ProblemConnectorOffline, Message: "connector session_state=" + status.SessionState}
	}
	return Check{Name: "connector_status", Status: local.CheckOK, Message: "connected", Observed: []string{"last_heartbeat_at=" + status.LastHeartbeatAt}}
}

func executorChecks(rt *runtimeContext) []Check {
	needed := map[string]bool{}
	for _, p := range rt.registry.Projects {
		profile := p.AdapterProfile
		if profile == "" {
			profile = p.DefaultAdapter
		}
		if strings.HasPrefix(profile, "codex") || p.DefaultAdapter == "codex" {
			needed["codex"] = true
		}
		if strings.HasPrefix(profile, "claude") || p.DefaultAdapter == "claude" {
			needed["claude"] = true
		}
	}
	checks := []Check{}
	for _, binary := range []string{"codex", "claude"} {
		if !needed[binary] {
			continue
		}
		result := local.DefaultProbe(binary, "--version")
		if result.Err != nil {
			checks = append(checks, Check{Name: "executor_" + binary, Status: local.CheckError, BlockerType: ProblemExecutorMissing, Message: result.Err.Error()})
			continue
		}
		checks = append(checks, Check{Name: "executor_" + binary, Status: local.CheckOK, Message: result.Path})
	}
	return checks
}

func staleExecutionChecks(ctx context.Context, rt *runtimeContext, opts Options) []Check {
	if rt.project == nil {
		return nil
	}
	client := localexec.NewClient(daemonURL(rt), opts.HTTPClient)
	runs, err := client.ListRuns(ctx, rt.project.ID)
	if err != nil {
		return nil
	}
	now := now(opts)
	runThreshold := durationOr(rt.config.Runtime.StaleRunningAfter, 30*time.Minute)
	stepThreshold := durationOr(rt.config.Runtime.StaleWaitAfter, 30*time.Minute)
	checks := []Check{}
	for _, run := range runs {
		if run.State.IsTerminal() {
			continue
		}
		if !run.UpdatedAt.IsZero() && now.Sub(run.UpdatedAt) > runThreshold {
			checks = append(checks, Check{Name: "stale_run", Status: local.CheckError, BlockerType: ProblemStaleRun, Message: "run active beyond stale threshold", Observed: []string{run.ID, "updated_at=" + run.UpdatedAt.Format(time.RFC3339)}})
		}
		steps, err := client.GetRunSteps(ctx, run.ID)
		if err != nil {
			continue
		}
		for _, step := range steps {
			if step.State.IsTerminal() || step.State == domain.StepStateNeedsApproval || step.State == domain.StepStateNeedsManualAttention {
				continue
			}
			threshold := stepThreshold
			if step.TimeoutSeconds > 0 {
				threshold = time.Duration(step.TimeoutSeconds) * time.Second
			}
			if !step.UpdatedAt.IsZero() && now.Sub(step.UpdatedAt) > threshold {
				checks = append(checks, Check{Name: "stale_step", Status: local.CheckError, BlockerType: ProblemStaleStep, Message: "step active beyond stale threshold", Observed: []string{step.ID, "state=" + string(step.State), "updated_at=" + step.UpdatedAt.Format(time.RFC3339)}})
			}
		}
	}
	return checks
}

func lockChecks(ctx context.Context, rt *runtimeContext, opts Options) []Check {
	if rt.project == nil {
		return nil
	}
	lockPath := filepath.Join(rt.project.RepoRoot, ".codencer", "workspace", ".codencer.lock")
	data, err := os.ReadFile(lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Check{{Name: "workspace_lock", Status: local.CheckOK, Message: "no lock present"}}
		}
		return []Check{{Name: "workspace_lock", Status: local.CheckWarning, Message: err.Error()}}
	}
	owner := strings.TrimSpace(string(data))
	if owner == "" {
		return []Check{{Name: "workspace_lock", Status: local.CheckError, BlockerType: ProblemStaleLock, Message: "lock file has no owner", Observed: []string{lockPath}}}
	}
	client := localexec.NewClient(daemonURL(rt), opts.HTTPClient)
	run, err := client.GetRun(ctx, owner)
	if err != nil {
		_ = ctx
		return []Check{{Name: "workspace_lock", Status: local.CheckWarning, Message: "lock owner could not be verified", Observed: []string{lockPath, "owner=" + owner}}}
	}
	if run == nil || run.State.IsTerminal() {
		return []Check{{Name: "workspace_lock", Status: local.CheckError, BlockerType: ProblemStaleLock, Message: "lock owner is not active", Observed: []string{lockPath, "owner=" + owner}}}
	}
	return []Check{{Name: "workspace_lock", Status: local.CheckOK, Message: "active lock owner verified", Observed: []string{"owner=" + owner}}}
}

func canWriteDir(dir string) error {
	tmp, err := os.CreateTemp(dir, ".codencer-write-check-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	return os.Remove(name)
}

func durationOr(raw string, fallback time.Duration) time.Duration {
	parsed, err := time.ParseDuration(raw)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func healthTimeout(cfg local.Config) time.Duration {
	return durationOr(cfg.Runtime.ServiceHealthTimeout, 5*time.Second)
}

func firstPlannerToken(cfg *relay.Config) string {
	if cfg == nil {
		return ""
	}
	if len(cfg.PlannerTokens) > 0 {
		return cfg.PlannerTokens[0].Token
	}
	return cfg.PlannerToken
}

func now(opts Options) time.Time {
	if opts.Now != nil {
		return opts.Now().UTC()
	}
	return time.Now().UTC()
}
