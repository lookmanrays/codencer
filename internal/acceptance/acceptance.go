package acceptance

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent-bridge/internal/buildinfo"
	"agent-bridge/internal/live"
	"agent-bridge/internal/local"
	"agent-bridge/internal/project"
	"agent-bridge/internal/readiness"
	"agent-bridge/internal/release"
	"agent-bridge/internal/security"
	"agent-bridge/internal/setup"
	"agent-bridge/internal/supervisor"
)

const (
	VerdictReady          = "ready"
	VerdictReadyWithSkips = "ready_with_skips"
	VerdictNotReady       = "not_ready"
)

type Options struct {
	Profile              string
	EnableCodex          bool
	EnableClaude         bool
	EnableRelayMCP       bool
	EnableServiceRestart bool
	EnableAll            bool
	CodencerHome         string
	RepoRoot             string
	BinDir               string
	Now                  func() time.Time
}

type Gate struct {
	ID       string         `json:"id"`
	Status   string         `json:"status"`
	Required bool           `json:"required"`
	Live     bool           `json:"live"`
	Reason   string         `json:"reason,omitempty"`
	Evidence map[string]any `json:"evidence,omitempty"`
}

type Report struct {
	OK               bool           `json:"ok"`
	Verdict          string         `json:"verdict"`
	Version          string         `json:"version"`
	Commit           string         `json:"commit,omitempty"`
	Profile          string         `json:"profile"`
	StartedAt        time.Time      `json:"started_at"`
	CompletedAt      time.Time      `json:"completed_at"`
	Gates            []Gate         `json:"gates"`
	BlockingGates    []string       `json:"blocking_gates"`
	SkippedLiveGates []string       `json:"skipped_live_gates"`
	Reports          []string       `json:"reports,omitempty"`
	ReportPath       string         `json:"report_path,omitempty"`
	Build            buildinfo.Info `json:"build"`
}

func LocalProduction(ctx context.Context, opts Options) (Report, error) {
	started := now(opts.Now)
	build := buildinfo.Current()
	profile := firstNonEmpty(opts.Profile, "all")
	repo, err := repoRoot(opts.RepoRoot)
	if err != nil {
		return Report{}, err
	}
	paths, err := local.ResolvePathsForHome(repo, "", opts.CodencerHome)
	if err != nil {
		return Report{}, err
	}
	_, _ = local.EnsureHome(paths, started)
	report := Report{
		OK:        true,
		Verdict:   VerdictReady,
		Version:   build.Version,
		Commit:    build.Commit,
		Profile:   profile,
		StartedAt: started,
		Build:     build,
	}

	cfg, _ := local.LoadConfig(paths.ConfigFile)
	registry, _ := project.LoadRegistry(paths.ProjectsFile)
	doctor := local.BuildDoctorReport(local.DoctorOptions{Paths: paths, Config: cfg, RepoRoot: repo, ToolchainOnly: true})
	report.add(gate("doctor_toolchain", statusForOK(doctor.OK), true, false, fmt.Sprintf("errors=%d warnings=%d", doctor.Summary.Errors, doctor.Summary.Warnings), nil))
	report.add(gate("project_registry", registryStatus(registry), true, false, "", map[string]any{"project_count": len(registry.Projects)}))

	if profile == "local" || profile == "all" {
		demo, err := setup.DemoLocal(ctx, setup.DemoOptions{BinDir: opts.BinDir})
		report.Reports = append(report.Reports, demo.Reports...)
		if err != nil {
			report.add(gate("local_demo", live.StatusFailed, true, false, err.Error(), nil))
		} else {
			report.add(gate("local_manifest_fake_success", stepStatus(demo, "manifest_success"), true, false, stepDetail(demo, "manifest_success"), nil))
			report.add(gate("local_manifest_fake_blocker", stepStatus(demo, "manifest_blocker"), true, false, stepDetail(demo, "manifest_blocker"), nil))
			report.add(gate("local_manifest_validation_failure", stepStatus(demo, "manifest_validation_failure"), true, false, stepDetail(demo, "manifest_validation_failure"), nil))
		}
	} else {
		report.add(gate("local_manifest_fake_success", live.StatusSkipped, false, false, "profile excludes local gates", nil))
	}

	if profile == "relay" || profile == "all" {
		relayReport, err := live.RunRelayMCP(ctx, live.Options{Profile: "relay", EnableRelayMCP: true, RepoRoot: repo, BinDir: opts.BinDir, CodencerHome: opts.CodencerHome})
		report.Reports = appendNonEmpty(report.Reports, relayReport.ReportPath)
		if err != nil {
			report.add(gate("relay_mcp_fake_execution", live.StatusFailed, true, false, err.Error(), nil))
		} else {
			report.add(gate("relay_mcp_fake_execution", statusForOK(relayReport.OK), true, false, fmt.Sprintf("passed=%d failed=%d blocked=%d", relayReport.Summary.Passed, relayReport.Summary.Failed, relayReport.Summary.Blocked), map[string]any{"report_path": relayReport.ReportPath}))
		}
	} else {
		report.add(gate("relay_mcp_fake_execution", live.StatusSkipped, false, false, "profile excludes relay gates", nil))
	}

	service, err := supervisor.Service(ctx, "install", supervisor.Options{All: true, RepoRoot: repo, BinDir: opts.BinDir, DryRun: true, Manager: supervisor.ManagerManual})
	if err != nil {
		report.add(gate("service_supervisor_dry_run", live.StatusFailed, true, false, err.Error(), nil))
	} else {
		report.add(gate("service_supervisor_dry_run", statusForOK(service.OK), true, false, fmt.Sprintf("services=%d", len(service.Services)), nil))
	}
	watchdog, err := supervisor.WatchdogOnce(ctx, supervisor.Options{RepoRoot: repo, BinDir: opts.BinDir, Manager: supervisor.ManagerManual})
	if err != nil {
		report.add(gate("watchdog_command_runs", live.StatusFailed, true, false, err.Error(), nil))
		report.add(gate("watchdog_health_ok", live.StatusSkipped, true, false, "watchdog command did not complete", nil))
	} else {
		report.add(gate("watchdog_command_runs", live.StatusPassed, true, false, fmt.Sprintf("checks=%d blockers=%d", len(watchdog.Checks), len(watchdog.Blockers)), nil))
		report.add(gate("watchdog_health_ok", watchdogHealthStatus(watchdog), true, false, fmt.Sprintf("ok=%t blockers=%d", watchdog.OK, len(watchdog.Blockers)), nil))
	}
	recovery, err := supervisor.Recover(ctx, supervisor.RecoveryOptions{Options: supervisor.Options{RepoRoot: repo, BinDir: opts.BinDir, DryRun: true, Manager: supervisor.ManagerManual}, Mode: "all"})
	if err != nil {
		report.add(gate("recover_dry_run_command_runs", live.StatusFailed, true, false, err.Error(), nil))
		report.add(gate("recover_dry_run_safe", live.StatusSkipped, true, false, "recover dry-run did not complete", nil))
	} else {
		report.add(gate("recover_dry_run_command_runs", live.StatusPassed, true, false, fmt.Sprintf("actions=%d blockers=%d", len(recovery.Actions), len(recovery.Blockers)), nil))
		report.add(gate("recover_dry_run_safe", recoverySafetyStatus(recovery), true, false, fmt.Sprintf("ok=%t actions=%d blockers=%d", recovery.OK, len(recovery.Actions), len(recovery.Blockers)), nil))
	}
	ready, err := readiness.Build(ctx, readiness.Options{Local: true, Relay: profile == "relay", RepoRoot: repo, CodencerHome: opts.CodencerHome})
	if err != nil {
		report.add(gate("readiness", live.StatusFailed, true, false, err.Error(), nil))
	} else {
		report.Reports = appendNonEmpty(report.Reports, ready.ReportPath)
		report.add(gate("readiness", readinessStatus(ready.Verdict), true, false, ready.Verdict, map[string]any{"report_path": ready.ReportPath}))
	}
	matrix, err := live.Matrix(ctx, live.Options{Profile: matrixProfile(profile), RepoRoot: repo, BinDir: opts.BinDir, CodencerHome: opts.CodencerHome})
	if err != nil {
		report.add(gate("live_matrix", live.StatusFailed, true, false, err.Error(), nil))
	} else {
		report.Reports = appendNonEmpty(report.Reports, matrix.ReportPath)
		report.add(gate("live_matrix", statusForOK(matrix.OK), true, false, fmt.Sprintf("skipped=%d", matrix.Summary.Skipped), map[string]any{"report_path": matrix.ReportPath}))
	}

	if opts.EnableCodex || opts.EnableAll {
		addLiveSubreport(ctx, &report, "codex_live_execution", live.RunCodex, live.Options{EnableCodex: true, RepoRoot: repo, BinDir: opts.BinDir, CodencerHome: opts.CodencerHome})
	} else {
		report.add(gate("codex_live_execution", live.StatusSkipped, false, true, "not enabled", nil))
	}
	if opts.EnableClaude || opts.EnableAll {
		addLiveSubreport(ctx, &report, "claude_live_execution", live.RunClaude, live.Options{EnableClaude: true, RepoRoot: repo, BinDir: opts.BinDir, CodencerHome: opts.CodencerHome})
	} else {
		report.add(gate("claude_live_execution", live.StatusSkipped, false, true, "not enabled", nil))
	}
	if opts.EnableServiceRestart || opts.EnableAll {
		addLiveSubreport(ctx, &report, "restart_reconnect_live", live.RunRestartReconnect, live.Options{EnableServiceRestart: true, RepoRoot: repo, BinDir: opts.BinDir, CodencerHome: opts.CodencerHome})
	} else {
		report.add(gate("restart_reconnect_live", live.StatusSkipped, false, true, "not enabled", nil))
	}

	securityChecks := security.RunChecks(security.Options{Paths: paths, Config: cfg, Registry: registry, RepoRoot: repo})
	for _, check := range securityChecks {
		report.add(gate(check.ID, mapSecurityStatus(check.Status), check.Required, false, check.Reason, map[string]any{"observed_facts": check.ObservedFacts}))
	}
	report.add(gate("docs_release_docs_present", docsStatus(repo), true, false, "", nil))
	report.add(gate("release_manifest_available", releaseManifestStatus(repo), false, false, "dist/manifest.json is optional until release-snapshot is run", nil))
	report.add(gate("release_artifacts_present", releaseArtifactsStatus(repo), true, false, "built manifest artifacts must exist and match checksums", nil))

	report.CompletedAt = time.Now().UTC()
	report.computeVerdict()
	path, err := live.PersistJSON(paths, "acceptance", report)
	if err != nil {
		return report, err
	}
	report.ReportPath = path
	if err := overwrite(path, report); err != nil {
		return report, err
	}
	return report, nil
}

func Reports(homeOverride string) ([]live.ReportFile, error) {
	return live.ListReports(homeOverride, "acceptance")
}

func (r *Report) add(g Gate) {
	r.Gates = append(r.Gates, g)
}

func (r *Report) computeVerdict() {
	r.OK = true
	skips := false
	for _, gate := range r.Gates {
		switch gate.Status {
		case live.StatusFailed, live.StatusBlocked:
			if gate.Required {
				r.OK = false
				r.BlockingGates = append(r.BlockingGates, gate.ID)
			}
		case live.StatusSkipped, live.StatusUnsupported, live.StatusNotConfigured:
			skips = true
			if gate.Live {
				r.SkippedLiveGates = append(r.SkippedLiveGates, gate.ID)
			}
		}
	}
	if !r.OK {
		r.Verdict = VerdictNotReady
		return
	}
	if skips {
		r.Verdict = VerdictReadyWithSkips
		return
	}
	r.Verdict = VerdictReady
}

func gate(id, status string, required, liveGate bool, reason string, evidence map[string]any) Gate {
	return Gate{ID: id, Status: status, Required: required, Live: liveGate, Reason: live.Redact(reason), Evidence: evidence}
}

func registryStatus(registry *project.Registry) string {
	if registry == nil || len(registry.Projects) == 0 {
		return live.StatusSkipped
	}
	return live.StatusPassed
}

func stepStatus(report setup.Report, id string) string {
	for _, step := range report.Steps {
		if step.ID == id {
			if step.Status == "passed" {
				return live.StatusPassed
			}
			if step.Status == "skipped" {
				return live.StatusSkipped
			}
			return live.StatusFailed
		}
	}
	return live.StatusFailed
}

func stepDetail(report setup.Report, id string) string {
	for _, step := range report.Steps {
		if step.ID == id {
			return step.Detail
		}
	}
	return "missing step"
}

func addLiveSubreport(ctx context.Context, report *Report, id string, fn func(context.Context, live.Options) (live.Report, error), opts live.Options) {
	child, err := fn(ctx, opts)
	report.Reports = appendNonEmpty(report.Reports, child.ReportPath)
	if err != nil {
		report.add(gate(id, live.StatusFailed, false, true, err.Error(), nil))
		return
	}
	report.add(gate(id, statusForOK(child.OK), false, true, fmt.Sprintf("passed=%d skipped=%d", child.Summary.Passed, child.Summary.Skipped), map[string]any{"report_path": child.ReportPath}))
}

func statusForOK(ok bool) string {
	if ok {
		return live.StatusPassed
	}
	return live.StatusFailed
}

func readinessStatus(verdict string) string {
	if verdict == readiness.VerdictNotReady {
		return live.StatusFailed
	}
	return live.StatusPassed
}

func mapSecurityStatus(status string) string {
	switch status {
	case security.StatusPassed:
		return live.StatusPassed
	case security.StatusFailed:
		return live.StatusFailed
	case security.StatusNotConfigured:
		return live.StatusNotConfigured
	default:
		return live.StatusSkipped
	}
}

func docsStatus(repo string) string {
	required := []string{"README.md", "docs/local-production.md", "docs/SELF_HOST_REFERENCE.md", "docs/runtime-supervisor.md", "docs/live-execution-matrix.md", "docs/mcp/integrations.md", "docs/mcp/chatgpt-live-checklist.md", "docs/acceptance/local-production-v0.3.yaml"}
	for _, rel := range required {
		if _, err := os.Stat(filepath.Join(repo, rel)); err != nil {
			return live.StatusFailed
		}
	}
	return live.StatusPassed
}

func releaseManifestStatus(repo string) string {
	if _, err := os.Stat(filepath.Join(repo, "dist", "manifest.json")); err != nil {
		return live.StatusSkipped
	}
	return live.StatusPassed
}

func releaseArtifactsStatus(repo string) string {
	if _, err := os.Stat(filepath.Join(repo, "dist", "manifest.json")); err != nil {
		return live.StatusSkipped
	}
	if err := release.ValidateDist(filepath.Join(repo, "dist")); err != nil {
		return live.StatusFailed
	}
	return live.StatusPassed
}

func watchdogHealthStatus(report supervisor.WatchdogReport) string {
	if report.OK {
		return live.StatusPassed
	}
	if len(report.Blockers) > 0 {
		return live.StatusBlocked
	}
	return live.StatusFailed
}

func recoverySafetyStatus(report supervisor.RecoveryReport) string {
	if report.OK {
		return live.StatusPassed
	}
	if len(report.Blockers) > 0 {
		return live.StatusBlocked
	}
	return live.StatusFailed
}

func matrixProfile(profile string) string {
	switch profile {
	case "relay":
		return "relay"
	case "local":
		return "local"
	default:
		return "all"
	}
}

func repoRoot(value string) (string, error) {
	if strings.TrimSpace(value) != "" {
		return filepath.Abs(value)
	}
	return os.Getwd()
}

func now(fn func() time.Time) time.Time {
	if fn != nil {
		return fn().UTC()
	}
	return time.Now().UTC()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func appendNonEmpty(values []string, next string) []string {
	if strings.TrimSpace(next) == "" {
		return values
	}
	return append(values, next)
}

func overwrite(path string, report Report) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0600)
}
