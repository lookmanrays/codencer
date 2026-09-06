package supervisor

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"agent-bridge/internal/local"
	projectpkg "agent-bridge/internal/project"
)

type runtimeContext struct {
	paths      local.Paths
	config     local.Config
	registry   *projectpkg.Registry
	project    *projectpkg.Project
	resolution string
	repoRoot   string
	platform   PlatformInfo
	specs      []ServiceSpec
	warnings   []string
}

func resolveRuntimeContext(opts Options) (*runtimeContext, error) {
	repoRoot := strings.TrimSpace(opts.RepoRoot)
	if repoRoot == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		repoRoot = wd
	}
	paths, err := local.ResolvePaths(repoRoot, opts.ConfigPath)
	if err != nil {
		return nil, err
	}
	cfg, err := local.LoadConfig(paths.ConfigFile)
	if err != nil {
		return nil, err
	}
	registry, err := projectpkg.LoadRegistry(paths.ProjectsFile)
	if err != nil {
		return nil, err
	}
	ctx := &runtimeContext{
		paths:    paths,
		config:   cfg,
		registry: registry,
		repoRoot: repoRoot,
		platform: platformOrDetect(firstNonEmpty(opts.Manager, cfg.Runtime.ServiceManager, ManagerAuto), opts.Runner, opts.Platform),
	}
	if len(registry.Projects) > 0 || strings.TrimSpace(opts.ProjectID) != "" {
		resolved, err := projectpkg.ResolveProject(registry, projectpkg.ResolveOptions{ExplicitID: opts.ProjectID, CWD: repoRoot})
		if err != nil {
			ctx.warnings = append(ctx.warnings, err.Error())
		} else {
			ctx.project = &resolved.Project
			ctx.resolution = resolved.Source
		}
	}
	ctx.specs = buildSpecs(ctx, opts)
	return ctx, nil
}

func buildSpecs(ctx *runtimeContext, opts Options) []ServiceSpec {
	servicesDir := filepath.Join(ctx.paths.RuntimeDir, "services")
	specs := []ServiceSpec{
		daemonSpec(ctx, opts, servicesDir),
		relaySpec(ctx, opts, servicesDir),
		connectorSpec(ctx, opts, servicesDir),
	}
	for i := range specs {
		specs[i].Label = "io.codencer." + specs[i].Name
		specs[i].UnitName = "codencer-" + specs[i].Name + ".service"
		if home, err := os.UserHomeDir(); err == nil {
			specs[i].LaunchAgentPath = filepath.Join(home, "Library", "LaunchAgents", specs[i].Label+".plist")
			specs[i].SystemdUnitPath = filepath.Join(home, ".config", "systemd", "user", specs[i].UnitName)
		}
	}
	return specs
}

func daemonSpec(ctx *runtimeContext, opts Options, servicesDir string) ServiceSpec {
	spec := baseSpec(ctx, ServiceDaemon)
	spec.ConfigPath = filepath.Join(servicesDir, "daemon.config.json")
	spec.Configured = ctx.project != nil
	spec.Dependencies = nil
	if ctx.project == nil {
		spec.LastError = "no project resolved"
		return spec
	}
	spec.WorkingDir = ctx.project.RepoRoot
	spec.Binary = resolveBinary(ServiceDaemon, "orchestratord", ctx, opts)
	spec.Args = []string{"--config", spec.ConfigPath, "--repo-root", ctx.project.RepoRoot}
	spec.HealthURL = daemonURL(ctx) + "/health"
	if spec.Binary == "" {
		spec.LastError = "orchestratord binary not found"
	}
	return spec
}

func relaySpec(ctx *runtimeContext, opts Options, _ string) ServiceSpec {
	spec := baseSpec(ctx, ServiceRelay)
	spec.ConfigPath = strings.TrimSpace(ctx.config.RelayConfigPath)
	spec.Configured = spec.ConfigPath != ""
	if !spec.Configured {
		spec.LastError = "relay config not configured"
		return spec
	}
	if _, err := os.Stat(spec.ConfigPath); err != nil {
		spec.LastError = "relay config not readable: " + err.Error()
		spec.Configured = false
		return spec
	}
	spec.Binary = resolveBinary(ServiceRelay, "codencer-relayd", ctx, opts)
	spec.Args = []string{"serve", "--config", spec.ConfigPath}
	spec.WorkingDir = ctx.paths.Home
	if spec.Binary == "" {
		spec.LastError = "codencer-relayd binary not found"
	}
	return spec
}

func connectorSpec(ctx *runtimeContext, opts Options, _ string) ServiceSpec {
	spec := baseSpec(ctx, ServiceConnector)
	spec.ConfigPath = strings.TrimSpace(ctx.config.ConnectorConfigPath)
	spec.Configured = spec.ConfigPath != ""
	spec.Dependencies = []string{ServiceDaemon, ServiceRelay}
	if !spec.Configured {
		spec.LastError = "connector config not configured"
		return spec
	}
	if _, err := os.Stat(spec.ConfigPath); err != nil {
		spec.LastError = "connector config not readable: " + err.Error()
		spec.Configured = false
		return spec
	}
	spec.Binary = resolveBinary(ServiceConnector, "codencer-connectord", ctx, opts)
	spec.Args = []string{"run", "--config", spec.ConfigPath}
	spec.WorkingDir = ctx.paths.Home
	if spec.Binary == "" {
		spec.LastError = "codencer-connectord binary not found"
	}
	return spec
}

func baseSpec(ctx *runtimeContext, name string) ServiceSpec {
	env := map[string]string{
		local.HomeEnvName: ctx.paths.Home,
	}
	// launchd/systemd do not inherit the user's PATH; pass it through so
	// adapters that resolve external binaries (e.g. OPENCODE_BINARY) work.
	if currentPath := os.Getenv("PATH"); currentPath != "" {
		env["PATH"] = currentPath
	}
	// Propagate executor-specific binary and simulation overrides so that
	// adapters work correctly under launchd/systemd.
	for _, key := range []string{"OPENCODE_BINARY", "OPENCODE_SIMULATION_MODE"} {
		if v := os.Getenv(key); v != "" {
			env[key] = v
		}
	}
	return ServiceSpec{
		Name:       name,
		Configured: true,
		Env:        env,
		WorkingDir: ctx.paths.Home,
		StdoutLog:  filepath.Join(ctx.paths.LogsDir, name+".stdout.log"),
		StderrLog:  filepath.Join(ctx.paths.LogsDir, name+".stderr.log"),
	}
}

func selectSpecs(specs []ServiceSpec, service string, all bool) ([]ServiceSpec, error) {
	if all || strings.TrimSpace(service) == "" {
		return specs, nil
	}
	for _, spec := range specs {
		if spec.Name == service {
			return []ServiceSpec{spec}, nil
		}
	}
	return nil, fmt.Errorf("unknown service %q", service)
}

func resolveBinary(serviceName, binaryName string, ctx *runtimeContext, opts Options) string {
	candidates := []string{}
	if opts.BinDir != "" {
		candidates = append(candidates, filepath.Join(opts.BinDir, binaryName))
	}
	if ctx.config.BinaryPaths != nil {
		candidates = append(candidates, ctx.config.BinaryPaths[serviceName], ctx.config.BinaryPaths[binaryName])
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), binaryName))
	}
	candidates = append(candidates, filepath.Join(ctx.repoRoot, "bin", binaryName), binaryName)
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if filepath.IsAbs(candidate) || strings.Contains(candidate, string(filepath.Separator)) {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				if abs, err := filepath.Abs(candidate); err == nil {
					return filepath.Clean(abs)
				}
				return filepath.Clean(candidate)
			}
			continue
		}
		if resolved, err := exec.LookPath(candidate); err == nil {
			return resolved
		}
	}
	return ""
}

func daemonURL(ctx *runtimeContext) string {
	if ctx.project != nil && strings.TrimSpace(ctx.project.DaemonURL) != "" {
		return strings.TrimRight(strings.TrimSpace(ctx.project.DaemonURL), "/")
	}
	return strings.TrimRight(strings.TrimSpace(ctx.config.DefaultDaemonURL), "/")
}

func daemonListen(daemonBaseURL string) (string, int) {
	u, err := url.Parse(daemonBaseURL)
	if err != nil {
		return "127.0.0.1", 8085
	}
	host := u.Hostname()
	if host == "" {
		host = "127.0.0.1"
	}
	port := 8085
	if raw := u.Port(); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			port = parsed
		}
	}
	return host, port
}

func writeGeneratedConfigs(ctx *runtimeContext, specs []ServiceSpec, dryRun bool) ([]RecoveryAction, error) {
	actions := []RecoveryAction{}
	for _, spec := range specs {
		if spec.Name != ServiceDaemon || !spec.Configured || ctx.project == nil {
			continue
		}
		host, port := daemonListen(daemonURL(ctx))
		payload := map[string]any{
			"log_level":      "info",
			"db_path":        filepath.Join(ctx.project.RepoRoot, ".codencer", "codencer.db"),
			"artifact_root":  filepath.Join(ctx.project.RepoRoot, ".codencer", "artifacts"),
			"workspace_root": filepath.Join(ctx.project.RepoRoot, ".codencer", "workspace"),
			"repo_root":      ctx.project.RepoRoot,
			"host":           host,
			"port":           port,
		}
		action := RecoveryAction{
			Type:   "write_generated_config",
			Target: spec.ConfigPath,
			Safe:   true,
			Reason: "daemon service config is derived from the project registry and local config",
			Done:   false,
		}
		if !dryRun {
			if err := writeJSONFile(spec.ConfigPath, payload, 0600); err != nil {
				return actions, err
			}
			action.Done = true
		}
		actions = append(actions, action)
	}
	return actions, nil
}

func writeJSONFile(path string, value any, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*.json")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(name)
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		_ = os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		_ = os.Remove(name)
		return err
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
