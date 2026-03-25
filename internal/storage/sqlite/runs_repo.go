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
	q := `INSERT INTO runs (id, project_id, state, created_at, updated_at, recovery_notes) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, q, run.ID, run.ProjectID, string(run.State), run.CreatedAt, run.UpdatedAt, run.RecoveryNotes)
	if err != nil {
		return fmt.Errorf("failed to create run %s: %w", run.ID, err)
	}
	return nil
}

// Get retrieves a run from the database by ID.
func (r *RunsRepo) Get(ctx context.Context, id string) (*domain.Run, error) {
	q := `SELECT id, project_id, state, created_at, updated_at, recovery_notes FROM runs WHERE id = ?`
	row := r.db.QueryRowContext(ctx, q, id)

	var run domain.Run
	var state string
	var recNotes sql.NullString
	err := row.Scan(&run.ID, &run.ProjectID, &state, &run.CreatedAt, &run.UpdatedAt, &recNotes)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // not found
		}
		return nil, fmt.Errorf("failed to get run %s: %w", id, err)
	}
	run.State = domain.RunState(state)
	if recNotes.Valid {
		run.RecoveryNotes = recNotes.String
	}

	return &run, nil
}

// UpdateState modifies the state string and updated_at time of a run, and includes recovery_notes.
func (r *RunsRepo) UpdateState(ctx context.Context, run *domain.Run) error {
	q := `UPDATE runs SET state = ?, updated_at = ?, recovery_notes = ? WHERE id = ?`
	res, err := r.db.ExecContext(ctx, q, string(run.State), run.UpdatedAt, run.RecoveryNotes, run.ID)
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

// ListByState retrieves all runs currently in a specific state.
func (r *RunsRepo) ListByState(ctx context.Context, state domain.RunState) ([]*domain.Run, error) {
	q := `SELECT id, project_id, state, created_at, updated_at, recovery_notes FROM runs WHERE state = ?`
	rows, err := r.db.QueryContext(ctx, q, string(state))
	if err != nil {
		return nil, fmt.Errorf("failed to list runs by state: %w", err)
	}
	defer rows.Close()

	var runs []*domain.Run
	for rows.Next() {
		var run domain.Run
		var s string
		var recNotes sql.NullString
		if err := rows.Scan(&run.ID, &run.ProjectID, &s, &run.CreatedAt, &run.UpdatedAt, &recNotes); err != nil {
			return nil, err
		}
		run.State = domain.RunState(s)
		if recNotes.Valid {
			run.RecoveryNotes = recNotes.String
		}
		runs = append(runs, &run)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return runs, nil
}

// List retrieves all runs in the database, ordered by latest first.
func (r *RunsRepo) List(ctx context.Context) ([]*domain.Run, error) {
	q := `SELECT id, project_id, state, created_at, updated_at, recovery_notes FROM runs ORDER BY created_at DESC LIMIT 50`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("failed to list runs: %w", err)
	}
	defer rows.Close()

	var runs []*domain.Run
	for rows.Next() {
		var run domain.Run
		var s string
		var recNotes sql.NullString
		if err := rows.Scan(&run.ID, &run.ProjectID, &s, &run.CreatedAt, &run.UpdatedAt, &recNotes); err != nil {
			return nil, err
		}
		run.State = domain.RunState(s)
		if recNotes.Valid {
			run.RecoveryNotes = recNotes.String
		}
		runs = append(runs, &run)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return runs, nil
}
