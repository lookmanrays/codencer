package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"agent-bridge/internal/domain"
)

// AttemptsRepo manages Attempt persistence.
type AttemptsRepo struct {
	db *sql.DB
}

// NewAttemptsRepo creates a new AttemptsRepo.
func NewAttemptsRepo(db *sql.DB) *AttemptsRepo {
	return &AttemptsRepo{db: db}
}

// Create inserts a new attempt.
func (r *AttemptsRepo) Create(ctx context.Context, attempt *domain.Attempt) error {
	const q = `
		INSERT INTO attempts (id, step_id, number, adapter, status, summary, needs_human_decision, warnings, questions, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	var status string
	var summary string
	var needsDecision bool
	var warningsJSON, questionsJSON []byte
	
	if attempt.Result != nil {
		status = string(attempt.Result.Status)
		summary = attempt.Result.Summary
		needsDecision = attempt.Result.NeedsHumanDecision
		warningsJSON, _ = json.Marshal(attempt.Result.Warnings)
		questionsJSON, _ = json.Marshal(attempt.Result.Questions)
	} else {
		status = string(domain.StepStatePending)
	}

	_, err := r.db.ExecContext(ctx, q,
		attempt.ID,
		attempt.StepID,
		attempt.Number,
		attempt.Adapter,
		status,
		summary,
		needsDecision,
		string(warningsJSON),
		string(questionsJSON),
		attempt.CreatedAt,
		attempt.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create attempt: %w", err)
	}
	return nil
}

// Get retrieves an attempt by ID.
func (r *AttemptsRepo) Get(ctx context.Context, id string) (*domain.Attempt, error) {
	const q = `
		SELECT id, step_id, number, adapter, status, summary, needs_human_decision, warnings, questions, created_at, updated_at
		FROM attempts WHERE id = ?
	`
	row := r.db.QueryRowContext(ctx, q, id)
	
	var attempt domain.Attempt
	var statusStr string
	var summary string
	var needsDecision bool
	var warningsStr, questionsStr sql.NullString

	err := row.Scan(
		&attempt.ID,
		&attempt.StepID,
		&attempt.Number,
		&attempt.Adapter,
		&statusStr,
		&summary,
		&needsDecision,
		&warningsStr,
		&questionsStr,
		&attempt.CreatedAt,
		&attempt.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil // not found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get attempt: %w", err)
	}

	if statusStr != "" {
		attempt.Result = &domain.Result{
			Status:             domain.StepState(statusStr),
			Summary:            summary,
			NeedsHumanDecision: needsDecision,
		}
		if warningsStr.Valid && warningsStr.String != "" {
			_ = json.Unmarshal([]byte(warningsStr.String), &attempt.Result.Warnings)
		}
		if questionsStr.Valid && questionsStr.String != "" {
			_ = json.Unmarshal([]byte(questionsStr.String), &attempt.Result.Questions)
		}
	}

	return &attempt, nil
}

// UpdateResult updates the result of an attempt.
func (r *AttemptsRepo) UpdateResult(ctx context.Context, attempt *domain.Attempt) error {
	if attempt.Result == nil {
		return fmt.Errorf("attempt result is nil")
	}

	const q = `
		UPDATE attempts SET status = ?, summary = ?, needs_human_decision = ?, warnings = ?, questions = ?, updated_at = ?
		WHERE id = ?
	`
	warningsJSON, _ := json.Marshal(attempt.Result.Warnings)
	questionsJSON, _ := json.Marshal(attempt.Result.Questions)

	_, err := r.db.ExecContext(ctx, q,
		string(attempt.Result.Status),
		attempt.Result.Summary,
		attempt.Result.NeedsHumanDecision,
		string(warningsJSON),
		string(questionsJSON),
		attempt.UpdatedAt,
		attempt.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update attempt result: %w", err)
	}
	return nil
}

// ListByStep returns all attempts for a step.
func (r *AttemptsRepo) ListByStep(ctx context.Context, stepID string) ([]*domain.Attempt, error) {
	const q = `
		SELECT id, step_id, number, adapter, status, summary, needs_human_decision, warnings, questions, created_at, updated_at
		FROM attempts WHERE step_id = ? ORDER BY number ASC
	`
	rows, err := r.db.QueryContext(ctx, q, stepID)
	if err != nil {
		return nil, fmt.Errorf("failed to list attempts: %w", err)
	}
	defer rows.Close()

	var attempts []*domain.Attempt
	for rows.Next() {
		var attempt domain.Attempt
		var statusStr string
		var summary string
		var needsDecision bool
		var warningsStr, questionsStr sql.NullString

		if err := rows.Scan(
			&attempt.ID, &attempt.StepID, &attempt.Number, &attempt.Adapter,
			&statusStr, &summary, &needsDecision, &warningsStr, &questionsStr,
			&attempt.CreatedAt, &attempt.UpdatedAt,
		); err != nil {
			return nil, err
		}

		if statusStr != "" && statusStr != string(domain.StepStatePending) {
			attempt.Result = &domain.Result{
				Status:             domain.StepState(statusStr),
				Summary:            summary,
				NeedsHumanDecision: needsDecision,
			}
			if warningsStr.Valid && warningsStr.String != "" {
				_ = json.Unmarshal([]byte(warningsStr.String), &attempt.Result.Warnings)
			}
			if questionsStr.Valid && questionsStr.String != "" {
				_ = json.Unmarshal([]byte(questionsStr.String), &attempt.Result.Questions)
			}
		}
		attempts = append(attempts, &attempt)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return attempts, nil
}
