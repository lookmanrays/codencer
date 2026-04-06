package validation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"agent-bridge/internal/domain"
	"gopkg.in/yaml.v3"
)

type DirectTaskOptions struct {
	Goal               string
	Title              string
	Context            string
	Adapter            string
	TimeoutSeconds     int
	Policy             string
	AcceptanceCriteria []string
	ValidationCommands []string
}

type NormalizeTaskInputRequest struct {
	RunID      string
	SourceKind domain.SubmissionSourceKind
	SourceName string
	Content    []byte
	Direct     DirectTaskOptions
}

type NormalizedTaskInput struct {
	Task       *domain.TaskSpec
	Provenance *domain.SubmissionProvenance
}

func ParseTaskSpecFile(path string) (*domain.TaskSpec, []byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("could not read task spec file: %w", err)
	}
	spec, err := ParseTaskSpecBytes(b)
	if err != nil {
		return nil, nil, err
	}
	return spec, b, nil
}

func ParseTaskSpecBytes(b []byte) (*domain.TaskSpec, error) {
	var spec domain.TaskSpec
	if err := yaml.Unmarshal(b, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse task spec yaml/json: %w", err)
	}
	return &spec, nil
}

func ParseTaskSpecJSONBytes(b []byte) (*domain.TaskSpec, error) {
	var spec domain.TaskSpec
	if err := json.Unmarshal(b, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse task spec json: %w", err)
	}
	return &spec, nil
}

func NormalizeTaskInput(req NormalizeTaskInputRequest) (*NormalizedTaskInput, error) {
	switch req.SourceKind {
	case domain.SubmissionSourceTaskFile:
		return normalizeCanonicalTask(req, false)
	case domain.SubmissionSourceTaskJSON:
		return normalizeCanonicalTask(req, true)
	case domain.SubmissionSourcePromptFile, domain.SubmissionSourceGoal, domain.SubmissionSourceStdin:
		return normalizeDirectTask(req)
	default:
		return nil, fmt.Errorf("unsupported submission source %q", req.SourceKind)
	}
}

func normalizeCanonicalTask(req NormalizeTaskInputRequest, strictJSON bool) (*NormalizedTaskInput, error) {
	var (
		task *domain.TaskSpec
		err  error
	)

	if strictJSON {
		task, err = ParseTaskSpecJSONBytes(req.Content)
	} else {
		task, err = ParseTaskSpecBytes(req.Content)
	}
	if err != nil {
		return nil, err
	}

	authoredRunID := task.RunID
	if task.RunID == "" {
		task.RunID = req.RunID
	}
	if task.RunID != req.RunID {
		return nil, fmt.Errorf("task run_id %q does not match submit run ID %q", task.RunID, req.RunID)
	}

	provenance := &domain.SubmissionProvenance{
		SourceKind:      req.SourceKind,
		SourceName:      filepath.Base(req.SourceName),
		OriginalFormat:  canonicalSourceFormat(req.SourceKind, req.SourceName),
		OriginalInput:   string(req.Content),
		DefaultsApplied: nil,
	}
	if task.SubmissionProvenance != nil {
		provenance.DefaultsApplied = append(provenance.DefaultsApplied, task.SubmissionProvenance.DefaultsApplied...)
	}
	if authoredRunID == "" {
		provenance.DefaultsApplied = append(provenance.DefaultsApplied, "run_id")
	}
	task.SubmissionProvenance = provenance

	return &NormalizedTaskInput{
		Task:       task,
		Provenance: provenance,
	}, nil
}

func normalizeDirectTask(req NormalizeTaskInputRequest) (*NormalizedTaskInput, error) {
	goal := string(req.Content)
	if strings.TrimSpace(goal) == "" {
		return nil, fmt.Errorf("direct input is empty")
	}

	defaults := []string{"version"}
	title := strings.TrimSpace(req.Direct.Title)
	if title == "" {
		switch req.SourceKind {
		case domain.SubmissionSourcePromptFile:
			title = promptDefaultTitle(req.SourceName)
		default:
			title = "Direct task"
		}
		defaults = append(defaults, "title")
	}

	task := &domain.TaskSpec{
		Version:        "v1",
		RunID:          req.RunID,
		Title:          title,
		Goal:           goal,
		Acceptance:     append([]string(nil), req.Direct.AcceptanceCriteria...),
		PolicyBundle:   req.Direct.Policy,
		AdapterProfile: req.Direct.Adapter,
		TimeoutSeconds: req.Direct.TimeoutSeconds,
		Validations:    normalizeValidationCommands(req.Direct.ValidationCommands),
	}
	if strings.TrimSpace(req.Direct.Context) != "" {
		task.Context = domain.TaskContext{Summary: req.Direct.Context}
	}

	provenance := &domain.SubmissionProvenance{
		SourceKind:      req.SourceKind,
		SourceName:      filepath.Base(req.SourceName),
		OriginalFormat:  directSourceFormat(req.SourceKind, req.SourceName),
		OriginalInput:   goal,
		DefaultsApplied: defaults,
	}
	task.SubmissionProvenance = provenance

	return &NormalizedTaskInput{
		Task:       task,
		Provenance: provenance,
	}, nil
}

func normalizeValidationCommands(cmds []string) []domain.ValidationCommand {
	if len(cmds) == 0 {
		return nil
	}

	validations := make([]domain.ValidationCommand, 0, len(cmds))
	for i, cmd := range cmds {
		validations = append(validations, domain.ValidationCommand{
			Name:    fmt.Sprintf("validation-%d", i+1),
			Command: cmd,
		})
	}
	return validations
}

func promptDefaultTitle(sourceName string) string {
	base := filepath.Base(sourceName)
	ext := filepath.Ext(base)
	if ext == "" {
		return base
	}
	return strings.TrimSuffix(base, ext)
}

func canonicalSourceFormat(sourceKind domain.SubmissionSourceKind, sourceName string) string {
	if sourceKind == domain.SubmissionSourceTaskJSON {
		return "json"
	}
	switch ext := strings.ToLower(filepath.Ext(sourceName)); ext {
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	default:
		return "yaml"
	}
}

func directSourceFormat(sourceKind domain.SubmissionSourceKind, sourceName string) string {
	switch sourceKind {
	case domain.SubmissionSourcePromptFile:
		switch ext := strings.ToLower(filepath.Ext(sourceName)); ext {
		case ".md":
			return "md"
		case ".txt":
			return "txt"
		default:
			if ext != "" {
				return strings.TrimPrefix(ext, ".")
			}
			return "txt"
		}
	default:
		return "txt"
	}
}
