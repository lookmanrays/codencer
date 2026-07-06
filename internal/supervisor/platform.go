package supervisor

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func DetectPlatform(requested string, runner CommandRunner) PlatformInfo {
	info := PlatformInfo{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
		WSL:  detectWSL(),
	}
	if requested == "" {
		requested = ManagerAuto
	}
	if requested != ManagerAuto {
		info.ServiceManager = requested
		if requested == ManagerSystemd {
			info.SystemdUser = systemdUserAvailable(runner)
		}
		return info
	}

	switch runtime.GOOS {
	case "darwin":
		info.ServiceManager = ManagerLaunchd
	case "linux":
		if systemdUserAvailable(runner) {
			info.ServiceManager = ManagerSystemd
			info.SystemdUser = true
		} else {
			info.ServiceManager = ManagerManual
			if info.WSL {
				info.UnsupportedNote = "WSL systemd user manager is unavailable; use service render or foreground/manual commands."
			} else {
				info.UnsupportedNote = "systemd user manager is unavailable; use service render or manual commands."
			}
		}
	default:
		info.ServiceManager = ManagerManual
		info.UnsupportedNote = "user-level service manager is unsupported on this platform"
	}
	return info
}

func systemdUserAvailable(runner CommandRunner) bool {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return false
	}
	result := runnerOrDefault(runner).Run(context.Background(), "systemctl", "--user", "show-environment")
	return result.Err == nil
}

func detectWSL() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	if os.Getenv("WSL_DISTRO_NAME") != "" || os.Getenv("WSL_INTEROP") != "" {
		return true
	}
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), "microsoft")
}

func platformOrDetect(requested string, runner CommandRunner, override PlatformInfo) PlatformInfo {
	if override.OS != "" || override.ServiceManager != "" {
		if override.OS == "" {
			override.OS = runtime.GOOS
		}
		if override.Arch == "" {
			override.Arch = runtime.GOARCH
		}
		if override.ServiceManager == "" {
			override.ServiceManager = requested
			if override.ServiceManager == "" {
				override.ServiceManager = ManagerAuto
			}
		}
		if override.ServiceManager == ManagerAuto {
			detected := DetectPlatform(requested, runner)
			override.ServiceManager = detected.ServiceManager
			override.SystemdUser = detected.SystemdUser
			override.UnsupportedNote = detected.UnsupportedNote
		}
		return override
	}
	return DetectPlatform(requested, runner)
}
