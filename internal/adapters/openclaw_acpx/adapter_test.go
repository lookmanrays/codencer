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

func TestAdapter_Lifecycle(t *testing.T) {
	// 1. Enable simulation mode
	os.Setenv("OPENCLAW-ACPX_SIMULATION_MODE", "1")
	defer os.Unsetenv("OPENCLAW-ACPX_SIMULATION_MODE")

	a := NewAdapter()
	step := &domain.Step{ID: "step-1", Goal: "Test lifecycle"}
	attempt := &domain.Attempt{ID: "att-1", Adapter: a.Name()}
	
	tmpDir, err := os.MkdirTemp("", "openclaw-lifecycle-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// 2. Start (Initiates background process)
	err = a.Start(context.Background(), step, attempt, tmpDir, tmpDir)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// 3. Poll - Should be true immediately after start (background goroutine running)
	running, err := a.Poll(context.Background(), attempt.ID)
	if err != nil {
		t.Errorf("Poll failed: %v", err)
	}
	if !running {
		t.Error("expected adapter to be running after Start")
	}

	// 4. Cancel
	err = a.Cancel(context.Background(), attempt.ID)
	if err != nil {
		t.Errorf("Cancel failed: %v", err)
	}

	// 5. Poll - Should be false after Cancel
	running, err = a.Poll(context.Background(), attempt.ID)
	if err != nil {
		t.Errorf("Poll after cancel failed: %v", err)
	}
	if running {
		t.Error("expected adapter to stop running after Cancel")
	}
}

func TestAdapter_NormalizeResult(t *testing.T) {
	a := NewAdapter()
	attemptID := "att-normalize"
	
	// 1. Create a dummy result.json
	tmpDir, err := os.MkdirTemp("", "openclaw-normalize-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	resultPath := tmpDir + "/result.json"
	resultContent := `{"state":"completed","summary":"Task finished successfully"}`
	if err := os.WriteFile(resultPath, []byte(resultContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 2. Mock artifacts
	artifacts := []*domain.Artifact{
		{
			ID:   "art-1",
			Type: domain.ArtifactTypeResultJSON,
			Path: resultPath,
		},
	}

	// 3. Normalize
	res, err := a.NormalizeResult(context.Background(), attemptID, artifacts)
	if err != nil {
		t.Fatalf("NormalizeResult failed: %v", err)
	}

	if res.State != domain.StepStateCompleted {
		t.Errorf("expected state completed, got %s", res.State)
	}
	if res.Summary != "Task finished successfully" {
		t.Errorf("expected summary 'Task finished successfully', got %s", res.Summary)
	}
}
