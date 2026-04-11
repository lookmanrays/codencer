package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"agent-bridge/internal/domain"
)

type claudeResultEnvelope struct {
	Type    string   `json:"type"`
	Subtype string   `json:"subtype"`
	Result  string   `json:"result"`
	IsError bool     `json:"is_error"`
	Errors  []string `json:"errors"`
}

func NormalizeCore(attemptID string, artifacts []*domain.Artifact, adapterName string, isSimulation bool) (*domain.ResultSpec, error) {
	defaultRes := &domain.ResultSpec{
		Version:      "v1",
		AttemptID:    attemptID,
		Adapter:      adapterName,
		IsSimulation: isSimulation,
		UpdatedAt:    time.Now().UTC(),
	}

	var resultPath string
	for _, art := range artifacts {
		if art.Type == domain.ArtifactTypeResultJSON {
			resultPath = art.Path
			break
		}
	}

	if resultPath == "" {
		if isSimulation {
			defaultRes.State = domain.StepStateCompleted
			defaultRes.Summary = "Simulation: Claude adapter relay completed successfully."
			return defaultRes, nil
		}

		defaultRes.State = domain.StepStateFailedTerminal
		defaultRes.Summary = "Bridge Interface Error: Claude finished but failed to produce result.json."
		return defaultRes, nil
	}

	data, err := os.ReadFile(resultPath)
	if err != nil {
		defaultRes.State = domain.StepStateFailedTerminal
		defaultRes.Summary = fmt.Sprintf("Bridge Interface Error: Failed to read Claude result.json: %v", err)
		return defaultRes, nil
	}

	var res domain.ResultSpec
	if err := json.Unmarshal(data, &res); err != nil {
		defaultRes.State = domain.StepStateFailedTerminal
		defaultRes.Summary = fmt.Sprintf("Bridge Interface Error: Claude result.json malformed: %v", err)
		return defaultRes, nil
	}

	res.Version = "v1"
	res.AttemptID = attemptID
	res.Adapter = adapterName
	res.IsSimulation = isSimulation
	res.UpdatedAt = time.Now().UTC()
	if res.Artifacts == nil {
		res.Artifacts = make(map[string]string)
	}

	for _, art := range artifacts {
		res.Artifacts[art.Name] = art.Path
		if art.Type == domain.ArtifactTypeStdout {
			res.RawOutputRef = art.Path
		}
	}

	if res.State == "" {
		res.State = domain.StepStateFailedTerminal
		res.Summary = "Bridge Interface Error: Claude result spec missing 'state' field."
	}

	return &res, nil
}

func parseClaudeResultFromStdout(stdoutPath string) (*domain.ResultSpec, error) {
	data, err := os.ReadFile(stdoutPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read stdout log: %w", err)
	}

	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, fmt.Errorf("stdout was empty")
	}

	var payload claudeResultEnvelope
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return nil, fmt.Errorf("stdout was not valid JSON: %w", err)
	}
	if payload.Type != "result" {
		return nil, fmt.Errorf("expected Claude result event, got type %q", payload.Type)
	}

	return mapClaudeResult(payload), nil
}

func mapClaudeResult(payload claudeResultEnvelope) *domain.ResultSpec {
	res := &domain.ResultSpec{
		Version: "v1",
		State:   domain.StepStateFailedTerminal,
	}

	errorText := strings.TrimSpace(strings.Join(payload.Errors, "; "))
	resultText := strings.TrimSpace(payload.Result)

	switch payload.Subtype {
	case "success":
		if !payload.IsError {
			res.State = domain.StepStateCompleted
			if resultText != "" {
				res.Summary = resultText
			} else {
				res.Summary = "Claude task completed successfully."
			}
			break
		}
		fallthrough
	case "error_during_execution":
		res.State = domain.StepStateFailedAdapter
		res.Summary = firstNonEmpty(errorText, resultText, "Claude execution error")
	case "error_max_turns":
		res.State = domain.StepStateFailedTerminal
		res.Summary = firstNonEmpty(resultText, "Claude reached the maximum turn limit")
	case "error_max_budget_usd":
		res.State = domain.StepStateFailedTerminal
		res.Summary = firstNonEmpty(resultText, "Claude exceeded the configured budget")
	case "error_max_structured_output_retries":
		res.State = domain.StepStateFailedTerminal
		res.Summary = firstNonEmpty(resultText, "Claude exhausted structured output retries")
	default:
		res.State = domain.StepStateFailedTerminal
		res.Summary = firstNonEmpty(resultText, errorText, fmt.Sprintf("Unknown Claude result subtype %q", payload.Subtype))
	}

	return res
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
