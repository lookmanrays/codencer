package readiness

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	"agent-bridge/internal/live"
	"agent-bridge/internal/local"
)

const (
	VerdictReady          = "ready"
	VerdictReadyWithSkips = "ready_with_skips"
	VerdictNotReady       = "not_ready"
)

type Options struct {
	Local        bool
	Relay        bool
	Live         bool
	CodencerHome string
	RepoRoot     string
	Now          func() time.Time
}

type Evidence struct {
	Reports []string `json:"reports"`
}

type Report struct {
	OK               bool              `json:"ok"`
	Verdict          string            `json:"verdict"`
	Profiles         map[string]string `json:"profiles"`
	BlockingGates    []string          `json:"blocking_gates"`
	SkippedLiveGates []string          `json:"skipped_live_gates"`
	Evidence         Evidence          `json:"evidence"`
	ReportPath       string            `json:"report_path,omitempty"`
}

func Build(ctx context.Context, opts Options) (Report, error) {
	repo := strings.TrimSpace(opts.RepoRoot)
	if repo == "" {
		wd, err := os.Getwd()
		if err != nil {
			return Report{}, err
		}
		repo = wd
	}
	paths, err := local.ResolvePathsForHome(repo, "", opts.CodencerHome)
	if err != nil {
		return Report{}, err
	}
	report := Report{
		OK:       true,
		Verdict:  VerdictReady,
		Profiles: map[string]string{},
	}
	if _, err := local.EnsureHome(paths, now(opts)); err != nil {
		report.Profiles["local_cli"] = "not_ready"
		report.BlockingGates = append(report.BlockingGates, "codencer_home")
	} else {
		report.Profiles["local_cli"] = "ready"
	}

	cfg, err := local.LoadConfig(paths.ConfigFile)
	if err != nil {
		report.Profiles["local_cli"] = "not_ready"
		report.BlockingGates = append(report.BlockingGates, "local_config")
	} else {
		status := local.BuildStatusReport(local.StatusOptions{Paths: paths, Config: cfg, RepoRoot: repo})
		if status.ProjectCount == 0 {
			report.Profiles["local_cli"] = "ready_with_skips"
			report.SkippedLiveGates = append(report.SkippedLiveGates, "project_registry_empty")
		} else if status.Status == local.RuntimeError {
			report.Profiles["local_cli"] = "not_ready"
			report.BlockingGates = append(report.BlockingGates, "local_cli_status")
		}
		if opts.Relay {
			relayReady := false
			if status.Project != nil && status.Project.SharedToRelay && cfg.RelayConfigPath != "" && cfg.ConnectorConfigPath != "" {
				relayReady = true
			}
			if relayReady {
				report.Profiles["relay_mcp"] = "ready"
			} else {
				report.Profiles["relay_mcp"] = "not_ready"
				report.BlockingGates = append(report.BlockingGates, "relay_mcp_config")
			}
		} else {
			report.Profiles["relay_mcp"] = "skipped"
			report.SkippedLiveGates = append(report.SkippedLiveGates, "relay_mcp")
		}
	}

	report.Profiles["codex_live"] = "skipped"
	report.Profiles["claude_live"] = "skipped"
	report.Profiles["wsl"] = "skipped"
	report.SkippedLiveGates = append(report.SkippedLiveGates, "codex_live_execution", "claude_live_execution", "wsl")
	if opts.Live {
		matrix, err := live.Matrix(ctx, live.Options{Profile: "all", CodencerHome: opts.CodencerHome, RepoRoot: repo})
		if err != nil {
			report.BlockingGates = append(report.BlockingGates, "live_matrix")
		} else {
			report.Evidence.Reports = append(report.Evidence.Reports, matrix.ReportPath)
			updateLiveProfile(report.Profiles, "codex_live", matrix, "codex_live_execution")
			updateLiveProfile(report.Profiles, "claude_live", matrix, "claude_live_execution")
			updateLiveProfile(report.Profiles, "relay_mcp", matrix, "relay_mcp_live")
			updateLiveProfile(report.Profiles, "wsl", matrix, "wsl_environment")
			for _, check := range matrix.Checks {
				if check.Status == live.StatusFailed || check.Status == live.StatusBlocked {
					report.BlockingGates = append(report.BlockingGates, check.ID)
				}
			}
		}
	}

	if len(report.BlockingGates) > 0 {
		report.OK = false
		report.Verdict = VerdictNotReady
	} else if hasSkippedOrReadyWithSkips(report) {
		report.OK = true
		report.Verdict = VerdictReadyWithSkips
	} else {
		report.OK = true
		report.Verdict = VerdictReady
	}
	path, err := live.PersistJSON(paths, "readiness", report)
	if err != nil {
		return report, err
	}
	report.ReportPath = path
	_ = overwriteReadinessReport(path, report)
	return report, nil
}

func updateLiveProfile(profiles map[string]string, key string, matrix live.Report, checkID string) {
	for _, check := range matrix.Checks {
		if check.ID != checkID {
			continue
		}
		switch check.Status {
		case live.StatusPassed:
			profiles[key] = "ready"
		case live.StatusSkipped, live.StatusUnsupported, live.StatusNotConfigured:
			profiles[key] = "skipped"
		default:
			profiles[key] = "not_ready"
		}
		return
	}
}

func hasSkippedOrReadyWithSkips(report Report) bool {
	if len(report.SkippedLiveGates) > 0 {
		return true
	}
	for _, value := range report.Profiles {
		if value == "skipped" || value == "ready_with_skips" {
			return true
		}
	}
	return false
}

func now(opts Options) time.Time {
	if opts.Now != nil {
		return opts.Now().UTC()
	}
	return time.Now().UTC()
}

func overwriteReadinessReport(path string, report Report) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0600)
}
