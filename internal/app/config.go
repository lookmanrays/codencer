package app

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config represents the application configuration.
type Config struct {
	LogLevel     string `json:"log_level"`
	DBPath       string `json:"db_path"`
	ArtifactRoot  string `json:"artifact_root"`
	WorkspaceRoot string `json:"workspace_root"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
}

// DefaultConfig defines default values for the daemon configuration.
func DefaultConfig() *Config {
	return &Config{
		LogLevel:     "info",
		DBPath:       "codencer.db",
		ArtifactRoot:  ".artifacts",
		WorkspaceRoot: ".workspace",
		Host:          "127.0.0.1",
		Port:          8080,
	} // MVP values
}

// LoadConfig reads the configuration from the specified file path via JSON.
// If the path is empty or validation fails, it issues a fallback default.
func LoadConfig(path string) (*Config, error) {
	config := DefaultConfig()

	if path == "" {
		return config, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// fallback without error if optional
			return config, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config JSON: %w", err)
	}

	return config, nil
}

// Validate ensures all required configuration invariants are met.
func (c *Config) Validate() error {
	if c.DBPath == "" {
		return fmt.Errorf("db_path configuration is required")
	}
	if c.ArtifactRoot == "" {
		return fmt.Errorf("artifact_root configuration is required")
	}
	if c.WorkspaceRoot == "" {
		return fmt.Errorf("workspace_root configuration is required")
	}
	return nil
}
