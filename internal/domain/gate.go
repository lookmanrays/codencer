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
	ID          string
	RunID       string
	StepID      string
	Description string
	Status      GateStatus
	CreatedAt   time.Time
	ResolvedAt  *time.Time
}
