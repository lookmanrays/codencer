package cloud

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

type mcpTool struct {
	Name        string
	Description string
	Scope       string
	InputSchema map[string]any
	Invoke      func(ctx context.Context, token *APIToken, args map[string]any) (mcpToolResult, *apiError)
}

func buildMCPTools(server *mcpServer) map[string]mcpTool {
	return map[string]mcpTool{
		"codencer.list_instances": {
			Name:        "codencer.list_instances",
			Description: "List shared Codencer instances available through the cloud control plane.",
			Scope:       "runtime_instances:read",
			InputSchema: objectSchema(nil, nil),
			Invoke: func(ctx context.Context, token *APIToken, args map[string]any) (mcpToolResult, *apiError) {
				if err := server.cloud.syncRuntimeScope(ctx, token.OrgID, token.WorkspaceID, token.ProjectID); err != nil {
					return mcpToolResult{}, &apiError{Status: http.StatusInternalServerError, Code: "runtime_sync_failed", Message: err.Error()}
				}
				instances, err := server.cloud.store.ListRuntimeInstances(ctx, token.OrgID, token.WorkspaceID, token.ProjectID, "")
				if err != nil {
					return mcpToolResult{}, &apiError{Status: http.StatusInternalServerError, Code: "runtime_lookup_failed", Message: err.Error()}
				}
				return successToolResult("Listed tenant-scoped runtime instances.", filterRuntimeInstances(instances, false)), nil
			},
		},
		"codencer.get_instance": {
			Name:        "codencer.get_instance",
			Description: "Get a tenant-scoped shared Codencer instance descriptor.",
			Scope:       "runtime_instances:read",
			InputSchema: objectSchema([]string{"instance_id"}, map[string]any{"instance_id": stringSchema("Cloud runtime instance identifier.")}),
			Invoke: func(ctx context.Context, token *APIToken, args map[string]any) (mcpToolResult, *apiError) {
				instanceID, apiErr := requiredString(args, "instance_id")
				if apiErr != nil {
					return mcpToolResult{}, apiErr
				}
				instance, apiErr := server.loadAuthorizedInstance(ctx, token, instanceID)
				if apiErr != nil {
					return mcpToolResult{}, apiErr
				}
				connector, err := server.cloud.store.GetRuntimeConnectorInstallation(ctx, instance.RuntimeConnectorInstallationID)
				if err != nil {
					return mcpToolResult{}, &apiError{Status: http.StatusInternalServerError, Code: "runtime_connector_lookup_failed", Message: err.Error()}
				}
				return successToolResult("Fetched instance descriptor.", map[string]any{"instance": instance, "runtime_connector": connector}), nil
			},
		},
		"codencer.start_run": runtimeProxyTool(server, "codencer.start_run", "Start a run on a tenant-scoped shared instance.", "runs:write",
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
		"codencer.get_run": runtimeProxyTool(server, "codencer.get_run", "Get a run on a tenant-scoped shared instance.", "runs:read",
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
		"codencer.list_run_gates": runtimeProxyTool(server, "codencer.list_run_gates", "List gates for a run on a tenant-scoped shared instance.", "gates:read",
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
		"codencer.submit_task": runtimeProxyTool(server, "codencer.submit_task", "Submit a Codencer task to a run.", "steps:write",
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
		"codencer.get_step": routedProxyTool(server, "codencer.get_step", "Get a step by identifier.", "steps:read",
			objectSchema([]string{"step_id"}, map[string]any{"step_id": stringSchema("Step identifier.")}),
			func(args map[string]any) (string, []byte, *apiError) {
				stepID, apiErr := requiredString(args, "step_id")
				if apiErr != nil {
					return "", nil, apiErr
				}
				return fmt.Sprintf("/api/v2/steps/%s", stepID), nil, nil
			}),
		"codencer.wait_step": routedProxyTool(server, "codencer.wait_step", "Wait for a step to become terminal with a bounded timeout.", "steps:read",
			objectSchema([]string{"step_id"}, map[string]any{
				"step_id":        stringSchema("Step identifier."),
				"timeout_ms":     intSchema("Maximum wait time in milliseconds."),
				"interval_ms":    intSchema("Polling interval in milliseconds."),
				"include_result": boolSchema("Include the step result when terminal."),
			}), func(args map[string]any) (string, []byte, *apiError) {
				stepID, apiErr := requiredString(args, "step_id")
				if apiErr != nil {
					return "", nil, apiErr
				}
				payload := map[string]any{}
				copyOptional(payload, args, "timeout_ms", "interval_ms", "include_result")
				body, _ := json.Marshal(payload)
				return fmt.Sprintf("/api/v2/steps/%s/wait", stepID), body, nil
			}),
		"codencer.get_step_result": routedProxyTool(server, "codencer.get_step_result", "Get the result payload for a step.", "steps:read",
			objectSchema([]string{"step_id"}, map[string]any{"step_id": stringSchema("Step identifier.")}),
			func(args map[string]any) (string, []byte, *apiError) {
				stepID, apiErr := requiredString(args, "step_id")
				if apiErr != nil {
					return "", nil, apiErr
				}
				return fmt.Sprintf("/api/v2/steps/%s/result", stepID), nil, nil
			}),
		"codencer.list_step_artifacts": routedProxyTool(server, "codencer.list_step_artifacts", "List artifacts emitted by a step.", "artifacts:read",
			objectSchema([]string{"step_id"}, map[string]any{"step_id": stringSchema("Step identifier.")}),
			func(args map[string]any) (string, []byte, *apiError) {
				stepID, apiErr := requiredString(args, "step_id")
				if apiErr != nil {
					return "", nil, apiErr
				}
				return fmt.Sprintf("/api/v2/steps/%s/artifacts", stepID), nil, nil
			}),
		"codencer.get_step_logs": {
			Name:        "codencer.get_step_logs",
			Description: "Fetch the collected step logs as text or base64 content.",
			Scope:       "steps:read",
			InputSchema: objectSchema([]string{"step_id"}, map[string]any{"step_id": stringSchema("Step identifier.")}),
			Invoke: func(ctx context.Context, token *APIToken, args map[string]any) (mcpToolResult, *apiError) {
				stepID, apiErr := requiredString(args, "step_id")
				if apiErr != nil {
					return mcpToolResult{}, apiErr
				}
				_, headers, body, err := callRoutedTool(server, ctx, token, fmt.Sprintf("/api/v2/steps/%s/logs", stepID), nil, "steps:read")
				if err != nil {
					return mcpToolResult{}, err
				}
				payload := artifactContentPayload(headers.Get("Content-Type"), body)
				payload["step_id"] = stepID
				return successToolResult("Fetched step logs.", payload), nil
			},
		},
		"codencer.get_artifact_content": {
			Name:        "codencer.get_artifact_content",
			Description: "Fetch artifact content by artifact identifier with explicit text or base64 encoding.",
			Scope:       "artifacts:read",
			InputSchema: objectSchema([]string{"artifact_id"}, map[string]any{"artifact_id": stringSchema("Artifact identifier.")}),
			Invoke: func(ctx context.Context, token *APIToken, args map[string]any) (mcpToolResult, *apiError) {
				artifactID, apiErr := requiredString(args, "artifact_id")
				if apiErr != nil {
					return mcpToolResult{}, apiErr
				}
				_, headers, body, err := callRoutedTool(server, ctx, token, fmt.Sprintf("/api/v2/artifacts/%s/content", artifactID), nil, "artifacts:read")
				if err != nil {
					return mcpToolResult{}, err
				}
				payload := artifactContentPayload(headers.Get("Content-Type"), body)
				payload["artifact_id"] = artifactID
				return successToolResult("Fetched artifact content.", payload), nil
			},
		},
		"codencer.get_step_validations": routedProxyTool(server, "codencer.get_step_validations", "Get validation outcomes for a step.", "steps:read",
			objectSchema([]string{"step_id"}, map[string]any{"step_id": stringSchema("Step identifier.")}),
			func(args map[string]any) (string, []byte, *apiError) {
				stepID, apiErr := requiredString(args, "step_id")
				if apiErr != nil {
					return "", nil, apiErr
				}
				return fmt.Sprintf("/api/v2/steps/%s/validations", stepID), nil, nil
			}),
		"codencer.approve_gate": runtimeProxyTool(server, "codencer.approve_gate", "Approve a pending gate for a shared instance.", "gates:write",
			objectSchema([]string{"instance_id", "gate_id"}, map[string]any{
				"instance_id": stringSchema("Target shared instance identifier."),
				"gate_id":     stringSchema("Gate identifier."),
			}), func(args map[string]any) (string, string, []byte, *apiError) {
				instanceID, apiErr := requiredString(args, "instance_id")
				if apiErr != nil {
					return "", "", nil, apiErr
				}
				gateID, apiErr := requiredString(args, "gate_id")
				if apiErr != nil {
					return "", "", nil, apiErr
				}
				return instanceID, fmt.Sprintf("/api/v2/gates/%s/approve", gateID), nil, nil
			}),
		"codencer.reject_gate": runtimeProxyTool(server, "codencer.reject_gate", "Reject a pending gate for a shared instance.", "gates:write",
			objectSchema([]string{"instance_id", "gate_id"}, map[string]any{
				"instance_id": stringSchema("Target shared instance identifier."),
				"gate_id":     stringSchema("Gate identifier."),
			}), func(args map[string]any) (string, string, []byte, *apiError) {
				instanceID, apiErr := requiredString(args, "instance_id")
				if apiErr != nil {
					return "", "", nil, apiErr
				}
				gateID, apiErr := requiredString(args, "gate_id")
				if apiErr != nil {
					return "", "", nil, apiErr
				}
				return instanceID, fmt.Sprintf("/api/v2/gates/%s/reject", gateID), nil, nil
			}),
		"codencer.abort_run": runtimeProxyTool(server, "codencer.abort_run", "Abort a run on a shared instance.", "runs:write",
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
		"codencer.retry_step": runtimeProxyTool(server, "codencer.retry_step", "Retry a step on a shared instance.", "steps:write",
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
				return instanceID, fmt.Sprintf("/api/v2/steps/%s/retry", stepID), nil, nil
			}),
	}
}

func runtimeProxyTool(server *mcpServer, name, description, scope string, schema map[string]any, route func(args map[string]any) (string, string, []byte, *apiError)) mcpTool {
	return mcpTool{
		Name:        name,
		Description: description,
		Scope:       scope,
		InputSchema: schema,
		Invoke: func(ctx context.Context, token *APIToken, args map[string]any) (mcpToolResult, *apiError) {
			instanceID, path, body, apiErr := route(args)
			if apiErr != nil {
				return mcpToolResult{}, apiErr
			}
			instance, apiErr := server.loadAuthorizedInstance(ctx, token, instanceID)
			if apiErr != nil {
				return mcpToolResult{}, apiErr
			}
			_, _, responseBody, err := server.callRuntimeRoute(ctx, token, methodForPath(path, body), path, body, []string{instance.ID}, scope)
			if err != nil {
				return mcpToolResult{}, err
			}
			return successToolResult(description, decodeMCPPayload(responseBody)), nil
		},
	}
}

func routedProxyTool(server *mcpServer, name, description, scope string, schema map[string]any, route func(args map[string]any) (string, []byte, *apiError)) mcpTool {
	return mcpTool{
		Name:        name,
		Description: description,
		Scope:       scope,
		InputSchema: schema,
		Invoke: func(ctx context.Context, token *APIToken, args map[string]any) (mcpToolResult, *apiError) {
			path, body, apiErr := route(args)
			if apiErr != nil {
				return mcpToolResult{}, apiErr
			}
			_, _, responseBody, err := callRoutedTool(server, ctx, token, path, body, scope)
			if err != nil {
				return mcpToolResult{}, err
			}
			return successToolResult(description, decodeMCPPayload(responseBody)), nil
		},
	}
}

func callRoutedTool(server *mcpServer, ctx context.Context, token *APIToken, path string, body []byte, scope string) (int, http.Header, []byte, *apiError) {
	instanceIDs, apiErr := server.authorizedRuntimeInstanceIDs(ctx, token)
	if apiErr != nil {
		return 0, nil, nil, apiErr
	}
	return server.callRuntimeRoute(ctx, token, methodForPath(path, body), path, body, instanceIDs, scope)
}

func decodeMCPPayload(responseBody []byte) any {
	if len(responseBody) == 0 {
		return map[string]any{"ok": true}
	}
	var payload any
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return map[string]any{"raw": string(responseBody)}
	}
	return payload
}

func methodForPath(path string, body []byte) string {
	method := http.MethodGet
	if body != nil {
		method = http.MethodPost
	}
	if strings.HasSuffix(path, "/abort") || strings.HasSuffix(path, "/approve") || strings.HasSuffix(path, "/reject") || strings.HasSuffix(path, "/retry") || strings.HasSuffix(path, "/wait") {
		method = http.MethodPost
	}
	return method
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
	if !ok || strings.TrimSpace(value) == "" {
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

func errorToolResult(code, message string) mcpToolResult {
	return mcpToolResult{
		IsError: true,
		Content: []map[string]string{{"type": "text", "text": message}},
		StructuredContent: map[string]any{
			"error": map[string]any{"code": code, "message": message},
		},
	}
}

func successToolResult(summary string, payload any) mcpToolResult {
	result := mcpToolResult{StructuredContent: payload}
	if summary != "" {
		result.Content = []map[string]string{{"type": "text", "text": summary}}
	}
	return result
}

func artifactContentPayload(contentType string, body []byte) map[string]any {
	payload := map[string]any{
		"content_type": contentType,
	}
	if len(body) == 0 {
		payload["encoding"] = "utf-8"
		payload["content"] = ""
		return payload
	}
	if strings.HasPrefix(strings.ToLower(contentType), "text/") || strings.Contains(strings.ToLower(contentType), "json") || strings.Contains(strings.ToLower(contentType), "yaml") {
		payload["encoding"] = "utf-8"
		payload["content"] = string(body)
		return payload
	}
	payload["encoding"] = "base64"
	payload["content"] = base64.StdEncoding.EncodeToString(body)
	return payload
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

func stringSchema(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func intSchema(description string) map[string]any {
	return map[string]any{"type": "integer", "description": description}
}

func boolSchema(description string) map[string]any {
	return map[string]any{"type": "boolean", "description": description}
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
