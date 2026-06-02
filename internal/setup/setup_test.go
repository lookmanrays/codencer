package setup

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRelayWithoutTokenGuidesWithoutConfiguring(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODENCER_HOME", home)
	report, err := Relay(nilContext(), RelayOptions{BaseURL: "https://relay.example.com"})
	if err != nil {
		t.Fatalf("relay setup: %v", err)
	}
	if !report.OK || report.Configured || report.ExitCode != 0 {
		t.Fatalf("expected non-strict guidance success, got %+v", report)
	}
	if len(report.NextCommands) == 0 {
		t.Fatalf("expected next commands")
	}
}

func TestRelayGenerateTokenWrites0600Files(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODENCER_HOME", home)
	report, err := Relay(nilContext(), RelayOptions{BaseURL: "https://relay.example.com", GeneratePlannerToken: true})
	if err != nil {
		t.Fatalf("relay setup: %v", err)
	}
	if !report.OK || !report.Configured {
		t.Fatalf("expected configured relay, got %+v", report)
	}
	info, err := os.Stat(filepath.Join(home, "tokens", "planner-token"))
	if err != nil {
		t.Fatalf("planner token file missing: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("token file mode = %o", info.Mode().Perm())
	}
}

func TestRelayOAuthDevSetupWritesHashesAndRedactsSecrets(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODENCER_HOME", home)
	report, err := Relay(nilContext(), RelayOptions{
		BaseURL:               "https://relay.example.com",
		GeneratePlannerToken:  true,
		EnableChatGPTOAuthDev: true,
		OAuthClientSecret:     "literal-client-secret",
		OAuthClientID:         "codencer-chatgpt-dev",
	})
	if err != nil {
		t.Fatalf("relay setup: %v", err)
	}
	if !report.OK || !report.Configured {
		t.Fatalf("expected configured report, got %+v", report)
	}
	encoded := mustJSON(t, report)
	if strings.Contains(encoded, "literal-client-secret") {
		t.Fatalf("client secret leaked in report: %s", encoded)
	}
	operatorPath := filepath.Join(home, "tokens", "chatgpt-oauth-operator-code")
	info, err := os.Stat(operatorPath)
	if err != nil {
		t.Fatalf("operator code file missing: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("operator code file mode = %o", info.Mode().Perm())
	}
	cfgPath := filepath.Join(home, "runtime", "relay", "config.json")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("relay config missing: %v", err)
	}
	if strings.Contains(string(data), "literal-client-secret") || strings.Contains(string(data), "operator-code") {
		t.Fatalf("relay config leaked secret material: %s", data)
	}
	if !strings.Contains(string(data), `"client_secret_hash"`) || !strings.Contains(string(data), `"operator_code_hash"`) {
		t.Fatalf("relay config did not store OAuth hashes: %s", data)
	}
}

func TestRelayDevNoAuthDefaultsToFakeReadOnlyProjects(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODENCER_HOME", home)
	report, err := Relay(nilContext(), RelayOptions{
		BaseURL:              "https://relay.example.com",
		GeneratePlannerToken: true,
		ChatGPTDevNoAuth:     true,
	})
	if err != nil {
		t.Fatalf("relay setup: %v", err)
	}
	if !report.OK || !report.Configured {
		t.Fatalf("expected configured report, got %+v", report)
	}
	data, err := os.ReadFile(filepath.Join(home, "runtime", "relay", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		ChatGPTDevNoAuth struct {
			Enabled    bool     `json:"enabled"`
			Scopes     []string `json:"scopes"`
			ProjectIDs []string `json:"project_ids"`
		} `json:"chatgpt_dev_noauth"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if !raw.ChatGPTDevNoAuth.Enabled {
		t.Fatalf("dev-noauth was not enabled: %s", data)
	}
	for _, scope := range raw.ChatGPTDevNoAuth.Scopes {
		if strings.HasSuffix(scope, ":write") {
			t.Fatalf("dev-noauth default scope should be read-only, got %v", raw.ChatGPTDevNoAuth.Scopes)
		}
	}
	if len(raw.ChatGPTDevNoAuth.ProjectIDs) == 0 || raw.ChatGPTDevNoAuth.ProjectIDs[0] != "fake" {
		t.Fatalf("dev-noauth should restrict default project ids, got %v", raw.ChatGPTDevNoAuth.ProjectIDs)
	}
}

func TestMCPConfigRedactsLiteralToken(t *testing.T) {
	report, err := MCP(MCPOptions{Client: "claude-code", Endpoint: "https://relay.example.com/mcp", Token: "super-secret"})
	if err != nil {
		t.Fatalf("mcp setup: %v", err)
	}
	encoded := mustJSON(t, report)
	if contains(encoded, "super-secret") {
		t.Fatalf("literal token leaked: %s", encoded)
	}
}

func nilContext() context.Context {
	return context.Background()
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func contains(value, needle string) bool {
	return strings.Contains(value, needle)
}
