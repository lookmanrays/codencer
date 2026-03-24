package domain

// ValidationResult represents the outcome of a validation run (e.g. tests or lint).
type ValidationResult struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Error  string `json:"error,omitempty"`
}

// ValidationCommand represents a command to execute for verifying correctness.
type ValidationCommand struct {
	Name    string `json:"name"`
	Command string `json:"command"`
}
