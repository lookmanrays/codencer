package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"agent-bridge/internal/domain"
)

// ValidationsRepo provides persistence logic for validation results.
type ValidationsRepo struct {
	db *sql.DB
}

func NewValidationsRepo(db *sql.DB) *ValidationsRepo {
	return &ValidationsRepo{db: db}
}

// Create inserts a new validation result.
func (r *ValidationsRepo) Create(ctx context.Context, attemptID string, res *domain.ValidationResult) error {
	q := `
		INSERT INTO validations (
			attempt_id, name, command, status, passed, exit_code, stdout_ref, stderr_ref, error_msg, duration_ms, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now().UTC()
	if res.UpdatedAt.IsZero() {
		res.UpdatedAt = now
	}
	_, err := r.db.ExecContext(ctx, q,
		attemptID, res.Name, res.Command, string(res.Status), res.Passed, res.ExitCode,
		res.StdoutRef, res.StderrRef, res.Error, res.DurationMs, now, res.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create validation %s for attempt %s: %w", res.Name, attemptID, err)
	}
	return nil
}

// ListByStep retrieves all validations for all attempts of a step.
func (r *ValidationsRepo) ListByStep(ctx context.Context, stepID string) (map[string][]*domain.ValidationResult, error) {
	q := `
		SELECT v.attempt_id, v.name, v.command, v.status, v.passed, v.exit_code, v.stdout_ref, v.stderr_ref, v.error_msg, v.duration_ms, v.updated_at
		FROM validations v
		INNER JOIN attempts a ON v.attempt_id = a.id
		WHERE a.step_id = ?
		ORDER BY v.updated_at ASC
	`
	rows, err := r.db.QueryContext(ctx, q, stepID)
	if err != nil {
		return nil, fmt.Errorf("failed to list validations for step %s: %w", stepID, err)
	}
	defer rows.Close()

	results := make(map[string][]*domain.ValidationResult)
	for rows.Next() {
		var attemptID string
		var res domain.ValidationResult
		var statusStr string
		var stdout, stderr, errorMsg sql.NullString
		if err := rows.Scan(
			&attemptID, &res.Name, &res.Command, &statusStr, &res.Passed, &res.ExitCode,
			&stdout, &stderr, &errorMsg, &res.DurationMs, &res.UpdatedAt,
		); err != nil {
			return nil, err
		}
		res.Status = domain.ValidationStatus(statusStr)
		if stdout.Valid { res.StdoutRef = stdout.String }
		if stderr.Valid { res.StderrRef = stderr.String }
		if errorMsg.Valid { res.Error = errorMsg.String }
		
		results[attemptID] = append(results[attemptID], &res)
	}
	return results, rows.Err()
}

// ListByAttempt retrieves validations for a specific attempt.
func (r *ValidationsRepo) ListByAttempt(ctx context.Context, attemptID string) ([]*domain.ValidationResult, error) {
	q := `
		SELECT name, command, status, passed, exit_code, stdout_ref, stderr_ref, error_msg, duration_ms, updated_at
		FROM validations WHERE attempt_id = ?
	`
	rows, err := r.db.QueryContext(ctx, q, attemptID)
	if err != nil {
		return nil, fmt.Errorf("failed to list validations for attempt %s: %w", attemptID, err)
	}
	defer rows.Close()

	var results []*domain.ValidationResult
	for rows.Next() {
		var vr domain.ValidationResult
		var statusStr string
		var stdout, stderr, errorMsg sql.NullString
		if err := rows.Scan(
			&vr.Name, &vr.Command, &statusStr, &vr.Passed, &vr.ExitCode,
			&stdout, &stderr, &errorMsg, &vr.DurationMs, &vr.UpdatedAt,
		); err != nil {
			return nil, err
		}
		vr.Status = domain.ValidationStatus(statusStr)
		if stdout.Valid { vr.StdoutRef = stdout.String }
		if stderr.Valid { vr.StderrRef = stderr.String }
		if errorMsg.Valid { vr.Error = errorMsg.String }
		results = append(results, &vr)
	}
	return results, rows.Err()
}
