package connector

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"agent-bridge/internal/domain"
)

func TestDiscoverInstancesMergesConfiguredAndDiscoveredViews(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()

	writeManifest := func(root, repo, id, daemonURL string) string {
		t.Helper()
		manifestPath := filepath.Join(root, repo, ".codencer", "instance.json")
		if err := os.MkdirAll(filepath.Dir(manifestPath), 0755); err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(domain.InstanceInfo{
			ID:           id,
			RepoRoot:     filepath.Join(root, repo),
			ManifestPath: manifestPath,
			BaseURL:      daemonURL,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(manifestPath, data, 0644); err != nil {
			t.Fatal(err)
		}
		return manifestPath
	}

	sharedManifest := writeManifest(rootA, "repo-shared", "inst-shared", "http://127.0.0.1:8085")
	discoveredOnlyManifest := writeManifest(rootB, "repo-only", "inst-only", "http://127.0.0.1:8087")

	cfg := &Config{
		DiscoveryRoots: []string{rootA},
		Instances: []SharedInstanceConfig{
			{InstanceID: "inst-shared", Share: true},
			{InstanceID: "inst-hidden", ManifestPath: filepath.Join(rootA, "repo-hidden", ".codencer", "instance.json"), Share: false},
		},
	}

	entries, err := DiscoverInstances(context.Background(), cfg, []string{rootB}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected three discover entries, got %+v", entries)
	}

	assertEntry := func(state, instanceID, repoRoot, manifestPath, daemonURL string) {
		t.Helper()
		for _, entry := range entries {
			if entry.State == state && entry.InstanceID == instanceID {
				if repoRoot != "" && entry.RepoRoot != repoRoot {
					t.Fatalf("expected repo_root %s for %s, got %s", repoRoot, instanceID, entry.RepoRoot)
				}
				if manifestPath != "" && entry.ManifestPath != manifestPath {
					t.Fatalf("expected manifest_path %s for %s, got %s", manifestPath, instanceID, entry.ManifestPath)
				}
				if daemonURL != "" && entry.DaemonURL != daemonURL {
					t.Fatalf("expected daemon_url %s for %s, got %s", daemonURL, instanceID, entry.DaemonURL)
				}
				return
			}
		}
		t.Fatalf("missing discover entry state=%s instance=%s in %+v", state, instanceID, entries)
	}

	assertEntry(DiscoverStateShared, "inst-shared", filepath.Join(rootA, "repo-shared"), sharedManifest, "http://127.0.0.1:8085")
	assertEntry(DiscoverStateKnownUnshared, "inst-hidden", "", filepath.Join(rootA, "repo-hidden", ".codencer", "instance.json"), "")
	assertEntry(DiscoverStateDiscoveredOnly, "inst-only", filepath.Join(rootB, "repo-only"), discoveredOnlyManifest, "http://127.0.0.1:8087")
}
