package sqlite

import (
	"context"
	"database/sql"
	"fmt"

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
	q := `INSERT INTO gates (id, run_id, step_id, description, status, created_at, resolved_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, q, gate.ID, gate.RunID, gate.StepID, gate.Description, string(gate.Status), gate.CreatedAt, gate.ResolvedAt)
	if err != nil {
		return fmt.Errorf("failed to create gate %s: %w", gate.ID, err)
	}
	return nil
}

// UpdateStatus modifies the status and resolved_at fields of a gate.
func (r *GatesRepo) UpdateStatus(ctx context.Context, gate *domain.Gate) error {
	q := `UPDATE gates SET status = ?, resolved_at = ? WHERE id = ?`
	res, err := r.db.ExecContext(ctx, q, string(gate.Status), gate.ResolvedAt, gate.ID)
	if err != nil {
		return fmt.Errorf("failed to update gate %s: %w", gate.ID, err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("gate not found: %s", gate.ID)
	}
	return nil
}

// Get retrieves a gate by its ID.
func (r *GatesRepo) Get(ctx context.Context, id string) (*domain.Gate, error) {
	q := `SELECT id, run_id, step_id, description, status, created_at, resolved_at FROM gates WHERE id = ?`
	row := r.db.QueryRowContext(ctx, q, id)
	
	var gate domain.Gate
	var status string
	err := row.Scan(&gate.ID, &gate.RunID, &gate.StepID, &gate.Description, &status, &gate.CreatedAt, &gate.ResolvedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get gate %s: %w", id, err)
	}
	gate.Status = domain.GateStatus(status)
	return &gate, nil
}
