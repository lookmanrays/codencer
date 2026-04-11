package claude

import (
	"fmt"
	"os"
	"strings"

	"agent-bridge/internal/domain"
)

type promptPayload struct {
	title          string
	goal           string
	context        string
	constraints    []string
	allowedPaths   []string
	forbiddenPaths []string
	acceptance     []string
	validations    []domain.ValidationCommand
	stopConditions []string
	policy         string
	timeoutSeconds int
}

func buildPrompt(step *domain.Step) string {
	payload := promptFromStep(step)

	var sections []string
	appendTextSection(&sections, "Task Title", payload.title)
	appendTextSection(&sections, "Goal", payload.goal)
	appendTextSection(&sections, "Context", payload.context)
	appendListSection(&sections, "Constraints", payload.constraints)
	appendListSection(&sections, "Allowed Paths", payload.allowedPaths)
	appendListSection(&sections, "Forbidden Paths", payload.forbiddenPaths)
	appendListSection(&sections, "Acceptance Criteria", payload.acceptance)
	appendValidationSection(&sections, "Validations", payload.validations)
	appendListSection(&sections, "Stop Conditions", payload.stopConditions)
	appendTextSection(&sections, "Policy", payload.policy)
	if payload.timeoutSeconds > 0 {
		appendTextSection(&sections, "Timeout Seconds", fmt.Sprintf("%d", payload.timeoutSeconds))
	}

	if len(sections) == 0 {
		return "Goal\nComplete the requested task."
	}

	return strings.Join(sections, "\n\n")
}

func writePromptArtifact(path string, step *domain.Step) (string, error) {
	prompt := buildPrompt(step)
	if err := os.WriteFile(path, []byte(prompt), 0644); err != nil {
		return "", err
	}
	return prompt, nil
}

func promptFromStep(step *domain.Step) promptPayload {
	payload := promptPayload{}
	if step == nil {
		return payload
	}

	if step.TaskSpecSnapshot != nil {
		spec := step.TaskSpecSnapshot
		payload.title = firstNonEmpty(spec.Title, step.Title)
		payload.goal = firstNonEmpty(spec.Goal, step.Goal)
		payload.context = strings.TrimSpace(spec.Context.Summary)
		payload.constraints = append([]string(nil), spec.Constraints...)
		payload.allowedPaths = append([]string(nil), spec.AllowedPaths...)
		payload.forbiddenPaths = append([]string(nil), spec.ForbiddenPaths...)
		payload.acceptance = append([]string(nil), spec.Acceptance...)
		payload.validations = append([]domain.ValidationCommand(nil), spec.Validations...)
		payload.stopConditions = append([]string(nil), spec.StopConditions...)
		payload.policy = firstNonEmpty(spec.PolicyBundle, step.Policy)
		if spec.TimeoutSeconds > 0 {
			payload.timeoutSeconds = spec.TimeoutSeconds
		} else {
			payload.timeoutSeconds = step.TimeoutSeconds
		}
	} else {
		payload.title = strings.TrimSpace(step.Title)
		payload.goal = strings.TrimSpace(step.Goal)
		payload.validations = append([]domain.ValidationCommand(nil), step.Validations...)
		payload.policy = strings.TrimSpace(step.Policy)
		payload.timeoutSeconds = step.TimeoutSeconds
	}

	if payload.title == "" && payload.goal == "" {
		payload.goal = "Complete the requested task."
	}

	if len(payload.validations) == 0 && len(step.Validations) > 0 {
		payload.validations = append([]domain.ValidationCommand(nil), step.Validations...)
	}

	return payload
}

func appendTextSection(sections *[]string, header string, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	*sections = append(*sections, fmt.Sprintf("%s\n%s", header, value))
}

func appendListSection(sections *[]string, header string, values []string) {
	lines := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		lines = append(lines, "- "+value)
	}
	if len(lines) == 0 {
		return
	}

	*sections = append(*sections, fmt.Sprintf("%s\n%s", header, strings.Join(lines, "\n")))
}

func appendValidationSection(sections *[]string, header string, values []domain.ValidationCommand) {
	lines := make([]string, 0, len(values))
	for _, validation := range values {
		command := strings.TrimSpace(validation.Command)
		if command == "" {
			continue
		}

		name := strings.TrimSpace(validation.Name)
		if name != "" {
			lines = append(lines, fmt.Sprintf("- %s: %s", name, command))
			continue
		}
		lines = append(lines, "- "+command)
	}
	if len(lines) == 0 {
		return
	}

	*sections = append(*sections, fmt.Sprintf("%s\n%s", header, strings.Join(lines, "\n")))
}
