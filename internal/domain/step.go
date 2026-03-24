package domain

import (
	"time"
)

type StepState string

const (
	StepStatePending               StepState = "pending"
	StepStateDispatching           StepState = "dispatching"
	StepStateRunning               StepState = "running"
	StepStateCollectingArtifacts   StepState = "collecting_artifacts"
	StepStateValidating            StepState = "validating"
	StepStateCompleted             StepState = "completed"
	StepStateCompletedWithWarnings StepState = "completed_with_warnings"
	StepStateNeedsApproval         StepState = "needs_approval"
	StepStateFailedRetryable       StepState = "failed_retryable"
	StepStateFailedTerminal        StepState = "failed_terminal"
	StepStateCancelled             StepState = "cancelled"
)

// Step represents a discrete chunk of work executed by an adapter.
type Step struct {
	ID          string
	PhaseID     string
	Title       string
	Goal        string
	State       StepState
	Policy      string // ID or name of the policy bundle
	Adapter     string // ID or profile of the adapter
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// IsTerminal returns true if the step has reached a final state.
func (s StepState) IsTerminal() bool {
	switch s {
	case StepStateCompleted, StepStateCompletedWithWarnings, StepStateFailedTerminal, StepStateCancelled:
		return true
	default:
		return false
	}
}
