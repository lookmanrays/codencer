package domain

import (
	"time"
)

type RunState string

const (
	RunStateCreated       RunState = "created"
	RunStateRunning       RunState = "running"
	RunStatePausedForGate RunState = "paused_for_gate"
	RunStateCompleted     RunState = "completed"
	RunStateFailed        RunState = "failed"
	RunStateCancelled     RunState = "cancelled"
)

// Run represents an end-to-end execution of a planner's intent.
type Run struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	State     RunState  `json:"state"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	RecoveryNotes string `json:"recovery_notes"`
	
	// Relationships
	Phases []*Phase
}

// Phase breaks a run into sequential segments of logic.
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
