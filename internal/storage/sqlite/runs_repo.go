package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"agent-bridge/internal/domain"
)

// RunsRepo provides persistence logic for domain.Run.
type RunsRepo struct {
	db *sql.DB
}

// NewRunsRepo creates a new RunsRepo instance.
func NewRunsRepo(db *sql.DB) *RunsRepo {
	return &RunsRepo{db: db}
}

// Create inserts a new run into the database.
func (r *RunsRepo) Create(ctx context.Context, run *domain.Run) error {
	q := `INSERT INTO runs (id, project_id, state, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, q, run.ID, run.ProjectID, string(run.State), run.CreatedAt, run.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create run %s: %w", run.ID, err)
	}
	return nil
}

// Get retrieves a run from the database by ID.
func (r *RunsRepo) Get(ctx context.Context, id string) (*domain.Run, error) {
	q := `SELECT id, project_id, state, created_at, updated_at FROM runs WHERE id = ?`
	row := r.db.QueryRowContext(ctx, q, id)

	var run domain.Run
	var state string
	err := row.Scan(&run.ID, &run.ProjectID, &state, &run.CreatedAt, &run.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // not found
		}
		return nil, fmt.Errorf("failed to get run %s: %w", id, err)
	}
	run.State = domain.RunState(state)

	return &run, nil
}

// UpdateState modifies the state string and updated_at time of a run.
func (r *RunsRepo) UpdateState(ctx context.Context, run *domain.Run) error {
	q := `UPDATE runs SET state = ?, updated_at = ? WHERE id = ?`
	res, err := r.db.ExecContext(ctx, q, string(run.State), run.UpdatedAt, run.ID)
	if err != nil {
		return fmt.Errorf("failed to update run state %s: %w", run.ID, err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("run not found: %s", run.ID)
	}
	return nil
}
