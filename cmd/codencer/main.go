package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"agent-bridge/internal/acceptance"
	"agent-bridge/internal/activation"
	"agent-bridge/internal/app"
	"agent-bridge/internal/buildinfo"
	"agent-bridge/internal/connectorops"
	"agent-bridge/internal/live"
	"agent-bridge/internal/local"
	"agent-bridge/internal/localexec"
	projectpkg "agent-bridge/internal/project"
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
	case "connector":
		return runConnector(args[1:], stdout)
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
	if result.ConfigCreated || result.RegistryCreated {
		fmt.Fprintln(stdout, "Initialized local production files.")
	} else {
		fmt.Fprintln(stdout, "Local production files already exist.")
	}
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
		return usageError(hasBoolFlag(args, "json"), stdout, "usage: codencer project <init|list|get|use|status|share|unshare|remove>")
	}
	switch args[0] {
	case "init":
		return runProjectInit(args[1:], stdout)
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

func runConnector(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return usageError(hasBoolFlag(args, "json"), stdout, "usage: codencer connector <enroll|run|status|config show> [flags]")
	}
	switch args[0] {
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

func runProjectInit(args []string, stdout io.Writer) error {
	parsed, err := parseArgs(args, []string{"json", "force", "share-to-relay"}, []string{"id", "repo", "adapter", "name", "adapter-profile", "profile", "daemon-url", "relay-instance-id", "config"})
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return usageError(parsed.bool("json"), stdout, "project init does not accept positional arguments")
	}
	paths, err := local.ResolvePaths("", parsed.value("config"))
	if err != nil {
		return err
	}
	cfg, err := local.LoadConfig(paths.ConfigFile)
	if err != nil {
		return err
	}
	daemonURL := parsed.value("daemon-url")
	if daemonURL == "" {
		daemonURL = cfg.DefaultDaemonURL
	}
	adapterProfile, err := adapterProfileAlias(parsed.value("adapter-profile"), parsed.value("profile"))
	if err != nil {
		return jsonAwareError(parsed.bool("json"), stdout, exitUsage, err.Error())
	}
	next, warnings, err := projectpkg.NewProject(projectpkg.ProjectOptions{
		ID:              parsed.value("id"),
		Name:            parsed.value("name"),
		RepoRoot:        parsed.value("repo"),
		DefaultAdapter:  parsed.value("adapter"),
		AdapterProfile:  adapterProfile,
		DaemonURL:       daemonURL,
		RelayInstanceID: parsed.value("relay-instance-id"),
		SharedToRelay:   parsed.bool("share-to-relay"),
	})
	if err != nil {
		return jsonAwareError(parsed.bool("json"), stdout, exitUsage, err.Error())
	}
	registry, err := projectpkg.LoadRegistry(paths.ProjectsFile)
	if err != nil {
		return err
	}
	saved, err := projectpkg.UpsertProject(registry, next, parsed.bool("force"), time.Now().UTC())
	if err != nil {
		return jsonAwareError(parsed.bool("json"), stdout, exitUsage, err.Error())
	}
	if err := projectpkg.SaveRegistry(paths.ProjectsFile, registry); err != nil {
		return err
	}
	payload := map[string]any{
		"project":            saved,
		"warnings":           warnings,
		"registry_path":      paths.ProjectsFile,
		"current_project_id": registry.CurrentProjectID,
	}
	if parsed.bool("json") {
		return writeJSON(stdout, payload)
	}
	fmt.Fprintf(stdout, "Registered project %s at %s\n", saved.ID, saved.RepoRoot)
	for _, warning := range warnings {
		fmt.Fprintf(stdout, "warning: %s\n", warning)
	}
	return nil
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
	registry, err := projectpkg.LoadRegistry(paths.ProjectsFile)
	if err != nil {
		return err
	}
	payload := map[string]any{
		"current_project_id": registry.CurrentProjectID,
		"projects":           projectpkg.ListProjects(registry),
		"registry_path":      paths.ProjectsFile,
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
	registry, err := projectpkg.LoadRegistry(paths.ProjectsFile)
	if err != nil {
		return err
	}
	p, err := projectpkg.GetProject(registry, parsed.positionals[0])
	if err != nil {
		return jsonAwareError(parsed.bool("json"), stdout, exitUsage, err.Error())
	}
	if parsed.bool("json") {
		return writeJSON(stdout, p)
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
	registry, err := projectpkg.LoadRegistry(paths.ProjectsFile)
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
	payload := map[string]any{"current_project_id": p.ID, "project": p}
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
	registry, err := projectpkg.LoadRegistry(paths.ProjectsFile)
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
	payload := map[string]any{"project": project, "shared_to_relay": project.SharedToRelay}
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
	registry, err := projectpkg.LoadRegistry(paths.ProjectsFile)
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
	payload := map[string]any{"project": project, "shared_to_relay": project.SharedToRelay}
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
	registry, err := projectpkg.LoadRegistry(paths.ProjectsFile)
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
		return usageError(hasBoolFlag(args, "json"), stdout, "usage: codencer setup <local|relay|mcp> [flags]")
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
		return usageError(hasBoolFlag(args, "json"), stdout, "usage: codencer activation <check|package|chatgpt|codex|claude-code> [--json]")
	}
	parsed, err := parseArgs(args[1:], []string{"json", "run-fake-manifest", "check-oauth", "check-chatgpt-readiness"}, []string{"relay", "mcp-url", "token-env", "token", "project", "auth"})
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return usageError(parsed.bool("json"), stdout, "activation command does not accept positional arguments")
	}
	opts := activation.Options{
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
	fmt.Fprintln(w, "Usage: codencer <version|init|paths|config|doctor|status|project|connector|run|submit|run-plan|profile|service|watchdog|recover|live|readiness|setup|activation|accept|proof|demo> [flags]")
}

func printPaths(w io.Writer, paths local.Paths) {
	fmt.Fprintf(w, "home:           %s\n", paths.Home)
	fmt.Fprintf(w, "projects_file:  %s\n", paths.ProjectsFile)
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
