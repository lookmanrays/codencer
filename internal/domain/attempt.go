package domain

import (
	"time"
)

// Attempt represents a single execution try of a Step.
type Attempt struct {
	ID        string
	StepID    string
	Number    int
	Adapter   string
	Result    *ResultSpec
	CreatedAt time.Time
	UpdatedAt time.Time
}

