package domain

// CompatibilityAdapter describes the runtime availability of a registered adapter.
type CompatibilityAdapter struct {
	ID           string   `json:"id"`
	Available    bool     `json:"available"`
	Status       string   `json:"status"`
	Mode         string   `json:"mode"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// CompatibilityEnvironment captures the relevant runtime environment signals.
type CompatibilityEnvironment struct {
	OS             string `json:"os"`
	VSCodeDetected bool   `json:"vscode_detected"`
}

// CompatibilityInfo is the daemon-reported runtime compatibility surface.
type CompatibilityInfo struct {
	Tier        int                      `json:"tier"`
	Adapters    []CompatibilityAdapter   `json:"adapters"`
	Environment CompatibilityEnvironment `json:"environment"`
}
