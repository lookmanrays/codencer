package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const RegistryVersion = 1

var projectIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)

var (
	ErrProjectNotFound = errors.New("project not found")
	ErrDuplicateID     = errors.New("project id already exists")
)

type ValidationCommand struct {
	Name    string `json:"name"`
	Command string `json:"command"`
}

type Project struct {
	ID                 string              `json:"id"`
	Name               string              `json:"name"`
	RepoRoot           string              `json:"repo_root"`
	DefaultAdapter     string              `json:"default_adapter"`
	AdapterProfile     string              `json:"adapter_profile"`
	DaemonURL          string              `json:"daemon_url"`
	RelayInstanceID    string              `json:"relay_instance_id"`
	SharedToRelay      bool                `json:"shared_to_relay"`
	AllowedPaths       []string            `json:"allowed_paths"`
	ForbiddenPaths     []string            `json:"forbidden_paths"`
	DefaultValidations []ValidationCommand `json:"default_validations"`
	CreatedAt          time.Time           `json:"created_at"`
	UpdatedAt          time.Time           `json:"updated_at"`
}

type Registry struct {
	Version          int       `json:"version"`
	CurrentProjectID string    `json:"current_project_id,omitempty"`
	Projects         []Project `json:"projects"`
}

type ProjectOptions struct {
	ID                 string
	Name               string
	RepoRoot           string
	DefaultAdapter     string
	AdapterProfile     string
	DaemonURL          string
	RelayInstanceID    string
	SharedToRelay      bool
	AllowedPaths       []string
	ForbiddenPaths     []string
	DefaultValidations []ValidationCommand
}

type ResolveOptions struct {
	ExplicitID string
	CWD        string
}

type ResolveResult struct {
	Project Project
	Source  string
}

func EmptyRegistry() *Registry {
	return &Registry{
		Version:  RegistryVersion,
		Projects: []Project{},
	}
}

func LoadRegistry(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return EmptyRegistry(), nil
		}
		return nil, fmt.Errorf("read project registry: %w", err)
	}

	var registry Registry
	if err := json.Unmarshal(data, &registry); err != nil {
		return nil, fmt.Errorf("decode project registry: %w", err)
	}
	normalizeRegistry(&registry)
	return &registry, nil
}

func SaveRegistry(path string, registry *Registry) error {
	if registry == nil {
		registry = EmptyRegistry()
	}
	normalizeRegistry(registry)
	if err := ValidateRegistry(registry); err != nil {
		return err
	}
	return writeJSONAtomic(path, registry, 0600)
}

func ValidateID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("project id is required")
	}
	if strings.Contains(id, "..") {
		return fmt.Errorf("project id %q must not contain path traversal", id)
	}
	if strings.ContainsAny(id, `/\`) {
		return fmt.Errorf("project id %q must not contain path separators", id)
	}
	if !projectIDPattern.MatchString(id) {
		return fmt.Errorf("project id %q must match [a-z0-9][a-z0-9._-]{0,62}", id)
	}
	return nil
}

func NewProject(opts ProjectOptions) (Project, []string, error) {
	id := strings.TrimSpace(opts.ID)
	if err := ValidateID(id); err != nil {
		return Project{}, nil, err
	}

	repoRoot, warnings, err := NormalizeRepoRoot(opts.RepoRoot)
	if err != nil {
		return Project{}, warnings, err
	}

	defaultAdapter := strings.TrimSpace(opts.DefaultAdapter)
	if defaultAdapter == "" {
		return Project{}, warnings, fmt.Errorf("default adapter is required")
	}
	adapterProfile := strings.TrimSpace(opts.AdapterProfile)
	if adapterProfile == "" {
		adapterProfile = defaultAdapter
	}
	name := strings.TrimSpace(opts.Name)
	if name == "" {
		name = id
	}

	allowedPaths := cleanList(opts.AllowedPaths)
	if len(allowedPaths) == 0 {
		allowedPaths = []string{"."}
	}
	forbiddenPaths := cleanList(opts.ForbiddenPaths)
	if len(forbiddenPaths) == 0 {
		forbiddenPaths = []string{".env", ".codencer/secrets"}
	}

	defaultValidations := append([]ValidationCommand{}, opts.DefaultValidations...)

	return Project{
		ID:                 id,
		Name:               name,
		RepoRoot:           repoRoot,
		DefaultAdapter:     defaultAdapter,
		AdapterProfile:     adapterProfile,
		DaemonURL:          strings.TrimSpace(opts.DaemonURL),
		RelayInstanceID:    strings.TrimSpace(opts.RelayInstanceID),
		SharedToRelay:      opts.SharedToRelay,
		AllowedPaths:       allowedPaths,
		ForbiddenPaths:     forbiddenPaths,
		DefaultValidations: defaultValidations,
	}, warnings, nil
}

func NormalizeRepoRoot(repoRoot string) (string, []string, error) {
	repoRoot = strings.TrimSpace(repoRoot)
	if repoRoot == "" {
		return "", nil, fmt.Errorf("repo root is required")
	}
	abs, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", nil, fmt.Errorf("resolve repo root: %w", err)
	}
	abs = filepath.Clean(abs)

	info, err := os.Stat(abs)
	if err != nil {
		return "", nil, fmt.Errorf("repo root %q is not accessible: %w", abs, err)
	}
	if !info.IsDir() {
		return "", nil, fmt.Errorf("repo root %q is not a directory", abs)
	}

	warnings := []string{}
	if _, err := os.Stat(filepath.Join(abs, ".git")); err != nil {
		if os.IsNotExist(err) {
			warnings = append(warnings, fmt.Sprintf("repo root %q is not a git repository", abs))
		} else {
			warnings = append(warnings, fmt.Sprintf("could not inspect git metadata for %q: %v", abs, err))
		}
	}
	return abs, warnings, nil
}

func UpsertProject(registry *Registry, next Project, force bool, now time.Time) (Project, error) {
	if registry == nil {
		return Project{}, fmt.Errorf("project registry is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	normalizeRegistry(registry)
	if err := ValidateProject(next); err != nil {
		return Project{}, err
	}

	for i := range registry.Projects {
		if registry.Projects[i].ID != next.ID {
			continue
		}
		if !force {
			return Project{}, ErrDuplicateID
		}
		if registry.Projects[i].CreatedAt.IsZero() {
			next.CreatedAt = now
		} else {
			next.CreatedAt = registry.Projects[i].CreatedAt
		}
		next.UpdatedAt = now
		registry.Projects[i] = next
		sortProjects(registry.Projects)
		if registry.CurrentProjectID == "" {
			registry.CurrentProjectID = next.ID
		}
		return next, nil
	}

	next.CreatedAt = now
	next.UpdatedAt = now
	registry.Projects = append(registry.Projects, next)
	sortProjects(registry.Projects)
	if registry.CurrentProjectID == "" {
		registry.CurrentProjectID = next.ID
	}
	return next, nil
}

func ListProjects(registry *Registry) []Project {
	if registry == nil {
		return nil
	}
	projects := append([]Project(nil), registry.Projects...)
	sortProjects(projects)
	return projects
}

func GetProject(registry *Registry, id string) (Project, error) {
	if registry == nil {
		return Project{}, ErrProjectNotFound
	}
	for _, p := range registry.Projects {
		if p.ID == id {
			return p, nil
		}
	}
	return Project{}, fmt.Errorf("%w: %s", ErrProjectNotFound, id)
}

func UseProject(registry *Registry, id string) (Project, error) {
	if registry == nil {
		return Project{}, fmt.Errorf("project registry is required")
	}
	project, err := GetProject(registry, id)
	if err != nil {
		return Project{}, err
	}
	registry.CurrentProjectID = project.ID
	return project, nil
}

func ShareProject(registry *Registry, id, relayInstanceID, daemonURL string, now time.Time) (Project, error) {
	if registry == nil {
		return Project{}, fmt.Errorf("project registry is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	for i := range registry.Projects {
		if registry.Projects[i].ID != id {
			continue
		}
		registry.Projects[i].SharedToRelay = true
		if strings.TrimSpace(relayInstanceID) != "" {
			registry.Projects[i].RelayInstanceID = strings.TrimSpace(relayInstanceID)
		}
		if strings.TrimSpace(daemonURL) != "" {
			registry.Projects[i].DaemonURL = strings.TrimSpace(daemonURL)
		}
		registry.Projects[i].UpdatedAt = now
		return registry.Projects[i], nil
	}
	return Project{}, fmt.Errorf("%w: %s", ErrProjectNotFound, id)
}

func UnshareProject(registry *Registry, id string, now time.Time) (Project, error) {
	if registry == nil {
		return Project{}, fmt.Errorf("project registry is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	for i := range registry.Projects {
		if registry.Projects[i].ID != id {
			continue
		}
		registry.Projects[i].SharedToRelay = false
		registry.Projects[i].UpdatedAt = now
		return registry.Projects[i], nil
	}
	return Project{}, fmt.Errorf("%w: %s", ErrProjectNotFound, id)
}

func RemoveProject(registry *Registry, id string) (Project, error) {
	if registry == nil {
		return Project{}, fmt.Errorf("project registry is required")
	}
	for i, p := range registry.Projects {
		if p.ID != id {
			continue
		}
		registry.Projects = append(registry.Projects[:i], registry.Projects[i+1:]...)
		if registry.CurrentProjectID == id {
			registry.CurrentProjectID = ""
		}
		return p, nil
	}
	return Project{}, fmt.Errorf("%w: %s", ErrProjectNotFound, id)
}

func ResolveProject(registry *Registry, opts ResolveOptions) (ResolveResult, error) {
	if registry == nil {
		return ResolveResult{}, ErrProjectNotFound
	}
	if explicitID := strings.TrimSpace(opts.ExplicitID); explicitID != "" {
		project, err := GetProject(registry, explicitID)
		if err != nil {
			return ResolveResult{}, err
		}
		return ResolveResult{Project: project, Source: "explicit"}, nil
	}
	if registry.CurrentProjectID != "" {
		project, err := GetProject(registry, registry.CurrentProjectID)
		if err != nil {
			return ResolveResult{}, fmt.Errorf("current project %q is not registered", registry.CurrentProjectID)
		}
		return ResolveResult{Project: project, Source: "current"}, nil
	}

	cwd := strings.TrimSpace(opts.CWD)
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return ResolveResult{}, fmt.Errorf("resolve current directory: %w", err)
		}
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return ResolveResult{}, fmt.Errorf("resolve current directory: %w", err)
	}
	abs = filepath.Clean(abs)

	var match *Project
	for _, candidate := range registry.Projects {
		if !pathWithin(abs, candidate.RepoRoot) {
			continue
		}
		candidateCopy := candidate
		if match == nil || len(candidate.RepoRoot) > len(match.RepoRoot) {
			match = &candidateCopy
		}
	}
	if match != nil {
		return ResolveResult{Project: *match, Source: "cwd"}, nil
	}
	return ResolveResult{}, ErrProjectNotFound
}

func ValidateRegistry(registry *Registry) error {
	if registry == nil {
		return fmt.Errorf("project registry is required")
	}
	seen := map[string]struct{}{}
	for _, project := range registry.Projects {
		if err := ValidateProject(project); err != nil {
			return err
		}
		if _, ok := seen[project.ID]; ok {
			return fmt.Errorf("duplicate project id %q", project.ID)
		}
		seen[project.ID] = struct{}{}
	}
	if registry.CurrentProjectID != "" {
		if _, err := GetProject(registry, registry.CurrentProjectID); err != nil {
			return fmt.Errorf("current project %q is not registered", registry.CurrentProjectID)
		}
	}
	return nil
}

func ValidateProject(project Project) error {
	if err := ValidateID(project.ID); err != nil {
		return err
	}
	if strings.TrimSpace(project.DefaultAdapter) == "" {
		return fmt.Errorf("project %q default adapter is required", project.ID)
	}
	if strings.TrimSpace(project.AdapterProfile) == "" {
		return fmt.Errorf("project %q adapter profile is required", project.ID)
	}
	if !filepath.IsAbs(project.RepoRoot) {
		return fmt.Errorf("project %q repo root must be absolute", project.ID)
	}
	info, err := os.Stat(project.RepoRoot)
	if err != nil {
		return fmt.Errorf("project %q repo root is not accessible: %w", project.ID, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("project %q repo root is not a directory", project.ID)
	}
	return nil
}

func normalizeRegistry(registry *Registry) {
	if registry.Version == 0 {
		registry.Version = RegistryVersion
	}
	if registry.Projects == nil {
		registry.Projects = []Project{}
	}
	sortProjects(registry.Projects)
}

func sortProjects(projects []Project) {
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].ID < projects[j].ID
	})
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

func pathWithin(path, root string) bool {
	if path == "" || root == "" {
		return false
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
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
