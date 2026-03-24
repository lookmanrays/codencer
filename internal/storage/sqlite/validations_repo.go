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
	q := `INSERT INTO validations (attempt_id, name, passed, error_msg, created_at) VALUES (?, ?, ?, ?, ?)`
	createdAt := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, q, attemptID, res.Name, res.Passed, res.Error, createdAt)
	if err != nil {
		return fmt.Errorf("failed to create validation %s for attempt %s: %w", res.Name, attemptID, err)
	}
	return nil
}

// ListByAttempt retrieves validations for a specific attempt.
func (r *ValidationsRepo) ListByAttempt(ctx context.Context, attemptID string) ([]*domain.ValidationResult, error) {
	q := `SELECT name, passed, error_msg FROM validations WHERE attempt_id = ?`
	rows, err := r.db.QueryContext(ctx, q, attemptID)
	if err != nil {
		return nil, fmt.Errorf("failed to list validations for attempt %s: %w", attemptID, err)
	}
	defer rows.Close()

	var results []*domain.ValidationResult
	for rows.Next() {
		var vr domain.ValidationResult
		var errMsg sql.NullString
		if err := rows.Scan(&vr.Name, &vr.Passed, &errMsg); err != nil {
			return nil, err
		}
		if errMsg.Valid {
			vr.Error = errMsg.String
		}
		results = append(results, &vr)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}
