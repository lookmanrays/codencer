package relay

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
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
		if port, err := strconv.Atoi(value); err == nil {
			cfg.Port = port
		}
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
	return nil
}
