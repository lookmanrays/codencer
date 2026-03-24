package service

import (
	"agent-bridge/internal/domain"
	"strings"
)

// PolicyEvaluation represents the outcome of a policy check.
type PolicyEvaluation struct {
	ShouldGate  bool
	GateReasons []string
	ShouldFail  bool
	FailReasons []string
	ShouldRetry bool
}

// Evaluate step execution artifacts & outcomes against a policy to determine the next action.
func Evaluate(p *domain.Policy, result *domain.Result, changedFiles []string) PolicyEvaluation {
	var eval PolicyEvaluation

	// Check failures
	if p.FailWhen.ArtifactPersistenceFailed && (result.Status == domain.StepStateFailedTerminal) { // Simplification
		eval.ShouldFail = true
		eval.FailReasons = append(eval.FailReasons, "Artifact persistence marked as failed")
	}

	// Check gating rules
	if p.GateWhen.ChangedFilesOver > 0 && len(changedFiles) > p.GateWhen.ChangedFilesOver {
		eval.ShouldGate = true
		eval.GateReasons = append(eval.GateReasons, "Changed files exceeded threshold")
	}

	hasDependencyChanges := false
	hasMigrations := false
	for _, f := range changedFiles {
		if strings.Contains(f, "go.mod") || strings.Contains(f, "go.sum") || strings.Contains(f, "package.json") {
			hasDependencyChanges = true
		}
		if strings.Contains(f, "migrations/") {
			hasMigrations = true
		}
	}

	if p.GateWhen.DependencyFilesChanged && hasDependencyChanges {
		eval.ShouldGate = true
		eval.GateReasons = append(eval.GateReasons, "Dependency files were changed")
	}

	if p.GateWhen.MigrationsDetected && hasMigrations {
		eval.ShouldGate = true
		eval.GateReasons = append(eval.GateReasons, "Database migrations were changed")
	}

	if p.GateWhen.UnresolvedQuestionsPresent && len(result.Questions) > 0 {
		eval.ShouldGate = true
		eval.GateReasons = append(eval.GateReasons, "Unresolved questions presented by adapter")
	}

	return eval
}
