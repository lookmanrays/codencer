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
