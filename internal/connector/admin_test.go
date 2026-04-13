package connector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
