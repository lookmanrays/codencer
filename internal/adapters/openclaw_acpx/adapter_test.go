package openclaw_acpx

import (
	"context"
	"os"
	"testing"

	"agent-bridge/internal/domain"
)

func TestAdapter_Metadata(t *testing.T) {
	a := NewAdapter()
	if a.Name() != "openclaw-acpx" {
		t.Errorf("expected name openclaw-acpx, got %s", a.Name())
	}

	caps := a.Capabilities()
	found := false
	for _, c := range caps {
		if c == "acp_compliance" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected capability acp_compliance not found")
	}
}

func TestAdapter_SimulationMode(t *testing.T) {
	// 1. Enable simulation mode via env
	os.Setenv("OPENCLAW-ACPX_SIMULATION_MODE", "1")
	defer os.Unsetenv("OPENCLAW-ACPX_SIMULATION_MODE")

	a := NewAdapter()
	step := &domain.Step{Goal: "Test simulation"}
	attempt := &domain.Attempt{ID: "test-att-1", Adapter: a.Name()}
	
	tmpDir, err := os.MkdirTemp("", "openclaw-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	artifactRoot := tmpDir
	workspace := tmpDir

	// 2. Start should return nil (success) in simulation mode even without acpx binary
	err = a.Start(context.Background(), step, attempt, workspace, artifactRoot)
	if err != nil {
		t.Errorf("expected no error in simulation mode, got %v", err)
	}
}
