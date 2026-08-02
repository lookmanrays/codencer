package local

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"agent-bridge/internal/project"
)

const (
	CheckOK      = "ok"
	CheckWarning = "warning"
	CheckError   = "error"
	CheckSkipped = "skipped"
	CheckUnknown = "unknown"
)

type Check struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

type Summary struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
	Skipped  int `json:"skipped"`
	Unknown  int `json:"unknown"`
}

type DoctorReport struct {
	OK          bool        `json:"ok"`
	Checks      []Check     `json:"checks"`
	Summary     Summary     `json:"summary"`
	Environment Environment `json:"environment"`
}

type ProbeResult struct {
	Path   string
	Output string
	Err    error
}

type ProbeFunc func(command string, args ...string) ProbeResult

type DoctorOptions struct {
	Paths         Paths
	Config        Config
	RepoRoot      string
	ProjectID     string
	ToolchainOnly bool
	Strict        bool
	Probe         ProbeFunc
	HTTPClient    *http.Client
}

func BuildDoctorReport(opts DoctorOptions) DoctorReport {
	probe := opts.Probe
	if probe == nil {
		probe = DefaultProbe
	}
	checks := []Check{
		{Name: "os", Status: CheckOK, Detail: fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)},
	}
	if DetectEnvironment().WSL {
		checks = append(checks, Check{Name: "wsl", Status: CheckOK, Detail: "running inside WSL"})
	} else {
		checks = append(checks, Check{Name: "wsl", Status: CheckSkipped, Detail: "not running inside WSL"})
	}
	checks = append(checks,
		commandCheck(probe, "go", "go", []string{"version"}, true),
		commandCheck(probe, "git", "git", []string{"--version"}, true),
		commandCheck(probe, "c_compiler", "cc", []string{"--version"}, true),
		commandCheck(probe, "curl", "curl", []string{"--version"}, true),
		goModuleCheck(opts.RepoRoot),
		cgoCheck(),
	)

	if opts.ToolchainOnly {
		return aggregate(checks, opts.Strict)
	}

	checks = append(checks,
		homeCheck(opts.Paths.Home),
		registryCheck(opts.Paths.ProjectsFile, opts.ProjectID),
		lowLevelBinaryCheck("orchestratord", "cmd/orchestratord", "bin/orchestratord", opts.RepoRoot),
		lowLevelBinaryCheck("orchestratorctl", "cmd/orchestratorctl", "bin/orchestratorctl", opts.RepoRoot),
		lowLevelBinaryCheck("codencer-relayd", "cmd/codencer-relayd", "bin/codencer-relayd", opts.RepoRoot),
		lowLevelBinaryCheck("codencer-connectord", "cmd/codencer-connectord", "bin/codencer-connectord", opts.RepoRoot),
		agentBinaryCheck(probe, "codex_cli", "CODEX_BINARY", "codex"),
		agentBinaryCheck(probe, "claude_cli", "CLAUDE_BINARY", "claude"),
		agentBinaryCheck(probe, "opencode_cli", "OPENCODE_BINARY", "opencode"),
		relayConnectorPresenceCheck(opts.RepoRoot, opts.Config),
		daemonHealthCheck(opts.Config.DefaultDaemonURL, opts.HTTPClient),
	)
	return aggregate(checks, opts.Strict)
}

func DefaultProbe(command string, args ...string) ProbeResult {
	path := command
	if !filepath.IsAbs(command) {
		resolved, err := exec.LookPath(command)
		if err != nil {
			return ProbeResult{Err: err}
		}
		path = resolved
	} else if _, err := os.Stat(command); err != nil {
		return ProbeResult{Err: err}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, args...)
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return ProbeResult{Path: path, Output: strings.TrimSpace(string(output)), Err: ctx.Err()}
	}
	return ProbeResult{Path: path, Output: strings.TrimSpace(string(output)), Err: err}
}

func commandCheck(probe ProbeFunc, name, command string, args []string, required bool) Check {
	result := probe(command, args...)
	if result.Err != nil {
		status := CheckWarning
		if required {
			status = CheckError
		}
		return Check{Name: name, Status: status, Detail: result.Err.Error()}
	}
	return Check{Name: name, Status: CheckOK, Detail: firstLine(result.Output, result.Path)}
}

func goModuleCheck(repoRoot string) Check {
	if repoRoot == "" {
		return Check{Name: "go_module", Status: CheckSkipped, Detail: "repo root not provided"}
	}
	data, err := os.ReadFile(filepath.Join(repoRoot, "go.mod"))
	if err != nil {
		return Check{Name: "go_module", Status: CheckWarning, Detail: fmt.Sprintf("go.mod not readable: %v", err)}
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "go" {
			return Check{Name: "go_module", Status: CheckOK, Detail: "go " + fields[1]}
		}
	}
	return Check{Name: "go_module", Status: CheckUnknown, Detail: "go directive not found"}
}

func cgoCheck() Check {
	value := os.Getenv("CGO_ENABLED")
	if value == "" {
		return Check{Name: "cgo", Status: CheckUnknown, Detail: "CGO_ENABLED not set; Go default applies"}
	}
	if value == "1" {
		return Check{Name: "cgo", Status: CheckOK, Detail: "CGO_ENABLED=1"}
	}
	return Check{Name: "cgo", Status: CheckWarning, Detail: "CGO_ENABLED=" + value + " may prevent sqlite builds"}
}

func homeCheck(home string) Check {
	info, err := os.Stat(home)
	if err != nil {
		if os.IsNotExist(err) {
			parent := filepath.Dir(home)
			if err := canWriteDir(parent); err != nil {
				return Check{Name: "codencer_home", Status: CheckError, Detail: fmt.Sprintf("home missing and parent is not writable: %v", err)}
			}
			return Check{Name: "codencer_home", Status: CheckWarning, Detail: "home not initialized; run codencer init"}
		}
		return Check{Name: "codencer_home", Status: CheckError, Detail: err.Error()}
	}
	if !info.IsDir() {
		return Check{Name: "codencer_home", Status: CheckError, Detail: "home path is not a directory"}
	}
	if err := canWriteDir(home); err != nil {
		return Check{Name: "codencer_home", Status: CheckError, Detail: err.Error()}
	}
	return Check{Name: "codencer_home", Status: CheckOK, Detail: home}
}

func registryCheck(path, explicitProjectID string) Check {
	parent := filepath.Dir(path)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			if _, parentErr := os.Stat(parent); parentErr != nil {
				return Check{Name: "project_registry", Status: CheckWarning, Detail: "registry not initialized; run codencer init"}
			}
			if err := canWriteDir(parent); err != nil {
				return Check{Name: "project_registry", Status: CheckError, Detail: fmt.Sprintf("registry parent is not writable: %v", err)}
			}
		} else {
			return Check{Name: "project_registry", Status: CheckError, Detail: err.Error()}
		}
	} else if err := canWriteDir(parent); err != nil {
		return Check{Name: "project_registry", Status: CheckError, Detail: fmt.Sprintf("registry parent is not writable: %v", err)}
	}
	registry, err := project.LoadRegistry(path)
	if err != nil {
		return Check{Name: "project_registry", Status: CheckError, Detail: err.Error()}
	}
	if len(registry.Projects) == 0 {
		return Check{Name: "project_registry", Status: CheckWarning, Detail: "no projects registered"}
	}
	resolve, err := project.ResolveProject(registry, project.ResolveOptions{ExplicitID: explicitProjectID})
	if err != nil {
		return Check{Name: "project_registry", Status: CheckWarning, Detail: err.Error()}
	}
	return Check{Name: "project_registry", Status: CheckOK, Detail: fmt.Sprintf("%d project(s), current resolution=%s:%s", len(registry.Projects), resolve.Source, resolve.Project.ID)}
}

func lowLevelBinaryCheck(name, sourceDir, binPath, repoRoot string) Check {
	if repoRoot == "" {
		return Check{Name: name, Status: CheckSkipped, Detail: "repo root not provided"}
	}
	sourcePath := filepath.Join(repoRoot, sourceDir)
	if info, err := os.Stat(sourcePath); err != nil || !info.IsDir() {
		return Check{Name: name, Status: CheckError, Detail: fmt.Sprintf("source missing at %s", sourcePath)}
	}
	fullBinPath := filepath.Join(repoRoot, binPath)
	if _, err := os.Stat(fullBinPath); err == nil {
		return Check{Name: name, Status: CheckOK, Detail: fmt.Sprintf("source present; built binary at %s", fullBinPath)}
	}
	return Check{Name: name, Status: CheckWarning, Detail: "source present; binary not built"}
}

func agentBinaryCheck(probe ProbeFunc, name, envName, fallback string) Check {
	binary := strings.TrimSpace(os.Getenv(envName))
	if binary == "" {
		binary = fallback
	}
	result := probe(binary, "--version")
	if result.Err != nil {
		result = probe(binary, "version")
	}
	if result.Err != nil {
		return Check{Name: name, Status: CheckWarning, Detail: fmt.Sprintf("%s not available via %s or PATH: %v", fallback, envName, result.Err)}
	}
	return Check{Name: name, Status: CheckOK, Detail: firstLine(result.Output, result.Path)}
}

func relayConnectorPresenceCheck(repoRoot string, cfg Config) Check {
	details := []string{}
	status := CheckOK
	if cfg.RelayConfigPath == "" {
		status = CheckSkipped
		details = append(details, "relay config not configured")
	} else if _, err := os.Stat(cfg.RelayConfigPath); err != nil {
		status = CheckWarning
		details = append(details, "relay config not readable: "+err.Error())
	} else {
		details = append(details, "relay config present")
	}
	if cfg.ConnectorConfigPath == "" {
		details = append(details, "connector config not configured")
	} else if _, err := os.Stat(cfg.ConnectorConfigPath); err != nil {
		if status == CheckOK {
			status = CheckWarning
		}
		details = append(details, "connector config not readable: "+err.Error())
	} else {
		details = append(details, "connector config present")
	}
	if repoRoot != "" {
		for _, rel := range []string{"cmd/codencer-relayd", "cmd/codencer-connectord"} {
			if _, err := os.Stat(filepath.Join(repoRoot, rel)); err != nil {
				status = CheckWarning
				details = append(details, rel+" source missing")
			}
		}
	}
	return Check{Name: "relay_connector", Status: status, Detail: strings.Join(details, "; ")}
}

func daemonHealthCheck(baseURL string, client *http.Client) Check {
	status := CheckDaemon(baseURL, client)
	return Check{Name: "daemon", Status: checkStatusForDaemon(status.Status), Detail: status.Detail}
}

func checkStatusForDaemon(status string) string {
	switch status {
	case RuntimeOK:
		return CheckOK
	case RuntimeNotConfigured:
		return CheckSkipped
	case RuntimeNotRunning:
		return CheckWarning
	case RuntimeError:
		return CheckError
	default:
		return CheckUnknown
	}
}

func aggregate(checks []Check, strict bool) DoctorReport {
	summary := Summary{}
	for _, check := range checks {
		switch check.Status {
		case CheckError:
			summary.Errors++
		case CheckWarning:
			summary.Warnings++
		case CheckSkipped:
			summary.Skipped++
		case CheckUnknown:
			summary.Unknown++
		}
	}
	ok := summary.Errors == 0
	if strict && summary.Warnings > 0 {
		ok = false
	}
	return DoctorReport{
		OK:          ok,
		Checks:      checks,
		Summary:     summary,
		Environment: DetectEnvironment(),
	}
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

func firstLine(output, fallback string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return fallback
	}
	line := strings.Split(output, "\n")[0]
	if len(line) > 200 {
		line = line[:200]
	}
	return line
}
