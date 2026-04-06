package domain

// ProvisioningResult captures the outcome and telemetry of a workspace preparation.
type ProvisioningResult struct {
	Success    bool     `json:"success"`
	Summary    string   `json:"summary"`
	Log        []string `json:"log"`
	DurationMs int64    `json:"duration_ms"`
}
