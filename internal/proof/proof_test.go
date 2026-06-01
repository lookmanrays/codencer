package proof

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-bridge/internal/live"
	"agent-bridge/internal/local"
)

func TestBundleCopiesLatestReportsAndRedacts(t *testing.T) {
	home := t.TempDir()
	repo := t.TempDir()
	t.Setenv("CODENCER_HOME", home)
	paths, err := local.ResolvePathsForHome(repo, "", home)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := local.EnsureHome(paths, now(nil)); err != nil {
		t.Fatal(err)
	}
	if _, err := live.PersistJSON(paths, "acceptance", map[string]any{"token": "secret"}); err != nil {
		t.Fatal(err)
	}
	report, err := Bundle(Options{CodencerHome: home, RepoRoot: repo})
	if err != nil {
		t.Fatalf("bundle: %v", err)
	}
	if !report.OK || len(report.IncludedReports) != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	data, err := os.ReadFile(filepath.Join(report.ReportsDir, filepath.Base(report.IncludedReports[0])))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret") {
		t.Fatalf("secret leaked in copied report: %s", data)
	}
}
