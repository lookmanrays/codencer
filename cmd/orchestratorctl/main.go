package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"agent-bridge/internal/app"
	"agent-bridge/internal/domain"
	"agent-bridge/internal/validation"
)

var (
	orchestratordURL = "http://127.0.0.1:8080"
)

func main() {
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
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: orchestratorctl <command> [args]")
	fmt.Println("Commands:")
	fmt.Println("  version        Show version")
	fmt.Println("  run start      <id> <project_id>")
	fmt.Println("  run status     <id>")
	fmt.Println("  run abort      <id>")
	fmt.Println("  step start     <runID> <taskFile.yaml>")
	fmt.Println("  submit        <runID> <taskFile.yaml> (Synonym for step start)")
	fmt.Println("  step status    <stepID>")
	fmt.Println("  step result    <stepID>")
	fmt.Println("  step artifacts <stepID>")
	fmt.Println("  step validations <stepID>")
	fmt.Println("  step wait      <stepID>")
	fmt.Println("  gate approve    <id>")
	fmt.Println("  gate reject    <id>")
}

func handleRunCommand(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: orchestratorctl run <start|status|abort> [args]")
		os.Exit(1)
	}

	cmd := args[0]
	switch cmd {
	case "start":
		if len(args) < 3 {
			fmt.Println("Usage: orchestratorctl run start <id> <project_id>")
			os.Exit(1)
		}
		startRun(args[1], args[2])
	case "status":
		if len(args) < 2 {
			fmt.Println("Usage: orchestratorctl run status <id>")
			os.Exit(1)
		}
		runStatus(args[1])
	case "abort":
		if len(args) < 2 {
			fmt.Println("Usage: orchestratorctl run abort <id>")
			os.Exit(1)
		}
		abortRun(args[1])
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
		fmt.Println(string(body))
		os.Exit(1)
	}

	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))
}

func runStatus(id string) {
	resp, err := http.Get(orchestratordURL + "/api/v1/runs/" + id)
	if err != nil {
		fmt.Printf(`{"error": "connecting to orchestratord: %v"}`+"\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Println(string(body))
		os.Exit(1)
	}

	body, _ := io.ReadAll(resp.Body)
	// Output raw JSON response for machine readability
	fmt.Println(string(body))
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
		fmt.Println(string(body))
		os.Exit(1)
	}

	respBody := map[string]string{
		"id":     id,
		"action": "abort",
		"status": "success",
	}
	out, _ := json.Marshal(respBody)
	fmt.Println(string(out))
}

func handleStepCommand(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: orchestratorctl step <start|status|result|artifacts|validations|wait> [args]")
		os.Exit(1)
	}

	cmd := args[0]
	// Handle 'submit <runID> <file>' case
	if cmd == "submit" {
		if len(args) < 3 {
			fmt.Println("Usage: orchestratorctl submit <runID> <taskFile.yaml>")
			os.Exit(1)
		}
		startStep(args[1], args[2])
		return
	}

	// Handle 'step <subcommand>' case
	subArgs := args[1:]
	if len(subArgs) < 1 {
		fmt.Println("Usage: orchestratorctl step <start|status|result|artifacts|validations|wait> [args]")
		os.Exit(1)
	}
	switch subArgs[0] {
	case "start":
		if len(subArgs) < 3 {
			fmt.Println("Usage: orchestratorctl step start <runID> <taskFile.yaml>")
			os.Exit(1)
		}
		startStep(subArgs[1], subArgs[2])
	case "status":
		if len(args) < 2 {
			fmt.Println("Usage: orchestratorctl step status <stepID>")
			os.Exit(1)
		}
		stepStatus(args[1])
	case "result":
		if len(args) < 2 {
			fmt.Println("Usage: orchestratorctl step result <stepID>")
			os.Exit(1)
		}
		stepResult(args[1])
	case "artifacts":
		if len(args) < 2 {
			fmt.Println("Usage: orchestratorctl step artifacts <stepID>")
			os.Exit(1)
		}
		stepArtifacts(args[1])
	case "validations":
		if len(args) < 2 {
			fmt.Println("Usage: orchestratorctl step validations <stepID>")
			os.Exit(1)
		}
		stepValidations(args[1])
	case "wait":
		if len(args) < 2 {
			fmt.Println("Usage: orchestratorctl step wait <stepID>")
			os.Exit(1)
		}
		stepWait(args[1])
	default:
		fmt.Printf("Unknown step command: %s\n", cmd)
		os.Exit(1)
	}
}

func startStep(runID, taskFile string) {
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
		fmt.Println(string(body))
		os.Exit(1)
	}

	// Output raw JSON response for machine readability
	fmt.Println(string(body))
}

func stepStatus(stepID string) {
	resp, err := http.Get(orchestratordURL + "/api/v1/steps/" + stepID)
	if err != nil {
		fmt.Printf(`{"error": "connecting to orchestratord: %v"}`+"\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Println(string(body))
		os.Exit(1)
	}

	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))
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
		fmt.Println(string(body))
		os.Exit(1)
	}

	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))
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
		fmt.Println(string(body))
		os.Exit(1)
	}

	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))
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
		fmt.Println(string(body))
		os.Exit(1)
	}

	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))
}

func stepWait(stepID string) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		resp, err := http.Get(orchestratordURL + "/api/v1/steps/" + stepID + "/result")
		if err != nil {
			fmt.Printf(`{"error": "connecting to orchestratord: %v"}`+"\n", err)
			os.Exit(1)
		}

		if resp.StatusCode >= 400 {
			body, _ := io.ReadAll(resp.Body)
			fmt.Println(string(body))
			resp.Body.Close()
			os.Exit(1)
		}

		var result struct {
			State string `json:"state"`
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err := json.Unmarshal(body, &result); err != nil {
			fmt.Printf(`{"error": "parsing response: %v"}`+"\n", err)
			os.Exit(1)
		}

		// Check for terminal or intervention-required states
		st := domain.StepState(result.State)
		if st.IsTerminal() || st == domain.StepStateNeedsApproval || st == domain.StepStateNeedsManualAttention {
			fmt.Println(string(body))
			return
		}

		<-ticker.C
	}
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
		fmt.Println(string(body))
		os.Exit(1)
	}

	respBody := map[string]string{
		"id":     id,
		"action": cmd,
		"status": "success",
	}
	out, _ := json.Marshal(respBody)
	fmt.Println(string(out))
}
