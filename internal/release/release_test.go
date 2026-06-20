package release

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSnapshotDryRunWritesManifest(t *testing.T) {
	repo := makeReleaseRepo(t)
	report, err := Snapshot(Options{Version: "v-test", RepoRoot: repo, DistDir: filepath.Join(repo, "dist"), DryRun: true, Targets: "host"})
	if err != nil {
		t.Fatalf("snapshot dry-run: %v", err)
	}
	if !report.OK {
		t.Fatalf("dry run should be ok: %+v", report)
	}
	if _, err := os.Stat(report.ManifestPath); err != nil {
		t.Fatalf("manifest missing: %v", err)
	}
	if len(report.Artifacts) != 1 || report.Artifacts[0].Status != artifactStatusSkip {
		t.Fatalf("expected skipped host artifact: %+v", report.Artifacts)
	}
}

func TestTargetParsingAndRequiredValidation(t *testing.T) {
	targets, err := parseTargets("darwin/arm64,linux/amd64,darwin/arm64")
	if err != nil {
		t.Fatalf("parse targets: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("expected duplicate target removal: %+v", targets)
	}
	if _, err := parseTargets("windows/amd64"); err == nil || !strings.Contains(err.Error(), "not claimed") {
		t.Fatalf("expected windows native rejection, got %v", err)
	}
	if _, err := parseRequiredTargets("linux/amd64", []target{{OS: "darwin", Arch: "arm64"}}); err == nil {
		t.Fatal("expected required target outside selected target set to fail")
	}
}

func TestReportFailsRequiredTargetWithoutPartial(t *testing.T) {
	repo := makeReleaseRepo(t)
	report, err := Snapshot(Options{
		Version:        "v-test",
		RepoRoot:       repo,
		DistDir:        filepath.Join(repo, "dist"),
		Targets:        "host",
		RequireTargets: "host",
		CommandRunner: func(cmd *exec.Cmd) ([]byte, error) {
			return nil, os.ErrNotExist
		},
	})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if report.OK {
		t.Fatalf("required failed target should make report not ok: %+v", report)
	}
	if len(report.Errors) == 0 {
		t.Fatalf("expected required target error: %+v", report)
	}
}

func TestReportAllowsExplicitPartialWithBuiltArtifact(t *testing.T) {
	artifacts := []Artifact{
		{Status: artifactStatusBuilt},
		{Status: artifactStatusFailed, Required: true},
	}
	if !reportOK(false, true, artifacts, []string{"required target missing"}) {
		t.Fatal("expected explicit partial mode to allow report with at least one built artifact")
	}
	if reportOK(false, false, artifacts, []string{"required target missing"}) {
		t.Fatal("expected non-partial mode to fail")
	}
}

func TestDockerBuildCommandForLinuxFromNonLinuxHost(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("docker command selection is only relevant from non-Linux hosts")
	}
	repo := t.TempDir()
	out := filepath.Join(repo, "dist", ".work", "bin", "codencer")
	cmd, err := buildCommand(repo, out, "./cmd/codencer", "-X x=y", target{OS: "linux", Arch: "amd64"}, "docker", "golang:1.25-bookworm")
	if err != nil {
		t.Fatalf("build command: %v", err)
	}
	joined := strings.Join(cmd.Args, " ")
	for _, want := range []string{"docker run", "--platform linux/amd64", "GOOS=linux", "GOARCH=amd64", "golang:1.25-bookworm", "/workspace/dist/.work/bin/codencer"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("docker command missing %q: %s", want, joined)
		}
	}
}

func TestBundleIncludesDocsAndScripts(t *testing.T) {
	repo := makeReleaseRepo(t)
	stage := filepath.Join(t.TempDir(), "stage")
	if err := writeBundleFiles(repo, stage); err != nil {
		t.Fatalf("write bundle files: %v", err)
	}
	for _, rel := range []string{
		"README.md",
		"LICENSE",
		"NOTICE",
		"TRADEMARKS.md",
		"SECURITY.md",
		"CONTRIBUTING.md",
		"CODE_OF_CONDUCT.md",
		"QUICKSTART.txt",
		"scripts/install.sh",
		"scripts/uninstall.sh",
		"scripts/upgrade.sh",
		"docs/local-production.md",
		"docs/mcp/integrations.md",
	} {
		if _, err := os.Stat(filepath.Join(stage, rel)); err != nil {
			t.Fatalf("expected bundle file %s: %v", rel, err)
		}
	}
	info, err := os.Stat(filepath.Join(stage, "scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0111 == 0 {
		t.Fatalf("expected install script executable mode, got %v", info.Mode())
	}
}

func TestValidateDistDetectsMissingBuiltArtifact(t *testing.T) {
	dist := t.TempDir()
	manifest := Manifest{
		Version: "v-test",
		Artifacts: []Artifact{{
			Name:   "missing.tar.gz",
			OS:     "darwin",
			Arch:   "arm64",
			Status: artifactStatusBuilt,
			SHA256: "abc",
		}},
	}
	writeTestJSON(t, filepath.Join(dist, "manifest.json"), manifest)
	if err := os.WriteFile(filepath.Join(dist, "checksums.txt"), []byte("abc  missing.tar.gz\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDist(dist); err == nil || !strings.Contains(err.Error(), "missing on disk") {
		t.Fatalf("expected missing artifact validation error, got %v", err)
	}
}

func TestValidateDistChecksumsMatch(t *testing.T) {
	dist := t.TempDir()
	artifact := filepath.Join(dist, "artifact.tar.gz")
	if err := os.WriteFile(artifact, []byte("payload"), 0644); err != nil {
		t.Fatal(err)
	}
	sum, err := sha256File(artifact)
	if err != nil {
		t.Fatal(err)
	}
	manifest := Manifest{
		Version: "v-test",
		Artifacts: []Artifact{{
			Name:   filepath.Base(artifact),
			OS:     "darwin",
			Arch:   "arm64",
			Status: artifactStatusBuilt,
			SHA256: sum,
		}},
	}
	writeTestJSON(t, filepath.Join(dist, "manifest.json"), manifest)
	if err := os.WriteFile(filepath.Join(dist, "checksums.txt"), []byte(sum+"  "+filepath.Base(artifact)+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDist(dist); err != nil {
		t.Fatalf("validate dist: %v", err)
	}
}

func TestTarPreservesExecutableScripts(t *testing.T) {
	repo := makeReleaseRepo(t)
	stage := filepath.Join(t.TempDir(), "stage")
	if err := writeBundleFiles(repo, stage); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "bundle.tar.gz")
	if err := tarGzDir(stage, archive); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err != nil {
			break
		}
		if header.Name == filepath.ToSlash(filepath.Base(stage)+"/scripts/install.sh") {
			if header.FileInfo().Mode().Perm()&0111 == 0 {
				t.Fatalf("expected executable script in archive, got %v", header.FileInfo().Mode())
			}
			return
		}
	}
	t.Fatal("install script not found in archive")
}

func makeReleaseRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	files := map[string]string{
		"README.md":                     "readme",
		"LICENSE":                       "license",
		"NOTICE":                        "notice",
		"TRADEMARKS.md":                 "trademarks",
		"SECURITY.md":                   "security",
		"CONTRIBUTING.md":               "contributing",
		"CODE_OF_CONDUCT.md":            "conduct",
		"scripts/install.sh":            "#!/bin/sh\n",
		"scripts/uninstall.sh":          "#!/bin/sh\n",
		"scripts/upgrade.sh":            "#!/bin/sh\n",
		"docs/local-production.md":      "local",
		"docs/mcp/integrations.md":      "mcp",
		"docs/mcp/relay_tools.md":       "tools",
		"docs/runtime-supervisor.md":    "runtime",
		"docs/live-execution-matrix.md": "live",
	}
	for rel, body := range files {
		mode := os.FileMode(0644)
		if strings.HasPrefix(rel, "scripts/") {
			mode = 0755
		}
		path := filepath.Join(repo, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
	}
	return repo
}

func writeTestJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
}
