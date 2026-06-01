package supervisor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"agent-bridge/internal/local"
	"agent-bridge/internal/localexec"
)

type RecoveryOptions struct {
	Options
	Mode            string
	RunID           string
	RestartServices bool
}

func Recover(ctx context.Context, opts RecoveryOptions) (RecoveryReport, error) {
	rt, err := resolveRuntimeContext(opts.Options)
	if err != nil {
		return RecoveryReport{}, err
	}
	report := RecoveryReport{
		OK:       true,
		DryRun:   opts.DryRun,
		ExitCode: localexec.ExitSuccess,
	}
	if opts.Mode == "run" {
		return recoverRun(ctx, rt, opts, report)
	}
	if opts.Mode == "" || opts.Mode == "all" {
		report.Actions = append(report.Actions, ensureRuntimeDirs(rt.paths, opts.DryRun)...)
	}
	if opts.Mode == "" || opts.Mode == "all" || opts.Mode == "locks" {
		report = recoverLocks(ctx, rt, opts, report)
	}
	if opts.RestartServices {
		report = recoverServices(ctx, rt, opts, report)
	}
	if len(report.Blockers) > 0 {
		report.OK = false
		report.ExitCode = localexec.ExitBlocked
	}
	return report, nil
}

func recoverRun(ctx context.Context, rt *runtimeContext, opts RecoveryOptions, report RecoveryReport) (RecoveryReport, error) {
	if strings.TrimSpace(opts.RunID) == "" {
		return RecoveryReport{}, fmt.Errorf("run id is required")
	}
	body, _ := json.Marshal(map[string]bool{"dry_run": opts.DryRun})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, daemonURL(rt)+"/api/v1/recovery/runs/"+opts.RunID, bytes.NewReader(body))
	if err != nil {
		return RecoveryReport{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: healthTimeout(rt.config)}
	}
	resp, err := client.Do(req)
	if err != nil {
		report.Blockers = append(report.Blockers, RuntimeBlocker{
			Type:                    ProblemDaemonNotRunning,
			Message:                 err.Error(),
			PlannerDecisionRequired: true,
			ObservedFacts:           []string{daemonURL(rt)},
		})
		report.OK = false
		report.ExitCode = localexec.ExitBlocked
		return report, nil
	}
	defer resp.Body.Close()
	var payload map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&payload)
	report.Actions = append(report.Actions, RecoveryAction{
		Type:   "daemon_recover_run",
		Target: opts.RunID,
		Safe:   true,
		Reason: fmt.Sprintf("daemon recovery endpoint returned %s", resp.Status),
		Done:   resp.StatusCode >= 200 && resp.StatusCode < 300 && !opts.DryRun,
	})
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		report.Blockers = append(report.Blockers, RuntimeBlocker{
			Type:                    ProblemUnknownRuntimeState,
			Message:                 fmt.Sprintf("daemon recovery endpoint returned %s", resp.Status),
			PlannerDecisionRequired: true,
			ObservedFacts:           []string{fmt.Sprintf("%v", payload)},
		})
		report.OK = false
		report.ExitCode = localexec.ExitBlocked
	}
	return report, nil
}

func ensureRuntimeDirs(paths local.Paths, dryRun bool) []RecoveryAction {
	dirs := []string{paths.Home, paths.LogsDir, paths.RuntimeDir, paths.TokensDir, paths.ArtifactsDir, filepath.Join(paths.RuntimeDir, "services")}
	actions := make([]RecoveryAction, 0, len(dirs))
	for _, dir := range dirs {
		action := RecoveryAction{Type: "ensure_runtime_dir", Target: dir, Safe: true, Reason: "runtime directory is managed by CODENCER_HOME"}
		if !dryRun {
			perm := os.FileMode(0755)
			if dir == paths.TokensDir {
				perm = 0700
			}
			if err := os.MkdirAll(dir, perm); err == nil {
				action.Done = true
			} else {
				action.Safe = false
				action.Reason = err.Error()
			}
		}
		actions = append(actions, action)
	}
	return actions
}

func recoverLocks(ctx context.Context, rt *runtimeContext, opts RecoveryOptions, report RecoveryReport) RecoveryReport {
	if rt.project == nil {
		report.Blockers = append(report.Blockers, RuntimeBlocker{Type: ProblemUnknownRuntimeState, Message: "no project resolved for lock recovery", PlannerDecisionRequired: true})
		return report
	}
	lockPath := filepath.Join(rt.project.RepoRoot, ".codencer", "workspace", ".codencer.lock")
	data, err := os.ReadFile(lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			report.Actions = append(report.Actions, RecoveryAction{Type: "inspect_lock", Target: lockPath, Safe: true, Reason: "no lock file present", Done: true})
			return report
		}
		report.Blockers = append(report.Blockers, RuntimeBlocker{Type: ProblemUnknownRuntimeState, Message: err.Error(), PlannerDecisionRequired: true, ObservedFacts: []string{lockPath}})
		return report
	}
	owner := strings.TrimSpace(string(data))
	if owner == "" {
		report.Blockers = append(report.Blockers, RuntimeBlocker{Type: ProblemUnknownRuntimeState, Message: "lock exists but owner is empty", PlannerDecisionRequired: true, ObservedFacts: []string{lockPath}})
		return report
	}
	client := localexec.NewClient(daemonURL(rt), opts.HTTPClient)
	run, err := client.GetRun(ctx, owner)
	if err != nil {
		report.Blockers = append(report.Blockers, RuntimeBlocker{Type: ProblemUnknownRuntimeState, Message: "lock exists but run ownership could not be verified", PlannerDecisionRequired: true, ObservedFacts: []string{lockPath, "owner=" + owner, err.Error()}})
		return report
	}
	if run != nil && !run.State.IsTerminal() {
		report.Actions = append(report.Actions, RecoveryAction{Type: "inspect_lock", Target: lockPath, Safe: true, Reason: "lock owner is still active", Done: true, Facts: []string{"owner=" + owner}})
		return report
	}
	action := RecoveryAction{Type: "remove_stale_lock", Target: lockPath, Safe: true, Reason: "lock owner is missing or terminal", Facts: []string{"owner=" + owner}}
	if !opts.DryRun {
		if err := os.Remove(lockPath); err != nil && !os.IsNotExist(err) {
			action.Safe = false
			action.Reason = err.Error()
			report.Blockers = append(report.Blockers, RuntimeBlocker{Type: ProblemUnknownRuntimeState, Message: err.Error(), PlannerDecisionRequired: true, ObservedFacts: []string{lockPath}})
		} else {
			action.Done = true
		}
	}
	report.Actions = append(report.Actions, action)
	return report
}

func recoverServices(ctx context.Context, rt *runtimeContext, opts RecoveryOptions, report RecoveryReport) RecoveryReport {
	mgr := managerFor(rt.platform, opts.Runner)
	for _, spec := range rt.specs {
		if !spec.Configured {
			continue
		}
		status := mgr.Status(ctx, spec)
		if !status.Installed || status.ObservedState == StateRunning {
			continue
		}
		action := RecoveryAction{Type: "restart_service", Target: spec.Name, Safe: true, Reason: "service is installed but not running"}
		if !opts.DryRun {
			_, err := mgr.Restart(ctx, spec, false)
			if err != nil {
				action.Safe = false
				action.Reason = err.Error()
				report.Blockers = append(report.Blockers, RuntimeBlocker{Type: ProblemUnknownRuntimeState, Message: err.Error(), PlannerDecisionRequired: true, ObservedFacts: []string{spec.Name}})
			} else {
				action.Done = true
			}
		}
		report.Actions = append(report.Actions, action)
	}
	return report
}
