package opencode

import (
	"encoding/json"
	"strings"

	"agent-bridge/internal/domain"
)

func buildPrompt(step *domain.Step, artifactRoot string) string {
	var builder strings.Builder
	builder.WriteString("You are the local OpenCode executor running under Codencer. Execute only the approved task below in the current workspace. Make the requested workspace changes; do not choose follow-up tasks.\n\n")
	builder.WriteString("Codencer is the source of record. Keep the work scoped, and report what changed in your final response. Codencer runs declared validations after you return.\n\n")
	builder.WriteString("Artifact root (for evidence only):\n")
	builder.WriteString(artifactRoot)
	builder.WriteString("\n\n")
	if step != nil && step.TaskSpecSnapshot != nil {
		if data, err := json.MarshalIndent(step.TaskSpecSnapshot, "", "  "); err == nil {
			builder.WriteString("TaskSpec:\n")
			builder.Write(data)
			builder.WriteString("\n")
			return builder.String()
		}
	}
	if step != nil && strings.TrimSpace(step.Title) != "" {
		builder.WriteString("Title:\n")
		builder.WriteString(strings.TrimSpace(step.Title))
		builder.WriteString("\n\n")
	}
	builder.WriteString("Goal:\n")
	if step == nil || strings.TrimSpace(step.Goal) == "" {
		builder.WriteString("Complete the requested task.\n")
	} else {
		builder.WriteString(strings.TrimSpace(step.Goal))
		builder.WriteString("\n")
	}
	return builder.String()
}
