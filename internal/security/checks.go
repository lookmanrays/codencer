package security

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"agent-bridge/internal/local"
	"agent-bridge/internal/profile"
	"agent-bridge/internal/project"
)

const (
	StatusPassed        = "passed"
	StatusFailed        = "failed"
	StatusSkipped       = "skipped"
	StatusNotConfigured = "not_configured"
)

type Check struct {
	ID            string   `json:"id"`
	Status        string   `json:"status"`
	Required      bool     `json:"required"`
	Reason        string   `json:"reason,omitempty"`
	ObservedFacts []string `json:"observed_facts,omitempty"`
}

type Options struct {
	Paths     local.Paths
	Config    local.Config
	Registry  *project.Registry
	RepoRoot  string
	RelayOnly bool
}

func RunChecks(opts Options) []Check {
	checks := []Check{
		checkExplicitProjectSharing(opts.Registry),
		checkDangerousProfileGuardrail(),
		checkDaemonLocalhost(opts.Config),
		checkLogPaths(opts.Paths),
		checkReportRedaction(opts.Paths),
		checkInstallScriptSafety("scripts/uninstall.sh"),
		checkOAuthMetadataNoSecrets(opts.Config),
	}
	checks = append(checks, checkRelayAuthConfig(opts.Config, opts.Paths))
	checks = append(checks, checkRemotePathSanitizer())
	return checks
}

func checkExplicitProjectSharing(registry *project.Registry) Check {
	if registry == nil || len(registry.Projects) == 0 {
		return Check{ID: "security_project_sharing_explicit", Status: StatusSkipped, Required: true, Reason: "no projects registered"}
	}
	for _, p := range registry.Projects {
		if p.SharedToRelay {
			return Check{ID: "security_project_sharing_explicit", Status: StatusPassed, Required: true, ObservedFacts: []string{"shared project ids are explicit registry fields"}}
		}
	}
	return Check{ID: "security_project_sharing_explicit", Status: StatusPassed, Required: true, ObservedFacts: []string{"no projects are shared by default"}}
}

func checkDangerousProfileGuardrail() Check {
	_, err := profile.Resolve(profile.ResolveOptions{ProfileID: "codex-danger-bypass", Adapter: "codex"})
	if err == nil {
		return Check{ID: "security_dangerous_bypass_guardrail", Status: StatusFailed, Required: true, Reason: "codex-danger-bypass resolved without explicit allow env"}
	}
	return Check{ID: "security_dangerous_bypass_guardrail", Status: StatusPassed, Required: true}
}

func checkDaemonLocalhost(cfg local.Config) Check {
	u, err := url.Parse(cfg.DefaultDaemonURL)
	if err != nil {
		return Check{ID: "security_daemon_localhost", Status: StatusFailed, Required: true, Reason: err.Error()}
	}
	host := u.Hostname()
	if host == "127.0.0.1" || host == "localhost" || host == "::1" {
		return Check{ID: "security_daemon_localhost", Status: StatusPassed, Required: true, ObservedFacts: []string{cfg.DefaultDaemonURL}}
	}
	return Check{ID: "security_daemon_localhost", Status: StatusFailed, Required: true, Reason: "default daemon URL is not loopback"}
}

func checkLogPaths(paths local.Paths) Check {
	if paths.LogsDir == "" || paths.Home == "" {
		return Check{ID: "security_log_paths_controlled", Status: StatusFailed, Required: true, Reason: "paths are incomplete"}
	}
	rel, err := filepath.Rel(paths.Home, paths.LogsDir)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return Check{ID: "security_log_paths_controlled", Status: StatusFailed, Required: true, Reason: "logs dir is outside CODENCER_HOME"}
	}
	return Check{ID: "security_log_paths_controlled", Status: StatusPassed, Required: true, ObservedFacts: []string{"logs_dir=" + rel}}
}

func checkReportRedaction(paths local.Paths) Check {
	for _, dir := range []string{"acceptance", "live-matrix", "readiness", "proof-bundles"} {
		root := filepath.Join(paths.ArtifactsDir, dir)
		leaked := false
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d == nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr == nil && ContainsObviousSecret(string(data)) {
				leaked = true
			}
			return nil
		})
		if leaked {
			return Check{ID: "security_reports_redacted", Status: StatusFailed, Required: true, Reason: "obvious secret marker found in generated reports"}
		}
	}
	return Check{ID: "security_reports_redacted", Status: StatusPassed, Required: true}
}

func checkInstallScriptSafety(path string) Check {
	data, err := os.ReadFile(path)
	if err != nil {
		return Check{ID: "security_uninstall_purge_guard", Status: StatusNotConfigured, Required: false, Reason: "uninstall script not present yet"}
	}
	text := string(data)
	if strings.Contains(text, "--purge") && strings.Contains(text, "PURGE=1") {
		return Check{ID: "security_uninstall_purge_guard", Status: StatusPassed, Required: true}
	}
	return Check{ID: "security_uninstall_purge_guard", Status: StatusFailed, Required: true, Reason: "uninstall script must gate data removal behind --purge"}
}

func checkOAuthMetadataNoSecrets(cfg local.Config) Check {
	if cfg.RelayConfigPath == "" {
		return Check{ID: "security_oauth_metadata_no_secrets", Status: StatusSkipped, Required: true, Reason: "relay config is not configured"}
	}
	data, err := os.ReadFile(cfg.RelayConfigPath)
	if err != nil {
		return Check{ID: "security_oauth_metadata_no_secrets", Status: StatusNotConfigured, Required: true, Reason: err.Error()}
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return Check{ID: "security_oauth_metadata_no_secrets", Status: StatusFailed, Required: true, Reason: err.Error()}
	}
	delete(raw, "planner_token")
	delete(raw, "planner_tokens")
	delete(raw, "enrollment_secret")
	encoded, _ := json.Marshal(raw)
	if ContainsObviousSecret(string(encoded)) {
		return Check{ID: "security_oauth_metadata_no_secrets", Status: StatusFailed, Required: true, Reason: "non-auth relay metadata contains secret-like value"}
	}
	return Check{ID: "security_oauth_metadata_no_secrets", Status: StatusPassed, Required: true}
}

func checkRelayAuthConfig(cfg local.Config, paths local.Paths) Check {
	if cfg.RelayConfigPath == "" {
		return Check{ID: "security_relay_auth_required", Status: StatusSkipped, Required: true, Reason: "relay config is not configured"}
	}
	data, err := os.ReadFile(cfg.RelayConfigPath)
	if err != nil {
		return Check{ID: "security_relay_auth_required", Status: StatusNotConfigured, Required: true, Reason: err.Error()}
	}
	var raw struct {
		PlannerToken  string `json:"planner_token"`
		PlannerTokens []struct {
			Token  string   `json:"token"`
			Scopes []string `json:"scopes"`
		} `json:"planner_tokens"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return Check{ID: "security_relay_auth_required", Status: StatusFailed, Required: true, Reason: err.Error()}
	}
	if raw.PlannerToken != "" || len(raw.PlannerTokens) > 0 {
		return Check{ID: "security_relay_auth_required", Status: StatusPassed, Required: true, ObservedFacts: []string{"relay config has planner auth"}}
	}
	_ = paths
	return Check{ID: "security_relay_auth_required", Status: StatusFailed, Required: true, Reason: "relay config has no planner token"}
}

func checkRemotePathSanitizer() Check {
	sample := []byte(`{"project":{"repo_root":"/Users/example/private/repo","project_config_path":"/Users/example/private/repo/.codencer/project.json","allowed_paths":["."],"forbidden_paths":[".env"],"daemon_url":"http://127.0.0.1:8085"}}`)
	out := string(SanitizeRemoteJSON(sample))
	if strings.Contains(out, "/Users/example") || strings.Contains(out, "project_config_path") || strings.Contains(out, "allowed_paths") || strings.Contains(out, "forbidden_paths") || strings.Contains(out, "daemon_url") {
		return Check{ID: "security_remote_path_sanitizer", Status: StatusFailed, Required: true, Reason: "sanitizer leaked local path fields"}
	}
	return Check{ID: "security_remote_path_sanitizer", Status: StatusPassed, Required: true}
}
