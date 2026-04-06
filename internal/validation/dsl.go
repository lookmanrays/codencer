package validation

import (
	"encoding/json"
	"fmt"

	"agent-bridge/internal/domain"
)

// ParseTaskSpec reads a task definition into a domain.TaskSpec. Positional task
// files remain format-agnostic and accept YAML or JSON.
func ParseTaskSpec(path string) (*domain.TaskSpec, error) {
	spec, _, err := ParseTaskSpecFile(path)
	return spec, err
}

// GenerateResultSpec payload helper for exporting attempt results
func GenerateResultSpec(spec *domain.ResultSpec) ([]byte, error) {
	return json.MarshalIndent(spec, "", "  ")
}

// ParseResultSpec parses JSON output produced by an adapter
func ParseResultSpec(b []byte) (*domain.ResultSpec, error) {
	var spec domain.ResultSpec
	if err := json.Unmarshal(b, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse result spec json: %w", err)
	}
	return &spec, nil
}
