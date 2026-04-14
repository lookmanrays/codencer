package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"agent-bridge/internal/domain"
)

// ArtifactsRepo provides persistence logic for domain.Artifact.
type ArtifactsRepo struct {
	db *sql.DB
}

func NewArtifactsRepo(db *sql.DB) *ArtifactsRepo {
	return &ArtifactsRepo{db: db}
}

// Create inserts a new artifact record into the database.
func (r *ArtifactsRepo) Create(ctx context.Context, artifact *domain.Artifact) error {
	q := `INSERT INTO artifacts (id, attempt_id, type, name, path, size, hash, mime_type, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, q, artifact.ID, artifact.AttemptID, string(artifact.Type), artifact.Name, artifact.Path, artifact.Size, artifact.Hash, artifact.MimeType, artifact.CreatedAt, artifact.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create artifact %s: %w", artifact.ID, err)
	}
	return nil
}

// Get retrieves a single artifact by ID.
func (r *ArtifactsRepo) Get(ctx context.Context, id string) (*domain.Artifact, error) {
	q := `SELECT id, attempt_id, type, name, path, size, hash, mime_type, created_at, updated_at FROM artifacts WHERE id = ?`
	row := r.db.QueryRowContext(ctx, q, id)

	var artifact domain.Artifact
	var artifactType string
	if err := row.Scan(
		&artifact.ID,
		&artifact.AttemptID,
		&artifactType,
		&artifact.Name,
		&artifact.Path,
		&artifact.Size,
		&artifact.Hash,
		&artifact.MimeType,
		&artifact.CreatedAt,
		&artifact.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get artifact %s: %w", id, err)
	}

	artifact.Type = domain.ArtifactType(artifactType)
	return &artifact, nil
}

// ListByStep retrieves all artifacts for all attempts of a step.
func (r *ArtifactsRepo) ListByStep(ctx context.Context, stepID string) ([]*domain.Artifact, error) {
	q := `
		SELECT a.id, a.attempt_id, a.type, a.name, a.path, a.size, a.hash, a.mime_type, a.created_at, a.updated_at
		FROM artifacts a
		INNER JOIN attempts att ON a.attempt_id = att.id
		WHERE att.step_id = ?
		ORDER BY a.created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, q, stepID)
	if err != nil {
		return nil, fmt.Errorf("failed to list artifacts for step %s: %w", stepID, err)
	}
	defer rows.Close()

	var artifacts []*domain.Artifact
	for rows.Next() {
		var a domain.Artifact
		var t string
		if err := rows.Scan(&a.ID, &a.AttemptID, &t, &a.Name, &a.Path, &a.Size, &a.Hash, &a.MimeType, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		a.Type = domain.ArtifactType(t)
		artifacts = append(artifacts, &a)
	}
	return artifacts, rows.Err()
}

// ListByAttempt retrieves all artifacts associated with a specific attempt.
func (r *ArtifactsRepo) ListByAttempt(ctx context.Context, attemptID string) ([]*domain.Artifact, error) {
	q := `
		SELECT id, attempt_id, type, name, path, size, hash, mime_type, created_at, updated_at
		FROM artifacts
		WHERE attempt_id = ?
		ORDER BY created_at DESC, updated_at DESC, id DESC
	`
	rows, err := r.db.QueryContext(ctx, q, attemptID)
	if err != nil {
		return nil, fmt.Errorf("failed to list artifacts for attempt %s: %w", attemptID, err)
	}
	defer rows.Close()

	var artifacts []*domain.Artifact
	for rows.Next() {
		var a domain.Artifact
		var t string
		if err := rows.Scan(&a.ID, &a.AttemptID, &t, &a.Name, &a.Path, &a.Size, &a.Hash, &a.MimeType, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		a.Type = domain.ArtifactType(t)
		artifacts = append(artifacts, &a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return artifacts, nil
}
