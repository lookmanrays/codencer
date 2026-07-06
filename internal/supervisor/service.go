package supervisor

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"agent-bridge/internal/local"
	"agent-bridge/internal/localexec"
)

func Service(ctx context.Context, action string, opts Options) (ServiceReport, error) {
	rt, err := resolveRuntimeContext(opts)
	if err != nil {
		return ServiceReport{}, err
	}
	selected, err := selectSpecs(rt.specs, opts.Service, opts.All)
	if err != nil {
		return ServiceReport{}, err
	}
	selected = orderSpecsForAction(selected, action)
	mgr := managerFor(rt.platform, opts.Runner)
	report := ServiceReport{
		OK:       true,
		Action:   action,
		DryRun:   opts.DryRun,
		Platform: rt.platform,
		Warnings: append([]string(nil), rt.warnings...),
		ExitCode: localexec.ExitSuccess,
	}
	if action == "install" || action == "start" || action == "restart" {
		if !opts.DryRun {
			if _, err := local.EnsureHome(rt.paths, now(opts)); err != nil {
				return report, err
			}
		}
		if _, err := writeGeneratedConfigs(rt, selected, opts.DryRun); err != nil {
			return report, err
		}
	}
	for _, spec := range selected {
		status, err := performServiceAction(ctx, mgr, action, spec, opts)
		if err != nil {
			status.LastError = redact(err.Error())
		}
		status = enrichHealth(ctx, status, spec, rt.config, opts)
		report.Services = append(report.Services, status)
		if status.failed(opts.Strict) || (opts.Strict && status.ObservedState == StateNotConfigured) {
			report.OK = false
		}
	}
	if !report.OK {
		report.ExitCode = localexec.ExitDaemonFailed
	}
	return report, nil
}

func orderSpecsForAction(specs []ServiceSpec, action string) []ServiceSpec {
	switch action {
	case "start", "restart":
		return orderSpecs(specs, []string{ServiceRelay, ServiceDaemon, ServiceConnector})
	case "stop", "uninstall":
		return orderSpecs(specs, []string{ServiceConnector, ServiceDaemon, ServiceRelay})
	default:
		return specs
	}
}

func orderSpecs(specs []ServiceSpec, names []string) []ServiceSpec {
	byName := map[string]ServiceSpec{}
	for _, spec := range specs {
		byName[spec.Name] = spec
	}
	ordered := make([]ServiceSpec, 0, len(specs))
	seen := map[string]bool{}
	for _, name := range names {
		if spec, ok := byName[name]; ok {
			ordered = append(ordered, spec)
			seen[name] = true
		}
	}
	for _, spec := range specs {
		if !seen[spec.Name] {
			ordered = append(ordered, spec)
		}
	}
	return ordered
}

func RenderService(opts Options) (string, error) {
	rt, err := resolveRuntimeContext(opts)
	if err != nil {
		return "", err
	}
	selected, err := selectSpecs(rt.specs, opts.Service, false)
	if err != nil {
		return "", err
	}
	format := opts.Format
	if format == "" || format == ManagerAuto {
		format = rt.platform.ServiceManager
		if format == ManagerManual {
			format = ManagerSystemd
		}
	}
	return Render(selected[0], format)
}

func Logs(ctx context.Context, opts Options, stdout io.Writer) (ServiceReport, error) {
	rt, err := resolveRuntimeContext(opts)
	if err != nil {
		return ServiceReport{}, err
	}
	selected, err := selectSpecs(rt.specs, opts.Service, false)
	if err != nil {
		return ServiceReport{}, err
	}
	spec := selected[0]
	report := ServiceReport{
		OK:       true,
		Action:   "logs",
		Platform: rt.platform,
		ExitCode: localexec.ExitSuccess,
	}
	status := baseStatus(spec, rt.platform.ServiceManager)
	status.ObservedState = StateUnknown
	report.Services = append(report.Services, status)
	tail := opts.Tail
	if tail <= 0 {
		tail = 100
	}
	found := false
	for _, path := range []string{spec.StdoutLog, spec.StderrLog} {
		data, err := tailFile(path, tail)
		if err != nil {
			continue
		}
		found = true
		if _, err := fmt.Fprintf(stdout, "==> %s <==\n%s", path, data); err != nil {
			return report, err
		}
		if !strings.HasSuffix(data, "\n") {
			_, _ = fmt.Fprintln(stdout)
		}
	}
	if !found {
		_, _ = fmt.Fprintf(stdout, "no log files found for %s\n", spec.Name)
	}
	if opts.Follow {
		_, _ = fmt.Fprintln(stdout, "follow mode is not available in this non-interactive invocation")
	}
	_ = ctx
	return report, nil
}

func performServiceAction(ctx context.Context, mgr Manager, action string, spec ServiceSpec, opts Options) (ServiceStatus, error) {
	if !spec.Configured {
		status := baseStatus(spec, mgr.Name())
		status.ObservedState = StateNotConfigured
		status.Health = HealthNotConfigured
		return status, nil
	}
	if spec.Binary == "" && action != "status" && action != "uninstall" {
		status := baseStatus(spec, mgr.Name())
		status.ObservedState = StateNotConfigured
		status.Health = HealthNotConfigured
		status.LastError = firstNonEmpty(spec.LastError, "binary is not configured")
		return status, nil
	}
	switch action {
	case "install":
		return mgr.Install(ctx, spec, opts.DryRun)
	case "uninstall":
		return mgr.Uninstall(ctx, spec, opts.DryRun)
	case "start":
		return mgr.Start(ctx, spec, opts.DryRun)
	case "stop":
		return mgr.Stop(ctx, spec, opts.DryRun)
	case "restart":
		return mgr.Restart(ctx, spec, opts.DryRun)
	case "status":
		return mgr.Status(ctx, spec), nil
	default:
		return ServiceStatus{}, fmt.Errorf("unknown service action %q", action)
	}
}

func enrichHealth(ctx context.Context, status ServiceStatus, spec ServiceSpec, cfg local.Config, opts Options) ServiceStatus {
	if !spec.Configured {
		status.Health = HealthNotConfigured
		return status
	}
	if spec.HealthURL == "" {
		if status.Health == "" || status.Health == HealthUnknown {
			status.Health = HealthUnknown
		}
		return status
	}
	timeout := 1200 * time.Millisecond
	if parsed, err := time.ParseDuration(cfg.Runtime.ServiceHealthTimeout); err == nil && parsed > 0 {
		timeout = parsed
	}
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, spec.HealthURL, nil)
	if err != nil {
		status.Health = HealthError
		status.LastError = redact(err.Error())
		return status
	}
	resp, err := client.Do(req)
	if err != nil {
		status.Health = HealthNotRunning
		if status.ObservedState == StateUnknown {
			status.ObservedState = StateNotRunning
		}
		return status
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		status.Health = HealthOK
		return status
	}
	status.Health = HealthError
	status.LastError = fmt.Sprintf("health returned %s", resp.Status)
	return status
}

func tailFile(path string, lines int) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	parts := strings.SplitAfter(string(data), "\n")
	if len(parts) > lines {
		parts = parts[len(parts)-lines:]
	}
	return strings.Join(parts, ""), nil
}
