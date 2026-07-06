package supervisor

import (
	"net/http"
	"time"

	"agent-bridge/internal/local"
)

const (
	ServiceDaemon    = "daemon"
	ServiceRelay     = "relay"
	ServiceConnector = "connector"

	ManagerAuto    = "auto"
	ManagerLaunchd = "launchd"
	ManagerSystemd = "systemd"
	ManagerManual  = "manual"

	StateInstalled     = "installed"
	StateNotInstalled  = "not_installed"
	StateRunning       = "running"
	StateNotRunning    = "not_running"
	StateStarting      = "starting"
	StateStopping      = "stopping"
	StateFailed        = "failed"
	StateNotConfigured = "not_configured"
	StateUnsupported   = "unsupported"
	StateUnknown       = "unknown"

	HealthOK            = "ok"
	HealthNotRunning    = "not_running"
	HealthNotConfigured = "not_configured"
	HealthUnknown       = "unknown"
	HealthError         = "error"

	ProblemDaemonNotRunning     = "daemon_not_running"
	ProblemConnectorOffline     = "connector_offline"
	ProblemRelayUnreachable     = "relay_unreachable"
	ProblemStaleRun             = "stale_run"
	ProblemStaleStep            = "stale_step"
	ProblemStaleLock            = "stale_lock"
	ProblemExecutorMissing      = "executor_missing"
	ProblemArtifactMissing      = "artifact_missing"
	ProblemValidationIncomplete = "validation_incomplete"
	ProblemUnknownRuntimeState  = "unknown_runtime_state"
)

type Options struct {
	Service    string
	All        bool
	ProjectID  string
	RepoRoot   string
	ConfigPath string
	Manager    string
	BinDir     string
	DryRun     bool
	Strict     bool
	Format     string
	Tail       int
	Follow     bool
	HTTPClient *http.Client
	Runner     CommandRunner
	Platform   PlatformInfo
	Now        func() time.Time
}

type ServiceSpec struct {
	Name            string            `json:"name"`
	Binary          string            `json:"binary,omitempty"`
	Args            []string          `json:"args,omitempty"`
	Env             map[string]string `json:"env,omitempty"`
	WorkingDir      string            `json:"working_dir,omitempty"`
	StdoutLog       string            `json:"stdout_log,omitempty"`
	StderrLog       string            `json:"stderr_log,omitempty"`
	HealthURL       string            `json:"health_url,omitempty"`
	ConfigPath      string            `json:"config_path,omitempty"`
	Configured      bool              `json:"configured"`
	Dependencies    []string          `json:"dependencies,omitempty"`
	Label           string            `json:"label,omitempty"`
	UnitName        string            `json:"unit_name,omitempty"`
	LaunchAgentPath string            `json:"launch_agent_path,omitempty"`
	SystemdUnitPath string            `json:"systemd_unit_path,omitempty"`
	LastError       string            `json:"last_error,omitempty"`
}

type PlatformInfo struct {
	OS              string `json:"os"`
	Arch            string `json:"arch"`
	WSL             bool   `json:"wsl"`
	ServiceManager  string `json:"service_manager"`
	SystemdUser     bool   `json:"systemd_user_available,omitempty"`
	UnsupportedNote string `json:"unsupported_note,omitempty"`
}

type ServiceStatus struct {
	Name          string `json:"name"`
	Installed     bool   `json:"installed"`
	Configured    bool   `json:"configured"`
	DesiredState  string `json:"desired_state"`
	ObservedState string `json:"observed_state"`
	Health        string `json:"health"`
	PID           int    `json:"pid,omitempty"`
	HealthURL     string `json:"health_url,omitempty"`
	StdoutLog     string `json:"stdout_log,omitempty"`
	StderrLog     string `json:"stderr_log,omitempty"`
	Manager       string `json:"manager"`
	UnitPath      string `json:"unit_path,omitempty"`
	Label         string `json:"label,omitempty"`
	Binary        string `json:"binary,omitempty"`
	ConfigPath    string `json:"config_path,omitempty"`
	LastError     string `json:"last_error,omitempty"`
	Rendered      string `json:"rendered,omitempty"`
}

type ServiceReport struct {
	OK       bool            `json:"ok"`
	Action   string          `json:"action"`
	DryRun   bool            `json:"dry_run,omitempty"`
	Platform PlatformInfo    `json:"platform"`
	Services []ServiceStatus `json:"services"`
	Warnings []string        `json:"warnings,omitempty"`
	ExitCode int             `json:"exit_code"`
}

type Check struct {
	Name        string   `json:"name"`
	Status      string   `json:"status"`
	BlockerType string   `json:"blocker_type,omitempty"`
	Message     string   `json:"message"`
	Observed    []string `json:"observed_facts,omitempty"`
}

type RuntimeBlocker struct {
	Type                    string   `json:"type"`
	Message                 string   `json:"message"`
	PlannerDecisionRequired bool     `json:"planner_decision_required"`
	ObservedFacts           []string `json:"observed_facts,omitempty"`
	OperatorCommand         string   `json:"operator_command,omitempty"`
}

type WatchdogReport struct {
	OK          bool              `json:"ok"`
	Platform    PlatformInfo      `json:"platform"`
	Environment local.Environment `json:"environment"`
	Checks      []Check           `json:"checks"`
	Blockers    []RuntimeBlocker  `json:"blockers,omitempty"`
	ExitCode    int               `json:"exit_code"`
}

type RecoveryAction struct {
	Type   string   `json:"type"`
	Target string   `json:"target"`
	Safe   bool     `json:"safe"`
	Reason string   `json:"reason"`
	Done   bool     `json:"done"`
	Facts  []string `json:"observed_facts,omitempty"`
}

type RecoveryReport struct {
	OK       bool             `json:"ok"`
	DryRun   bool             `json:"dry_run"`
	Actions  []RecoveryAction `json:"actions"`
	Blockers []RuntimeBlocker `json:"blockers,omitempty"`
	ExitCode int              `json:"exit_code"`
}
