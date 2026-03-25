package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"agent-bridge/internal/domain"
)

// GatesRepo provides persistence logic for domain.Gate.
type GatesRepo struct {
	db *sql.DB
}

func NewGatesRepo(db *sql.DB) *GatesRepo {
	return &GatesRepo{db: db}
}

// Create inserts a new gate record into the database.
func (r *GatesRepo) Create(ctx context.Context, gate *domain.Gate) error {
	q := `INSERT INTO gates (id, run_id, step_id, description, state, created_at) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, q, gate.ID, gate.RunID, gate.StepID, gate.Description, string(gate.State), gate.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create gate %s: %w", gate.ID, err)
	}
	return nil
}

// Resolve modifies the status and resolved_at fields of a gate.
func (r *GatesRepo) Resolve(ctx context.Context, id string, state domain.GateState) error {
	now := time.Now().UTC()
	q := `UPDATE gates SET state = ?, resolved_at = ? WHERE id = ?`
	res, err := r.db.ExecContext(ctx, q, string(state), now, id)
	if err != nil {
		return fmt.Errorf("failed to resolve gate %s: %w", id, err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("gate not found: %s", id)
	}
	return nil
}

// Get retrieves a gate by its ID.
func (r *GatesRepo) Get(ctx context.Context, id string) (*domain.Gate, error) {
	q := `SELECT id, run_id, step_id, description, state, created_at, resolved_at FROM gates WHERE id = ?`
	row := r.db.QueryRowContext(ctx, q, id)
	
	var gate domain.Gate
	var s string // Changed from 'status' to 's'
	err := row.Scan(&gate.ID, &gate.RunID, &gate.StepID, &gate.Description, &s, &gate.CreatedAt, &gate.ResolvedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get gate %s: %w", id, err)
	}
	gate.State = domain.GateState(s) // Changed from 'gate.Status = domain.GateStatus(status)'
	return &gate, nil
}

// ListByRun retrieves all gates associated with a specific run.
func (r *GatesRepo) ListByRun(ctx context.Context, runID string) ([]*domain.Gate, error) {
	q := `SELECT id, run_id, step_id, description, state, created_at, resolved_at FROM gates WHERE run_id = ? ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, q, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to list gates: %w", err)
	}
	defer rows.Close()

	var gates []*domain.Gate
	for rows.Next() {
		var gate domain.Gate
		var s string // Changed from 'status' to 's'
		if err := rows.Scan(&gate.ID, &gate.RunID, &gate.StepID, &gate.Description, &s, &gate.CreatedAt, &gate.ResolvedAt); err != nil {
			return nil, err
		}
		gate.State = domain.GateState(s) // Changed from 'gate.Status = domain.GateStatus(status)'
		gates = append(gates, &gate)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return gates, nil
}
