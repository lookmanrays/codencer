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
	Result    *Result
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Result summarizes what happened in an Attempt.
type Result struct {
	State               StepState `json:"state"`
	Summary             string    `json:"summary"`
	NeedsHumanDecision  bool      `json:"needs_human_decision"`
	Warnings            []string  `json:"warnings"`
	Questions           []string  `json:"questions"`
}
