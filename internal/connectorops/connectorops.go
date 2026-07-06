package connectorops

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"agent-bridge/internal/connector"
	"agent-bridge/internal/local"
	"agent-bridge/internal/localexec"
)

// EnrollOptions captures the user-facing connector enrollment inputs used by the
// codencer facade. The low-level connector package still owns enrollment.
type EnrollOptions struct {
	RelayURL        string
	DaemonURL       string
	EnrollmentToken string
	ConfigPath      string
	CodencerHome    string
	Label           string
}

type EnrollReport struct {
	OK                 bool     `json:"ok"`
	Action             string   `json:"action"`
	ConfigPath         string   `json:"config_path"`
	CodencerHome       string   `json:"codencer_home"`
	ConnectorID        string   `json:"connector_id,omitempty"`
	MachineID          string   `json:"machine_id,omitempty"`
	RelayURL           string   `json:"relay_url,omitempty"`
	WebsocketURL       string   `json:"websocket_url,omitempty"`
	DaemonURL          string   `json:"daemon_url,omitempty"`
	LocalConfigUpdated bool     `json:"local_config_updated"`
	Warnings           []string `json:"warnings,omitempty"`
	ExitCode           int      `json:"exit_code"`
}

type StatusReport struct {
	OK         bool              `json:"ok"`
	Action     string            `json:"action"`
	ConfigPath string            `json:"config_path"`
	Status     *connector.Status `json:"status,omitempty"`
	ExitCode   int               `json:"exit_code"`
}

type ConfigReport struct {
	OK          bool              `json:"ok"`
	Action      string            `json:"action"`
	ConfigPath  string            `json:"config_path"`
	ShowSecrets bool              `json:"show_secrets"`
	Config      *connector.Config `json:"config,omitempty"`
	ExitCode    int               `json:"exit_code"`
}

func Enroll(ctx context.Context, opts EnrollOptions) (*EnrollReport, error) {
	configPath, err := ResolveConfigPath(opts.ConfigPath, opts.CodencerHome)
	if err != nil {
		return nil, err
	}
	paths, err := local.ResolvePathsForHome("", "", opts.CodencerHome)
	if err != nil {
		return nil, err
	}

	cfg, err := connector.Enroll(ctx, opts.RelayURL, opts.DaemonURL, opts.EnrollmentToken, opts.Label, configPath)
	if err != nil {
		return nil, err
	}
	cfg.CodencerHome = paths.Home
	if err := connector.SaveConfig(configPath, cfg); err != nil {
		return nil, err
	}
	if err := connector.NewStatusStore(connector.StatusPathForConfig(configPath)).SyncConfig(cfg); err != nil {
		return nil, err
	}

	report := &EnrollReport{
		OK:           true,
		Action:       "connector_enroll",
		ConfigPath:   configPath,
		CodencerHome: paths.Home,
		ConnectorID:  cfg.ConnectorID,
		MachineID:    cfg.MachineID,
		RelayURL:     cfg.RelayURL,
		WebsocketURL: cfg.WebsocketURL,
		DaemonURL:    opts.DaemonURL,
		ExitCode:     localexec.ExitSuccess,
	}
	if err := updateLocalConnectorConfigPath(paths, configPath); err != nil {
		report.Warnings = append(report.Warnings, "local config connector_config_path not updated: "+err.Error())
	} else {
		report.LocalConfigUpdated = true
	}
	return report, nil
}

func LoadStatus(configPath, codencerHome string) (*StatusReport, error) {
	resolved, err := ResolveConfigPath(configPath, codencerHome)
	if err != nil {
		return nil, err
	}
	if _, err := connector.LoadConfig(resolved); err != nil {
		return nil, err
	}
	status, err := connector.LoadStatus(connector.StatusPathForConfig(resolved))
	if err != nil {
		return nil, err
	}
	return &StatusReport{
		OK:         true,
		Action:     "connector_status",
		ConfigPath: resolved,
		Status:     status,
		ExitCode:   localexec.ExitSuccess,
	}, nil
}

func LoadConfig(configPath, codencerHome string, showSecrets bool) (*ConfigReport, error) {
	resolved, err := ResolveConfigPath(configPath, codencerHome)
	if err != nil {
		return nil, err
	}
	cfg, err := connector.LoadConfig(resolved)
	if err != nil {
		return nil, err
	}
	return &ConfigReport{
		OK:          true,
		Action:      "connector_config_show",
		ConfigPath:  resolved,
		ShowSecrets: showSecrets,
		Config:      connector.RedactedConfig(cfg, showSecrets),
		ExitCode:    localexec.ExitSuccess,
	}, nil
}

func Run(ctx context.Context, configPath, codencerHome string) error {
	resolved, err := ResolveConfigPath(configPath, codencerHome)
	if err != nil {
		return err
	}
	cfg, err := connector.LoadConfig(resolved)
	if err != nil {
		return err
	}
	err = connector.NewClient(cfg).Run(ctx)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func ResolveConfigPath(configPath, codencerHome string) (string, error) {
	if configPath != "" {
		return filepath.Abs(configPath)
	}
	paths, err := local.ResolvePathsForHome("", "", codencerHome)
	if err != nil {
		return "", err
	}
	cfg, err := local.LoadConfig(paths.ConfigFile)
	if err == nil && cfg.ConnectorConfigPath != "" {
		return filepath.Abs(cfg.ConnectorConfigPath)
	}
	return filepath.Join(paths.RuntimeDir, "connector", "config.json"), nil
}

func updateLocalConnectorConfigPath(paths local.Paths, configPath string) error {
	if _, err := local.EnsureHome(paths, time.Now().UTC()); err != nil {
		return err
	}
	cfg, err := local.LoadConfig(paths.ConfigFile)
	if err != nil {
		return err
	}
	abs, err := filepath.Abs(configPath)
	if err != nil {
		return err
	}
	cfg.ConnectorConfigPath = abs
	return local.SaveConfig(paths.ConfigFile, cfg)
}
