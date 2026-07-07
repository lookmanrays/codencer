package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInstallAndUpgradeScriptsReportMissingBinaries(t *testing.T) {
	repo := filepath.Join("..", "..")
	for _, script := range []string{"install.sh", "upgrade.sh"} {
		cmd := exec.Command("bash", filepath.Join(repo, "scripts", script), "--bin-dir", t.TempDir(), "--dry-run", "--json")
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("%s should fail when required binaries are missing: %s", script, out)
		}
		var payload map[string]any
		if err := json.Unmarshal(out, &payload); err != nil {
			t.Fatalf("%s output is not JSON: %v\n%s", script, err, out)
		}
		if payload["ok"] == true || payload["partial"] != true {
			t.Fatalf("%s should report ok=false partial=true: %+v", script, payload)
		}
		missing, _ := payload["missing_binaries"].([]any)
		if len(missing) != 5 {
			t.Fatalf("%s expected all binaries missing, got %+v", script, payload["missing_binaries"])
		}
	}
}

func TestInstallAndUpgradeScriptsPassWithRequiredBinaries(t *testing.T) {
	repo := filepath.Join("..", "..")
	binDir := t.TempDir()
	for _, name := range []string{"codencer", "orchestratord", "codencer-relayd", "codencer-gatewayd", "codencer-connectord"} {
		if err := os.WriteFile(filepath.Join(binDir, name), []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, script := range []string{"install.sh", "upgrade.sh"} {
		cmd := exec.Command("bash", filepath.Join(repo, "scripts", script), "--bin-dir", binDir, "--dry-run", "--json")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s should pass with required binaries: %v\n%s", script, err, out)
		}
		var payload map[string]any
		if err := json.Unmarshal(out, &payload); err != nil {
			t.Fatalf("%s output is not JSON: %v\n%s", script, err, out)
		}
		if payload["ok"] != true || payload["partial"] != false {
			t.Fatalf("%s should report ok=true partial=false: %+v", script, payload)
		}
	}
}

func TestInstallScriptPipedModeUsesReleaseBootstrapNotCallerBin(t *testing.T) {
	repo := filepath.Join("..", "..")
	version := "v9.9.9"
	platform := "darwin_arm64"
	downloadDir, _, _ := createInstallReleaseFixture(t, repo, version, platform)
	installDir := t.TempDir()
	home := t.TempDir()
	callerCWD := t.TempDir()
	callerBin := filepath.Join(callerCWD, "bin")
	if err := os.MkdirAll(callerBin, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"codencer", "orchestratord", "codencer-relayd", "codencer-gatewayd", "codencer-connectord"} {
		if err := os.WriteFile(filepath.Join(callerBin, name), []byte("#!/bin/sh\necho caller-cwd-"+name+"\n"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	script, err := os.ReadFile(filepath.Join(repo, "scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("sh", "-s", "--",
		"--version", version,
		"--repo", "lookmanrays/codencer",
		"--platform", platform,
		"--download-dir", downloadDir,
		"--no-download",
		"--install-dir", installDir,
		"--codencer-home", home,
		"--json",
	)
	cmd.Dir = callerCWD
	cmd.Stdin = strings.NewReader(string(script))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("piped install should use release-bootstrap fixture: %v\n%s", err, out)
	}
	if strings.Contains(string(out), "install:bin/codencer") || strings.Contains(string(out), callerBin) {
		t.Fatalf("piped install leaked caller cwd bin behavior: %s", out)
	}

	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("install output is not JSON: %v\n%s", err, out)
	}
	if payload["mode"] != "release-bootstrap" {
		t.Fatalf("expected release-bootstrap mode, got %+v", payload)
	}
	if payload["artifact"] != "codencer_"+version+"_"+platform+".tar.gz" {
		t.Fatalf("unexpected artifact: %+v", payload)
	}
	if _, err := os.Stat(filepath.Join(installDir, "codencer")); err != nil {
		t.Fatalf("installed codencer missing from install dir: %v", err)
	}
	installed, err := os.ReadFile(filepath.Join(installDir, "codencer"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(installed), "caller-cwd") {
		t.Fatalf("installed binary came from caller cwd bin: %s", installed)
	}
}

func TestInstallScriptPackageLocalExplicitBinDirWorksWithoutNetwork(t *testing.T) {
	repo := filepath.Join("..", "..")
	binDir := t.TempDir()
	for _, name := range []string{"codencer", "orchestratord", "codencer-relayd", "codencer-gatewayd", "codencer-connectord"} {
		body := "#!/bin/sh\nif [ \"$1\" = \"init\" ]; then exit 0; fi\necho explicit-" + name + "\n"
		if err := os.WriteFile(filepath.Join(binDir, name), []byte(body), 0755); err != nil {
			t.Fatal(err)
		}
	}
	fakeBin := t.TempDir()
	if err := os.WriteFile(filepath.Join(fakeBin, "curl"), []byte("#!/bin/sh\necho network forbidden >&2\nexit 99\n"), 0755); err != nil {
		t.Fatal(err)
	}
	installDir := t.TempDir()
	cmd := exec.Command("sh", filepath.Join(repo, "scripts", "install.sh"),
		"--bin-dir", binDir,
		"--install-dir", installDir,
		"--codencer-home", t.TempDir(),
		"--json",
	)
	cmd.Env = append(os.Environ(), "PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("explicit package-local install should not need network: %v\n%s", err, out)
	}
	if strings.Contains(string(out), "network forbidden") {
		t.Fatalf("package-local install used network: %s", out)
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("install output is not JSON: %v\n%s", err, out)
	}
	if payload["mode"] != "package-local" {
		t.Fatalf("expected package-local mode, got %+v", payload)
	}
	installed, err := os.ReadFile(filepath.Join(installDir, "codencer"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(installed), "explicit-codencer") {
		t.Fatalf("installed binary did not come from explicit bin dir: %s", installed)
	}
}

func TestInstallScriptPackageLocalUsesScriptPathNotCallerCWD(t *testing.T) {
	repo := filepath.Join("..", "..")
	version := "v9.9.9"
	platform := "darwin_arm64"
	_, packageDir, _ := createInstallReleaseFixture(t, repo, version, platform)
	installDir := t.TempDir()
	home := t.TempDir()

	cmd := exec.Command("sh", filepath.Join(packageDir, "scripts", "install.sh"),
		"--install-dir", installDir,
		"--codencer-home", home,
		"--json",
	)
	cmd.Dir = t.TempDir()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("package-local install should succeed without network: %v\n%s", err, out)
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("install output is not JSON: %v\n%s", err, out)
	}
	if payload["mode"] != "package-local" {
		t.Fatalf("expected package-local mode, got %+v", payload)
	}
	if _, err := os.Stat(filepath.Join(installDir, "codencer")); err != nil {
		t.Fatalf("installed codencer missing from install dir: %v", err)
	}
}

func TestInstallScriptReleaseDryRunDoesNotDownloadExtractOrCreateDirs(t *testing.T) {
	repo := filepath.Join("..", "..")
	script, err := os.ReadFile(filepath.Join(repo, "scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	downloadDir := filepath.Join(root, "downloads")
	installDir := filepath.Join(root, "install")
	home := filepath.Join(root, "home")
	fakeBin := t.TempDir()
	if err := os.WriteFile(filepath.Join(fakeBin, "curl"), []byte("#!/bin/sh\necho dry-run network forbidden >&2\nexit 99\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fakeBin, "tar"), []byte("#!/bin/sh\necho dry-run extraction forbidden >&2\nexit 99\n"), 0755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-s", "--",
		"--platform", "darwin_arm64",
		"--download-dir", downloadDir,
		"--install-dir", installDir,
		"--codencer-home", home,
		"--dry-run",
		"--json",
	)
	cmd.Stdin = strings.NewReader(string(script))
	cmd.Env = append(os.Environ(), "PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("dry-run should not require network or extraction: %v\n%s", err, out)
	}
	if strings.Contains(string(out), "forbidden") {
		t.Fatalf("dry-run used network or extraction: %s", out)
	}
	for _, path := range []string{downloadDir, installDir, home} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("dry-run should not create %s, stat err=%v", path, err)
		}
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("dry-run output is not JSON: %v\n%s", err, out)
	}
	if payload["mode"] != "release-bootstrap" || payload["version"] != "latest" {
		t.Fatalf("unexpected dry-run payload: %+v", payload)
	}
	if payload["artifact"] != "codencer_latest_darwin_arm64.tar.gz" {
		t.Fatalf("unexpected dry-run artifact: %+v", payload)
	}
	planned, _ := payload["planned_assets"].([]any)
	if len(planned) != 3 || planned[0] != "codencer_latest_darwin_arm64.tar.gz" || planned[1] != "checksums.txt" || planned[2] != "manifest.json" {
		t.Fatalf("unexpected dry-run planned assets: %+v", payload)
	}
}

func TestInstallScriptJSONInitFailureReportsPartial(t *testing.T) {
	repo := filepath.Join("..", "..")
	binDir := t.TempDir()
	for _, name := range []string{"codencer", "orchestratord", "codencer-relayd", "codencer-gatewayd", "codencer-connectord"} {
		body := "#!/bin/sh\n"
		if name == "codencer" {
			body += "if [ \"$1\" = \"init\" ]; then echo raw child init stderr >&2; exit 42; fi\n"
		}
		body += "echo " + name + "\n"
		if err := os.WriteFile(filepath.Join(binDir, name), []byte(body), 0755); err != nil {
			t.Fatal(err)
		}
	}
	installDir := t.TempDir()
	cmd := exec.Command("sh", filepath.Join(repo, "scripts", "install.sh"),
		"--bin-dir", binDir,
		"--install-dir", installDir,
		"--codencer-home", t.TempDir(),
		"--json",
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("install should fail when post-copy init fails: %s", out)
	}
	if strings.Contains(string(out), "raw child init stderr") {
		t.Fatalf("child init stderr leaked into JSON output: %s", out)
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("install failure output is not JSON: %v\n%s", err, out)
	}
	if payload["ok"] != false || payload["partial"] != true {
		t.Fatalf("expected ok=false partial=true for init failure after binary copy, got %+v", payload)
	}
	if got := payload["error"]; got != "codencer init failed after installing binaries" {
		t.Fatalf("unexpected init failure error: %+v", payload)
	}
	if _, err := os.Stat(filepath.Join(installDir, "codencer")); err != nil {
		t.Fatalf("codencer binary should have been copied before init failure: %v", err)
	}
}

func TestInstallScriptSourceSafety(t *testing.T) {
	repo := filepath.Join("..", "..")
	version := "v9.9.9"
	platform := "darwin_arm64"
	downloadDir, _, _ := createInstallReleaseFixture(t, repo, version, platform)
	installDir := t.TempDir()
	home := t.TempDir()
	outFile := filepath.Join(t.TempDir(), "install.json")
	cmdText := fmt.Sprintf(
		"source %s --version %s --repo lookmanrays/codencer --platform %s --download-dir %s --no-download --install-dir %s --codencer-home %s --json > %s; echo shell-still-alive",
		shellQuote(filepath.Join(repo, "scripts", "install.sh")),
		shellQuote(version),
		shellQuote(platform),
		shellQuote(downloadDir),
		shellQuote(installDir),
		shellQuote(home),
		shellQuote(outFile),
	)
	cmd := exec.Command("bash", "-lc", cmdText)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sourcing install.sh should not close child shell: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "shell-still-alive") {
		t.Fatalf("child shell did not continue after source: %s", out)
	}
}

func TestInstallScriptLatestFailureReturnsJSON(t *testing.T) {
	repo := filepath.Join("..", "..")
	script, err := os.ReadFile(filepath.Join(repo, "scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	fakeBin := t.TempDir()
	if err := os.WriteFile(filepath.Join(fakeBin, "curl"), []byte("#!/bin/sh\necho raw-curl-stderr >&2\nexit 22\n"), 0755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("sh", "-s", "--",
		"--repo", "lookmanrays/codencer",
		"--platform", "darwin_arm64",
		"--install-dir", t.TempDir(),
		"--codencer-home", t.TempDir(),
		"--json",
	)
	cmd.Stdin = strings.NewReader(string(script))
	cmd.Env = append(os.Environ(), "PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("latest release resolution should fail with fake curl: %s", out)
	}
	if strings.Contains(string(out), "raw-curl-stderr") {
		t.Fatalf("curl stderr leaked instead of installer JSON contract: %s", out)
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("install failure output is not JSON: %v\n%s", err, out)
	}
	if payload["ok"] != false {
		t.Fatalf("expected ok=false, got %+v", payload)
	}
	if got := payload["error"]; got != "failed to resolve latest release for lookmanrays/codencer" {
		t.Fatalf("unexpected latest failure error: %+v", payload)
	}
}

func TestInstallScriptUnsupportedDetectedPlatformReturnsJSON(t *testing.T) {
	repo := filepath.Join("..", "..")
	script, err := os.ReadFile(filepath.Join(repo, "scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	fakeBin := t.TempDir()
	uname := `#!/bin/sh
case "$1" in
  -s) printf Linux ;;
  -m) printf aarch64 ;;
  *) printf unknown ;;
esac
`
	if err := os.WriteFile(filepath.Join(fakeBin, "uname"), []byte(uname), 0755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-s", "--",
		"--repo", "lookmanrays/codencer",
		"--install-dir", t.TempDir(),
		"--codencer-home", t.TempDir(),
		"--json",
	)
	cmd.Stdin = strings.NewReader(string(script))
	cmd.Env = append(os.Environ(), "PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("unsupported detected platform should fail: %s", out)
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("install platform failure output is not JSON: %v\n%s", err, out)
	}
	if payload["ok"] != false {
		t.Fatalf("expected ok=false, got %+v", payload)
	}
	if got := payload["error"]; got != "unsupported Linux architecture: aarch64" {
		t.Fatalf("unexpected platform failure error: %+v", payload)
	}
}

func TestInstallScriptManifestVerificationFailureReturnsJSON(t *testing.T) {
	repo := filepath.Join("..", "..")
	version := "v9.9.9"
	platform := "darwin_arm64"
	downloadDir, _, _ := createInstallReleaseFixture(t, repo, version, platform)
	if err := os.WriteFile(filepath.Join(downloadDir, "manifest.json"), []byte(`{
  "version": "v9.9.9",
  "tag_name": "v9.9.9",
  "assets": [
    {"filename": "codencer_v9.9.9_darwin_arm64.tar.gz", "sha256": "wrong", "os": "darwin", "arch": "arm64"}
  ]
}
`), 0644); err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile(filepath.Join(repo, "scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-s", "--",
		"--version", version,
		"--repo", "lookmanrays/codencer",
		"--platform", platform,
		"--download-dir", downloadDir,
		"--no-download",
		"--install-dir", t.TempDir(),
		"--codencer-home", t.TempDir(),
		"--json",
	)
	cmd.Stdin = strings.NewReader(string(script))
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("manifest mismatch should fail: %s", out)
	}
	if strings.Contains(string(out), "manifest sha256 mismatch") {
		t.Fatalf("raw manifest verifier stderr leaked into JSON output: %s", out)
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("install manifest failure output is not JSON: %v\n%s", err, out)
	}
	if payload["ok"] != false {
		t.Fatalf("expected ok=false, got %+v", payload)
	}
	if got := payload["error"]; got != "manifest verification failed for codencer_v9.9.9_darwin_arm64.tar.gz" {
		t.Fatalf("unexpected manifest failure error: %+v", payload)
	}
}

func TestInstallScriptManifestVerificationRejectsMalformedJSON(t *testing.T) {
	repo := filepath.Join("..", "..")
	version := "v9.9.9"
	platform := "darwin_arm64"
	downloadDir, _, sha := createInstallReleaseFixture(t, repo, version, platform)
	artifactName := "codencer_" + version + "_" + platform + ".tar.gz"
	if err := os.WriteFile(filepath.Join(downloadDir, "manifest.json"), []byte(fmt.Sprintf(`{
  "version": %q,
  "tag_name": %q,
  "assets": [
    {"filename": %q, "sha256": %q, "os": "darwin", "arch": "arm64"}
`, version, version, artifactName, sha)), 0644); err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile(filepath.Join(repo, "scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-s", "--",
		"--version", version,
		"--repo", "lookmanrays/codencer",
		"--platform", platform,
		"--download-dir", downloadDir,
		"--no-download",
		"--install-dir", t.TempDir(),
		"--codencer-home", t.TempDir(),
		"--json",
	)
	cmd.Stdin = strings.NewReader(string(script))
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("malformed manifest JSON should fail: %s", out)
	}
	if strings.Contains(string(out), "manifest JSON is invalid") {
		t.Fatalf("raw manifest verifier stderr leaked into JSON output: %s", out)
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("install malformed manifest failure output is not JSON: %v\n%s", err, out)
	}
	if payload["ok"] != false {
		t.Fatalf("expected ok=false, got %+v", payload)
	}
	if got := payload["error"]; got != "manifest verification failed for codencer_v9.9.9_darwin_arm64.tar.gz" {
		t.Fatalf("unexpected malformed manifest error: %+v", payload)
	}
}

func TestInstallScriptManifestVerificationRejectsUnescapedControlCharacters(t *testing.T) {
	repo := filepath.Join("..", "..")
	version := "v9.9.9"
	platform := "darwin_arm64"
	for _, tc := range []struct {
		name    string
		control byte
	}{
		{name: "null", control: 0x00},
		{name: "start of heading", control: 0x01},
		{name: "form feed", control: 0x0c},
		{name: "unit separator", control: 0x1f},
	} {
		t.Run(tc.name, func(t *testing.T) {
			downloadDir, _, sha := createInstallReleaseFixture(t, repo, version, platform)
			artifactName := "codencer_" + version + "_" + platform + ".tar.gz"
			manifest := fmt.Sprintf(`{
  "version": %q,
  "tag_name": %q,
  "metadata": {"note": "beforeCONTROLafter"},
  "assets": [
    {"filename": %q, "sha256": %q, "os": "darwin", "arch": "arm64"}
  ]
}
`, version, version, artifactName, sha)
			manifest = strings.Replace(manifest, "CONTROL", string([]byte{tc.control}), 1)
			if err := os.WriteFile(filepath.Join(downloadDir, "manifest.json"), []byte(manifest), 0644); err != nil {
				t.Fatal(err)
			}
			script, err := os.ReadFile(filepath.Join(repo, "scripts", "install.sh"))
			if err != nil {
				t.Fatal(err)
			}
			installDir := t.TempDir()
			cmd := exec.Command("sh", "-s", "--",
				"--version", version,
				"--repo", "lookmanrays/codencer",
				"--platform", platform,
				"--download-dir", downloadDir,
				"--no-download",
				"--install-dir", installDir,
				"--codencer-home", t.TempDir(),
				"--json",
			)
			cmd.Stdin = strings.NewReader(string(script))
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("manifest with raw control 0x%02x should fail: %s", tc.control, out)
			}
			var payload map[string]any
			if err := json.Unmarshal(out, &payload); err != nil {
				t.Fatalf("install raw control failure output is not JSON: %v\n%s", err, out)
			}
			if payload["ok"] != false {
				t.Fatalf("expected ok=false, got %+v", payload)
			}
			if got := payload["error"]; got != "manifest verification failed for codencer_v9.9.9_darwin_arm64.tar.gz" {
				t.Fatalf("unexpected raw control manifest error: %+v", payload)
			}
			if _, err := os.Stat(filepath.Join(installDir, "codencer")); !os.IsNotExist(err) {
				t.Fatalf("raw control manifest installed a binary: %v", err)
			}
		})
	}
}

func TestInstallScriptManifestVerificationPreservesStringSpaces(t *testing.T) {
	repo := filepath.Join("..", "..")
	version := "v9.9.9"
	platform := "darwin_arm64"
	for _, tc := range []struct {
		name     string
		manifest func(artifactName, sha string) string
	}{
		{
			name: "version",
			manifest: func(artifactName, sha string) string {
				return fmt.Sprintf(`{
  "version": "v 9.9.9",
  "tag_name": "v 9.9.9",
  "assets": [
    {"filename": %q, "sha256": %q, "os": "darwin", "arch": "arm64"}
  ]
}
`, artifactName, sha)
			},
		},
		{
			name: "sha",
			manifest: func(artifactName, sha string) string {
				return fmt.Sprintf(`{
  "version": %q,
  "tag_name": %q,
  "assets": [
    {"filename": %q, "sha256": %q, "os": "darwin", "arch": "arm64"}
  ]
}
`, version, version, artifactName, sha[:8]+" "+sha[8:])
			},
		},
		{
			name: "os",
			manifest: func(artifactName, sha string) string {
				return fmt.Sprintf(`{
  "version": %q,
  "tag_name": %q,
  "assets": [
    {"filename": %q, "sha256": %q, "os": "dar win", "arch": "arm64"}
  ]
}
`, version, version, artifactName, sha)
			},
		},
		{
			name: "arch",
			manifest: func(artifactName, sha string) string {
				return fmt.Sprintf(`{
  "version": %q,
  "tag_name": %q,
  "assets": [
    {"filename": %q, "sha256": %q, "os": "darwin", "arch": "arm 64"}
  ]
}
`, version, version, artifactName, sha)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			downloadDir, _, sha := createInstallReleaseFixture(t, repo, version, platform)
			artifactName := "codencer_" + version + "_" + platform + ".tar.gz"
			if err := os.WriteFile(filepath.Join(downloadDir, "manifest.json"), []byte(tc.manifest(artifactName, sha)), 0644); err != nil {
				t.Fatal(err)
			}
			script, err := os.ReadFile(filepath.Join(repo, "scripts", "install.sh"))
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("sh", "-s", "--",
				"--version", version,
				"--repo", "lookmanrays/codencer",
				"--platform", platform,
				"--download-dir", downloadDir,
				"--no-download",
				"--install-dir", t.TempDir(),
				"--codencer-home", t.TempDir(),
				"--json",
			)
			cmd.Stdin = strings.NewReader(string(script))
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("manifest with spaced %s field should fail: %s", tc.name, out)
			}
			var payload map[string]any
			if err := json.Unmarshal(out, &payload); err != nil {
				t.Fatalf("install spaced %s manifest failure output is not JSON: %v\n%s", tc.name, err, out)
			}
			if payload["ok"] != false {
				t.Fatalf("expected ok=false, got %+v", payload)
			}
			if got := payload["error"]; got != "manifest verification failed for codencer_v9.9.9_darwin_arm64.tar.gz" {
				t.Fatalf("unexpected spaced %s manifest error: %+v", tc.name, payload)
			}
		})
	}
}

func TestInstallScriptManifestVerificationRejectsDuplicateKeys(t *testing.T) {
	repo := filepath.Join("..", "..")
	version := "v9.9.9"
	platform := "darwin_arm64"
	for _, tc := range []struct {
		name     string
		manifest func(artifactName, sha string) string
	}{
		{
			name: "version",
			manifest: func(artifactName, sha string) string {
				return fmt.Sprintf(`{
  "version": %q,
  "version": "v9.9.8",
  "tag_name": %q,
  "assets": [
    {"filename": %q, "sha256": %q, "os": "darwin", "arch": "arm64"}
  ]
}
`, version, version, artifactName, sha)
			},
		},
		{
			name: "tag_name",
			manifest: func(artifactName, sha string) string {
				return fmt.Sprintf(`{
  "version": %q,
  "tag_name": %q,
  "tag_name": "v9.9.8",
  "assets": [
    {"filename": %q, "sha256": %q, "os": "darwin", "arch": "arm64"}
  ]
}
`, version, version, artifactName, sha)
			},
		},
		{
			name: "filename",
			manifest: func(artifactName, sha string) string {
				return fmt.Sprintf(`{
  "version": %q,
  "tag_name": %q,
  "assets": [
    {"filename": %q, "filename": "different.tar.gz", "sha256": %q, "os": "darwin", "arch": "arm64"}
  ]
}
`, version, version, artifactName, sha)
			},
		},
		{
			name: "snapshot name",
			manifest: func(artifactName, sha string) string {
				return fmt.Sprintf(`{
  "version": %q,
  "artifacts": [
    {"name": %q, "name": "different.tar.gz", "sha256": %q, "os": "darwin", "arch": "arm64"}
  ]
}
`, version, artifactName, sha)
			},
		},
		{
			name: "sha256",
			manifest: func(artifactName, sha string) string {
				return fmt.Sprintf(`{
  "version": %q,
  "tag_name": %q,
  "assets": [
    {"filename": %q, "sha256": %q, "sha256": "wrong", "os": "darwin", "arch": "arm64"}
  ]
}
`, version, version, artifactName, sha)
			},
		},
		{
			name: "os",
			manifest: func(artifactName, sha string) string {
				return fmt.Sprintf(`{
  "version": %q,
  "tag_name": %q,
  "assets": [
    {"filename": %q, "sha256": %q, "os": "darwin", "os": "linux", "arch": "arm64"}
  ]
}
`, version, version, artifactName, sha)
			},
		},
		{
			name: "arch",
			manifest: func(artifactName, sha string) string {
				return fmt.Sprintf(`{
  "version": %q,
  "tag_name": %q,
  "assets": [
    {"filename": %q, "sha256": %q, "os": "darwin", "arch": "arm64", "arch": "amd64"}
  ]
}
`, version, version, artifactName, sha)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			downloadDir, _, sha := createInstallReleaseFixture(t, repo, version, platform)
			artifactName := "codencer_" + version + "_" + platform + ".tar.gz"
			if err := os.WriteFile(filepath.Join(downloadDir, "manifest.json"), []byte(tc.manifest(artifactName, sha)), 0644); err != nil {
				t.Fatal(err)
			}
			script, err := os.ReadFile(filepath.Join(repo, "scripts", "install.sh"))
			if err != nil {
				t.Fatal(err)
			}
			installDir := t.TempDir()
			cmd := exec.Command("sh", "-s", "--",
				"--version", version,
				"--repo", "lookmanrays/codencer",
				"--platform", platform,
				"--download-dir", downloadDir,
				"--no-download",
				"--install-dir", installDir,
				"--codencer-home", t.TempDir(),
				"--json",
			)
			cmd.Stdin = strings.NewReader(string(script))
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("manifest with duplicate %s should fail: %s", tc.name, out)
			}
			var payload map[string]any
			if err := json.Unmarshal(out, &payload); err != nil {
				t.Fatalf("install duplicate %s failure output is not JSON: %v\n%s", tc.name, err, out)
			}
			if payload["ok"] != false {
				t.Fatalf("expected ok=false, got %+v", payload)
			}
			if got := payload["error"]; got != "manifest verification failed for codencer_v9.9.9_darwin_arm64.tar.gz" {
				t.Fatalf("unexpected duplicate %s manifest error: %+v", tc.name, payload)
			}
			if _, err := os.Stat(filepath.Join(installDir, "codencer")); !os.IsNotExist(err) {
				t.Fatalf("duplicate %s manifest installed a binary: %v", tc.name, err)
			}
		})
	}
}

func TestInstallScriptManifestVerificationDoesNotNeedPython(t *testing.T) {
	repo := filepath.Join("..", "..")
	version := "v9.9.9"
	platform := "darwin_arm64"
	downloadDir, _, _ := createInstallReleaseFixture(t, repo, version, platform)
	script, err := os.ReadFile(filepath.Join(repo, "scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	fakeBin := t.TempDir()
	if err := os.WriteFile(filepath.Join(fakeBin, "python3"), []byte("#!/bin/sh\necho python must not be called >&2\nexit 99\n"), 0755); err != nil {
		t.Fatal(err)
	}
	installDir := t.TempDir()
	cmd := exec.Command("sh", "-s", "--",
		"--version", version,
		"--repo", "lookmanrays/codencer",
		"--platform", platform,
		"--download-dir", downloadDir,
		"--no-download",
		"--install-dir", installDir,
		"--codencer-home", t.TempDir(),
		"--json",
	)
	cmd.Stdin = strings.NewReader(string(script))
	cmd.Env = append(os.Environ(), "PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install should verify manifest without Python: %v\n%s", err, out)
	}
	if strings.Contains(string(out), "python must not be called") {
		t.Fatalf("manifest verification invoked Python: %s", out)
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("install output is not JSON: %v\n%s", err, out)
	}
	if payload["ok"] != true {
		t.Fatalf("expected ok=true, got %+v", payload)
	}
	if _, err := os.Stat(filepath.Join(installDir, "codencer")); err != nil {
		t.Fatalf("installed codencer missing from install dir: %v", err)
	}
}

func TestInstallScriptManifestVerificationRejectsStaleTag(t *testing.T) {
	repo := filepath.Join("..", "..")
	version := "v9.9.9"
	platform := "darwin_arm64"
	downloadDir, _, sha := createInstallReleaseFixture(t, repo, version, platform)
	artifactName := "codencer_" + version + "_" + platform + ".tar.gz"
	if err := os.WriteFile(filepath.Join(downloadDir, "manifest.json"), []byte(fmt.Sprintf(`{
  "version": "v9.9.8",
  "tag_name": "v9.9.8",
  "assets": [
    {"filename": %q, "sha256": %q, "os": "darwin", "arch": "arm64"}
  ]
}
`, artifactName, sha)), 0644); err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile(filepath.Join(repo, "scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-s", "--",
		"--version", version,
		"--repo", "lookmanrays/codencer",
		"--platform", platform,
		"--download-dir", downloadDir,
		"--no-download",
		"--install-dir", t.TempDir(),
		"--codencer-home", t.TempDir(),
		"--json",
	)
	cmd.Stdin = strings.NewReader(string(script))
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("stale manifest tag should fail: %s", out)
	}
	if strings.Contains(string(out), "manifest version mismatch") || strings.Contains(string(out), "manifest tag_name mismatch") {
		t.Fatalf("raw manifest verifier stderr leaked into JSON output: %s", out)
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("install stale manifest failure output is not JSON: %v\n%s", err, out)
	}
	if payload["ok"] != false {
		t.Fatalf("expected ok=false, got %+v", payload)
	}
	if got := payload["error"]; got != "manifest verification failed for codencer_v9.9.9_darwin_arm64.tar.gz" {
		t.Fatalf("unexpected stale manifest error: %+v", payload)
	}
}

func TestInstallScriptManifestVerificationUsesTopLevelTagFields(t *testing.T) {
	repo := filepath.Join("..", "..")
	version := "v9.9.9"
	platform := "darwin_arm64"
	downloadDir, _, sha := createInstallReleaseFixture(t, repo, version, platform)
	artifactName := "codencer_" + version + "_" + platform + ".tar.gz"
	if err := os.WriteFile(filepath.Join(downloadDir, "manifest.json"), []byte(fmt.Sprintf(`{
  "version": "v9.9.8",
  "tag_name": "v9.9.8",
  "meta": {"version": %q, "tag_name": %q},
  "assets": [
    {"filename": %q, "sha256": %q, "os": "darwin", "arch": "arm64"}
  ]
}
`, version, version, artifactName, sha)), 0644); err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile(filepath.Join(repo, "scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-s", "--",
		"--version", version,
		"--repo", "lookmanrays/codencer",
		"--platform", platform,
		"--download-dir", downloadDir,
		"--no-download",
		"--install-dir", t.TempDir(),
		"--codencer-home", t.TempDir(),
		"--json",
	)
	cmd.Stdin = strings.NewReader(string(script))
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("nested matching tags should not override stale top-level tags: %s", out)
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("install nested tag failure output is not JSON: %v\n%s", err, out)
	}
	if payload["ok"] != false {
		t.Fatalf("expected ok=false, got %+v", payload)
	}
	if got := payload["error"]; got != "manifest verification failed for codencer_v9.9.9_darwin_arm64.tar.gz" {
		t.Fatalf("unexpected nested tag manifest error: %+v", payload)
	}
}

func TestInstallScriptManifestVerificationAllowsSnapshotManifestWithoutTagName(t *testing.T) {
	repo := filepath.Join("..", "..")
	version := "v9.9.9"
	platform := "darwin_arm64"
	downloadDir, _, sha := createInstallReleaseFixture(t, repo, version, platform)
	artifactName := "codencer_" + version + "_" + platform + ".tar.gz"
	if err := os.WriteFile(filepath.Join(downloadDir, "manifest.json"), []byte(fmt.Sprintf(`{
  "version": %q,
  "commit": "fixture",
  "built_at": "2026-07-07T00:00:00Z",
  "targets": ["darwin/arm64"],
  "required_targets": ["darwin/arm64"],
  "allow_partial": false,
  "partial": false,
  "artifacts": [
    {"name": %q, "sha256": %q, "os": "darwin", "arch": "arm64", "status": "built", "required": true}
  ]
}
`, version, artifactName, sha)), 0644); err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile(filepath.Join(repo, "scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	installDir := t.TempDir()
	cmd := exec.Command("sh", "-s", "--",
		"--version", version,
		"--repo", "lookmanrays/codencer",
		"--platform", platform,
		"--download-dir", downloadDir,
		"--no-download",
		"--install-dir", installDir,
		"--codencer-home", t.TempDir(),
		"--json",
	)
	cmd.Stdin = strings.NewReader(string(script))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("snapshot manifest without tag_name should install: %v\n%s", err, out)
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("install snapshot manifest output is not JSON: %v\n%s", err, out)
	}
	if payload["ok"] != true {
		t.Fatalf("expected ok=true, got %+v", payload)
	}
	if _, err := os.Stat(filepath.Join(installDir, "codencer")); err != nil {
		t.Fatalf("installed codencer missing from install dir: %v", err)
	}
}

func TestInstallScriptManifestVerificationRejectsSubstringArtifact(t *testing.T) {
	repo := filepath.Join("..", "..")
	version := "v9.9.9"
	platform := "darwin_arm64"
	downloadDir, _, sha := createInstallReleaseFixture(t, repo, version, platform)
	artifactName := "codencer_" + version + "_" + platform + ".tar.gz"
	if err := os.WriteFile(filepath.Join(downloadDir, "manifest.json"), []byte(fmt.Sprintf(`{
  "version": %q,
  "tag_name": %q,
  "assets": [
    {"filename": %q, "sha256": %q, "os": "darwin", "arch": "arm64"}
  ]
}
`, version, version, artifactName+".sig", sha)), 0644); err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile(filepath.Join(repo, "scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-s", "--",
		"--version", version,
		"--repo", "lookmanrays/codencer",
		"--platform", platform,
		"--download-dir", downloadDir,
		"--no-download",
		"--install-dir", t.TempDir(),
		"--codencer-home", t.TempDir(),
		"--json",
	)
	cmd.Stdin = strings.NewReader(string(script))
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("substring manifest artifact should fail: %s", out)
	}
	if strings.Contains(string(out), "must reference") {
		t.Fatalf("raw manifest verifier stderr leaked into JSON output: %s", out)
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("install substring manifest failure output is not JSON: %v\n%s", err, out)
	}
	if payload["ok"] != false {
		t.Fatalf("expected ok=false, got %+v", payload)
	}
	if got := payload["error"]; got != "manifest verification failed for codencer_v9.9.9_darwin_arm64.tar.gz" {
		t.Fatalf("unexpected substring manifest error: %+v", payload)
	}
}

func TestInstallScriptManifestVerificationRequiresAssetEntry(t *testing.T) {
	repo := filepath.Join("..", "..")
	version := "v9.9.9"
	platform := "darwin_arm64"
	downloadDir, _, sha := createInstallReleaseFixture(t, repo, version, platform)
	artifactName := "codencer_" + version + "_" + platform + ".tar.gz"
	if err := os.WriteFile(filepath.Join(downloadDir, "manifest.json"), []byte(fmt.Sprintf(`{
  "version": %q,
  "tag_name": %q,
  "metadata": {
    "filename": %q,
    "sha256": %q,
    "os": "darwin",
    "arch": "arm64"
  },
  "assets": []
}
`, version, version, artifactName, sha)), 0644); err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile(filepath.Join(repo, "scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-s", "--",
		"--version", version,
		"--repo", "lookmanrays/codencer",
		"--platform", platform,
		"--download-dir", downloadDir,
		"--no-download",
		"--install-dir", t.TempDir(),
		"--codencer-home", t.TempDir(),
		"--json",
	)
	cmd.Stdin = strings.NewReader(string(script))
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("manifest metadata outside assets should not verify artifact: %s", out)
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("install metadata artifact failure output is not JSON: %v\n%s", err, out)
	}
	if payload["ok"] != false {
		t.Fatalf("expected ok=false, got %+v", payload)
	}
	if got := payload["error"]; got != "manifest verification failed for codencer_v9.9.9_darwin_arm64.tar.gz" {
		t.Fatalf("unexpected metadata artifact manifest error: %+v", payload)
	}
}

func TestInstallScriptManifestVerificationIgnoresNestedAssetMetadata(t *testing.T) {
	repo := filepath.Join("..", "..")
	version := "v9.9.9"
	platform := "darwin_arm64"
	downloadDir, _, sha := createInstallReleaseFixture(t, repo, version, platform)
	artifactName := "codencer_" + version + "_" + platform + ".tar.gz"
	if err := os.WriteFile(filepath.Join(downloadDir, "manifest.json"), []byte(fmt.Sprintf(`{
  "version": %q,
  "tag_name": %q,
  "assets": [
    {
      "metadata": {
        "filename": %q,
        "sha256": %q,
        "os": "darwin",
        "arch": "arm64"
      }
    }
  ]
}
`, version, version, artifactName, sha)), 0644); err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile(filepath.Join(repo, "scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-s", "--",
		"--version", version,
		"--repo", "lookmanrays/codencer",
		"--platform", platform,
		"--download-dir", downloadDir,
		"--no-download",
		"--install-dir", t.TempDir(),
		"--codencer-home", t.TempDir(),
		"--json",
	)
	cmd.Stdin = strings.NewReader(string(script))
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("nested asset metadata should not verify artifact: %s", out)
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("install nested asset metadata failure output is not JSON: %v\n%s", err, out)
	}
	if payload["ok"] != false {
		t.Fatalf("expected ok=false, got %+v", payload)
	}
	if got := payload["error"]; got != "manifest verification failed for codencer_v9.9.9_darwin_arm64.tar.gz" {
		t.Fatalf("unexpected nested asset metadata manifest error: %+v", payload)
	}
}

func TestInstallScriptManifestVerificationRejectsNestedAssetArrays(t *testing.T) {
	repo := filepath.Join("..", "..")
	version := "v9.9.9"
	platform := "darwin_arm64"
	downloadDir, _, sha := createInstallReleaseFixture(t, repo, version, platform)
	artifactName := "codencer_" + version + "_" + platform + ".tar.gz"
	if err := os.WriteFile(filepath.Join(downloadDir, "manifest.json"), []byte(fmt.Sprintf(`{
  "version": %q,
  "tag_name": %q,
  "assets": [[
    {"filename": %q, "sha256": %q, "os": "darwin", "arch": "arm64"}
  ]]
}
`, version, version, artifactName, sha)), 0644); err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile(filepath.Join(repo, "scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	installDir := t.TempDir()
	cmd := exec.Command("sh", "-s", "--",
		"--version", version,
		"--repo", "lookmanrays/codencer",
		"--platform", platform,
		"--download-dir", downloadDir,
		"--no-download",
		"--install-dir", installDir,
		"--codencer-home", t.TempDir(),
		"--json",
	)
	cmd.Stdin = strings.NewReader(string(script))
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("manifest nested asset array should fail: %s", out)
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("install nested asset array failure output is not JSON: %v\n%s", err, out)
	}
	if payload["ok"] != false {
		t.Fatalf("expected ok=false, got %+v", payload)
	}
	if got := payload["error"]; got != "manifest verification failed for codencer_v9.9.9_darwin_arm64.tar.gz" {
		t.Fatalf("unexpected nested asset array manifest error: %+v", payload)
	}
	if _, err := os.Stat(filepath.Join(installDir, "codencer")); !os.IsNotExist(err) {
		t.Fatalf("nested asset array manifest installed a binary: %v", err)
	}
}

func TestInstallScriptDownloadFailureReturnsCleanJSON(t *testing.T) {
	repo := filepath.Join("..", "..")
	script, err := os.ReadFile(filepath.Join(repo, "scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	fakeBin := t.TempDir()
	if err := os.WriteFile(filepath.Join(fakeBin, "curl"), []byte("#!/bin/sh\necho raw curl download stderr >&2\nexit 22\n"), 0755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-s", "--",
		"--version", "v9.9.9",
		"--repo", "lookmanrays/codencer",
		"--platform", "darwin_arm64",
		"--install-dir", t.TempDir(),
		"--codencer-home", t.TempDir(),
		"--json",
	)
	cmd.Stdin = strings.NewReader(string(script))
	cmd.Env = append(os.Environ(), "PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("download failure should fail: %s", out)
	}
	if strings.Contains(string(out), "raw curl download stderr") {
		t.Fatalf("curl stderr leaked into JSON output: %s", out)
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("install download failure output is not JSON: %v\n%s", err, out)
	}
	if payload["ok"] != false {
		t.Fatalf("expected ok=false, got %+v", payload)
	}
	if got, _ := payload["error"].(string); !strings.Contains(got, "download failed: https://github.com/lookmanrays/codencer/releases/download/v9.9.9/codencer_v9.9.9_darwin_arm64.tar.gz") {
		t.Fatalf("unexpected download failure error: %+v", payload)
	}
}

func TestInstallScriptMalformedArchiveReturnsJSON(t *testing.T) {
	repo := filepath.Join("..", "..")
	version := "v9.9.9"
	platform := "darwin_arm64"
	downloadDir := t.TempDir()
	emptyDir := t.TempDir()
	artifactName := "codencer_" + version + "_" + platform + ".tar.gz"
	artifactPath := filepath.Join(downloadDir, artifactName)
	if err := writeTarGz(emptyDir, artifactPath); err != nil {
		t.Fatal(err)
	}
	sha := sha256Path(t, artifactPath)
	if err := os.WriteFile(filepath.Join(downloadDir, "checksums.txt"), []byte(sha+"  "+artifactName+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	manifest := fmt.Sprintf(`{
  "version": %q,
  "tag_name": %q,
  "assets": [
    {"filename": %q, "sha256": %q, "os": "darwin", "arch": "arm64"}
  ]
}
`, version, version, artifactName, sha)
	if err := os.WriteFile(filepath.Join(downloadDir, "manifest.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile(filepath.Join(repo, "scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-s", "--",
		"--version", version,
		"--repo", "lookmanrays/codencer",
		"--platform", platform,
		"--download-dir", downloadDir,
		"--no-download",
		"--install-dir", t.TempDir(),
		"--codencer-home", t.TempDir(),
		"--json",
	)
	cmd.Stdin = strings.NewReader(string(script))
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("malformed archive should fail: %s", out)
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("install malformed archive output is not JSON: %v\n%s", err, out)
	}
	if payload["ok"] != false {
		t.Fatalf("expected ok=false, got %+v", payload)
	}
	if got := payload["error"]; got != "expected exactly one unpacked Codencer bin directory, got 0" {
		t.Fatalf("unexpected malformed archive error: %+v", payload)
	}
}

func TestReleaseAssetsWorkflowPublishesInstallerPlatforms(t *testing.T) {
	repo := filepath.Join("..", "..")
	installScript, err := os.ReadFile(filepath.Join(repo, "scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	workflow, err := os.ReadFile(filepath.Join(repo, ".github", "workflows", "release-assets.yml"))
	if err != nil {
		t.Fatal(err)
	}
	installText := string(installScript)
	workflowText := string(workflow)
	for _, want := range []string{
		"darwin_arm64",
		"darwin_amd64",
	} {
		if !strings.Contains(installText, want) {
			t.Fatalf("installer no longer selects %s", want)
		}
		if !strings.Contains(workflowText, "codencer_${TAG_NAME}_"+want+".tar.gz") {
			t.Fatalf("release-assets workflow does not publish installer platform %s", want)
		}
	}
	if strings.Contains(strings.ToLower(workflowText), "expected exactly one darwin") {
		t.Fatal("release-assets workflow still enforces a single Darwin host artifact")
	}
	scanner := bufio.NewScanner(strings.NewReader(workflowText))
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == "run: make verify-release" {
			t.Fatal("release-assets workflow must not run make verify-release after building tag artifacts")
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(workflowText, "Verify macOS release metadata") ||
		!strings.Contains(workflowText, "checksum mismatch for {name}") ||
		!strings.Contains(workflowText, "manifest platform mismatch for {name}") {
		t.Fatal("release-assets workflow no longer verifies built macOS tag artifact metadata")
	}
}

func TestReleaseArtifactSelfHostVerifierContract(t *testing.T) {
	repo := filepath.Join("..", "..")
	scriptPath := filepath.Join(repo, "scripts", "verify_release_artifact_selfhost.sh")
	info, err := os.Stat(scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0111 == 0 {
		t.Fatalf("expected %s to be executable, got %v", scriptPath, info.Mode())
	}
	script, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"release-snapshot",
		"manifest.json",
		"checksums.txt",
		"CODENCER_BIN_DIR=\"$bin_dir\"",
		"CODENCER_MANIFEST_FILE=\"$manifest_file\"",
		"codencer-gatewayd",
		"codencer-connectord",
		"private key block",
		"generated report/screenshot artifact",
	} {
		if !strings.Contains(string(script), want) {
			t.Fatalf("release artifact verifier missing contract marker %q", want)
		}
	}
	makefile, err := os.ReadFile(filepath.Join(repo, "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(makefile), ".PHONY: verify-release-artifact-selfhost") ||
		!strings.Contains(string(makefile), "verify-release-artifact-selfhost VERSION=v0.3.0-selfhost-artifact-verify") {
		t.Fatal("Makefile does not expose verify-release-artifact-selfhost in the public self-host gate")
	}
}

func createInstallReleaseFixture(t *testing.T, repo, version, platform string) (downloadDir, packageDir, sha string) {
	t.Helper()
	packageName := "codencer_" + version + "_" + platform
	packageDir = filepath.Join(t.TempDir(), packageName)
	binDir := filepath.Join(packageDir, "bin")
	scriptsDir := filepath.Join(packageDir, "scripts")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"codencer", "orchestratord", "codencer-relayd", "codencer-gatewayd", "codencer-connectord"} {
		body := "#!/bin/sh\nif [ \"$1\" = \"init\" ]; then exit 0; fi\necho " + name + "\n"
		if err := os.WriteFile(filepath.Join(binDir, name), []byte(body), 0755); err != nil {
			t.Fatal(err)
		}
	}
	installScript, err := os.ReadFile(filepath.Join(repo, "scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scriptsDir, "install.sh"), installScript, 0755); err != nil {
		t.Fatal(err)
	}

	downloadDir = t.TempDir()
	artifactName := packageName + ".tar.gz"
	artifactPath := filepath.Join(downloadDir, artifactName)
	if err := writeTarGz(packageDir, artifactPath); err != nil {
		t.Fatal(err)
	}
	sha = sha256Path(t, artifactPath)
	if err := os.WriteFile(filepath.Join(downloadDir, "checksums.txt"), []byte(sha+"  "+artifactName+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	parts := strings.SplitN(platform, "_", 2)
	manifest := fmt.Sprintf(`{
  "version": %q,
  "tag_name": %q,
  "release_sha": "fixture",
  "built_at": "2026-07-07T00:00:00Z",
  "assets": [
    {"filename": %q, "sha256": %q, "os": %q, "arch": %q, "runner": "fixture"}
  ]
}
`, version, version, artifactName, sha, parts[0], parts[1])
	if err := os.WriteFile(filepath.Join(downloadDir, "manifest.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	return downloadDir, packageDir, sha
}

func writeTarGz(srcDir, dst string) error {
	file, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer file.Close()
	gz := gzip.NewWriter(file)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()
	base := filepath.Dir(srcDir)
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		header.ModTime = time.Unix(1700000000, 0).UTC()
		header.AccessTime = header.ModTime
		header.ChangeTime = header.ModTime
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		_, err = io.Copy(tw, in)
		return err
	})
}

func sha256Path(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:])
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func TestPublicSelfHostRCVerifierRejectsRealExecutorSimulation(t *testing.T) {
	repo := filepath.Join("..", "..")
	scriptPath := filepath.Join(repo, "scripts", "verify_public_selfhost_rc.sh")
	script, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	body := string(script)
	for _, want := range []string{
		"reject_real_executor_simulation_env",
		"ALL_ADAPTERS_SIMULATION_MODE=0",
		"CODEX_SIMULATION_MODE=0",
		"CODEX_BINARY=\"$real_command\"",
		"${gate_name}_env.log",
		"required_real_executor_proofs",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("public self-host RC verifier missing real executor simulation guard marker %q", want)
		}
	}
	if strings.Contains(body, "PARTIAL for Public Self-host RC") || strings.Contains(body, `write_summary "PARTIAL"`) {
		t.Fatal("public self-host RC verifier must not emit PARTIAL under the public release gate")
	}
	verifier, err := os.ReadFile(filepath.Join(repo, "web", "gateway-console", "tests", "live", "verify-live.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	verifierBody := string(verifier)
	for _, want := range []string{
		"assertRealExecutorReport",
		"assertNoSimulationText",
		"Simulation Mode",
		"Executing Simulated [A-Za-z0-9_-]+",
		"Simulated successful [A-Za-z0-9_-]+ task",
		"is_simulation",
	} {
		if !strings.Contains(verifierBody, want) {
			t.Fatalf("Gateway Console live verifier missing real executor simulation assertion marker %q", want)
		}
	}
}
