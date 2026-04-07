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
	
	tmpDir, err := os.MkdirTemp("", "openclaw-normalize-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("Standard result.json", func(t *testing.T) {
		resultPath := tmpDir + "/result.json"
		resultContent := `{"status":"completed","summary":"Task finished successfully"}`
		if err := os.WriteFile(resultPath, []byte(resultContent), 0644); err != nil {
			t.Fatal(err)
		}

		artifacts := []*domain.Artifact{
			{Name: "result.json", Path: resultPath},
		}

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
	})

	t.Run("ACP-specific acp-status.json with error", func(t *testing.T) {
		resultPath := tmpDir + "/acp-status.json"
		resultContent := `{"status":"failed","summary":"Agent hit a wall","error":"Rate limit exceeded"}`
		if err := os.WriteFile(resultPath, []byte(resultContent), 0644); err != nil {
			t.Fatal(err)
		}

		artifacts := []*domain.Artifact{
			{Name: "acp-status.json", Path: resultPath},
		}

		res, err := a.NormalizeResult(context.Background(), attemptID, artifacts)
		if err != nil {
			t.Fatalf("NormalizeResult failed: %v", err)
		}

		if res.State != domain.StepStateFailedTerminal {
			t.Errorf("expected state failed_terminal, got %s", res.State)
		}
		if res.Summary != "Agent hit a wall (Error: Rate limit exceeded)" {
			t.Errorf("expected detailed summary, got %s", res.Summary)
		}
	})

	t.Run("No structured result fallback", func(t *testing.T) {
		artifacts := []*domain.Artifact{
			{Name: "stdout.log", Path: "/tmp/stdout.log", Type: domain.ArtifactTypeStdout},
		}

		res, err := a.NormalizeResult(context.Background(), attemptID, artifacts)
		if err != nil {
			t.Fatalf("NormalizeResult failed: %v", err)
		}

		if res.State != domain.StepStateCompleted {
			t.Errorf("expected default success, got %s", res.State)
		}
		if res.RawOutputRef != "/tmp/stdout.log" {
			t.Error("expected RawOutputRef to be linked")
		}
	})

	t.Run("Cancelled and Timeout states", func(t *testing.T) {
		states := map[string]domain.StepState{
			"cancelled": domain.StepStateCancelled,
			"stopped":   domain.StepStateCancelled,
			"timeout":   domain.StepStateTimeout,
			"timed_out": domain.StepStateTimeout,
		}

		for acpStatus, expectedState := range states {
			resultPath := tmpDir + "/status-" + acpStatus + ".json"
			resultContent := `{"status":"` + acpStatus + `","summary":"Task ended"}`
			if err := os.WriteFile(resultPath, []byte(resultContent), 0644); err != nil {
				t.Fatal(err)
			}

			artifacts := []*domain.Artifact{
				{Name: "acp-status.json", Path: resultPath},
			}

			res, err := a.NormalizeResult(context.Background(), attemptID, artifacts)
			if err != nil {
				t.Fatalf("NormalizeResult failed for %s: %v", acpStatus, err)
			}

			if res.State != expectedState {
				t.Errorf("status %s: expected state %s, got %s", acpStatus, expectedState, res.State)
			}
		}
	})
}
