package activation

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPackageGenerationRedactsTokensAndWritesExpectedFiles(t *testing.T) {
	home := t.TempDir()
	report, err := Package(context.Background(), Options{
		Relay:        "https://relay.example.com",
		Token:        "literal-secret-token",
		TokenEnv:     "CODENCER_MCP_TOKEN",
		ProjectID:    "codencer",
		CodencerHome: home,
		Now:          fixedNow,
	})
	if err != nil {
		t.Fatalf("package: %v", err)
	}
	if !report.OK || report.PackagePath == "" {
		t.Fatalf("expected package success, got %+v", report)
	}
	expected := []string{"activation-package.json", "README.md", "curl-smoke.sh", "codex-config.toml", "claude-code-command.sh", "chatgpt-app-setup.md", "connector-enrollment.sh"}
	for _, name := range expected {
		data, err := os.ReadFile(filepath.Join(report.PackagePath, name))
		if err != nil {
			t.Fatalf("%s missing: %v", name, err)
		}
		if strings.Contains(string(data), "literal-secret-token") {
			t.Fatalf("%s leaked literal token: %s", name, data)
		}
		if name == "activation-package.json" && strings.Contains(string(data), home) {
			t.Fatalf("activation package should not expose local CODENCER_HOME: %s", data)
		}
	}
	if info, err := os.Stat(filepath.Join(report.PackagePath, "curl-smoke.sh")); err != nil || info.Mode().Perm() != 0700 {
		t.Fatalf("curl-smoke.sh mode mismatch: info=%v err=%v", info, err)
	}
	curlSmoke, _ := os.ReadFile(filepath.Join(report.PackagePath, "curl-smoke.sh"))
	for _, want := range []string{"initialize", "tools/list", "codencer.list_projects", "MCP-Session-Id", "RUN_FAKE_MANIFEST", "codencer.run_project_manifest"} {
		if !strings.Contains(string(curlSmoke), want) {
			t.Fatalf("curl-smoke.sh missing %q:\n%s", want, curlSmoke)
		}
	}
	connectorEnrollment, _ := os.ReadFile(filepath.Join(report.PackagePath, "connector-enrollment.sh"))
	for _, want := range []string{"codencer-relayd enrollment-token create", "codencer connector enroll", "codencer-connectord enroll", "codencer project share"} {
		if !strings.Contains(string(connectorEnrollment), want) {
			t.Fatalf("connector-enrollment.sh missing %q:\n%s", want, connectorEnrollment)
		}
	}
}

func TestGatewayPackagePointsClientsAtGatewayURL(t *testing.T) {
	home := t.TempDir()
	report, err := Gateway(context.Background(), Options{
		Gateway:      "https://mcp.codencer.dev",
		Relay:        "https://relay.example.com",
		Token:        "literal-gateway-token",
		TokenEnv:     "CODENCER_GATEWAY_MCP_TOKEN",
		ProjectID:    "codencer",
		CodencerHome: home,
		Now:          fixedNow,
	})
	if err != nil {
		t.Fatalf("gateway package: %v", err)
	}
	if !report.OK || report.PackagePath == "" {
		t.Fatalf("expected gateway package success, got %+v", report)
	}
	expected := []string{"activation-package.json", "README.md", "gateway-curl-smoke.sh", "codex-config.toml", "claude-code-command.sh", "chatgpt-app-setup.md", "relay-profile-setup.sh", "relay-profile-setup.md", "connector-login.md", "evidence-checklist.md"}
	for _, name := range expected {
		data, err := os.ReadFile(filepath.Join(report.PackagePath, name))
		if err != nil {
			t.Fatalf("%s missing: %v", name, err)
		}
		text := string(data)
		if strings.Contains(text, "literal-gateway-token") {
			t.Fatalf("%s leaked literal token: %s", name, data)
		}
		if (name == "codex-config.toml" || name == "claude-code-command.sh" || name == "chatgpt-app-setup.md" || name == "gateway-curl-smoke.sh") && !strings.Contains(text, "https://mcp.codencer.dev/mcp") {
			t.Fatalf("%s does not point at Gateway MCP URL:\n%s", name, data)
		}
		if (name == "codex-config.toml" || name == "claude-code-command.sh" || name == "chatgpt-app-setup.md") && strings.Contains(text, "https://relay.example.com/mcp") {
			t.Fatalf("%s points client at direct Relay MCP:\n%s", name, data)
		}
	}
	readme, _ := os.ReadFile(filepath.Join(report.PackagePath, "README.md"))
	if !strings.Contains(string(readme), "AI client -> Codencer Gateway -> selected Relay") {
		t.Fatalf("gateway README missing official routing path:\n%s", readme)
	}
	relaySetup, _ := os.ReadFile(filepath.Join(report.PackagePath, "relay-profile-setup.sh"))
	for _, want := range []string{"codencer login --gateway", "codencer gateway relay add", "https://relay.example.com"} {
		if !strings.Contains(string(relaySetup), want) {
			t.Fatalf("relay profile setup missing %q:\n%s", want, relaySetup)
		}
	}
	chatgpt, _ := os.ReadFile(filepath.Join(report.PackagePath, "chatgpt-app-setup.md"))
	if !strings.Contains(string(chatgpt), filepath.Join(home, "tokens", "gateway-oauth-client-secret")) {
		t.Fatalf("Gateway ChatGPT setup should reference gateway OAuth secret files:\n%s", chatgpt)
	}
}

func TestClientOutputsContainExpectedActivationGuidance(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(Options) (Report, error)
		want string
	}{
		{name: "chatgpt", run: ChatGPT, want: "pending_manual_product_proof"},
		{name: "codex", run: Codex, want: "codex mcp add"},
		{name: "claude-code", run: ClaudeCode, want: "claude mcp add"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report, err := tc.run(Options{Relay: "https://relay.example.com", Token: "secret-token", ProjectID: "codencer"})
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if !report.OK {
				t.Fatalf("expected ok report: %+v", report)
			}
			data, _ := json.Marshal(report)
			if !strings.Contains(string(data), tc.want) {
				t.Fatalf("expected %q in report: %s", tc.want, data)
			}
			if strings.Contains(string(data), "secret-token") {
				t.Fatalf("literal token leaked: %s", data)
			}
		})
	}
}

func TestChatGPTSetupSheetIncludesRequiredFields(t *testing.T) {
	home := t.TempDir()
	report, err := ChatGPT(Options{
		Relay:        "https://relay.example.com",
		Token:        "literal-secret-token",
		ProjectID:    "codencer",
		AuthMode:     "oauth",
		CodencerHome: home,
	})
	if err != nil {
		t.Fatal(err)
	}
	sheet, ok := report.Output.(map[string]any)
	if !ok {
		t.Fatalf("unexpected output: %#v", report.Output)
	}
	for _, key := range []string{
		"mcp_endpoint",
		"auth_mode",
		"client_id",
		"client_secret_file",
		"operator_code_file",
		"authorization_server_metadata",
		"openid_configuration",
		"authorization_endpoint",
		"token_endpoint",
		"protected_resource_metadata",
		"scopes",
		"expected_tools",
		"chatgpt_ui_steps",
		"test_prompts",
		"evidence_checklist",
	} {
		if _, ok := sheet[key]; !ok {
			t.Fatalf("ChatGPT setup sheet missing %q: %#v", key, sheet)
		}
	}
	data, _ := json.Marshal(sheet)
	if strings.Contains(string(data), "literal-secret-token") {
		t.Fatalf("literal token leaked: %s", data)
	}
	if !strings.Contains(string(data), filepath.Join(home, "tokens", "chatgpt-oauth-client-secret")) {
		t.Fatalf("client secret file path missing: %s", data)
	}
	if !strings.Contains(string(data), "valid redirect URIs are accepted for dev") {
		t.Fatalf("redirect behavior missing: %s", data)
	}
}

func TestCheckWithoutRelayIsLocalOnly(t *testing.T) {
	home := t.TempDir()
	report, err := CheckActivation(context.Background(), Options{CodencerHome: home, Now: fixedNow})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !report.OK || report.Summary.Skipped == 0 {
		t.Fatalf("expected local-only check with skipped relay, got %+v", report)
	}
	if _, err := os.Stat(filepath.Join(home, "config.json")); err != nil {
		t.Fatalf("config not initialized: %v", err)
	}
}

func fixedNow() time.Time {
	return time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
}
