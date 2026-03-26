package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"agent-bridge/internal/domain"
)

// StepsRepo manages Step persistence.
type StepsRepo struct {
	db *sql.DB
}

// NewStepsRepo creates a new StepsRepo.
func NewStepsRepo(db *sql.DB) *StepsRepo {
	return &StepsRepo{db: db}
}

// Create inserts a new step.
func (r *StepsRepo) Create(ctx context.Context, step *domain.Step) error {
	const q = `
		INSERT INTO steps (id, phase_id, title, goal, state, policy, adapter, timeout_seconds, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, q,
		step.ID,
		step.PhaseID,
		step.Title,
		step.Goal,
		string(step.State),
		step.Policy,
		step.Adapter,
		step.TimeoutSeconds,
		step.CreatedAt,
		step.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create step: %w", err)
	}
	return nil
}

// Get retrieves a step by ID.
func (r *StepsRepo) Get(ctx context.Context, id string) (*domain.Step, error) {
	const q = `
		SELECT id, phase_id, title, goal, state, policy, adapter, timeout_seconds, created_at, updated_at
		FROM steps WHERE id = ?
	`
	row := r.db.QueryRowContext(ctx, q, id)
	
	var step domain.Step
	var stateStr string
	err := row.Scan(
		&step.ID,
		&step.PhaseID,
		&step.Title,
		&step.Goal,
		&stateStr,
		&step.Policy,
		&step.Adapter,
		&step.TimeoutSeconds,
		&step.CreatedAt,
		&step.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil // not found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get step: %w", err)
	}
	
	step.State = domain.StepState(stateStr)
	return &step, nil
}

// UpdateState updates the state of a step.
func (r *StepsRepo) UpdateState(ctx context.Context, step *domain.Step) error {
	const q = `
		UPDATE steps SET state = ?, updated_at = ? WHERE id = ?
	`
	_, err := r.db.ExecContext(ctx, q, string(step.State), step.UpdatedAt, step.ID)
	if err != nil {
		return fmt.Errorf("failed to update step state: %w", err)
	}
	return nil
}

// ListByPhase returns all steps for a phase.
func (r *StepsRepo) ListByPhase(ctx context.Context, phaseID string) ([]*domain.Step, error) {
	const q = `
		SELECT id, phase_id, title, goal, state, policy, adapter, timeout_seconds, created_at, updated_at
		FROM steps WHERE phase_id = ? ORDER BY created_at ASC
	`
	rows, err := r.db.QueryContext(ctx, q, phaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to list steps: %w", err)
	}
	defer rows.Close()

	var steps []*domain.Step
	for rows.Next() {
		var step domain.Step
		var stateStr string
		if err := rows.Scan(
			&step.ID, &step.PhaseID, &step.Title, &step.Goal,
			&stateStr, &step.Policy, &step.Adapter, &step.TimeoutSeconds,
			&step.CreatedAt, &step.UpdatedAt,
		); err != nil {
			return nil, err
		}
		step.State = domain.StepState(stateStr)
		steps = append(steps, &step)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return steps, nil
}

// ListByRun returns all steps for a run, joining on phases to find them efficiently.
func (r *StepsRepo) ListByRun(ctx context.Context, runID string) ([]*domain.Step, error) {
	const q = `
		SELECT s.id, s.phase_id, s.title, s.goal, s.state, s.policy, s.adapter, s.timeout_seconds, s.created_at, s.updated_at
		FROM steps s
		JOIN phases p ON s.phase_id = p.id
		WHERE p.run_id = ? ORDER BY s.created_at ASC
	`
	rows, err := r.db.QueryContext(ctx, q, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to list steps for run: %w", err)
	}
	defer rows.Close()

	var steps []*domain.Step
	for rows.Next() {
		var step domain.Step
		var stateStr string
		if err := rows.Scan(
			&step.ID, &step.PhaseID, &step.Title, &step.Goal,
			&stateStr, &step.Policy, &step.Adapter, &step.TimeoutSeconds,
			&step.CreatedAt, &step.UpdatedAt,
		); err != nil {
			return nil, err
		}
		step.State = domain.StepState(stateStr)
		steps = append(steps, &step)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return steps, nil
}
