package domain

import "time"

// InstanceBrokerInfo captures the daemon's current Antigravity broker/binding status.
type InstanceBrokerInfo struct {
	Enabled       bool        `json:"enabled"`
	Mode          string      `json:"mode"`
	URL           string      `json:"url,omitempty"`
	BoundInstance *AGInstance `json:"bound_instance,omitempty"`
}

// InstanceInfo represents the identity and state of a Codencer daemon instance.
type InstanceInfo struct {
	ID            string                 `json:"id"`
	Version       string                 `json:"version"`
	RepoName      string                 `json:"repo_name"`
	RepoRoot      string                 `json:"repo_root"`
	StateDir      string                 `json:"state_dir"`
	WorkspaceRoot string                 `json:"workspace_root"`
	ManifestPath  string                 `json:"manifest_path"`
	Host          string                 `json:"host"`
	Port          int                    `json:"port"`
	BaseURL       string                 `json:"base_url"`
	ExecutionMode string                 `json:"execution_mode"`
	PID           int                    `json:"pid"`
	StartedAt     time.Time              `json:"started_at"`
	Adapters      []CompatibilityAdapter `json:"adapters,omitempty"`
	Broker        InstanceBrokerInfo     `json:"broker"`
}
