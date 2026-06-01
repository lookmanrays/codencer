package release

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotDryRunWritesManifest(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("readme"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "LICENSE"), []byte("license"), 0644); err != nil {
		t.Fatal(err)
	}
	report, err := Snapshot(Options{Version: "v-test", RepoRoot: repo, DistDir: filepath.Join(repo, "dist"), DryRun: true})
	if err != nil {
		t.Fatalf("snapshot dry-run: %v", err)
	}
	if !report.OK {
		t.Fatalf("dry run should be ok: %+v", report)
	}
	if _, err := os.Stat(report.ManifestPath); err != nil {
		t.Fatalf("manifest missing: %v", err)
	}
	if len(report.Artifacts) == 0 || report.Artifacts[0].Status != "skipped" {
		t.Fatalf("expected skipped artifacts: %+v", report.Artifacts)
	}
}
