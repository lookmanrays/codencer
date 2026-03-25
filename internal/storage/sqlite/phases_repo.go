package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"agent-bridge/internal/domain"
)

// PhasesRepo manages Phase persistence.
type PhasesRepo struct {
	db *sql.DB
}

// NewPhasesRepo creates a new PhasesRepo.
func NewPhasesRepo(db *sql.DB) *PhasesRepo {
	return &PhasesRepo{db: db}
}

// Create inserts a new phase.
func (r *PhasesRepo) Create(ctx context.Context, phase *domain.Phase) error {
	const q = `
		INSERT INTO phases (id, run_id, name, seq_order, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, q,
		phase.ID,
		phase.RunID,
		phase.Name,
		phase.SeqOrder,
		phase.CreatedAt,
		phase.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create phase: %w", err)
	}
	return nil
}

// ListByRun returns all phases for a run.
func (r *PhasesRepo) ListByRun(ctx context.Context, runID string) ([]*domain.Phase, error) {
	const q = `
		SELECT id, run_id, name, seq_order, created_at, updated_at
		FROM phases WHERE run_id = ? ORDER BY seq_order ASC
	`
	rows, err := r.db.QueryContext(ctx, q, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to list phases: %w", err)
	}
	defer rows.Close()

	var phases []*domain.Phase
	for rows.Next() {
		var phase domain.Phase
		if err := rows.Scan(
			&phase.ID, &phase.RunID, &phase.Name, &phase.SeqOrder,
			&phase.CreatedAt, &phase.UpdatedAt,
		); err != nil {
			return nil, err
		}
		phases = append(phases, &phase)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return phases, nil
}

// Get returns a single phase by ID.
func (r *PhasesRepo) Get(ctx context.Context, id string) (*domain.Phase, error) {
	const q = `
		SELECT id, run_id, name, seq_order, created_at, updated_at
		FROM phases WHERE id = ?
	`
	var p domain.Phase
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&p.ID, &p.RunID, &p.Name, &p.SeqOrder, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get phase: %w", err)
	}
	return &p, nil
}
