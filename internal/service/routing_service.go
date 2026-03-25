package service

import (
	"context"
	"fmt"
	"log/slog"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/storage/sqlite"
)

// RoutingService manages the smart fallback chaining of adapters based on task profiles and benchmarks.
type RoutingService struct {
	benchmarksRepo *sqlite.BenchmarksRepo
	adapters       map[string]domain.Adapter
}

// NOTE: Current routing implementation is a heuristic-based static fallback chain.
// Benchmarks are logged but not yet actively dynamically evaluated for the primary selection.

func NewRoutingService(benchRepo *sqlite.BenchmarksRepo, adapters map[string]domain.Adapter) *RoutingService {
	return &RoutingService{
		benchmarksRepo: benchRepo,
		adapters:       adapters,
	}
}

// BuildFallbackChain calculates the priority ordered slice of adapter IDs given a target profile.
func (rs *RoutingService) BuildFallbackChain(ctx context.Context, requestedProfile string) ([]string, error) {
	// If a specific adapter was requested, it must be the first in our chain
	chain := []string{}
	if requestedProfile != "" {
		if _, ok := rs.adapters[requestedProfile]; ok {
			chain = append(chain, requestedProfile)
		} else {
			return nil, fmt.Errorf("requested adapter profile '%s' is not registered", requestedProfile)
		}
	}

	// Dynamic inference based on capabilities (for MVP fallback)
	fallbackPreferences := []string{"ide-chat", "qwen", "claude", "codex"}

	for _, id := range fallbackPreferences {
		if id != requestedProfile {
			if _, ok := rs.adapters[id]; ok {
				chain = append(chain, id)
			}
		}
	}

	slog.Info("RoutingService: Constructed execution fallback chain", "primary", requestedProfile, "chain", chain)
	return chain, nil
}

// GetAdapter retrieves the specific adapter instance directly.
func (rs *RoutingService) GetAdapter(profile string) (domain.Adapter, bool) {
	adapter, ok := rs.adapters[profile]
	return adapter, ok
}
