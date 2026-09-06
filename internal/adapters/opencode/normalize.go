package opencode

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"agent-bridge/internal/adapters/common"
	"agent-bridge/internal/domain"
)

type runEvent struct {
	Type  string          `json:"type"`
	Part  json.RawMessage `json:"part"`
	Error json.RawMessage `json:"error"`
}

type textPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
	Time struct {
		End *int64 `json:"end"`
	} `json:"time"`
}

func parseEvents(stdoutPath string) (lastText, eventError string) {
	file, err := os.Open(stdoutPath)
	if err != nil {
		return "", ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buffer := make([]byte, 0, 64*1024)
	scanner.Buffer(buffer, 2*1024*1024)
	for scanner.Scan() {
		var event runEvent
		if json.Unmarshal(scanner.Bytes(), &event) != nil {
			continue // stderr and future informational output must not invalidate a completed run.
		}
		switch event.Type {
		case "text":
			var part textPart
			if json.Unmarshal(event.Part, &part) == nil && part.Type == "text" && part.Time.End != nil && strings.TrimSpace(part.Text) != "" {
				lastText = strings.TrimSpace(part.Text)
			}
		case "error":
			if message := eventErrorMessage(event.Error); message != "" {
				eventError = message
			}
		}
	}
	return lastText, eventError
}

func eventErrorMessage(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "OpenCode reported an execution error."
	}
	var stringValue string
	if json.Unmarshal(raw, &stringValue) == nil && strings.TrimSpace(stringValue) != "" {
		return strings.TrimSpace(stringValue)
	}
	var object struct {
		Message string `json:"message"`
		Data    struct {
			Message string `json:"message"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &object) == nil {
		return firstNonEmpty(object.Data.Message, object.Message)
	}
	return "OpenCode reported an execution error."
}

func normalizeResult(attemptID string, artifacts []*domain.Artifact, isSimulation bool) (*domain.ResultSpec, error) {
	var resultPath string
	for _, artifact := range artifacts {
		if artifact.Type == domain.ArtifactTypeResultJSON {
			resultPath = artifact.Path
			break
		}
	}
	if resultPath == "" {
		if isSimulation {
			return &domain.ResultSpec{Version: "v1", AttemptID: attemptID, Adapter: AdapterID, State: domain.StepStateCompleted, Summary: "Simulation: OpenCode adapter relay completed successfully.", IsSimulation: true}, nil
		}
		return &domain.ResultSpec{Version: "v1", AttemptID: attemptID, Adapter: AdapterID, State: domain.StepStateFailedTerminal, Summary: "Bridge Interface Error: OpenCode finished but failed to produce result.json."}, nil
	}

	data, err := os.ReadFile(resultPath)
	if err != nil {
		return nil, fmt.Errorf("read OpenCode result: %w", err)
	}
	var result domain.ResultSpec
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode OpenCode result: %w", err)
	}
	result.Version = "v1"
	result.AttemptID = attemptID
	result.Adapter = AdapterID
	result.IsSimulation = isSimulation || common.ResultLooksSimulated(AdapterID, &result)
	result.UpdatedAt = time.Now().UTC()
	if result.Artifacts == nil {
		result.Artifacts = make(map[string]string)
	}
	for _, artifact := range artifacts {
		result.Artifacts[artifact.Name] = artifact.Path
		if artifact.Type == domain.ArtifactTypeStdout {
			result.RawOutputRef = artifact.Path
		}
	}
	if result.State == "" {
		result.State = domain.StepStateFailedTerminal
		result.Summary = "Bridge Interface Error: OpenCode result missing state."
	}
	return &result, nil
}
