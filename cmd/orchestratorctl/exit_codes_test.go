package main

import (
	"testing"

	"agent-bridge/internal/domain"
)

func TestExitCodeForStepState(t *testing.T) {
	tests := []struct {
		state domain.StepState
		want  int
	}{
		{state: domain.StepStateCompleted, want: exitCodeSuccess},
		{state: domain.StepStateCompletedWithWarnings, want: exitCodeSuccess},
		{state: domain.StepStateFailedTerminal, want: exitCodeTerminalFailed},
		{state: domain.StepStateFailedValidation, want: exitCodeTerminalFailed},
		{state: domain.StepStateFailedRetryable, want: exitCodeTerminalFailed},
		{state: domain.StepStateTimeout, want: exitCodeTimeout},
		{state: domain.StepStateCancelled, want: exitCodeIntervention},
		{state: domain.StepStateNeedsApproval, want: exitCodeIntervention},
		{state: domain.StepStateNeedsManualAttention, want: exitCodeIntervention},
		{state: domain.StepStateFailedBridge, want: exitCodeInfrastructure},
		{state: domain.StepStateFailedAdapter, want: exitCodeInfrastructure},
	}

	for _, tt := range tests {
		if got := exitCodeForStepState(tt.state); got != tt.want {
			t.Fatalf("state %q: got %d want %d", tt.state, got, tt.want)
		}
	}
}

func TestExitCodeForRunState(t *testing.T) {
	tests := []struct {
		state domain.RunState
		want  int
	}{
		{state: domain.RunStateCompleted, want: exitCodeSuccess},
		{state: domain.RunStateFailed, want: exitCodeTerminalFailed},
		{state: domain.RunStateCancelled, want: exitCodeIntervention},
		{state: domain.RunStatePausedForGate, want: exitCodeIntervention},
	}

	for _, tt := range tests {
		if got := exitCodeForRunState(tt.state); got != tt.want {
			t.Fatalf("state %q: got %d want %d", tt.state, got, tt.want)
		}
	}
}
