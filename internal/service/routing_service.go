package service

import (
	"context"
	"fmt"
	"log/slog"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/storage/sqlite"
)

// RoutingService manages the order in which adapters are attempted.
type RoutingService struct {
	benchmarksRepo *sqlite.BenchmarksRepo
	adapters       map[string]domain.Adapter
}

// NOTE: Current routing implementation is a HEURISTIC STATIC FALLBACK CHAIN.
// While benchmarks are persisted to the database, they are NOT yet used for dynamic 
// selection or weight-based routing. Performance-based routing is a legacy roadmap item.

func NewRoutingService(benchRepo *sqlite.BenchmarksRepo, adapters map[string]domain.Adapter) *RoutingService {
	return &RoutingService{
		benchmarksRepo: benchRepo,
		adapters:       adapters,
	}
}

// BuildHeuristicChain calculates the priority ordered slice of adapter IDs given a target profile.
// NOTE: This currently uses a STATIC hardcoded fallback chain and does NOT yet 
// dynamically evaluate benchmarks for routing decisions.
func (rs *RoutingService) BuildHeuristicChain(ctx context.Context, requestedProfile string) ([]string, error) {
	// If a specific adapter was requested, it must be the first in our chain
	chain := []string{}
	if requestedProfile != "" {
		if _, ok := rs.adapters[requestedProfile]; ok {
			chain = append(chain, requestedProfile)
		} else {
			return nil, fmt.Errorf("requested adapter profile '%s' is not registered", requestedProfile)
		}
	}

	// Heuristic preferences (Static Fallback)
	fallbackPreferences := []string{"ide-chat", "qwen", "claude", "codex"}

	for _, id := range fallbackPreferences {
		if id != requestedProfile {
			if _, ok := rs.adapters[id]; ok {
				chain = append(chain, id)
			}
		}
	}

	slog.Info("RoutingService: Using Heuristic Static Fallback", "primary", requestedProfile, "chain", chain)
	return chain, nil
}

// GetAdapter retrieves the specific adapter instance directly.
func (rs *RoutingService) GetAdapter(profile string) (domain.Adapter, bool) {
	adapter, ok := rs.adapters[profile]
	return adapter, ok
}
