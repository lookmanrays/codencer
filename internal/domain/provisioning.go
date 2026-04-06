package domain

// ProvisioningResult captures the outcome and telemetry of a workspace preparation.
type ProvisioningResult struct {
	Success    bool     `json:"success"`
	Summary    string   `json:"summary"`
	Log        []string `json:"log"`
	DurationMs int64    `json:"duration_ms"`

	// Structured telemetry
	EnvironmentFiles []string `json:"environment_files,omitempty"`
	Symlinks         []string `json:"symlinks,omitempty"`
	PostCreateHook   string   `json:"post_create_hook,omitempty"`
	HookStatus       string   `json:"hook_status,omitempty"`
}
