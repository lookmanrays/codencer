package gateway

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"agent-bridge/internal/defaults"
)

const (
	ConfigVersion                     = 1
	DefaultListenAddr                 = "127.0.0.1:19090"
	DefaultGatewayToken               = "CODENCER_GATEWAY_MCP_TOKEN"
	DefaultRelayToken                 = "CODENCER_DEFAULT_RELAY_TOKEN"
	DefaultRelayRequestTimeoutSeconds = 300
)

var DefaultRelayURL = defaults.DefaultRelayURL()

var defaultGatewayDevNoAuthScopes = []string{
	"projects:read",
	"runs:read",
	"steps:read",
	"artifacts:read", "reports:read",
}

type Config struct {
	Version                    int             `json:"version"`
	PublicBaseURL              string          `json:"public_base_url"`
	MCPURL                     string          `json:"mcp_url"`
	ListenAddr                 string          `json:"listen_addr"`
	RelayRequestTimeoutSeconds int             `json:"relay_request_timeout_seconds,omitempty"`
	Store                      StoreConfig     `json:"store,omitempty"`
	DefaultRelay               DefaultRelay    `json:"default_relay,omitempty"`
	Auth                       AuthConfig      `json:"auth"`
	OAuthDev                   OAuthDevConfig  `json:"oauth_dev,omitempty"`
	DevNoAuth                  DevNoAuthConfig `json:"dev_noauth,omitempty"`
	RelayProfiles              []RelayProfile  `json:"relay_profiles,omitempty"`
}

type StoreConfig struct {
	Path string `json:"path,omitempty"`
}

type DefaultRelay struct {
	URL       string `json:"url,omitempty"`
	TokenEnv  string `json:"token_env,omitempty"`
	TokenFile string `json:"token_file,omitempty"`
}

type AuthConfig struct {
	Mode      string `json:"mode"`
	TokenEnv  string `json:"token_env,omitempty"`
	TokenFile string `json:"token_file,omitempty"`
}

type DevNoAuthConfig struct {
	Enabled bool     `json:"enabled,omitempty"`
	Scopes  []string `json:"scopes,omitempty"`
}

type OAuthDevConfig struct {
	Enabled              bool     `json:"enabled,omitempty"`
	Issuer               string   `json:"issuer,omitempty"`
	ClientID             string   `json:"client_id,omitempty"`
	ClientSecretHash     string   `json:"client_secret_hash,omitempty"`
	OperatorCodeHash     string   `json:"operator_code_hash,omitempty"`
	Scopes               []string `json:"scopes,omitempty"`
	TokenTTLSeconds      int      `json:"token_ttl_seconds,omitempty"`
	AuthorizationCodeTTL int      `json:"authorization_code_ttl_seconds,omitempty"`
	RequirePKCE          *bool    `json:"require_pkce,omitempty"`
}

type RelayProfile struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	URL       string `json:"url"`
	TokenEnv  string `json:"token_env,omitempty"`
	TokenFile string `json:"token_file,omitempty"`
	Enabled   bool   `json:"enabled"`
}

type RelayProfileStatus struct {
	ID       string `json:"id"`
	Name     string `json:"name,omitempty"`
	URL      string `json:"url"`
	TokenEnv string `json:"token_env,omitempty"`
	Enabled  bool   `json:"enabled"`
}

func DefaultConfig() *Config {
	return &Config{
		Version:                    ConfigVersion,
		PublicBaseURL:              defaults.DefaultGatewayBaseURL(),
		MCPURL:                     defaults.DefaultGatewayMCPURL(),
		ListenAddr:                 DefaultListenAddr,
		RelayRequestTimeoutSeconds: DefaultRelayRequestTimeoutSeconds,
		DefaultRelay: DefaultRelay{
			URL:      DefaultRelayURL,
			TokenEnv: DefaultRelayToken,
		},
		Auth: AuthConfig{
			Mode:     "bearer-dev",
			TokenEnv: DefaultGatewayToken,
		},
		OAuthDev: OAuthDevConfig{
			Enabled:     true,
			Issuer:      defaults.DefaultGatewayBaseURL(),
			ClientID:    "codencer-chatgpt-dev",
			RequirePKCE: boolPtr(true),
		},
	}
}

func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("gateway config path is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read gateway config: %w", err)
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("decode gateway config: %w", err)
	}
	return cfg, cfg.Validate()
}

func SaveConfig(path string, cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("gateway config is required")
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create gateway config parent: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode gateway config: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0600)
}

func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("gateway config is required")
	}
	if c.Version == 0 {
		c.Version = ConfigVersion
	}
	c.PublicBaseURL = strings.TrimRight(strings.TrimSpace(c.PublicBaseURL), "/")
	c.MCPURL = strings.TrimRight(strings.TrimSpace(c.MCPURL), "/")
	c.ListenAddr = strings.TrimSpace(c.ListenAddr)
	c.Store.Path = strings.TrimSpace(c.Store.Path)
	c.DefaultRelay.URL = strings.TrimRight(strings.TrimSpace(firstNonEmpty(c.DefaultRelay.URL, DefaultRelayURL)), "/")
	c.DefaultRelay.TokenEnv = strings.TrimSpace(firstNonEmpty(c.DefaultRelay.TokenEnv, DefaultRelayToken))
	c.DefaultRelay.TokenFile = strings.TrimSpace(c.DefaultRelay.TokenFile)
	if c.PublicBaseURL == "" {
		return fmt.Errorf("gateway public_base_url is required")
	}
	if c.MCPURL == "" {
		c.MCPURL = c.PublicBaseURL + "/mcp"
	}
	if !strings.HasSuffix(c.MCPURL, "/mcp") {
		c.MCPURL += "/mcp"
	}
	if c.ListenAddr == "" {
		c.ListenAddr = DefaultListenAddr
	}
	if c.RelayRequestTimeoutSeconds <= 0 {
		c.RelayRequestTimeoutSeconds = DefaultRelayRequestTimeoutSeconds
	}
	if err := validateListenAddr(c.ListenAddr); err != nil {
		return err
	}
	if err := validatePublicURL("public_base_url", c.PublicBaseURL); err != nil {
		return err
	}
	if err := validatePublicURL("mcp_url", c.MCPURL); err != nil {
		return err
	}
	if err := validatePublicURL("default_relay.url", c.DefaultRelay.URL); err != nil {
		return err
	}
	if c.Auth.Mode == "" {
		c.Auth.Mode = "bearer-dev"
	}
	if c.Auth.Mode != "bearer-dev" && c.Auth.Mode != "dev-noauth" {
		return fmt.Errorf("unsupported gateway auth.mode %q", c.Auth.Mode)
	}
	if c.Auth.Mode == "dev-noauth" {
		c.DevNoAuth.Enabled = true
		host, _, err := net.SplitHostPort(c.ListenAddr)
		if err == nil && !isLocalHost(host) {
			return fmt.Errorf("auth.mode %q requires listen_addr to bind to a loopback address for safety; got %q", c.Auth.Mode, c.ListenAddr)
		}
	}
	c.DevNoAuth.Scopes = cleanList(c.DevNoAuth.Scopes)
	if c.DevNoAuth.Enabled {
		if len(c.DevNoAuth.Scopes) == 0 {
			c.DevNoAuth.Scopes = append([]string(nil), defaultGatewayDevNoAuthScopes...)
		}
	}
	c.Auth.TokenEnv = strings.TrimSpace(c.Auth.TokenEnv)
	c.Auth.TokenFile = strings.TrimSpace(c.Auth.TokenFile)
	if c.Auth.TokenEnv == "" && c.Auth.TokenFile == "" {
		c.Auth.TokenEnv = DefaultGatewayToken
	}
	if c.OAuthDev.Enabled {
		c.OAuthDev.Issuer = strings.TrimRight(strings.TrimSpace(firstNonEmpty(c.OAuthDev.Issuer, c.PublicBaseURL)), "/")
		c.OAuthDev.ClientID = strings.TrimSpace(firstNonEmpty(c.OAuthDev.ClientID, "codencer-chatgpt-dev"))
		if c.OAuthDev.RequirePKCE == nil {
			c.OAuthDev.RequirePKCE = boolPtr(true)
		}
		c.OAuthDev.Scopes = cleanList(c.OAuthDev.Scopes)
		if len(c.OAuthDev.Scopes) == 0 {
			c.OAuthDev.Scopes = []string{"projects:read", "projects:write", "runs:read", "runs:write", "steps:read", "steps:write", "artifacts:read", "reports:read"}
		}
		if c.OAuthDev.TokenTTLSeconds <= 0 {
			c.OAuthDev.TokenTTLSeconds = 3600
		}
		if c.OAuthDev.AuthorizationCodeTTL <= 0 {
			c.OAuthDev.AuthorizationCodeTTL = 300
		}
	}
	seen := map[string]struct{}{}
	for i := range c.RelayProfiles {
		profile := &c.RelayProfiles[i]
		profile.ID = strings.TrimSpace(profile.ID)
		profile.Name = strings.TrimSpace(profile.Name)
		profile.URL = strings.TrimRight(strings.TrimSpace(profile.URL), "/")
		profile.TokenEnv = strings.TrimSpace(profile.TokenEnv)
		profile.TokenFile = strings.TrimSpace(profile.TokenFile)
		if profile.ID == "" {
			return fmt.Errorf("relay_profiles[%d].id is required", i)
		}
		if _, ok := seen[profile.ID]; ok {
			return fmt.Errorf("duplicate relay profile id %q", profile.ID)
		}
		seen[profile.ID] = struct{}{}
		if profile.Name == "" {
			profile.Name = profile.ID
		}
		if err := validatePublicURL("relay_profiles["+profile.ID+"].url", profile.URL); err != nil {
			return err
		}
		if profile.TokenEnv == "" && profile.TokenFile == "" {
			return fmt.Errorf("relay profile %q requires token_env or token_file", profile.ID)
		}
	}
	sort.Slice(c.RelayProfiles, func(i, j int) bool { return c.RelayProfiles[i].ID < c.RelayProfiles[j].ID })
	return nil
}

func UpsertRelayProfile(cfg *Config, profile RelayProfile) (*Config, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	profile.ID = strings.TrimSpace(profile.ID)
	if profile.ID == "" {
		return nil, fmt.Errorf("relay profile id is required")
	}
	profile.URL = strings.TrimRight(strings.TrimSpace(profile.URL), "/")
	if profile.URL == "" {
		return nil, fmt.Errorf("relay profile url is required")
	}
	if profile.Name == "" {
		profile.Name = profile.ID
	}
	for i := range cfg.RelayProfiles {
		if cfg.RelayProfiles[i].ID == profile.ID {
			cfg.RelayProfiles[i] = profile
			return cfg, cfg.Validate()
		}
	}
	cfg.RelayProfiles = append(cfg.RelayProfiles, profile)
	return cfg, cfg.Validate()
}

func RedactedConfig(cfg *Config) *Config {
	if cfg == nil {
		return nil
	}
	clone := *cfg
	clone.RelayProfiles = make([]RelayProfile, len(cfg.RelayProfiles))
	copy(clone.RelayProfiles, cfg.RelayProfiles)
	clone.DefaultRelay.TokenFile = redactPathToken(clone.DefaultRelay.TokenFile)
	clone.Auth.TokenFile = redactPathToken(clone.Auth.TokenFile)
	clone.OAuthDev.ClientSecretHash = redactedNonEmpty(clone.OAuthDev.ClientSecretHash)
	clone.OAuthDev.OperatorCodeHash = redactedNonEmpty(clone.OAuthDev.OperatorCodeHash)
	for i := range clone.RelayProfiles {
		clone.RelayProfiles[i].TokenFile = redactPathToken(clone.RelayProfiles[i].TokenFile)
	}
	return &clone
}

func RelayStatus(profile RelayProfile) RelayProfileStatus {
	return RelayProfileStatus{
		ID:       profile.ID,
		Name:     profile.Name,
		URL:      profile.URL,
		TokenEnv: profile.TokenEnv,
		Enabled:  profile.Enabled,
	}
}

func (p RelayProfile) Token() (string, error) {
	if env := strings.TrimSpace(p.TokenEnv); env != "" {
		if token := strings.TrimSpace(os.Getenv(env)); token != "" {
			return token, nil
		}
	}
	if file := strings.TrimSpace(p.TokenFile); file != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("read token_file for relay profile %s: %w", p.ID, err)
		}
		if token := strings.TrimSpace(string(data)); token != "" {
			return token, nil
		}
	}
	return "", fmt.Errorf("relay profile %s token is not configured", p.ID)
}

func (a AuthConfig) Token() (string, error) {
	if env := strings.TrimSpace(a.TokenEnv); env != "" {
		if token := strings.TrimSpace(os.Getenv(env)); token != "" {
			return token, nil
		}
	}
	if file := strings.TrimSpace(a.TokenFile); file != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("read gateway token_file: %w", err)
		}
		if token := strings.TrimSpace(string(data)); token != "" {
			return token, nil
		}
	}
	return "", fmt.Errorf("gateway bearer token is not configured")
}

func validateListenAddr(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid listen_addr %q: %w", addr, err)
	}
	if strings.TrimSpace(host) == "" || strings.TrimSpace(port) == "" {
		return fmt.Errorf("listen_addr must include host and port")
	}
	return nil
}

func validatePublicURL(field, value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%s must be an absolute URL", field)
	}
	switch parsed.Scheme {
	case "https":
		return nil
	case "http":
		if isLocalHost(parsed.Hostname()) {
			return nil
		}
		return fmt.Errorf("%s must use https unless it targets localhost for dev/test", field)
	default:
		return fmt.Errorf("%s must use http or https", field)
	}
}

func isLocalHost(host string) bool {
	host = strings.ToLower(strings.Trim(host, "[]"))
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func cleanList(values []string) []string {
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func boolPtr(value bool) *bool {
	return &value
}

func redactedNonEmpty(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return "<redacted>"
}

func redactPathToken(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return "<redacted-token-file>"
}
