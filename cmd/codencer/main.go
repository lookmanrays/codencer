package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"agent-bridge/internal/acceptance"
	"agent-bridge/internal/account"
	"agent-bridge/internal/activation"
	"agent-bridge/internal/app"
	"agent-bridge/internal/buildinfo"
	"agent-bridge/internal/connector"
	"agent-bridge/internal/connectorops"
	gatewaypkg "agent-bridge/internal/gateway"
	"agent-bridge/internal/live"
	"agent-bridge/internal/local"
	"agent-bridge/internal/localexec"
	projectpkg "agent-bridge/internal/project"
	"agent-bridge/internal/projectconfig"
	"agent-bridge/internal/proof"
	"agent-bridge/internal/readiness"
	setuppkg "agent-bridge/internal/setup"
	"agent-bridge/internal/supervisor"
)

const (
	exitSuccess = localexec.ExitSuccess
	exitUsage   = localexec.ExitInvalidInput
	exitFailed  = localexec.ExitInternal
)

type exitError struct {
	code    int
	message string
	printed bool
}

func (e exitError) Error() string { return e.message }

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		var ee exitError
		if errors.As(err, &ee) {
			if !ee.printed && ee.message != "" {
				fmt.Fprintln(os.Stderr, ee.message)
			}
			os.Exit(ee.code)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exitFailed)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printUsage(stderr)
		return exitError{code: exitUsage, message: "missing command", printed: true}
	}

	switch args[0] {
	case "version":
		return runVersion(args[1:], stdout)
	case "init":
		return runInit(args[1:], stdout)
	case "login":
		return runLogin(args[1:], stdout)
	case "whoami":
		return runWhoami(args[1:], stdout)
	case "logout":
		return runLogout(args[1:], stdout)
	case "paths":
		return runPaths(args[1:], stdout)
	case "config":
		return runConfig(args[1:], stdout)
	case "doctor":
		return runDoctor(args[1:], stdout)
	case "status":
		return runStatus(args[1:], stdout)
	case "project":
		return runProject(args[1:], stdout)
	case "machine":
		return runMachine(args[1:], stdout)
	case "connector":
		return runConnector(args[1:], stdout)
	case "gateway":
		return runGateway(args[1:], stdout)
	case "run":
		return runRun(args[1:], stdout)
	case "submit":
		return runSubmit(args[1:], stdout)
	case "run-plan":
		return runRunPlan(args[1:], stdout)
	case "profile":
		return runProfile(args[1:], stdout)
	case "service":
		return runService(args[1:], stdout)
	case "watchdog":
		return runWatchdog(args[1:], stdout)
	case "recover":
		return runRecover(args[1:], stdout)
	case "live":
		return runLive(args[1:], stdout)
	case "readiness":
		return runReadiness(args[1:], stdout)
	case "setup":
		return runSetup(args[1:], stdout)
	case "activation":
		return runActivation(args[1:], stdout)
	case "accept":
		return runAccept(args[1:], stdout)
	case "proof":
		return runProof(args[1:], stdout)
	case "demo":
		return runDemo(args[1:], stdout)
	case "up":
		return runService(append([]string{"start", "--all"}, args[1:]...), stdout)
	case "down":
		return runService(append([]string{"stop", "--all"}, args[1:]...), stdout)
	case "restart":
		return runService(append([]string{"restart", "--all"}, args[1:]...), stdout)
	case "logs":
		return runService(append([]string{"logs"}, args[1:]...), stdout)
	default:
		printUsage(stderr)
		return exitError{code: exitUsage, message: fmt.Sprintf("unknown command %q", args[0]), printed: true}
	}
}

func runVersion(args []string, stdout io.Writer) error {
	parsed, err := parseArgs(args, []string{"json"}, nil)
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return usageError(parsed.bool("json"), stdout, "version does not accept positional arguments")
	}
	if parsed.bool("json") {
		return writeJSON(stdout, buildinfo.Current())
	}
	_, err = fmt.Fprintf(stdout, "codencer version %s\n", app.Version)
	return err
}

func runInit(args []string, stdout io.Writer) error {
	parsed, err := parseArgs(args, []string{"json"}, []string{"config"})
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return usageError(parsed.bool("json"), stdout, "init does not accept positional arguments")
	}
	paths, err := local.ResolvePaths("", parsed.value("config"))
	if err != nil {
		return err
	}
	result, err := local.EnsureHome(paths, time.Now().UTC())
	if err != nil {
		return err
	}
	if parsed.bool("json") {
		return writeJSON(stdout, result)
	}
	fmt.Fprintf(stdout, "Codencer home: %s\n", result.Paths.Home)
	fmt.Fprintf(stdout, "Config:        %s\n", result.Paths.ConfigFile)
	fmt.Fprintf(stdout, "Projects:      %s\n", result.Paths.ProjectsFile)
	fmt.Fprintf(stdout, "Machine:       %s\n", result.Paths.MachineFile)
	if result.ConfigCreated || result.RegistryCreated {
		fmt.Fprintln(stdout, "Initialized local production files.")
	} else {
		fmt.Fprintln(stdout, "Local production files already exist.")
	}
	return nil
}

func runLogin(args []string, stdout io.Writer) error {
	parsed, err := parseArgs(args, []string{"json", "dev-approve"}, []string{"gateway", "email", "display-name", "timeout", "config"})
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return usageError(parsed.bool("json"), stdout, "login does not accept positional arguments")
	}
	paths, err := local.ResolvePaths("", parsed.value("config"))
	if err != nil {
		return err
	}
	if _, err := local.EnsureHome(paths, time.Now().UTC()); err != nil {
		return err
	}
	sessionPath := account.SessionPath(paths.Home)
	gatewayURL := account.NormalizeGatewayURL(parsed.value("gateway"))
	client := account.NewClient(gatewayURL, "")
	auth, err := client.DeviceAuthorize(contextBackground(), parsed.value("email"), parsed.value("display-name"))
	if err != nil {
		return jsonAwareError(parsed.bool("json"), stdout, exitFailed, err.Error())
	}
	if !parsed.bool("json") {
		fmt.Fprintf(stdout, "Open %s\n", auth.VerificationURI)
		fmt.Fprintf(stdout, "Code: %s\n", auth.UserCode)
	}
	if parsed.bool("dev-approve") {
		if err := client.DeviceApprove(contextBackground(), auth.UserCode); err != nil {
			return jsonAwareError(parsed.bool("json"), stdout, exitFailed, err.Error())
		}
	}
	timeout, err := parseDurationOrDefault(parsed.value("timeout"), 2*time.Minute)
	if err != nil {
		return jsonAwareError(parsed.bool("json"), stdout, exitUsage, err.Error())
	}
	interval := time.Duration(auth.Interval) * time.Second
	if interval <= 0 {
		interval = time.Second
	}
	deadline := time.Now().Add(timeout)
	var token account.TokenResponse
	for {
		token, err = client.DeviceToken(contextBackground(), auth.DeviceCode)
		if err == nil {
			break
		}
		apiErr, ok := err.(*account.APIError)
		if !ok || apiErr.Code != "authorization_pending" {
			return jsonAwareError(parsed.bool("json"), stdout, exitFailed, err.Error())
		}
		if time.Now().Add(interval).After(deadline) {
			return jsonAwareError(parsed.bool("json"), stdout, exitFailed, "device authorization timed out")
		}
		time.Sleep(interval)
	}
	session := account.NewSession(gatewayURL, token, time.Now().UTC())
	if err := account.SaveSession(sessionPath, session); err != nil {
		return jsonAwareError(parsed.bool("json"), stdout, exitFailed, err.Error())
	}
	payload := map[string]any{
		"ok":           true,
		"action":       "login",
		"session":      session.Safe(sessionPath),
		"device":       map[string]any{"verification_uri": auth.VerificationURI, "user_code": auth.UserCode, "expires_in": auth.ExpiresIn},
		"token_stored": true,
	}
	if parsed.bool("json") {
		return writeJSON(stdout, payload)
	}
	fmt.Fprintf(stdout, "Logged in to %s\n", session.GatewayURL)
	fmt.Fprintf(stdout, "Workspace: %s\n", session.WorkspaceID)
	return nil
}

func runWhoami(args []string, stdout io.Writer) error {
	parsed, err := parseArgs(args, []string{"json"}, []string{"gateway", "config"})
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return usageError(parsed.bool("json"), stdout, "whoami does not accept positional arguments")
	}
	paths, sessionPath, session, err := loadCodencerSession(parsed.value("config"))
	if err != nil {
		_ = paths
		return jsonAwareError(parsed.bool("json"), stdout, exitUsage, err.Error())
	}
	gatewayURL := firstNonEmpty(parsed.value("gateway"), session.GatewayURL)
	client := account.NewClient(gatewayURL, session.AccessToken)
	who, err := client.Whoami(contextBackground())
	if err != nil {
		return jsonAwareError(parsed.bool("json"), stdout, exitFailed, err.Error())
	}
	payload := map[string]any{"ok": true, "session": session.Safe(sessionPath), "gateway": who}
	if parsed.bool("json") {
		return writeJSON(stdout, payload)
	}
	fmt.Fprintf(stdout, "Gateway:   %s\n", gatewayURL)
	fmt.Fprintf(stdout, "MCP URL:   %s\n", who.MCPURL)
	fmt.Fprintf(stdout, "User:      %s\n", who.UserID)
	fmt.Fprintf(stdout, "Workspace: %s\n", who.WorkspaceID)
	return nil
}

func runLogout(args []string, stdout io.Writer) error {
	parsed, err := parseArgs(args, []string{"json"}, []string{"gateway", "config"})
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return usageError(parsed.bool("json"), stdout, "logout does not accept positional arguments")
	}
	_, sessionPath, pathErr := resolveSessionPath(parsed.value("config"))
	if pathErr != nil {
		return pathErr
	}
	_, _, session, err := loadCodencerSession(parsed.value("config"))
	if err == nil {
		gatewayURL := firstNonEmpty(parsed.value("gateway"), session.GatewayURL)
		_ = account.NewClient(gatewayURL, session.AccessToken).Logout(contextBackground())
	} else if !os.IsNotExist(err) {
		return jsonAwareError(parsed.bool("json"), stdout, exitUsage, err.Error())
	}
	if err := account.RemoveSession(sessionPath); err != nil {
		return jsonAwareError(parsed.bool("json"), stdout, exitFailed, err.Error())
	}
	payload := map[string]any{"ok": true, "action": "logout", "session_path": sessionPath}
	if parsed.bool("json") {
		return writeJSON(stdout, payload)
	}
	fmt.Fprintln(stdout, "Logged out.")
	return nil
}

func runPaths(args []string, stdout io.Writer) error {
	parsed, err := parseArgs(args, []string{"json"}, []string{"repo", "config"})
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return usageError(parsed.bool("json"), stdout, "paths does not accept positional arguments")
	}
	paths, err := local.ResolvePaths(parsed.value("repo"), parsed.value("config"))
	if err != nil {
		return err
	}
	if parsed.bool("json") {
		return writeJSON(stdout, paths)
	}
	printPaths(stdout, paths)
	return nil
}

func runConfig(args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] != "show" {
		return usageError(hasBoolFlag(args, "json"), stdout, "usage: codencer config show [--json] [--config <path>]")
	}
	parsed, err := parseArgs(args[1:], []string{"json"}, []string{"config"})
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return usageError(parsed.bool("json"), stdout, "config show does not accept positional arguments")
	}
	paths, err := local.ResolvePaths("", parsed.value("config"))
	if err != nil {
		return err
	}
	cfg, err := local.LoadConfig(paths.ConfigFile)
	if err != nil {
		return err
	}
	payload := map[string]any{"config": cfg, "config_file": paths.ConfigFile}
	if parsed.bool("json") {
		return writeJSON(stdout, payload)
	}
	fmt.Fprintf(stdout, "Config file:        %s\n", paths.ConfigFile)
	fmt.Fprintf(stdout, "Default daemon URL: %s\n", cfg.DefaultDaemonURL)
	if cfg.RelayConfigPath != "" {
		fmt.Fprintf(stdout, "Relay config:       %s\n", cfg.RelayConfigPath)
	}
	if cfg.ConnectorConfigPath != "" {
		fmt.Fprintf(stdout, "Connector config:   %s\n", cfg.ConnectorConfigPath)
	}
	if cfg.GatewayConfigPath != "" {
		fmt.Fprintf(stdout, "Gateway config:     %s\n", cfg.GatewayConfigPath)
	}
	return nil
}

func runDoctor(args []string, stdout io.Writer) error {
	toolchainOnly := false
	if len(args) > 0 && args[0] == "toolchain" {
		toolchainOnly = true
		args = args[1:]
	}
	parsed, err := parseArgs(args, []string{"json", "strict"}, []string{"repo", "config", "project"})
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return usageError(parsed.bool("json"), stdout, "doctor does not accept positional arguments")
	}
	repoRoot, err := repoRootForCommand(parsed.value("repo"))
	if err != nil {
		return err
	}
	paths, err := local.ResolvePaths(repoRoot, parsed.value("config"))
	if err != nil {
		return err
	}
	if _, _, err := loadRegistryWithMachine(paths, time.Now().UTC()); err != nil {
		return err
	}
	cfg, err := local.LoadConfig(paths.ConfigFile)
	if err != nil {
		return err
	}
	report := local.BuildDoctorReport(local.DoctorOptions{
		Paths:         paths,
		Config:        cfg,
		RepoRoot:      repoRoot,
		ProjectID:     parsed.value("project"),
		ToolchainOnly: toolchainOnly,
		Strict:        parsed.bool("strict"),
	})
	if parsed.bool("json") {
		if err := writeJSON(stdout, report); err != nil {
			return err
		}
	} else {
		printDoctor(stdout, report)
	}
	if !report.OK {
		return exitError{code: exitFailed, message: "doctor checks failed", printed: true}
	}
	return nil
}

func runStatus(args []string, stdout io.Writer) error {
	parsed, err := parseArgs(args, []string{"json"}, []string{"repo", "config", "project"})
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return usageError(parsed.bool("json"), stdout, "status does not accept positional arguments")
	}
	repoRoot, err := repoRootForCommand(parsed.value("repo"))
	if err != nil {
		return err
	}
	paths, err := local.ResolvePaths(repoRoot, parsed.value("config"))
	if err != nil {
		return err
	}
	cfg, err := local.LoadConfig(paths.ConfigFile)
	if err != nil {
		return err
	}
	report := local.BuildStatusReport(local.StatusOptions{
		Paths:     paths,
		Config:    cfg,
		ProjectID: parsed.value("project"),
		RepoRoot:  repoRoot,
	})
	if parsed.bool("json") {
		if err := writeJSON(stdout, report); err != nil {
			return err
		}
	} else {
		printStatus(stdout, report)
	}
	if report.Status == local.RuntimeError {
		return exitError{code: exitFailed, message: "status check failed", printed: true}
	}
	return nil
}

func runProject(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return usageError(hasBoolFlag(args, "json"), stdout, "usage: codencer project <init|adopt|scan|list|get|use|status|share|unshare|remove>")
	}
	switch args[0] {
	case "init":
		return runProjectInit(args[1:], stdout)
	case "adopt":
		return runProjectAdopt(args[1:], stdout)
	case "scan":
		return runProjectScan(args[1:], stdout)
	case "list":
		return runProjectList(args[1:], stdout)
	case "get":
		return runProjectGet(args[1:], stdout)
	case "use":
		return runProjectUse(args[1:], stdout)
	case "status":
		return runProjectStatus(args[1:], stdout)
	case "share":
		return runProjectShare(args[1:], stdout)
	case "unshare":
		return runProjectUnshare(args[1:], stdout)
	case "remove":
		return runProjectRemove(args[1:], stdout)
	default:
		return usageError(hasBoolFlag(args, "json"), stdout, fmt.Sprintf("unknown project command %q", args[0]))
	}
}

func runMachine(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return usageError(hasBoolFlag(args, "json"), stdout, "usage: codencer machine <show|set-label> [--json]")
	}
	switch args[0] {
	case "show":
		parsed, err := parseArgs(args[1:], []string{"json"}, []string{"config"})
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 0 {
			return usageError(parsed.bool("json"), stdout, "machine show does not accept positional arguments")
		}
		paths, err := local.ResolvePaths("", parsed.value("config"))
		if err != nil {
			return err
		}
		if _, err := local.EnsureHome(paths, time.Now().UTC()); err != nil {
			return err
		}
		machine, _, err := local.EnsureMachine(paths.MachineFile, time.Now().UTC())
		if err != nil {
			return err
		}
		payload := map[string]any{"machine": machine, "machine_path": paths.MachineFile}
		if parsed.bool("json") {
			return writeJSON(stdout, payload)
		}
		printMachine(stdout, machine, paths.MachineFile)
		return nil
	case "set-label":
		parsed, err := parseArgs(args[1:], []string{"json"}, []string{"config"})
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 1 {
			return usageError(parsed.bool("json"), stdout, "usage: codencer machine set-label <label> [--json]")
		}
		paths, err := local.ResolvePaths("", parsed.value("config"))
		if err != nil {
			return err
		}
		if _, err := local.EnsureHome(paths, time.Now().UTC()); err != nil {
			return err
		}
		machine, err := local.SetMachineHostLabel(paths.MachineFile, parsed.positionals[0], time.Now().UTC())
		if err != nil {
			return jsonAwareError(parsed.bool("json"), stdout, exitUsage, err.Error())
		}
		payload := map[string]any{"machine": machine, "machine_path": paths.MachineFile}
		if parsed.bool("json") {
			return writeJSON(stdout, payload)
		}
		printMachine(stdout, machine, paths.MachineFile)
		return nil
	default:
		return usageError(hasBoolFlag(args, "json"), stdout, fmt.Sprintf("unknown machine command %q", args[0]))
	}
}

func runConnector(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return usageError(hasBoolFlag(args, "json"), stdout, "usage: codencer connector <login|enroll|run|status|config show> [flags]")
	}
	switch args[0] {
	case "login":
		parsed, err := parseArgs(args[1:], []string{"json"}, []string{"gateway", "relay", "daemon-url", "config", "codencer-home", "label"})
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 0 {
			return usageError(parsed.bool("json"), stdout, "connector login does not accept positional arguments")
		}
		report, err := runConnectorLogin(parsed)
		if err != nil {
			return jsonAwareError(parsed.bool("json"), stdout, exitFailed, err.Error())
		}
		if parsed.bool("json") {
			return writeJSON(stdout, report)
		}
		fmt.Fprintf(stdout, "Connector logged in: %s\n", report["connector_id"])
		fmt.Fprintf(stdout, "Relay profile: %s\n", report["relay_profile_id"])
		fmt.Fprintf(stdout, "Config: %s\n", report["config_path"])
		return nil
	case "enroll":
		parsed, err := parseArgs(args[1:], []string{"json"}, []string{"relay-url", "daemon-url", "enrollment-token", "config", "codencer-home", "label"})
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 0 {
			return usageError(parsed.bool("json"), stdout, "connector enroll does not accept positional arguments")
		}
		if strings.TrimSpace(parsed.value("relay-url")) == "" {
			return usageError(parsed.bool("json"), stdout, "connector enroll requires --relay-url")
		}
		if strings.TrimSpace(parsed.value("enrollment-token")) == "" {
			return usageError(parsed.bool("json"), stdout, "connector enroll requires --enrollment-token")
		}
		report, err := connectorops.Enroll(contextBackground(), connectorops.EnrollOptions{
			RelayURL:        parsed.value("relay-url"),
			DaemonURL:       parsed.value("daemon-url"),
			EnrollmentToken: parsed.value("enrollment-token"),
			ConfigPath:      parsed.value("config"),
			CodencerHome:    parsed.value("codencer-home"),
			Label:           parsed.value("label"),
		})
		if err != nil {
			return jsonAwareError(parsed.bool("json"), stdout, exitFailed, err.Error())
		}
		if parsed.bool("json") {
			if err := writeJSON(stdout, report); err != nil {
				return err
			}
		} else {
			printConnectorEnrollReport(stdout, report)
		}
		if report.ExitCode != exitSuccess {
			return exitError{code: report.ExitCode, message: "connector enrollment failed", printed: true}
		}
		return nil
	case "run":
		parsed, err := parseArgs(args[1:], nil, []string{"config"})
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 0 {
			return usageError(false, stdout, "connector run does not accept positional arguments")
		}
		return connectorops.Run(contextBackground(), parsed.value("config"), "")
	case "status":
		parsed, err := parseArgs(args[1:], []string{"json"}, []string{"config"})
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 0 {
			return usageError(parsed.bool("json"), stdout, "connector status does not accept positional arguments")
		}
		report, err := connectorops.LoadStatus(parsed.value("config"), "")
		if err != nil {
			return jsonAwareError(parsed.bool("json"), stdout, exitFailed, err.Error())
		}
		if parsed.bool("json") {
			return writeJSON(stdout, report)
		}
		printConnectorStatusReport(stdout, report)
		return nil
	case "config":
		if len(args) < 2 || args[1] != "show" {
			return usageError(hasBoolFlag(args, "json"), stdout, "usage: codencer connector config show [--config <path>] [--json] [--show-secrets]")
		}
		parsed, err := parseArgs(args[2:], []string{"json", "show-secrets"}, []string{"config"})
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 0 {
			return usageError(parsed.bool("json"), stdout, "connector config show does not accept positional arguments")
		}
		report, err := connectorops.LoadConfig(parsed.value("config"), "", parsed.bool("show-secrets"))
		if err != nil {
			return jsonAwareError(parsed.bool("json"), stdout, exitFailed, err.Error())
		}
		if parsed.bool("json") {
			return writeJSON(stdout, report)
		}
		return writeJSON(stdout, report.Config)
	default:
		return usageError(hasBoolFlag(args, "json"), stdout, fmt.Sprintf("unknown connector command %q", args[0]))
	}
}

func runConnectorLogin(parsed parsedArgs) (map[string]any, error) {
	paths, err := local.ResolvePathsForHome("", "", parsed.value("codencer-home"))
	if err != nil {
		return nil, err
	}
	if _, err := local.EnsureHome(paths, time.Now().UTC()); err != nil {
		return nil, err
	}
	sessionPath := account.SessionPath(paths.Home)
	session, err := account.LoadSession(sessionPath)
	if err != nil {
		return nil, fmt.Errorf("codencer login required: %w", err)
	}
	gatewayURL := firstNonEmpty(parsed.value("gateway"), session.GatewayURL)
	client := account.NewClient(gatewayURL, session.AccessToken)
	machine, _, err := local.EnsureMachine(paths.MachineFile, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	localCfg, err := local.LoadConfig(paths.ConfigFile)
	if err != nil {
		return nil, err
	}
	daemonURL := firstNonEmpty(parsed.value("daemon-url"), localCfg.DefaultDaemonURL)
	relayID := firstNonEmpty(parsed.value("relay"), "default")
	login, err := client.ConnectorLogin(contextBackground(), account.ConnectorLoginRequest{
		Relay: relayID,
		Machine: account.MachineInput{
			MachineID: machine.MachineID,
			Hostname:  machine.Hostname,
			HostLabel: machine.HostLabel,
			OS:        machine.OS,
			Arch:      machine.Arch,
		},
		Label: firstNonEmpty(parsed.value("label"), machine.HostLabel, machine.MachineID),
	})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(login.EnrollmentToken) == "" {
		return nil, fmt.Errorf("Gateway did not return connector enrollment token")
	}
	enroll, err := connectorops.Enroll(contextBackground(), connectorops.EnrollOptions{
		RelayURL:        login.RelayURL,
		DaemonURL:       daemonURL,
		EnrollmentToken: login.EnrollmentToken,
		ConfigPath:      parsed.value("config"),
		CodencerHome:    paths.Home,
		Label:           firstNonEmpty(parsed.value("label"), machine.HostLabel, machine.MachineID),
	})
	if err != nil {
		return nil, err
	}
	cfg, err := connector.LoadConfig(enroll.ConfigPath)
	if err != nil {
		return nil, err
	}
	if err := client.ConnectorComplete(contextBackground(), account.ConnectorCompleteRequest{
		BindingID:        login.BindingID,
		RelayConnectorID: enroll.ConnectorID,
		RelayMachineID:   enroll.MachineID,
		PublicKey:        cfg.PublicKey,
	}); err != nil {
		return nil, err
	}
	return map[string]any{
		"ok":                   true,
		"action":               "connector_login",
		"gateway_url":          account.NormalizeGatewayURL(gatewayURL),
		"workspace_id":         login.WorkspaceID,
		"binding_id":           login.BindingID,
		"relay_profile_id":     firstNonEmpty(login.RelayProfile.RelayProfileID, login.RelayProfile.ID),
		"relay_profile":        login.RelayProfile,
		"relay_url":            login.RelayURL,
		"local_machine_id":     machine.MachineID,
		"host_label":           machine.HostLabel,
		"connector_id":         enroll.ConnectorID,
		"relay_machine_id":     enroll.MachineID,
		"config_path":          enroll.ConfigPath,
		"codencer_home":        paths.Home,
		"daemon_url":           daemonURL,
		"local_config_updated": enroll.LocalConfigUpdated,
	}, nil
}

func runGateway(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return usageError(hasBoolFlag(args, "json"), stdout, "usage: codencer gateway <relay|status|config> [flags]")
	}
	switch args[0] {
	case "relay":
		return runGatewayRelay(args[1:], stdout)
	case "status":
		parsed, err := parseArgs(args[1:], []string{"json"}, []string{"config"})
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 0 {
			return usageError(parsed.bool("json"), stdout, "gateway status does not accept positional arguments")
		}
		path, cfg, err := loadGatewayConfig(parsed.value("config"))
		if err != nil {
			return jsonAwareError(parsed.bool("json"), stdout, exitUsage, err.Error())
		}
		payload := map[string]any{
			"ok":                  true,
			"config_file":         path,
			"mcp_url":             cfg.MCPURL,
			"public_base_url":     cfg.PublicBaseURL,
			"listen_addr":         cfg.ListenAddr,
			"auth_mode":           cfg.Auth.Mode,
			"oauth_dev_enabled":   cfg.OAuthDev.Enabled,
			"relay_profile_count": len(cfg.RelayProfiles),
			"relay_profiles":      gatewayRelayStatuses(cfg),
		}
		if parsed.bool("json") {
			return writeJSON(stdout, payload)
		}
		fmt.Fprintf(stdout, "Gateway config: %s\n", path)
		fmt.Fprintf(stdout, "MCP URL:        %s\n", cfg.MCPURL)
		fmt.Fprintf(stdout, "Relay profiles: %d\n", len(cfg.RelayProfiles))
		return nil
	case "config":
		if len(args) < 2 || args[1] != "show" {
			return usageError(hasBoolFlag(args, "json"), stdout, "usage: codencer gateway config show [--config <path>] [--json]")
		}
		parsed, err := parseArgs(args[2:], []string{"json"}, []string{"config"})
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 0 {
			return usageError(parsed.bool("json"), stdout, "gateway config show does not accept positional arguments")
		}
		path, cfg, err := loadGatewayConfig(parsed.value("config"))
		if err != nil {
			return jsonAwareError(parsed.bool("json"), stdout, exitUsage, err.Error())
		}
		payload := map[string]any{"config_file": path, "config": gatewaypkg.RedactedConfig(cfg)}
		if parsed.bool("json") {
			return writeJSON(stdout, payload)
		}
		fmt.Fprintf(stdout, "Gateway config: %s\n", path)
		fmt.Fprintf(stdout, "Public URL:     %s\n", cfg.PublicBaseURL)
		fmt.Fprintf(stdout, "MCP URL:        %s\n", cfg.MCPURL)
		fmt.Fprintf(stdout, "Relay profiles: %d\n", len(cfg.RelayProfiles))
		return nil
	default:
		return usageError(hasBoolFlag(args, "json"), stdout, fmt.Sprintf("unknown gateway command %q", args[0]))
	}
}

func runGatewayRelay(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return usageError(hasBoolFlag(args, "json"), stdout, "usage: codencer gateway relay <add|list|status|remove> [flags]")
	}
	switch args[0] {
	case "add":
		parsed, err := parseArgs(args[1:], []string{"json", "disabled"}, []string{"config", "gateway", "id", "name", "url", "token-env", "token-file"})
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 0 {
			return usageError(parsed.bool("json"), stdout, "gateway relay add does not accept positional arguments")
		}
		relayURL := strings.TrimSpace(parsed.value("url"))
		if relayURL == "" {
			return usageError(parsed.bool("json"), stdout, "gateway relay add requires --url")
		}
		if parsed.value("token-env") == "" && parsed.value("token-file") == "" {
			return usageError(parsed.bool("json"), stdout, "gateway relay add requires --token-env or --token-file")
		}
		client, session, remote, err := gatewayRelayRemoteClient(parsed.value("config"), parsed.value("gateway"))
		if err != nil {
			return jsonAwareError(parsed.bool("json"), stdout, exitUsage, err.Error())
		}
		if remote {
			enabled := !parsed.bool("disabled")
			relay, err := client.AddRelay(contextBackground(), account.RelayProfileInput{
				ID:        parsed.value("id"),
				Name:      parsed.value("name"),
				URL:       relayURL,
				TokenEnv:  parsed.value("token-env"),
				TokenFile: parsed.value("token-file"),
				Type:      "self_host",
				Enabled:   &enabled,
			})
			if err != nil {
				return jsonAwareError(parsed.bool("json"), stdout, exitFailed, err.Error())
			}
			payload := map[string]any{"ok": true, "gateway_url": session.GatewayURL, "relay": relay}
			if parsed.bool("json") {
				return writeJSON(stdout, payload)
			}
			fmt.Fprintf(stdout, "Gateway relay profile %q saved in workspace %s\n", firstNonEmpty(relay.RelayProfileID, relay.ID), session.WorkspaceID)
			return nil
		}
		id := strings.TrimSpace(firstNonEmpty(parsed.value("id"), parsed.value("name")))
		if id == "" {
			return usageError(parsed.bool("json"), stdout, "gateway relay add requires --id or --name")
		}
		path, cfg, err := loadOrDefaultGatewayConfig(parsed.value("config"))
		if err != nil {
			return jsonAwareError(parsed.bool("json"), stdout, exitUsage, err.Error())
		}
		cfg, err = gatewaypkg.UpsertRelayProfile(cfg, gatewaypkg.RelayProfile{
			ID:        id,
			Name:      parsed.value("name"),
			URL:       relayURL,
			TokenEnv:  parsed.value("token-env"),
			TokenFile: parsed.value("token-file"),
			Enabled:   !parsed.bool("disabled"),
		})
		if err != nil {
			return jsonAwareError(parsed.bool("json"), stdout, exitUsage, err.Error())
		}
		if err := gatewaypkg.SaveConfig(path, cfg); err != nil {
			return jsonAwareError(parsed.bool("json"), stdout, exitFailed, err.Error())
		}
		payload := map[string]any{"ok": true, "config_file": path, "relay": gatewaypkg.RelayStatus(mustGatewayProfile(cfg, id))}
		if parsed.bool("json") {
			return writeJSON(stdout, payload)
		}
		fmt.Fprintf(stdout, "Gateway relay profile %q saved in %s\n", id, path)
		return nil
	case "list", "status":
		parsed, err := parseArgs(args[1:], []string{"json"}, []string{"config", "gateway"})
		if err != nil {
			return err
		}
		if args[0] == "list" && len(parsed.positionals) != 0 {
			return usageError(parsed.bool("json"), stdout, "gateway relay list does not accept positional arguments")
		}
		if args[0] == "status" && len(parsed.positionals) > 1 {
			return usageError(parsed.bool("json"), stdout, "usage: codencer gateway relay status [id] [--json]")
		}
		client, session, remote, err := gatewayRelayRemoteClient(parsed.value("config"), parsed.value("gateway"))
		if err != nil {
			return jsonAwareError(parsed.bool("json"), stdout, exitUsage, err.Error())
		}
		if remote {
			if args[0] == "status" && len(parsed.positionals) == 1 {
				relay, err := client.GetRelay(contextBackground(), parsed.positionals[0])
				if err != nil {
					return jsonAwareError(parsed.bool("json"), stdout, exitFailed, err.Error())
				}
				payload := map[string]any{"ok": true, "gateway_url": session.GatewayURL, "relay": relay}
				if parsed.bool("json") {
					return writeJSON(stdout, payload)
				}
				fmt.Fprintf(stdout, "%s\t%s\tenabled=%t\tstatus=%s\n", firstNonEmpty(relay.RelayProfileID, relay.ID), relay.URL, relay.Enabled, relay.Status)
				return nil
			}
			relays, err := client.ListRelays(contextBackground())
			if err != nil {
				return jsonAwareError(parsed.bool("json"), stdout, exitFailed, err.Error())
			}
			payload := map[string]any{"ok": true, "gateway_url": session.GatewayURL, "relays": relays}
			if parsed.bool("json") {
				return writeJSON(stdout, payload)
			}
			for _, relay := range relays {
				fmt.Fprintf(stdout, "%s\t%s\tenabled=%t\tstatus=%s\ttoken_configured=%t\n", firstNonEmpty(relay.RelayProfileID, relay.ID), relay.URL, relay.Enabled, relay.Status, relay.TokenConfigured)
			}
			return nil
		}
		path, cfg, err := loadGatewayConfig(parsed.value("config"))
		if err != nil {
			return jsonAwareError(parsed.bool("json"), stdout, exitUsage, err.Error())
		}
		if args[0] == "status" && len(parsed.positionals) == 1 {
			id := parsed.positionals[0]
			profile, apiErr := gatewayProfileByID(cfg, id)
			if apiErr != nil {
				return jsonAwareError(parsed.bool("json"), stdout, exitUsage, apiErr.Error())
			}
			payload := map[string]any{"config_file": path, "relay": gatewaypkg.RelayStatus(profile)}
			if parsed.bool("json") {
				return writeJSON(stdout, payload)
			}
			fmt.Fprintf(stdout, "%s\t%s\tenabled=%t\ttoken_env=%s\n", profile.ID, profile.URL, profile.Enabled, profile.TokenEnv)
			return nil
		}
		payload := map[string]any{"config_file": path, "relays": gatewayRelayStatuses(cfg)}
		if parsed.bool("json") {
			return writeJSON(stdout, payload)
		}
		for _, profile := range cfg.RelayProfiles {
			fmt.Fprintf(stdout, "%s\t%s\tenabled=%t\ttoken_env=%s\n", profile.ID, profile.URL, profile.Enabled, profile.TokenEnv)
		}
		return nil
	case "remove":
		parsed, err := parseArgs(args[1:], []string{"json"}, []string{"config", "gateway"})
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 1 {
			return usageError(parsed.bool("json"), stdout, "usage: codencer gateway relay remove <id> [--json]")
		}
		client, session, remote, err := gatewayRelayRemoteClient(parsed.value("config"), parsed.value("gateway"))
		if err != nil {
			return jsonAwareError(parsed.bool("json"), stdout, exitUsage, err.Error())
		}
		if remote {
			if err := client.RemoveRelay(contextBackground(), parsed.positionals[0]); err != nil {
				return jsonAwareError(parsed.bool("json"), stdout, exitFailed, err.Error())
			}
			payload := map[string]any{"ok": true, "gateway_url": session.GatewayURL, "relay_profile_id": parsed.positionals[0]}
			if parsed.bool("json") {
				return writeJSON(stdout, payload)
			}
			fmt.Fprintf(stdout, "Gateway relay profile %q removed from workspace %s\n", parsed.positionals[0], session.WorkspaceID)
			return nil
		}
		return jsonAwareError(parsed.bool("json"), stdout, exitUsage, "gateway relay remove requires codencer login or --gateway")
	default:
		return usageError(hasBoolFlag(args, "json"), stdout, fmt.Sprintf("unknown gateway relay command %q", args[0]))
	}
}

func gatewayRelayRemoteClient(configFlag, gatewayFlag string) (*account.Client, account.Session, bool, error) {
	if strings.TrimSpace(configFlag) != "" && strings.TrimSpace(gatewayFlag) == "" {
		return nil, account.Session{}, false, nil
	}
	_, sessionPath, session, err := loadCodencerSession(configFlag)
	if err != nil {
		if strings.TrimSpace(gatewayFlag) != "" {
			return nil, account.Session{}, false, fmt.Errorf("codencer login required: %w", err)
		}
		if os.IsNotExist(err) {
			return nil, account.Session{}, false, nil
		}
		return nil, account.Session{}, false, err
	}
	_ = sessionPath
	gatewayURL := firstNonEmpty(gatewayFlag, session.GatewayURL)
	session.GatewayURL = account.NormalizeGatewayURL(gatewayURL)
	return account.NewClient(gatewayURL, session.AccessToken), session, true, nil
}

func gatewayProfileByID(cfg *gatewaypkg.Config, id string) (gatewaypkg.RelayProfile, error) {
	id = strings.TrimSpace(id)
	for _, profile := range cfg.RelayProfiles {
		if profile.ID == id {
			return profile, nil
		}
	}
	return gatewaypkg.RelayProfile{}, fmt.Errorf("relay profile %q not found", id)
}

func gatewayConfigPath(flagValue string) (string, error) {
	if strings.TrimSpace(flagValue) != "" {
		return filepath.Abs(flagValue)
	}
	paths, err := local.ResolvePaths("", "")
	if err != nil {
		return "", err
	}
	cfg, err := local.LoadConfig(paths.ConfigFile)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(cfg.GatewayConfigPath) != "" {
		return cfg.GatewayConfigPath, nil
	}
	return filepath.Join(paths.RuntimeDir, "gateway", "config.json"), nil
}

func loadGatewayConfig(flagValue string) (string, *gatewaypkg.Config, error) {
	path, err := gatewayConfigPath(flagValue)
	if err != nil {
		return "", nil, err
	}
	cfg, err := gatewaypkg.LoadConfig(path)
	if err != nil {
		return "", nil, err
	}
	return path, cfg, nil
}

func loadOrDefaultGatewayConfig(flagValue string) (string, *gatewaypkg.Config, error) {
	path, err := gatewayConfigPath(flagValue)
	if err != nil {
		return "", nil, err
	}
	cfg, err := gatewaypkg.LoadConfig(path)
	if err == nil {
		return path, cfg, nil
	}
	if !os.IsNotExist(err) {
		return "", nil, err
	}
	paths, pathErr := local.ResolvePaths("", "")
	if pathErr != nil {
		return "", nil, pathErr
	}
	if _, ensureErr := local.EnsureHome(paths, time.Now().UTC()); ensureErr != nil {
		return "", nil, ensureErr
	}
	localCfg, loadErr := local.LoadConfig(paths.ConfigFile)
	if loadErr != nil {
		return "", nil, loadErr
	}
	localCfg.GatewayConfigPath = path
	localCfg.UpdatedAt = time.Now().UTC()
	if saveErr := local.SaveConfig(paths.ConfigFile, localCfg); saveErr != nil {
		return "", nil, saveErr
	}
	return path, gatewaypkg.DefaultConfig(), nil
}

func gatewayRelayStatuses(cfg *gatewaypkg.Config) []gatewaypkg.RelayProfileStatus {
	if cfg == nil {
		return nil
	}
	out := make([]gatewaypkg.RelayProfileStatus, 0, len(cfg.RelayProfiles))
	for _, profile := range cfg.RelayProfiles {
		out = append(out, gatewaypkg.RelayStatus(profile))
	}
	return out
}

func mustGatewayProfile(cfg *gatewaypkg.Config, id string) gatewaypkg.RelayProfile {
	for _, profile := range cfg.RelayProfiles {
		if profile.ID == id {
			return profile
		}
	}
	return gatewaypkg.RelayProfile{ID: id}
}

func runProjectInit(args []string, stdout io.Writer) error {
	parsed, err := parseArgs(args, []string{"json", "force", "share-to-relay", "scan", "update-project-config"}, []string{"id", "repo", "adapter", "name", "adapter-profile", "profile", "daemon-url", "relay-instance-id", "config"})
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return usageError(parsed.bool("json"), stdout, "project init does not accept positional arguments")
	}
	report, err := initOrAdoptProject(projectInitOptions{
		Repo:                parsed.value("repo"),
		ID:                  parsed.value("id"),
		Name:                parsed.value("name"),
		Adapter:             parsed.value("adapter"),
		AdapterProfile:      parsed.value("adapter-profile"),
		Profile:             parsed.value("profile"),
		DaemonURL:           parsed.value("daemon-url"),
		RelayInstanceID:     parsed.value("relay-instance-id"),
		ConfigPath:          parsed.value("config"),
		ShareToRelay:        parsed.bool("share-to-relay"),
		Scan:                parsed.bool("scan"),
		UpdateProjectConfig: parsed.bool("update-project-config") || parsed.bool("force"),
		RequireExisting:     false,
		Action:              "project_init",
	}, time.Now().UTC())
	if err != nil {
		return jsonAwareError(parsed.bool("json"), stdout, exitUsage, err.Error())
	}
	if parsed.bool("json") {
		return writeJSON(stdout, report)
	}
	for _, message := range report.Messages {
		fmt.Fprintln(stdout, message)
	}
	return nil
}

func runProjectAdopt(args []string, stdout io.Writer) error {
	parsed, err := parseArgs(args, []string{"json"}, []string{"repo", "daemon-url", "relay-instance-id", "config"})
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return usageError(parsed.bool("json"), stdout, "project adopt does not accept positional arguments")
	}
	report, err := initOrAdoptProject(projectInitOptions{
		Repo:            parsed.value("repo"),
		DaemonURL:       parsed.value("daemon-url"),
		RelayInstanceID: parsed.value("relay-instance-id"),
		ConfigPath:      parsed.value("config"),
		RequireExisting: true,
		Action:          "project_adopt",
	}, time.Now().UTC())
	if err != nil {
		return jsonAwareError(parsed.bool("json"), stdout, exitUsage, err.Error())
	}
	if parsed.bool("json") {
		return writeJSON(stdout, report)
	}
	for _, message := range report.Messages {
		fmt.Fprintln(stdout, message)
	}
	return nil
}

func runProjectScan(args []string, stdout io.Writer) error {
	parsed, err := parseArgs(args, []string{"json"}, []string{"repo"})
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return usageError(parsed.bool("json"), stdout, "project scan does not accept positional arguments")
	}
	repoRoot, err := repoRootForCommand(parsed.value("repo"))
	if err != nil {
		return err
	}
	proposal, err := projectconfig.Scan(repoRoot)
	if err != nil {
		return jsonAwareError(parsed.bool("json"), stdout, exitUsage, err.Error())
	}
	if parsed.bool("json") {
		return writeJSON(stdout, map[string]any{"proposal": proposal, "read_only": true})
	}
	fmt.Fprintf(stdout, "Suggested project id: %s\n", proposal.SuggestedProjectID)
	fmt.Fprintf(stdout, "Detected files: %s\n", strings.Join(proposal.DetectedFiles, ", "))
	return nil
}

type projectInitOptions struct {
	Repo                string
	ID                  string
	Name                string
	Adapter             string
	AdapterProfile      string
	Profile             string
	DaemonURL           string
	RelayInstanceID     string
	ConfigPath          string
	ShareToRelay        bool
	Scan                bool
	UpdateProjectConfig bool
	RequireExisting     bool
	Action              string
}

type projectInitReport struct {
	OK                  bool                        `json:"ok"`
	Action              string                      `json:"action"`
	Project             projectpkg.Project          `json:"project"`
	Machine             local.MachineIdentity       `json:"machine"`
	ProjectConfig       projectconfig.Config        `json:"project_config"`
	ProjectConfigPath   string                      `json:"project_config_path"`
	ProjectConfigAction string                      `json:"project_config_action"`
	RegistryPath        string                      `json:"registry_path"`
	CurrentProjectID    string                      `json:"current_project_id,omitempty"`
	Warnings            []string                    `json:"warnings,omitempty"`
	Messages            []string                    `json:"messages"`
	ScanProposal        *projectconfig.ScanProposal `json:"scan_proposal,omitempty"`
}

func initOrAdoptProject(opts projectInitOptions, now time.Time) (projectInitReport, error) {
	repoRoot, err := repoRootForCommand(opts.Repo)
	if err != nil {
		return projectInitReport{}, err
	}
	absRepo, err := filepath.Abs(repoRoot)
	if err != nil {
		return projectInitReport{}, fmt.Errorf("resolve repo path: %w", err)
	}
	absRepo = filepath.Clean(absRepo)
	paths, err := local.ResolvePaths("", opts.ConfigPath)
	if err != nil {
		return projectInitReport{}, err
	}
	if _, err := local.EnsureHome(paths, now); err != nil {
		return projectInitReport{}, err
	}
	localCfg, err := local.LoadConfig(paths.ConfigFile)
	if err != nil {
		return projectInitReport{}, err
	}
	machine, _, err := local.EnsureMachine(paths.MachineFile, now)
	if err != nil {
		return projectInitReport{}, err
	}
	configPath := projectconfig.Path(absRepo)
	configExists := projectconfig.Exists(absRepo)
	messages := []string{}
	var proposal *projectconfig.ScanProposal
	if opts.Scan {
		scanned, err := projectconfig.Scan(absRepo)
		if err != nil {
			return projectInitReport{}, err
		}
		proposal = &scanned
	}

	var projectConfig projectconfig.Config
	configAction := "adopted"
	warnings := []string{}
	if configExists {
		loaded, loadWarnings, err := projectconfig.Load(absRepo)
		if err != nil {
			return projectInitReport{}, err
		}
		projectConfig = loaded
		warnings = append(warnings, loadWarnings...)
		messages = append(messages, "Found .codencer/project.json", "Validated project config")
		if opts.UpdateProjectConfig {
			projectConfig = applyProjectConfigOverrides(projectConfig, opts, proposal, absRepo)
			if err := projectconfig.Save(absRepo, projectConfig); err != nil {
				return projectInitReport{}, err
			}
			configAction = "updated"
			messages = append(messages, "Updated .codencer/project.json")
		}
	} else {
		if opts.RequireExisting {
			return projectInitReport{}, fmt.Errorf(".codencer/project.json is required; run codencer project init first")
		}
		projectConfig = newProjectConfigForInit(opts, proposal, absRepo)
		if strings.TrimSpace(projectConfig.Project.ID) == "" {
			return projectInitReport{}, fmt.Errorf("project id is required and could not be inferred")
		}
		if err := projectconfig.Save(absRepo, projectConfig); err != nil {
			return projectInitReport{}, err
		}
		configAction = "created"
		messages = append(messages, "Created .codencer/project.json")
	}

	registryProject, projectWarnings, err := projectFromConfig(absRepo, configPath, projectConfig, opts, localCfg, machine)
	if err != nil {
		return projectInitReport{}, err
	}
	warnings = append(warnings, projectWarnings...)
	registry, err := projectpkg.LoadRegistry(paths.ProjectsFile)
	if err != nil {
		return projectInitReport{}, err
	}
	if existing, err := projectpkg.GetProject(registry, registryProject.ID); err == nil {
		if !opts.ShareToRelay {
			registryProject.SharedToRelay = existing.SharedToRelay
		}
		if strings.TrimSpace(opts.RelayInstanceID) == "" {
			registryProject.RelayInstanceID = existing.RelayInstanceID
		}
		if strings.TrimSpace(opts.DaemonURL) == "" && existing.DaemonURL != "" {
			registryProject.DaemonURL = existing.DaemonURL
		}
	}
	saved, err := projectpkg.UpsertProject(registry, registryProject, true, now)
	if err != nil {
		return projectInitReport{}, err
	}
	if err := projectpkg.SaveRegistry(paths.ProjectsFile, registry); err != nil {
		return projectInitReport{}, err
	}
	messages = append(messages, "Updated local registry")
	return projectInitReport{
		OK:                  true,
		Action:              opts.Action,
		Project:             saved,
		Machine:             machine,
		ProjectConfig:       projectConfig,
		ProjectConfigPath:   configPath,
		ProjectConfigAction: configAction,
		RegistryPath:        paths.ProjectsFile,
		CurrentProjectID:    registry.CurrentProjectID,
		Warnings:            warnings,
		Messages:            messages,
		ScanProposal:        proposal,
	}, nil
}

func newProjectConfigForInit(opts projectInitOptions, proposal *projectconfig.ScanProposal, repoRoot string) projectconfig.Config {
	id := strings.TrimSpace(opts.ID)
	if id == "" && proposal != nil {
		id = proposal.SuggestedProjectID
	}
	if id == "" {
		id = projectconfig.InferID(repoRoot)
	}
	name := strings.TrimSpace(opts.Name)
	if name == "" && proposal != nil {
		name = proposal.SuggestedProjectName
	}
	adapter := strings.TrimSpace(opts.Adapter)
	profile, _ := adapterProfileAlias(opts.AdapterProfile, opts.Profile)
	forbidden := projectconfig.DefaultForbiddenPaths()
	if proposal != nil && len(proposal.SuggestedForbiddenPaths) > 0 {
		forbidden = proposal.SuggestedForbiddenPaths
	}
	return projectconfig.Default(projectconfig.DefaultOptions{
		ProjectID:      id,
		ProjectName:    name,
		DefaultAdapter: adapter,
		DefaultProfile: profile,
		ForbiddenPaths: forbidden,
	})
}

func applyProjectConfigOverrides(cfg projectconfig.Config, opts projectInitOptions, proposal *projectconfig.ScanProposal, repoRoot string) projectconfig.Config {
	if strings.TrimSpace(opts.ID) != "" {
		cfg.Project.ID = strings.TrimSpace(opts.ID)
	}
	if strings.TrimSpace(opts.Name) != "" {
		cfg.Project.Name = strings.TrimSpace(opts.Name)
	}
	if strings.TrimSpace(opts.Adapter) != "" {
		cfg.Execution.DefaultAdapter = strings.TrimSpace(opts.Adapter)
	}
	if profile, err := adapterProfileAlias(opts.AdapterProfile, opts.Profile); err == nil && strings.TrimSpace(profile) != "" {
		cfg.Execution.DefaultProfile = profile
	}
	if opts.Scan && proposal != nil && len(proposal.SuggestedForbiddenPaths) > 0 {
		cfg.Workspace.ForbiddenPaths = proposal.SuggestedForbiddenPaths
	}
	if strings.TrimSpace(cfg.Project.ID) == "" {
		cfg.Project.ID = projectconfig.InferID(repoRoot)
	}
	return cfg
}

func projectFromConfig(repoRoot, configPath string, cfg projectconfig.Config, opts projectInitOptions, localCfg local.Config, machine local.MachineIdentity) (projectpkg.Project, []string, error) {
	if strings.TrimSpace(opts.ID) != "" && !opts.UpdateProjectConfig && strings.TrimSpace(opts.ID) != cfg.Project.ID {
		return projectpkg.Project{}, nil, fmt.Errorf("--id %q does not match existing project config id %q; use --update-project-config to change it", opts.ID, cfg.Project.ID)
	}
	adapter := firstNonEmpty(opts.Adapter, cfg.Execution.DefaultAdapter)
	profile, err := adapterProfileAlias(opts.AdapterProfile, opts.Profile)
	if err != nil {
		return projectpkg.Project{}, nil, err
	}
	profile = firstNonEmpty(profile, cfg.Execution.DefaultProfile)
	daemonURL := firstNonEmpty(opts.DaemonURL, localCfg.DefaultDaemonURL)
	name := firstNonEmpty(opts.Name, cfg.Project.Name)
	return projectpkg.NewProject(projectpkg.ProjectOptions{
		ID:                 cfg.Project.ID,
		Name:               name,
		RepoRoot:           repoRoot,
		DefaultAdapter:     adapter,
		AdapterProfile:     profile,
		DaemonURL:          daemonURL,
		RelayInstanceID:    opts.RelayInstanceID,
		SharedToRelay:      opts.ShareToRelay,
		MachineID:          machine.MachineID,
		HostLabel:          machine.HostLabel,
		Hostname:           machine.Hostname,
		ProjectConfigPath:  configPath,
		AllowedPaths:       []string{cfg.Workspace.Root},
		ForbiddenPaths:     cfg.Workspace.ForbiddenPaths,
		DefaultValidations: projectValidationCommands(cfg.Validations),
	})
}

func projectValidationCommands(validations []projectconfig.ValidationCommand) []projectpkg.ValidationCommand {
	out := make([]projectpkg.ValidationCommand, 0, len(validations))
	for _, validation := range validations {
		out = append(out, projectpkg.ValidationCommand{Name: validation.Name, Command: validation.Command})
	}
	return out
}

func loadRegistryWithMachine(paths local.Paths, now time.Time) (*projectpkg.Registry, local.MachineIdentity, error) {
	if _, err := local.EnsureHome(paths, now); err != nil {
		return nil, local.MachineIdentity{}, err
	}
	machine, _, err := local.EnsureMachine(paths.MachineFile, now)
	if err != nil {
		return nil, local.MachineIdentity{}, err
	}
	registry, err := projectpkg.LoadRegistry(paths.ProjectsFile)
	if err != nil {
		return nil, local.MachineIdentity{}, err
	}
	if projectpkg.BackfillMachineMetadata(registry, machine.MachineID, machine.HostLabel, machine.Hostname, now) {
		if err := projectpkg.SaveRegistry(paths.ProjectsFile, registry); err != nil {
			return nil, local.MachineIdentity{}, err
		}
	}
	return registry, machine, nil
}

func runProjectList(args []string, stdout io.Writer) error {
	parsed, err := parseArgs(args, []string{"json"}, []string{"config"})
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return usageError(parsed.bool("json"), stdout, "project list does not accept positional arguments")
	}
	paths, err := local.ResolvePaths("", parsed.value("config"))
	if err != nil {
		return err
	}
	registry, machine, err := loadRegistryWithMachine(paths, time.Now().UTC())
	if err != nil {
		return err
	}
	payload := map[string]any{
		"current_project_id": registry.CurrentProjectID,
		"projects":           projectpkg.ListProjects(registry),
		"registry_path":      paths.ProjectsFile,
		"machine":            machine,
	}
	if parsed.bool("json") {
		return writeJSON(stdout, payload)
	}
	if len(registry.Projects) == 0 {
		fmt.Fprintln(stdout, "no projects registered")
		return nil
	}
	for _, p := range projectpkg.ListProjects(registry) {
		current := " "
		if p.ID == registry.CurrentProjectID {
			current = "*"
		}
		fmt.Fprintf(stdout, "%s %-24s %s\n", current, p.ID, p.RepoRoot)
	}
	return nil
}

func runProjectGet(args []string, stdout io.Writer) error {
	parsed, err := parseArgs(args, []string{"json"}, []string{"config"})
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 1 {
		return usageError(parsed.bool("json"), stdout, "usage: codencer project get <id> [--json]")
	}
	paths, err := local.ResolvePaths("", parsed.value("config"))
	if err != nil {
		return err
	}
	registry, machine, err := loadRegistryWithMachine(paths, time.Now().UTC())
	if err != nil {
		return err
	}
	p, err := projectpkg.GetProject(registry, parsed.positionals[0])
	if err != nil {
		return jsonAwareError(parsed.bool("json"), stdout, exitUsage, err.Error())
	}
	if parsed.bool("json") {
		return writeJSON(stdout, map[string]any{"project": p, "machine": machine})
	}
	printProject(stdout, p)
	return nil
}

func runProjectUse(args []string, stdout io.Writer) error {
	parsed, err := parseArgs(args, []string{"json"}, []string{"config"})
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 1 {
		return usageError(parsed.bool("json"), stdout, "usage: codencer project use <id> [--json]")
	}
	paths, err := local.ResolvePaths("", parsed.value("config"))
	if err != nil {
		return err
	}
	registry, machine, err := loadRegistryWithMachine(paths, time.Now().UTC())
	if err != nil {
		return err
	}
	p, err := projectpkg.UseProject(registry, parsed.positionals[0])
	if err != nil {
		return jsonAwareError(parsed.bool("json"), stdout, exitUsage, err.Error())
	}
	if err := projectpkg.SaveRegistry(paths.ProjectsFile, registry); err != nil {
		return err
	}
	payload := map[string]any{"current_project_id": p.ID, "project": p, "machine": machine}
	if parsed.bool("json") {
		return writeJSON(stdout, payload)
	}
	fmt.Fprintf(stdout, "Current project: %s\n", p.ID)
	return nil
}

func runProjectStatus(args []string, stdout io.Writer) error {
	parsed, err := parseArgs(args, []string{"json"}, []string{"config", "repo"})
	if err != nil {
		return err
	}
	projectID := ""
	if len(parsed.positionals) > 1 {
		return usageError(parsed.bool("json"), stdout, "usage: codencer project status [id] [--json]")
	}
	if len(parsed.positionals) == 1 {
		projectID = parsed.positionals[0]
	}
	repoRoot, err := repoRootForCommand(parsed.value("repo"))
	if err != nil {
		return err
	}
	paths, err := local.ResolvePaths(repoRoot, parsed.value("config"))
	if err != nil {
		return err
	}
	cfg, err := local.LoadConfig(paths.ConfigFile)
	if err != nil {
		return err
	}
	report := local.BuildStatusReport(local.StatusOptions{
		Paths:     paths,
		Config:    cfg,
		ProjectID: projectID,
		RepoRoot:  repoRoot,
	})
	if parsed.bool("json") {
		if err := writeJSON(stdout, report); err != nil {
			return err
		}
	} else {
		printStatus(stdout, report)
	}
	if report.Status == local.RuntimeError {
		return exitError{code: exitFailed, message: "project status failed", printed: true}
	}
	return nil
}

func runProjectShare(args []string, stdout io.Writer) error {
	parsed, err := parseArgs(args, []string{"json"}, []string{"config", "relay-instance-id", "daemon-url"})
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 1 {
		return usageError(parsed.bool("json"), stdout, "usage: codencer project share <id> [--relay-instance-id <id>] [--daemon-url <url>] [--json]")
	}
	paths, err := local.ResolvePaths("", parsed.value("config"))
	if err != nil {
		return err
	}
	registry, machine, err := loadRegistryWithMachine(paths, time.Now().UTC())
	if err != nil {
		return err
	}
	project, err := projectpkg.ShareProject(registry, parsed.positionals[0], parsed.value("relay-instance-id"), parsed.value("daemon-url"), time.Now().UTC())
	if err != nil {
		return jsonAwareError(parsed.bool("json"), stdout, exitUsage, err.Error())
	}
	if err := projectpkg.SaveRegistry(paths.ProjectsFile, registry); err != nil {
		return err
	}
	payload := map[string]any{
		"project":         project,
		"project_id":      project.ID,
		"shared_to_relay": project.SharedToRelay,
		"machine_id":      machine.MachineID,
		"host_label":      machine.HostLabel,
	}
	if parsed.bool("json") {
		return writeJSON(stdout, payload)
	}
	fmt.Fprintf(stdout, "Shared project %s to relay\n", project.ID)
	return nil
}

func runProjectUnshare(args []string, stdout io.Writer) error {
	parsed, err := parseArgs(args, []string{"json"}, []string{"config"})
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 1 {
		return usageError(parsed.bool("json"), stdout, "usage: codencer project unshare <id> [--json]")
	}
	paths, err := local.ResolvePaths("", parsed.value("config"))
	if err != nil {
		return err
	}
	registry, machine, err := loadRegistryWithMachine(paths, time.Now().UTC())
	if err != nil {
		return err
	}
	project, err := projectpkg.UnshareProject(registry, parsed.positionals[0], time.Now().UTC())
	if err != nil {
		return jsonAwareError(parsed.bool("json"), stdout, exitUsage, err.Error())
	}
	if err := projectpkg.SaveRegistry(paths.ProjectsFile, registry); err != nil {
		return err
	}
	payload := map[string]any{"project": project, "shared_to_relay": project.SharedToRelay, "machine": machine}
	if parsed.bool("json") {
		return writeJSON(stdout, payload)
	}
	fmt.Fprintf(stdout, "Unshared project %s from relay\n", project.ID)
	return nil
}

func runProjectRemove(args []string, stdout io.Writer) error {
	parsed, err := parseArgs(args, []string{"json"}, []string{"config"})
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 1 {
		return usageError(parsed.bool("json"), stdout, "usage: codencer project remove <id> [--json]")
	}
	paths, err := local.ResolvePaths("", parsed.value("config"))
	if err != nil {
		return err
	}
	registry, _, err := loadRegistryWithMachine(paths, time.Now().UTC())
	if err != nil {
		return err
	}
	removed, err := projectpkg.RemoveProject(registry, parsed.positionals[0])
	if err != nil {
		return jsonAwareError(parsed.bool("json"), stdout, exitUsage, err.Error())
	}
	if err := projectpkg.SaveRegistry(paths.ProjectsFile, registry); err != nil {
		return err
	}
	payload := map[string]any{"removed_project": removed, "current_project_id": registry.CurrentProjectID}
	if parsed.bool("json") {
		return writeJSON(stdout, payload)
	}
	fmt.Fprintf(stdout, "Removed project: %s\n", removed.ID)
	return nil
}

func runRun(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return usageError(hasBoolFlag(args, "json"), stdout, "usage: codencer run <start|list|get|status> [flags]")
	}
	service := localexec.NewService()
	switch args[0] {
	case "start":
		parsed, err := parseArgs(args[1:], []string{"json"}, []string{"project", "repo", "config"})
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 0 {
			return usageError(parsed.bool("json"), stdout, "run start does not accept positional arguments")
		}
		report, err := service.StartRun(contextBackground(), localexec.RunOptions{BaseOptions: localexec.BaseOptions{
			ProjectID:  parsed.value("project"),
			RepoRoot:   parsed.value("repo"),
			ConfigPath: parsed.value("config"),
		}})
		return finishExecutionReport(stdout, parsed.bool("json"), report, err)
	case "list":
		parsed, err := parseArgs(args[1:], []string{"json"}, []string{"project", "repo", "config"})
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 0 {
			return usageError(parsed.bool("json"), stdout, "run list does not accept positional arguments")
		}
		report, err := service.ListRuns(contextBackground(), localexec.RunOptions{BaseOptions: localexec.BaseOptions{
			ProjectID:  parsed.value("project"),
			RepoRoot:   parsed.value("repo"),
			ConfigPath: parsed.value("config"),
		}})
		return finishExecutionReport(stdout, parsed.bool("json"), report, err)
	case "get":
		parsed, err := parseArgs(args[1:], []string{"json"}, []string{"project", "repo", "config"})
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 1 {
			return usageError(parsed.bool("json"), stdout, "usage: codencer run get <id> [--project <id>] [--json]")
		}
		report, err := service.GetRun(contextBackground(), localexec.RunOptions{
			BaseOptions: localexec.BaseOptions{
				ProjectID:  parsed.value("project"),
				RepoRoot:   parsed.value("repo"),
				ConfigPath: parsed.value("config"),
			},
			RunID: parsed.positionals[0],
		})
		return finishExecutionReport(stdout, parsed.bool("json"), report, err)
	case "status":
		parsed, err := parseArgs(args[1:], []string{"json"}, []string{"project", "repo", "config"})
		if err != nil {
			return err
		}
		if len(parsed.positionals) > 1 {
			return usageError(parsed.bool("json"), stdout, "usage: codencer run status [id] [--project <id>] [--json]")
		}
		runID := ""
		if len(parsed.positionals) == 1 {
			runID = parsed.positionals[0]
		}
		report, err := service.Status(contextBackground(), localexec.RunOptions{
			BaseOptions: localexec.BaseOptions{
				ProjectID:  parsed.value("project"),
				RepoRoot:   parsed.value("repo"),
				ConfigPath: parsed.value("config"),
			},
			RunID: runID,
		})
		return finishExecutionReport(stdout, parsed.bool("json"), report, err)
	default:
		return usageError(hasBoolFlag(args, "json"), stdout, fmt.Sprintf("unknown run command %q", args[0]))
	}
}

func runSubmit(args []string, stdout io.Writer) error {
	parsed, err := parseArgs(args, []string{"json", "wait", "stdin"}, []string{
		"project", "repo", "config", "run", "goal", "task-file", "prompt-file", "profile", "adapter-profile", "title", "timeout-seconds",
	})
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return usageError(parsed.bool("json"), stdout, "submit does not accept positional arguments")
	}
	adapterProfile, err := adapterProfileAlias(parsed.value("adapter-profile"), parsed.value("profile"))
	if err != nil {
		return jsonAwareError(parsed.bool("json"), stdout, exitUsage, err.Error())
	}
	timeoutSeconds := 0
	if raw := strings.TrimSpace(parsed.value("timeout-seconds")); raw != "" {
		timeoutSeconds, err = strconv.Atoi(raw)
		if err != nil || timeoutSeconds <= 0 {
			return jsonAwareError(parsed.bool("json"), stdout, exitUsage, "--timeout-seconds must be a positive integer")
		}
	}
	service := localexec.NewService()
	report, err := service.Submit(contextBackground(), localexec.SubmitOptions{
		BaseOptions: localexec.BaseOptions{
			ProjectID:  parsed.value("project"),
			RepoRoot:   parsed.value("repo"),
			ConfigPath: parsed.value("config"),
		},
		RunID:          parsed.value("run"),
		Goal:           parsed.value("goal"),
		TaskFile:       parsed.value("task-file"),
		PromptFile:     parsed.value("prompt-file"),
		UseStdin:       parsed.bool("stdin"),
		Wait:           parsed.bool("wait"),
		Profile:        adapterProfile,
		AdapterProfile: adapterProfile,
		Title:          parsed.value("title"),
		TimeoutSeconds: timeoutSeconds,
	})
	return finishExecutionReport(stdout, parsed.bool("json"), report, err)
}

func runRunPlan(args []string, stdout io.Writer) error {
	parsed, err := parseArgs(args, []string{"json", "wait"}, []string{"project", "repo", "config"})
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 1 {
		return usageError(parsed.bool("json"), stdout, "usage: codencer run-plan <manifest.yaml|json> --project <id> [--wait] --json")
	}
	service := localexec.NewService()
	report, err := service.RunPlan(contextBackground(), localexec.RunPlanOptions{
		BaseOptions: localexec.BaseOptions{
			ProjectID:  parsed.value("project"),
			RepoRoot:   parsed.value("repo"),
			ConfigPath: parsed.value("config"),
		},
		ManifestPath: parsed.positionals[0],
		Wait:         parsed.bool("wait"),
	})
	return finishRunPlanReport(stdout, parsed.bool("json"), report, err)
}

func runProfile(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return usageError(hasBoolFlag(args, "json"), stdout, "usage: codencer profile <list|get> --json")
	}
	service := localexec.NewService()
	switch args[0] {
	case "list":
		parsed, err := parseArgs(args[1:], []string{"json"}, nil)
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 0 {
			return usageError(parsed.bool("json"), stdout, "profile list does not accept positional arguments")
		}
		report := service.ListProfiles()
		return finishExecutionReport(stdout, parsed.bool("json"), report, nil)
	case "get":
		parsed, err := parseArgs(args[1:], []string{"json"}, nil)
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 1 {
			return usageError(parsed.bool("json"), stdout, "usage: codencer profile get <id> [--json]")
		}
		report, err := service.GetProfile(parsed.positionals[0])
		return finishExecutionReport(stdout, parsed.bool("json"), report, err)
	default:
		return usageError(hasBoolFlag(args, "json"), stdout, fmt.Sprintf("unknown profile command %q", args[0]))
	}
}

func runService(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return usageError(hasBoolFlag(args, "json"), stdout, "usage: codencer service <install|uninstall|start|stop|restart|status|logs|render> [service|--all]")
	}
	action := args[0]
	switch action {
	case "install", "uninstall", "start", "stop", "restart", "status":
		parsed, err := parseArgs(args[1:], []string{"json", "all", "strict", "dry-run"}, []string{"project", "repo", "config", "manager", "bin-dir"})
		if err != nil {
			return err
		}
		serviceName, err := serviceNameFromPositionals(parsed.positionals, parsed.bool("all"))
		if err != nil {
			return jsonAwareError(parsed.bool("json"), stdout, exitUsage, err.Error())
		}
		report, err := supervisor.Service(contextBackground(), action, supervisor.Options{
			Service:    serviceName,
			All:        parsed.bool("all") || serviceName == "",
			ProjectID:  parsed.value("project"),
			RepoRoot:   parsed.value("repo"),
			ConfigPath: parsed.value("config"),
			Manager:    parsed.value("manager"),
			BinDir:     parsed.value("bin-dir"),
			DryRun:     parsed.bool("dry-run"),
			Strict:     parsed.bool("strict"),
		})
		if err != nil {
			return jsonAwareError(parsed.bool("json"), stdout, exitFailed, err.Error())
		}
		if parsed.bool("json") {
			if err := writeJSON(stdout, report); err != nil {
				return err
			}
		} else {
			printServiceReport(stdout, report)
		}
		if parsed.bool("strict") && !report.OK {
			return exitError{code: report.ExitCode, message: "service " + action + " failed", printed: true}
		}
		return nil
	case "logs":
		parsed, err := parseArgs(args[1:], []string{"json", "follow"}, []string{"project", "repo", "config", "manager", "bin-dir", "tail"})
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 1 {
			return usageError(parsed.bool("json"), stdout, "usage: codencer service logs <service> [--tail 100] [--follow]")
		}
		tail := 100
		if raw := strings.TrimSpace(parsed.value("tail")); raw != "" {
			parsedTail, err := strconv.Atoi(raw)
			if err != nil || parsedTail <= 0 {
				return jsonAwareError(parsed.bool("json"), stdout, exitUsage, "--tail must be a positive integer")
			}
			tail = parsedTail
		}
		if parsed.bool("json") {
			report, err := supervisor.Service(contextBackground(), "status", supervisor.Options{
				Service:    parsed.positionals[0],
				ProjectID:  parsed.value("project"),
				RepoRoot:   parsed.value("repo"),
				ConfigPath: parsed.value("config"),
				Manager:    parsed.value("manager"),
				BinDir:     parsed.value("bin-dir"),
			})
			if err != nil {
				return jsonAwareError(true, stdout, exitFailed, err.Error())
			}
			return writeJSON(stdout, report)
		}
		_, err = supervisor.Logs(contextBackground(), supervisor.Options{
			Service:    parsed.positionals[0],
			ProjectID:  parsed.value("project"),
			RepoRoot:   parsed.value("repo"),
			ConfigPath: parsed.value("config"),
			Manager:    parsed.value("manager"),
			BinDir:     parsed.value("bin-dir"),
			Tail:       tail,
			Follow:     parsed.bool("follow"),
		}, stdout)
		return err
	case "render":
		parsed, err := parseArgs(args[1:], []string{"json"}, []string{"project", "repo", "config", "manager", "bin-dir", "format"})
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 1 {
			return usageError(parsed.bool("json"), stdout, "usage: codencer service render <service> --format <launchd|systemd>")
		}
		rendered, err := supervisor.RenderService(supervisor.Options{
			Service:    parsed.positionals[0],
			ProjectID:  parsed.value("project"),
			RepoRoot:   parsed.value("repo"),
			ConfigPath: parsed.value("config"),
			Manager:    parsed.value("manager"),
			BinDir:     parsed.value("bin-dir"),
			Format:     parsed.value("format"),
		})
		if err != nil {
			return jsonAwareError(parsed.bool("json"), stdout, exitFailed, err.Error())
		}
		if parsed.bool("json") {
			return writeJSON(stdout, map[string]string{"service": parsed.positionals[0], "format": parsed.value("format"), "rendered": rendered})
		}
		_, err = fmt.Fprint(stdout, rendered)
		return err
	default:
		return usageError(hasBoolFlag(args, "json"), stdout, fmt.Sprintf("unknown service command %q", action))
	}
}

func runWatchdog(args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] != "once" {
		return usageError(hasBoolFlag(args, "json"), stdout, "usage: codencer watchdog once [--json] [--strict]")
	}
	parsed, err := parseArgs(args[1:], []string{"json", "strict"}, []string{"project", "repo", "config", "manager", "bin-dir"})
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return usageError(parsed.bool("json"), stdout, "watchdog once does not accept positional arguments")
	}
	report, err := supervisor.WatchdogOnce(contextBackground(), supervisor.Options{
		ProjectID:  parsed.value("project"),
		RepoRoot:   parsed.value("repo"),
		ConfigPath: parsed.value("config"),
		Manager:    parsed.value("manager"),
		BinDir:     parsed.value("bin-dir"),
		Strict:     parsed.bool("strict"),
	})
	if err != nil {
		return jsonAwareError(parsed.bool("json"), stdout, exitFailed, err.Error())
	}
	if parsed.bool("json") {
		if err := writeJSON(stdout, report); err != nil {
			return err
		}
	} else {
		printWatchdogReport(stdout, report)
	}
	if parsed.bool("strict") && !report.OK {
		return exitError{code: report.ExitCode, message: "watchdog checks failed", printed: true}
	}
	return nil
}

func runRecover(args []string, stdout io.Writer) error {
	mode := "all"
	runID := ""
	parseFrom := args
	if len(args) > 0 {
		switch args[0] {
		case "locks":
			mode = "locks"
			parseFrom = args[1:]
		case "run":
			mode = "run"
			if len(args) < 2 {
				return usageError(hasBoolFlag(args, "json"), stdout, "usage: codencer recover run <run-id> [--json]")
			}
			runID = args[1]
			parseFrom = args[2:]
		}
	}
	parsed, err := parseArgs(parseFrom, []string{"json", "dry-run", "restart-services", "strict"}, []string{"project", "repo", "config", "manager", "bin-dir"})
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return usageError(parsed.bool("json"), stdout, "recover does not accept extra positional arguments")
	}
	report, err := supervisor.Recover(contextBackground(), supervisor.RecoveryOptions{
		Options: supervisor.Options{
			ProjectID:  parsed.value("project"),
			RepoRoot:   parsed.value("repo"),
			ConfigPath: parsed.value("config"),
			Manager:    parsed.value("manager"),
			BinDir:     parsed.value("bin-dir"),
			DryRun:     parsed.bool("dry-run"),
			Strict:     parsed.bool("strict"),
		},
		Mode:            mode,
		RunID:           runID,
		RestartServices: parsed.bool("restart-services"),
	})
	if err != nil {
		return jsonAwareError(parsed.bool("json"), stdout, exitFailed, err.Error())
	}
	if parsed.bool("json") {
		if err := writeJSON(stdout, report); err != nil {
			return err
		}
	} else {
		printRecoveryReport(stdout, report)
	}
	if parsed.bool("strict") && !report.OK {
		return exitError{code: report.ExitCode, message: "recovery blocked", printed: true}
	}
	return nil
}

func runLive(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return usageError(hasBoolFlag(args, "json"), stdout, "usage: codencer live <matrix|codex|claude|relay-mcp|codex-mcp|claude-mcp|wsl|restart-reconnect|reports> [--json]")
	}
	if args[0] == "reports" {
		parsed, err := parseArgs(args[1:], []string{"json"}, []string{"config"})
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 0 {
			return usageError(parsed.bool("json"), stdout, "live reports does not accept positional arguments")
		}
		files, err := live.ListReports("", "live-matrix")
		if err != nil {
			return jsonAwareError(parsed.bool("json"), stdout, exitFailed, err.Error())
		}
		payload := map[string]any{"reports": files}
		if parsed.bool("json") {
			return writeJSON(stdout, payload)
		}
		for _, file := range files {
			fmt.Fprintf(stdout, "%s\n", file.Path)
		}
		return nil
	}

	parsed, err := parseArgs(args[1:], []string{"json", "enable-codex", "enable-claude", "enable-relay-mcp", "enable-wsl", "enable-service-restart", "enable-all"}, []string{"profile", "repo", "bin-dir", "endpoint"})
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return usageError(parsed.bool("json"), stdout, "live command does not accept positional arguments")
	}
	opts := live.Options{
		Profile:              parsed.value("profile"),
		RepoRoot:             parsed.value("repo"),
		BinDir:               parsed.value("bin-dir"),
		Endpoint:             parsed.value("endpoint"),
		EnableCodex:          parsed.bool("enable-codex"),
		EnableClaude:         parsed.bool("enable-claude"),
		EnableRelayMCP:       parsed.bool("enable-relay-mcp"),
		EnableWSL:            parsed.bool("enable-wsl"),
		EnableServiceRestart: parsed.bool("enable-service-restart"),
		EnableAll:            parsed.bool("enable-all"),
	}
	var report live.Report
	switch args[0] {
	case "matrix":
		if opts.Profile == "" {
			opts.Profile = "local"
		}
		report, err = live.Matrix(contextBackground(), opts)
	case "codex":
		report, err = live.RunCodex(contextBackground(), opts)
	case "claude":
		report, err = live.RunClaude(contextBackground(), opts)
	case "relay-mcp":
		report, err = live.RunRelayMCP(contextBackground(), opts)
	case "codex-mcp":
		report, err = live.RunCodexMCP(contextBackground(), opts)
	case "claude-mcp":
		report, err = live.RunClaudeMCP(contextBackground(), opts)
	case "wsl":
		report, err = live.RunWSL(contextBackground(), opts)
	case "restart-reconnect":
		report, err = live.RunRestartReconnect(contextBackground(), opts)
	default:
		return usageError(parsed.bool("json"), stdout, fmt.Sprintf("unknown live command %q", args[0]))
	}
	if err != nil {
		return jsonAwareError(parsed.bool("json"), stdout, exitFailed, err.Error())
	}
	if parsed.bool("json") {
		if err := writeJSON(stdout, report); err != nil {
			return err
		}
	} else {
		printLiveReport(stdout, report)
	}
	if code := live.ExitCode(report); code != exitSuccess {
		return exitError{code: code, message: "live check failed", printed: true}
	}
	return nil
}

func runReadiness(args []string, stdout io.Writer) error {
	if len(args) > 0 && args[0] == "reports" {
		parsed, err := parseArgs(args[1:], []string{"json"}, nil)
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 0 {
			return usageError(parsed.bool("json"), stdout, "readiness reports does not accept positional arguments")
		}
		files, err := live.ListReports("", "readiness")
		if err != nil {
			return jsonAwareError(parsed.bool("json"), stdout, exitFailed, err.Error())
		}
		payload := map[string]any{"reports": files}
		if parsed.bool("json") {
			return writeJSON(stdout, payload)
		}
		for _, file := range files {
			fmt.Fprintf(stdout, "%s\n", file.Path)
		}
		return nil
	}
	parsed, err := parseArgs(args, []string{"json", "local", "relay", "live"}, []string{"repo"})
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return usageError(parsed.bool("json"), stdout, "readiness does not accept positional arguments")
	}
	report, err := readiness.Build(contextBackground(), readiness.Options{
		Local:    parsed.bool("local"),
		Relay:    parsed.bool("relay"),
		Live:     parsed.bool("live"),
		RepoRoot: parsed.value("repo"),
	})
	if err != nil {
		return jsonAwareError(parsed.bool("json"), stdout, exitFailed, err.Error())
	}
	if parsed.bool("json") {
		if err := writeJSON(stdout, report); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(stdout, "verdict: %s\n", report.Verdict)
		for name, status := range report.Profiles {
			fmt.Fprintf(stdout, "%s: %s\n", name, status)
		}
		if report.ReportPath != "" {
			fmt.Fprintf(stdout, "report: %s\n", report.ReportPath)
		}
	}
	if report.Verdict == readiness.VerdictNotReady {
		return exitError{code: exitFailed, message: "readiness not ready", printed: true}
	}
	return nil
}

func runSetup(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return usageError(hasBoolFlag(args, "json"), stdout, "usage: codencer setup <local|relay|gateway|mcp> [flags]")
	}
	switch args[0] {
	case "local":
		parsed, err := parseArgs(args[1:], []string{"json", "install-services", "start-services", "strict"}, []string{"project-id", "repo", "adapter", "profile", "adapter-profile", "manager", "bin-dir"})
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 0 {
			return usageError(parsed.bool("json"), stdout, "setup local does not accept positional arguments")
		}
		adapterProfile, err := adapterProfileAlias(parsed.value("adapter-profile"), parsed.value("profile"))
		if err != nil {
			return jsonAwareError(parsed.bool("json"), stdout, exitUsage, err.Error())
		}
		report, err := setuppkg.Local(contextBackground(), setuppkg.LocalOptions{
			ProjectID:       parsed.value("project-id"),
			RepoRoot:        parsed.value("repo"),
			Adapter:         parsed.value("adapter"),
			AdapterProfile:  adapterProfile,
			InstallServices: parsed.bool("install-services"),
			StartServices:   parsed.bool("start-services"),
			Manager:         parsed.value("manager"),
			BinDir:          parsed.value("bin-dir"),
			Strict:          parsed.bool("strict"),
		})
		return finishSetupReport(stdout, parsed.bool("json"), report, err)
	case "relay":
		parsed, err := parseArgs(args[1:], []string{"json", "generate-planner-token", "enable-chatgpt-oauth-dev", "chatgpt-dev-noauth", "allow-real-projects-in-dev-noauth", "install-services", "start-services", "strict"}, []string{"base-url", "mcp-url", "relay-config", "connector-config", "planner-token", "planner-token-env", "oauth-issuer", "oauth-client-id", "oauth-client-secret", "manager", "bin-dir"})
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 0 {
			return usageError(parsed.bool("json"), stdout, "setup relay does not accept positional arguments")
		}
		report, err := setuppkg.Relay(contextBackground(), setuppkg.RelayOptions{
			BaseURL:                      parsed.value("base-url"),
			MCPURL:                       parsed.value("mcp-url"),
			RelayConfigPath:              parsed.value("relay-config"),
			ConnectorConfigPath:          parsed.value("connector-config"),
			PlannerToken:                 parsed.value("planner-token"),
			GeneratePlannerToken:         parsed.bool("generate-planner-token"),
			PlannerTokenEnv:              parsed.value("planner-token-env"),
			EnableChatGPTOAuthDev:        parsed.bool("enable-chatgpt-oauth-dev"),
			OAuthIssuer:                  parsed.value("oauth-issuer"),
			OAuthClientID:                parsed.value("oauth-client-id"),
			OAuthClientSecret:            parsed.value("oauth-client-secret"),
			ChatGPTDevNoAuth:             parsed.bool("chatgpt-dev-noauth"),
			AllowRealProjectsInDevNoAuth: parsed.bool("allow-real-projects-in-dev-noauth"),
			InstallServices:              parsed.bool("install-services"),
			StartServices:                parsed.bool("start-services"),
			Manager:                      parsed.value("manager"),
			BinDir:                       parsed.value("bin-dir"),
			Strict:                       parsed.bool("strict"),
		})
		return finishSetupReport(stdout, parsed.bool("json"), report, err)
	case "gateway":
		parsed, err := parseArgs(args[1:], []string{"json", "enable-oauth-dev", "install-services", "start-services", "strict"}, []string{"base-url", "mcp-url", "listen", "auth", "token-env", "token-file", "gateway-config", "store", "default-relay-url", "default-relay-token-env", "default-relay-token-file", "oauth-issuer", "oauth-client-id", "oauth-client-secret", "manager", "bin-dir"})
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 0 {
			return usageError(parsed.bool("json"), stdout, "setup gateway does not accept positional arguments")
		}
		report, err := setuppkg.Gateway(contextBackground(), setuppkg.GatewayOptions{
			BaseURL:               parsed.value("base-url"),
			MCPURL:                parsed.value("mcp-url"),
			ListenAddr:            parsed.value("listen"),
			GatewayConfigPath:     parsed.value("gateway-config"),
			StorePath:             parsed.value("store"),
			AuthMode:              parsed.value("auth"),
			TokenEnv:              parsed.value("token-env"),
			TokenFile:             parsed.value("token-file"),
			DefaultRelayURL:       parsed.value("default-relay-url"),
			DefaultRelayTokenEnv:  parsed.value("default-relay-token-env"),
			DefaultRelayTokenFile: parsed.value("default-relay-token-file"),
			EnableOAuthDev:        parsed.bool("enable-oauth-dev"),
			OAuthIssuer:           parsed.value("oauth-issuer"),
			OAuthClientID:         parsed.value("oauth-client-id"),
			OAuthClientSecret:     parsed.value("oauth-client-secret"),
			InstallServices:       parsed.bool("install-services"),
			StartServices:         parsed.bool("start-services"),
			Manager:               parsed.value("manager"),
			BinDir:                parsed.value("bin-dir"),
			Strict:                parsed.bool("strict"),
		})
		return finishSetupReport(stdout, parsed.bool("json"), report, err)
	case "mcp":
		parsed, err := parseArgs(args[1:], []string{"json"}, []string{"client", "relay", "endpoint", "token-env", "token", "name"})
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 0 {
			return usageError(parsed.bool("json"), stdout, "setup mcp does not accept positional arguments")
		}
		endpoint := firstNonEmpty(parsed.value("endpoint"), parsed.value("relay"))
		report, err := setuppkg.MCP(setuppkg.MCPOptions{
			Client:   parsed.value("client"),
			Endpoint: endpoint,
			TokenEnv: parsed.value("token-env"),
			Token:    parsed.value("token"),
			Name:     parsed.value("name"),
		})
		return finishSetupReport(stdout, parsed.bool("json"), report, err)
	default:
		return usageError(hasBoolFlag(args, "json"), stdout, fmt.Sprintf("unknown setup command %q", args[0]))
	}
}

func runActivation(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return usageError(hasBoolFlag(args, "json"), stdout, "usage: codencer activation <check|package|gateway|official|chatgpt|codex|claude-code> [--json]")
	}
	parsed, err := parseArgs(args[1:], []string{"json", "run-fake-manifest", "check-oauth", "check-chatgpt-readiness"}, []string{"gateway", "relay", "mcp-url", "token-env", "token", "project", "auth"})
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return usageError(parsed.bool("json"), stdout, "activation command does not accept positional arguments")
	}
	opts := activation.Options{
		Gateway:               parsed.value("gateway"),
		Relay:                 parsed.value("relay"),
		MCPURL:                parsed.value("mcp-url"),
		TokenEnv:              parsed.value("token-env"),
		Token:                 parsed.value("token"),
		ProjectID:             parsed.value("project"),
		RunFakeManifest:       parsed.bool("run-fake-manifest"),
		CheckOAuth:            parsed.bool("check-oauth"),
		CheckChatGPTReadiness: parsed.bool("check-chatgpt-readiness"),
		AuthMode:              parsed.value("auth"),
	}
	var report activation.Report
	switch args[0] {
	case "check":
		report, err = activation.CheckActivation(contextBackground(), opts)
	case "package":
		report, err = activation.Package(contextBackground(), opts)
	case "gateway":
		report, err = activation.Gateway(contextBackground(), opts)
	case "official":
		report, err = activation.Gateway(contextBackground(), opts)
		report.Mode = "official"
	case "chatgpt":
		report, err = activation.ChatGPT(opts)
		if err != nil {
			return finishActivationReport(stdout, parsed.bool("json"), report, err)
		}
		if parsed.bool("json") {
			if err := writeJSON(stdout, report.Output); err != nil {
				return err
			}
			if report.ExitCode != exitSuccess {
				return exitError{code: report.ExitCode, message: "activation check failed", printed: true}
			}
			return nil
		}
	case "codex":
		report, err = activation.Codex(opts)
	case "claude-code":
		report, err = activation.ClaudeCode(opts)
	default:
		return usageError(parsed.bool("json"), stdout, fmt.Sprintf("unknown activation command %q", args[0]))
	}
	return finishActivationReport(stdout, parsed.bool("json"), report, err)
}

func runAccept(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return usageError(hasBoolFlag(args, "json"), stdout, "usage: codencer accept <local-production|reports> [--json]")
	}
	if args[0] == "reports" {
		parsed, err := parseArgs(args[1:], []string{"json"}, nil)
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 0 {
			return usageError(parsed.bool("json"), stdout, "accept reports does not accept positional arguments")
		}
		files, err := acceptance.Reports("")
		if err != nil {
			return jsonAwareError(parsed.bool("json"), stdout, exitFailed, err.Error())
		}
		payload := map[string]any{"reports": files}
		if parsed.bool("json") {
			return writeJSON(stdout, payload)
		}
		for _, file := range files {
			fmt.Fprintln(stdout, file.Path)
		}
		return nil
	}
	if args[0] != "local-production" {
		return usageError(hasBoolFlag(args, "json"), stdout, "usage: codencer accept local-production [--profile local|relay|all] [--json]")
	}
	parsed, err := parseArgs(args[1:], []string{"json", "enable-codex", "enable-claude", "enable-relay-mcp", "enable-service-restart", "enable-all"}, []string{"profile", "repo", "bin-dir"})
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return usageError(parsed.bool("json"), stdout, "accept local-production does not accept positional arguments")
	}
	report, err := acceptance.LocalProduction(contextBackground(), acceptance.Options{
		Profile:              parsed.value("profile"),
		RepoRoot:             parsed.value("repo"),
		BinDir:               parsed.value("bin-dir"),
		EnableCodex:          parsed.bool("enable-codex"),
		EnableClaude:         parsed.bool("enable-claude"),
		EnableRelayMCP:       parsed.bool("enable-relay-mcp"),
		EnableServiceRestart: parsed.bool("enable-service-restart"),
		EnableAll:            parsed.bool("enable-all"),
	})
	if err != nil {
		return jsonAwareError(parsed.bool("json"), stdout, exitFailed, err.Error())
	}
	if parsed.bool("json") {
		if err := writeJSON(stdout, report); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(stdout, "verdict: %s\n", report.Verdict)
		for _, gate := range report.Gates {
			fmt.Fprintf(stdout, "%-14s %s\n", strings.ToUpper(gate.Status), gate.ID)
		}
		if report.ReportPath != "" {
			fmt.Fprintf(stdout, "report: %s\n", report.ReportPath)
		}
	}
	if report.Verdict == acceptance.VerdictNotReady {
		return exitError{code: exitFailed, message: "acceptance not ready", printed: true}
	}
	return nil
}

func runProof(args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] != "bundle" {
		return usageError(hasBoolFlag(args, "json"), stdout, "usage: codencer proof bundle [--json]")
	}
	parsed, err := parseArgs(args[1:], []string{"json"}, []string{"repo"})
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return usageError(parsed.bool("json"), stdout, "proof bundle does not accept positional arguments")
	}
	report, err := proof.Bundle(proof.Options{RepoRoot: parsed.value("repo")})
	if err != nil {
		return jsonAwareError(parsed.bool("json"), stdout, exitFailed, err.Error())
	}
	if parsed.bool("json") {
		return writeJSON(stdout, report)
	}
	fmt.Fprintf(stdout, "proof: %s\n", report.ProofPath)
	fmt.Fprintf(stdout, "bundle: %s\n", report.BundleDir)
	return nil
}

func runDemo(args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] != "local" {
		return usageError(hasBoolFlag(args, "json"), stdout, "usage: codencer demo local [--json] [--bin-dir <path>] [--keep]")
	}
	parsed, err := parseArgs(args[1:], []string{"json", "keep"}, []string{"bin-dir"})
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return usageError(parsed.bool("json"), stdout, "demo local does not accept positional arguments")
	}
	report, err := setuppkg.DemoLocal(contextBackground(), setuppkg.DemoOptions{BinDir: parsed.value("bin-dir"), Keep: parsed.bool("keep")})
	return finishSetupReport(stdout, parsed.bool("json"), report, err)
}

func finishSetupReport(stdout io.Writer, asJSON bool, report setuppkg.Report, err error) error {
	if err != nil {
		return jsonAwareError(asJSON, stdout, exitFailed, err.Error())
	}
	if asJSON {
		if err := writeJSON(stdout, report); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(stdout, "mode: %s\nok: %t\nconfigured: %t\n", report.Mode, report.OK, report.Configured)
		for _, step := range report.Steps {
			fmt.Fprintf(stdout, "%-14s %s %s\n", strings.ToUpper(step.Status), step.ID, step.Detail)
		}
		for _, command := range report.NextCommands {
			fmt.Fprintf(stdout, "next: %s\n", command)
		}
	}
	if report.ExitCode != exitSuccess {
		return exitError{code: report.ExitCode, message: "setup failed", printed: true}
	}
	return nil
}

func finishActivationReport(stdout io.Writer, asJSON bool, report activation.Report, err error) error {
	if err != nil {
		return jsonAwareError(asJSON, stdout, exitFailed, err.Error())
	}
	if asJSON {
		if err := writeJSON(stdout, report); err != nil {
			return err
		}
	} else {
		printActivationReport(stdout, report)
	}
	if report.ExitCode != exitSuccess {
		return exitError{code: report.ExitCode, message: "activation check failed", printed: true}
	}
	return nil
}

func serviceNameFromPositionals(positionals []string, all bool) (string, error) {
	if all {
		if len(positionals) != 0 {
			return "", fmt.Errorf("--all does not accept a service argument")
		}
		return "", nil
	}
	if len(positionals) > 1 {
		return "", fmt.Errorf("expected at most one service")
	}
	if len(positionals) == 0 {
		return "", nil
	}
	return positionals[0], nil
}

func repoRootForCommand(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return wd, nil
	}
	return value, nil
}

func contextBackground() context.Context {
	return context.Background()
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: codencer <version|init|login|whoami|logout|paths|config|doctor|status|project|machine|connector|gateway|run|submit|run-plan|profile|service|watchdog|recover|live|readiness|setup|activation|accept|proof|demo> [flags]")
}

func printPaths(w io.Writer, paths local.Paths) {
	fmt.Fprintf(w, "home:           %s\n", paths.Home)
	fmt.Fprintf(w, "projects_file:  %s\n", paths.ProjectsFile)
	fmt.Fprintf(w, "machine_file:   %s\n", paths.MachineFile)
	fmt.Fprintf(w, "config_file:    %s\n", paths.ConfigFile)
	fmt.Fprintf(w, "logs_dir:       %s\n", paths.LogsDir)
	fmt.Fprintf(w, "runtime_dir:    %s\n", paths.RuntimeDir)
	fmt.Fprintf(w, "tokens_dir:     %s\n", paths.TokensDir)
	fmt.Fprintf(w, "artifacts_dir:  %s\n", paths.ArtifactsDir)
	if paths.RepoRuntimeDir != "" {
		fmt.Fprintf(w, "repo_runtime:   %s\n", paths.RepoRuntimeDir)
	}
}

func printProject(w io.Writer, p projectpkg.Project) {
	fmt.Fprintf(w, "id:              %s\n", p.ID)
	fmt.Fprintf(w, "name:            %s\n", p.Name)
	fmt.Fprintf(w, "repo_root:       %s\n", p.RepoRoot)
	fmt.Fprintf(w, "default_adapter: %s\n", p.DefaultAdapter)
	fmt.Fprintf(w, "adapter_profile: %s\n", p.AdapterProfile)
	if p.DaemonURL != "" {
		fmt.Fprintf(w, "daemon_url:      %s\n", p.DaemonURL)
	}
	fmt.Fprintf(w, "shared_to_relay: %t\n", p.SharedToRelay)
	if p.RelayInstanceID != "" {
		fmt.Fprintf(w, "relay_instance:  %s\n", p.RelayInstanceID)
	}
	if p.MachineID != "" {
		fmt.Fprintf(w, "machine_id:      %s\n", p.MachineID)
	}
	if p.HostLabel != "" {
		fmt.Fprintf(w, "host_label:      %s\n", p.HostLabel)
	}
}

func printMachine(w io.Writer, machine local.MachineIdentity, path string) {
	fmt.Fprintf(w, "machine_id: %s\n", machine.MachineID)
	fmt.Fprintf(w, "host_label: %s\n", machine.HostLabel)
	fmt.Fprintf(w, "hostname:   %s\n", machine.Hostname)
	fmt.Fprintf(w, "os:         %s\n", machine.OS)
	fmt.Fprintf(w, "arch:       %s\n", machine.Arch)
	fmt.Fprintf(w, "path:       %s\n", path)
}

func printConnectorEnrollReport(w io.Writer, report *connectorops.EnrollReport) {
	fmt.Fprintf(w, "connector_id: %s\n", report.ConnectorID)
	fmt.Fprintf(w, "machine_id:   %s\n", report.MachineID)
	fmt.Fprintf(w, "relay_url:    %s\n", report.RelayURL)
	fmt.Fprintf(w, "config:       %s\n", report.ConfigPath)
	fmt.Fprintf(w, "home:         %s\n", report.CodencerHome)
	for _, warning := range report.Warnings {
		fmt.Fprintf(w, "warning:      %s\n", warning)
	}
}

func printConnectorStatusReport(w io.Writer, report *connectorops.StatusReport) {
	if report.Status == nil {
		fmt.Fprintf(w, "config: %s\n", report.ConfigPath)
		return
	}
	fmt.Fprintf(w, "connector_id: %s\n", report.Status.ConnectorID)
	fmt.Fprintf(w, "machine_id:   %s\n", report.Status.MachineID)
	fmt.Fprintf(w, "relay_url:    %s\n", report.Status.RelayURL)
	fmt.Fprintf(w, "state:        %s\n", report.Status.SessionState)
	if report.Status.LastError != "" {
		fmt.Fprintf(w, "last_error:   %s\n", report.Status.LastError)
	}
}

func printDoctor(w io.Writer, report local.DoctorReport) {
	for _, check := range report.Checks {
		fmt.Fprintf(w, "%-9s %-24s %s\n", strings.ToUpper(check.Status), check.Name, check.Detail)
	}
	fmt.Fprintf(w, "summary: errors=%d warnings=%d skipped=%d unknown=%d\n", report.Summary.Errors, report.Summary.Warnings, report.Summary.Skipped, report.Summary.Unknown)
}

func printStatus(w io.Writer, report local.StatusReport) {
	fmt.Fprintf(w, "status:        %s\n", report.Status)
	fmt.Fprintf(w, "codencer_home: %s\n", report.Paths.Home)
	fmt.Fprintf(w, "projects:      %d\n", report.ProjectCount)
	if report.CurrentProjectID != "" {
		fmt.Fprintf(w, "current:       %s\n", report.CurrentProjectID)
	}
	if report.Project != nil {
		fmt.Fprintf(w, "project:       %s (%s)\n", report.Project.ID, report.Project.RepoRoot)
		fmt.Fprintf(w, "adapter:       %s/%s\n", report.Project.DefaultAdapter, report.Project.AdapterProfile)
	}
	if report.Machine != nil {
		fmt.Fprintf(w, "machine:       %s host_label=%s hostname=%s\n", report.Machine.MachineID, report.Machine.HostLabel, report.Machine.Hostname)
	}
	fmt.Fprintf(w, "daemon:        %s %s\n", report.Daemon.Status, report.Daemon.Detail)
	fmt.Fprintf(w, "relay:         %s %s\n", report.Relay.Status, report.Relay.Detail)
	for _, executor := range report.Executors {
		fmt.Fprintf(w, "executor:      %s %s %s\n", executor.ID, executor.Status, executor.Detail)
	}
	for _, warning := range report.Warnings {
		fmt.Fprintf(w, "warning:       %s\n", warning)
	}
}

func printServiceReport(w io.Writer, report supervisor.ServiceReport) {
	fmt.Fprintf(w, "action:  %s\n", report.Action)
	fmt.Fprintf(w, "manager: %s\n", report.Platform.ServiceManager)
	for _, service := range report.Services {
		fmt.Fprintf(w, "service: %s configured=%t installed=%t state=%s health=%s\n", service.Name, service.Configured, service.Installed, service.ObservedState, service.Health)
		if service.LastError != "" {
			fmt.Fprintf(w, "error:   %s\n", service.LastError)
		}
	}
	for _, warning := range report.Warnings {
		fmt.Fprintf(w, "warning: %s\n", warning)
	}
}

func printWatchdogReport(w io.Writer, report supervisor.WatchdogReport) {
	for _, check := range report.Checks {
		fmt.Fprintf(w, "%-9s %-24s %s\n", strings.ToUpper(check.Status), check.Name, check.Message)
	}
	for _, blocker := range report.Blockers {
		fmt.Fprintf(w, "blocker: %s %s\n", blocker.Type, blocker.Message)
	}
}

func printRecoveryReport(w io.Writer, report supervisor.RecoveryReport) {
	for _, action := range report.Actions {
		fmt.Fprintf(w, "action: %s target=%s safe=%t done=%t reason=%s\n", action.Type, action.Target, action.Safe, action.Done, action.Reason)
	}
	for _, blocker := range report.Blockers {
		fmt.Fprintf(w, "blocker: %s %s\n", blocker.Type, blocker.Message)
	}
}

func printLiveReport(w io.Writer, report live.Report) {
	fmt.Fprintf(w, "profile: %s\n", report.Profile)
	fmt.Fprintf(w, "ok: %t\n", report.OK)
	for _, check := range report.Checks {
		fmt.Fprintf(w, "%-14s %-28s %s\n", strings.ToUpper(check.Status), check.ID, check.Reason)
		if check.Blocker != nil {
			fmt.Fprintf(w, "blocker: %s %s\n", check.Blocker.Type, check.Blocker.Message)
		}
	}
	fmt.Fprintf(w, "summary: passed=%d failed=%d blocked=%d skipped=%d\n", report.Summary.Passed, report.Summary.Failed, report.Summary.Blocked, report.Summary.Skipped)
	if report.ReportPath != "" {
		fmt.Fprintf(w, "report: %s\n", report.ReportPath)
	}
}

func printActivationReport(w io.Writer, report activation.Report) {
	fmt.Fprintf(w, "mode: %s\nok: %t\n", report.Mode, report.OK)
	if report.Relay != "" {
		fmt.Fprintf(w, "relay: %s\n", report.Relay)
	}
	if report.MCPURL != "" {
		fmt.Fprintf(w, "mcp: %s\n", report.MCPURL)
	}
	for _, check := range report.Checks {
		fmt.Fprintf(w, "%-14s %-32s %s\n", strings.ToUpper(check.Status), check.ID, check.Detail)
	}
	if report.PackagePath != "" {
		fmt.Fprintf(w, "package: %s\n", report.PackagePath)
	}
}

func finishExecutionReport(stdout io.Writer, asJSON bool, report localexec.ExecutionReport, err error) error {
	if err != nil {
		return finishLocalexecError(stdout, asJSON, err)
	}
	if asJSON {
		if writeErr := writeJSON(stdout, report); writeErr != nil {
			return writeErr
		}
	} else {
		printExecutionReport(stdout, report)
	}
	if report.ExitCode != exitSuccess {
		return exitError{code: report.ExitCode, message: reportMessage(report), printed: true}
	}
	return nil
}

func finishRunPlanReport(stdout io.Writer, asJSON bool, report localexec.RunPlanReport, err error) error {
	if err != nil {
		return finishLocalexecError(stdout, asJSON, err)
	}
	if asJSON {
		if writeErr := writeJSON(stdout, report); writeErr != nil {
			return writeErr
		}
	} else {
		printRunPlanReport(stdout, report)
	}
	if report.ExitCode != exitSuccess {
		return exitError{code: report.ExitCode, message: runPlanMessage(report), printed: true}
	}
	return nil
}

func finishLocalexecError(stdout io.Writer, asJSON bool, err error) error {
	report := localexec.ErrorReportFor(err)
	if asJSON {
		_ = writeJSON(stdout, report)
		return exitError{code: report.ExitCode, message: report.Error, printed: true}
	}
	return exitError{code: report.ExitCode, message: report.Error}
}

func printExecutionReport(w io.Writer, report localexec.ExecutionReport) {
	fmt.Fprintf(w, "status: %s\n", report.Status)
	if report.Project != nil {
		fmt.Fprintf(w, "project: %s %s\n", report.Project.ID, report.Project.RepoRoot)
	}
	if report.Run != nil {
		fmt.Fprintf(w, "run: %s %s\n", report.Run.ID, report.Run.State)
	}
	for _, run := range report.Runs {
		fmt.Fprintf(w, "run: %s %s %s\n", run.ID, run.ProjectID, run.State)
	}
	for _, step := range report.Steps {
		fmt.Fprintf(w, "step: %s %s %s\n", step.ID, step.Adapter, step.State)
	}
	if report.Task != nil {
		fmt.Fprintf(w, "task: %s %s step=%s profile=%s\n", report.Task.TaskID, report.Task.Status, report.Task.StepID, report.Task.Profile)
		if report.Task.Summary != "" {
			fmt.Fprintf(w, "summary: %s\n", report.Task.Summary)
		}
	}
	if report.Profile != nil {
		fmt.Fprintf(w, "profile: %s adapter=%s daemon_adapter=%s\n", report.Profile.ID, report.Profile.Adapter, report.Profile.DaemonAdapter)
	}
	for _, profile := range report.Profiles {
		fmt.Fprintf(w, "profile: %-20s adapter=%s daemon_adapter=%s\n", profile.ID, profile.Adapter, profile.DaemonAdapter)
	}
	if report.Blocker != nil {
		fmt.Fprintf(w, "blocker: %s %s\n", report.Blocker.Type, report.Blocker.Message)
	}
}

func printRunPlanReport(w io.Writer, report localexec.RunPlanReport) {
	fmt.Fprintf(w, "status: %s\n", report.Status)
	if report.Run != nil {
		fmt.Fprintf(w, "run: %s %s\n", report.Run.ID, report.Run.State)
	}
	for _, task := range report.Tasks {
		fmt.Fprintf(w, "task: %s %s step=%s\n", task.TaskID, task.Status, task.StepID)
	}
	if report.Blocker != nil {
		fmt.Fprintf(w, "blocker: %s %s\n", report.Blocker.Type, report.Blocker.Message)
	}
	if report.ReportPath != "" {
		fmt.Fprintf(w, "report: %s\n", report.ReportPath)
	}
}

func reportMessage(report localexec.ExecutionReport) string {
	if report.Blocker != nil {
		return report.Blocker.Message
	}
	if report.Task != nil && report.Task.Blocker != nil {
		return report.Task.Blocker.Message
	}
	if report.Status != "" {
		return report.Status
	}
	return "command failed"
}

func runPlanMessage(report localexec.RunPlanReport) string {
	if report.Blocker != nil {
		return report.Blocker.Message
	}
	if report.Status != "" {
		return report.Status
	}
	return "run-plan failed"
}

func adapterProfileAlias(adapterProfile, profile string) (string, error) {
	adapterProfile = strings.TrimSpace(adapterProfile)
	profile = strings.TrimSpace(profile)
	if adapterProfile != "" && profile != "" && adapterProfile != profile {
		return "", fmt.Errorf("--profile and --adapter-profile must match when both are set")
	}
	if adapterProfile != "" {
		return adapterProfile, nil
	}
	return profile, nil
}

func resolveSessionPath(configOverride string) (local.Paths, string, error) {
	paths, err := local.ResolvePaths("", configOverride)
	if err != nil {
		return local.Paths{}, "", err
	}
	return paths, account.SessionPath(paths.Home), nil
}

func loadCodencerSession(configOverride string) (local.Paths, string, account.Session, error) {
	paths, sessionPath, err := resolveSessionPath(configOverride)
	if err != nil {
		return local.Paths{}, "", account.Session{}, err
	}
	session, err := account.LoadSession(sessionPath)
	if err != nil {
		return paths, sessionPath, account.Session{}, err
	}
	return paths, sessionPath, session, nil
}

func parseDurationOrDefault(value string, fallback time.Duration) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds <= 0 {
			return 0, fmt.Errorf("duration must be positive")
		}
		return time.Duration(seconds) * time.Second, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, err
	}
	if duration <= 0 {
		return 0, fmt.Errorf("duration must be positive")
	}
	return duration, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func writeJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func usageError(asJSON bool, stdout io.Writer, message string) error {
	return jsonAwareError(asJSON, stdout, exitUsage, message)
}

func jsonAwareError(asJSON bool, stdout io.Writer, code int, message string) error {
	if asJSON {
		_ = writeJSON(stdout, map[string]string{"error": message})
		return exitError{code: code, message: message, printed: true}
	}
	return exitError{code: code, message: message}
}

type parsedArgs struct {
	positionals []string
	bools       map[string]bool
	values      map[string]string
}

func (p parsedArgs) bool(name string) bool {
	return p.bools[name]
}

func (p parsedArgs) value(name string) string {
	return p.values[name]
}

func parseArgs(args []string, boolFlags, valueFlags []string) (parsedArgs, error) {
	parsed := parsedArgs{bools: map[string]bool{}, values: map[string]string{}}
	boolSet := toSet(boolFlags)
	valueSet := toSet(valueFlags)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "--") || arg == "--" {
			if arg == "--" {
				parsed.positionals = append(parsed.positionals, args[i+1:]...)
				break
			}
			parsed.positionals = append(parsed.positionals, arg)
			continue
		}
		nameValue := strings.TrimPrefix(arg, "--")
		name := nameValue
		value := ""
		if idx := strings.Index(nameValue, "="); idx >= 0 {
			name = nameValue[:idx]
			value = nameValue[idx+1:]
		}
		if boolSet[name] {
			if value != "" {
				return parsed, fmt.Errorf("--%s does not accept a value", name)
			}
			parsed.bools[name] = true
			continue
		}
		if valueSet[name] {
			if value == "" {
				if i+1 >= len(args) {
					return parsed, fmt.Errorf("--%s requires a value", name)
				}
				i++
				value = args[i]
			}
			parsed.values[name] = value
			continue
		}
		return parsed, fmt.Errorf("unknown flag --%s", name)
	}
	return parsed, nil
}

func hasBoolFlag(args []string, name string) bool {
	target := "--" + name
	for _, arg := range args {
		if arg == target {
			return true
		}
	}
	return false
}

func toSet(values []string) map[string]bool {
	set := map[string]bool{}
	for _, value := range values {
		set[value] = true
	}
	return set
}
