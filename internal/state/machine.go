package state

import (
	"fmt"

	"agent-bridge/internal/domain"
)

var (
	ErrInvalidRunTransition  = fmt.Errorf("invalid run transition")
	ErrInvalidStepTransition = fmt.Errorf("invalid step transition")
)

// ValidRunTransitions defines the allowed target states from a given Run state.
var ValidRunTransitions = map[domain.RunState][]domain.RunState{
	domain.RunStateCreated:       {domain.RunStateRunning, domain.RunStateCancelled},
	domain.RunStateRunning:       {domain.RunStatePausedForGate, domain.RunStateCompleted, domain.RunStateFailed, domain.RunStateCancelled},
	domain.RunStatePausedForGate: {domain.RunStateRunning, domain.RunStateCancelled, domain.RunStateFailed},
	// Terminal states have no valid outward transitions.
	domain.RunStateCompleted: {},
	domain.RunStateFailed:    {},
	domain.RunStateCancelled: {},
}

// ValidStepTransitions defines the allowed target states from a given Step state.
var ValidStepTransitions = map[domain.StepState][]domain.StepState{
	domain.StepStatePending:             {domain.StepStateDispatching, domain.StepStateCancelled},
	domain.StepStateDispatching:         {domain.StepStateRunning, domain.StepStateFailedRetryable, domain.StepStateFailedTerminal, domain.StepStateCancelled, domain.StepStateTimeout},
	domain.StepStateRunning:             {domain.StepStateCollectingArtifacts, domain.StepStateFailedRetryable, domain.StepStateFailedTerminal, domain.StepStateCancelled, domain.StepStateTimeout, domain.StepStateNeedsManualAttention},
	domain.StepStateCollectingArtifacts: {domain.StepStateValidating, domain.StepStateFailedRetryable, domain.StepStateFailedTerminal, domain.StepStateCancelled, domain.StepStateTimeout, domain.StepStateNeedsManualAttention},
	domain.StepStateValidating:          {domain.StepStateCompleted, domain.StepStateCompletedWithWarnings, domain.StepStateNeedsApproval, domain.StepStateFailedRetryable, domain.StepStateFailedTerminal, domain.StepStateCancelled, domain.StepStateTimeout, domain.StepStateNeedsManualAttention},
	domain.StepStateNeedsApproval:       {domain.StepStateCompleted, domain.StepStateCompletedWithWarnings, domain.StepStateFailedRetryable, domain.StepStateFailedTerminal, domain.StepStateCancelled},
	domain.StepStateFailedRetryable:     {domain.StepStateDispatching, domain.StepStateCancelled},
	
	// Terminal/Sink states
	domain.StepStateCompleted:             {},
	domain.StepStateCompletedWithWarnings: {},
	domain.StepStateFailedTerminal:        {},
	domain.StepStateTimeout:               {},
	domain.StepStateCancelled:             {},
	domain.StepStateNeedsManualAttention:  {},
}

// CheckRunTransition evaluates if a run can move from 'current' to 'next'.
func CheckRunTransition(current, next domain.RunState) error {
	for _, target := range ValidRunTransitions[current] {
		if next == target {
			return nil
		}
	}
	return fmt.Errorf("%w: cannot transition from %q to %q", ErrInvalidRunTransition, current, next)
}

// CheckStepTransition evaluates if a step can move from 'current' to 'next'.
func CheckStepTransition(current, next domain.StepState) error {
	for _, target := range ValidStepTransitions[current] {
		if next == target {
			return nil
		}
	}
	return fmt.Errorf("%w: cannot transition from %q to %q", ErrInvalidStepTransition, current, next)
}
