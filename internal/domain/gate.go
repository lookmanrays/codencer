package domain

import (
	"time"
)

// GateStatus indicates whether a gate is pending, approved, or rejected.
type GateStatus string

const (
	GateStatusPending  GateStatus = "pending"
	GateStatusApproved GateStatus = "approved"
	GateStatusRejected GateStatus = "rejected"
)

// Gate is a policy pause that requires operator decision.
type Gate struct {
	ID          string     `json:"id"`
	RunID       string     `json:"run_id"`
	StepID      string     `json:"step_id"`
	Description string     `json:"description"`
	Status      GateStatus `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	ResolvedAt  *time.Time `json:"resolved_at"`
}
