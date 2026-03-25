package domain

import (
	"time"
)

// BenchmarkScore represents the objective evaluation of an adapter's performance on a specific task.
type BenchmarkScore struct {
	ID             string    `json:"id"`
	Adapter        string    `json:"adapter"`
	PhaseID        string    `json:"phase_id"` // E.g., which corpus/test this benchmark targeted
	DurationMs     int64     `json:"duration_ms"`
	ValidationsHit int       `json:"validations_hit"` // Total validations passed
	ValidationsMax int       `json:"validations_max"` // Expected validations
	CostCents      float64   `json:"cost_cents"`
	FailureReason  string    `json:"failure_reason,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// Successful determines natively if the benchmark run was deemed fully successful.
func (b *BenchmarkScore) Successful() bool {
	return b.FailureReason == "" && b.ValidationsHit >= b.ValidationsMax
}
