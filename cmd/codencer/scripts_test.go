package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
