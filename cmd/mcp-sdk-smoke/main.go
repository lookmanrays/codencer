package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type authRoundTripper struct {
	base          http.RoundTripper
	authorization string
}

func (rt authRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	base := rt.base
	if base == nil {
		base = http.DefaultTransport
	}
	cloned := req.Clone(req.Context())
	cloned.Header = req.Header.Clone()
	if rt.authorization != "" {
		cloned.Header.Set("Authorization", rt.authorization)
	}
	return base.RoundTrip(cloned)
}

type instanceRecord struct {
	InstanceID string `json:"instance_id"`
}

type stepRecord struct {
	ID    string `json:"id"`
	State string `json:"state"`
}

type waitRecord struct {
	StepID   string `json:"step_id"`
	State    string `json:"state"`
	Terminal bool   `json:"terminal"`
	TimedOut bool   `json:"timed_out"`
}

type smokeOutput struct {
	SessionID       string `json:"session_id"`
	ProtocolVersion string `json:"protocol_version"`
	InstanceID      string `json:"instance_id"`
	RunID           string `json:"run_id"`
	StepID          string `json:"step_id"`
	StepState       string `json:"step_state"`
	ToolNames       []string
	Result          any `json:"result,omitempty"`
	Validations     any `json:"validations,omitempty"`
	Logs            any `json:"logs,omitempty"`
	RunGates        any `json:"run_gates,omitempty"`
	Artifacts       any `json:"artifacts,omitempty"`
	ArtifactContent any `json:"artifact_content,omitempty"`
}

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "mcp-sdk-smoke: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("mcp-sdk-smoke", flag.ContinueOnError)
	fs.SetOutput(stderr)
	endpoint := fs.String("endpoint", "http://127.0.0.1:8090/mcp", "Relay MCP endpoint")
	token := fs.String("token", "", "Planner bearer token")
	instanceID := fs.String("instance-id", "", "Target instance id; defaults to the first shared instance")
	runID := fs.String("run-id", fmt.Sprintf("sdk-smoke-%d", time.Now().Unix()), "Run id to create")
	projectID := fs.String("project-id", "sdk-smoke-project", "Project id for the run")
	goal := fs.String("goal", "Verify official Go SDK interoperability", "Task goal for submit_task")
	adapterProfile := fs.String("adapter-profile", "", "Optional adapter profile for submit_task")
	validationCommand := fs.String("validation-command", "go build ./...", "Optional validation command to attach; set empty to disable")
	waitTimeoutMS := fs.Int("wait-timeout-ms", 5000, "wait_step timeout in milliseconds")
	waitIntervalMS := fs.Int("wait-interval-ms", 50, "wait_step poll interval in milliseconds")
	jsonOutput := fs.Bool("json", true, "Print JSON output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*token) == "" {
		return errors.New("--token is required")
	}

	httpClient := &http.Client{
		Transport: authRoundTripper{authorization: "Bearer " + strings.TrimSpace(*token)},
	}
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "codencer-mcp-sdk-smoke",
		Version: "1.0.0",
	}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:   *endpoint,
		HTTPClient: httpClient,
	}, nil)
	if err != nil {
		return fmt.Errorf("connect to relay MCP: %w", err)
	}
	defer func() { _ = session.Close() }()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		return fmt.Errorf("list tools: %w", err)
	}
	toolNames := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		toolNames = append(toolNames, tool.Name)
	}

	if strings.TrimSpace(*instanceID) == "" {
		result, err := callTool(ctx, session, "codencer.list_instances", map[string]any{})
		if err != nil {
			return err
		}
		var instances []instanceRecord
		if err := decodeStructured(result.StructuredContent, &instances); err != nil {
			return fmt.Errorf("decode list_instances: %w", err)
		}
		if len(instances) == 0 || strings.TrimSpace(instances[0].InstanceID) == "" {
			return errors.New("no shared instances were returned by the relay MCP surface")
		}
		*instanceID = instances[0].InstanceID
	}

	if _, err := callTool(ctx, session, "codencer.start_run", map[string]any{
		"instance_id": *instanceID,
		"payload": map[string]any{
			"id":         *runID,
			"project_id": *projectID,
		},
	}); err != nil {
		return err
	}

	task := map[string]any{
		"version": "v1",
		"goal":    *goal,
	}
	if strings.TrimSpace(*adapterProfile) != "" {
		task["adapter_profile"] = strings.TrimSpace(*adapterProfile)
	}
	if strings.TrimSpace(*validationCommand) != "" {
		task["validations"] = []map[string]any{{
			"name":    "bridge-build",
			"command": strings.TrimSpace(*validationCommand),
		}}
	}
	submitted, err := callTool(ctx, session, "codencer.submit_task", map[string]any{
		"instance_id": *instanceID,
		"run_id":      *runID,
		"task":        task,
	})
	if err != nil {
		return err
	}
	var step stepRecord
	if err := decodeStructured(submitted.StructuredContent, &step); err != nil {
		return fmt.Errorf("decode submit_task response: %w", err)
	}
	if strings.TrimSpace(step.ID) == "" {
		return errors.New("submit_task did not return a step id")
	}

	waited, err := callTool(ctx, session, "codencer.wait_step", map[string]any{
		"instance_id": *instanceID,
		"step_id":     step.ID,
		"timeout_ms":  *waitTimeoutMS,
		"interval_ms": *waitIntervalMS,
	})
	if err != nil {
		return err
	}
	var waitInfo waitRecord
	if err := decodeStructured(waited.StructuredContent, &waitInfo); err != nil {
		return fmt.Errorf("decode wait_step response: %w", err)
	}
	if !waitInfo.Terminal {
		return fmt.Errorf("wait_step did not reach a terminal state: %+v", waitInfo)
	}

	result, err := callTool(ctx, session, "codencer.get_step_result", map[string]any{
		"instance_id": *instanceID,
		"step_id":     step.ID,
	})
	if err != nil {
		return err
	}
	validations, err := callTool(ctx, session, "codencer.get_step_validations", map[string]any{
		"instance_id": *instanceID,
		"step_id":     step.ID,
	})
	if err != nil {
		return err
	}
	logs, err := callTool(ctx, session, "codencer.get_step_logs", map[string]any{
		"instance_id": *instanceID,
		"step_id":     step.ID,
	})
	if err != nil {
		return err
	}
	runGates, err := callTool(ctx, session, "codencer.list_run_gates", map[string]any{
		"instance_id": *instanceID,
		"run_id":      *runID,
	})
	if err != nil {
		return err
	}
	artifacts, err := callTool(ctx, session, "codencer.list_step_artifacts", map[string]any{
		"instance_id": *instanceID,
		"step_id":     step.ID,
	})
	if err != nil {
		return err
	}

	output := smokeOutput{
		SessionID:       session.ID(),
		ProtocolVersion: session.InitializeResult().ProtocolVersion,
		InstanceID:      *instanceID,
		RunID:           *runID,
		StepID:          step.ID,
		StepState:       waitInfo.State,
		ToolNames:       toolNames,
		Result:          result.StructuredContent,
		Validations:     validations.StructuredContent,
		Logs:            logs.StructuredContent,
		RunGates:        runGates.StructuredContent,
		Artifacts:       artifacts.StructuredContent,
	}

	var artifactList []map[string]any
	if err := decodeStructured(artifacts.StructuredContent, &artifactList); err == nil && len(artifactList) > 0 {
		if artifactID, _ := artifactList[0]["id"].(string); artifactID != "" {
			artifactContent, err := callTool(ctx, session, "codencer.get_artifact_content", map[string]any{
				"artifact_id": artifactID,
			})
			if err != nil {
				return err
			}
			output.ArtifactContent = artifactContent.StructuredContent
		}
	}

	if *jsonOutput {
		return writeJSON(stdout, output)
	}
	_, err = fmt.Fprintf(stdout, "session_id=%s protocol=%s instance_id=%s run_id=%s step_id=%s step_state=%s\n",
		output.SessionID,
		output.ProtocolVersion,
		output.InstanceID,
		output.RunID,
		output.StepID,
		output.StepState,
	)
	return err
}

func callTool(ctx context.Context, session *mcp.ClientSession, name string, args any) (*mcp.CallToolResult, error) {
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		return nil, fmt.Errorf("%s failed: %w", name, err)
	}
	if result.IsError {
		return nil, fmt.Errorf("%s failed: %s", name, resultErrorText(result))
	}
	return result, nil
}

func resultErrorText(result *mcp.CallToolResult) string {
	if result == nil {
		return "tool error"
	}
	var parts []string
	for _, content := range result.Content {
		if text, ok := content.(*mcp.TextContent); ok && strings.TrimSpace(text.Text) != "" {
			parts = append(parts, strings.TrimSpace(text.Text))
		}
	}
	if len(parts) == 0 {
		return "tool returned isError=true"
	}
	return strings.Join(parts, "; ")
}

func decodeStructured(value any, out any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

func writeJSON(stdout io.Writer, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, string(data))
	return err
}
