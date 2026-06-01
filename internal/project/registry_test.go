package project

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRegistryLifecyclePersistsAndSortsProjects(t *testing.T) {
	tempDir := t.TempDir()
	registryPath := filepath.Join(tempDir, "projects.json")
	repoA := makeRepo(t, tempDir, "repo-a", true)
	repoB := makeRepo(t, tempDir, "repo-b", true)

	registry, err := LoadRegistry(registryPath)
	if err != nil {
		t.Fatalf("load empty registry: %v", err)
	}

	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	projectB, warnings, err := NewProject(ProjectOptions{ID: "beta", RepoRoot: repoB, DefaultAdapter: "claude"})
	if err != nil {
		t.Fatalf("new beta project: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if _, err := UpsertProject(registry, projectB, false, now); err != nil {
		t.Fatalf("upsert beta: %v", err)
	}

	projectA, _, err := NewProject(ProjectOptions{ID: "alpha", RepoRoot: repoA, DefaultAdapter: "codex", Name: "Alpha"})
	if err != nil {
		t.Fatalf("new alpha project: %v", err)
	}
	if _, err := UpsertProject(registry, projectA, false, now.Add(time.Minute)); err != nil {
		t.Fatalf("upsert alpha: %v", err)
	}

	projects := ListProjects(registry)
	if got, want := []string{projects[0].ID, projects[1].ID}, []string{"alpha", "beta"}; got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("projects not sorted: got %v want %v", got, want)
	}
	if registry.CurrentProjectID != "beta" {
		t.Fatalf("first registered project should become current, got %q", registry.CurrentProjectID)
	}

	if _, err := UseProject(registry, "alpha"); err != nil {
		t.Fatalf("use alpha: %v", err)
	}
	if registry.CurrentProjectID != "alpha" {
		t.Fatalf("current project = %q", registry.CurrentProjectID)
	}

	if err := SaveRegistry(registryPath, registry); err != nil {
		t.Fatalf("save registry: %v", err)
	}
	loaded, err := LoadRegistry(registryPath)
	if err != nil {
		t.Fatalf("reload registry: %v", err)
	}
	gotAlpha, err := GetProject(loaded, "alpha")
	if err != nil {
		t.Fatalf("get alpha: %v", err)
	}
	if gotAlpha.Name != "Alpha" || !filepath.IsAbs(gotAlpha.RepoRoot) {
		t.Fatalf("unexpected alpha project: %+v", gotAlpha)
	}

	removed, err := RemoveProject(loaded, "alpha")
	if err != nil {
		t.Fatalf("remove alpha: %v", err)
	}
	if removed.ID != "alpha" || loaded.CurrentProjectID != "" {
		t.Fatalf("remove current should clear current id, removed=%+v current=%q", removed, loaded.CurrentProjectID)
	}
}

func TestDuplicateProjectRequiresForceAndPreservesCreatedAt(t *testing.T) {
	tempDir := t.TempDir()
	repo := makeRepo(t, tempDir, "repo", true)
	registry := EmptyRegistry()
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	p, _, err := NewProject(ProjectOptions{ID: "codencer", RepoRoot: repo, DefaultAdapter: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	created, err := UpsertProject(registry, p, false, now)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	p.Name = "Codencer Updated"
	if _, err := UpsertProject(registry, p, false, now.Add(time.Hour)); !errors.Is(err, ErrDuplicateID) {
		t.Fatalf("expected duplicate error, got %v", err)
	}
	updated, err := UpsertProject(registry, p, true, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("force update: %v", err)
	}
	if !updated.CreatedAt.Equal(created.CreatedAt) {
		t.Fatalf("force update should preserve created_at: created=%s updated=%s", created.CreatedAt, updated.CreatedAt)
	}
	if !updated.UpdatedAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("updated_at not refreshed: %s", updated.UpdatedAt)
	}
}

func TestProjectRelaySharingTogglesPersistedFields(t *testing.T) {
	tempDir := t.TempDir()
	repo := makeRepo(t, tempDir, "repo", true)
	registry := EmptyRegistry()
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	p, _, err := NewProject(ProjectOptions{ID: "codencer", RepoRoot: repo, DefaultAdapter: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := UpsertProject(registry, p, false, now); err != nil {
		t.Fatal(err)
	}

	shared, err := ShareProject(registry, "codencer", "inst-1", "http://127.0.0.1:18085", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("share project: %v", err)
	}
	if !shared.SharedToRelay || shared.RelayInstanceID != "inst-1" || shared.DaemonURL != "http://127.0.0.1:18085" {
		t.Fatalf("share did not persist relay fields: %+v", shared)
	}

	unshared, err := UnshareProject(registry, "codencer", now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("unshare project: %v", err)
	}
	if unshared.SharedToRelay || unshared.RelayInstanceID != "inst-1" || unshared.DaemonURL == "" {
		t.Fatalf("unshare should only clear sharing flag: %+v", unshared)
	}
}

func TestProjectIDValidation(t *testing.T) {
	valid := []string{"a", "codencer", "repo-1", "repo_1", "repo.1"}
	for _, id := range valid {
		if err := ValidateID(id); err != nil {
			t.Fatalf("expected %q to be valid: %v", id, err)
		}
	}

	invalid := []string{"", "Codencer", "-repo", ".repo", "repo/one", `repo\one`, "repo..one", "repo one"}
	for _, id := range invalid {
		if err := ValidateID(id); err == nil {
			t.Fatalf("expected %q to be invalid", id)
		}
	}
}

func TestRepoRootNormalizationWarnsForNonGitRepo(t *testing.T) {
	tempDir := t.TempDir()
	repo := makeRepo(t, tempDir, "plain", false)
	project, warnings, err := NewProject(ProjectOptions{ID: "plain", RepoRoot: repo, DefaultAdapter: "codex"})
	if err != nil {
		t.Fatalf("new project should accept non-git repo with warning: %v", err)
	}
	if !filepath.IsAbs(project.RepoRoot) {
		t.Fatalf("repo root should be absolute: %q", project.RepoRoot)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected non-git warning, got %v", warnings)
	}
}

func TestResolveProjectOrder(t *testing.T) {
	tempDir := t.TempDir()
	repoA := makeRepo(t, tempDir, "repo-a", true)
	repoB := makeRepo(t, tempDir, "repo-b", true)
	nested := filepath.Join(repoB, "nested")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}

	registry := EmptyRegistry()
	for _, opts := range []ProjectOptions{
		{ID: "alpha", RepoRoot: repoA, DefaultAdapter: "codex"},
		{ID: "beta", RepoRoot: repoB, DefaultAdapter: "claude"},
	} {
		p, _, err := NewProject(opts)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := UpsertProject(registry, p, false, time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
	}

	result, err := ResolveProject(registry, ResolveOptions{ExplicitID: "beta", CWD: repoA})
	if err != nil {
		t.Fatalf("resolve explicit: %v", err)
	}
	if result.Project.ID != "beta" || result.Source != "explicit" {
		t.Fatalf("unexpected explicit resolution: %+v", result)
	}

	registry.CurrentProjectID = "alpha"
	result, err = ResolveProject(registry, ResolveOptions{CWD: nested})
	if err != nil {
		t.Fatalf("resolve current: %v", err)
	}
	if result.Project.ID != "alpha" || result.Source != "current" {
		t.Fatalf("unexpected current resolution: %+v", result)
	}

	registry.CurrentProjectID = ""
	result, err = ResolveProject(registry, ResolveOptions{CWD: nested})
	if err != nil {
		t.Fatalf("resolve cwd: %v", err)
	}
	if result.Project.ID != "beta" || result.Source != "cwd" {
		t.Fatalf("unexpected cwd resolution: %+v", result)
	}
}

func makeRepo(t *testing.T, parent, name string, withGit bool) string {
	t.Helper()
	repo := filepath.Join(parent, name)
	if err := os.MkdirAll(repo, 0755); err != nil {
		t.Fatal(err)
	}
	if withGit {
		if err := os.Mkdir(filepath.Join(repo, ".git"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	return repo
}
