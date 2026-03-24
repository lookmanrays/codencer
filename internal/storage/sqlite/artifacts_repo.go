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
	q := `INSERT INTO artifacts (id, attempt_id, type, path, size, created_at) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, q, artifact.ID, artifact.AttemptID, string(artifact.Type), artifact.Path, artifact.Size, artifact.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create artifact %s: %w", artifact.ID, err)
	}
	return nil
}

// ListByAttempt retrieves all artifacts associated with a specific attempt.
func (r *ArtifactsRepo) ListByAttempt(ctx context.Context, attemptID string) ([]*domain.Artifact, error) {
	q := `SELECT id, attempt_id, type, path, size, created_at FROM artifacts WHERE attempt_id = ?`
	rows, err := r.db.QueryContext(ctx, q, attemptID)
	if err != nil {
		return nil, fmt.Errorf("failed to list artifacts for attempt %s: %w", attemptID, err)
	}
	defer rows.Close()

	var artifacts []*domain.Artifact
	for rows.Next() {
		var a domain.Artifact
		var t string
		if err := rows.Scan(&a.ID, &a.AttemptID, &t, &a.Path, &a.Size, &a.CreatedAt); err != nil {
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
