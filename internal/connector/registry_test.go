package connector

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"agent-bridge/internal/domain"
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
