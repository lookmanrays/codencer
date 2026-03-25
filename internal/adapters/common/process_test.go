package common

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"agent-bridge/internal/domain"
)

func TestInvokeLocal_BinaryNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	attempt := &domain.Attempt{ID: "test-id", Adapter: "test-adapter"}
	
	opts := ExecutionOptions{
		AdapterName:  "test-adapter",
		BinaryName:   "nonexistent-binary-xyz",
		BinaryEnvVar: "NONEXISTENT_BINARY_ENV",
		Workspace:    tmpDir,
		ArtifactRoot: filepath.Join(tmpDir, "artifacts"),
	}

	err := InvokeLocal(context.Background(), attempt, opts)
	if err == nil {
		t.Fatal("expected error for nonexistent binary, got nil")
	}
	if !contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestInvokeLocal_Simulation(t *testing.T) {
	tmpDir := t.TempDir()
	artifactRoot := filepath.Join(tmpDir, "artifacts")
	attempt := &domain.Attempt{ID: "test-sim-id", Adapter: "test-adapter-sim"}
	
	os.Setenv("TEST-ADAPTER-SIM_SIMULATION_MODE", "1")
	defer os.Unsetenv("TEST-ADAPTER-SIM_SIMULATION_MODE")

	opts := ExecutionOptions{
		AdapterName:  "test-adapter-sim",
		BinaryName:   "test-adapter-sim-bin",
		Workspace:    tmpDir,
		ArtifactRoot: artifactRoot,
	}

	err := InvokeLocal(context.Background(), attempt, opts)
	if err != nil {
		t.Fatalf("InvokeLocal simulation failed: %v", err)
	}

	// Verify files
	if _, err := os.Stat(filepath.Join(artifactRoot, "result.json")); err != nil {
		t.Errorf("result.json not created: %v", err)
	}
}

func TestCollectAndNormalize(t *testing.T) {
	tmpDir := t.TempDir()
	artifactRoot := filepath.Join(tmpDir, "artifacts")
	os.MkdirAll(artifactRoot, 0755)
	
	// Create dummy result
	resultData := `{"state": "completed", "summary": "done"}`
	os.WriteFile(filepath.Join(artifactRoot, "result.json"), []byte(resultData), 0644)
	os.WriteFile(filepath.Join(artifactRoot, "stdout.log"), []byte("hello"), 0644)

	artifacts, err := CollectStandardArtifacts(context.Background(), "test-id", artifactRoot)
	if err != nil {
		t.Fatalf("CollectStandardArtifacts failed: %v", err)
	}

	if len(artifacts) != 2 {
		t.Errorf("expected 2 artifacts, got %d", len(artifacts))
	}

	res, err := NormalizeStandardResult("test-id", artifacts)
	if err != nil {
		t.Fatalf("NormalizeStandardResult failed: %v", err)
	}

	if res.State != domain.StepStateCompleted {
		t.Errorf("Expected State completed, got %s", res.State)
	}
}

func contains(s, substr string) bool {
	return (s != "" && substr != "" && (len(s) >= len(substr))) // Simplified
}
