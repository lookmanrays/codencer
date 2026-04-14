package connector

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
)

const (
	DiscoverStateShared         = "shared"
	DiscoverStateKnownUnshared  = "known_unshared"
	DiscoverStateDiscoveredOnly = "discovered_only"
)

type DiscoverEntry struct {
	State        string `json:"state"`
	InstanceID   string `json:"instance_id,omitempty"`
	RepoRoot     string `json:"repo_root,omitempty"`
	ManifestPath string `json:"manifest_path,omitempty"`
	DaemonURL    string `json:"daemon_url,omitempty"`
}

func DiscoverInstances(ctx context.Context, cfg *Config, rootOverrides []string, clientFactory func(string) *CodencerClient) ([]DiscoverEntry, error) {
	if cfg == nil {
		cfg = &Config{}
	}
	registryCfg := cfg.Clone()
	registryCfg.DiscoveryRoots = normalizeDiscoveryRoots(cfg.DiscoveryRoots, rootOverrides)
	registry := NewRegistry(registryCfg)
	if clientFactory != nil {
		registry.clientFactory = clientFactory
	}

	manifests, err := registry.DiscoveredManifests()
	if err != nil {
		return nil, err
	}

	discoveries := make([]DiscoverEntry, 0, len(manifests)+len(EffectiveSharedInstances(cfg)))
	matchedManifestPaths := map[string]struct{}{}
	emitted := map[string]struct{}{}

	for _, entry := range EffectiveSharedInstances(cfg) {
		discovery := DiscoverEntry{
			State:        DiscoverStateKnownUnshared,
			InstanceID:   entry.InstanceID,
			ManifestPath: entry.ManifestPath,
			DaemonURL:    entry.DaemonURL,
		}
		if entry.Share {
			discovery.State = DiscoverStateShared
		}

		if matched := matchManifest(entry, manifests); matched != nil {
			matchedManifestPaths[matched.Path] = struct{}{}
			discovery.InstanceID = firstNonEmpty(discovery.InstanceID, matched.Info.ID)
			discovery.RepoRoot = firstNonEmpty(discovery.RepoRoot, matched.Info.RepoRoot)
			discovery.ManifestPath = firstNonEmpty(discovery.ManifestPath, matched.Path, matched.Info.ManifestPath)
			discovery.DaemonURL = firstNonEmpty(discovery.DaemonURL, matched.Info.BaseURL)
		} else if info := resolveDiscoveryLiveInfo(ctx, entry, clientFactory); info != nil {
			discovery.InstanceID = firstNonEmpty(discovery.InstanceID, info.ID)
			discovery.RepoRoot = firstNonEmpty(discovery.RepoRoot, info.RepoRoot)
			discovery.ManifestPath = firstNonEmpty(discovery.ManifestPath, info.ManifestPath)
			discovery.DaemonURL = firstNonEmpty(discovery.DaemonURL, info.BaseURL)
		}

		key := discoverEntryKey(discovery)
		if _, ok := emitted[key]; ok {
			continue
		}
		emitted[key] = struct{}{}
		discoveries = append(discoveries, discovery)
	}

	for _, manifest := range manifests {
		if _, ok := matchedManifestPaths[manifest.Path]; ok {
			continue
		}
		discovery := DiscoverEntry{
			State:        DiscoverStateDiscoveredOnly,
			InstanceID:   manifest.Info.ID,
			RepoRoot:     manifest.Info.RepoRoot,
			ManifestPath: firstNonEmpty(manifest.Path, manifest.Info.ManifestPath),
			DaemonURL:    manifest.Info.BaseURL,
		}
		key := discoverEntryKey(discovery)
		if _, ok := emitted[key]; ok {
			continue
		}
		emitted[key] = struct{}{}
		discoveries = append(discoveries, discovery)
	}

	sort.Slice(discoveries, func(i, j int) bool {
		left := discoverEntrySortKey(discoveries[i])
		right := discoverEntrySortKey(discoveries[j])
		return left < right
	})
	return discoveries, nil
}

func discoverEntrySortKey(entry DiscoverEntry) string {
	priority := map[string]string{
		DiscoverStateShared:         "0",
		DiscoverStateKnownUnshared:  "1",
		DiscoverStateDiscoveredOnly: "2",
	}
	return priority[entry.State] + "|" + strings.ToLower(firstNonEmpty(entry.InstanceID, entry.RepoRoot, entry.ManifestPath, entry.DaemonURL))
}

func discoverEntryKey(entry DiscoverEntry) string {
	return strings.Join([]string{
		entry.State,
		firstNonEmpty(entry.InstanceID, "-"),
		firstNonEmpty(filepath.Clean(entry.ManifestPath), "-"),
		firstNonEmpty(filepath.Clean(entry.RepoRoot), "-"),
		firstNonEmpty(entry.DaemonURL, "-"),
	}, "|")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
