package domain
type TaskContext struct {
	Summary string `json:"summary" yaml:"summary"`
}

// TaskSpec represents a declarative instruction bundle for an agent adapter.
type TaskSpec struct {
	Version         string              `json:"version" yaml:"version"`
	ProjectID       string              `json:"project_id" yaml:"project_id"`
	RunID           string              `json:"run_id" yaml:"run_id"`
	PhaseID         string              `json:"phase_id" yaml:"phase_id"`
	StepID          string              `json:"step_id" yaml:"step_id"`
	Title           string              `json:"title" yaml:"title"`
	Goal            string              `json:"goal" yaml:"goal"`
	Context         TaskContext         `json:"context" yaml:"context"`
	Constraints     []string            `json:"constraints" yaml:"constraints"`
	AllowedPaths    []string            `json:"allowed_paths" yaml:"allowed_paths"`
	ForbiddenPaths  []string            `json:"forbidden_paths" yaml:"forbidden_paths"`
	Validations     []ValidationCommand `json:"validations" yaml:"validations"`
	Acceptance      []string            `json:"acceptance" yaml:"acceptance"`
	StopConditions  []string            `json:"stop_conditions" yaml:"stop_conditions"`
	PolicyBundle    string              `json:"policy_bundle" yaml:"policy_bundle"`
	AdapterProfile  string              `json:"adapter_profile" yaml:"adapter_profile"`
}
