package domain

import (
	"time"
)

// Attempt is a single, concrete execution try of a Step through a specific adapter.
// It captures the raw telemetry and execution results of one pass.
type Attempt struct {
	ID        string
	StepID    string
	Number    int
	Adapter   string
	Result    *ResultSpec
	CreatedAt time.Time
	UpdatedAt time.Time
}

