package domain

import (
	"context"
)

// Adapter interface specifies the provider-neutral execution contract.
type Adapter interface {
	Name() string
	Capabilities() []string
	
	// Start begins adapter execution and returns immediately, allowing polling/cancellation
	Start(ctx context.Context, step *Step, attempt *Attempt, workspaceRoot, artifactRoot string) error
	
	// Poll checks the status of the execution
	Poll(ctx context.Context, attemptID string) (bool, error)
	
	// Cancel stops the execution
	Cancel(ctx context.Context, attemptID string) error
	
	// CollectArtifacts gathers the raw process outputs (logs, diffs, etc.)
	CollectArtifacts(ctx context.Context, attemptID string, artifactRoot string) ([]*Artifact, error)
	
	// NormalizeResult parses the raw outputs into a normalized domain Result
	NormalizeResult(ctx context.Context, attemptID string, artifacts []*Artifact) (*ResultSpec, error)
}
