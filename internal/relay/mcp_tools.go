package relay

import (
	"context"
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
	Invoke      func(ctx context.Context, principal *plannerPrincipal, args map[string]any) (mcpToolResult, *apiError)
}

func (t mcpTool) instanceID(args map[string]any) string {
	value, _ := args["instance_id"].(string)
	return value
}

func buildMCPTools(server *mcpServer) map[string]mcpTool {
	return map[string]mcpTool{
		"codencer.list_instances": {
			Name:        "codencer.list_instances",
			Description: "List shared Codencer instances available through the relay.",
			Scope:       "instances:read",
			InputSchema: objectSchema(nil, nil),
			Invoke: func(ctx context.Context, principal *plannerPrincipal, args map[string]any) (mcpToolResult, *apiError) {
				status, _, body, err := server.callPlannerRoute(ctx, authHeaderForPrincipal(principal, server.relay.cfg), http.MethodGet, "/api/v2/instances", nil)
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
				status, _, body, err := server.callPlannerRoute(ctx, authHeaderForPrincipal(principal, server.relay.cfg), http.MethodGet, fmt.Sprintf("/api/v2/instances/%s", instanceID), nil)
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
				_, headers, body, err := server.callPlannerRoute(ctx, authHeaderForPrincipal(principal, server.relay.cfg), http.MethodGet, fmt.Sprintf("/api/v2/artifacts/%s/content", artifactID), nil)
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
			_, _, responseBody, err := server.callPlannerRoute(ctx, authHeaderForPrincipal(principal, server.relay.cfg), method, path, body)
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

func authHeaderForPrincipal(principal *plannerPrincipal, cfg *Config) string {
	if principal == nil {
		return ""
	}
	for _, candidate := range cfg.PlannerTokens {
		if candidate.Name == principal.Name {
			return "Bearer " + candidate.Token
		}
	}
	if cfg.PlannerToken != "" {
		return "Bearer " + cfg.PlannerToken
	}
	return ""
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
			"version":               stringSchema("Task contract version."),
			"project_id":            stringSchema("Project identifier."),
			"run_id":                stringSchema("Optional run identifier."),
			"phase_id":              stringSchema("Optional phase identifier."),
			"step_id":               stringSchema("Optional step identifier."),
			"title":                 stringSchema("Optional human-readable title."),
			"goal":                  stringSchema("Primary instruction for the adapter."),
			"context":               objectSchema(nil, map[string]any{"summary": stringSchema("Optional contextual summary.")}),
			"constraints":           map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"allowed_paths":         map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"forbidden_paths":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"validations":           map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
			"acceptance":            map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"stop_conditions":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"policy_bundle":         stringSchema("Optional policy bundle."),
			"adapter_profile":       stringSchema("Preferred adapter profile."),
			"timeout_seconds":       intSchema("Optional task timeout in seconds."),
			"is_simulation":         boolSchema("Simulation flag."),
			"submission_provenance": map[string]any{"type": "object"},
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
