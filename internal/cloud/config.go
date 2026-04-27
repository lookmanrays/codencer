package cloud

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const defaultDBPath = ".codencer/cloud/cloud.db"

// Config holds the minimal cloud backend configuration for this alpha foundation.
type Config struct {
	Host            string `json:"host"`
	Port            int    `json:"port"`
	DBPath          string `json:"db_path"`
	MasterKey       string `json:"master_key,omitempty"`
	RelayConfigPath string `json:"relay_config_path,omitempty"`
	ConfigPath      string `json:"-"`
}

// DefaultConfig returns the baseline cloud configuration.
func DefaultConfig() *Config {
	return &Config{
		Host:   "127.0.0.1",
		Port:   8190,
		DBPath: defaultDBPath,
	}
}

// LoadConfig reads config from disk when path is set and applies cloud-prefixed env overrides.
func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read cloud config: %w", err)
		}
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("decode cloud config: %w", err)
		}
		cfg.ConfigPath = path
	}

	if value := strings.TrimSpace(os.Getenv("CODENCER_CLOUD_DB_PATH")); value != "" {
		cfg.DBPath = value
	}
	if value := strings.TrimSpace(os.Getenv("CODENCER_CLOUD_HOST")); value != "" {
		cfg.Host = value
	}
	if value := strings.TrimSpace(os.Getenv("CODENCER_CLOUD_PORT")); value != "" {
		port, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf("invalid CODENCER_CLOUD_PORT %q: %w", value, err)
		}
		cfg.Port = port
	}
	if value := strings.TrimSpace(os.Getenv("CODENCER_CLOUD_MASTER_KEY")); value != "" {
		cfg.MasterKey = value
	}
	if value := strings.TrimSpace(os.Getenv("CODENCER_CLOUD_RELAY_CONFIG")); value != "" {
		cfg.RelayConfigPath = value
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate keeps the config intentionally small and explicit.
func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("cloud config is required")
	}
	if strings.TrimSpace(c.DBPath) == "" {
		return fmt.Errorf("cloud db_path is required")
	}
	if strings.TrimSpace(c.Host) == "" {
		return fmt.Errorf("cloud host is required")
	}
	if c.Port <= 0 {
		return fmt.Errorf("cloud port must be greater than zero")
	}
	return nil
}

// ResolveDBDir returns the parent directory for the configured database path.
func (c *Config) ResolveDBDir() string {
	if c == nil || strings.TrimSpace(c.DBPath) == "" {
		return ""
	}
	return filepath.Dir(c.DBPath)
}
