package relay

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config controls the self-hosted relay server.
type Config struct {
	Host                     string               `json:"host"`
	Port                     int                  `json:"port"`
	DBPath                   string               `json:"db_path"`
	PlannerToken             string               `json:"planner_token,omitempty"`
	PlannerTokens            []PlannerTokenConfig `json:"planner_tokens,omitempty"`
	EnrollmentSecret         string               `json:"enrollment_secret,omitempty"`
	HeartbeatIntervalSeconds int                  `json:"heartbeat_interval_seconds,omitempty"`
	SessionTTLSeconds        int                  `json:"session_ttl_seconds,omitempty"`
	ChallengeTTLSeconds      int                  `json:"challenge_ttl_seconds,omitempty"`
	ProxyTimeoutSeconds      int                  `json:"proxy_timeout_seconds,omitempty"`
	AllowedOrigins           []string             `json:"allowed_origins,omitempty"`
}

type PlannerTokenConfig struct {
	Name        string   `json:"name,omitempty"`
	Token       string   `json:"token"`
	Scopes      []string `json:"scopes,omitempty"`
	InstanceIDs []string `json:"instance_ids,omitempty"`
}

func DefaultConfig() *Config {
	return &Config{
		Host:                     "127.0.0.1",
		Port:                     8090,
		DBPath:                   ".codencer/relay/relay.db",
		HeartbeatIntervalSeconds: 15,
		SessionTTLSeconds:        45,
		ChallengeTTLSeconds:      30,
		ProxyTimeoutSeconds:      300,
	}
}

func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read relay config: %w", err)
		}
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("decode relay config: %w", err)
		}
	}
	if value := os.Getenv("RELAY_HOST"); value != "" {
		cfg.Host = value
	}
	if value := os.Getenv("RELAY_PORT"); value != "" {
		port, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf("invalid RELAY_PORT %q: %w", value, err)
		}
		cfg.Port = port
	}
	if value := os.Getenv("RELAY_DB_PATH"); value != "" {
		cfg.DBPath = value
	}
	if value := os.Getenv("RELAY_PLANNER_TOKEN"); value != "" {
		cfg.PlannerToken = value
	}
	if value := os.Getenv("RELAY_ENROLLMENT_SECRET"); value != "" {
		cfg.EnrollmentSecret = value
	}
	if value := os.Getenv("RELAY_PROXY_TIMEOUT_SECONDS"); value != "" {
		timeout, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf("invalid RELAY_PROXY_TIMEOUT_SECONDS %q: %w", value, err)
		}
		cfg.ProxyTimeoutSeconds = timeout
	}
	if value := os.Getenv("RELAY_ALLOWED_ORIGINS"); value != "" {
		cfg.AllowedOrigins = splitCSV(value)
	}
	return cfg, cfg.Validate()
}

func (c *Config) Validate() error {
	if c.DBPath == "" {
		return fmt.Errorf("relay db_path is required")
	}
	if len(c.PlannerTokens) == 0 && c.PlannerToken != "" {
		c.PlannerTokens = []PlannerTokenConfig{{
			Name:   "default",
			Token:  c.PlannerToken,
			Scopes: []string{"*"},
		}}
	}
	if len(c.PlannerTokens) == 0 {
		return fmt.Errorf("relay planner_tokens or planner_token is required")
	}
	for _, token := range c.PlannerTokens {
		if token.Token == "" {
			return fmt.Errorf("relay planner token entries must include token")
		}
	}
	if c.HeartbeatIntervalSeconds <= 0 {
		c.HeartbeatIntervalSeconds = 15
	}
	if c.SessionTTLSeconds <= 0 {
		c.SessionTTLSeconds = c.HeartbeatIntervalSeconds * 3
	}
	if c.ChallengeTTLSeconds <= 0 {
		c.ChallengeTTLSeconds = 30
	}
	if c.ProxyTimeoutSeconds <= 0 {
		c.ProxyTimeoutSeconds = 300
	}
	return nil
}

func SaveConfig(path string, cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("relay config is required")
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func splitCSV(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}
