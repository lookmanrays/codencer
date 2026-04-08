package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"agent-bridge/internal/app"
	"agent-bridge/internal/domain"
	"agent-bridge/internal/validation"
	"github.com/joho/godotenv"
	"strconv"
)

var (
	orchestratordURL = "http://127.0.0.1:8085"
)

func main() {
	_ = godotenv.Load(".env")

	if env := os.Getenv("ORCHESTRATORD_URL"); env != "" {
		orchestratordURL = env
	} else {
		host := os.Getenv("HOST")
		if host == "" {
			host = "127.0.0.1"
		}
		port := os.Getenv("PORT")
		if port == "" {
			port = "8085"
		}
		orchestratordURL = fmt.Sprintf("http://%s:%s", host, port)
	}

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(exitCodeUsage)
	}

	command := os.Args[1]
	switch command {
	case "version":
		fmt.Printf("orchestratorctl version %s\n", app.Version)
	case "run":
		handleRunCommand(os.Args[2:])
	case "step":
		handleStepCommand(os.Args[1:])
	case "submit":
		handleSubmitCommand(os.Args[2:])
	case "gate":
		handleGateCmd(os.Args[2:])
	case "antigravity":
		handleAntigravityCmd(os.Args[2:])
	case "doctor":
		runDoctor()
	case "instance":
		handleInstanceCommand(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(exitCodeUsage)
	}
}

func printUsage() {
	fmt.Println("Usage: orchestratorctl <command> [args]")
	fmt.Println("\n1. Session & Mission Management:")
	fmt.Println("  run start      [id] [p] [--project x] [--conversation y] [--json]  Initialize a mission")
	fmt.Println("  run list       [--project x] [--conversation y] [--json]            Show mission history")
	fmt.Println("  run state      <runID> [--json]                                     Check mission state")
	fmt.Println("  run wait       <runID> [--interval d] [--timeout d] [--json]        Poll a run to terminal state")
	fmt.Println("  run abort      <runID> [--json]                                     Halt an active mission")

	fmt.Println("\n2. Tactical Execution (The Bridge):")
	fmt.Println("  submit         <runID> <task-file>|--task-json <path|->|--prompt-file <path>|--goal <text>|--stdin [flags] [--wait] [--json]")
	fmt.Println("                 exactly one primary source is required; direct metadata flags apply only to --prompt-file/--goal/--stdin")
	fmt.Println("                 direct submissions persist original-input.* and normalized-task.json as attempt evidence")
	fmt.Println("  step list      <runID> [--json]                                     List all task handles in a mission")
	fmt.Println("  step state     <handle> [--json]                                    Check a specific UUID state")
	fmt.Println("  step wait      <handle> [--interval d] [--timeout d] [--json]       Poll a specific UUID until completion")

	fmt.Println("\n3. Evidence & Inspection (The Truth):")
	fmt.Println("  step result    <handle> [--json]                                    Authoritative Truth (Summary)")
	fmt.Println("  step logs      <handle> [--json]                                    Raw agent stdout/stderr trail")
	fmt.Println("  step artifacts <handle> [--json]                                    List harvested files and diffs")
	fmt.Println("  step validations <handle> [--json]                                  Check specific test/lint results")

	fmt.Println("\n4. Maintenance & Health:")
	fmt.Println("  doctor                                                              Verify local environment/binaries")
	fmt.Println("  gate approve   <gateID> [--json]                                   Approve a paused policy gate")
	fmt.Println("  gate reject    <gateID> [--json]                                   Reject a paused policy gate")
	fmt.Println("  antigravity    <cmd>                                                Manage antigravity bindings")
	fmt.Println("  instance      [--json]                                              Show current daemon identity")
	fmt.Println("  version                                                             Show version")
}

func handleRunCommand(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: orchestratorctl run <start|list|state|abort|wait> [args]")
		fmt.Println("  start: orchestratorctl run start [id] [project] [--project p] [--conversation c] [--planner p] [--executor e] [--json]")
		fmt.Println("  list:  orchestratorctl run list [--project p] [--conversation c] [--state s] [--json]")
		os.Exit(exitCodeUsage)
	}

	cmd := args[0]
	switch cmd {
	case "start":
		id := ""
		if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
			id = args[1]
		}

		flags := parseRunStartFlags(args)
		startRun(id, flags)
	case "list":
		filters := parseRunListFilters(args[1:])
		listRuns(filters)
	case "status", "state":
		if len(args) < 2 {
			fmt.Println("Usage: orchestratorctl run state <runID> [--json]")
			os.Exit(exitCodeUsage)
		}
		runState(args[1], hasFlag(args[2:], "--json"))
	case "abort":
		if len(args) < 2 {
			fmt.Println("Usage: orchestratorctl run abort <id> [--json]")
			os.Exit(exitCodeUsage)
		}
		abortRun(args[1], hasFlag(args[2:], "--json"))
	case "wait":
		if len(args) < 2 {
			fmt.Println("Usage: orchestratorctl run wait <id> [--interval <d>] [--timeout <d>] [--json]")
			os.Exit(exitCodeUsage)
		}
		interval, timeout, asJSON := parseWaitFlags(args[2:])
		os.Exit(runWait(args[1], interval, timeout, asJSON))
	default:
		fmt.Fprintf(os.Stderr, "Unknown run command: %s\n", cmd)
		os.Exit(exitCodeUsage)
	}
}

func startRun(id string, flags map[string]string) {
	reqBody := map[string]string{
		"id":              id,
		"project_id":      flags["project"],
		"conversation_id": flags["conversation"],
		"planner_id":      flags["planner"],
		"executor_id":     flags["executor"],
	}
	data, _ := json.Marshal(reqBody)

	resp, err := http.Post(orchestratordURL+"/api/v1/runs", "application/json", bytes.NewReader(data))
	if err != nil {
		failCLI(flags["json"] == "true", exitCodeInfrastructure, "connecting to orchestratord", err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		failHTTP(flags["json"] == "true", resp.StatusCode, body)
	}

	body, _ := io.ReadAll(resp.Body)
	emitJSONBody(body)
}

func runState(id string, asJSON bool) {
	resp, err := http.Get(orchestratordURL + "/api/v1/runs/" + id)
	if err != nil {
		failCLI(asJSON, exitCodeInfrastructure, "connecting to orchestratord", err.Error())
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		failHTTP(asJSON, resp.StatusCode, body)
	}

	if asJSON {
		emitJSONBody(body)
		return
	}

	var run domain.Run
	if err := json.Unmarshal(body, &run); err != nil {
		printJSON(body)
		return
	}

	fmt.Printf("--- Run State: %s ---\n", run.ID)
	fmt.Printf("State:        %s\n", run.State)
	fmt.Printf("Project:      %s\n", run.ProjectID)
	fmt.Printf("Conversation: %s\n", run.ConversationID)
	fmt.Printf("Planner:      %s\n", run.PlannerID)
	fmt.Printf("Executor:     %s\n", run.ExecutorID)
	fmt.Printf("Created:      %s\n", run.CreatedAt.Format(time.RFC3339))
	fmt.Printf("Updated:      %s\n", run.UpdatedAt.Format(time.RFC3339))
	if run.RecoveryNotes != "" {
		fmt.Printf("Notes:        %s\n", run.RecoveryNotes)
	}
	fmt.Println("---------------------------")
}

func abortRun(id string, asJSON bool) {
	reqBody := map[string]string{
		"action": "abort",
	}
	data, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPatch, orchestratordURL+"/api/v1/runs/"+id, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		failCLI(asJSON, exitCodeInfrastructure, "connecting to orchestratord", err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		failHTTP(asJSON, resp.StatusCode, body)
	}

	respBody := map[string]string{
		"id":     id,
		"action": "abort",
		"status": "success",
	}
	if asJSON {
		emitJSONDocument(respBody)
		return
	}
	fmt.Printf("Run %s aborted successfully\n", id)
}

func listRuns(filters map[string]string) {
	asJSON := filters["json"] == "true"
	u, _ := url.Parse(orchestratordURL + "/api/v1/runs")
	if len(filters) > 0 {
		q := u.Query()
		if v, ok := filters["project"]; ok {
			q.Set("project_id", v)
		}
		if v, ok := filters["conversation"]; ok {
			q.Set("conversation_id", v)
		}
		if v, ok := filters["state"]; ok {
			q.Set("state", v)
		}
		u.RawQuery = q.Encode()
	}

	resp, err := http.Get(u.String())
	if err != nil {
		failCLI(asJSON, exitCodeInfrastructure, "connecting to orchestratord", err.Error())
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		failHTTP(asJSON, resp.StatusCode, body)
	}

	if asJSON {
		emitJSONBody(body)
		return
	}

	var runs []domain.Run
	if err := json.Unmarshal(body, &runs); err == nil {
		fmt.Printf("%-24s %-15s %-15s %-15s %s\n", "ID", "STATE", "PROJECT", "CONVERSATION", "CREATED")
		fmt.Println(strings.Repeat("-", 85))
		for _, r := range runs {
			proj := r.ProjectID
			if len(proj) > 15 {
				proj = proj[:12] + "..."
			}
			conv := r.ConversationID
			if len(conv) > 15 {
				conv = conv[:12] + "..."
			}
			fmt.Printf("%-24s %-15s %-15s %-15s %s\n", r.ID, r.State, proj, conv, r.CreatedAt.Format("2006-01-02 15:04"))
		}
	} else {
		emitJSONBody(body)
	}
}

func parseRunStartFlags(args []string) map[string]string {
	flags := make(map[string]string)
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			if i+1 < len(args) {
				flags["project"] = args[i+1]
				i++
			}
		case "--conversation":
			if i+1 < len(args) {
				flags["conversation"] = args[i+1]
				i++
			}
		case "--planner":
			if i+1 < len(args) {
				flags["planner"] = args[i+1]
				i++
			}
		case "--executor":
			if i+1 < len(args) {
				flags["executor"] = args[i+1]
				i++
			}
		case "--json":
			flags["json"] = "true"
		}
	}
	// Support legacy positional project if not provided via flag
	// positional project is usually args[2] if args[1] is runID
	if flags["project"] == "" && len(args) > 2 && !strings.HasPrefix(args[2], "-") {
		flags["project"] = args[2]
	}
	return flags
}

func parseRunListFilters(args []string) map[string]string {
	filters := make(map[string]string)
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			if i+1 < len(args) {
				filters["project"] = args[i+1]
				i++
			}
		case "--conversation":
			if i+1 < len(args) {
				filters["conversation"] = args[i+1]
				i++
			}
		case "--state":
			if i+1 < len(args) {
				filters["state"] = args[i+1]
				i++
			}
		case "--json":
			filters["json"] = "true"
		}
	}
	return filters
}

func runWait(runID string, interval, timeout time.Duration, asJSON bool) int {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var deadline <-chan time.Time
	if timeout > 0 {
		deadline = time.After(timeout)
	}

	fmt.Fprintf(os.Stderr, "Waiting for run %s (interval: %v, timeout: %v)...", runID, interval, timeout)
	for {
		select {
		case <-deadline:
			fmt.Fprintf(os.Stderr, "\n")
			if asJSON {
				emitJSONDocument(cliErrorPayload{
					Error:   "client_side_timeout",
					Message: fmt.Sprintf("wait exceeded CLI limit of %v", timeout),
				})
			} else {
				fmt.Fprintf(os.Stderr, "wait exceeded CLI limit of %v\n", timeout)
			}
			return exitCodeTimeout
		default:
			resp, err := http.Get(orchestratordURL + "/api/v1/runs/" + runID)
			if err != nil {
				failCLI(asJSON, exitCodeInfrastructure, "connecting to orchestratord", err.Error())
			}

			if resp.StatusCode >= 400 {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				failHTTP(asJSON, resp.StatusCode, body)
			}

			var run domain.Run
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if err := json.Unmarshal(body, &run); err != nil {
				failCLI(asJSON, exitCodeInfrastructure, "parsing response", err.Error())
			}

			if run.State.IsTerminal() || run.State == domain.RunStatePausedForGate {
				fmt.Fprintf(os.Stderr, "\nTerminal/Intervention condition reached for Run %s: %s\n", runID, run.State)

				// Hint at artifact directory if possible
				if run.State == domain.RunStateCompleted {
					fmt.Fprintf(os.Stderr, "Artifacts: .codencer/artifacts/%s\n", runID)
				}

				emitJSONBody(body)
				return exitCodeForRunState(run.State)
			}

			fmt.Fprintf(os.Stderr, ".")
			<-ticker.C
		}
	}
}

func handleStepCommand(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: orchestratorctl step <start|list|state|result|artifacts|validations|wait> [args]")
		os.Exit(exitCodeUsage)
	}

	if len(args) < 2 {
		fmt.Println("Usage: orchestratorctl step <command> [args]")
		os.Exit(exitCodeUsage)
	}

	subCommand := args[1]
	subArgs := args[1:]

	switch subCommand {
	case "start":
		if len(subArgs) < 3 {
			fmt.Println("Usage: orchestratorctl step start <runID> <taskFile.yaml> [--wait] [--json]")
			os.Exit(exitCodeUsage)
		}
		opts := parseStartOptions(subArgs[3:])
		stepStart(subArgs[1], subArgs[2], opts.wait, opts.json)
	case "list":
		if len(subArgs) < 2 {
			fmt.Println("Usage: orchestratorctl step list <runID> [--json]")
			os.Exit(exitCodeUsage)
		}
		listStepsByRun(subArgs[1], hasFlag(subArgs[2:], "--json"))
	case "status", "state":
		if len(subArgs) < 2 {
			fmt.Println("Usage: orchestratorctl step state <stepID> [--json]")
			os.Exit(exitCodeUsage)
		}
		stepState(subArgs[1], hasFlag(subArgs[2:], "--json"))
	case "result":
		if len(subArgs) < 2 {
			fmt.Println("Usage: orchestratorctl step result <stepID> [--json]")
			os.Exit(exitCodeUsage)
		}
		stepResult(subArgs[1], hasFlag(subArgs[2:], "--json"))
	case "logs":
		if len(subArgs) < 2 {
			fmt.Println("Usage: orchestratorctl step logs <stepID> [--json]")
			os.Exit(exitCodeUsage)
		}
		stepLogs(subArgs[1], hasFlag(subArgs[2:], "--json"))
	case "artifacts":
		if len(subArgs) < 2 {
			fmt.Println("Usage: orchestratorctl step artifacts <stepID> [--json]")
			os.Exit(exitCodeUsage)
		}
		stepArtifacts(subArgs[1], hasFlag(subArgs[2:], "--json"))
	case "validations":
		if len(subArgs) < 2 {
			fmt.Println("Usage: orchestratorctl step validations <stepID> [--json]")
			os.Exit(exitCodeUsage)
		}
		stepValidations(subArgs[1], hasFlag(subArgs[2:], "--json"))
	case "wait":
		if len(subArgs) < 2 {
			fmt.Println("Usage: orchestratorctl step wait <id> [--interval <d>] [--timeout <d>] [--json]")
			os.Exit(exitCodeUsage)
		}
		interval, timeout, asJSON := parseWaitFlags(subArgs[2:])
		os.Exit(stepWait(subArgs[1], interval, timeout, asJSON))
	default:
		fmt.Fprintf(os.Stderr, "Unknown step command: %s\n", subCommand)
		os.Exit(exitCodeUsage)
	}
}

func stepStart(runID, taskFile string, shouldWait, asJSON bool) {
	normalized, err := validation.NormalizeTaskInput(validation.NormalizeTaskInputRequest{
		RunID:      runID,
		SourceKind: domain.SubmissionSourceTaskFile,
		SourceName: taskFile,
		Content: func() []byte {
			b, readErr := os.ReadFile(taskFile)
			if readErr != nil {
				failCLI(asJSON, exitCodeUsage, "reading task spec", readErr.Error())
			}
			return b
		}(),
	})
	if err != nil {
		failCLI(asJSON, exitCodeUsage, "parsing task spec", err.Error())
	}
	submitTaskSpec(runID, normalized.Task, shouldWait, asJSON)
}

func listStepsByRun(runID string, asJSON bool) {
	resp, err := http.Get(orchestratordURL + "/api/v1/runs/" + runID + "/steps")
	if err != nil {
		failCLI(asJSON, exitCodeInfrastructure, "connecting to orchestratord", err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		failHTTP(asJSON, resp.StatusCode, body)
	}

	body, _ := io.ReadAll(resp.Body)
	if asJSON {
		emitJSONBody(body)
		return
	}
	var steps []domain.Step
	if err := json.Unmarshal(body, &steps); err == nil {
		fmt.Printf("%-24s %-20s %-20s %-20s\n", "STEP ID", "TITLE", "STATE", "UPDATED")
		fmt.Println(strings.Repeat("-", 85))
		for _, s := range steps {
			title := s.Title
			if len(title) > 18 {
				title = title[:15] + "..."
			}
			state := string(s.State)
			if s.StatusReason != "" && (s.State == domain.StepStateFailedBridge || s.State == domain.StepStateFailedAdapter || s.State == domain.StepStateFailedValidation) {
				// Optionally append (failed) or similar? For now just show original state enum.
			}
			fmt.Printf("%-24s %-20s %-20s %-20s\n", s.ID, title, state, s.UpdatedAt.Format("2006-01-02 15:04"))
		}
	} else {
		emitJSONBody(body)
	}
}

func stepState(stepID string, asJSON bool) {
	resp, err := http.Get(orchestratordURL + "/api/v1/steps/" + stepID)
	if err != nil {
		failCLI(asJSON, exitCodeInfrastructure, "connecting to orchestratord", err.Error())
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		failHTTP(asJSON, resp.StatusCode, body)
	}

	if asJSON {
		emitJSONBody(body)
		return
	}

	var step domain.Step
	if err := json.Unmarshal(body, &step); err == nil {
		fmt.Printf("--- Step State: %s ---\n", step.ID)
		fmt.Printf("State:        %s\n", step.State)
		fmt.Printf("Title:        %s\n", step.Title)
		fmt.Printf("Goal:         %s\n", step.Goal)
		fmt.Printf("Adapter:      %s\n", step.Adapter)
		if step.StatusReason != "" {
			fmt.Printf("Reason:       %s\n", step.StatusReason)
		}
		fmt.Printf("Created:      %s\n", step.CreatedAt.Format(time.RFC3339))
		fmt.Printf("Updated:      %s\n", step.UpdatedAt.Format(time.RFC3339))
		fmt.Println("---------------------------")
	} else {
		emitJSONBody(body)
	}
}

func stepResult(stepID string, asJSON bool) {
	resp, err := http.Get(orchestratordURL + "/api/v1/steps/" + stepID + "/result")
	if err != nil {
		failCLI(asJSON, exitCodeInfrastructure, "connecting to orchestratord", err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		failHTTP(asJSON, resp.StatusCode, body)
	}

	body, _ := io.ReadAll(resp.Body)
	if asJSON {
		emitJSONBody(body)
		return
	}
	var result domain.ResultSpec
	if err := json.Unmarshal(body, &result); err == nil {
		// Fetch Run metadata for Bridge Context (Batch 2/3 Alignment)
		var project, conversation string
		if runResp, err := http.Get(orchestratordURL + "/api/v1/runs/" + result.RunID); err == nil {
			defer runResp.Body.Close()
			var run domain.Run
			if runData, _ := io.ReadAll(runResp.Body); json.Unmarshal(runData, &run) == nil {
				project = run.ProjectID
				conversation = run.ConversationID
			}
		}

		fmt.Printf("--- Authoritative Truth (Summary): %s ---\n", stepID)
		if project != "" {
			fmt.Printf("Bridge Context:  project=%s  conversation=%s\n", project, conversation)
		}
		fmt.Printf("State:   %s\n", result.State)
		fmt.Printf("Summary: %s\n", result.Summary)

		// Provisioning Outcome Visibility
		if result.Provisioning != nil {
			status := "SUCCESS"
			if !result.Provisioning.Success {
				status = "FAILED"
			}
			fmt.Printf("\nWorkspace Provisioning [%s] (%dms):\n", status, result.Provisioning.DurationMs)
			if len(result.Provisioning.EnvironmentFiles) > 0 {
				fmt.Printf("  Files:     %s\n", strings.Join(result.Provisioning.EnvironmentFiles, ", "))
			}
			if len(result.Provisioning.Symlinks) > 0 {
				fmt.Printf("  Symlinks:  %s\n", strings.Join(result.Provisioning.Symlinks, ", "))
			}
			if result.Provisioning.PostCreateHook != "" {
				fmt.Printf("  Hook:      %s (%s)\n", result.Provisioning.PostCreateHook, result.Provisioning.HookStatus)
			}
			if result.Provisioning.Summary != "" && !result.Provisioning.Success {
				fmt.Printf("  Error:     %s\n", result.Provisioning.Summary)
			}
		}

		// Execution Context Visibility (Broker-backed)
		if result.Artifacts != nil {
			taskID := result.Artifacts["broker_task_id"]
			repoRoot := result.Artifacts["broker_repo_root"]
			if taskID != "" || repoRoot != "" {
				fmt.Println("\nExecution Context (Antigravity Broker):")
				if taskID != "" {
					fmt.Printf("  Task ID:   %s\n", taskID)
				}
				if repoRoot != "" {
					fmt.Printf("  Bound Repo: %s\n", repoRoot)
				}
			}
		}

		if len(result.Validations) > 0 {
			fmt.Println("\n--- Validations ---")
			for _, v := range result.Validations {
				status := "[PASS]"
				if !v.Passed {
					status = "[FAIL]"
				}
				fmt.Printf("  %s %-20s %s\n", status, v.Name, v.State)
			}
		}

		fmt.Println("\n[GUIDE] Evidence Drill-down:")
		if result.RawOutputRef != "" {
			fmt.Printf("  Logs:      ./bin/orchestratorctl step logs %s\n", stepID)
			fmt.Printf("  Artifacts: ./bin/orchestratorctl step artifacts %s\n", stepID)
		}
		fmt.Printf("  Validations: ./bin/orchestratorctl step validations %s\n", stepID)
		fmt.Println("---------------------------")
	} else {
		emitJSONBody(body)
	}
}

func stepLogs(stepID string, asJSON bool) {
	if !asJSON {
		fmt.Fprintf(os.Stderr, "Fetching logs for step %s...\n", stepID)
	}
	resp, err := http.Get(orchestratordURL + "/api/v1/steps/" + stepID + "/logs")
	if err != nil {
		failCLI(asJSON, exitCodeInfrastructure, "connecting to orchestratord", err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		if asJSON {
			emitJSONDocument(map[string]any{
				"step_id":      stepID,
				"available":    false,
				"content_type": "text/plain",
				"content":      "",
			})
			return
		}
		fmt.Fprintln(os.Stderr, "No logs available for this step.")
		return
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		failHTTP(asJSON, resp.StatusCode, body)
	}

	body, _ := io.ReadAll(resp.Body)
	if asJSON {
		contentType := resp.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "text/plain"
		}
		emitJSONDocument(map[string]any{
			"step_id":      stepID,
			"available":    true,
			"content_type": contentType,
			"content":      string(body),
		})
		return
	}

	fmt.Fprint(os.Stdout, string(body))
}

func stepArtifacts(stepID string, asJSON bool) {
	resp, err := http.Get(orchestratordURL + "/api/v1/steps/" + stepID + "/artifacts")
	if err != nil {
		failCLI(asJSON, exitCodeInfrastructure, "connecting to orchestratord", err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		failHTTP(asJSON, resp.StatusCode, body)
	}

	body, _ := io.ReadAll(resp.Body)
	if asJSON {
		emitJSONBody(body)
		return
	}
	var artifacts []domain.Artifact
	if err := json.Unmarshal(body, &artifacts); err == nil {
		fmt.Printf("--- Artifacts for Step: %s ---\n", stepID)
		if len(artifacts) == 0 {
			fmt.Println("No artifacts recorded.")
			return
		}

		// Group by AttemptID
		byAttempt := make(map[string][]domain.Artifact)
		var attempts []string
		for _, a := range artifacts {
			if _, ok := byAttempt[a.AttemptID]; !ok {
				attempts = append(attempts, a.AttemptID)
			}
			byAttempt[a.AttemptID] = append(byAttempt[a.AttemptID], a)
		}

		for _, attID := range attempts {
			fmt.Printf("\nAttempt %s:\n", attID)
			arts := byAttempt[attID]
			if len(arts) > 0 {
				fmt.Printf("  Directory: %s\n", filepath.Dir(arts[0].Path))
			}
			for _, a := range arts {
				fmt.Printf("  - [%s] %-20s (%s)\n", a.Type, filepath.Base(a.Path), a.MimeType)
			}
		}
		fmt.Println("\n-------------------------------")
	} else {
		emitJSONBody(body)
	}
}
func stepValidations(stepID string, asJSON bool) {
	resp, err := http.Get(orchestratordURL + "/api/v1/steps/" + stepID + "/validations")
	if err != nil {
		failCLI(asJSON, exitCodeInfrastructure, "connecting to orchestratord", err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		failHTTP(asJSON, resp.StatusCode, body)
	}

	body, _ := io.ReadAll(resp.Body)
	if asJSON {
		emitJSONBody(body)
		return
	}
	var results map[string][]*domain.ValidationResult
	if err := json.Unmarshal(body, &results); err == nil {
		fmt.Printf("--- Validation Summary for Step: %s ---\n", stepID)
		if len(results) == 0 {
			fmt.Println("No validations recorded.")
		}
		for attempt, resList := range results {
			fmt.Printf("\nAttempt %s:\n", attempt)
			for _, v := range resList {
				status := "[PASS]"
				if !v.Passed {
					status = "[FAIL]"
				}
				fmt.Printf("  %-6s %-16s %-12s\n", status, v.Name, v.State)
				if v.Error != "" {
					fmt.Printf("         Error: %s\n", v.Error)
				}
			}
		}
		fmt.Println("\n---------------------------------------")
	} else {
		emitJSONBody(body)
	}
}

func stepWait(stepID string, interval, timeout time.Duration, asJSON bool) int {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var deadline <-chan time.Time
	if timeout > 0 {
		deadline = time.After(timeout)
	}

	fmt.Fprintf(os.Stderr, "Waiting for step %s (interval: %v, timeout: %v)...", stepID, interval, timeout)
	for {
		select {
		case <-deadline:
			fmt.Fprintf(os.Stderr, "\n")
			if asJSON {
				emitJSONDocument(cliErrorPayload{
					Error:   "client_side_timeout",
					Message: fmt.Sprintf("wait exceeded CLI limit of %v", timeout),
				})
			} else {
				fmt.Fprintf(os.Stderr, "wait exceeded CLI limit of %v\n", timeout)
			}
			return exitCodeTimeout
		default:
			resp, err := http.Get(orchestratordURL + "/api/v1/steps/" + stepID + "/result")
			if err != nil {
				failCLI(asJSON, exitCodeInfrastructure, "connecting to orchestratord", err.Error())
			}

			if resp.StatusCode >= 400 {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				failHTTP(asJSON, resp.StatusCode, body)
			}

			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			var result domain.ResultSpec
			if err := json.Unmarshal(body, &result); err != nil {
				failCLI(asJSON, exitCodeInfrastructure, "parsing response", err.Error())
			}

			// Check for terminal or intervention-required states
			if result.State.IsTerminal() || result.State == domain.StepStateNeedsApproval || result.State == domain.StepStateNeedsManualAttention {
				fmt.Fprintf(os.Stderr, "\n[BRIDGE] Mission Handle %s reached terminal condition: %s\n", stepID, result.State)

				switch result.State {
				case domain.StepStateNeedsApproval:
					fmt.Fprintf(os.Stderr, "\n[ACTION REQUIRED] Policy gate hit. Bridge is waiting for approval.\n")
					fmt.Fprintf(os.Stderr, "  To approve: ./bin/orchestratorctl gate approve gate-%s\n", stepID)
					fmt.Fprintf(os.Stderr, "  To reject:  ./bin/orchestratorctl gate reject gate-%s\n", stepID)
				case domain.StepStateFailedTerminal:
					fmt.Fprintf(os.Stderr, "\n[AUDIT REQUIRED] Goal not met (e.g., task failed/unmet). Bridge does not retry automatically.\n")
					fmt.Fprintf(os.Stderr, "  Next Step:  ./bin/orchestratorctl step result %s\n", stepID)
					fmt.Fprintf(os.Stderr, "  Evidence:   ./bin/orchestratorctl step validations %s\n", stepID)
				case domain.StepStateTimeout:
					fmt.Fprintf(os.Stderr, "\n[AUDIT REQUIRED] Execution exceeded time limit. Bridge killed the process.\n")
					fmt.Fprintf(os.Stderr, "  Next Step:  ./bin/orchestratorctl step logs %s\n", stepID)
					fmt.Fprintf(os.Stderr, "  Resolution: Check for loops or increase timeout_seconds in TaskSpec.\n")
				case domain.StepStateNeedsManualAttention:
					fmt.Fprintf(os.Stderr, "\n[SYSTEM HALT] Ambient failure or bridge/agent crash. Control returned to human.\n")
					fmt.Fprintf(os.Stderr, "  Next Step:  Check .codencer/smoke_daemon.log (or daemon log)\n")
					fmt.Fprintf(os.Stderr, "  Evidence:   ./bin/orchestratorctl step logs %s\n", stepID)
				case domain.StepStateCancelled:
					fmt.Fprintf(os.Stderr, "\n[NOTE] Execution was explicitly stopped by operator/mission abort.\n")
				case domain.StepStateFailedRetryable:
					fmt.Fprintf(os.Stderr, "\n[RECOVERY OPPORTUNITY] Transient failure (e.g., rate limit). Safe to retry mission.\n")
					fmt.Fprintf(os.Stderr, "  Submit:     ./bin/orchestratorctl submit <runID> <file> --wait\n")
				}
				fmt.Fprintln(os.Stderr)

				// Hint at artifact directory if possible
				if result.RawOutputRef != "" {
					fmt.Fprintf(os.Stderr, "[DONE] Terminal outcome: %s\n", result.State)
					fmt.Fprintf(os.Stderr, "Summary:   %s\n", result.Summary)
					fmt.Fprintf(os.Stderr, "\n[GUIDE] To view the human-readable summary (Authoritative Truth):\n  ./bin/orchestratorctl step result %s\n", stepID)
					fmt.Fprintf(os.Stderr, "[GUIDE] To drill down into artifacts and validations:\n  ./bin/orchestratorctl step artifacts %s\n  ./bin/orchestratorctl step validations %s\n", stepID, stepID)
				}

				emitJSONBody(body)
				return exitCodeForStepState(result.State)
			}

			fmt.Fprintf(os.Stderr, ".")
			<-ticker.C
		}
	}
}

func runDoctor() {
	fmt.Println("==> Verifying local environment...")

	// 1. Check .env presence
	if _, err := os.Stat(".env"); os.IsNotExist(err) {
		fmt.Println("[WARN]  .env file missing. Run 'cp .env.example .env'")
	} else {
		fmt.Println("[OK]    .env file found")
	}

	// 2. Check .codencer presence and parity
	if _, err := os.Stat(".codencer"); os.IsNotExist(err) {
		fmt.Println("[WARN]  .codencer directory missing. Run 'make setup'")
	} else {
		fmt.Println("[OK]    .codencer directory found")
		// Check write permission
		tmpFile := filepath.Join(".codencer", ".doctor_tmp")
		if err := os.WriteFile(tmpFile, []byte("ok"), 0644); err != nil {
			fmt.Printf("[ERROR] .codencer NOT writable: %v\n", err)
		} else {
			fmt.Println("[OK]    .codencer is writable")
			os.Remove(tmpFile)
		}
	}

	// 3. Check Critical Runtime Binaries (git, go, c-compiler, curl)
	bins := []struct {
		name string
		cmd  string
		arg  string
		req  bool
	}{
		{"Git", "git", "--version", true},
		{"Go", "go", "version", true},
		{"C Compiler (for SQLite CGO)", "cc", "--version", true},
		{"curl (for Makefile polling)", "curl", "--version", true},
	}

	for _, b := range bins {
		out, err := exec.Command(b.cmd, b.arg).Output()
		if err != nil {
			if b.req {
				fmt.Printf("[ERROR] %s NOT found or failed to report version. Please install %s.\n", b.name, b.name)
			} else {
				fmt.Printf("[INFO]  %s NOT found (Optional, unless building CGO bridge).\n", b.name)
			}
		} else {
			outStr := strings.TrimSpace(string(out))
			if len(outStr) > 40 {
				outStr = outStr[:37] + "..."
			}
			fmt.Printf("[OK]    %s detected: %s\n", b.name, outStr)
		}
	}

	// 4. Check Daemon connectivity
	resp, err := http.Get(orchestratordURL + "/api/v1/compatibility")
	if err != nil {
		fmt.Printf("[INFO]  Daemon unreachable at %s (ignore if daemon is stopped)\n", orchestratordURL)
	} else {
		defer resp.Body.Close()
		fmt.Printf("[OK]    Daemon reachable at %s\n", orchestratordURL)
	}

	// 5. Check Execution Mode
	simMode := os.Getenv("ALL_ADAPTERS_SIMULATION_MODE")
	if simMode == "1" || simMode == "true" {
		fmt.Println("[INFO]  Execution Mode: SIMULATION (Bridge only)")
	} else {
		fmt.Println("[INFO]  Execution Mode: REAL (Requires agent binaries)")
	}

	// 6. Check Agent Binaries
	fmt.Println("\nChecking adapter binaries...")
	adapters := []struct {
		name string
		bin  string
		env  string
	}{
		{"Codex", "codex-agent", "CODEX_BINARY"},
		{"Claude", "claude-code", "CLAUDE_BINARY"},
		{"Qwen", "qwen-local", "QWEN_BINARY"},
		{"OpenClaw", "acpx", "OPENCLAW_ACPX_BINARY"},
	}

	for _, a := range adapters {
		path := os.Getenv(a.env)
		if path == "" {
			path = a.bin
		}

		found := false
		if filepath.IsAbs(path) {
			if _, err := os.Stat(path); err == nil {
				found = true
			}
		} else {
			if _, err := exec.LookPath(path); err == nil {
				found = true
			}
		}

		if !found {
			fmt.Printf("[INFO]  %s binary (%s) NOT in PATH or %s\n", a.name, path, a.env)
		} else {
			// Optional: check version
			vOut, vErr := exec.Command(path, "--help").CombinedOutput()
			if vErr == nil && len(vOut) > 0 {
				fmt.Printf("[OK]    %s binary detected\n", a.name)
			} else {
				fmt.Printf("[OK]    %s binary detected (path: %s)\n", a.name, path)
			}
		}
	}

	fmt.Println("\nConfiguration verified. Use 'make smoke' for full relay validation.")
}

func handleGateCmd(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: orchestratorctl gate <approve|reject> <id> [--json]")
		os.Exit(exitCodeUsage)
	}

	action := args[0]
	id := args[1]
	asJSON := hasFlag(args[2:], "--json")

	reqBody := map[string]string{"action": action}
	data, _ := json.Marshal(reqBody)

	resp, err := http.Post(orchestratordURL+"/api/v1/gates/"+id, "application/json", bytes.NewReader(data))
	if err != nil {
		failCLI(asJSON, exitCodeInfrastructure, "connecting to orchestratord", err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		failHTTP(asJSON, resp.StatusCode, body)
	}
	if asJSON {
		emitJSONDocument(map[string]string{
			"id":     id,
			"action": action,
			"status": "success",
		})
		return
	}
	fmt.Printf("Gate %s processed successfully\n", id)
}

func handleAntigravityCmd(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: orchestratorctl antigravity <command>")
		fmt.Println("Commands: list, bind, unbind, status")
		os.Exit(exitCodeUsage)
	}

	cmd := args[0]
	switch cmd {
	case "list":
		asJSON := hasFlag(args[1:], "--json")
		resp, err := http.Get(orchestratordURL + "/api/v1/antigravity/instances")
		if err != nil {
			failCLI(asJSON, exitCodeInfrastructure, "connecting to orchestratord", err.Error())
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 400 {
			failHTTP(asJSON, resp.StatusCode, body)
		}

		if asJSON {
			emitJSONBody(body)
			return
		}

		var instances []domain.AGInstance
		if err := json.Unmarshal(body, &instances); err != nil {
			emitJSONBody(body)
			return
		}

		fmt.Printf("%-8s %-8s %-12s %-20s\n", "PID", "PORT", "REACHABLE", "WORKSPACE")
		fmt.Println(strings.Repeat("-", 60))
		for _, inst := range instances {
			reachable := "no"
			if inst.IsReachable {
				reachable = "yes"
			}
			fmt.Printf("%-8d %-8d %-12s %-20s\n", inst.PID, inst.HTTPSPort, reachable, inst.WorkspaceRoot)
		}

	case "bind":
		if len(args) < 2 || args[1] == "--help" || args[1] == "-h" {
			fmt.Println("Usage: orchestratorctl antigravity bind <PID>")
			fmt.Println("\nBinding records a repo-scoped Antigravity target identity for this daemon.")
			fmt.Println("Execution still depends on explicit adapter selection via adapter_profile.")
			os.Exit(exitCodeUsage)
		}
		asJSON := hasFlag(args[2:], "--json")
		pid, err := strconv.Atoi(args[1])
		if err != nil {
			failCLI(asJSON, exitCodeUsage, "invalid pid", err.Error())
		}

		reqBody := map[string]int{"pid": pid}
		data, _ := json.Marshal(reqBody)
		resp, err := http.Post(orchestratordURL+"/api/v1/antigravity/bind", "application/json", bytes.NewReader(data))
		if err != nil {
			failCLI(asJSON, exitCodeInfrastructure, "connecting to orchestratord", err.Error())
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			body, _ := io.ReadAll(resp.Body)
			failHTTP(asJSON, resp.StatusCode, body)
		}
		if asJSON {
			emitJSONDocument(map[string]any{
				"status": "success",
				"pid":    pid,
			})
			return
		}
		fmt.Printf("Successfully bound repo to Antigravity PID %d\n", pid)

	case "unbind":
		asJSON := hasFlag(args[1:], "--json")
		req, _ := http.NewRequest(http.MethodDelete, orchestratordURL+"/api/v1/antigravity/bind", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			failCLI(asJSON, exitCodeInfrastructure, "connecting to orchestratord", err.Error())
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			body, _ := io.ReadAll(resp.Body)
			failHTTP(asJSON, resp.StatusCode, body)
		}
		if asJSON {
			emitJSONDocument(map[string]string{
				"status": "success",
				"action": "unbind",
			})
			return
		}
		fmt.Println("Successfully unbound repo from Antigravity")

	case "status":
		asJSON := hasFlag(args[1:], "--json")
		resp, err := http.Get(orchestratordURL + "/api/v1/antigravity/status")
		if err != nil {
			failCLI(asJSON, exitCodeInfrastructure, "connecting to orchestratord", err.Error())
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 400 {
			failHTTP(asJSON, resp.StatusCode, body)
		}

		if asJSON {
			emitJSONBody(body)
			return
		}

		if string(body) == "null" {
			fmt.Println("No Antigravity instance currently bound to this repo.")
			return
		}

		var inst domain.AGInstance
		if err := json.Unmarshal(body, &inst); err != nil {
			emitJSONBody(body)
			return
		}

		fmt.Printf("Status:      %s\n", func() string {
			if inst.IsReachable {
				return "BOUND (Active)"
			}
			return "STALE (Process not reachable)"
		}())
		fmt.Printf("PID:         %d\n", inst.PID)
		if inst.IsReachable {
			fmt.Printf("Port:        %d\n", inst.HTTPSPort)
			fmt.Printf("Workspace:   %s\n", inst.WorkspaceRoot)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		os.Exit(exitCodeUsage)
	}
}

func handleInstanceCommand(args []string) {
	asJSON := false
	for _, arg := range args {
		if arg == "--json" {
			asJSON = true
		}
	}

	resp, err := http.Get(orchestratordURL + "/api/v1/instance")
	if err != nil {
		failCLI(asJSON, exitCodeInfrastructure, "connecting to orchestratord", err.Error())
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		failHTTP(asJSON, resp.StatusCode, body)
	}

	if asJSON {
		emitJSONBody(body)
		return
	}

	var info domain.InstanceInfo
	if err := json.Unmarshal(body, &info); err != nil {
		emitJSONBody(body)
		return
	}

	fmt.Printf("--- Codencer Instance Identity ---\n")
	fmt.Printf("Version:       %s\n", info.Version)
	fmt.Printf("Repo Root:     %s\n", info.RepoRoot)
	fmt.Printf("Base URL:      %s\n", info.BaseURL)
	fmt.Printf("PID:           %d\n", info.PID)
	fmt.Printf("Started At:    %s\n", info.StartedAt.Format(time.RFC3339))
	fmt.Printf("Execution:     %s\n", info.ExecutionMode)
	fmt.Printf("State Dir:     %s\n", info.StateDir)
	fmt.Printf("Workspace:     %s\n", info.WorkspaceRoot)
	fmt.Println("----------------------------------")
}
