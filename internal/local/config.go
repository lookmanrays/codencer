package local

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent-bridge/internal/defaults"
	"agent-bridge/internal/project"
)

const (
	ConfigVersion        = 1
	DefaultProfileName   = "self-host"
	GatewayURLEnvName    = defaults.GatewayURLEnv
	GatewayMCPURLEnvName = defaults.MCPURLEnv
	RelayURLEnvName      = defaults.RelayURLEnv
	ConsoleURLEnvName    = defaults.ConsoleURLEnv
)

type Config struct {
	Version             int                `json:"version"`
	DefaultDaemonURL    string             `json:"default_daemon_url"`
	ActiveProfile       string             `json:"active_profile,omitempty"`
	Profiles            map[string]Profile `json:"profiles,omitempty"`
	RelayConfigPath     string             `json:"relay_config_path,omitempty"`
	ConnectorConfigPath string             `json:"connector_config_path,omitempty"`
	GatewayConfigPath   string             `json:"gateway_config_path,omitempty"`
	Runtime             RuntimeConfig      `json:"runtime,omitempty"`
	BinaryPaths         map[string]string  `json:"binary_paths,omitempty"`
	CreatedAt           time.Time          `json:"created_at"`
	UpdatedAt           time.Time          `json:"updated_at"`
}

type Profile struct {
	Name       string `json:"name,omitempty"`
	GatewayURL string `json:"gateway_url,omitempty"`
	MCPURL     string `json:"mcp_url,omitempty"`
	RelayURL   string `json:"relay_url,omitempty"`
	ConsoleURL string `json:"console_url,omitempty"`
}

type ResolvedConnection struct {
	Profile     string `json:"profile,omitempty"`
	GatewayURL  string `json:"gateway_url"`
	MCPURL      string `json:"mcp_url"`
	RelayURL    string `json:"relay_url"`
	ConsoleURL  string `json:"console_url"`
	Source      string `json:"source"`
	EnvOverride bool   `json:"env_override"`
}

type RuntimeConfig struct {
	StaleRunningAfter    string `json:"stale_running_after,omitempty"`
	StaleWaitAfter       string `json:"stale_wait_after,omitempty"`
	ServiceHealthTimeout string `json:"service_health_timeout,omitempty"`
	ServiceManager       string `json:"service_manager,omitempty"`
}

type InitResult struct {
	Paths           Paths            `json:"paths"`
	ConfigCreated   bool             `json:"config_created"`
	RegistryCreated bool             `json:"registry_created"`
	MachineCreated  bool             `json:"machine_created"`
	Machine         *MachineIdentity `json:"machine,omitempty"`
	DirsCreated     []string         `json:"dirs_created"`
}

func DefaultConfig(now time.Time) Config {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return Config{
		Version:          ConfigVersion,
		DefaultDaemonURL: defaultDaemonURL,
		ActiveProfile:    DefaultProfileName,
		Profiles: map[string]Profile{
			DefaultProfileName: DefaultProfile(),
		},
		Runtime: RuntimeConfig{
			StaleRunningAfter:    "30m",
			StaleWaitAfter:       "30m",
			ServiceHealthTimeout: "5s",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func LoadConfig(path string) (Config, error) {
	cfg := DefaultConfig(time.Now().UTC())
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return Config{}, fmt.Errorf("read local config: %w", err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode local config: %w", err)
	}
	if cfg.Version == 0 {
		cfg.Version = ConfigVersion
	}
	if cfg.DefaultDaemonURL == "" {
		cfg.DefaultDaemonURL = defaultDaemonURL
	}
	cfg.Runtime = defaultRuntimeConfig(cfg.Runtime)
	cfg = normalizeConfig(cfg)
	return cfg, nil
}

func SaveConfig(path string, cfg Config) error {
	if cfg.Version == 0 {
		cfg.Version = ConfigVersion
	}
	if cfg.DefaultDaemonURL == "" {
		cfg.DefaultDaemonURL = defaultDaemonURL
	}
	cfg.Runtime = defaultRuntimeConfig(cfg.Runtime)
	cfg = normalizeConfig(cfg)
	if cfg.CreatedAt.IsZero() {
		cfg.CreatedAt = time.Now().UTC()
	}
	if cfg.UpdatedAt.IsZero() {
		cfg.UpdatedAt = cfg.CreatedAt
	}
	return writeJSONAtomic(path, cfg, 0600)
}

func DefaultProfile() Profile {
	return Profile{
		Name:       "Self-host local",
		GatewayURL: defaults.DefaultGatewayBaseURL(),
		MCPURL:     defaults.DefaultGatewayMCPURL(),
		RelayURL:   defaults.DefaultRelayURL(),
		ConsoleURL: defaults.DefaultConsoleURL(),
	}
}

func ResolveConnection(cfg Config, gatewayFlag string) ResolvedConnection {
	cfg = normalizeConfig(cfg)
	profileName := cfg.ActiveProfile
	profile := cfg.Profiles[profileName]
	if profileName == "" {
		profileName = DefaultProfileName
		profile = DefaultProfile()
	}
	resolved := ResolvedConnection{
		Profile:    profileName,
		GatewayURL: normalizeGatewayURL(firstNonEmpty(profile.GatewayURL, defaults.DefaultGatewayBaseURL())),
		MCPURL:     normalizeMCPURL(firstNonEmpty(profile.MCPURL, defaults.DefaultGatewayMCPURL())),
		RelayURL:   normalizeURL(firstNonEmpty(profile.RelayURL, defaults.DefaultRelayURL())),
		ConsoleURL: normalizeURL(firstNonEmpty(profile.ConsoleURL, defaults.DefaultConsoleURL())),
		Source:     "profile:" + profileName,
	}
	if envGateway := os.Getenv(GatewayURLEnvName); strings.TrimSpace(envGateway) != "" {
		resolved.GatewayURL = normalizeGatewayURL(envGateway)
		resolved.MCPURL = normalizeMCPURL(firstNonEmpty(os.Getenv(GatewayMCPURLEnvName), resolved.GatewayURL+"/mcp"))
		resolved.Source = "env:" + GatewayURLEnvName
		resolved.EnvOverride = true
	}
	if envMCP := os.Getenv(GatewayMCPURLEnvName); strings.TrimSpace(envMCP) != "" && !resolved.EnvOverride {
		resolved.MCPURL = normalizeMCPURL(envMCP)
		resolved.GatewayURL = normalizeGatewayURL(strings.TrimSuffix(resolved.MCPURL, "/mcp"))
		resolved.Source = "env:" + GatewayMCPURLEnvName
		resolved.EnvOverride = true
	}
	if envRelay := os.Getenv(RelayURLEnvName); strings.TrimSpace(envRelay) != "" {
		resolved.RelayURL = normalizeURL(envRelay)
		resolved.EnvOverride = true
		if !strings.HasPrefix(resolved.Source, "env:") {
			resolved.Source = "env:" + RelayURLEnvName
		}
	}
	if envConsole := os.Getenv(ConsoleURLEnvName); strings.TrimSpace(envConsole) != "" {
		resolved.ConsoleURL = normalizeURL(envConsole)
		resolved.EnvOverride = true
		if !strings.HasPrefix(resolved.Source, "env:") {
			resolved.Source = "env:" + ConsoleURLEnvName
		}
	}
	if strings.TrimSpace(gatewayFlag) != "" {
		resolved.GatewayURL = normalizeGatewayURL(gatewayFlag)
		resolved.MCPURL = normalizeMCPURL(resolved.GatewayURL + "/mcp")
		resolved.Source = "flag:gateway"
	}
	return resolved
}

func UseProfile(cfg Config, name string) (Config, error) {
	cfg = normalizeConfig(cfg)
	name = strings.TrimSpace(name)
	if name == "" {
		return cfg, fmt.Errorf("profile name is required")
	}
	if _, ok := cfg.Profiles[name]; !ok {
		return cfg, fmt.Errorf("profile %q does not exist", name)
	}
	cfg.ActiveProfile = name
	cfg.UpdatedAt = time.Now().UTC()
	return cfg, nil
}

func SetProfileValue(cfg Config, key, value string) (Config, error) {
	cfg = normalizeConfig(cfg)
	key = strings.TrimSpace(strings.ToLower(key))
	value = strings.TrimSpace(value)
	if key == "" || value == "" {
		return cfg, fmt.Errorf("config set requires key and value")
	}
	active := firstNonEmpty(cfg.ActiveProfile, DefaultProfileName)
	profile := cfg.Profiles[active]
	if profile.Name == "" {
		profile.Name = active
	}
	switch key {
	case "gateway.url", "gateway_url":
		profile.GatewayURL = normalizeGatewayURL(value)
		profile.MCPURL = normalizeMCPURL(profile.GatewayURL + "/mcp")
	case "gateway.mcp_url", "gateway.mcp", "mcp.url", "mcp_url":
		profile.MCPURL = normalizeMCPURL(value)
		profile.GatewayURL = normalizeGatewayURL(strings.TrimSuffix(profile.MCPURL, "/mcp"))
	case "relay.url", "relay_url":
		profile.RelayURL = normalizeURL(value)
	case "console.url", "console_url":
		profile.ConsoleURL = normalizeURL(value)
	default:
		return cfg, fmt.Errorf("unsupported config key %q", key)
	}
	cfg.Profiles[active] = profile
	cfg.UpdatedAt = time.Now().UTC()
	return cfg, nil
}

func normalizeConfig(cfg Config) Config {
	if cfg.Version == 0 {
		cfg.Version = ConfigVersion
	}
	if cfg.DefaultDaemonURL == "" {
		cfg.DefaultDaemonURL = defaultDaemonURL
	}
	if cfg.ActiveProfile == "" {
		cfg.ActiveProfile = DefaultProfileName
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]Profile{}
	}
	if _, ok := cfg.Profiles[DefaultProfileName]; !ok {
		cfg.Profiles[DefaultProfileName] = DefaultProfile()
	}
	for name, profile := range cfg.Profiles {
		if profile.Name == "" {
			profile.Name = name
		}
		profile.GatewayURL = normalizeGatewayURL(firstNonEmpty(profile.GatewayURL, defaults.DefaultGatewayBaseURL()))
		profile.MCPURL = normalizeMCPURL(firstNonEmpty(profile.MCPURL, profile.GatewayURL+"/mcp"))
		profile.RelayURL = normalizeURL(firstNonEmpty(profile.RelayURL, defaults.DefaultRelayURL()))
		profile.ConsoleURL = normalizeURL(firstNonEmpty(profile.ConsoleURL, defaults.DefaultConsoleURL()))
		cfg.Profiles[name] = profile
	}
	if _, ok := cfg.Profiles[cfg.ActiveProfile]; !ok {
		cfg.ActiveProfile = DefaultProfileName
	}
	return cfg
}

func normalizeGatewayURL(value string) string {
	value = normalizeURL(value)
	value = strings.TrimSuffix(value, "/mcp")
	return strings.TrimRight(value, "/")
}

func normalizeMCPURL(value string) string {
	value = normalizeURL(value)
	if !strings.HasSuffix(value, "/mcp") {
		value += "/mcp"
	}
	return value
}

func normalizeURL(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func defaultRuntimeConfig(cfg RuntimeConfig) RuntimeConfig {
	if cfg.StaleRunningAfter == "" {
		cfg.StaleRunningAfter = "30m"
	}
	if cfg.StaleWaitAfter == "" {
		cfg.StaleWaitAfter = "30m"
	}
	if cfg.ServiceHealthTimeout == "" {
		cfg.ServiceHealthTimeout = "5s"
	}
	return cfg
}

func EnsureHome(paths Paths, now time.Time) (InitResult, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	result := InitResult{Paths: paths}
	for _, dir := range []struct {
		path string
		perm os.FileMode
	}{
		{paths.Home, 0755},
		{paths.LogsDir, 0755},
		{paths.RuntimeDir, 0755},
		{paths.TokensDir, 0700},
		{paths.ArtifactsDir, 0755},
	} {
		created, err := ensureDir(dir.path, dir.perm)
		if err != nil {
			return result, err
		}
		if created {
			result.DirsCreated = append(result.DirsCreated, dir.path)
		}
	}

	if _, err := os.Stat(paths.ConfigFile); err != nil {
		if !os.IsNotExist(err) {
			return result, fmt.Errorf("inspect local config: %w", err)
		}
		if err := SaveConfig(paths.ConfigFile, DefaultConfig(now)); err != nil {
			return result, err
		}
		result.ConfigCreated = true
	}

	if _, err := os.Stat(paths.ProjectsFile); err != nil {
		if !os.IsNotExist(err) {
			return result, fmt.Errorf("inspect project registry: %w", err)
		}
		if err := project.SaveRegistry(paths.ProjectsFile, project.EmptyRegistry()); err != nil {
			return result, err
		}
		result.RegistryCreated = true
	}

	machine, created, err := EnsureMachine(paths.MachineFile, now)
	if err != nil {
		return result, fmt.Errorf("ensure machine identity: %w", err)
	}
	result.MachineCreated = created
	result.Machine = &machine

	return result, nil
}

func ensureDir(path string, perm os.FileMode) (bool, error) {
	if info, err := os.Stat(path); err == nil {
		if !info.IsDir() {
			return false, fmt.Errorf("%s exists and is not a directory", path)
		}
		if perm == 0700 {
			_ = os.Chmod(path, perm)
		}
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("inspect directory %s: %w", path, err)
	}
	if err := os.MkdirAll(path, perm); err != nil {
		return false, fmt.Errorf("create directory %s: %w", path, err)
	}
	return true, nil
}

func writeJSONAtomic(path string, value any, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode JSON: %w", err)
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*.json")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("replace file: %w", err)
	}
	return nil
}
