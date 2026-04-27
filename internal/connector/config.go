package connector

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const DefaultHeartbeatIntervalSeconds = 15

// SharedInstanceConfig controls which local Codencer instances the connector may expose.
type SharedInstanceConfig struct {
	InstanceID   string `json:"instance_id,omitempty"`
	DaemonURL    string `json:"daemon_url,omitempty"`
	ManifestPath string `json:"manifest_path,omitempty"`
	Share        bool   `json:"share"`
}

// Config is the persisted connector configuration and identity state.
type Config struct {
	RelayURL                 string                 `json:"relay_url"`
	WebsocketURL             string                 `json:"websocket_url,omitempty"`
	ConnectorID              string                 `json:"connector_id,omitempty"`
	MachineID                string                 `json:"machine_id,omitempty"`
	Label                    string                 `json:"label,omitempty"`
	PrivateKey               string                 `json:"private_key,omitempty"`
	PublicKey                string                 `json:"public_key,omitempty"`
	HeartbeatIntervalSeconds int                    `json:"heartbeat_interval_seconds,omitempty"`
	DiscoveryRoots           []string               `json:"discovery_roots,omitempty"`
	Instances                []SharedInstanceConfig `json:"instances,omitempty"`
	DaemonURL                string                 `json:"daemon_url,omitempty"` // compatibility/default single-instance seed
	ConfigPath               string                 `json:"-"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read connector config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("decode connector config: %w", err)
	}
	if cfg.HeartbeatIntervalSeconds <= 0 {
		cfg.HeartbeatIntervalSeconds = DefaultHeartbeatIntervalSeconds
	}
	cfg.ConfigPath = path
	return &cfg, nil
}

func SaveConfig(path string, cfg *Config) error {
	if cfg.HeartbeatIntervalSeconds <= 0 {
		cfg.HeartbeatIntervalSeconds = DefaultHeartbeatIntervalSeconds
	}
	cfg.ConfigPath = path
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}
	clone := *c
	clone.DiscoveryRoots = append([]string(nil), c.DiscoveryRoots...)
	clone.Instances = append([]SharedInstanceConfig(nil), c.Instances...)
	return &clone
}

func StatusPathForConfig(configPath string) string {
	if configPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(configPath), "status.json")
}

func (c *Config) StatusPath() string {
	if c == nil {
		return ""
	}
	return StatusPathForConfig(c.ConfigPath)
}

func (c *Config) UpsertSharedInstance(instance SharedInstanceConfig) {
	for i := range c.Instances {
		if instance.InstanceID != "" && c.Instances[i].InstanceID == instance.InstanceID {
			c.Instances[i] = mergeInstanceConfig(c.Instances[i], instance)
			return
		}
		if instance.DaemonURL != "" && c.Instances[i].DaemonURL == instance.DaemonURL {
			c.Instances[i] = mergeInstanceConfig(c.Instances[i], instance)
			return
		}
		if instance.ManifestPath != "" && c.Instances[i].ManifestPath == instance.ManifestPath {
			c.Instances[i] = mergeInstanceConfig(c.Instances[i], instance)
			return
		}
	}
	c.Instances = append(c.Instances, instance)
}

func mergeInstanceConfig(current, next SharedInstanceConfig) SharedInstanceConfig {
	if next.InstanceID != "" {
		current.InstanceID = next.InstanceID
	}
	if next.DaemonURL != "" {
		current.DaemonURL = next.DaemonURL
	}
	if next.ManifestPath != "" {
		current.ManifestPath = next.ManifestPath
	}
	current.Share = next.Share
	return current
}

func normalizeDiscoveryRoots(configured []string, overrides []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(configured)+len(overrides))
	for _, root := range append(append([]string(nil), configured...), overrides...) {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		cleaned := filepath.Clean(root)
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		out = append(out, cleaned)
	}
	return out
}
