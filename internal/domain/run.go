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
	ID        string
	ProjectID string
	State     RunState
	CreatedAt time.Time
	UpdatedAt time.Time
	
	// Relationships
	Phases []*Phase
}

// Phase breaks a run into sequential segments of logic.
type Phase struct {
	ID        string
	RunID     string
	Name      string
	Order     int
	CreatedAt time.Time
	UpdatedAt time.Time
	
	// Relationships
	Steps []*Step
}

// IsTerminal returns true if the run has reached a final state.
func (s RunState) IsTerminal() bool {
	return s == RunStateCompleted || s == RunStateFailed || s == RunStateCancelled
}
