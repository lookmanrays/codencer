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
	cfg := readRelaySetupConfig(t, home)
	if cfg.ProxyTimeoutSeconds != 300 {
		t.Fatalf("default proxy timeout = %d, want 300", cfg.ProxyTimeoutSeconds)
	}
	output, ok := report.Output.(map[string]any)
	if !ok || output["proxy_timeout_seconds"] != 300 {
		t.Fatalf("relay output missing proxy timeout: %#v", report.Output)
	}
}

func TestRelayCustomProxyTimeoutWritesConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODENCER_HOME", home)
	report, err := Relay(nilContext(), RelayOptions{
		BaseURL:              "https://relay.example.com",
		GeneratePlannerToken: true,
		ProxyTimeoutSeconds:  600,
	})
	if err != nil {
		t.Fatalf("relay setup: %v", err)
	}
	if !report.OK || !report.Configured {
		t.Fatalf("expected configured relay, got %+v", report)
	}
	cfg := readRelaySetupConfig(t, home)
	if cfg.ProxyTimeoutSeconds != 600 {
		t.Fatalf("custom proxy timeout = %d, want 600", cfg.ProxyTimeoutSeconds)
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

func TestGatewaySetupWritesConfigHashesAndRedactsSecrets(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODENCER_HOME", home)
	report, err := Gateway(nilContext(), GatewayOptions{
		BaseURL:           "http://127.0.0.1:19090",
		MCPURL:            "http://127.0.0.1:19090/mcp",
		ListenAddr:        "127.0.0.1:19090",
		TokenEnv:          "CODENCER_GATEWAY_MCP_TOKEN",
		EnableOAuthDev:    true,
		OAuthClientSecret: "literal-gateway-client-secret",
	})
	if err != nil {
		t.Fatalf("gateway setup: %v", err)
	}
	if !report.OK || !report.Configured {
		t.Fatalf("expected configured gateway, got %+v", report)
	}
	encoded := mustJSON(t, report)
	if strings.Contains(encoded, "literal-gateway-client-secret") {
		t.Fatalf("gateway report leaked client secret: %s", encoded)
	}
	cfgPath := filepath.Join(home, "runtime", "gateway", "config.json")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("gateway config missing: %v", err)
	}
	if strings.Contains(string(data), "literal-gateway-client-secret") || strings.Contains(string(data), "operator-code") {
		t.Fatalf("gateway config leaked secret material: %s", data)
	}
	for _, want := range []string{`"client_secret_hash"`, `"operator_code_hash"`, `"token_env": "CODENCER_GATEWAY_MCP_TOKEN"`} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("gateway config missing %q: %s", want, data)
		}
	}
	cfg := readGatewaySetupConfig(t, home)
	if cfg.RelayRequestTimeoutSeconds != 300 {
		t.Fatalf("default relay request timeout = %d, want 300", cfg.RelayRequestTimeoutSeconds)
	}
	output, ok := report.Output.(map[string]any)
	if !ok || output["relay_request_timeout_seconds"] != 300 {
		t.Fatalf("gateway output missing relay request timeout: %#v", report.Output)
	}
	if _, err := os.Stat(filepath.Join(home, "tokens", "gateway-oauth-operator-code")); err != nil {
		t.Fatalf("gateway operator code file missing: %v", err)
	}
	localConfig, err := os.ReadFile(filepath.Join(home, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(localConfig), `"gateway_config_path"`) {
		t.Fatalf("local config did not record gateway config path: %s", localConfig)
	}
}

func TestGatewayAndSelfHostCustomRelayRequestTimeoutWriteConfig(t *testing.T) {
	t.Run("gateway", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("CODENCER_HOME", home)
		report, err := Gateway(nilContext(), GatewayOptions{
			BaseURL:                    "http://127.0.0.1:19090",
			RelayRequestTimeoutSeconds: 600,
		})
		if err != nil {
			t.Fatalf("gateway setup: %v", err)
		}
		if !report.OK || !report.Configured {
			t.Fatalf("expected configured gateway, got %+v", report)
		}
		cfg := readGatewaySetupConfig(t, home)
		if cfg.RelayRequestTimeoutSeconds != 600 {
			t.Fatalf("custom gateway relay request timeout = %d, want 600", cfg.RelayRequestTimeoutSeconds)
		}
	})

	t.Run("self-host", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("CODENCER_HOME", home)
		report, err := SelfHost(nilContext(), SelfHostOptions{
			GatewayURL:                 "http://127.0.0.1:19090",
			RelayURL:                   "http://127.0.0.1:8090",
			RelayRequestTimeoutSeconds: 700,
		})
		if err != nil {
			t.Fatalf("self-host setup: %v", err)
		}
		if !report.OK || !report.Configured {
			t.Fatalf("expected configured self-host gateway, got %+v", report)
		}
		cfg := readGatewaySetupConfig(t, home)
		if cfg.RelayRequestTimeoutSeconds != 700 {
			t.Fatalf("custom self-host relay request timeout = %d, want 700", cfg.RelayRequestTimeoutSeconds)
		}
	})
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

func readRelaySetupConfig(t *testing.T, home string) struct {
	ProxyTimeoutSeconds int `json:"proxy_timeout_seconds"`
} {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, "runtime", "relay", "config.json"))
	if err != nil {
		t.Fatalf("read relay config: %v", err)
	}
	var cfg struct {
		ProxyTimeoutSeconds int `json:"proxy_timeout_seconds"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("decode relay config: %v", err)
	}
	return cfg
}

func readGatewaySetupConfig(t *testing.T, home string) struct {
	RelayRequestTimeoutSeconds int `json:"relay_request_timeout_seconds"`
} {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, "runtime", "gateway", "config.json"))
	if err != nil {
		t.Fatalf("read gateway config: %v", err)
	}
	var cfg struct {
		RelayRequestTimeoutSeconds int `json:"relay_request_timeout_seconds"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("decode gateway config: %v", err)
	}
	return cfg
}
