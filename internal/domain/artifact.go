package domain

import (
	"time"
)

// ArtifactType represents the kind of execution evidence.
type ArtifactType string

const (
	ArtifactTypeDiff         ArtifactType = "diff"
	ArtifactTypeStdout       ArtifactType = "stdout"
	ArtifactTypeStderr       ArtifactType = "stderr"
	ArtifactTypeResultJSON   ArtifactType = "result_json"
	ArtifactTypeInputJSON    ArtifactType = "input_json"
	ArtifactTypeChangedFiles ArtifactType = "changed_files"
	ArtifactTypeValidations  ArtifactType = "validations"
)

// Artifact is a deterministic file output from a step attempt.
type Artifact struct {
	ID        string    `json:"id"`
	AttemptID string    `json:"attempt_id"`
	Type      ArtifactType `json:"type"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	Hash      string    `json:"hash,omitempty"`
	MimeType  string    `json:"mime_type,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
