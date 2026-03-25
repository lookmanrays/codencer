package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"agent-bridge/internal/domain"
)

// BenchmarksRepo SQLite implementation
type BenchmarksRepo struct {
	db *sql.DB
}

func NewBenchmarksRepo(db *sql.DB) *BenchmarksRepo {
	return &BenchmarksRepo{db: db}
}

func (r *BenchmarksRepo) Save(ctx context.Context, b *domain.BenchmarkScore) error {
	query := `
		INSERT INTO benchmarks (id, adapter, phase_id, duration_ms, validations_hit, validations_max, cost_cents, failure_reason, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			duration_ms=excluded.duration_ms,
			validations_hit=excluded.validations_hit,
			validations_max=excluded.validations_max,
			cost_cents=excluded.cost_cents,
			failure_reason=excluded.failure_reason
	`

	_, err := r.db.ExecContext(ctx, query,
		b.ID, b.Adapter, b.PhaseID, b.DurationMs, b.ValidationsHit, b.ValidationsMax, b.CostCents, b.FailureReason, b.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save benchmark: %w", err)
	}
	return nil
}

func (r *BenchmarksRepo) GetScoresByAdapter(ctx context.Context, adapter string) ([]*domain.BenchmarkScore, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, adapter, phase_id, duration_ms, validations_hit, validations_max, cost_cents, failure_reason, created_at FROM benchmarks WHERE adapter = ? ORDER BY created_at DESC`, adapter)
	if err != nil {
		return nil, fmt.Errorf("failed to query benchmark scores: %w", err)
	}
	defer rows.Close()

	var list []*domain.BenchmarkScore
	for rows.Next() {
		b := &domain.BenchmarkScore{}
		var fr sql.NullString
		if err := rows.Scan(&b.ID, &b.Adapter, &b.PhaseID, &b.DurationMs, &b.ValidationsHit, &b.ValidationsMax, &b.CostCents, &fr, &b.CreatedAt); err != nil {
			return nil, err
		}
		if fr.Valid {
			b.FailureReason = fr.String
		}
		list = append(list, b)
	}
	return list, nil
}
