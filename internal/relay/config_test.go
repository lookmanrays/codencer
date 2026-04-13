package relay_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"agent-bridge/internal/relay"
)

func TestLoadConfigRejectsInvalidRelayPortEnv(t *testing.T) {
	t.Setenv("RELAY_PORT", "not-a-port")
	t.Setenv("RELAY_PLANNER_TOKEN", "planner-token")
	t.Setenv("RELAY_DB_PATH", filepath.Join(t.TempDir(), "relay.db"))

	if _, err := relay.LoadConfig(""); err == nil {
		t.Fatal("expected invalid RELAY_PORT to fail")
	}
}

func TestLoadConfigReadsProxyTimeoutAndAllowedOrigins(t *testing.T) {
	t.Setenv("RELAY_PLANNER_TOKEN", "planner-token")
	t.Setenv("RELAY_DB_PATH", filepath.Join(t.TempDir(), "relay.db"))
	t.Setenv("RELAY_PROXY_TIMEOUT_SECONDS", "120")
	t.Setenv("RELAY_ALLOWED_ORIGINS", "https://chat.example.com, https://claude.example.com")

	cfg, err := relay.LoadConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ProxyTimeoutSeconds != 120 {
		t.Fatalf("expected proxy timeout 120, got %d", cfg.ProxyTimeoutSeconds)
	}
	expected := []string{"https://chat.example.com", "https://claude.example.com"}
	if !reflect.DeepEqual(cfg.AllowedOrigins, expected) {
		t.Fatalf("expected allowed origins %v, got %v", expected, cfg.AllowedOrigins)
	}
}

func TestSaveConfigPersistsPlannerTokens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "relay.json")
	cfg := relay.DefaultConfig()
	cfg.DBPath = filepath.Join(t.TempDir(), "relay.db")
	cfg.PlannerTokens = []relay.PlannerTokenConfig{{
		Name:   "operator",
		Token:  "planner-token",
		Scopes: []string{"*"},
	}}

	if err := relay.SaveConfig(path, cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}

	loaded, err := relay.LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.PlannerTokens) != 1 || loaded.PlannerTokens[0].Name != "operator" {
		t.Fatalf("unexpected planner tokens after save/load: %+v", loaded.PlannerTokens)
	}
}
