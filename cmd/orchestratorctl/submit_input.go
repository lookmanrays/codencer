package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/validation"
)

type submitOptions struct {
	runID         string
	wait          bool
	json          bool
	sourceKind    domain.SubmissionSourceKind
	sourceName    string
	directOptions validation.DirectTaskOptions
}

func handleSubmitCommand(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, submitUsage())
		os.Exit(exitCodeUsage)
	}

	opts, err := parseSubmitOptions(args)
	if err != nil {
		failCLI(hasFlag(args, "--json"), exitCodeUsage, "invalid submit input", err.Error())
	}

	normalized, err := loadAndNormalizeSubmitInput(opts)
	if err != nil {
		failCLI(opts.json, exitCodeUsage, "preparing task submission", err.Error())
	}

	submitTaskSpec(opts.runID, normalized.Task, opts.wait, opts.json)
}

func parseSubmitOptions(args []string) (*submitOptions, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("%s", submitUsage())
	}

	opts := &submitOptions{runID: args[0]}
	var positionalTaskFile string
	var sourceCount int
	var sawTaskJSON, sawPromptFile, sawGoal, sawStdin bool

	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--wait":
			opts.wait = true
		case "--json":
			opts.json = true
		case "--task-json":
			value, next, err := requireValue(args, i, "--task-json")
			if err != nil {
				return nil, err
			}
			i = next
			opts.sourceKind = domain.SubmissionSourceTaskJSON
			opts.sourceName = value
			sawTaskJSON = true
			sourceCount++
		case "--prompt-file":
			value, next, err := requireValue(args, i, "--prompt-file")
			if err != nil {
				return nil, err
			}
			i = next
			opts.sourceKind = domain.SubmissionSourcePromptFile
			opts.sourceName = value
			sawPromptFile = true
			sourceCount++
		case "--goal":
			value, next, err := requireValue(args, i, "--goal")
			if err != nil {
				return nil, err
			}
			i = next
			opts.sourceKind = domain.SubmissionSourceGoal
			opts.sourceName = "inline-goal"
			opts.directOptions.Goal = value
			sawGoal = true
			sourceCount++
		case "--stdin":
			opts.sourceKind = domain.SubmissionSourceStdin
			opts.sourceName = "stdin"
			sawStdin = true
			sourceCount++
		case "--title":
			value, next, err := requireValue(args, i, "--title")
			if err != nil {
				return nil, err
			}
			i = next
			opts.directOptions.Title = value
		case "--context":
			value, next, err := requireValue(args, i, "--context")
			if err != nil {
				return nil, err
			}
			i = next
			opts.directOptions.Context = value
		case "--adapter":
			value, next, err := requireValue(args, i, "--adapter")
			if err != nil {
				return nil, err
			}
			i = next
			opts.directOptions.Adapter = value
		case "--timeout":
			value, next, err := requireValue(args, i, "--timeout")
			if err != nil {
				return nil, err
			}
			i = next
			timeoutSeconds, err := parseIntArg(value, "--timeout")
			if err != nil {
				return nil, err
			}
			opts.directOptions.TimeoutSeconds = timeoutSeconds
		case "--policy":
			value, next, err := requireValue(args, i, "--policy")
			if err != nil {
				return nil, err
			}
			i = next
			opts.directOptions.Policy = value
		case "--acceptance":
			value, next, err := requireValue(args, i, "--acceptance")
			if err != nil {
				return nil, err
			}
			i = next
			opts.directOptions.AcceptanceCriteria = append(opts.directOptions.AcceptanceCriteria, value)
		case "--validation":
			value, next, err := requireValue(args, i, "--validation")
			if err != nil {
				return nil, err
			}
			i = next
			opts.directOptions.ValidationCommands = append(opts.directOptions.ValidationCommands, value)
		default:
			if strings.HasPrefix(arg, "--") {
				return nil, fmt.Errorf("unknown submit flag %q", arg)
			}
			if positionalTaskFile != "" {
				return nil, fmt.Errorf("submit accepts at most one positional task file")
			}
			positionalTaskFile = arg
			opts.sourceKind = domain.SubmissionSourceTaskFile
			opts.sourceName = arg
			sourceCount++
		}
	}

	if sourceCount != 1 {
		return nil, fmt.Errorf("submit requires exactly one primary input source: positional task file, --task-json, --prompt-file, --goal, or --stdin")
	}

	if opts.runID == "" {
		return nil, fmt.Errorf("submit requires a run ID")
	}

	if isCanonicalSubmitSource(opts.sourceKind) && hasDirectMetadata(opts.directOptions) {
		return nil, fmt.Errorf("direct metadata flags (--title, --context, --adapter, --timeout, --policy, --acceptance, --validation) are only supported with --prompt-file, --goal, or --stdin")
	}

	if opts.sourceKind == domain.SubmissionSourceGoal && !sawGoal {
		return nil, fmt.Errorf("--goal requires a value")
	}
	if opts.sourceKind == domain.SubmissionSourceTaskJSON && !sawTaskJSON {
		return nil, fmt.Errorf("--task-json requires a value")
	}
	if opts.sourceKind == domain.SubmissionSourcePromptFile && !sawPromptFile {
		return nil, fmt.Errorf("--prompt-file requires a value")
	}
	if opts.sourceKind == domain.SubmissionSourceStdin && !sawStdin {
		return nil, fmt.Errorf("--stdin was not selected")
	}

	return opts, nil
}

func loadAndNormalizeSubmitInput(opts *submitOptions) (*validation.NormalizedTaskInput, error) {
	var content []byte
	switch opts.sourceKind {
	case domain.SubmissionSourceTaskFile:
		b, err := os.ReadFile(opts.sourceName)
		if err != nil {
			return nil, fmt.Errorf("could not read task spec file: %w", err)
		}
		content = b
	case domain.SubmissionSourceTaskJSON:
		b, err := readSubmitSourceBytes(opts.sourceName)
		if err != nil {
			return nil, fmt.Errorf("could not read task json: %w", err)
		}
		content = b
	case domain.SubmissionSourcePromptFile:
		b, err := os.ReadFile(opts.sourceName)
		if err != nil {
			return nil, fmt.Errorf("could not read prompt file: %w", err)
		}
		content = b
	case domain.SubmissionSourceGoal:
		content = []byte(opts.directOptions.Goal)
	case domain.SubmissionSourceStdin:
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("failed to read task input from stdin: %w", err)
		}
		content = b
	default:
		return nil, fmt.Errorf("unsupported submit source %q", opts.sourceKind)
	}

	return validation.NormalizeTaskInput(validation.NormalizeTaskInputRequest{
		RunID:      opts.runID,
		SourceKind: opts.sourceKind,
		SourceName: opts.sourceName,
		Content:    content,
		Direct:     opts.directOptions,
	})
}

func submitTaskSpec(runID string, spec *domain.TaskSpec, shouldWait, asJSON bool) {
	data, _ := json.Marshal(spec)

	resp, err := http.Post(orchestratordURL+"/api/v1/runs/"+runID+"/steps", "application/json", bytes.NewReader(data))
	if err != nil {
		failCLI(asJSON, exitCodeInfrastructure, "connecting to orchestratord", err.Error())
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		failHTTP(asJSON, resp.StatusCode, body)
	}

	var step domain.Step
	if err := json.Unmarshal(body, &step); err != nil {
		emitJSONBody(body)
		return
	}

	if shouldWait {
		if !asJSON {
			printJSON(body)
		}
		fmt.Fprintf(os.Stderr, "==> Auto-waiting for Step %s...\n", step.ID)
		os.Exit(stepWait(step.ID, 2*time.Second, 0, asJSON))
	}

	emitJSONBody(body)
	if !asJSON {
		fmt.Fprintf(os.Stderr, "\n[SUCCESS] Unified Step UUID: %s\n", step.ID)
		fmt.Fprintf(os.Stderr, "[GUIDE] To monitor transition:\n  ./bin/orchestratorctl step wait %s\n", step.ID)
		fmt.Fprintf(os.Stderr, "[GUIDE] To view total audit trail:\n  ./bin/orchestratorctl step result %s\n", step.ID)
	}
}

func requireValue(args []string, i int, flag string) (string, int, error) {
	if i+1 >= len(args) {
		return "", i, fmt.Errorf("%s requires a value", flag)
	}
	return args[i+1], i + 1, nil
}

func parseIntArg(value, flag string) (int, error) {
	timeout, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s expects an integer number of seconds", flag)
	}
	return timeout, nil
}

func readSubmitSourceBytes(name string) ([]byte, error) {
	if name == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(name)
}

func hasDirectMetadata(opts validation.DirectTaskOptions) bool {
	return opts.Title != "" ||
		opts.Context != "" ||
		opts.Adapter != "" ||
		opts.TimeoutSeconds != 0 ||
		opts.Policy != "" ||
		len(opts.AcceptanceCriteria) > 0 ||
		len(opts.ValidationCommands) > 0
}

func isCanonicalSubmitSource(kind domain.SubmissionSourceKind) bool {
	return kind == domain.SubmissionSourceTaskFile || kind == domain.SubmissionSourceTaskJSON
}

func submitUsage() string {
	return strings.Join([]string{
		"Usage: orchestratorctl submit <runID> <task-file>|--task-json <path|->|--prompt-file <path>|--goal <text>|--stdin [flags]",
		"Primary source: choose exactly one of positional task file, --task-json, --prompt-file, --goal, or --stdin.",
		"Direct metadata flags (direct sources only): --title, --context, --adapter, --timeout, --policy, repeated --acceptance, repeated --validation.",
		"Auditability: accepted submissions retain original-input.* and normalized-task.json under the attempt artifact root.",
	}, "\n")
}
