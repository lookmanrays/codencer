package setup

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"agent-bridge/internal/connector"
	"agent-bridge/internal/domain"
	"agent-bridge/internal/gateway"
	"agent-bridge/internal/local"
	"agent-bridge/internal/localexec"
	manifestpkg "agent-bridge/internal/manifest"
	"agent-bridge/internal/mcpconfig"
	"agent-bridge/internal/project"
	"agent-bridge/internal/readiness"
	"agent-bridge/internal/relay"
	"agent-bridge/internal/security"
	"agent-bridge/internal/supervisor"
)

type Step struct {
	ID            string   `json:"id"`
	Status        string   `json:"status"`
	Detail        string   `json:"detail,omitempty"`
	ObservedFacts []string `json:"observed_facts,omitempty"`
}

type Report struct {
	OK           bool         `json:"ok"`
	Mode         string       `json:"mode"`
	Configured   bool         `json:"configured"`
	StartedAt    time.Time    `json:"started_at"`
	CompletedAt  time.Time    `json:"completed_at"`
	Steps        []Step       `json:"steps"`
	Paths        *local.Paths `json:"paths,omitempty"`
	Reports      []string     `json:"reports,omitempty"`
	NextCommands []string     `json:"next_commands,omitempty"`
	Output       any          `json:"output,omitempty"`
	ExitCode     int          `json:"exit_code"`
}

type LocalOptions struct {
	ProjectID       string
	RepoRoot        string
	Adapter         string
	AdapterProfile  string
	InstallServices bool
	StartServices   bool
	Manager         string
	BinDir          string
	Strict          bool
	Now             func() time.Time
}

type RelayOptions struct {
	BaseURL                      string
	MCPURL                       string
	RelayConfigPath              string
	ConnectorConfigPath          string
	PlannerToken                 string
	GeneratePlannerToken         bool
	PlannerTokenEnv              string
	EnableChatGPTOAuthDev        bool
	OAuthIssuer                  string
	OAuthClientID                string
	OAuthClientSecret            string
	ChatGPTDevNoAuth             bool
	AllowRealProjectsInDevNoAuth bool
	InstallServices              bool
	StartServices                bool
	Manager                      string
	BinDir                       string
	Strict                       bool
	Now                          func() time.Time
}

type GatewayOptions struct {
	BaseURL           string
	MCPURL            string
	ListenAddr        string
	GatewayConfigPath string
	AuthMode          string
	TokenEnv          string
	TokenFile         string
	EnableOAuthDev    bool
	OAuthIssuer       string
	OAuthClientID     string
	OAuthClientSecret string
	InstallServices   bool
	StartServices     bool
	Manager           string
	BinDir            string
	Strict            bool
	Now               func() time.Time
}

type MCPOptions struct {
	Client   string
	Endpoint string
	TokenEnv string
	Token    string
	Name     string
	Now      func() time.Time
}

type DemoOptions struct {
	BinDir string
	Keep   bool
	Now    func() time.Time
}

func Local(ctx context.Context, opts LocalOptions) (Report, error) {
	started := now(opts.Now)
	repo, err := repoRoot(opts.RepoRoot)
	if err != nil {
		return Report{}, err
	}
	paths, err := local.ResolvePaths(repo, "")
	if err != nil {
		return Report{}, err
	}
	report := baseReport("local", started, &paths)
	initResult, err := local.EnsureHome(paths, started)
	if err != nil {
		report.add("init_home", "failed", err.Error())
		return report.finish(localexec.ExitInternal), nil
	}
	report.add("init_home", "passed", "local production home is initialized", paths.Home)
	if initResult.ConfigCreated {
		report.add("config_created", "passed", paths.ConfigFile)
	}
	if initResult.RegistryCreated {
		report.add("registry_created", "passed", paths.ProjectsFile)
	}

	for _, binary := range []string{"codencer", "orchestratord", "codencer-relayd", "codencer-connectord"} {
		if resolved := resolveBinary(opts.BinDir, repo, binary); resolved != "" {
			report.add("binary_"+binary, "passed", resolved)
		} else {
			report.add("binary_"+binary, "not_configured", "binary not found")
		}
	}

	if strings.TrimSpace(opts.ProjectID) != "" {
		adapter := firstNonEmpty(opts.Adapter, "codex")
		profile := firstNonEmpty(opts.AdapterProfile, defaultProfile(adapter))
		cfg, _ := local.LoadConfig(paths.ConfigFile)
		machine, _, _ := local.EnsureMachine(paths.MachineFile, started)
		next, warnings, err := project.NewProject(project.ProjectOptions{
			ID:             opts.ProjectID,
			RepoRoot:       repo,
			DefaultAdapter: adapter,
			AdapterProfile: profile,
			DaemonURL:      cfg.DefaultDaemonURL,
			MachineID:      machine.MachineID,
			HostLabel:      machine.HostLabel,
			Hostname:       machine.Hostname,
		})
		if err != nil {
			report.add("project_register", "failed", err.Error())
			return report.finish(localexec.ExitInvalidInput), nil
		}
		registry, err := project.LoadRegistry(paths.ProjectsFile)
		if err != nil {
			return Report{}, err
		}
		if _, err := project.UpsertProject(registry, next, true, started); err != nil {
			report.add("project_register", "failed", err.Error())
			return report.finish(localexec.ExitInvalidInput), nil
		}
		if err := project.SaveRegistry(paths.ProjectsFile, registry); err != nil {
			return Report{}, err
		}
		report.add("project_register", "passed", next.ID, warnings...)
	}

	if opts.InstallServices {
		svc, err := supervisor.Service(ctx, "install", supervisor.Options{All: true, RepoRoot: repo, Manager: opts.Manager, BinDir: opts.BinDir, Strict: opts.Strict})
		if err != nil {
			report.add("service_install", "failed", err.Error())
		} else {
			report.Reports = append(report.Reports, "service_install")
			report.add("service_install", statusForOK(svc.OK), fmt.Sprintf("services=%d", len(svc.Services)))
		}
	} else {
		report.add("service_install", "skipped", "pass --install-services to install user services")
	}
	if opts.StartServices {
		svc, err := supervisor.Service(ctx, "start", supervisor.Options{All: true, RepoRoot: repo, Manager: opts.Manager, BinDir: opts.BinDir, Strict: opts.Strict})
		if err != nil {
			report.add("service_start", "failed", err.Error())
		} else {
			report.add("service_start", statusForOK(svc.OK), fmt.Sprintf("services=%d", len(svc.Services)))
		}
	} else {
		report.add("service_start", "skipped", "pass --start-services to start user services")
	}

	cfg, err := local.LoadConfig(paths.ConfigFile)
	if err != nil {
		return Report{}, err
	}
	doctor := local.BuildDoctorReport(local.DoctorOptions{Paths: paths, Config: cfg, RepoRoot: repo, Strict: opts.Strict})
	report.add("doctor", statusForOptionalOK(doctor.OK), fmt.Sprintf("errors=%d warnings=%d", doctor.Summary.Errors, doctor.Summary.Warnings))
	watchdog, err := supervisor.WatchdogOnce(ctx, supervisor.Options{RepoRoot: repo, Manager: firstNonEmpty(opts.Manager, supervisor.ManagerManual), BinDir: opts.BinDir, Strict: false})
	if err != nil {
		report.add("watchdog", "failed", err.Error())
	} else {
		report.add("watchdog", statusForOptionalOK(watchdog.OK), fmt.Sprintf("blockers=%d", len(watchdog.Blockers)))
	}
	ready, err := readiness.Build(ctx, readiness.Options{Local: true, RepoRoot: repo})
	if err != nil {
		report.add("readiness", "failed", err.Error())
	} else {
		report.Reports = append(report.Reports, ready.ReportPath)
		report.add("readiness", statusForVerdict(ready.Verdict), ready.Verdict)
	}
	report.Configured = true
	report.NextCommands = []string{
		"codencer project list --json",
		"codencer service status --all --json",
		"codencer readiness --json",
	}
	return report.finish(exitForSteps(report.Steps, opts.Strict)), nil
}

func Relay(ctx context.Context, opts RelayOptions) (Report, error) {
	started := now(opts.Now)
	paths, err := local.ResolvePaths("", "")
	if err != nil {
		return Report{}, err
	}
	report := baseReport("relay", started, &paths)
	if _, err := local.EnsureHome(paths, started); err != nil {
		report.add("init_home", "failed", err.Error())
		return report.finish(localexec.ExitInternal), nil
	}
	report.add("init_home", "passed", paths.Home)

	relayConfigPath := firstNonEmpty(opts.RelayConfigPath, filepath.Join(paths.RuntimeDir, "relay", "config.json"))
	connectorConfigPath := firstNonEmpty(opts.ConnectorConfigPath, filepath.Join(paths.RuntimeDir, "connector", "config.json"))
	baseURL := strings.TrimRight(firstNonEmpty(opts.BaseURL, "http://127.0.0.1:8090"), "/")
	mcpURL := strings.TrimRight(firstNonEmpty(opts.MCPURL, baseURL+"/mcp"), "/")
	token := strings.TrimSpace(opts.PlannerToken)
	if token == "" {
		token = plannerTokenFromExistingConfig(relayConfigPath)
	}
	if token == "" && opts.GeneratePlannerToken {
		token, err = randomToken()
		if err != nil {
			return Report{}, err
		}
		tokenPath := filepath.Join(paths.TokensDir, "planner-token")
		if err := os.WriteFile(tokenPath, []byte(token+"\n"), 0600); err != nil {
			return Report{}, err
		}
		report.add("planner_token_generated", "passed", tokenPath)
	}
	if token == "" {
		report.Configured = false
		report.add("relay_config", "not_configured", "planner token is required to write a runnable relay config")
		report.NextCommands = []string{
			"codencer setup relay --generate-planner-token --json",
			"codencer-relayd planner-token create --config " + relayConfigPath + " --write-config --json",
		}
		if opts.Strict {
			return report.finish(localexec.ExitInvalidInput), nil
		}
		return report.finish(localexec.ExitSuccess), nil
	}

	cfg := relay.DefaultConfig()
	cfg.DBPath = filepath.Join(paths.RuntimeDir, "relay", "relay.db")
	cfg.PublicBaseURL = strings.TrimSuffix(baseURL, "/")
	cfg.OAuthResourceDocumentation = mcpURL
	cfg.PlannerTokens = []relay.PlannerTokenConfig{{
		Name:   "operator",
		Token:  token,
		Scopes: []string{"admin:read", "projects:read", "projects:write", "runs:read", "runs:write", "steps:read", "steps:write", "artifacts:read", "reports:read", "connectors:read", "connectors:write", "connectors:enroll"},
	}}
	oauthOutput := map[string]any{}
	if opts.EnableChatGPTOAuthDev {
		clientSecret := strings.TrimSpace(opts.OAuthClientSecret)
		if clientSecret == "" {
			clientSecret, err = randomToken()
			if err != nil {
				return Report{}, err
			}
			clientSecretPath := filepath.Join(paths.TokensDir, "chatgpt-oauth-client-secret")
			if err := os.WriteFile(clientSecretPath, []byte(clientSecret+"\n"), 0600); err != nil {
				return Report{}, err
			}
			report.add("chatgpt_oauth_client_secret_generated", "passed", clientSecretPath)
			oauthOutput["client_secret_file"] = clientSecretPath
		} else {
			report.add("chatgpt_oauth_client_secret_supplied", "passed", "literal client secret accepted and redacted from output")
		}
		operatorCode, err := randomToken()
		if err != nil {
			return Report{}, err
		}
		operatorCodePath := filepath.Join(paths.TokensDir, "chatgpt-oauth-operator-code")
		if err := os.WriteFile(operatorCodePath, []byte(operatorCode+"\n"), 0600); err != nil {
			return Report{}, err
		}
		report.add("chatgpt_oauth_operator_code_generated", "passed", operatorCodePath)
		issuer := strings.TrimRight(firstNonEmpty(opts.OAuthIssuer, baseURL), "/")
		cfg.ChatGPTOAuthDev = relay.OAuthDevConfig{
			Enabled:          true,
			Issuer:           issuer,
			ClientID:         firstNonEmpty(opts.OAuthClientID, "codencer-chatgpt-dev"),
			ClientSecretHash: sha256Hex(clientSecret),
			OperatorCodeHash: sha256Hex(operatorCode),
			Scopes:           []string{"projects:read", "projects:write", "runs:read", "runs:write", "steps:read", "steps:write", "artifacts:read", "reports:read"},
			TokenTTLSeconds:  3600,
		}
		cfg.OAuthAuthorizationServers = []string{issuer}
		cfg.OAuthScopesSupported = append([]string(nil), cfg.ChatGPTOAuthDev.Scopes...)
		oauthOutput["enabled"] = true
		oauthOutput["issuer"] = issuer
		oauthOutput["client_id"] = cfg.ChatGPTOAuthDev.ClientID
		oauthOutput["operator_code_file"] = operatorCodePath
	}
	if opts.ChatGPTDevNoAuth {
		cfg.ChatGPTDevNoAuth = relay.DevNoAuthConfig{
			Enabled:           true,
			AllowRealProjects: opts.AllowRealProjectsInDevNoAuth,
		}
		if opts.AllowRealProjectsInDevNoAuth {
			cfg.ChatGPTDevNoAuth.Scopes = []string{"projects:read", "projects:write", "runs:read", "runs:write", "steps:read", "steps:write", "artifacts:read", "reports:read"}
			report.add("chatgpt_dev_noauth", "passed", "dev-noauth enabled with real project write tools; use only on private test relays")
		} else {
			cfg.ChatGPTDevNoAuth.Scopes = []string{"projects:read", "runs:read", "steps:read", "artifacts:read", "reports:read"}
			cfg.ChatGPTDevNoAuth.ProjectIDs = []string{"fake", "fake-success", "codencer-fake", "chatgpt-fake"}
			report.add("chatgpt_dev_noauth", "passed", "dev-noauth enabled read-only for fake/test project ids only")
		}
	}
	if err := relay.SaveConfig(relayConfigPath, cfg); err != nil {
		report.add("relay_config", "failed", err.Error())
		return report.finish(localexec.ExitInvalidInput), nil
	}
	report.add("relay_config", "passed", relayConfigPath)

	connectorCfg := &connector.Config{RelayURL: baseURL, CodencerHome: paths.Home, ConfigPath: connectorConfigPath}
	if err := connector.SaveConfig(connectorConfigPath, connectorCfg); err != nil {
		report.add("connector_config", "failed", err.Error())
		return report.finish(localexec.ExitInternal), nil
	}
	report.add("connector_config", "passed", connectorConfigPath)

	localCfg, err := local.LoadConfig(paths.ConfigFile)
	if err != nil {
		return Report{}, err
	}
	localCfg.RelayConfigPath = relayConfigPath
	localCfg.ConnectorConfigPath = connectorConfigPath
	localCfg.UpdatedAt = started
	if err := local.SaveConfig(paths.ConfigFile, localCfg); err != nil {
		return Report{}, err
	}
	report.add("local_config_updated", "passed", paths.ConfigFile)

	if opts.InstallServices {
		svc, err := supervisor.Service(ctx, "install", supervisor.Options{Service: supervisor.ServiceRelay, RepoRoot: "", Manager: opts.Manager, BinDir: opts.BinDir, Strict: opts.Strict})
		if err != nil {
			report.add("relay_service_install", "failed", err.Error())
		} else {
			report.add("relay_service_install", statusForOK(svc.OK), fmt.Sprintf("services=%d", len(svc.Services)))
		}
	} else {
		report.add("relay_service_install", "skipped", "pass --install-services to install relay service")
	}
	if opts.StartServices {
		svc, err := supervisor.Service(ctx, "start", supervisor.Options{Service: supervisor.ServiceRelay, RepoRoot: "", Manager: opts.Manager, BinDir: opts.BinDir, Strict: opts.Strict})
		if err != nil {
			report.add("relay_service_start", "failed", err.Error())
		} else {
			report.add("relay_service_start", statusForOK(svc.OK), fmt.Sprintf("services=%d", len(svc.Services)))
		}
	} else {
		report.add("relay_service_start", "skipped", "pass --start-services to start relay service")
	}
	report.Configured = true
	report.NextCommands = []string{
		"codencer project share <project-id> --json",
		"codencer setup mcp --client codex --endpoint " + mcpURL + " --json",
		"codencer readiness --relay --json",
	}
	report.Output = map[string]any{
		"base_url":            baseURL,
		"mcp_url":             mcpURL,
		"planner_token":       "<redacted>",
		"planner_token_env":   firstNonEmpty(opts.PlannerTokenEnv, "CODENCER_PLANNER_TOKEN"),
		"relay_config":        relayConfigPath,
		"connector_config":    connectorConfigPath,
		"token_was_generated": opts.GeneratePlannerToken,
		"chatgpt_oauth_dev":   oauthOutput,
		"chatgpt_dev_noauth": map[string]any{
			"enabled":             opts.ChatGPTDevNoAuth,
			"allow_real_projects": opts.AllowRealProjectsInDevNoAuth,
		},
	}
	return report.finish(exitForSteps(report.Steps, opts.Strict)), nil
}

func Gateway(ctx context.Context, opts GatewayOptions) (Report, error) {
	started := now(opts.Now)
	paths, err := local.ResolvePaths("", "")
	if err != nil {
		return Report{}, err
	}
	report := baseReport("gateway", started, &paths)
	if _, err := local.EnsureHome(paths, started); err != nil {
		report.add("init_home", "failed", err.Error())
		return report.finish(localexec.ExitInternal), nil
	}
	report.add("init_home", "passed", paths.Home)

	gatewayConfigPath := firstNonEmpty(opts.GatewayConfigPath, filepath.Join(paths.RuntimeDir, "gateway", "config.json"))
	baseURL := strings.TrimRight(firstNonEmpty(opts.BaseURL, "https://mcp.codencer.dev"), "/")
	mcpURL := strings.TrimRight(firstNonEmpty(opts.MCPURL, baseURL+"/mcp"), "/")
	if !strings.HasSuffix(mcpURL, "/mcp") {
		mcpURL += "/mcp"
	}
	cfg := gateway.DefaultConfig()
	cfg.PublicBaseURL = baseURL
	cfg.MCPURL = mcpURL
	cfg.ListenAddr = firstNonEmpty(opts.ListenAddr, gateway.DefaultListenAddr)
	cfg.Auth.Mode = firstNonEmpty(opts.AuthMode, "bearer-dev")
	cfg.Auth.TokenEnv = firstNonEmpty(opts.TokenEnv, gateway.DefaultGatewayToken)
	cfg.Auth.TokenFile = strings.TrimSpace(opts.TokenFile)
	cfg.OAuthDev.Enabled = opts.EnableOAuthDev
	cfg.OAuthDev.Issuer = strings.TrimRight(firstNonEmpty(opts.OAuthIssuer, baseURL), "/")
	cfg.OAuthDev.ClientID = firstNonEmpty(opts.OAuthClientID, "codencer-chatgpt-dev")

	oauthOutput := map[string]any{"enabled": opts.EnableOAuthDev}
	if opts.EnableOAuthDev {
		clientSecret := strings.TrimSpace(opts.OAuthClientSecret)
		if clientSecret == "" {
			clientSecret, err = randomToken()
			if err != nil {
				return Report{}, err
			}
			clientSecretPath := filepath.Join(paths.TokensDir, "gateway-oauth-client-secret")
			if err := os.WriteFile(clientSecretPath, []byte(clientSecret+"\n"), 0600); err != nil {
				return Report{}, err
			}
			report.add("gateway_oauth_client_secret_generated", "passed", clientSecretPath)
			oauthOutput["client_secret_file"] = clientSecretPath
		} else {
			report.add("gateway_oauth_client_secret_supplied", "passed", "literal client secret accepted and redacted from output")
		}
		operatorCode, err := randomToken()
		if err != nil {
			return Report{}, err
		}
		operatorCodePath := filepath.Join(paths.TokensDir, "gateway-oauth-operator-code")
		if err := os.WriteFile(operatorCodePath, []byte(operatorCode+"\n"), 0600); err != nil {
			return Report{}, err
		}
		report.add("gateway_oauth_operator_code_generated", "passed", operatorCodePath)
		cfg.OAuthDev.ClientSecretHash = sha256Hex(clientSecret)
		cfg.OAuthDev.OperatorCodeHash = sha256Hex(operatorCode)
		cfg.OAuthDev.TokenTTLSeconds = 3600
		cfg.OAuthDev.AuthorizationCodeTTL = 300
		oauthOutput["issuer"] = cfg.OAuthDev.Issuer
		oauthOutput["client_id"] = cfg.OAuthDev.ClientID
		oauthOutput["operator_code_file"] = operatorCodePath
	}

	if err := gateway.SaveConfig(gatewayConfigPath, cfg); err != nil {
		report.add("gateway_config", "failed", err.Error())
		return report.finish(localexec.ExitInvalidInput), nil
	}
	report.add("gateway_config", "passed", gatewayConfigPath)

	localCfg, err := local.LoadConfig(paths.ConfigFile)
	if err != nil {
		return Report{}, err
	}
	localCfg.GatewayConfigPath = gatewayConfigPath
	localCfg.UpdatedAt = started
	if err := local.SaveConfig(paths.ConfigFile, localCfg); err != nil {
		return Report{}, err
	}
	report.add("local_config_updated", "passed", paths.ConfigFile)

	if opts.InstallServices {
		report.add("gateway_service_install", "skipped", "codencer-gatewayd service install is not wired yet; run codencer-gatewayd serve under your supervisor")
	} else {
		report.add("gateway_service_install", "skipped", "pass --install-services after service templates include codencer-gatewayd")
	}
	if opts.StartServices {
		report.add("gateway_service_start", "skipped", "start codencer-gatewayd serve --config "+gatewayConfigPath)
	} else {
		report.add("gateway_service_start", "skipped", "pass --start-services after service templates include codencer-gatewayd")
	}
	report.Configured = true
	report.NextCommands = []string{
		"export " + cfg.Auth.TokenEnv + "=<gateway-client-token>",
		"codencer gateway relay add --id personal --url https://relay.example.com --token-env CODENCER_RELAY_PERSONAL_TOKEN --json",
		"codencer-gatewayd serve --config " + gatewayConfigPath,
		"codencer activation gateway --gateway " + baseURL + " --relay https://relay.example.com --project codencer --token-env " + cfg.Auth.TokenEnv + " --json",
	}
	report.Output = map[string]any{
		"base_url":       baseURL,
		"mcp_url":        mcpURL,
		"listen_addr":    cfg.ListenAddr,
		"auth_mode":      cfg.Auth.Mode,
		"token_env":      cfg.Auth.TokenEnv,
		"gateway_config": gatewayConfigPath,
		"oauth_dev":      oauthOutput,
	}
	_ = ctx
	return report.finish(exitForSteps(report.Steps, opts.Strict)), nil
}

func MCP(opts MCPOptions) (Report, error) {
	started := now(opts.Now)
	report := baseReport("mcp", started, nil)
	endpoint := firstNonEmpty(opts.Endpoint, "https://relay.example.com/mcp")
	payload, err := mcpconfig.Generate(mcpconfig.Options{
		Client:   opts.Client,
		Endpoint: endpoint,
		TokenEnv: opts.TokenEnv,
		Token:    opts.Token,
		Name:     opts.Name,
	})
	if err != nil {
		report.add("mcp_config", "failed", err.Error())
		return report.finish(localexec.ExitInvalidInput), nil
	}
	report.Configured = true
	report.Output = security.RedactJSON(payload)
	report.add("mcp_config", "passed", fmt.Sprintf("client=%s", payload["client"]))
	report.NextCommands = []string{"Review the generated snippet and add it to the selected MCP client only when ready."}
	return report.finish(localexec.ExitSuccess), nil
}

func DemoLocal(ctx context.Context, opts DemoOptions) (Report, error) {
	started := now(opts.Now)
	root, err := os.MkdirTemp("", "codencer-demo-local.")
	if err != nil {
		return Report{}, err
	}
	if !opts.Keep {
		defer os.RemoveAll(root)
	}
	repo := filepath.Join(root, "repo")
	home := filepath.Join(root, "home")
	state := filepath.Join(root, "state")
	for _, dir := range []string{repo, home, state} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return Report{}, err
		}
	}
	_ = os.WriteFile(filepath.Join(repo, "README.md"), []byte("codencer demo\n"), 0644)
	if err := initGitRepo(repo); err != nil {
		return Report{}, err
	}
	paths, err := local.ResolvePathsForHome(repo, "", home)
	if err != nil {
		return Report{}, err
	}
	report := baseReport("demo-local", started, &paths)
	if _, err := local.EnsureHome(paths, started); err != nil {
		report.add("init_home", "failed", err.Error())
		return report.finish(localexec.ExitInternal), nil
	}
	daemon, daemonURL, err := startDaemon(ctx, root, repo, state, opts.BinDir)
	if err != nil {
		report.add("daemon", "failed", err.Error())
		return report.finish(localexec.ExitDaemonFailed), nil
	}
	defer daemon.Process.Kill()
	report.add("daemon", "passed", daemonURL)
	machine, _, _ := local.EnsureMachine(paths.MachineFile, started)
	next, _, err := project.NewProject(project.ProjectOptions{ID: "demo", RepoRoot: repo, DefaultAdapter: "fake", AdapterProfile: "fake-success", DaemonURL: daemonURL, MachineID: machine.MachineID, HostLabel: machine.HostLabel, Hostname: machine.Hostname})
	if err != nil {
		return Report{}, err
	}
	registry, _ := project.LoadRegistry(paths.ProjectsFile)
	_, _ = project.UpsertProject(registry, next, true, started)
	if err := project.SaveRegistry(paths.ProjectsFile, registry); err != nil {
		return Report{}, err
	}
	report.add("project_register", "passed", "demo")
	service := localexec.NewService()
	success, successErr := service.RunPlan(ctx, localexec.RunPlanOptions{
		BaseOptions:  localexec.BaseOptions{ProjectID: "demo", RepoRoot: repo, CodencerHome: home},
		Manifest:     demoManifest("fake-success", "fake-success", nil),
		ManifestName: "demo-success.yaml",
		Wait:         true,
	})
	report.Reports = appendNonEmpty(report.Reports, success.ReportPath)
	report.add("manifest_success", statusForLocalexec(success.ExitCode, successErr), detailForRunPlan(success, successErr))
	blocker, blockerErr := service.RunPlan(ctx, localexec.RunPlanOptions{
		BaseOptions:  localexec.BaseOptions{ProjectID: "demo", RepoRoot: repo, CodencerHome: home},
		Manifest:     demoManifest("fake-blocker", "fake-blocker", nil),
		ManifestName: "demo-blocker.yaml",
		Wait:         true,
	})
	report.Reports = appendNonEmpty(report.Reports, blocker.ReportPath)
	if blockerErr != nil {
		report.add("manifest_blocker", "failed", blockerErr.Error())
	} else if blocker.ExitCode == localexec.ExitBlocked {
		report.add("manifest_blocker", "passed", "structured blocker returned")
	} else {
		report.add("manifest_blocker", "failed", fmt.Sprintf("expected exit 10 got %d", blocker.ExitCode))
	}
	validation, validationErr := service.RunPlan(ctx, localexec.RunPlanOptions{
		BaseOptions:  localexec.BaseOptions{ProjectID: "demo", RepoRoot: repo, CodencerHome: home},
		Manifest:     demoManifest("fake-validation-failure", "fake-success", []domain.ValidationCommand{{Name: "fail-validation", Command: "echo validation failed >&2; exit 7", TimeoutSeconds: 5}}),
		ManifestName: "demo-validation.yaml",
		Wait:         true,
	})
	report.Reports = appendNonEmpty(report.Reports, validation.ReportPath)
	if validationErr != nil {
		report.add("manifest_validation_failure", "failed", validationErr.Error())
	} else if validation.ExitCode == localexec.ExitValidationFailed {
		report.add("manifest_validation_failure", "passed", "validation failure returned exit 21")
	} else {
		report.add("manifest_validation_failure", "failed", fmt.Sprintf("expected exit 21 got %d", validation.ExitCode))
	}
	ready, err := readiness.Build(ctx, readiness.Options{Local: true, CodencerHome: home, RepoRoot: repo})
	if err != nil {
		report.add("readiness", "failed", err.Error())
	} else {
		report.Reports = appendNonEmpty(report.Reports, ready.ReportPath)
		report.add("readiness", statusForVerdict(ready.Verdict), ready.Verdict)
	}
	report.Configured = true
	if opts.Keep {
		report.NextCommands = []string{"CODENCER_HOME=" + home + " codencer project list --json"}
	}
	return report.finish(exitForSteps(report.Steps, false)), nil
}

func baseReport(mode string, started time.Time, paths *local.Paths) Report {
	return Report{OK: true, Mode: mode, StartedAt: started, Paths: paths, ExitCode: localexec.ExitSuccess}
}

func (r *Report) add(id, status, detail string, facts ...string) {
	r.Steps = append(r.Steps, Step{ID: id, Status: status, Detail: security.Redact(detail), ObservedFacts: redactList(facts)})
}

func (r Report) finish(exitCode int) Report {
	r.CompletedAt = time.Now().UTC()
	r.ExitCode = exitCode
	r.OK = exitCode == localexec.ExitSuccess
	r.Output = security.RedactJSON(r.Output)
	return r
}

func exitForSteps(steps []Step, strict bool) int {
	for _, step := range steps {
		if step.Status == "failed" {
			return localexec.ExitInternal
		}
		if strict && (step.Status == "not_configured" || step.Status == "blocked") {
			return localexec.ExitInvalidInput
		}
	}
	return localexec.ExitSuccess
}

func statusForOK(ok bool) string {
	if ok {
		return "passed"
	}
	return "failed"
}

func statusForOptionalOK(ok bool) string {
	if ok {
		return "passed"
	}
	return "not_configured"
}

func statusForVerdict(verdict string) string {
	if verdict == readiness.VerdictNotReady {
		return "failed"
	}
	return "passed"
}

func statusForLocalexec(exitCode int, err error) string {
	if err != nil {
		return "failed"
	}
	if exitCode == localexec.ExitSuccess {
		return "passed"
	}
	return "failed"
}

func detailForRunPlan(report localexec.RunPlanReport, err error) string {
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("status=%s exit_code=%d", report.Status, report.ExitCode)
}

func repoRoot(value string) (string, error) {
	if strings.TrimSpace(value) != "" {
		return filepath.Abs(value)
	}
	return os.Getwd()
}

func resolveBinary(binDir, repoRoot, name string) string {
	candidates := []string{}
	if binDir != "" {
		candidates = append(candidates, filepath.Join(binDir, name))
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), name))
	}
	if repoRoot != "" {
		candidates = append(candidates, filepath.Join(repoRoot, "bin", name))
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(wd, "bin", name))
	}
	candidates = append(candidates, name)
	for _, candidate := range candidates {
		if path, err := exec.LookPath(candidate); err == nil {
			return path
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func plannerTokenFromExistingConfig(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var raw struct {
		PlannerToken  string `json:"planner_token"`
		PlannerTokens []struct {
			Token string `json:"token"`
		} `json:"planner_tokens"`
	}
	if json.Unmarshal(data, &raw) != nil {
		return ""
	}
	if raw.PlannerToken != "" {
		return raw.PlannerToken
	}
	if len(raw.PlannerTokens) > 0 {
		return raw.PlannerTokens[0].Token
	}
	return ""
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "codencer_" + base64.RawURLEncoding.EncodeToString(buf), nil
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func defaultProfile(adapter string) string {
	switch strings.TrimSpace(adapter) {
	case "claude":
		return "claude-default"
	case "fake":
		return "fake-success"
	default:
		return "codex-workspace"
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func now(fn func() time.Time) time.Time {
	if fn != nil {
		return fn().UTC()
	}
	return time.Now().UTC()
}

func appendNonEmpty(values []string, next string) []string {
	if strings.TrimSpace(next) == "" {
		return values
	}
	return append(values, next)
}

func redactList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, security.Redact(value))
	}
	return out
}

func startDaemon(ctx context.Context, root, repo, state, binDir string) (*exec.Cmd, string, error) {
	port, err := freePort()
	if err != nil {
		return nil, "", err
	}
	binary := resolveBinary(binDir, repo, "orchestratord")
	if binary == "" {
		return nil, "", fmt.Errorf("orchestratord binary not found")
	}
	configPath := filepath.Join(root, "daemon.json")
	daemonURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	data, _ := json.Marshal(map[string]any{
		"log_level":      "error",
		"db_path":        filepath.Join(state, "codencer.db"),
		"artifact_root":  filepath.Join(state, "artifacts"),
		"workspace_root": filepath.Join(state, "workspace"),
		"repo_root":      repo,
		"host":           "127.0.0.1",
		"port":           port,
	})
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return nil, "", err
	}
	logPath := filepath.Join(root, "daemon.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, "", err
	}
	cmd := exec.CommandContext(ctx, binary, "--config", configPath, "--repo-root", repo)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = append(os.Environ(), "HOST=127.0.0.1", fmt.Sprintf("PORT=%d", port))
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return nil, "", err
	}
	go func() {
		_ = cmd.Wait()
		_ = logFile.Close()
	}()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if err := probeURL(ctx, daemonURL+"/health"); err == nil {
			return cmd, daemonURL, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = cmd.Process.Kill()
	data, _ = os.ReadFile(logPath)
	return nil, "", fmt.Errorf("daemon did not become healthy: %s", security.Redact(string(data)))
}

func initGitRepo(repo string) error {
	commands := [][]string{
		{"git", "-C", repo, "init", "-q"},
		{"git", "-C", repo, "add", "README.md"},
		{"git", "-C", repo, "-c", "user.name=Codencer", "-c", "user.email=codencer@example.invalid", "commit", "-q", "-m", "initial"},
	}
	for _, args := range commands {
		cmd := exec.Command(args[0], args[1:]...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
		}
	}
	return nil
}

func probeURL(ctx context.Context, rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" {
		return fmt.Errorf("invalid URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 1200 * time.Millisecond}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func freePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("unexpected listener address %s", listener.Addr())
	}
	return addr.Port, nil
}

func demoManifest(name, profile string, validations []domain.ValidationCommand) *manifestpkg.Manifest {
	task := manifestpkg.Task{ID: name, Title: name, Goal: "Run deterministic " + name, Profile: profile, Validations: validations}
	return &manifestpkg.Manifest{
		Version:  manifestpkg.APIVersion,
		Kind:     manifestpkg.Kind,
		Metadata: manifestpkg.Metadata{Name: name},
		Project:  manifestpkg.Project{ID: "demo"},
		Execution: manifestpkg.Execution{
			Adapter: "fake",
			Profile: profile,
		},
		Tasks: []manifestpkg.Task{task},
	}
}
