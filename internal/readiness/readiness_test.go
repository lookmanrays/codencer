package readiness

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestBuildDefaultReadyWithSkips(t *testing.T) {
	home := t.TempDir()
	repo := t.TempDir()
	report, err := Build(context.Background(), Options{CodencerHome: home, RepoRoot: repo})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Verdict != VerdictReadyWithSkips {
		t.Fatalf("expected ready_with_skips, got %+v", report)
	}
	if report.ReportPath == "" {
		t.Fatal("report path was not set")
	}
	if _, err := os.Stat(report.ReportPath); err != nil {
		t.Fatalf("readiness report missing: %v", err)
	}
}

func TestBuildRelayStrictBlocksWhenUnconfigured(t *testing.T) {
	report, err := Build(context.Background(), Options{CodencerHome: t.TempDir(), RepoRoot: t.TempDir(), Relay: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || report.Verdict != VerdictNotReady {
		t.Fatalf("expected not_ready when relay is required but unconfigured: %+v", report)
	}
	if !contains(report.BlockingGates, "relay_mcp_config") {
		t.Fatalf("expected relay_mcp_config blocker, got %+v", report.BlockingGates)
	}
}

func TestNoSuggestedNextAction(t *testing.T) {
	report, err := Build(context.Background(), Options{CodencerHome: t.TempDir(), RepoRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(report.ReportPath), "suggested_next_action") {
		t.Fatalf("unexpected suggested_next_action marker")
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
