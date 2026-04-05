package state

import (
	"testing"
	"agent-bridge/internal/domain"
)

func TestRunTransitions(t *testing.T) {
	// Valid transitions
	validPairs := []struct{from, to domain.RunState}{
		{domain.RunStateCreated, domain.RunStateRunning},
		{domain.RunStateRunning, domain.RunStateCompleted},
		{domain.RunStatePausedForGate, domain.RunStateRunning},
	}

	for _, pair := range validPairs {
		if err := CheckRunTransition(pair.from, pair.to); err != nil {
			t.Errorf("Expected valid transition %q -> %q, got err: %v", pair.from, pair.to, err)
		}
	}

	// Invalid transitions
	invalidPairs := []struct{from, to domain.RunState}{
		{domain.RunStateCreated, domain.RunStateCompleted}, // Cannot skip running
		{domain.RunStateCompleted, domain.RunStateRunning}, // Terminal
		{domain.RunStatePausedForGate, domain.RunStateCreated}, // Cannot go back
	}

	for _, pair := range invalidPairs {
		if err := CheckRunTransition(pair.from, pair.to); err == nil {
			t.Errorf("Expected invalid transition %q -> %q, but got no error", pair.from, pair.to)
		}
	}
}

func TestStepTransitions(t *testing.T) {
	// Valid transitions
	if err := CheckStepTransition(domain.StepStatePending, domain.StepStateDispatching); err != nil {
		t.Error(err)
	}
	if err := CheckStepTransition(domain.StepStateValidating, domain.StepStateCompletedWithWarnings); err != nil {
		t.Error(err)
	}
	if err := CheckStepTransition(domain.StepStateFailedRetryable, domain.StepStateDispatching); err != nil {
		t.Error(err)
	}
	if err := CheckStepTransition(domain.StepStateRunning, domain.StepStateTimeout); err != nil {
		t.Error(err)
	}
	if err := CheckStepTransition(domain.StepStateValidating, domain.StepStateNeedsManualAttention); err != nil {
		t.Error(err)
	}
	if err := CheckStepTransition(domain.StepStateValidating, domain.StepStateFailedValidation); err != nil {
		t.Error(err)
	}
	if err := CheckStepTransition(domain.StepStateRunning, domain.StepStateFailedAdapter); err != nil {
		t.Error(err)
	}
	if err := CheckStepTransition(domain.StepStateDispatching, domain.StepStateFailedBridge); err != nil {
		t.Error(err)
	}

	// Invalid transitions
	if err := CheckStepTransition(domain.StepStatePending, domain.StepStateCompleted); err == nil {
		t.Error("Pending -> Completed should be invalid")
	}
}
