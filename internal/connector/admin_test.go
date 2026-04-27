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

	"agent-bridge/internal/domain"
)

func TestShareInstanceEnrichesFromDaemonAndUnshareKeepsEntry(t *testing.T) {
	cfg := &Config{}

	var daemon *httptest.Server
	daemon = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/instance" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(domain.InstanceInfo{
			ID:           "inst-1",
			BaseURL:      daemon.URL,
			ManifestPath: "/repo/.codencer/instance.json",
		})
	}))
	defer daemon.Close()

	shared, err := ShareInstance(context.Background(), cfg, InstanceSelector{DaemonURL: daemon.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !shared.Share || shared.InstanceID != "inst-1" || shared.ManifestPath != "/repo/.codencer/instance.json" {
		t.Fatalf("expected enriched shared entry, got %+v", shared)
	}
	if len(cfg.Instances) != 1 {
		t.Fatalf("expected one persisted instance, got %+v", cfg.Instances)
	}

	unshared, err := UnshareInstance(cfg, InstanceSelector{InstanceID: "inst-1"})
	if err != nil {
		t.Fatal(err)
	}
	if unshared.Share {
		t.Fatalf("expected unshared entry to remain persisted, got %+v", unshared)
	}
	if len(cfg.Instances) != 1 || cfg.Instances[0].Share {
		t.Fatalf("expected config entry to remain with share=false, got %+v", cfg.Instances)
	}
}

func TestEffectiveSharedInstancesIncludesLegacyDaemonSeed(t *testing.T) {
	cfg := &Config{DaemonURL: "http://127.0.0.1:8085/"}

	effective := EffectiveSharedInstances(cfg)
	if len(effective) != 1 {
		t.Fatalf("expected one effective legacy instance, got %+v", effective)
	}
	if effective[0].DaemonURL != "http://127.0.0.1:8085" || !effective[0].Share {
		t.Fatalf("unexpected effective legacy instance: %+v", effective[0])
	}

	EnsureLegacySharedInstance(cfg)
	if len(cfg.Instances) != 1 || cfg.Instances[0].DaemonURL != "http://127.0.0.1:8085" || !cfg.Instances[0].Share {
		t.Fatalf("expected legacy seed to be normalized into persisted instances, got %+v", cfg.Instances)
	}
}

func TestRedactedConfigHidesPrivateKeyByDefault(t *testing.T) {
	cfg := &Config{
		RelayURL:   "http://relay.invalid",
		PrivateKey: "secret-key",
		PublicKey:  "public-key",
	}

	redacted := RedactedConfig(cfg, false)
	if redacted.PrivateKey != redactedSecret {
		t.Fatalf("expected private key to be redacted, got %q", redacted.PrivateKey)
	}
	if redacted.PublicKey != "public-key" {
		t.Fatalf("expected public key to remain visible, got %q", redacted.PublicKey)
	}

	visible := RedactedConfig(cfg, true)
	if visible.PrivateKey != "secret-key" {
		t.Fatalf("expected show-secrets view to preserve private key, got %q", visible.PrivateKey)
	}
}

func TestShareInstanceByInstanceIDRequiresResolvableLocalInstance(t *testing.T) {
	cfg := &Config{}

	_, err := ShareInstance(context.Background(), cfg, InstanceSelector{InstanceID: "inst-missing"}, nil)
	if err == nil {
		t.Fatal("expected share by unresolved instance id to fail")
	}
	if !strings.Contains(err.Error(), "did not resolve to a local daemon url") {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Instances) != 0 {
		t.Fatalf("expected unresolved share to avoid mutating config, got %+v", cfg.Instances)
	}
}

func TestShareInstanceByInstanceIDResolvesDiscoveryEntryAndPersistsRoutableMetadata(t *testing.T) {
	root := t.TempDir()
	repoRoot := filepath.Join(root, "repo-a")
	manifestPath := filepath.Join(repoRoot, ".codencer", "instance.json")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0755); err != nil {
		t.Fatal(err)
	}

	var daemon *httptest.Server
	daemon = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/instance" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(domain.InstanceInfo{
			ID:           "inst-discovered",
			RepoRoot:     repoRoot,
			BaseURL:      daemon.URL,
			ManifestPath: manifestPath,
		})
	}))
	defer daemon.Close()

	data, err := json.Marshal(domain.InstanceInfo{
		ID:           "inst-discovered",
		RepoRoot:     repoRoot,
		BaseURL:      daemon.URL,
		ManifestPath: manifestPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{DiscoveryRoots: []string{root}}
	shared, err := ShareInstance(context.Background(), cfg, InstanceSelector{InstanceID: "inst-discovered"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !shared.Share || shared.InstanceID != "inst-discovered" {
		t.Fatalf("expected resolved shared entry, got %+v", shared)
	}
	if shared.DaemonURL != daemon.URL || shared.ManifestPath != manifestPath {
		t.Fatalf("expected daemon and manifest metadata to be persisted, got %+v", shared)
	}
	if len(cfg.Instances) != 1 || cfg.Instances[0].InstanceID != "inst-discovered" || !cfg.Instances[0].Share {
		t.Fatalf("expected config to persist the resolved share entry, got %+v", cfg.Instances)
	}
}

func TestShareInstanceDoesNotFlipExistingEntryWhenDaemonIsUnreachable(t *testing.T) {
	cfg := &Config{
		Instances: []SharedInstanceConfig{{
			InstanceID: "inst-1",
			DaemonURL:  "http://127.0.0.1:1",
			Share:      false,
		}},
	}

	_, err := ShareInstance(context.Background(), cfg, InstanceSelector{InstanceID: "inst-1"}, nil)
	if err == nil {
		t.Fatal("expected unreachable daemon share to fail")
	}
	if !strings.Contains(err.Error(), "is not reachable through daemon") {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Instances) != 1 || cfg.Instances[0].Share {
		t.Fatalf("expected failed share to preserve share=false, got %+v", cfg.Instances)
	}
}
