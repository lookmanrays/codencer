package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"agent-bridge/internal/app"
	"agent-bridge/internal/domain"
	"agent-bridge/internal/validation"
	"github.com/joho/godotenv"
)

var (
	orchestratordURL = "http://127.0.0.1:8085"
)

func main() {
	_ = godotenv.Load(".env")
	
	if env := os.Getenv("ORCHESTRATORD_URL"); env != "" {
		orchestratordURL = env
	} else if env := os.Getenv("PORT"); env != "" {
		orchestratordURL = fmt.Sprintf("http://127.0.0.1:%s", env)
	}

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	switch command {
	case "version":
		fmt.Printf("orchestratorctl version %s\n", app.Version)
	case "run":
		handleRunCommand(os.Args[2:])
	case "step", "submit":
		handleStepCommand(os.Args[1:])
	case "gate":
		handleGateCommand(os.Args[2:])
	case "doctor":
		runDoctor()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: orchestratorctl <command> [args]")
	fmt.Println("\n1. Session & Mission Management:")
	fmt.Println("  run start      [id] [project]  Initialize a new mission (System of Record)")
	fmt.Println("  run list                      Show mission history")
	fmt.Println("  run state      <runID>         Check mission lifecycle state")
	fmt.Println("  run abort      <runID>         Halt an active mission")
	
	fmt.Println("\n2. Tactical Execution (The Bridge):")
	fmt.Println("  submit         <runID> <file>  Execute TaskSpec (returns UUID Handle)")
	fmt.Println("  submit --wait  <runID> <file>  Execute and poll until terminal state")
	fmt.Println("  step list      <runID>         List all task handles in a mission")
	fmt.Println("  step wait      <handle>        Poll a specific UUID until completion")
	
	fmt.Println("\n3. Evidence & Inspection (The Truth):")
	fmt.Println("  step result    <handle>        Authoritative Truth (Summary)")
	fmt.Println("  step logs      <handle>        Raw agent stdout/stderr trail")
	fmt.Println("  step artifacts <handle>        List harvested files and diffs")
	fmt.Println("  step validations <handle>      Check specific test/lint results")
	
	fmt.Println("\n4. Maintenance & Health:")
	fmt.Println("  doctor                        Verify local environment/binaries")
	fmt.Println("  gate approve   <gateID>        Approve a paused policy gate")
	fmt.Println("  gate reject    <gateID>        Reject a paused policy gate")
	fmt.Println("  version                       Show version")
}

func handleRunCommand(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: orchestratorctl run <start|list|state|abort|wait> [args]")
		os.Exit(1)
	}

	cmd := args[0]
	switch cmd {
	case "start":
		id := ""
		projectID := ""
		if len(args) > 1 {
			id = args[1]
		}
		if len(args) > 2 {
			projectID = args[2]
		}
		startRun(id, projectID)
	case "list":
		listRuns()
	case "status", "state":
		if len(args) < 2 {
			fmt.Println("Usage: orchestratorctl run state <runID>")
			os.Exit(1)
		}
		runState(args[1])
	case "abort":
		if len(args) < 2 {
			fmt.Println("Usage: orchestratorctl run abort <id>")
			os.Exit(1)
		}
		abortRun(args[1])
	case "wait":
		if len(args) < 2 {
			fmt.Println("Usage: orchestratorctl run wait <id> [--interval <d>] [--timeout <d>]")
			os.Exit(1)
		}
		interval, timeout := parseWaitFlags(args[2:])
		runWait(args[1], interval, timeout)
	default:
		fmt.Printf("Unknown run command: %s\n", cmd)
		os.Exit(1)
	}
}

func startRun(id, projectID string) {
	reqBody := map[string]string{
		"id":         id,
		"project_id": projectID,
	}
	data, _ := json.Marshal(reqBody)

	resp, err := http.Post(orchestratordURL+"/api/v1/runs", "application/json", bytes.NewReader(data))
	if err != nil {
		fmt.Printf(`{"error": "connecting to orchestratord: %v"}`+"\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		printJSON(body)
		os.Exit(1)
	}

	body, _ := io.ReadAll(resp.Body)
	printJSON(body)
}

func runState(id string) {
	resp, err := http.Get(orchestratordURL + "/api/v1/runs/" + id)
	if err != nil {
		fmt.Printf(`{"error": "connecting to orchestratord: %v"}`+"\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		printJSON(body)
		os.Exit(1)
	}

	body, _ := io.ReadAll(resp.Body)
	// Output raw JSON response for machine readability
	printJSON(body)
}

func abortRun(id string) {
	reqBody := map[string]string{
		"action": "abort",
	}
	data, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPatch, orchestratordURL+"/api/v1/runs/"+id, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf(`{"error": "connecting to orchestratord: %v"}`+"\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		printJSON(body)
		os.Exit(1)
	}

	respBody := map[string]string{
		"id":     id,
		"action": "abort",
		"status": "success",
	}
	out, _ := json.Marshal(respBody)
	printJSON(out)
}

func listRuns() {
	resp, err := http.Get(orchestratordURL + "/api/v1/runs")
	if err != nil {
		fmt.Printf(`{"error": "connecting to orchestratord: %v"}`+"\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		printJSON(body)
		os.Exit(1)
	}

	body, _ := io.ReadAll(resp.Body)
	var runs []domain.Run
	if err := json.Unmarshal(body, &runs); err == nil {
		fmt.Printf("%-24s %-20s %-15s %-20s\n", "RUN ID", "PROJECT", "STATE", "CREATED")
		fmt.Println("--------------------------------------------------------------------------------")
		for _, r := range runs {
			fmt.Printf("%-24s %-20s %-15s %-20s\n", r.ID, r.ProjectID, r.State, r.CreatedAt.Format("2006-01-02 15:04"))
		}
	} else {
		printJSON(body)
	}
}

func runWait(runID string, interval, timeout time.Duration) {
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
			fmt.Printf(`{"error": "client_side_timeout", "message": "wait exceeded CLI limit of %v"}`+"\n", timeout)
			os.Exit(1)
		default:
			resp, err := http.Get(orchestratordURL + "/api/v1/runs/" + runID)
			if err != nil {
				fmt.Printf(`{"error": "connecting to orchestratord: %v"}`+"\n", err)
				os.Exit(1)
			}

			if resp.StatusCode >= 400 {
				body, _ := io.ReadAll(resp.Body)
				printJSON(body)
				resp.Body.Close()
				os.Exit(1)
			}

			var run domain.Run
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if err := json.Unmarshal(body, &run); err != nil {
				fmt.Printf(`{"error": "parsing response: %v"}`+"\n", err)
				os.Exit(1)
			}

			if run.State.IsTerminal() || run.State == domain.RunStatePausedForGate {
				fmt.Fprintf(os.Stderr, "\nTerminal/Intervention condition reached for Run %s: %s\n", runID, run.State)
				
				// Hint at artifact directory if possible
				if run.State == domain.RunStateCompleted {
					fmt.Fprintf(os.Stderr, "Artifacts: .codencer/artifacts/%s\n", runID)
				}
				
				printJSON(body)
				return
			}

			fmt.Fprintf(os.Stderr, ".")
			<-ticker.C
		}
	}
}

func handleStepCommand(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: orchestratorctl step <start|list|state|result|artifacts|validations|wait> [args]")
		os.Exit(1)
	}

	// args[0] is "step" or "submit"
	if len(args) < 2 {
		if args[0] == "submit" {
			fmt.Println("Usage: orchestratorctl submit <runID> <taskFile.yaml>")
		} else {
			fmt.Println("Usage: orchestratorctl step <command> [args]")
		}
		os.Exit(1)
	}

	subCommand := args[1]
	subArgs := args[1:]

	// Alias 'submit <runID> <taskFile.yaml>' to 'step start <runID> <taskFile.yaml>'
	if args[0] == "submit" {
		subCommand = "start"
		subArgs = args
	}

	switch subCommand {
	case "start":
		if len(subArgs) < 3 {
			if args[0] == "submit" {
				fmt.Println("Usage: orchestratorctl submit <runID> <taskFile.yaml>")
			} else {
				fmt.Println("Usage: orchestratorctl step start <runID> <taskFile.yaml>")
			}
			os.Exit(1)
		}
		shouldWait := false
		if len(subArgs) > 3 && subArgs[3] == "--wait" {
			shouldWait = true
		}
		stepStart(subArgs[1], subArgs[2], shouldWait)
	case "list":
		if len(subArgs) < 2 {
			fmt.Println("Usage: orchestratorctl step list <runID>")
			os.Exit(1)
		}
		listStepsByRun(subArgs[1])
	case "status", "state":
		if len(subArgs) < 2 {
			fmt.Println("Usage: orchestratorctl step state <stepID>")
			os.Exit(1)
		}
		stepState(subArgs[1])
	case "result":
		if len(subArgs) < 2 {
			fmt.Println("Usage: orchestratorctl step result <stepID>")
			os.Exit(1)
		}
		stepResult(subArgs[1])
	case "logs":
		if len(subArgs) < 2 {
			fmt.Println("Usage: orchestratorctl step logs <stepID>")
			os.Exit(1)
		}
		stepLogs(subArgs[1])
	case "artifacts":
		if len(subArgs) < 2 {
			fmt.Println("Usage: orchestratorctl step artifacts <stepID>")
			os.Exit(1)
		}
		stepArtifacts(subArgs[1])
	case "validations":
		if len(subArgs) < 2 {
			fmt.Println("Usage: orchestratorctl step validations <stepID>")
			os.Exit(1)
		}
		stepValidations(subArgs[1])
	case "wait":
		if len(subArgs) < 2 {
			fmt.Println("Usage: orchestratorctl step wait <id> [--interval <d>] [--timeout <d>]")
			os.Exit(1)
		}
		interval, timeout := parseWaitFlags(subArgs[2:])
		stepWait(subArgs[1], interval, timeout)
	default:
		fmt.Printf("Unknown step command: %s\n", subCommand)
		os.Exit(1)
	}
}

func stepStart(runID, taskFile string, shouldWait bool) {
	spec, err := validation.ParseTaskSpec(taskFile)
	if err != nil {
		fmt.Printf(`{"error": "parsing task spec: %v"}`+"\n", err)
		os.Exit(1)
	}

	data, _ := json.Marshal(spec)

	resp, err := http.Post(orchestratordURL+"/api/v1/runs/"+runID+"/steps", "application/json", bytes.NewReader(data))
	if err != nil {
		fmt.Printf(`{"error": "connecting to orchestratord: %v"}`+"\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		printJSON(body)
		os.Exit(1)
	}

	var step domain.Step
	if err := json.Unmarshal(body, &step); err != nil {
		printJSON(body)
		return
	}

	if shouldWait {
		// Output the initial step body so the user has the ID if the wait fails
		printJSON(body)
		fmt.Fprintf(os.Stderr, "==> Auto-waiting for Step %s...\n", step.ID)
		stepWait(step.ID, 2*time.Second, 0)
		return
	}

	// Output raw JSON response for machine readability
	printJSON(body)
	fmt.Fprintf(os.Stderr, "\n[SUCCESS] Unified Step UUID: %s\n", step.ID)
	fmt.Fprintf(os.Stderr, "[GUIDE] To monitor transition:\n  ./bin/orchestratorctl step wait %s\n", step.ID)
	fmt.Fprintf(os.Stderr, "[GUIDE] To view total audit trail:\n  ./bin/orchestratorctl step result %s\n", step.ID)
}

func listStepsByRun(runID string) {
	resp, err := http.Get(orchestratordURL + "/api/v1/runs/" + runID + "/steps")
	if err != nil {
		fmt.Printf(`{"error": "connecting to orchestratord: %v"}`+"\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		printJSON(body)
		os.Exit(1)
	}

	body, _ := io.ReadAll(resp.Body)
	var steps []domain.Step
	if err := json.Unmarshal(body, &steps); err == nil {
		fmt.Printf("%-24s %-20s %-10s %-20s\n", "STEP ID", "TITLE", "STATE", "UPDATED")
		fmt.Println("--------------------------------------------------------------------------------")
		for _, s := range steps {
			title := s.Title
			if len(title) > 18 {
				title = title[:15] + "..."
			}
			fmt.Printf("%-24s %-20s %-10s %-20s\n", s.ID, title, s.State, s.UpdatedAt.Format("2006-01-02 15:04"))
		}
	} else {
		printJSON(body)
	}
}

func stepState(stepID string) {
	resp, err := http.Get(orchestratordURL + "/api/v1/steps/" + stepID)
	if err != nil {
		fmt.Printf(`{"error": "connecting to orchestratord: %v"}`+"\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		printJSON(body)
		os.Exit(1)
	}

	body, _ := io.ReadAll(resp.Body)
	printJSON(body)
}

func stepResult(stepID string) {
	resp, err := http.Get(orchestratordURL + "/api/v1/steps/" + stepID + "/result")
	if err != nil {
		fmt.Printf(`{"error": "connecting to orchestratord: %v"}`+"\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		printJSON(body)
		os.Exit(1)
	}

	body, _ := io.ReadAll(resp.Body)
	var result domain.ResultSpec
	if err := json.Unmarshal(body, &result); err == nil {
		fmt.Printf("--- Authoritative Truth (Summary): %s ---\n", stepID)
		fmt.Printf("State:   %s\n", result.State)
		fmt.Printf("Summary: %s\n", result.Summary)
		fmt.Println("\n[GUIDE] Evidence Drill-down:")
		if result.RawOutputRef != "" {
			fmt.Printf("  Logs:      ./bin/orchestratorctl step logs %s\n", stepID)
			fmt.Printf("  Artifacts: ./bin/orchestratorctl step artifacts %s\n", stepID)
		}
		fmt.Printf("  Validations: ./bin/orchestratorctl step validations %s\n", stepID)
		fmt.Println("---------------------------")
	} else {
		printJSON(body)
	}
}

func stepLogs(stepID string) {
	fmt.Printf("Fetching logs for step %s...\n", stepID)
	resp, err := http.Get(orchestratordURL + "/api/v1/steps/" + stepID + "/logs")
	if err != nil {
		fmt.Printf(`{"error": "connecting to orchestratord: %v"}`+"\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		fmt.Println("No logs available for this step.")
		return
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf(`{"error": "fetching logs", "details": %s}`+"\n", string(body))
		os.Exit(1)
	}

	_, _ = io.Copy(os.Stdout, resp.Body)
}

func stepArtifacts(stepID string) {
	resp, err := http.Get(orchestratordURL + "/api/v1/steps/" + stepID + "/artifacts")
	if err != nil {
		fmt.Printf(`{"error": "connecting to orchestratord: %v"}`+"\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		printJSON(body)
		os.Exit(1)
	}

	body, _ := io.ReadAll(resp.Body)
	var artifacts []domain.Artifact
	if err := json.Unmarshal(body, &artifacts); err == nil {
		fmt.Printf("--- Artifacts for Step: %s ---\n", stepID)
		if len(artifacts) > 0 {
			fmt.Printf("Directory: %s\n\n", filepath.Dir(artifacts[0].Path))
		}
		if len(artifacts) == 0 {
			fmt.Println("No artifacts recorded.")
		}
		for _, a := range artifacts {
			fmt.Printf("- [%s] %s (%s)\n", a.Type, a.Path, a.MimeType)
		}
		fmt.Println("-------------------------------")
	} else {
		printJSON(body)
	}
}
func stepValidations(stepID string) {
	resp, err := http.Get(orchestratordURL + "/api/v1/steps/" + stepID + "/validations")
	if err != nil {
		fmt.Printf(`{"error": "connecting to orchestratord: %v"}`+"\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		printJSON(body)
		os.Exit(1)
	}

	body, _ := io.ReadAll(resp.Body)
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
				msg := v.State
				if v.Error != "" {
					msg = domain.ValidationState(v.Error)
				}
				fmt.Printf("  %s %s: %s\n", status, v.Name, msg)
			}
		}
		fmt.Println("\n---------------------------------------")
	} else {
		printJSON(body)
	}
}

func stepWait(stepID string, interval, timeout time.Duration) {
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
			fmt.Printf(`{"error": "client_side_timeout", "message": "wait exceeded CLI limit of %v"}`+"\n", timeout)
			os.Exit(1)
		default:
			resp, err := http.Get(orchestratordURL + "/api/v1/steps/" + stepID + "/result")
			if err != nil {
				fmt.Printf(`{"error": "connecting to orchestratord: %v"}`+"\n", err)
				os.Exit(1)
			}

			if resp.StatusCode >= 400 {
				body, _ := io.ReadAll(resp.Body)
				printJSON(body)
				resp.Body.Close()
				os.Exit(1)
			}

			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			var result domain.ResultSpec
			if err := json.Unmarshal(body, &result); err != nil {
				fmt.Printf(`{"error": "parsing response: %v"}`+"\n", err)
				os.Exit(1)
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
				
				printJSON(body)
				return
			}

			fmt.Fprintf(os.Stderr, ".")
			<-ticker.C
		}
	}
}

func parseWaitFlags(args []string) (interval, timeout time.Duration) {
	interval = 2 * time.Second
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--interval":
			if i+1 < len(args) {
				d, err := time.ParseDuration(args[i+1])
				if err == nil {
					interval = d
					i++
				}
			}
		case "--timeout":
			if i+1 < len(args) {
				d, err := time.ParseDuration(args[i+1])
				if err == nil {
					timeout = d
					i++
				}
			}
		}
	}
	return
}

func handleGateCommand(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: orchestratorctl gate <approve|reject> <id>")
		os.Exit(1)
	}

	cmd := args[0]
	id := args[1]
	
	reqBody := map[string]string{"action": cmd}
	data, _ := json.Marshal(reqBody)

	resp, err := http.Post(orchestratordURL+"/api/v1/gates/"+id, "application/json", bytes.NewReader(data))
	if err != nil {
		fmt.Printf(`{"error": "connecting to orchestratord: %v"}`+"\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		printJSON(body)
		os.Exit(1)
	}

	respBody := map[string]string{
		"id":     id,
		"action": cmd,
		"status": "success",
	}
	out, _ := json.Marshal(respBody)
	printJSON(out)
	fmt.Fprintf(os.Stderr, "==> Gate %s: %sed successfully.\n", id, cmd)
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

	// 3. Check Critical Runtime Binaries (git, go, c-compiler)
	bins := []struct {
		name string
		cmd  string
		arg  string
		req  bool
	}{
		{"Git", "git", "--version", true},
		{"Go", "go", "version", true},
		{"C Compiler (for SQLite CGO)", "cc", "--version", true},
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
		{"Qwen/Aider", "aider", "AIDER_BINARY"},
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

func printJSON(body []byte) {
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, body, "", "  "); err == nil {
		fmt.Println(pretty.String())
	} else {
		fmt.Println(string(body))
	}
}
