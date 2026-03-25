package validation

import (
	"encoding/json"
	"fmt"
	"os"

	"agent-bridge/internal/domain"
	"gopkg.in/yaml.v3"
)

// ParseTaskSpec reads a YAML DSL definition into a domain.TaskSpec
func ParseTaskSpec(path string) (*domain.TaskSpec, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read task spec file: %w", err)
	}
	var spec domain.TaskSpec
	if err := yaml.Unmarshal(b, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse task spec yaml: %w", err)
	}
	return &spec, nil
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
