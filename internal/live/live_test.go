package live

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMatrixDefaultSkipsLiveAndPersists(t *testing.T) {
	clearLiveEnv(t)
	home := t.TempDir()
	repo := t.TempDir()
	report, err := Matrix(context.Background(), Options{Profile: "local", CodencerHome: home, RepoRoot: repo})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("default matrix should be ok with skips: %+v", report)
	}
	if report.ReportPath == "" {
		t.Fatal("report path was not set")
	}
	if _, err := os.Stat(report.ReportPath); err != nil {
		t.Fatalf("report was not persisted: %v", err)
	}
	var foundCodexSkip bool
	for _, check := range report.Checks {
		if check.ID == "codex_live_execution" && check.Status == StatusSkipped {
			foundCodexSkip = true
		}
	}
	if !foundCodexSkip {
		t.Fatalf("expected skipped codex live check: %+v", report.Checks)
	}
	data, _ := json.Marshal(report)
	if strings.Contains(string(data), "suggested_next_action") {
		t.Fatalf("live report must not emit suggested_next_action: %s", data)
	}
}

func TestGuardedLiveCommandsSkipWithoutEnv(t *testing.T) {
	clearLiveEnv(t)
	for name, fn := range map[string]func(context.Context, Options) (Report, error){
		"codex":             RunCodex,
		"claude":            RunClaude,
		"relay-mcp":         RunRelayMCP,
		"restart-reconnect": RunRestartReconnect,
	} {
		t.Run(name, func(t *testing.T) {
			report, err := fn(context.Background(), Options{CodencerHome: t.TempDir(), RepoRoot: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			if !report.OK || report.Summary.Skipped == 0 {
				t.Fatalf("expected ok skipped report, got %+v", report)
			}
		})
	}
}

func TestMCPProofSkippedStillGeneratesConfig(t *testing.T) {
	clearLiveEnv(t)
	report, err := RunCodexMCP(context.Background(), Options{CodencerHome: t.TempDir(), RepoRoot: t.TempDir(), Endpoint: "https://relay.example.com/mcp"})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("disabled MCP proof should be ok with skipped proof: %+v", report)
	}
	if len(report.Checks) < 2 || report.Checks[0].Status != StatusPassed {
		t.Fatalf("expected config generation check, got %+v", report.Checks)
	}
}

func TestRedactSecrets(t *testing.T) {
	input := `Authorization: Bearer super-secret-token planner_token private_key`
	got := Redact(input)
	if strings.Contains(got, "super-secret-token") || strings.Contains(got, "planner_token") || strings.Contains(got, "private_key") {
		t.Fatalf("secret was not redacted: %s", got)
	}
}

func TestListReportsEmpty(t *testing.T) {
	home := t.TempDir()
	files, err := ListReports(home, filepath.Join("missing", "reports"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("expected no reports, got %+v", files)
	}
}

func clearLiveEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		EnvLiveAll,
		EnvLiveCodex,
		EnvLiveClaude,
		EnvLiveRelayMCP,
		EnvLiveCodexMCP,
		EnvLiveClaudeMCP,
		EnvLiveWSL,
		EnvLiveServiceRestart,
		"CODENCER_LIVE_CODEX_SMOKE",
		"CODENCER_LIVE_CLAUDE_SMOKE",
	} {
		t.Setenv(name, "0")
	}
}
