package localexec

import (
	"agent-bridge/internal/domain"
	profilepkg "agent-bridge/internal/profile"
	"agent-bridge/internal/project"
)

const (
	ExitSuccess          = 0
	ExitBlocked          = 10
	ExitFailedTerminal   = 20
	ExitValidationFailed = 21
	ExitAdapterFailed    = 22
	ExitDaemonFailed     = 23
	ExitTimeout          = 24
	ExitInvalidInput     = 30
	ExitInternal         = 40
)

const (
	BlockerValidationFailed      = "validation_failed"
	BlockerManualApproval        = "manual_approval_required"
	BlockerQuestion              = "question"
	BlockerUnknown               = "unknown"
	BlockerAdapterError          = "adapter_error"
	BlockerBridgeError           = "bridge_error"
	BlockerDaemonNotRunning      = "daemon_not_running"
	BlockerTimeout               = "timeout"
	BlockerFailedTerminal        = "failed_terminal"
	BlockerUnsafeAction          = "unsafe_action"
	BlockerInvalidInput          = "invalid_input"
	BlockerConfigurationRequired = "configuration_required"
)

type Blocker struct {
	Type                 string   `json:"type"`
	Message              string   `json:"message"`
	NeedsPlannerDecision bool     `json:"needs_planner_decision"`
	Retryable            bool     `json:"retryable,omitempty"`
	Questions            []string `json:"questions,omitempty"`
	ObservedFacts        []string `json:"observed_facts,omitempty"`
	EvidenceRefs         []string `json:"evidence_refs,omitempty"`
}

type Evidence struct {
	Result      *domain.ResultSpec                    `json:"result,omitempty"`
	Artifacts   []*domain.Artifact                    `json:"artifacts,omitempty"`
	Validations map[string][]*domain.ValidationResult `json:"validations,omitempty"`
	Logs        string                                `json:"logs,omitempty"`
	LogsRef     string                                `json:"logs_ref,omitempty"`
	Warnings    []string                              `json:"warnings,omitempty"`
}

type ProjectSummary struct {
	ID              string `json:"id"`
	Name            string `json:"name,omitempty"`
	RepoRoot        string `json:"repo_root"`
	DefaultAdapter  string `json:"default_adapter"`
	Profile         string `json:"profile"`
	DaemonURL       string `json:"daemon_url"`
	Resolution      string `json:"resolution,omitempty"`
	SharedToRelay   bool   `json:"shared_to_relay"`
	RelayInstanceID string `json:"relay_instance_id,omitempty"`
}

type TaskReport struct {
	OK             bool         `json:"ok"`
	Status         string       `json:"status"`
	TaskID         string       `json:"task_id,omitempty"`
	ProjectID      string       `json:"project_id"`
	RunID          string       `json:"run_id"`
	StepID         string       `json:"step_id"`
	Adapter        string       `json:"adapter"`
	Profile        string       `json:"profile"`
	AdapterProfile string       `json:"adapter_profile"`
	Title          string       `json:"title,omitempty"`
	Summary        string       `json:"summary,omitempty"`
	Step           *domain.Step `json:"step,omitempty"`
	Blocker        *Blocker     `json:"blocker,omitempty"`
	Evidence       Evidence     `json:"evidence,omitempty"`
	ExitCode       int          `json:"exit_code"`
}

type ExecutionReport struct {
	OK               bool                 `json:"ok"`
	Status           string               `json:"status"`
	Project          *ProjectSummary      `json:"project,omitempty"`
	Run              *domain.Run          `json:"run,omitempty"`
	Runs             []*domain.Run        `json:"runs,omitempty"`
	Steps            []*domain.Step       `json:"steps,omitempty"`
	Task             *TaskReport          `json:"task,omitempty"`
	Blocker          *Blocker             `json:"blocker,omitempty"`
	DaemonURL        string               `json:"daemon_url,omitempty"`
	CurrentProjectID string               `json:"current_project_id,omitempty"`
	ProjectCount     int                  `json:"project_count,omitempty"`
	Profiles         []profilepkg.Profile `json:"profiles,omitempty"`
	Profile          *profilepkg.Profile  `json:"profile,omitempty"`
	ExitCode         int                  `json:"exit_code"`
}

type RunPlanReport struct {
	OK            bool            `json:"ok"`
	Status        string          `json:"status"`
	ManifestPath  string          `json:"manifest_path"`
	Project       *ProjectSummary `json:"project,omitempty"`
	Run           *domain.Run     `json:"run,omitempty"`
	Tasks         []TaskReport    `json:"tasks"`
	StoppedAtTask string          `json:"stopped_at_task,omitempty"`
	Blocker       *Blocker        `json:"blocker,omitempty"`
	Evidence      Evidence        `json:"evidence,omitempty"`
	ReportPath    string          `json:"report_path,omitempty"`
	ExitCode      int             `json:"exit_code"`
}

type ErrorReport struct {
	OK       bool     `json:"ok"`
	Status   string   `json:"status"`
	Error    string   `json:"error"`
	Blocker  *Blocker `json:"blocker,omitempty"`
	ExitCode int      `json:"exit_code"`
}

func summarizeProject(p project.Project, resolution, daemonURL, plannerProfile string) ProjectSummary {
	return ProjectSummary{
		ID:              p.ID,
		Name:            p.Name,
		RepoRoot:        p.RepoRoot,
		DefaultAdapter:  p.DefaultAdapter,
		Profile:         plannerProfile,
		DaemonURL:       daemonURL,
		Resolution:      resolution,
		SharedToRelay:   p.SharedToRelay,
		RelayInstanceID: p.RelayInstanceID,
	}
}
