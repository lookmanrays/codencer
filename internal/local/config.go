package local

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"agent-bridge/internal/project"
)

const ConfigVersion = 1

type Config struct {
	Version             int               `json:"version"`
	DefaultDaemonURL    string            `json:"default_daemon_url"`
	RelayConfigPath     string            `json:"relay_config_path,omitempty"`
	ConnectorConfigPath string            `json:"connector_config_path,omitempty"`
	GatewayConfigPath   string            `json:"gateway_config_path,omitempty"`
	Runtime             RuntimeConfig     `json:"runtime,omitempty"`
	BinaryPaths         map[string]string `json:"binary_paths,omitempty"`
	CreatedAt           time.Time         `json:"created_at"`
	UpdatedAt           time.Time         `json:"updated_at"`
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
	if cfg.CreatedAt.IsZero() {
		cfg.CreatedAt = time.Now().UTC()
	}
	if cfg.UpdatedAt.IsZero() {
		cfg.UpdatedAt = cfg.CreatedAt
	}
	return writeJSONAtomic(path, cfg, 0600)
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
