package domain

import (
	"time"
)

// ValidationState represents the lifecycle/outcome of a verification command.
type ValidationState string

const (
	ValidationStateNotRun  ValidationState = "not_run"
	ValidationStateRunning ValidationState = "running"
	ValidationStatePassed  ValidationState = "passed"
	ValidationStateFailed  ValidationState = "failed"
	ValidationStateErrored ValidationState = "errored"
)

// ValidationResult represents the outcome of a validation run (e.g. tests or lint).
type ValidationResult struct {
	Name       string          `json:"name"`
	Command    string          `json:"command"`
	State      ValidationState `json:"state"`
	Passed     bool            `json:"passed"`
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
