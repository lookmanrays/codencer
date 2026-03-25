package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"agent-bridge/internal/app"
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
	case "step":
		handleStepCommand(os.Args[2:])
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
	fmt.Println("  step status    <stepID>")
	fmt.Println("  step result    <stepID>")
	fmt.Println("  step artifacts <stepID>")
	fmt.Println("  step validations <stepID>")
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
		fmt.Printf("Error connecting to orchestratord: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Error: %s\n", string(body))
		os.Exit(1)
	}

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Run started:\n%s\n", string(body))
}

func runStatus(id string) {
	resp, err := http.Get(orchestratordURL + "/api/v1/runs/" + id)
	if err != nil {
		fmt.Printf("Error connecting to orchestratord: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Error: %s\n", string(body))
		os.Exit(1)
	}

	body, _ := io.ReadAll(resp.Body)
	var run map[string]interface{}
	_ = json.Unmarshal(body, &run)
	
	fmt.Printf("Run status:\n")
	fmt.Printf("  ID: %v\n", run["id"])
	fmt.Printf("  State: %v\n", run["state"])
	if notes, ok := run["recovery_notes"]; ok && notes != "" {
		fmt.Printf("  Recovery Notes: %v\n", notes)
	}
	fmt.Printf("  Created: %v\n", run["created_at"])
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
		fmt.Printf("Error connecting to orchestratord: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Error: %s\n", string(body))
		os.Exit(1)
	}

	fmt.Printf("Run %s aborted successfully.\n", id)
}

func handleStepCommand(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: orchestratorctl step <start|status|result|artifacts|validations> [args]")
		os.Exit(1)
	}

	cmd := args[0]
	switch cmd {
	case "start":
		if len(args) < 3 {
			fmt.Println("Usage: orchestratorctl step start <runID> <taskFile.yaml>")
			os.Exit(1)
		}
		startStep(args[1], args[2])
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
	default:
		fmt.Printf("Unknown step command: %s\n", cmd)
		os.Exit(1)
	}
}

func startStep(runID, taskFile string) {
	spec, err := validation.ParseTaskSpec(taskFile)
	if err != nil {
		fmt.Printf("Failed to parse task spec: %v\n", err)
		os.Exit(1)
	}

	reqBody := map[string]interface{}{
		"id":       spec.StepID,
		"phase_id": spec.PhaseID,
		"title":    spec.Title,
		"goal":     spec.Goal,
		"adapter":  spec.AdapterProfile,
	}
	data, _ := json.Marshal(reqBody)

	resp, err := http.Post(orchestratordURL+"/api/v1/runs/"+runID+"/steps", "application/json", bytes.NewReader(data))
	if err != nil {
		fmt.Printf("Error connecting to orchestratord: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Error: %s\n", string(body))
		os.Exit(1)
	}

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("TaskSpec dispatched successfully:\n%s\n", string(body))
}

func stepStatus(stepID string) {
	resp, err := http.Get(orchestratordURL + "/api/v1/steps/" + stepID)
	if err != nil {
		fmt.Printf("Error connecting to orchestratord: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Error: %s\n", string(body))
		os.Exit(1)
	}

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Step status:\n%s\n", string(body))
}

func stepResult(stepID string) {
	resp, err := http.Get(orchestratordURL + "/api/v1/steps/" + stepID + "/result")
	if err != nil {
		fmt.Printf("Error connecting to orchestratord: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Error: %s\n", string(body))
		os.Exit(1)
	}

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Step result:\n%s\n", string(body))
}

func stepArtifacts(stepID string) {
	resp, err := http.Get(orchestratordURL + "/api/v1/steps/" + stepID + "/artifacts")
	if err != nil {
		fmt.Printf("Error connecting to orchestratord: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Error: %s\n", string(body))
		os.Exit(1)
	}

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Step artifacts:\n%s\n", string(body))
}
func stepValidations(stepID string) {
	resp, err := http.Get(orchestratordURL + "/api/v1/steps/" + stepID + "/validations")
	if err != nil {
		fmt.Printf("Error connecting to orchestratord: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Error: %s\n", string(body))
		os.Exit(1)
	}

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Step validations:\n%s\n", string(body))
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
		fmt.Printf("Error connecting to orchestratord: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Error handling gate: %s\n", string(body))
		os.Exit(1)
	}

	fmt.Printf("Gate %s %s successfully.\n", id, cmd)
}
