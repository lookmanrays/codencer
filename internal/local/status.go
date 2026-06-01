package local

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent-bridge/internal/project"
)

const (
	RuntimeOK            = "ok"
	RuntimeNotRunning    = "not_running"
	RuntimeNotConfigured = "not_configured"
	RuntimeUnknown       = "unknown"
	RuntimeError         = "error"
)

type RuntimeStatus struct {
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
	URL    string `json:"url,omitempty"`
}

type ExecutorStatus struct {
	ID     string `json:"id"`
	Binary string `json:"binary"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type StatusReport struct {
	Status            string           `json:"status"`
	Paths             Paths            `json:"paths"`
	ProjectCount      int              `json:"project_count"`
	CurrentProjectID  string           `json:"current_project_id,omitempty"`
	ProjectResolution string           `json:"project_resolution,omitempty"`
	Project           *project.Project `json:"project,omitempty"`
	Daemon            RuntimeStatus    `json:"daemon"`
	Relay             RuntimeStatus    `json:"relay"`
	Executors         []ExecutorStatus `json:"executors"`
	Environment       Environment      `json:"environment"`
	Warnings          []string         `json:"warnings,omitempty"`
}

type StatusOptions struct {
	Paths      Paths
	Config     Config
	ProjectID  string
	RepoRoot   string
	Probe      ProbeFunc
	HTTPClient *http.Client
}

func BuildStatusReport(opts StatusOptions) StatusReport {
	probe := opts.Probe
	if probe == nil {
		probe = DefaultProbe
	}
	report := StatusReport{
		Status:      RuntimeOK,
		Paths:       opts.Paths,
		Environment: DetectEnvironment(),
		Executors: []ExecutorStatus{
			executorStatus(probe, "codex", "CODEX_BINARY", "codex"),
			executorStatus(probe, "claude", "CLAUDE_BINARY", "claude"),
		},
	}

	registry, err := project.LoadRegistry(opts.Paths.ProjectsFile)
	if err != nil {
		report.Status = RuntimeError
		report.Warnings = append(report.Warnings, err.Error())
		report.Daemon = RuntimeStatus{Status: RuntimeUnknown, Detail: "project registry unavailable"}
		report.Relay = RuntimeStatus{Status: RuntimeUnknown, Detail: "project registry unavailable"}
		return report
	}
	report.ProjectCount = len(registry.Projects)
	report.CurrentProjectID = registry.CurrentProjectID

	if len(registry.Projects) > 0 || strings.TrimSpace(opts.ProjectID) != "" {
		resolve, err := project.ResolveProject(registry, project.ResolveOptions{ExplicitID: opts.ProjectID, CWD: opts.RepoRoot})
		if err != nil {
			report.Status = RuntimeError
			report.Warnings = append(report.Warnings, err.Error())
		} else {
			report.Project = &resolve.Project
			report.ProjectResolution = resolve.Source
		}
	} else {
		report.Status = RuntimeNotConfigured
		report.Warnings = append(report.Warnings, "no projects registered")
	}

	daemonURL := opts.Config.DefaultDaemonURL
	if report.Project != nil && report.Project.DaemonURL != "" {
		daemonURL = report.Project.DaemonURL
	}
	report.Daemon = CheckDaemon(daemonURL, opts.HTTPClient)
	report.Relay = relayStatus(report.Project, opts.Config)
	if report.Daemon.Status == RuntimeError {
		report.Status = RuntimeError
	} else if report.Status == RuntimeOK && report.Daemon.Status == RuntimeNotRunning {
		report.Status = RuntimeNotRunning
	}
	return report
}

func CheckDaemon(baseURL string, client *http.Client) RuntimeStatus {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return RuntimeStatus{Status: RuntimeNotConfigured, Detail: "daemon URL not configured"}
	}
	if client == nil {
		client = &http.Client{Timeout: 1200 * time.Millisecond}
	}
	for _, endpoint := range []string{"/health", "/api/v1/instance"} {
		req, err := http.NewRequest(http.MethodGet, baseURL+endpoint, nil)
		if err != nil {
			return RuntimeStatus{Status: RuntimeError, URL: baseURL, Detail: err.Error()}
		}
		resp, err := client.Do(req)
		if err != nil {
			return RuntimeStatus{Status: RuntimeNotRunning, URL: baseURL, Detail: err.Error()}
		}
		func() {
			defer resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				var payload map[string]any
				if decodeErr := json.NewDecoder(resp.Body).Decode(&payload); decodeErr == nil {
					if id, ok := payload["id"].(string); ok && id != "" {
						resp.Header.Set("X-Codencer-Detail", "instance "+id)
					}
				}
			}
		}()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			detail := resp.Header.Get("X-Codencer-Detail")
			if detail == "" {
				detail = endpoint + " returned " + resp.Status
			}
			return RuntimeStatus{Status: RuntimeOK, URL: baseURL, Detail: detail}
		}
	}
	return RuntimeStatus{Status: RuntimeNotRunning, URL: baseURL, Detail: "daemon did not return a healthy response"}
}

func executorStatus(probe ProbeFunc, id, envName, fallback string) ExecutorStatus {
	binary := strings.TrimSpace(os.Getenv(envName))
	if binary == "" {
		binary = fallback
	}
	result := probe(binary, "--version")
	if result.Err != nil {
		result = probe(binary, "version")
	}
	if result.Err != nil {
		return ExecutorStatus{ID: id, Binary: binary, Status: RuntimeNotConfigured, Detail: result.Err.Error()}
	}
	return ExecutorStatus{ID: id, Binary: binary, Status: RuntimeOK, Detail: firstLine(result.Output, result.Path)}
}

func relayStatus(p *project.Project, cfg Config) RuntimeStatus {
	if p == nil {
		return RuntimeStatus{Status: RuntimeNotConfigured, Detail: "no project resolved"}
	}
	detail := fmt.Sprintf("shared_to_relay=%t", p.SharedToRelay)
	if p.RelayInstanceID != "" {
		detail += " relay_instance_id=" + p.RelayInstanceID
	}
	if cfg.RelayConfigPath != "" {
		if _, err := os.Stat(cfg.RelayConfigPath); err != nil {
			return RuntimeStatus{Status: RuntimeNotRunning, Detail: detail + "; relay config not readable: " + err.Error()}
		}
		return RuntimeStatus{Status: RuntimeUnknown, Detail: detail + "; relay config present at " + filepath.Clean(cfg.RelayConfigPath)}
	}
	return RuntimeStatus{Status: RuntimeNotConfigured, Detail: detail + "; relay config not configured"}
}
