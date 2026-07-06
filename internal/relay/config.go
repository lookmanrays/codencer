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
	Host                       string               `json:"host"`
	Port                       int                  `json:"port"`
	DBPath                     string               `json:"db_path"`
	PlannerToken               string               `json:"planner_token,omitempty"`
	PlannerTokens              []PlannerTokenConfig `json:"planner_tokens,omitempty"`
	EnrollmentSecret           string               `json:"enrollment_secret,omitempty"`
	HeartbeatIntervalSeconds   int                  `json:"heartbeat_interval_seconds,omitempty"`
	SessionTTLSeconds          int                  `json:"session_ttl_seconds,omitempty"`
	ChallengeTTLSeconds        int                  `json:"challenge_ttl_seconds,omitempty"`
	ProxyTimeoutSeconds        int                  `json:"proxy_timeout_seconds,omitempty"`
	AllowedOrigins             []string             `json:"allowed_origins,omitempty"`
	PublicBaseURL              string               `json:"public_base_url,omitempty"`
	OAuthAuthorizationServers  []string             `json:"oauth_authorization_servers,omitempty"`
	OAuthScopesSupported       []string             `json:"oauth_scopes_supported,omitempty"`
	OAuthResourceDocumentation string               `json:"oauth_resource_documentation,omitempty"`
	ChatGPTOAuthDev            OAuthDevConfig       `json:"chatgpt_oauth_dev,omitempty"`
	ChatGPTDevNoAuth           DevNoAuthConfig      `json:"chatgpt_dev_noauth,omitempty"`
	TLSCertFile                string               `json:"tls_cert_file,omitempty"`
	TLSKeyFile                 string               `json:"tls_key_file,omitempty"`
}

type PlannerTokenConfig struct {
	Name        string   `json:"name,omitempty"`
	Token       string   `json:"token"`
	Scopes      []string `json:"scopes,omitempty"`
	InstanceIDs []string `json:"instance_ids,omitempty"`
	ProjectIDs  []string `json:"project_ids,omitempty"`
}

type OAuthDevConfig struct {
	Enabled              bool     `json:"enabled,omitempty"`
	Issuer               string   `json:"issuer,omitempty"`
	ClientID             string   `json:"client_id,omitempty"`
	ClientSecretHash     string   `json:"client_secret_hash,omitempty"`
	OperatorCodeHash     string   `json:"operator_code_hash,omitempty"`
	Scopes               []string `json:"scopes,omitempty"`
	ProjectIDs           []string `json:"project_ids,omitempty"`
	TokenTTLSeconds      int      `json:"token_ttl_seconds,omitempty"`
	AuthorizationCodeTTL int      `json:"authorization_code_ttl_seconds,omitempty"`
}

type DevNoAuthConfig struct {
	Enabled           bool     `json:"enabled,omitempty"`
	AllowRealProjects bool     `json:"allow_real_projects,omitempty"`
	Scopes            []string `json:"scopes,omitempty"`
	ProjectIDs        []string `json:"project_ids,omitempty"`
}

var defaultChatGPTDevReadScopes = []string{"projects:read", "runs:read", "steps:read", "artifacts:read", "reports:read"}

var defaultChatGPTDevWriteScopes = []string{"projects:read", "projects:write", "runs:read", "runs:write", "steps:read", "steps:write", "artifacts:read", "reports:read"}

var defaultChatGPTDevProjectIDs = []string{"fake", "fake-success", "codencer-fake", "chatgpt-fake"}

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
	if value := os.Getenv("RELAY_PUBLIC_BASE_URL"); value != "" {
		cfg.PublicBaseURL = strings.TrimSpace(value)
	}
	if value := os.Getenv("RELAY_OAUTH_AUTHORIZATION_SERVERS"); value != "" {
		cfg.OAuthAuthorizationServers = splitCSV(value)
	}
	if value := os.Getenv("RELAY_OAUTH_SCOPES_SUPPORTED"); value != "" {
		cfg.OAuthScopesSupported = splitCSV(value)
	}
	if value := os.Getenv("RELAY_OAUTH_RESOURCE_DOCUMENTATION"); value != "" {
		cfg.OAuthResourceDocumentation = strings.TrimSpace(value)
	}
	if value := os.Getenv("RELAY_CHATGPT_OAUTH_DEV_ENABLED"); envBool(value) {
		cfg.ChatGPTOAuthDev.Enabled = true
	}
	if value := os.Getenv("RELAY_CHATGPT_OAUTH_DEV_ISSUER"); value != "" {
		cfg.ChatGPTOAuthDev.Issuer = strings.TrimSpace(value)
	}
	if value := os.Getenv("RELAY_CHATGPT_OAUTH_DEV_CLIENT_ID"); value != "" {
		cfg.ChatGPTOAuthDev.ClientID = strings.TrimSpace(value)
	}
	if value := os.Getenv("RELAY_CHATGPT_OAUTH_DEV_CLIENT_SECRET_HASH"); value != "" {
		cfg.ChatGPTOAuthDev.ClientSecretHash = strings.TrimSpace(value)
	}
	if value := os.Getenv("RELAY_CHATGPT_OAUTH_DEV_OPERATOR_CODE_HASH"); value != "" {
		cfg.ChatGPTOAuthDev.OperatorCodeHash = strings.TrimSpace(value)
	}
	if value := os.Getenv("RELAY_CHATGPT_DEV_NOAUTH"); envBool(value) {
		cfg.ChatGPTDevNoAuth.Enabled = true
	}
	if value := os.Getenv("RELAY_TLS_CERT_FILE"); value != "" {
		cfg.TLSCertFile = strings.TrimSpace(value)
	}
	if value := os.Getenv("RELAY_TLS_KEY_FILE"); value != "" {
		cfg.TLSKeyFile = strings.TrimSpace(value)
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
	c.PublicBaseURL = strings.TrimRight(strings.TrimSpace(c.PublicBaseURL), "/")
	c.OAuthAuthorizationServers = cleanConfigList(c.OAuthAuthorizationServers)
	c.OAuthScopesSupported = cleanConfigList(c.OAuthScopesSupported)
	c.OAuthResourceDocumentation = strings.TrimSpace(c.OAuthResourceDocumentation)
	c.ChatGPTOAuthDev.Issuer = strings.TrimRight(strings.TrimSpace(c.ChatGPTOAuthDev.Issuer), "/")
	c.ChatGPTOAuthDev.ClientID = strings.TrimSpace(c.ChatGPTOAuthDev.ClientID)
	c.ChatGPTOAuthDev.ClientSecretHash = strings.TrimSpace(c.ChatGPTOAuthDev.ClientSecretHash)
	c.ChatGPTOAuthDev.OperatorCodeHash = strings.TrimSpace(c.ChatGPTOAuthDev.OperatorCodeHash)
	c.ChatGPTOAuthDev.Scopes = cleanConfigList(c.ChatGPTOAuthDev.Scopes)
	c.ChatGPTOAuthDev.ProjectIDs = cleanConfigList(c.ChatGPTOAuthDev.ProjectIDs)
	if c.ChatGPTOAuthDev.Enabled {
		if c.ChatGPTOAuthDev.Issuer == "" {
			c.ChatGPTOAuthDev.Issuer = c.PublicBaseURL
		}
		if c.ChatGPTOAuthDev.ClientID == "" {
			c.ChatGPTOAuthDev.ClientID = "codencer-chatgpt-dev"
		}
		if c.ChatGPTOAuthDev.ClientSecretHash == "" {
			return fmt.Errorf("chatgpt_oauth_dev.client_secret_hash is required when OAuth dev mode is enabled")
		}
		if c.ChatGPTOAuthDev.OperatorCodeHash == "" {
			return fmt.Errorf("chatgpt_oauth_dev.operator_code_hash is required when OAuth dev mode is enabled")
		}
		if len(c.ChatGPTOAuthDev.Scopes) == 0 {
			c.ChatGPTOAuthDev.Scopes = append([]string(nil), defaultChatGPTDevWriteScopes...)
		}
		if c.ChatGPTOAuthDev.TokenTTLSeconds <= 0 {
			c.ChatGPTOAuthDev.TokenTTLSeconds = 3600
		}
		if c.ChatGPTOAuthDev.AuthorizationCodeTTL <= 0 {
			c.ChatGPTOAuthDev.AuthorizationCodeTTL = 300
		}
		if c.ChatGPTOAuthDev.Issuer != "" && !containsString(c.OAuthAuthorizationServers, c.ChatGPTOAuthDev.Issuer) {
			c.OAuthAuthorizationServers = append(c.OAuthAuthorizationServers, c.ChatGPTOAuthDev.Issuer)
		}
		for _, scope := range c.ChatGPTOAuthDev.Scopes {
			if !containsString(c.OAuthScopesSupported, scope) {
				c.OAuthScopesSupported = append(c.OAuthScopesSupported, scope)
			}
		}
	}
	c.ChatGPTDevNoAuth.Scopes = cleanConfigList(c.ChatGPTDevNoAuth.Scopes)
	c.ChatGPTDevNoAuth.ProjectIDs = cleanConfigList(c.ChatGPTDevNoAuth.ProjectIDs)
	if c.ChatGPTDevNoAuth.Enabled {
		if c.ChatGPTDevNoAuth.AllowRealProjects {
			if len(c.ChatGPTDevNoAuth.Scopes) == 0 {
				c.ChatGPTDevNoAuth.Scopes = append([]string(nil), defaultChatGPTDevWriteScopes...)
			}
		} else {
			if len(c.ChatGPTDevNoAuth.Scopes) == 0 {
				c.ChatGPTDevNoAuth.Scopes = append([]string(nil), defaultChatGPTDevReadScopes...)
			}
			if len(c.ChatGPTDevNoAuth.ProjectIDs) == 0 {
				c.ChatGPTDevNoAuth.ProjectIDs = append([]string(nil), defaultChatGPTDevProjectIDs...)
			}
		}
	}
	c.TLSCertFile = strings.TrimSpace(c.TLSCertFile)
	c.TLSKeyFile = strings.TrimSpace(c.TLSKeyFile)
	if (c.TLSCertFile == "") != (c.TLSKeyFile == "") {
		return fmt.Errorf("tls_cert_file and tls_key_file must be configured together")
	}
	return nil
}

func envBool(value string) bool {
	value = strings.TrimSpace(value)
	return value == "1" || strings.EqualFold(value, "true") || strings.EqualFold(value, "yes")
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
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

func cleanConfigList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}
