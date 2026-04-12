package service

import (
	"testing"

	"agent-bridge/internal/domain"
)

func TestDeriveRunState(t *testing.T) {
	tests := []struct {
		name     string
		steps    []*domain.Step
		expected domain.RunState
	}{
		{
			name: "completed when all steps succeeded",
			steps: []*domain.Step{
				{State: domain.StepStateCompleted},
				{State: domain.StepStateCompletedWithWarnings},
			},
			expected: domain.RunStateCompleted,
		},
		{
			name: "failed when any step failed",
			steps: []*domain.Step{
				{State: domain.StepStateCompleted},
				{State: domain.StepStateFailedValidation},
			},
			expected: domain.RunStateFailed,
		},
		{
			name: "paused for gate when approval is pending",
			steps: []*domain.Step{
				{State: domain.StepStateCompleted},
				{State: domain.StepStateNeedsApproval},
			},
			expected: domain.RunStatePausedForGate,
		},
		{
			name: "cancelled when all terminal steps were cancelled",
			steps: []*domain.Step{
				{State: domain.StepStateCancelled},
				{State: domain.StepStateCancelled},
			},
			expected: domain.RunStateCancelled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := deriveRunState(tt.steps); got != tt.expected {
				t.Fatalf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}
