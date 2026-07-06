package supervisor

import (
	"context"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

type Manager interface {
	Install(ctx context.Context, spec ServiceSpec, dryRun bool) (ServiceStatus, error)
	Uninstall(ctx context.Context, spec ServiceSpec, dryRun bool) (ServiceStatus, error)
	Start(ctx context.Context, spec ServiceSpec, dryRun bool) (ServiceStatus, error)
	Stop(ctx context.Context, spec ServiceSpec, dryRun bool) (ServiceStatus, error)
	Restart(ctx context.Context, spec ServiceSpec, dryRun bool) (ServiceStatus, error)
	Status(ctx context.Context, spec ServiceSpec) ServiceStatus
	Name() string
}

func managerFor(platform PlatformInfo, runner CommandRunner) Manager {
	switch platform.ServiceManager {
	case ManagerLaunchd:
		return launchdManager{runner: runnerOrDefault(runner)}
	case ManagerSystemd:
		return systemdManager{runner: runnerOrDefault(runner)}
	default:
		return manualManager{platform: platform}
	}
}

type launchdManager struct {
	runner CommandRunner
}

func (m launchdManager) Name() string { return ManagerLaunchd }

func (m launchdManager) Install(ctx context.Context, spec ServiceSpec, dryRun bool) (ServiceStatus, error) {
	status := baseStatus(spec, m.Name())
	rendered, err := RenderLaunchd(spec)
	if err != nil {
		status.LastError = err.Error()
		return status, err
	}
	status.Rendered = rendered
	status.DesiredState = StateInstalled
	if dryRun {
		return status, nil
	}
	if err := writeTextFile(spec.LaunchAgentPath, rendered, 0644); err != nil {
		status.LastError = err.Error()
		return status, err
	}
	target := launchdTarget(spec)
	result := m.runner.Run(ctx, "launchctl", "bootstrap", launchdDomain(), spec.LaunchAgentPath)
	if result.Err != nil && !strings.Contains(result.Stderr+result.Stdout, "already bootstrapped") {
		status.LastError = commandError(result)
		return status, result.Err
	}
	status.Installed = true
	status.ObservedState = StateInstalled
	_ = target
	return status, nil
}

func (m launchdManager) Uninstall(ctx context.Context, spec ServiceSpec, dryRun bool) (ServiceStatus, error) {
	status := baseStatus(spec, m.Name())
	status.DesiredState = StateNotInstalled
	if dryRun {
		return status, nil
	}
	_ = m.runner.Run(ctx, "launchctl", "bootout", launchdDomain()+"/"+spec.Label)
	_ = os.Remove(spec.LaunchAgentPath)
	status.Installed = false
	status.ObservedState = StateNotInstalled
	return status, nil
}

func (m launchdManager) Start(ctx context.Context, spec ServiceSpec, dryRun bool) (ServiceStatus, error) {
	status := baseStatus(spec, m.Name())
	status.DesiredState = StateRunning
	if dryRun {
		return status, nil
	}
	result := m.runner.Run(ctx, "launchctl", "kickstart", "-k", launchdDomain()+"/"+spec.Label)
	if result.Err != nil {
		status.LastError = commandError(result)
		return status, result.Err
	}
	return m.Status(ctx, spec), nil
}

func (m launchdManager) Stop(ctx context.Context, spec ServiceSpec, dryRun bool) (ServiceStatus, error) {
	status := baseStatus(spec, m.Name())
	status.DesiredState = StateNotRunning
	if dryRun {
		return status, nil
	}
	result := m.runner.Run(ctx, "launchctl", "bootout", launchdDomain()+"/"+spec.Label)
	if result.Err != nil && !strings.Contains(result.Stderr+result.Stdout, "No such process") {
		status.LastError = commandError(result)
		return status, result.Err
	}
	return m.Status(ctx, spec), nil
}

func (m launchdManager) Restart(ctx context.Context, spec ServiceSpec, dryRun bool) (ServiceStatus, error) {
	status := baseStatus(spec, m.Name())
	status.DesiredState = StateRunning
	if dryRun {
		return status, nil
	}
	result := m.runner.Run(ctx, "launchctl", "kickstart", "-k", launchdDomain()+"/"+spec.Label)
	if result.Err != nil {
		status.LastError = commandError(result)
		return status, result.Err
	}
	return m.Status(ctx, spec), nil
}

func (m launchdManager) Status(ctx context.Context, spec ServiceSpec) ServiceStatus {
	status := baseStatus(spec, m.Name())
	if _, err := os.Stat(spec.LaunchAgentPath); err == nil {
		status.Installed = true
	} else {
		status.ObservedState = StateNotInstalled
		return status
	}
	result := m.runner.Run(ctx, "launchctl", "print", launchdDomain()+"/"+spec.Label)
	if result.Err != nil {
		status.ObservedState = StateNotRunning
		status.LastError = commandError(result)
		return status
	}
	combined := result.Stdout + "\n" + result.Stderr
	status.PID = parsePID(combined)
	if status.PID > 0 || strings.Contains(combined, "state = running") {
		status.ObservedState = StateRunning
	} else {
		status.ObservedState = StateUnknown
	}
	return status
}

type systemdManager struct {
	runner CommandRunner
}

func (m systemdManager) Name() string { return ManagerSystemd }

func (m systemdManager) Install(ctx context.Context, spec ServiceSpec, dryRun bool) (ServiceStatus, error) {
	status := baseStatus(spec, m.Name())
	rendered, err := RenderSystemd(spec)
	if err != nil {
		status.LastError = err.Error()
		return status, err
	}
	status.Rendered = rendered
	status.DesiredState = StateInstalled
	if dryRun {
		return status, nil
	}
	if err := writeTextFile(spec.SystemdUnitPath, rendered, 0644); err != nil {
		status.LastError = err.Error()
		return status, err
	}
	if result := m.runner.Run(ctx, "systemctl", "--user", "daemon-reload"); result.Err != nil {
		status.LastError = commandError(result)
		return status, result.Err
	}
	if result := m.runner.Run(ctx, "systemctl", "--user", "enable", spec.UnitName); result.Err != nil {
		status.LastError = commandError(result)
		return status, result.Err
	}
	status.Installed = true
	status.ObservedState = StateInstalled
	return status, nil
}

func (m systemdManager) Uninstall(ctx context.Context, spec ServiceSpec, dryRun bool) (ServiceStatus, error) {
	status := baseStatus(spec, m.Name())
	status.DesiredState = StateNotInstalled
	if dryRun {
		return status, nil
	}
	_ = m.runner.Run(ctx, "systemctl", "--user", "disable", "--now", spec.UnitName)
	_ = os.Remove(spec.SystemdUnitPath)
	_ = m.runner.Run(ctx, "systemctl", "--user", "daemon-reload")
	status.ObservedState = StateNotInstalled
	return status, nil
}

func (m systemdManager) Start(ctx context.Context, spec ServiceSpec, dryRun bool) (ServiceStatus, error) {
	return m.runSystemctl(ctx, spec, dryRun, StateRunning, "start")
}

func (m systemdManager) Stop(ctx context.Context, spec ServiceSpec, dryRun bool) (ServiceStatus, error) {
	return m.runSystemctl(ctx, spec, dryRun, StateNotRunning, "stop")
}

func (m systemdManager) Restart(ctx context.Context, spec ServiceSpec, dryRun bool) (ServiceStatus, error) {
	return m.runSystemctl(ctx, spec, dryRun, StateRunning, "restart")
}

func (m systemdManager) runSystemctl(ctx context.Context, spec ServiceSpec, dryRun bool, desired, action string) (ServiceStatus, error) {
	status := baseStatus(spec, m.Name())
	status.DesiredState = desired
	if dryRun {
		return status, nil
	}
	result := m.runner.Run(ctx, "systemctl", "--user", action, spec.UnitName)
	if result.Err != nil {
		status.LastError = commandError(result)
		return status, result.Err
	}
	return m.Status(ctx, spec), nil
}

func (m systemdManager) Status(ctx context.Context, spec ServiceSpec) ServiceStatus {
	status := baseStatus(spec, m.Name())
	if _, err := os.Stat(spec.SystemdUnitPath); err == nil {
		status.Installed = true
	} else {
		status.ObservedState = StateNotInstalled
		return status
	}
	result := m.runner.Run(ctx, "systemctl", "--user", "show", spec.UnitName, "--property=ActiveState,SubState,MainPID")
	if result.Err != nil {
		status.ObservedState = StateUnknown
		status.LastError = commandError(result)
		return status
	}
	props := parseSystemdProps(result.Stdout)
	status.PID, _ = strconv.Atoi(props["MainPID"])
	switch props["ActiveState"] {
	case "active":
		status.ObservedState = StateRunning
	case "failed":
		status.ObservedState = StateFailed
	case "activating":
		status.ObservedState = StateStarting
	case "deactivating":
		status.ObservedState = StateStopping
	case "inactive":
		status.ObservedState = StateNotRunning
	default:
		status.ObservedState = StateUnknown
	}
	return status
}

type manualManager struct {
	platform PlatformInfo
}

func (m manualManager) Name() string { return ManagerManual }

func (m manualManager) Install(_ context.Context, spec ServiceSpec, _ bool) (ServiceStatus, error) {
	status := baseStatus(spec, m.Name())
	status.ObservedState = StateUnsupported
	status.Health = HealthUnknown
	status.LastError = firstNonEmpty(m.platform.UnsupportedNote, "manual service manager cannot install services")
	return status, nil
}

func (m manualManager) Uninstall(_ context.Context, spec ServiceSpec, _ bool) (ServiceStatus, error) {
	status := baseStatus(spec, m.Name())
	status.ObservedState = StateUnsupported
	status.LastError = firstNonEmpty(m.platform.UnsupportedNote, "manual service manager cannot uninstall services")
	return status, nil
}

func (m manualManager) Start(_ context.Context, spec ServiceSpec, _ bool) (ServiceStatus, error) {
	status := baseStatus(spec, m.Name())
	status.ObservedState = StateUnsupported
	status.LastError = firstNonEmpty(m.platform.UnsupportedNote, "manual service manager cannot start services")
	return status, nil
}

func (m manualManager) Stop(_ context.Context, spec ServiceSpec, _ bool) (ServiceStatus, error) {
	status := baseStatus(spec, m.Name())
	status.ObservedState = StateUnsupported
	status.LastError = firstNonEmpty(m.platform.UnsupportedNote, "manual service manager cannot stop services")
	return status, nil
}

func (m manualManager) Restart(_ context.Context, spec ServiceSpec, _ bool) (ServiceStatus, error) {
	status := baseStatus(spec, m.Name())
	status.ObservedState = StateUnsupported
	status.LastError = firstNonEmpty(m.platform.UnsupportedNote, "manual service manager cannot restart services")
	return status, nil
}

func (m manualManager) Status(_ context.Context, spec ServiceSpec) ServiceStatus {
	status := baseStatus(spec, m.Name())
	status.ObservedState = StateUnsupported
	status.LastError = firstNonEmpty(m.platform.UnsupportedNote, "manual service manager cannot observe user services")
	return status
}

func baseStatus(spec ServiceSpec, manager string) ServiceStatus {
	state := StateUnknown
	if !spec.Configured {
		state = StateNotConfigured
	}
	unitPath := spec.SystemdUnitPath
	if manager == ManagerLaunchd {
		unitPath = spec.LaunchAgentPath
	}
	return ServiceStatus{
		Name:          spec.Name,
		Configured:    spec.Configured,
		ObservedState: state,
		Health:        HealthUnknown,
		HealthURL:     spec.HealthURL,
		StdoutLog:     spec.StdoutLog,
		StderrLog:     spec.StderrLog,
		Manager:       manager,
		UnitPath:      unitPath,
		Label:         spec.Label,
		Binary:        spec.Binary,
		ConfigPath:    spec.ConfigPath,
		LastError:     spec.LastError,
	}
}

func launchdDomain() string {
	if u, err := user.Current(); err == nil && u.Uid != "" {
		return "gui/" + u.Uid
	}
	return "gui/0"
}

func launchdTarget(spec ServiceSpec) string {
	return launchdDomain() + "/" + spec.Label
}

func writeTextFile(path, value string, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".codencer-service-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.WriteString(value); err != nil {
		_ = tmp.Close()
		_ = os.Remove(name)
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		_ = os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	return os.Rename(name, path)
}

func commandError(result CommandResult) string {
	parts := []string{}
	if result.Err != nil {
		parts = append(parts, result.Err.Error())
	}
	if result.Stderr != "" {
		parts = append(parts, result.Stderr)
	}
	if result.Stdout != "" {
		parts = append(parts, result.Stdout)
	}
	return redact(strings.Join(parts, ": "))
}

func parsePID(output string) int {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "pid =") {
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				pid, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
				return pid
			}
		}
	}
	return 0
}

func parseSystemdProps(output string) map[string]string {
	props := map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok {
			props[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return props
}

func (s ServiceStatus) failed(strict bool) bool {
	if !strict && (s.ObservedState == StateNotConfigured || s.ObservedState == StateUnsupported || s.ObservedState == StateNotInstalled) {
		return false
	}
	return s.ObservedState == StateFailed || s.ObservedState == StateUnknown || s.Health == HealthError
}
