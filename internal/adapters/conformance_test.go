package adapters_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"agent-bridge/internal/adapters/claude"
	"agent-bridge/internal/adapters/codex"
	"agent-bridge/internal/adapters/qwen"
	"agent-bridge/internal/domain"
)

func TestAdapters_SimulationConformance(t *testing.T) {
	// Force simulation mode for all tests
	os.Setenv("CODEX_SIMULATION_MODE", "1")
	os.Setenv("CLAUDE_SIMULATION_MODE", "1")
	os.Setenv("QWEN_SIMULATION_MODE", "1")
	defer func() {
		os.Unsetenv("CODEX_SIMULATION_MODE")
		os.Unsetenv("CLAUDE_SIMULATION_MODE")
		os.Unsetenv("QWEN_SIMULATION_MODE")
	}()

	conformanceAdapters := []domain.Adapter{
		codex.NewAdapter(),
		claude.NewAdapter(),
		qwen.NewAdapter(),
	}

	for _, a := range conformanceAdapters {
		t.Run("Conformance_"+a.Name(), func(t *testing.T) {
			ctx := context.Background()
			attempt := &domain.Attempt{
				ID:        "test-attempt-" + a.Name(),
				StepID:    "test-step",
				Number:    1,
				Adapter:   a.Name(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			workspaceRoot := filepath.Join(t.TempDir(), "workspace")
			artifactRoot := filepath.Join(t.TempDir(), "artifacts")
			os.MkdirAll(workspaceRoot, 0755)
			os.MkdirAll(artifactRoot, 0755)

			// Start execution
			if err := a.Start(ctx, attempt, workspaceRoot, artifactRoot); err != nil {
				t.Fatalf("Start failed for %s: %v", a.Name(), err)
			}

			// Block-poll locally since simulation triggers almost instantly
			timeout := time.After(5 * time.Second)
			for {
				select {
				case <-timeout:
					t.Fatalf("timeout waiting for %s simulation to finish", a.Name())
				default:
					running, err := a.Poll(ctx, attempt.ID)
					if err != nil {
						t.Fatalf("Poll failed for %s: %v", a.Name(), err)
					}
					if !running {
						goto Collection
					}
					time.Sleep(100 * time.Millisecond)
				}
			}

Collection:
			// Ensure artifacts are emitted to the correct bound
			artifacts, err := a.CollectArtifacts(ctx, attempt.ID, artifactRoot)
			if err != nil {
				t.Fatalf("CollectArtifacts failed for %s: %v", a.Name(), err)
			}

			foundResultJson := false
			for _, art := range artifacts {
				if art.Type == "result_json" {
					foundResultJson = true
				}
			}
			if !foundResultJson {
				t.Fatalf("Adapter %s did not emit a result_json artifact during simulation", a.Name())
			}

			// Validate payload layout conformance
			res, err := a.NormalizeResult(ctx, attempt.ID, artifacts)
			if res.State == "" {
				t.Error("NormalizeResult returned empty State")
			}
			if err != nil {
				t.Fatalf("NormalizeResult failed for %s: %v", a.Name(), err)
			}

			// Expected simulated structural layout tests
			if res.State != domain.StepStateCompleted {
				t.Errorf("Expected StepStateCompleted for simulated %s, got %s", a.Name(), res.State)
			}

			// Conformance tests ensure memory capabilities are populated natively mapped correctly
			caps := a.Capabilities()
			if len(caps) == 0 {
				t.Errorf("Expected non-empty capabilities for %s", a.Name())
			}
		})
	}
}
