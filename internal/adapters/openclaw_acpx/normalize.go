package openclaw_acpx

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"agent-bridge/internal/domain"
)

// NormalizeResult expects to find structured evidence in the session directory or artifact root.
func NormalizeResult(attemptID string, artifacts []*domain.Artifact, isSimulation bool) (*domain.ResultSpec, error) {
	res := &domain.ResultSpec{
		Version:      "v1",
		AttemptID:    attemptID,
		Adapter:      AdapterID,
		IsSimulation: isSimulation,
		State:        domain.StepStateCompleted, // Default
		UpdatedAt:    time.Now().UTC(),
		Artifacts:    make(map[string]string),
	}

	// 1. Link all collected artifacts
	for _, art := range artifacts {
		res.Artifacts[art.Name] = art.Path
		if art.Type == domain.ArtifactTypeStdout {
			res.RawOutputRef = art.Path
		}
	}

	if isSimulation {
		res.Summary = "Simulated OpenClaw execution successful."
		return res, nil
	}

	// 2. Search for structured ACP result
	// ACP typically drops a status.json or result.json in the session dir
	var acpStatusPath string
	for _, art := range artifacts {
		if art.Name == "acp-status.json" || art.Name == "result.json" {
			acpStatusPath = art.Path
			break
		}
	}

	if acpStatusPath == "" {
		// Fallback: check if we just have stdout and it looks okay
		res.Summary = "OpenClaw task completed. (No structured ACP result found; relying on stdout log)."
		return res, nil
	}

	data, err := os.ReadFile(acpStatusPath)
	if err != nil {
		res.State = domain.StepStateFailedBridge
		res.Summary = fmt.Sprintf("Failed to read ACP result file: %v", err)
		return res, nil
	}

	// Simple mapping for now
	type acpPayload struct {
		Status  string `json:"status"`
		Summary string `json:"summary"`
		Error   string `json:"error,omitempty"`
	}

	var payload acpPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		res.State = domain.StepStateFailedBridge
		res.Summary = fmt.Sprintf("ACP result file malformed: %v", err)
		return res, nil
	}

	res.Summary = payload.Summary
	if payload.Error != "" {
		res.Summary = fmt.Sprintf("%s (Error: %s)", payload.Summary, payload.Error)
	}

	switch payload.Status {
	case "completed", "success":
		res.State = domain.StepStateCompleted
	case "failed", "error":
		res.State = domain.StepStateFailedTerminal
	default:
		res.State = domain.StepStateFailedTerminal
		res.Summary = fmt.Sprintf("Unknown ACP status '%s'. %s", payload.Status, res.Summary)
	}

	return res, nil
}
