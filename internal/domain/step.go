package domain

import (
	"time"
)

type StepState string

const (
	// StepStatePending: Work is queued but not yet accepted by an executor.
	StepStatePending StepState = "pending"
	// StepStateDispatching: Bridge is preparing the environment or selecting an adapter.
	StepStateDispatching StepState = "dispatching"
	// StepStateRunning: Adapter process is active and executing the task.
	StepStateRunning StepState = "running"
	// StepStateCollectingArtifacts: Adapter finished; Bridge is retrieving logs/files.
	StepStateCollectingArtifacts StepState = "collecting_artifacts"
	// StepStateValidating: Bridge is running configured verification commands.
	StepStateValidating StepState = "validating"
	// StepStateCompleted: Task finished successfully as reported by the adapter. Resulting evidence is available for the planner.
	StepStateCompleted StepState = "completed"
	// StepStateCompletedWithWarnings: Success, but with non-critical lint/test issues.
	StepStateCompletedWithWarnings StepState = "completed_with_warnings"
	// StepStateNeedsApproval: Bridge reports a policy gate hit; Planner must approve/reject to proceed.
	StepStateNeedsApproval StepState = "needs_approval"
	// StepStateNeedsManualAttention: Bridge reports a blocking condition (e.g. unknown error) that it cannot resolve; control is returned to the planner.
	StepStateNeedsManualAttention StepState = "needs_manual_attention"
	// StepStateFailedRetryable: Bridge reports an unsuccessful outcome but identifies it as potentially recoverable via retry.
	StepStateFailedRetryable StepState = "failed_retryable"
	// StepStateFailedTerminal: Execution reached an unsuccessful terminal state; requires a new plan or fix by the planner.
	StepStateFailedTerminal StepState = "failed_terminal"
	// StepStateFailedValidation: Agent finished but one or more tests/validations failed.
	StepStateFailedValidation StepState = "failed_validation"
	// StepStateFailedAdapter: The adapter process or agent binary crashed/failed.
	StepStateFailedAdapter StepState = "failed_adapter"
	// StepStateFailedBridge: Orchestrator/Bridge error (e.g. worktree, lock, disk failure).
	StepStateFailedBridge StepState = "failed_bridge"
	// StepStateTimeout: Execution exceeded defined limits and was killed by the bridge supervisor.
	StepStateTimeout StepState = "timeout"
	// StepStateCancelled: Execution was explicitly stopped by the planner/operator.
	StepStateCancelled StepState = "cancelled"
)

// Step is a specific, atomic execution unit issued by the planner.
// The bridge executes steps by dispatching them to adapters and reporting the outcome.
type Step struct {
	ID                   string                `json:"id"`
	PhaseID              string                `json:"phase_id"`
	Title                string                `json:"title"`
	Goal                 string                `json:"goal"`
	State                StepState             `json:"state"`
	Policy               string                `json:"policy"`
	Adapter              string                `json:"adapter"`
	TimeoutSeconds       int                   `json:"timeout_seconds"`
	Validations          []ValidationCommand   `json:"validations,omitempty"`
	StatusReason         string                `json:"status_reason,omitempty"`
	CreatedAt            time.Time             `json:"created_at"`
	UpdatedAt            time.Time             `json:"updated_at"`
	TaskSpecSnapshot     *TaskSpec             `json:"-"`
	SubmissionProvenance *SubmissionProvenance `json:"-"`
}

// IsTerminal returns true if the step has reached a final state.
func (s StepState) IsTerminal() bool {
	switch s {
	case StepStateCompleted, StepStateCompletedWithWarnings, StepStateFailedTerminal,
		StepStateFailedValidation, StepStateFailedAdapter, StepStateFailedBridge,
		StepStateFailedRetryable, StepStateTimeout, StepStateCancelled:
		return true
	default:
		return false
	}
}
