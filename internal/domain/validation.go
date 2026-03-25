package domain

import (
	"time"
)

// ValidationStatus represents the lifecycle/outcome of a verification command.
type ValidationStatus string

const (
	ValidationStatusNotRun  ValidationStatus = "not_run"
	ValidationStatusRunning ValidationStatus = "running"
	ValidationStatusPassed  ValidationStatus = "passed"
	ValidationStatusFailed  ValidationStatus = "failed"
	ValidationStatusErrored ValidationStatus = "errored"
)

// ValidationResult represents the outcome of a validation run (e.g. tests or lint).
type ValidationResult struct {
	Name       string           `json:"name"`
	Command    string           `json:"command"`
	Status     ValidationStatus `json:"status"`
	Passed     bool             `json:"passed"`
	ExitCode   int              `json:"exit_code"`
	StdoutRef  string           `json:"stdout_ref,omitempty"`
	StderrRef  string           `json:"stderr_ref,omitempty"`
	Error      string           `json:"error,omitempty"`
	DurationMs int64            `json:"duration_ms"`
	UpdatedAt  time.Time        `json:"updated_at"`
}

// ValidationCommand represents a command to execute for verifying correctness.
type ValidationCommand struct {
	Name    string `json:"name"`
	Command string `json:"command"`
}
