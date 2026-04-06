package domain

// AGInstance represents a discovered Antigravity Language Server instance.
type AGInstance struct {
	PID           int    `json:"pid"`
	HTTPSPort     int    `json:"https_port"`
	CSRFToken     string `json:"csrf_token"`
	WorkspaceRoot string `json:"workspace_root,omitempty"`
	IsReachable   bool   `json:"is_reachable"`
}
