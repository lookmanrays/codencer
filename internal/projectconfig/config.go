package projectconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	projectpkg "agent-bridge/internal/project"
)

const (
	APIVersion = "codencer.io/v1alpha1"
	Kind       = "ProjectConfig"
	DirName    = ".codencer"
	FileName   = "project.json"
)

var windowsAbsPathPattern = regexp.MustCompile(`^[A-Za-z]:[\\/].*`)

type Project struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Execution struct {
	DefaultAdapter string `json:"default_adapter"`
	DefaultProfile string `json:"default_profile"`
}

type Workspace struct {
	Root           string   `json:"root"`
	ForbiddenPaths []string `json:"forbidden_paths"`
}

type ValidationCommand struct {
	Name    string `json:"name"`
	Command string `json:"command"`
}

type Config struct {
	Version     string              `json:"version"`
	Kind        string              `json:"kind"`
	Project     Project             `json:"project"`
	Execution   Execution           `json:"execution"`
	Workspace   Workspace           `json:"workspace"`
	Validations []ValidationCommand `json:"validations"`
}

type DefaultOptions struct {
	ProjectID      string
	ProjectName    string
	Description    string
	DefaultAdapter string
	DefaultProfile string
	WorkspaceRoot  string
	ForbiddenPaths []string
	Validations    []ValidationCommand
}

func Path(repoRoot string) string {
	return filepath.Join(repoRoot, DirName, FileName)
}

func Exists(repoRoot string) bool {
	_, err := os.Stat(Path(repoRoot))
	return err == nil
}

func Load(repoRoot string) (Config, []string, error) {
	return LoadFile(Path(repoRoot))
}

func LoadFile(path string) (Config, []string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, nil, fmt.Errorf("read project config: %w", err)
	}
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return Config{}, nil, fmt.Errorf("decode project config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, nil, fmt.Errorf("decode project config: %w", err)
	}
	warnings, err := ValidateWithRaw(cfg, raw)
	if err != nil {
		return Config{}, warnings, err
	}
	return cfg, warnings, nil
}

func Save(repoRoot string, cfg Config) error {
	return SaveFile(Path(repoRoot), cfg)
}

func SaveFile(path string, cfg Config) error {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	if _, err := ValidateWithRaw(cfg, value); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create project config dir: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode project config: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}

func Default(opts DefaultOptions) Config {
	id := strings.TrimSpace(opts.ProjectID)
	name := strings.TrimSpace(opts.ProjectName)
	if name == "" {
		name = TitleFromID(id)
	}
	defaultAdapter := strings.TrimSpace(opts.DefaultAdapter)
	if defaultAdapter == "" {
		defaultAdapter = "codex"
	}
	defaultProfile := strings.TrimSpace(opts.DefaultProfile)
	if defaultProfile == "" {
		defaultProfile = "codex-workspace"
	}
	root := strings.TrimSpace(opts.WorkspaceRoot)
	if root == "" {
		root = "."
	}
	forbidden := cleanList(opts.ForbiddenPaths)
	if len(forbidden) == 0 {
		forbidden = DefaultForbiddenPaths()
	}
	return Config{
		Version: APIVersion,
		Kind:    Kind,
		Project: Project{
			ID:          id,
			Name:        name,
			Description: strings.TrimSpace(opts.Description),
		},
		Execution: Execution{
			DefaultAdapter: defaultAdapter,
			DefaultProfile: defaultProfile,
		},
		Workspace: Workspace{
			Root:           root,
			ForbiddenPaths: forbidden,
		},
		Validations: append([]ValidationCommand(nil), opts.Validations...),
	}
}

func DefaultForbiddenPaths() []string {
	return []string{
		".env",
		".env.*",
		".git",
		"node_modules",
		"dist",
		"build",
	}
}

func Validate(cfg Config) ([]string, error) {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return ValidateWithRaw(cfg, value)
}

func ValidateWithRaw(cfg Config, raw any) ([]string, error) {
	warnings := []string{}
	if strings.TrimSpace(cfg.Version) != APIVersion {
		return warnings, fmt.Errorf("project config version must be %q", APIVersion)
	}
	if strings.TrimSpace(cfg.Kind) != Kind {
		return warnings, fmt.Errorf("project config kind must be %q", Kind)
	}
	if err := projectpkg.ValidateID(cfg.Project.ID); err != nil {
		return warnings, err
	}
	if strings.TrimSpace(cfg.Project.Name) == "" {
		return warnings, fmt.Errorf("project name is required")
	}
	if strings.TrimSpace(cfg.Execution.DefaultAdapter) == "" {
		return warnings, fmt.Errorf("execution.default_adapter is required")
	}
	if strings.TrimSpace(cfg.Execution.DefaultProfile) == "" {
		return warnings, fmt.Errorf("execution.default_profile is required")
	}
	if err := validateRelativePath("workspace.root", cfg.Workspace.Root); err != nil {
		return warnings, err
	}
	for i, path := range cfg.Workspace.ForbiddenPaths {
		if err := validateRelativePath(fmt.Sprintf("workspace.forbidden_paths[%d]", i), path); err != nil {
			return warnings, err
		}
	}
	if err := rejectUnsafeRawValue("", raw); err != nil {
		return warnings, err
	}
	return warnings, nil
}

func InferID(repoRoot string) string {
	name := filepath.Base(filepath.Clean(repoRoot))
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	lastDash := false
	for _, r := range name {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-'
		if !ok {
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
			continue
		}
		b.WriteRune(r)
		lastDash = r == '-'
	}
	id := strings.Trim(b.String(), ".-_")
	if len(id) > 63 {
		id = strings.Trim(id[:63], ".-_")
	}
	if err := projectpkg.ValidateID(id); err != nil {
		return ""
	}
	return id
}

func TitleFromID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	parts := strings.FieldsFunc(id, func(r rune) bool {
		return r == '-' || r == '_' || r == '.'
	})
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func validateRelativePath(field, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s is required", field)
	}
	if isDangerousPath(value) {
		return fmt.Errorf("%s must be a relative repository path, got %q", field, value)
	}
	return nil
}

func rejectUnsafeRawValue(path string, value any) error {
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			nextPath := key
			if path != "" {
				nextPath = path + "." + key
			}
			if isTokenLikeField(key) {
				return fmt.Errorf("project config field %q is not commit-safe", nextPath)
			}
			if err := rejectUnsafeRawValue(nextPath, item); err != nil {
				return err
			}
		}
	case []any:
		for i, item := range typed {
			if err := rejectUnsafeRawValue(fmt.Sprintf("%s[%d]", path, i), item); err != nil {
				return err
			}
		}
	case string:
		if isDangerousPath(typed) {
			return fmt.Errorf("project config value %q must not be an absolute or host-specific path", path)
		}
		if strings.Contains(strings.ToLower(typed), "://") {
			return fmt.Errorf("project config value %q must not contain URLs", path)
		}
	}
	return nil
}

func isTokenLikeField(key string) bool {
	lower := strings.ToLower(strings.TrimSpace(key))
	if lower == "" {
		return false
	}
	for _, marker := range []string{
		"token",
		"secret",
		"password",
		"private_key",
		"apikey",
		"api_key",
		"authorization",
		"daemon_url",
		"relay_url",
		"base_url",
		"connector_id",
		"machine_id",
		"privatekey",
		"enrollment",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func isDangerousPath(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	lower := strings.ToLower(value)
	if value == "/" || value == "~" || strings.HasPrefix(value, "~/") || strings.HasPrefix(value, "~\\") {
		return true
	}
	if lower == "$home" || strings.HasPrefix(lower, "$home/") || strings.HasPrefix(lower, "$home\\") {
		return true
	}
	if strings.HasPrefix(lower, "${home}") || strings.HasPrefix(lower, "%userprofile%") {
		return true
	}
	if filepath.IsAbs(value) || windowsAbsPathPattern.MatchString(value) || strings.HasPrefix(value, `\\`) {
		return true
	}
	return false
}

func cleanList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
