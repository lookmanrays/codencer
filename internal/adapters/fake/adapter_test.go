package fake

import (
	"context"
	"path/filepath"
	"testing"

	"agent-bridge/internal/domain"
)

func TestFakeAdapterSuccessFailureBlocker(t *testing.T) {
	cases := []struct {
		id    string
		state domain.StepState
	}{
		{Success, domain.StepStateCompleted},
		{Failure, domain.StepStateFailedTerminal},
		{Blocker, domain.StepStateNeedsManualAttention},
	}

	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			adapter := New(tc.id)
			root := t.TempDir()
			attempt := &domain.Attempt{ID: "attempt-1", Adapter: tc.id}
			step := &domain.Step{
				ID:      "step-1",
				PhaseID: "phase-1",
				Adapter: tc.id,
				TaskSpecSnapshot: &domain.TaskSpec{
					RunID: "run-1",
				},
			}

			if err := adapter.Start(context.Background(), step, attempt, root, filepath.Join(root, "artifacts")); err != nil {
				t.Fatalf("Start failed: %v", err)
			}
			running, err := adapter.Poll(context.Background(), attempt.ID)
			if err != nil {
				t.Fatalf("Poll failed: %v", err)
			}
			if running {
				t.Fatal("expected fake adapter to finish immediately")
			}
			artifacts, err := adapter.CollectArtifacts(context.Background(), attempt.ID, filepath.Join(root, "artifacts"))
			if err != nil {
				t.Fatalf("CollectArtifacts failed: %v", err)
			}
			result, err := adapter.NormalizeResult(context.Background(), attempt.ID, artifacts)
			if err != nil {
				t.Fatalf("NormalizeResult failed: %v", err)
			}
			if result.State != tc.state {
				t.Fatalf("state = %s, want %s", result.State, tc.state)
			}
			if tc.id == Blocker && !result.NeedsHumanDecision {
				t.Fatal("expected blocker to require a human decision")
			}
		})
	}
}

func TestFakeAdapterTimeoutRunsUntilCancelled(t *testing.T) {
	adapter := New(Timeout)
	attempt := &domain.Attempt{ID: "attempt-timeout", Adapter: Timeout}
	step := &domain.Step{ID: "step-timeout", PhaseID: "phase-1", Adapter: Timeout}
	root := t.TempDir()

	if err := adapter.Start(context.Background(), step, attempt, root, filepath.Join(root, "artifacts")); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	running, err := adapter.Poll(context.Background(), attempt.ID)
	if err != nil {
		t.Fatalf("Poll failed: %v", err)
	}
	if !running {
		t.Fatal("expected timeout fake adapter to remain running")
	}
	if err := adapter.Cancel(context.Background(), attempt.ID); err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}
	running, err = adapter.Poll(context.Background(), attempt.ID)
	if err != nil {
		t.Fatalf("Poll after cancel failed: %v", err)
	}
	if running {
		t.Fatal("expected timeout fake adapter to stop after cancel")
	}
}
