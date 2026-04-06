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
		INSERT INTO attempts (id, step_id, number, adapter, state, summary, needs_human_decision, warnings, questions, files_changed, raw_output, raw_output_ref, is_simulation, retryable, version, artifacts, provisioning, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	var state string
	var summary string
	var needsDecision bool
	var warningsJSON, questionsJSON, filesJSON []byte
	var rawOutput, rawOutputRef string
	var isSim, retryable bool
	var version string
	var artifactsJSON, provisioningJSON []byte
	
	if attempt.Result != nil {
		state = string(attempt.Result.State)
		summary = attempt.Result.Summary
		needsDecision = attempt.Result.NeedsHumanDecision
		warningsJSON, _ = json.Marshal(attempt.Result.Warnings)
		questionsJSON, _ = json.Marshal(attempt.Result.Questions)
		filesJSON, _ = json.Marshal(attempt.Result.FilesChanged)
		rawOutput = attempt.Result.RawOutput
		rawOutputRef = attempt.Result.RawOutputRef
		isSim = attempt.Result.IsSimulation
		retryable = attempt.Result.Retryable
		version = attempt.Result.Version
		artifactsJSON, _ = json.Marshal(attempt.Result.Artifacts)
		provisioningJSON, _ = json.Marshal(attempt.Result.Provisioning)
	} else {
		state = string(domain.StepStatePending)
	}

	_, err := r.db.ExecContext(ctx, q,
		attempt.ID,
		attempt.StepID,
		attempt.Number,
		attempt.Adapter,
		state,
		summary,
		needsDecision,
		string(warningsJSON),
		string(questionsJSON),
		string(filesJSON),
		rawOutput,
		rawOutputRef,
		isSim,
		retryable,
		version,
		string(artifactsJSON),
		string(provisioningJSON),
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
		SELECT id, step_id, number, adapter, state, summary, needs_human_decision, warnings, questions, files_changed, raw_output, raw_output_ref, is_simulation, retryable, version, artifacts, provisioning, created_at, updated_at
		FROM attempts WHERE id = ?
	`
	row := r.db.QueryRowContext(ctx, q, id)
	
	var attempt domain.Attempt
	var stateStr string
	var resSummary string
	var needsDecision bool
	var warningsStr, questionsStr, filesStr, rawOutput, rawOutputRef, versionStr, artifactsStr, provisioningStr sql.NullString
	var isSim, retryable bool

	err := row.Scan(
		&attempt.ID,
		&attempt.StepID,
		&attempt.Number,
		&attempt.Adapter,
		&stateStr,
		&resSummary,
		&needsDecision,
		&warningsStr,
		&questionsStr,
		&filesStr,
		&rawOutput,
		&rawOutputRef,
		&isSim,
		&retryable,
		&versionStr,
		&artifactsStr,
		&provisioningStr,
		&attempt.CreatedAt,
		&attempt.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil // not found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get attempt: %w", err)
	}

	if stateStr != "" {
		attempt.Result = &domain.ResultSpec{
			State:              domain.StepState(stateStr),
			Summary:            resSummary,
			NeedsHumanDecision: needsDecision,
			RawOutput:          rawOutput.String,
			RawOutputRef:       rawOutputRef.String,
			IsSimulation:       isSim,
			Retryable:          retryable,
			Version:            versionStr.String,
		}
		if artifactsStr.Valid && artifactsStr.String != "" {
			_ = json.Unmarshal([]byte(artifactsStr.String), &attempt.Result.Artifacts)
		}
		if warningsStr.Valid && warningsStr.String != "" {
			_ = json.Unmarshal([]byte(warningsStr.String), &attempt.Result.Warnings)
		}
		if questionsStr.Valid && questionsStr.String != "" {
			_ = json.Unmarshal([]byte(questionsStr.String), &attempt.Result.Questions)
		}
		if filesStr.Valid && filesStr.String != "" {
			_ = json.Unmarshal([]byte(filesStr.String), &attempt.Result.FilesChanged)
		}
		if provisioningStr.Valid && provisioningStr.String != "" {
			_ = json.Unmarshal([]byte(provisioningStr.String), &attempt.Result.Provisioning)
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
		UPDATE attempts SET state = ?, summary = ?, needs_human_decision = ?, warnings = ?, questions = ?, files_changed = ?, raw_output = ?, raw_output_ref = ?, is_simulation = ?, retryable = ?, version = ?, artifacts = ?, provisioning = ?, updated_at = ?
		WHERE id = ?
	`
	warningsJSON, _ := json.Marshal(attempt.Result.Warnings)
	questionsJSON, _ := json.Marshal(attempt.Result.Questions)
	filesJSON, _ := json.Marshal(attempt.Result.FilesChanged)
	artifactsJSON, _ := json.Marshal(attempt.Result.Artifacts)
	provisioningJSON, _ := json.Marshal(attempt.Result.Provisioning)

	_, err := r.db.ExecContext(ctx, q,
		string(attempt.Result.State),
		attempt.Result.Summary,
		attempt.Result.NeedsHumanDecision,
		string(warningsJSON),
		string(questionsJSON),
		string(filesJSON),
		attempt.Result.RawOutput,
		attempt.Result.RawOutputRef,
		attempt.Result.IsSimulation,
		attempt.Result.Retryable,
		attempt.Result.Version,
		string(artifactsJSON),
		string(provisioningJSON),
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
		SELECT id, step_id, number, adapter, state, summary, needs_human_decision, warnings, questions, files_changed, raw_output, raw_output_ref, is_simulation, retryable, version, artifacts, provisioning, created_at, updated_at
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
		var stateStr string
		var resSummary string
		var needsDecision bool
		var warningsStr, questionsStr, filesStr, rawOutput, rawOutputRef, versionStr, artifactsStr, provisioningStr sql.NullString
		var isSim, retryable bool

		if err := rows.Scan(
			&attempt.ID, &attempt.StepID, &attempt.Number, &attempt.Adapter,
			&stateStr, &resSummary, &needsDecision, &warningsStr, &questionsStr,
			&filesStr, &rawOutput, &rawOutputRef, &isSim, &retryable,
			&versionStr, &artifactsStr, &provisioningStr,
			&attempt.CreatedAt, &attempt.UpdatedAt,
		); err != nil {
			return nil, err
		}

		if stateStr != "" && stateStr != string(domain.StepStatePending) {
			attempt.Result = &domain.ResultSpec{
				State:              domain.StepState(stateStr),
				Summary:            resSummary,
				NeedsHumanDecision: needsDecision,
				RawOutput:          rawOutput.String,
				RawOutputRef:       rawOutputRef.String,
				IsSimulation:       isSim,
				Retryable:          retryable,
				Version:            versionStr.String,
			}
			if artifactsStr.Valid && artifactsStr.String != "" {
				_ = json.Unmarshal([]byte(artifactsStr.String), &attempt.Result.Artifacts)
			}
			if warningsStr.Valid && warningsStr.String != "" {
				_ = json.Unmarshal([]byte(warningsStr.String), &attempt.Result.Warnings)
			}
			if questionsStr.Valid && questionsStr.String != "" {
				_ = json.Unmarshal([]byte(questionsStr.String), &attempt.Result.Questions)
			}
			if filesStr.Valid && filesStr.String != "" {
				_ = json.Unmarshal([]byte(filesStr.String), &attempt.Result.FilesChanged)
			}
			if provisioningStr.Valid && provisioningStr.String != "" {
				_ = json.Unmarshal([]byte(provisioningStr.String), &attempt.Result.Provisioning)
			}
		}
		attempts = append(attempts, &attempt)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return attempts, nil
}

// GetLatestByStep retrieves the latest attempt (highest number) for a specific step.
func (r *AttemptsRepo) GetLatestByStep(ctx context.Context, stepID string) (*domain.Attempt, error) {
	const q = `
		SELECT id, step_id, number, adapter, state, summary, needs_human_decision, warnings, questions, files_changed, raw_output, raw_output_ref, is_simulation, retryable, version, artifacts, provisioning, created_at, updated_at
		FROM attempts WHERE step_id = ? ORDER BY number DESC LIMIT 1
	`
	row := r.db.QueryRowContext(ctx, q, stepID)
	
	var attempt domain.Attempt
	var stateStr string
	var resSummary string
	var needsDecision bool
	var warningsStr, questionsStr, filesStr, rawOutput, rawOutputRef, versionStr, artifactsStr, provisioningStr sql.NullString
	var isSim, retryable bool

	err := row.Scan(
		&attempt.ID, &attempt.StepID, &attempt.Number, &attempt.Adapter,
		&stateStr, &resSummary, &needsDecision, &warningsStr, &questionsStr,
		&filesStr, &rawOutput, &rawOutputRef, &isSim, &retryable,
		&versionStr, &artifactsStr, &provisioningStr,
		&attempt.CreatedAt, &attempt.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil // not found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get latest attempt: %w", err)
	}

	if stateStr != "" && stateStr != string(domain.StepStatePending) {
		attempt.Result = &domain.ResultSpec{
			State:              domain.StepState(stateStr),
			Summary:            resSummary,
			NeedsHumanDecision: needsDecision,
			RawOutput:          rawOutput.String,
			RawOutputRef:       rawOutputRef.String,
			IsSimulation:       isSim,
			Retryable:          retryable,
			Version:            versionStr.String,
		}
		if artifactsStr.Valid && artifactsStr.String != "" {
			_ = json.Unmarshal([]byte(artifactsStr.String), &attempt.Result.Artifacts)
		}
		if warningsStr.Valid && warningsStr.String != "" {
			_ = json.Unmarshal([]byte(warningsStr.String), &attempt.Result.Warnings)
		}
		if questionsStr.Valid && questionsStr.String != "" {
			_ = json.Unmarshal([]byte(questionsStr.String), &attempt.Result.Questions)
		}
		if filesStr.Valid && filesStr.String != "" {
			_ = json.Unmarshal([]byte(filesStr.String), &attempt.Result.FilesChanged)
		}
		if provisioningStr.Valid && provisioningStr.String != "" {
			_ = json.Unmarshal([]byte(provisioningStr.String), &attempt.Result.Provisioning)
		}
	}

	return &attempt, nil
}
