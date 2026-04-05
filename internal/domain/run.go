package domain

import (
	"time"
)

type RunState string

const (
	RunStateCreated       RunState = "created"
	RunStateRunning       RunState = "running"
	RunStatePausedForGate RunState = "paused_for_gate"
	RunStateCompleted     RunState = "completed" // Run finished successfully
	RunStateFailed        RunState = "failed"    // Run reached an unsuccessful terminal state
	RunStateCancelled     RunState = "cancelled" // Run was explicitly stopped
)

// Run is an execution session that acts as a container for related work phases and steps.
// It tracks the overall lifecycle of a project-level objective as reported by the bridge.
type Run struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	ConversationID string `json:"conversation_id,omitempty"`
	PlannerID      string `json:"planner_id,omitempty"`
	ExecutorID     string `json:"executor_id,omitempty"`
	State     RunState  `json:"state"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	RecoveryNotes string `json:"recovery_notes"`
	
	// Relationships
	Phases []*Phase
}

// Phase is a logical grouping of steps within a Run. 
// It provides structure for the planner to organize complex work into sequential segments.
type Phase struct {
	ID        string    `json:"id"`
	RunID     string    `json:"run_id"`
	Name      string    `json:"name"`
	SeqOrder  int       `json:"seq_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	
	// Relationships
	Steps []*Step
}

// IsTerminal returns true if the run has reached a final state.
func (s RunState) IsTerminal() bool {
	return s == RunStateCompleted || s == RunStateFailed || s == RunStateCancelled
}
