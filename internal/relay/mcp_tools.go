package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

type mcpTool struct {
	Name        string
	Description string
	Scope       string
	InputSchema map[string]any
	Annotations map[string]any
	Invoke      func(ctx context.Context, principal *plannerPrincipal, args map[string]any) (mcpToolResult, *apiError)
}

func (t mcpTool) instanceID(args map[string]any) string {
	value, _ := args["instance_id"].(string)
	return value
}

func buildMCPTools(server *mcpServer) map[string]mcpTool {
	tools := map[string]mcpTool{
		"codencer.list_instances": {
			Name:        "codencer.list_instances",
			Description: "List shared Codencer instances available through the relay.",
			Scope:       "instances:read",
			InputSchema: objectSchema(nil, nil),
			Invoke: func(ctx context.Context, principal *plannerPrincipal, args map[string]any) (mcpToolResult, *apiError) {
				status, _, body, err := server.callPlannerRoute(ctx, principal, http.MethodGet, "/api/v2/instances", nil)
				if err != nil {
					return mcpToolResult{}, err
				}
				var payload any
				if decodeErr := json.Unmarshal(body, &payload); decodeErr != nil {
					return mcpToolResult{}, &apiError{Status: status, Code: "relay_internal_error", Message: decodeErr.Error()}
				}
				return successToolResult("Listed shared instances.", payload), nil
			},
		},
		"codencer.get_instance": {
			Name:        "codencer.get_instance",
			Description: "Get a single shared Codencer instance descriptor.",
			Scope:       "instances:read",
			InputSchema: objectSchema([]string{"instance_id"}, map[string]any{
				"instance_id": stringSchema("Relay instance identifier."),
			}),
			Invoke: func(ctx context.Context, principal *plannerPrincipal, args map[string]any) (mcpToolResult, *apiError) {
				instanceID, apiErr := requiredString(args, "instance_id")
				if apiErr != nil {
					return mcpToolResult{}, apiErr
				}
				status, _, body, err := server.callPlannerRoute(ctx, principal, http.MethodGet, fmt.Sprintf("/api/v2/instances/%s", instanceID), nil)
				if err != nil {
					return mcpToolResult{}, err
				}
				var payload any
				if decodeErr := json.Unmarshal(body, &payload); decodeErr != nil {
					return mcpToolResult{}, &apiError{Status: status, Code: "relay_internal_error", Message: decodeErr.Error()}
				}
				return successToolResult("Fetched instance descriptor.", payload), nil
			},
		},
		"codencer.start_run": plannerProxyTool(server, "codencer.start_run", "Start a run on a shared instance.", "runs:write",
			objectSchema([]string{"instance_id", "payload"}, map[string]any{
				"instance_id": stringSchema("Target shared instance identifier."),
				"payload": objectSchema([]string{"project_id"}, map[string]any{
					"id":              stringSchema("Optional run identifier."),
					"project_id":      stringSchema("Project identifier."),
					"conversation_id": stringSchema("Optional planner conversation identifier."),
					"planner_id":      stringSchema("Optional planner identifier."),
					"executor_id":     stringSchema("Optional executor identifier."),
				}),
			}), func(args map[string]any) (string, string, []byte, *apiError) {
				instanceID, apiErr := requiredString(args, "instance_id")
				if apiErr != nil {
					return "", "", nil, apiErr
				}
				payload, apiErr := requiredObjectJSON(args, "payload")
				if apiErr != nil {
					return "", "", nil, apiErr
				}
				return instanceID, fmt.Sprintf("/api/v2/instances/%s/runs", instanceID), payload, nil
			}),
		"codencer.get_run": plannerProxyTool(server, "codencer.get_run", "Get a run on a shared instance.", "runs:read",
			objectSchema([]string{"instance_id", "run_id"}, map[string]any{
				"instance_id": stringSchema("Target shared instance identifier."),
				"run_id":      stringSchema("Run identifier."),
			}), func(args map[string]any) (string, string, []byte, *apiError) {
				instanceID, apiErr := requiredString(args, "instance_id")
				if apiErr != nil {
					return "", "", nil, apiErr
				}
				runID, apiErr := requiredString(args, "run_id")
				if apiErr != nil {
					return "", "", nil, apiErr
				}
				return instanceID, fmt.Sprintf("/api/v2/instances/%s/runs/%s", instanceID, runID), nil, nil
			}),
		"codencer.list_run_gates": plannerProxyTool(server, "codencer.list_run_gates", "List gates for a run on a shared instance.", "gates:read",
			objectSchema([]string{"instance_id", "run_id"}, map[string]any{
				"instance_id": stringSchema("Target shared instance identifier."),
				"run_id":      stringSchema("Run identifier."),
			}), func(args map[string]any) (string, string, []byte, *apiError) {
				instanceID, apiErr := requiredString(args, "instance_id")
				if apiErr != nil {
					return "", "", nil, apiErr
				}
				runID, apiErr := requiredString(args, "run_id")
				if apiErr != nil {
					return "", "", nil, apiErr
				}
				return instanceID, fmt.Sprintf("/api/v2/instances/%s/runs/%s/gates", instanceID, runID), nil, nil
			}),
		"codencer.submit_task": plannerProxyTool(server, "codencer.submit_task", "Submit a Codencer task to a run.", "steps:write",
			objectSchema([]string{"instance_id", "run_id", "task"}, map[string]any{
				"instance_id": stringSchema("Target shared instance identifier."),
				"run_id":      stringSchema("Run identifier."),
				"task":        taskSpecSchema(),
			}), func(args map[string]any) (string, string, []byte, *apiError) {
				instanceID, apiErr := requiredString(args, "instance_id")
				if apiErr != nil {
					return "", "", nil, apiErr
				}
				runID, apiErr := requiredString(args, "run_id")
				if apiErr != nil {
					return "", "", nil, apiErr
				}
				payload, apiErr := requiredObjectJSON(args, "task")
				if apiErr != nil {
					return "", "", nil, apiErr
				}
				return instanceID, fmt.Sprintf("/api/v2/instances/%s/runs/%s/steps", instanceID, runID), payload, nil
			}),
		"codencer.get_step": plannerProxyTool(server, "codencer.get_step", "Get a step by identifier.", "steps:read",
			objectSchema([]string{"step_id"}, map[string]any{
				"step_id": stringSchema("Step identifier."),
			}), func(args map[string]any) (string, string, []byte, *apiError) {
				stepID, apiErr := requiredString(args, "step_id")
				if apiErr != nil {
					return "", "", nil, apiErr
				}
				return "", fmt.Sprintf("/api/v2/steps/%s", stepID), nil, nil
			}),
		"codencer.wait_step": plannerProxyTool(server, "codencer.wait_step", "Wait for a step to become terminal with a bounded timeout.", "steps:read",
			objectSchema([]string{"step_id"}, map[string]any{
				"step_id":        stringSchema("Step identifier."),
				"timeout_ms":     intSchema("Maximum wait time in milliseconds."),
				"interval_ms":    intSchema("Polling interval in milliseconds."),
				"include_result": boolSchema("Include the step result when terminal."),
			}), func(args map[string]any) (string, string, []byte, *apiError) {
				stepID, apiErr := requiredString(args, "step_id")
				if apiErr != nil {
					return "", "", nil, apiErr
				}
				payload := map[string]any{}
				copyOptional(payload, args, "timeout_ms", "interval_ms", "include_result")
				body, _ := json.Marshal(payload)
				return "", fmt.Sprintf("/api/v2/steps/%s/wait", stepID), body, nil
			}),
		"codencer.get_step_result": plannerProxyTool(server, "codencer.get_step_result", "Get the result payload for a step.", "steps:read",
			objectSchema([]string{"step_id"}, map[string]any{
				"step_id": stringSchema("Step identifier."),
			}), func(args map[string]any) (string, string, []byte, *apiError) {
				stepID, apiErr := requiredString(args, "step_id")
				if apiErr != nil {
					return "", "", nil, apiErr
				}
				return "", fmt.Sprintf("/api/v2/steps/%s/result", stepID), nil, nil
			}),
		"codencer.list_step_artifacts": plannerProxyTool(server, "codencer.list_step_artifacts", "List artifacts emitted by a step.", "artifacts:read",
			objectSchema([]string{"step_id"}, map[string]any{
				"step_id": stringSchema("Step identifier."),
			}), func(args map[string]any) (string, string, []byte, *apiError) {
				stepID, apiErr := requiredString(args, "step_id")
				if apiErr != nil {
					return "", "", nil, apiErr
				}
				return "", fmt.Sprintf("/api/v2/steps/%s/artifacts", stepID), nil, nil
			}),
		"codencer.get_step_logs": {
			Name:        "codencer.get_step_logs",
			Description: "Fetch the collected step logs as text or base64 content.",
			Scope:       "steps:read",
			InputSchema: objectSchema([]string{"step_id"}, map[string]any{
				"step_id": stringSchema("Step identifier."),
			}),
			Invoke: func(ctx context.Context, principal *plannerPrincipal, args map[string]any) (mcpToolResult, *apiError) {
				stepID, apiErr := requiredString(args, "step_id")
				if apiErr != nil {
					return mcpToolResult{}, apiErr
				}
				_, headers, body, err := server.callPlannerRoute(ctx, principal, http.MethodGet, fmt.Sprintf("/api/v2/steps/%s/logs", stepID), nil)
				if err != nil {
					return mcpToolResult{}, err
				}
				contentType := headers.Get("Content-Type")
				payload := artifactContentPayload(contentType, body)
				payload["step_id"] = stepID
				return successToolResult("Fetched step logs.", payload), nil
			},
		},
		"codencer.get_artifact_content": {
			Name:        "codencer.get_artifact_content",
			Description: "Fetch artifact content by artifact identifier with explicit text or base64 encoding.",
			Scope:       "artifacts:read",
			InputSchema: objectSchema([]string{"artifact_id"}, map[string]any{
				"artifact_id": stringSchema("Artifact identifier."),
			}),
			Invoke: func(ctx context.Context, principal *plannerPrincipal, args map[string]any) (mcpToolResult, *apiError) {
				artifactID, apiErr := requiredString(args, "artifact_id")
				if apiErr != nil {
					return mcpToolResult{}, apiErr
				}
				_, headers, body, err := server.callPlannerRoute(ctx, principal, http.MethodGet, fmt.Sprintf("/api/v2/artifacts/%s/content", artifactID), nil)
				if err != nil {
					return mcpToolResult{}, err
				}
				contentType := headers.Get("Content-Type")
				payload := artifactContentPayload(contentType, body)
				payload["artifact_id"] = artifactID
				return successToolResult("Fetched artifact content.", payload), nil
			},
		},
		"codencer.get_step_validations": plannerProxyTool(server, "codencer.get_step_validations", "Get validation outcomes for a step.", "steps:read",
			objectSchema([]string{"step_id"}, map[string]any{
				"step_id": stringSchema("Step identifier."),
			}), func(args map[string]any) (string, string, []byte, *apiError) {
				stepID, apiErr := requiredString(args, "step_id")
				if apiErr != nil {
					return "", "", nil, apiErr
				}
				return "", fmt.Sprintf("/api/v2/steps/%s/validations", stepID), nil, nil
			}),
		"codencer.approve_gate": plannerProxyTool(server, "codencer.approve_gate", "Approve a pending gate for a shared instance.", "gates:write",
			objectSchema([]string{"instance_id", "gate_id"}, map[string]any{
				"instance_id": stringSchema("Target shared instance identifier."),
				"gate_id":     stringSchema("Gate identifier."),
			}), func(args map[string]any) (string, string, []byte, *apiError) {
				instanceID, gateID, apiErr := requireInstanceAndGate(args)
				if apiErr != nil {
					return "", "", nil, apiErr
				}
				if apiErr := server.requireRoutedInstance(context.Background(), "gate", gateID, instanceID); apiErr != nil {
					return "", "", nil, apiErr
				}
				return instanceID, fmt.Sprintf("/api/v2/gates/%s/approve", gateID), nil, nil
			}),
		"codencer.reject_gate": plannerProxyTool(server, "codencer.reject_gate", "Reject a pending gate for a shared instance.", "gates:write",
			objectSchema([]string{"instance_id", "gate_id"}, map[string]any{
				"instance_id": stringSchema("Target shared instance identifier."),
				"gate_id":     stringSchema("Gate identifier."),
			}), func(args map[string]any) (string, string, []byte, *apiError) {
				instanceID, gateID, apiErr := requireInstanceAndGate(args)
				if apiErr != nil {
					return "", "", nil, apiErr
				}
				if apiErr := server.requireRoutedInstance(context.Background(), "gate", gateID, instanceID); apiErr != nil {
					return "", "", nil, apiErr
				}
				return instanceID, fmt.Sprintf("/api/v2/gates/%s/reject", gateID), nil, nil
			}),
		"codencer.abort_run": plannerProxyTool(server, "codencer.abort_run", "Abort a run on a shared instance.", "runs:write",
			objectSchema([]string{"instance_id", "run_id"}, map[string]any{
				"instance_id": stringSchema("Target shared instance identifier."),
				"run_id":      stringSchema("Run identifier."),
			}), func(args map[string]any) (string, string, []byte, *apiError) {
				instanceID, apiErr := requiredString(args, "instance_id")
				if apiErr != nil {
					return "", "", nil, apiErr
				}
				runID, apiErr := requiredString(args, "run_id")
				if apiErr != nil {
					return "", "", nil, apiErr
				}
				return instanceID, fmt.Sprintf("/api/v2/instances/%s/runs/%s/abort", instanceID, runID), nil, nil
			}),
		"codencer.retry_step": plannerProxyTool(server, "codencer.retry_step", "Retry a step on a shared instance.", "steps:write",
			objectSchema([]string{"instance_id", "step_id"}, map[string]any{
				"instance_id": stringSchema("Target shared instance identifier."),
				"step_id":     stringSchema("Step identifier."),
			}), func(args map[string]any) (string, string, []byte, *apiError) {
				instanceID, apiErr := requiredString(args, "instance_id")
				if apiErr != nil {
					return "", "", nil, apiErr
				}
				stepID, apiErr := requiredString(args, "step_id")
				if apiErr != nil {
					return "", "", nil, apiErr
				}
				if apiErr := server.requireRoutedInstance(context.Background(), "step", stepID, instanceID); apiErr != nil {
					return "", "", nil, apiErr
				}
				return instanceID, fmt.Sprintf("/api/v2/steps/%s/retry", stepID), nil, nil
			}),
	}
	addProjectMCPTools(server, tools)
	return tools
}

func plannerProxyTool(server *mcpServer, name, description, scope string, schema map[string]any, route func(args map[string]any) (string, string, []byte, *apiError)) mcpTool {
	return mcpTool{
		Name:        name,
		Description: description,
		Scope:       scope,
		InputSchema: schema,
		Invoke: func(ctx context.Context, principal *plannerPrincipal, args map[string]any) (mcpToolResult, *apiError) {
			instanceID, path, body, apiErr := route(args)
			if apiErr != nil {
				return mcpToolResult{}, apiErr
			}
			if requiresExplicitInstance(scope) && instanceID == "" {
				return mcpToolResult{}, &apiError{Status: http.StatusBadRequest, Code: "malformed_request", Message: "instance_id is required for this tool"}
			}
			method := http.MethodGet
			if body != nil {
				method = http.MethodPost
			}
			if stringsHasSuffix(path, "/abort") || stringsHasSuffix(path, "/approve") || stringsHasSuffix(path, "/reject") || stringsHasSuffix(path, "/retry") || stringsHasSuffix(path, "/wait") {
				method = http.MethodPost
			}
			_, _, responseBody, err := server.callPlannerRoute(ctx, principal, method, path, body)
			if err != nil {
				return mcpToolResult{}, err
			}
			var payload any
			if len(responseBody) == 0 {
				payload = map[string]any{"ok": true}
			} else if decodeErr := json.Unmarshal(responseBody, &payload); decodeErr != nil {
				payload = map[string]any{"raw": string(responseBody)}
			}
			return successToolResult(description, payload), nil
		},
	}
}

func addProjectMCPTools(server *mcpServer, tools map[string]mcpTool) {
	tools["codencer.list_projects"] = mcpTool{
		Name:        "codencer.list_projects",
		Description: "List shared Codencer projects available through the relay.",
		Scope:       "projects:read",
		InputSchema: objectSchema(nil, nil),
		Annotations: readOnlyAnnotations(),
		Invoke: func(ctx context.Context, principal *plannerPrincipal, args map[string]any) (mcpToolResult, *apiError) {
			return callProjectTool(server, ctx, principal, http.MethodGet, "/api/v2/projects", nil, "Listed shared projects.")
		},
	}
	tools["codencer.get_project"] = mcpTool{
		Name:        "codencer.get_project",
		Description: "Get one shared Codencer project descriptor.",
		Scope:       "projects:read",
		InputSchema: objectSchema([]string{"project_id"}, map[string]any{"project_id": stringSchema("Shared project identifier.")}),
		Annotations: readOnlyAnnotations(),
		Invoke: func(ctx context.Context, principal *plannerPrincipal, args map[string]any) (mcpToolResult, *apiError) {
			projectID, apiErr := requiredString(args, "project_id")
			if apiErr != nil {
				return mcpToolResult{}, apiErr
			}
			return callProjectTool(server, ctx, principal, http.MethodGet, fmt.Sprintf("/api/v2/projects/%s", projectID), nil, "Fetched project descriptor.")
		},
	}
	tools["codencer.start_project_run"] = projectRouteTool(server, "codencer.start_project_run", "Start a run for a shared project.", "runs:write", objectSchema([]string{"project_id"}, map[string]any{
		"project_id": stringSchema("Shared project identifier."),
	}), func(projectID string, args map[string]any) (string, []byte, *apiError) {
		return fmt.Sprintf("/api/v2/projects/%s/runs", projectID), []byte(`{}`), nil
	})
	tools["codencer.list_project_runs"] = projectRouteTool(server, "codencer.list_project_runs", "List runs for a shared project.", "runs:read", objectSchema([]string{"project_id"}, map[string]any{
		"project_id": stringSchema("Shared project identifier."),
	}), func(projectID string, args map[string]any) (string, []byte, *apiError) {
		return fmt.Sprintf("/api/v2/projects/%s/runs", projectID), nil, nil
	})
	setToolReadOnly(tools, "codencer.list_project_runs")
	tools["codencer.get_project_run"] = projectRouteTool(server, "codencer.get_project_run", "Get a run for a shared project.", "runs:read", objectSchema([]string{"project_id", "run_id"}, map[string]any{
		"project_id": stringSchema("Shared project identifier."),
		"run_id":     stringSchema("Run identifier."),
	}), func(projectID string, args map[string]any) (string, []byte, *apiError) {
		runID, apiErr := requiredString(args, "run_id")
		if apiErr != nil {
			return "", nil, apiErr
		}
		return fmt.Sprintf("/api/v2/projects/%s/runs/%s", projectID, runID), nil, nil
	})
	setToolReadOnly(tools, "codencer.get_project_run")
	tools["codencer.submit_project_task"] = projectSubmitTool(server, "codencer.submit_project_task", false)
	tools["codencer.submit_project_task_and_wait"] = projectSubmitTool(server, "codencer.submit_project_task_and_wait", true)
	tools["codencer.run_project_manifest"] = projectRouteTool(server, "codencer.run_project_manifest", "Run a project manifest sequentially for a shared project.", "runs:write", objectSchema([]string{"project_id"}, map[string]any{
		"project_id":    stringSchema("Shared project identifier."),
		"manifest":      objectSchema(nil, nil),
		"manifest_text": stringSchema("YAML or JSON manifest text."),
		"manifest_name": stringSchema("Optional manifest display name."),
		"wait":          boolSchema("Wait for manifest completion."),
	}), func(projectID string, args map[string]any) (string, []byte, *apiError) {
		payload := map[string]any{}
		copyOptional(payload, args, "manifest", "manifest_text", "manifest_name", "wait")
		body, err := json.Marshal(payload)
		if err != nil {
			return "", nil, &apiError{Status: http.StatusBadRequest, Code: "malformed_request", Message: err.Error()}
		}
		return fmt.Sprintf("/api/v2/projects/%s/run-plan", projectID), body, nil
	})
	tools["codencer.get_execution_report"] = projectRouteTool(server, "codencer.get_execution_report", "Get a persisted run-plan execution report for a shared project.", "reports:read", objectSchema([]string{"project_id", "run_id"}, map[string]any{
		"project_id": stringSchema("Shared project identifier."),
		"run_id":     stringSchema("Run identifier."),
	}), func(projectID string, args map[string]any) (string, []byte, *apiError) {
		runID, apiErr := requiredString(args, "run_id")
		if apiErr != nil {
			return "", nil, apiErr
		}
		return fmt.Sprintf("/api/v2/projects/%s/reports/run-plans/%s", projectID, runID), nil, nil
	})
	setToolReadOnly(tools, "codencer.get_execution_report")
	tools["codencer.get_run_report"] = projectRouteTool(server, "codencer.get_run_report", "Alias for codencer.get_execution_report.", "reports:read", objectSchema([]string{"project_id", "run_id"}, map[string]any{
		"project_id": stringSchema("Shared project identifier."),
		"run_id":     stringSchema("Run identifier."),
	}), func(projectID string, args map[string]any) (string, []byte, *apiError) {
		runID, apiErr := requiredString(args, "run_id")
		if apiErr != nil {
			return "", nil, apiErr
		}
		return fmt.Sprintf("/api/v2/projects/%s/reports/run-plans/%s", projectID, runID), nil, nil
	})
	setToolReadOnly(tools, "codencer.get_run_report")
	tools["codencer.get_project_blocker"] = mcpTool{
		Name:        "codencer.get_project_blocker",
		Description: "Get the top-level blocker from a persisted project execution report.",
		Scope:       "reports:read",
		InputSchema: withProjectSelectorSchema(objectSchema([]string{"project_id", "run_id"}, map[string]any{
			"project_id": stringSchema("Shared project identifier."),
			"run_id":     stringSchema("Run identifier."),
		})),
		Annotations: readOnlyAnnotations(),
		Invoke: func(ctx context.Context, principal *plannerPrincipal, args map[string]any) (mcpToolResult, *apiError) {
			projectID, apiErr := requiredString(args, "project_id")
			if apiErr != nil {
				return mcpToolResult{}, apiErr
			}
			runID, apiErr := requiredString(args, "run_id")
			if apiErr != nil {
				return mcpToolResult{}, apiErr
			}
			path := appendProjectSelector(fmt.Sprintf("/api/v2/projects/%s/reports/run-plans/%s", projectID, runID), args)
			_, _, body, err := server.callPlannerRoute(ctx, principal, http.MethodGet, path, nil)
			if err != nil {
				return mcpToolResult{}, err
			}
			payload := map[string]any{"project_id": projectID, "run_id": runID, "blocker": nil}
			var report map[string]any
			if decodeErr := json.Unmarshal(body, &report); decodeErr == nil {
				if blocker, ok := report["blocker"]; ok {
					payload["blocker"] = blocker
				}
				payload["report_status"] = report["status"]
			}
			return successToolResult("Fetched project blocker.", payload), nil
		},
	}
	blockerAlias := tools["codencer.get_project_blocker"]
	blockerAlias.Name = "codencer.get_blocker"
	blockerAlias.Description = "Read-only alias for codencer.get_project_blocker."
	tools["codencer.get_blocker"] = blockerAlias
	for _, resource := range []string{"result", "artifacts", "logs", "validations"} {
		name := "codencer.get_project_step_" + resource
		scope := scopeForProjectEvidence(resource)
		description := "Get project step " + resource + " evidence."
		routeResource := resource
		tools[name] = projectRouteTool(server, name, description, scope, objectSchema([]string{"project_id", "step_id"}, map[string]any{
			"project_id": stringSchema("Shared project identifier."),
			"step_id":    stringSchema("Step identifier."),
		}), func(projectID string, args map[string]any) (string, []byte, *apiError) {
			stepID, apiErr := requiredString(args, "step_id")
			if apiErr != nil {
				return "", nil, apiErr
			}
			return fmt.Sprintf("/api/v2/projects/%s/steps/%s/%s", projectID, stepID, routeResource), nil, nil
		})
		setToolReadOnly(tools, name)
	}
}

func projectSubmitTool(server *mcpServer, name string, wait bool) mcpTool {
	description := "Submit a task to a shared project."
	if wait {
		description = "Submit a task to a shared project and wait for structured evidence."
	}
	return projectRouteTool(server, name, description, "steps:write", objectSchema([]string{"project_id"}, map[string]any{
		"project_id":      stringSchema("Shared project identifier."),
		"run_id":          stringSchema("Optional run identifier."),
		"goal":            stringSchema("Direct goal text."),
		"prompt":          stringSchema("Prompt text."),
		"task":            taskSpecSchema(),
		"profile":         stringSchema("Planner-facing profile id."),
		"adapter_profile": stringSchema("Daemon-facing adapter profile override."),
		"title":           stringSchema("Task title."),
		"timeout_seconds": intSchema("Optional timeout in seconds."),
	}), func(projectID string, args map[string]any) (string, []byte, *apiError) {
		payload := map[string]any{"wait": wait}
		copyOptional(payload, args, "run_id", "goal", "prompt", "task", "profile", "adapter_profile", "title", "timeout_seconds")
		body, err := json.Marshal(payload)
		if err != nil {
			return "", nil, &apiError{Status: http.StatusBadRequest, Code: "malformed_request", Message: err.Error()}
		}
		return fmt.Sprintf("/api/v2/projects/%s/submit", projectID), body, nil
	})
}

func projectRouteTool(server *mcpServer, name, description, scope string, schema map[string]any, route func(projectID string, args map[string]any) (string, []byte, *apiError)) mcpTool {
	schema = withProjectSelectorSchema(schema)
	return mcpTool{
		Name:        name,
		Description: description,
		Scope:       scope,
		InputSchema: schema,
		Invoke: func(ctx context.Context, principal *plannerPrincipal, args map[string]any) (mcpToolResult, *apiError) {
			projectID, apiErr := requiredString(args, "project_id")
			if apiErr != nil {
				return mcpToolResult{}, apiErr
			}
			path, body, apiErr := route(projectID, args)
			if apiErr != nil {
				return mcpToolResult{}, apiErr
			}
			path = appendProjectSelector(path, args)
			method := http.MethodGet
			if body != nil {
				method = http.MethodPost
			}
			return callProjectTool(server, ctx, principal, method, path, body, description)
		},
	}
}

func callProjectTool(server *mcpServer, ctx context.Context, principal *plannerPrincipal, method, path string, body []byte, summary string) (mcpToolResult, *apiError) {
	_, _, responseBody, err := server.callPlannerRoute(ctx, principal, method, path, body)
	if err != nil {
		return mcpToolResult{}, err
	}
	var payload any
	if len(responseBody) == 0 {
		payload = map[string]any{"ok": true}
	} else if decodeErr := json.Unmarshal(responseBody, &payload); decodeErr != nil {
		payload = map[string]any{"raw": string(responseBody)}
	}
	return successToolResult(summary, payload), nil
}

func setToolReadOnly(tools map[string]mcpTool, name string) {
	tool := tools[name]
	tool.Annotations = readOnlyAnnotations()
	tools[name] = tool
}

func toolOrder(tools map[string]mcpTool) []mcpTool {
	values := make([]mcpTool, 0, len(tools))
	for _, tool := range tools {
		values = append(values, tool)
	}
	sort.Slice(values, func(i, j int) bool { return values[i].Name < values[j].Name })
	return values
}

func requiredString(args map[string]any, key string) (string, *apiError) {
	value, ok := args[key].(string)
	if !ok || stringsTrim(value) == "" {
		return "", &apiError{Status: http.StatusBadRequest, Code: "malformed_request", Message: fmt.Sprintf("%s is required", key)}
	}
	return value, nil
}

func requiredObjectJSON(args map[string]any, key string) ([]byte, *apiError) {
	value, ok := args[key]
	if !ok {
		return nil, &apiError{Status: http.StatusBadRequest, Code: "malformed_request", Message: fmt.Sprintf("%s is required", key)}
	}
	body, err := json.Marshal(value)
	if err != nil {
		return nil, &apiError{Status: http.StatusBadRequest, Code: "malformed_request", Message: err.Error()}
	}
	return body, nil
}

func requireInstanceAndGate(args map[string]any) (string, string, *apiError) {
	instanceID, apiErr := requiredString(args, "instance_id")
	if apiErr != nil {
		return "", "", apiErr
	}
	gateID, apiErr := requiredString(args, "gate_id")
	if apiErr != nil {
		return "", "", apiErr
	}
	return instanceID, gateID, nil
}

func objectSchema(required []string, properties map[string]any) map[string]any {
	if properties == nil {
		properties = map[string]any{}
	}
	return map[string]any{
		"type":       "object",
		"required":   required,
		"properties": properties,
	}
}

func withProjectSelectorSchema(schema map[string]any) map[string]any {
	if schema == nil {
		schema = objectSchema(nil, nil)
	}
	properties, _ := schema["properties"].(map[string]any)
	if properties == nil {
		properties = map[string]any{}
		schema["properties"] = properties
	}
	properties["machine_id"] = stringSchema("Optional machine_id selector for a shared project location.")
	properties["host_label"] = stringSchema("Optional host_label selector for a shared project location.")
	return schema
}

func appendProjectSelector(path string, args map[string]any) string {
	values := url.Values{}
	if machineID, _ := args["machine_id"].(string); strings.TrimSpace(machineID) != "" {
		values.Set("machine_id", strings.TrimSpace(machineID))
	}
	if hostLabel, _ := args["host_label"].(string); strings.TrimSpace(hostLabel) != "" {
		values.Set("host_label", strings.TrimSpace(hostLabel))
	}
	if len(values) == 0 {
		return path
	}
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	return path + separator + values.Encode()
}

func stringSchema(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func intSchema(description string) map[string]any {
	return map[string]any{"type": "integer", "description": description}
}

func boolSchema(description string) map[string]any {
	return map[string]any{"type": "boolean", "description": description}
}

func readOnlyAnnotations() map[string]any {
	return map[string]any{"readOnlyHint": true}
}

func taskSpecSchema() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "Canonical Codencer TaskSpec payload.",
		"required":    []string{"version", "goal"},
		"properties": map[string]any{
			"version":         stringSchema("Task contract version."),
			"project_id":      stringSchema("Project identifier."),
			"run_id":          stringSchema("Optional run identifier."),
			"phase_id":        stringSchema("Optional phase identifier."),
			"step_id":         stringSchema("Optional step identifier."),
			"title":           stringSchema("Optional human-readable title."),
			"goal":            stringSchema("Primary instruction for the adapter."),
			"context":         objectSchema(nil, map[string]any{"summary": stringSchema("Optional contextual summary.")}),
			"constraints":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"allowed_paths":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"forbidden_paths": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"validations": map[string]any{
				"type": "array",
				"items": objectSchema([]string{"name", "command"}, map[string]any{
					"name":    stringSchema("Validation name."),
					"command": stringSchema("Shell command executed by the daemon validation phase."),
				}),
			},
			"acceptance":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"stop_conditions": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"policy_bundle":   stringSchema("Optional policy bundle."),
			"adapter_profile": stringSchema("Preferred adapter profile."),
			"timeout_seconds": intSchema("Optional task timeout in seconds."),
			"is_simulation":   boolSchema("Simulation flag."),
			"submission_provenance": objectSchema(nil, map[string]any{
				"source_kind":      stringSchema("Normalized submit source kind."),
				"source_name":      stringSchema("Optional submit source name."),
				"original_format":  stringSchema("Original submit payload format."),
				"original_input":   stringSchema("Original submit payload reference or excerpt."),
				"defaults_applied": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			}),
		},
	}
}

func copyOptional(dst, src map[string]any, keys ...string) {
	for _, key := range keys {
		if value, ok := src[key]; ok {
			dst[key] = value
		}
	}
}

func requiresExplicitInstance(scope string) bool {
	switch scope {
	case "runs:write", "steps:write", "gates:write":
		return true
	default:
		return false
	}
}

func (s *mcpServer) requireRoutedInstance(ctx context.Context, resourceKind, resourceID, instanceID string) *apiError {
	resolvedInstance, apiErr := s.relay.resolveResourceRoute(ctx, &plannerPrincipal{Scopes: []string{"*"}}, resourceKind, resourceID, "", instanceID)
	if apiErr != nil {
		return apiErr
	}
	if resolvedInstance != instanceID {
		return &apiError{Status: http.StatusForbidden, Code: "instance_denied", Message: fmt.Sprintf("%s is not routed to the requested instance", resourceKind)}
	}
	return nil
}

func stringsHasSuffix(value, suffix string) bool {
	return len(value) >= len(suffix) && value[len(value)-len(suffix):] == suffix
}

func stringsTrim(value string) string {
	return strings.TrimSpace(value)
}
