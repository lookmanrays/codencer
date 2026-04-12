package service

import "agent-bridge/internal/domain"

func deriveRunState(steps []*domain.Step) domain.RunState {
	if len(steps) == 0 {
		return domain.RunStateRunning
	}

	hasPending := false
	hasApproval := false
	hasFailure := false
	hasCancelled := false
	allCompleted := true

	for _, step := range steps {
		switch step.State {
		case domain.StepStateNeedsApproval:
			hasApproval = true
			allCompleted = false
		case domain.StepStatePending, domain.StepStateDispatching, domain.StepStateRunning, domain.StepStateCollectingArtifacts, domain.StepStateValidating:
			hasPending = true
			allCompleted = false
		case domain.StepStateCompleted, domain.StepStateCompletedWithWarnings:
		case domain.StepStateCancelled:
			hasCancelled = true
			allCompleted = false
		default:
			hasFailure = true
			allCompleted = false
		}
	}

	switch {
	case hasApproval:
		return domain.RunStatePausedForGate
	case hasPending:
		return domain.RunStateRunning
	case hasFailure:
		return domain.RunStateFailed
	case allCompleted:
		return domain.RunStateCompleted
	case hasCancelled:
		return domain.RunStateCancelled
	default:
		return domain.RunStateRunning
	}
}
