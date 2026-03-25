package service

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"agent-bridge/internal/domain"
	"gopkg.in/yaml.v3"
)

// PolicyRegistry stores and manages loaded execution policies.
type PolicyRegistry struct {
	policies map[string]*domain.Policy
	mu       sync.RWMutex
}

// NewPolicyRegistry initializes an empty registry.
func NewPolicyRegistry() *PolicyRegistry {
	return &PolicyRegistry{
		policies: make(map[string]*domain.Policy),
	}
}

// Register adds a policy manually.
func (r *PolicyRegistry) Register(p *domain.Policy) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.policies[p.Name] = p
}

// Lookup retrieves a policy by name. Returns DefaultPolicy if not found.
func (r *PolicyRegistry) Lookup(name string) *domain.Policy {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.policies[name]
	if !ok {
		return domain.DefaultPolicy()
	}
	return p
}

// LoadFromDir reads all YAML files in a directory and registers policies.
func (r *PolicyRegistry) LoadFromDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read policy directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || (filepath.Ext(entry.Name()) != ".yaml" && filepath.Ext(entry.Name()) != ".yml") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			slog.Error("Failed to read policy file", "path", path, "error", err)
			continue
		}

		var p domain.Policy
		if err := yaml.Unmarshal(data, &p); err != nil {
			slog.Error("Failed to unmarshal policy", "path", path, "error", err)
			continue
		}

		if p.Name == "" {
			p.Name = strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		}

		r.Register(&p)
		slog.Info("Registered execution policy", "name", p.Name, "path", path)
	}

	return nil
}
