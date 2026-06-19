package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
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
