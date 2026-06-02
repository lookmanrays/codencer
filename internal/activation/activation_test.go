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
	expected := []string{"activation-package.json", "README.md", "curl-smoke.sh", "codex-config.toml", "claude-code-command.sh", "chatgpt-app-setup.md"}
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
