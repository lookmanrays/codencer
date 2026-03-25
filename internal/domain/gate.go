package domain

import (
	"time"
)

// GateState indicates whether a gate is pending, approved, or rejected.
type GateState string

const (
	GateStatePending  GateState = "pending"
	GateStateApproved GateState = "approved"
	GateStateRejected GateState = "rejected"
)

// Gate is a policy pause that requires operator decision.
type Gate struct {
	ID          string     `json:"id"`
	RunID       string     `json:"run_id"`
	StepID      string     `json:"step_id"`
	Description string     `json:"description"`
	State       GateState  `json:"state"`
	CreatedAt   time.Time  `json:"created_at"`
	ResolvedAt  *time.Time `json:"resolved_at"`
}
