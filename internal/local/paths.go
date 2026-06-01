package local

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	HomeEnvName      = "CODENCER_HOME"
	defaultDaemonURL = "http://127.0.0.1:8085"
	configFileName   = "config.json"
	projectsFileName = "projects.json"
	repoStateDirName = ".codencer"
)

type Paths struct {
	Home           string `json:"home"`
	ProjectsFile   string `json:"projects_file"`
	ConfigFile     string `json:"config_file"`
	LogsDir        string `json:"logs_dir"`
	RuntimeDir     string `json:"runtime_dir"`
	TokensDir      string `json:"tokens_dir"`
	ArtifactsDir   string `json:"artifacts_dir"`
	RepoRuntimeDir string `json:"repo_runtime_dir,omitempty"`
}

type Environment struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
	WSL  bool   `json:"wsl"`
}

func ResolvePaths(repoOverride, configOverride string) (Paths, error) {
	return ResolvePathsForHome(repoOverride, configOverride, "")
}

func ResolvePathsForHome(repoOverride, configOverride, homeOverride string) (Paths, error) {
	home, err := resolveHome(homeOverride)
	if err != nil {
		return Paths{}, err
	}

	configFile := filepath.Join(home, configFileName)
	if strings.TrimSpace(configOverride) != "" {
		configFile, err = filepath.Abs(configOverride)
		if err != nil {
			return Paths{}, fmt.Errorf("resolve config path: %w", err)
		}
		configFile = filepath.Clean(configFile)
	}

	paths := Paths{
		Home:         home,
		ProjectsFile: filepath.Join(home, projectsFileName),
		ConfigFile:   configFile,
		LogsDir:      filepath.Join(home, "logs"),
		RuntimeDir:   filepath.Join(home, "runtime"),
		TokensDir:    filepath.Join(home, "tokens"),
		ArtifactsDir: filepath.Join(home, "artifacts"),
	}

	if strings.TrimSpace(repoOverride) != "" {
		repoRoot, err := filepath.Abs(repoOverride)
		if err != nil {
			return Paths{}, fmt.Errorf("resolve repo path: %w", err)
		}
		paths.RepoRuntimeDir = filepath.Join(filepath.Clean(repoRoot), repoStateDirName)
	}

	return paths, nil
}

func DetectEnvironment() Environment {
	return Environment{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
		WSL:  detectWSL(),
	}
}

func resolveHome(override string) (string, error) {
	home := strings.TrimSpace(override)
	if home == "" {
		home = strings.TrimSpace(os.Getenv(HomeEnvName))
	}
	if home == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home: %w", err)
		}
		home = filepath.Join(userHome, ".codencer")
	}
	abs, err := filepath.Abs(home)
	if err != nil {
		return "", fmt.Errorf("resolve codencer home: %w", err)
	}
	return filepath.Clean(abs), nil
}

func detectWSL() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	if os.Getenv("WSL_DISTRO_NAME") != "" || os.Getenv("WSL_INTEROP") != "" {
		return true
	}
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), "microsoft")
}
