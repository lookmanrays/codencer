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

	entry := SharedInstanceConfig{
		InstanceID:   selector.InstanceID,
		DaemonURL:    selector.DaemonURL,
		ManifestPath: selector.ManifestPath,
		Share:        true,
	}
	if selector.DaemonURL != "" {
		if clientFactory == nil {
			clientFactory = func(baseURL string) *CodencerClient { return NewCodencerClient(baseURL) }
		}
		if info, err := clientFactory(selector.DaemonURL).GetInstance(ctx); err == nil {
			if entry.InstanceID == "" {
				entry.InstanceID = info.ID
			}
			if entry.ManifestPath == "" {
				entry.ManifestPath = info.ManifestPath
			}
			if entry.DaemonURL == "" {
				entry.DaemonURL = info.BaseURL
			}
		}
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
