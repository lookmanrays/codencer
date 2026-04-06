package main

import "agent-bridge/internal/domain"

const (
	exitCodeSuccess        = 0
	exitCodeUsage          = 1
	exitCodeTerminalFailed = 2
	exitCodeTimeout        = 3
	exitCodeIntervention   = 4
	exitCodeInfrastructure = 5
)

func exitCodeForStepState(state domain.StepState) int {
	switch state {
	case domain.StepStateCompleted, domain.StepStateCompletedWithWarnings:
		return exitCodeSuccess
	case domain.StepStateFailedTerminal, domain.StepStateFailedValidation, domain.StepStateFailedRetryable:
		return exitCodeTerminalFailed
	case domain.StepStateTimeout:
		return exitCodeTimeout
	case domain.StepStateCancelled, domain.StepStateNeedsApproval, domain.StepStateNeedsManualAttention:
		return exitCodeIntervention
	case domain.StepStateFailedBridge, domain.StepStateFailedAdapter:
		return exitCodeInfrastructure
	default:
		return exitCodeInfrastructure
	}
}

func exitCodeForRunState(state domain.RunState) int {
	switch state {
	case domain.RunStateCompleted:
		return exitCodeSuccess
	case domain.RunStateFailed:
		return exitCodeTerminalFailed
	case domain.RunStateCancelled, domain.RunStatePausedForGate:
		return exitCodeIntervention
	default:
		return exitCodeInfrastructure
	}
}

func exitCodeForHTTPStatus(status int) int {
	switch {
	case status == 404 || status == 400 || status == 405:
		return exitCodeUsage
	case status >= 400:
		return exitCodeInfrastructure
	default:
		return exitCodeSuccess
	}
}
