package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const redactedSecret = "[redacted]"

type InstanceSelector struct {
	InstanceID   string
	DaemonURL    string
	ManifestPath string
}

func (s InstanceSelector) normalized() InstanceSelector {
	s.InstanceID = strings.TrimSpace(s.InstanceID)
	s.DaemonURL = strings.TrimRight(strings.TrimSpace(s.DaemonURL), "/")
	s.ManifestPath = strings.TrimSpace(s.ManifestPath)
	return s
}

func (s InstanceSelector) Validate() error {
	s = s.normalized()
	if s.InstanceID == "" && s.DaemonURL == "" && s.ManifestPath == "" {
		return fmt.Errorf("one of instance-id, daemon-url, or manifest-path is required")
	}
	return nil
}

func EffectiveSharedInstances(cfg *Config) []SharedInstanceConfig {
	if cfg == nil {
		return nil
	}
	if len(cfg.Instances) > 0 {
		out := make([]SharedInstanceConfig, len(cfg.Instances))
		copy(out, cfg.Instances)
		return out
	}
	if cfg.DaemonURL == "" {
		return nil
	}
	return []SharedInstanceConfig{{
		DaemonURL: strings.TrimRight(cfg.DaemonURL, "/"),
		Share:     true,
	}}
}

func EnsureLegacySharedInstance(cfg *Config) {
	if cfg == nil || len(cfg.Instances) > 0 || cfg.DaemonURL == "" {
		return
	}
	cfg.UpsertSharedInstance(SharedInstanceConfig{
		DaemonURL: strings.TrimRight(cfg.DaemonURL, "/"),
		Share:     true,
	})
}

func ShareInstance(ctx context.Context, cfg *Config, selector InstanceSelector, clientFactory func(string) *CodencerClient) (SharedInstanceConfig, error) {
	if cfg == nil {
		return SharedInstanceConfig{}, fmt.Errorf("connector config is required")
	}
	selector = selector.normalized()
	if err := selector.Validate(); err != nil {
		return SharedInstanceConfig{}, err
	}
	EnsureLegacySharedInstance(cfg)
	entry, err := resolveShareEntry(ctx, cfg, selector, clientFactory)
	if err != nil {
		return SharedInstanceConfig{}, err
	}
	cfg.UpsertSharedInstance(entry)

	index := findSharedInstanceIndex(cfg, selector)
	if index < 0 {
		index = findSharedInstanceIndex(cfg, InstanceSelector{
			InstanceID:   entry.InstanceID,
			DaemonURL:    entry.DaemonURL,
			ManifestPath: entry.ManifestPath,
		})
	}
	if index < 0 {
		return SharedInstanceConfig{}, fmt.Errorf("shared instance entry was not persisted")
	}
	return cfg.Instances[index], nil
}

func resolveShareEntry(ctx context.Context, cfg *Config, selector InstanceSelector, clientFactory func(string) *CodencerClient) (SharedInstanceConfig, error) {
	if clientFactory == nil {
		clientFactory = func(baseURL string) *CodencerClient { return NewCodencerClient(baseURL) }
	}

	entry := SharedInstanceConfig{
		InstanceID:   selector.InstanceID,
		DaemonURL:    selector.DaemonURL,
		ManifestPath: selector.ManifestPath,
		Share:        true,
	}
	if index := findSharedInstanceIndex(cfg, selector); index >= 0 {
		entry = mergeInstanceConfig(cfg.Instances[index], entry)
		entry.Share = true
	}

	registry := NewRegistry(cfg.Clone())
	registry.clientFactory = clientFactory
	manifests, err := registry.DiscoveredManifests()
	if err != nil {
		return SharedInstanceConfig{}, err
	}
	if matched := matchManifest(entry, manifests); matched != nil {
		entry.InstanceID = firstNonEmpty(entry.InstanceID, matched.Info.ID)
		entry.ManifestPath = firstNonEmpty(entry.ManifestPath, matched.Path, matched.Info.ManifestPath)
		entry.DaemonURL = firstNonEmpty(entry.DaemonURL, matched.Info.BaseURL)
	} else if entry.ManifestPath != "" {
		info, loadErr := loadManifest(entry.ManifestPath)
		if loadErr != nil {
			return SharedInstanceConfig{}, fmt.Errorf("load manifest for %s: %w", selectorDescription(selector), loadErr)
		}
		entry.InstanceID = firstNonEmpty(entry.InstanceID, info.ID)
		entry.ManifestPath = firstNonEmpty(entry.ManifestPath, info.ManifestPath)
		entry.DaemonURL = firstNonEmpty(entry.DaemonURL, info.BaseURL)
	}

	entry.DaemonURL = strings.TrimRight(strings.TrimSpace(entry.DaemonURL), "/")
	if entry.DaemonURL == "" {
		return SharedInstanceConfig{}, fmt.Errorf("%s did not resolve to a local daemon url; discover it first or share by --daemon-url", selectorDescription(selector))
	}

	info, err := clientFactory(entry.DaemonURL).GetInstance(ctx)
	if err != nil {
		return SharedInstanceConfig{}, fmt.Errorf("%s is not reachable through daemon %s: %w", selectorDescription(selector), entry.DaemonURL, err)
	}
	if info.ID == "" {
		return SharedInstanceConfig{}, fmt.Errorf("daemon %s did not report an instance id for %s", entry.DaemonURL, selectorDescription(selector))
	}
	if entry.InstanceID != "" && info.ID != entry.InstanceID {
		return SharedInstanceConfig{}, fmt.Errorf("%s resolved to daemon %s, but that daemon reports instance %s", selectorDescription(selector), entry.DaemonURL, info.ID)
	}

	entry.InstanceID = info.ID
	entry.DaemonURL = firstNonEmpty(strings.TrimRight(info.BaseURL, "/"), entry.DaemonURL)
	entry.ManifestPath = firstNonEmpty(info.ManifestPath, entry.ManifestPath)
	entry.Share = true
	return entry, nil
}

func selectorDescription(selector InstanceSelector) string {
	selector = selector.normalized()
	switch {
	case selector.InstanceID != "":
		return fmt.Sprintf("instance %s", selector.InstanceID)
	case selector.ManifestPath != "":
		return fmt.Sprintf("manifest %s", selector.ManifestPath)
	case selector.DaemonURL != "":
		return fmt.Sprintf("daemon %s", selector.DaemonURL)
	default:
		return "instance selector"
	}
}

func UnshareInstance(cfg *Config, selector InstanceSelector) (SharedInstanceConfig, error) {
	if cfg == nil {
		return SharedInstanceConfig{}, fmt.Errorf("connector config is required")
	}
	selector = selector.normalized()
	if err := selector.Validate(); err != nil {
		return SharedInstanceConfig{}, err
	}
	EnsureLegacySharedInstance(cfg)
	index := findSharedInstanceIndex(cfg, selector)
	if index < 0 {
		return SharedInstanceConfig{}, fmt.Errorf("no configured instance matched the selector")
	}
	cfg.Instances[index].Share = false
	return cfg.Instances[index], nil
}

func findSharedInstanceIndex(cfg *Config, selector InstanceSelector) int {
	if cfg == nil {
		return -1
	}
	selector = selector.normalized()
	for i, inst := range cfg.Instances {
		if matchesSharedInstance(inst, selector) {
			return i
		}
	}
	return -1
}

func matchesSharedInstance(inst SharedInstanceConfig, selector InstanceSelector) bool {
	if selector.InstanceID != "" && inst.InstanceID != selector.InstanceID {
		return false
	}
	if selector.DaemonURL != "" && strings.TrimRight(inst.DaemonURL, "/") != selector.DaemonURL {
		return false
	}
	if selector.ManifestPath != "" && inst.ManifestPath != selector.ManifestPath {
		return false
	}
	return selector.InstanceID != "" || selector.DaemonURL != "" || selector.ManifestPath != ""
}

func RedactedConfig(cfg *Config, showSecrets bool) *Config {
	if cfg == nil {
		return nil
	}
	clone := *cfg
	clone.Instances = append([]SharedInstanceConfig(nil), cfg.Instances...)
	if !showSecrets && clone.PrivateKey != "" {
		clone.PrivateKey = redactedSecret
	}
	return &clone
}

func MarshalConfig(cfg *Config, showSecrets bool) ([]byte, error) {
	return json.MarshalIndent(RedactedConfig(cfg, showSecrets), "", "  ")
}
