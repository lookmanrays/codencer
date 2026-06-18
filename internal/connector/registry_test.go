package connector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/local"
	projectpkg "agent-bridge/internal/project"
)

func TestRegistry_SharedInstancesUsesAllowlist(t *testing.T) {
	root := t.TempDir()
	sharedManifest := filepath.Join(root, "repo-a", ".codencer", "instance.json")
	privateManifest := filepath.Join(root, "repo-b", ".codencer", "instance.json")
	if err := os.MkdirAll(filepath.Dir(sharedManifest), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(privateManifest), 0755); err != nil {
		t.Fatal(err)
	}

	write := func(path string, info domain.InstanceInfo) {
		t.Helper()
		data, _ := json.Marshal(info)
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(sharedManifest, domain.InstanceInfo{ID: "inst-shared", BaseURL: "http://127.0.0.1:8085", RepoRoot: filepath.Dir(filepath.Dir(sharedManifest))})
	write(privateManifest, domain.InstanceInfo{ID: "inst-private", BaseURL: "http://127.0.0.1:8086", RepoRoot: filepath.Dir(filepath.Dir(privateManifest))})

	cfg := &Config{
		DiscoveryRoots: []string{root},
		Instances: []SharedInstanceConfig{
			{InstanceID: "inst-shared", Share: true},
			{InstanceID: "inst-private", Share: false},
		},
	}
	registry := NewRegistry(cfg)
	registry.clientFactory = func(baseURL string) *CodencerClient { return NewCodencerClient(baseURL) }

	instances, err := registry.SharedInstances(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(instances) != 1 {
		t.Fatalf("expected one shared instance, got %d", len(instances))
	}
	if instances[0].Info.ID != "inst-shared" {
		t.Fatalf("expected shared instance inst-shared, got %s", instances[0].Info.ID)
	}
}

func TestRegistry_AdvertisementsIncludeSharedProjects(t *testing.T) {
	home := t.TempDir()
	repo := filepath.Join(home, "repo")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	paths, err := local.ResolvePathsForHome("", "", home)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := local.EnsureHome(paths, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	var daemon *httptest.Server
	daemon = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/instance" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(domain.InstanceInfo{ID: "inst-1", BaseURL: daemon.URL, RepoRoot: repo})
	}))
	defer daemon.Close()

	registry := projectpkg.EmptyRegistry()
	project, _, err := projectpkg.NewProject(projectpkg.ProjectOptions{
		ID:             "codencer",
		RepoRoot:       repo,
		DefaultAdapter: "fake",
		AdapterProfile: "fake-success",
		DaemonURL:      daemon.URL,
		SharedToRelay:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := projectpkg.UpsertProject(registry, project, false, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := projectpkg.SaveRegistry(paths.ProjectsFile, registry); err != nil {
		t.Fatal(err)
	}

	set, err := NewRegistry(&Config{CodencerHome: home}).Advertisements(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Projects) != 1 || set.ProjectIDs[0] != "codencer" || set.Projects[0].InstanceID != "inst-1" {
		t.Fatalf("expected shared project advertisement, got %+v", set)
	}
	if set.Projects[0].MachineID == "" || set.Projects[0].HostLabel == "" || set.Projects[0].Status != "available" {
		t.Fatalf("expected machine metadata in project advertisement, got %+v", set.Projects[0])
	}
	loaded, err := projectpkg.LoadRegistry(paths.ProjectsFile)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Projects[0].MachineID == "" || loaded.Projects[0].HostLabel == "" {
		t.Fatalf("expected local registry to be backfilled with machine metadata, got %+v", loaded.Projects[0])
	}
}

func TestRegistry_SkipsRelayInstanceMismatchWithWarning(t *testing.T) {
	home := t.TempDir()
	repo := filepath.Join(home, "repo")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	paths, err := local.ResolvePathsForHome("", "", home)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := local.EnsureHome(paths, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	daemon := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(domain.InstanceInfo{ID: "inst-live", BaseURL: "http://127.0.0.1:1", RepoRoot: repo})
	}))
	defer daemon.Close()

	registry := projectpkg.EmptyRegistry()
	project, _, err := projectpkg.NewProject(projectpkg.ProjectOptions{
		ID:              "codencer",
		RepoRoot:        repo,
		DefaultAdapter:  "fake",
		AdapterProfile:  "fake-success",
		DaemonURL:       daemon.URL,
		RelayInstanceID: "inst-expected",
		SharedToRelay:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := projectpkg.UpsertProject(registry, project, false, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := projectpkg.SaveRegistry(paths.ProjectsFile, registry); err != nil {
		t.Fatal(err)
	}

	set, err := NewRegistry(&Config{CodencerHome: home}).Advertisements(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Projects) != 0 || len(set.Warnings) == 0 || !strings.Contains(set.Warnings[0], "does not match") {
		t.Fatalf("expected mismatch warning without advertisement, got %+v", set)
	}
}
