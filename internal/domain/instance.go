package domain

import "time"

// InstanceInfo represents the identity and state of a Codencer daemon instance.
type InstanceInfo struct {
	Version       string    `json:"version"`
	RepoRoot      string    `json:"repo_root"`
	StateDir      string    `json:"state_dir"`
	WorkspaceRoot string    `json:"workspace_root"`
	Host          string    `json:"host"`
	Port          int       `json:"port"`
	BaseURL       string    `json:"base_url"`
	ExecutionMode string    `json:"execution_mode"`
	PID           int       `json:"pid"`
	StartedAt     time.Time `json:"started_at"`
}
