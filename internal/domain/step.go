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
	StepStateNeedsManualAttention  StepState = "needs_manual_attention"
	StepStateFailedRetryable       StepState = "failed_retryable"
	StepStateFailedTerminal        StepState = "failed_terminal"
	StepStateTimeout               StepState = "timeout"
	StepStateCancelled             StepState = "cancelled"
)

// Step represents a discrete chunk of work executed by an adapter.
type Step struct {
	ID          string    `json:"id"`
	PhaseID     string    `json:"phase_id"`
	Title       string    `json:"title"`
	Goal        string    `json:"goal"`
	State       StepState `json:"state"`
	Policy      string    `json:"policy"`
	Adapter     string    `json:"adapter"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// IsTerminal returns true if the step has reached a final state.
func (s StepState) IsTerminal() bool {
	switch s {
	case StepStateCompleted, StepStateCompletedWithWarnings, StepStateFailedTerminal, StepStateTimeout, StepStateCancelled:
		return true
	default:
		return false
	}
}
