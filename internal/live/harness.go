package live

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/local"
	"agent-bridge/internal/project"
)

type workspaceHarness struct {
	Root      string
	RepoRoot  string
	Home      string
	StateRoot string
	Paths     local.Paths
	Keep      bool
}

type processHandle struct {
	cmd     *exec.Cmd
	logPath string
}

func newWorkspaceHarness(opts Options) (*workspaceHarness, error) {
	root, err := os.MkdirTemp("", "codencer-live.")
	if err != nil {
		return nil, err
	}
	h := &workspaceHarness{
		Root:      root,
		RepoRoot:  filepath.Join(root, "repo"),
		Home:      filepath.Join(root, "home"),
		StateRoot: filepath.Join(root, "state"),
		Keep:      envEnabled(EnvKeepWorkspace),
	}
	for _, dir := range []string{h.RepoRoot, h.Home, h.StateRoot} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			h.Cleanup()
			return nil, err
		}
	}
	if err := initGitRepo(h.RepoRoot); err != nil {
		h.Cleanup()
		return nil, err
	}
	paths, err := local.ResolvePathsForHome(h.RepoRoot, "", h.Home)
	if err != nil {
		h.Cleanup()
		return nil, err
	}
	if _, err := local.EnsureHome(paths, now(opts)); err != nil {
		h.Cleanup()
		return nil, err
	}
	h.Paths = paths
	return h, nil
}

func (h *workspaceHarness) Cleanup() {
	if h == nil || h.Keep {
		return
	}
	_ = os.RemoveAll(h.Root)
}

func (h *workspaceHarness) registerProject(id, adapter, profile, daemonURL string, share bool) error {
	next, _, err := project.NewProject(project.ProjectOptions{
		ID:             id,
		Name:           id,
		RepoRoot:       h.RepoRoot,
		DefaultAdapter: adapter,
		AdapterProfile: profile,
		DaemonURL:      daemonURL,
		SharedToRelay:  share,
	})
	if err != nil {
		return err
	}
	registry, err := project.LoadRegistry(h.Paths.ProjectsFile)
	if err != nil {
		return err
	}
	if _, err := project.UpsertProject(registry, next, true, time.Now().UTC()); err != nil {
		return err
	}
	return project.SaveRegistry(h.Paths.ProjectsFile, registry)
}

func initGitRepo(repo string) error {
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("codencer live verification\n"), 0644); err != nil {
		return err
	}
	for _, command := range [][]string{
		{"git", "-C", repo, "init", "-q"},
		{"git", "-C", repo, "add", "README.md"},
		{"git", "-C", repo, "-c", "user.name=Codencer", "-c", "user.email=codencer@example.invalid", "commit", "-q", "-m", "initial"},
	} {
		cmd := exec.Command(command[0], command[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %w: %s", strings.Join(command, " "), err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}

func startDaemon(ctx context.Context, h *workspaceHarness, opts Options) (*processHandle, string, error) {
	port, err := freePort()
	if err != nil {
		return nil, "", err
	}
	daemonURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	binary, err := resolveBinary(opts, "orchestratord")
	if err != nil {
		return nil, "", err
	}
	configPath := filepath.Join(h.Root, "daemon.json")
	config := map[string]any{
		"log_level":      "error",
		"db_path":        filepath.Join(h.StateRoot, "codencer.db"),
		"artifact_root":  filepath.Join(h.StateRoot, "artifacts"),
		"workspace_root": filepath.Join(h.StateRoot, "workspace"),
		"repo_root":      h.RepoRoot,
		"host":           "127.0.0.1",
		"port":           port,
	}
	if err := writeJSONFile(configPath, config, 0600); err != nil {
		return nil, "", err
	}
	logPath := filepath.Join(h.Root, "daemon.log")
	cmd := exec.CommandContext(ctx, binary, "--config", configPath, "--repo-root", h.RepoRoot)
	cmd.Env = append(os.Environ(),
		"PORT="+fmt.Sprint(port),
		"HOST=127.0.0.1",
		"DB_PATH="+filepath.Join(h.StateRoot, "codencer.db"),
		"ARTIFACT_ROOT="+filepath.Join(h.StateRoot, "artifacts"),
		"WORKSPACE_ROOT="+filepath.Join(h.StateRoot, "workspace"),
		"LOG_LEVEL=error",
		"REPO_ROOT="+h.RepoRoot,
	)
	if err := startProcess(cmd, logPath); err != nil {
		return nil, "", err
	}
	handle := &processHandle{cmd: cmd, logPath: logPath}
	if err := waitHTTP(ctx, daemonURL+"/health", "", 15*time.Second); err != nil {
		handle.Stop()
		return nil, "", fmt.Errorf("daemon did not become healthy: %w; log: %s", err, readSmall(logPath))
	}
	return handle, daemonURL, nil
}

func startProcess(cmd *exec.Cmd, logPath string) error {
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return err
	}
	logFile, err := os.Create(logPath)
	if err != nil {
		return err
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return err
	}
	go func() {
		_ = cmd.Wait()
		_ = logFile.Close()
	}()
	return nil
}

func (p *processHandle) Stop() {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return
	}
	_ = p.cmd.Process.Kill()
}

func resolveBinary(opts Options, name string) (string, error) {
	candidates := []string{}
	if opts.BinDir != "" {
		candidates = append(candidates, filepath.Join(opts.BinDir, name))
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), name))
	}
	if opts.RepoRoot != "" {
		candidates = append(candidates, filepath.Join(opts.RepoRoot, "bin", name))
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(wd, "bin", name))
	}
	candidates = append(candidates, name)
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%s binary not found", name)
}

func probeBinary(envName, fallback string) (string, string, error) {
	binary := strings.TrimSpace(os.Getenv(envName))
	if binary == "" {
		binary = fallback
	}
	path, err := exec.LookPath(binary)
	if err != nil {
		return binary, "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, runErr := exec.CommandContext(ctx, path, "--version").CombinedOutput()
	if runErr != nil || len(strings.TrimSpace(string(out))) == 0 {
		out, runErr = exec.CommandContext(ctx, path, "version").CombinedOutput()
	}
	if ctx.Err() != nil {
		return path, strings.TrimSpace(string(out)), ctx.Err()
	}
	return path, strings.TrimSpace(string(out)), runErr
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

func waitHTTP(ctx context.Context, url, token string, timeout time.Duration) error {
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	client := &http.Client{Timeout: 1200 * time.Millisecond}
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
			lastErr = fmt.Errorf("HTTP %s", resp.Status)
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return lastErr
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
	return os.WriteFile(path, data, perm)
}

func readSmall(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if len(data) > 4096 {
		data = data[len(data)-4096:]
	}
	return Redact(string(data))
}

func liveManifest(profile string) *manifestForLive {
	return &manifestForLive{
		Version: "codencer.io/v1alpha1",
		Kind:    "RunManifest",
		Execution: map[string]string{
			"profile": profile,
			"timeout": "10m",
		},
		Tasks: []liveManifestTask{{
			ID:    "live-smoke",
			Title: "Live executor smoke",
			Goal:  "Create or update codencer-live-result.txt in the workspace with exactly the text CODENCER_LIVE_SMOKE_OK. Do not modify files outside the workspace.",
			Validations: []domain.ValidationCommand{{
				Name:           "live-result",
				Command:        "test -f codencer-live-result.txt && grep -q CODENCER_LIVE_SMOKE_OK codencer-live-result.txt",
				TimeoutSeconds: 30,
			}},
		}},
	}
}

type manifestForLive struct {
	Version   string             `json:"version" yaml:"version"`
	Kind      string             `json:"kind" yaml:"kind"`
	Execution map[string]string  `json:"execution" yaml:"execution"`
	Tasks     []liveManifestTask `json:"tasks" yaml:"tasks"`
}

type liveManifestTask struct {
	ID          string                     `json:"id" yaml:"id"`
	Title       string                     `json:"title" yaml:"title"`
	Goal        string                     `json:"goal" yaml:"goal"`
	Validations []domain.ValidationCommand `json:"validations" yaml:"validations"`
}
