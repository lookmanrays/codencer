package gateway

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

type relayProject struct {
	ProjectID      string            `json:"project_id"`
	Name           string            `json:"name,omitempty"`
	Online         bool              `json:"online"`
	Status         string            `json:"status,omitempty"`
	Locations      []projectLocation `json:"locations,omitempty"`
	DefaultAdapter string            `json:"default_adapter,omitempty"`
	AdapterProfile string            `json:"adapter_profile,omitempty"`
}

type projectLocation struct {
	MachineID    string `json:"machine_id,omitempty"`
	HostLabel    string `json:"host_label,omitempty"`
	Hostname     string `json:"hostname,omitempty"`
	ConnectorID  string `json:"connector_id,omitempty"`
	InstanceID   string `json:"instance_id,omitempty"`
	RepoLabel    string `json:"repo_label,omitempty"`
	RepoRootHash string `json:"repo_root_hash,omitempty"`
	Online       bool   `json:"online"`
	Status       string `json:"status,omitempty"`
}

type aggregatedProject struct {
	ProjectID      string                `json:"project_id"`
	Name           string                `json:"name,omitempty"`
	DefaultAdapter string                `json:"default_adapter,omitempty"`
	AdapterProfile string                `json:"adapter_profile,omitempty"`
	RelayProfiles  []projectRelayProfile `json:"relay_profiles"`
}

type projectRelayProfile struct {
	RelayProfileID string            `json:"relay_profile_id"`
	Name           string            `json:"name,omitempty"`
	Status         string            `json:"status"`
	DefaultAdapter string            `json:"default_adapter,omitempty"`
	AdapterProfile string            `json:"adapter_profile,omitempty"`
	Locations      []projectLocation `json:"locations,omitempty"`
}

type relayProjectMatch struct {
	Profile RelayProfile
	Project relayProject
}

func buildTools(server *Server) map[string]Tool {
	return map[string]Tool{
		"codencer.list_relays": {
			Name:           "codencer.list_relays",
			Description:    "List Gateway relay profiles available to the official Codencer connector.",
			InputSchema:    objectSchema(nil, nil),
			ReadOnly:       true,
			RequiredScopes: []string{"projects:read"},
			Invoke: func(ctx context.Context, principal *authPrincipal, args map[string]any) (ToolResult, *apiError) {
				profiles, apiErr := server.relayProfiles(ctx, principal)
				if apiErr != nil {
					return ToolResult{}, apiErr
				}
				relays := make([]map[string]any, 0, len(profiles))
				for _, profile := range profiles {
					payload := relayStatusMap(profile)
					payload["status"] = server.relayAvailability(ctx, profile)
					relays = append(relays, payload)
				}
				return successToolResult("Listed Gateway relay profiles.", map[string]any{"relays": relays}), nil
			},
		},
		"codencer.get_relay": {
			Name:           "codencer.get_relay",
			Description:    "Get one Gateway relay profile without exposing backend bearer tokens.",
			InputSchema:    objectSchema([]string{"relay_profile_id"}, map[string]any{"relay_profile_id": stringSchema("Gateway relay profile id.")}),
			ReadOnly:       true,
			RequiredScopes: []string{"projects:read"},
			Invoke: func(ctx context.Context, principal *authPrincipal, args map[string]any) (ToolResult, *apiError) {
				profile, apiErr := server.profileByID(ctx, principal, requiredStringValue(args, "relay_profile_id"))
				if apiErr != nil {
					return ToolResult{}, apiErr
				}
				payload := relayStatusMap(profile)
				payload["status"] = server.relayAvailability(ctx, profile)
				return successToolResult("Fetched Gateway relay profile.", payload), nil
			},
		},
		"codencer.list_projects": {
			Name:           "codencer.list_projects",
			Description:    "Aggregate shared Codencer projects across enabled backend Relays.",
			InputSchema:    objectSchema(nil, nil),
			ReadOnly:       true,
			RequiredScopes: []string{"projects:read"},
			Invoke: func(ctx context.Context, principal *authPrincipal, args map[string]any) (ToolResult, *apiError) {
				projects, relayErrors := server.aggregateProjects(ctx, principal)
				return successToolResult("Listed projects through Codencer Gateway.", map[string]any{"projects": projects, "relay_errors": relayErrors}), nil
			},
		},
		"codencer.get_project": {
			Name:           "codencer.get_project",
			Description:    "Get a shared project through the Gateway, selecting a relay profile when needed.",
			InputSchema:    withSelectorSchema(objectSchema([]string{"project_id"}, map[string]any{"project_id": stringSchema("Project id.")})),
			ReadOnly:       true,
			RequiredScopes: []string{"projects:read"},
			Invoke: func(ctx context.Context, principal *authPrincipal, args map[string]any) (ToolResult, *apiError) {
				projectID, apiErr := requiredString(args, "project_id")
				if apiErr != nil {
					return ToolResult{}, apiErr
				}
				match, apiErr := server.resolveProject(ctx, principal, projectID, args, false)
				if apiErr != nil {
					return ToolResult{}, apiErr
				}
				return successToolResult("Fetched Gateway project.", gatewayProjectPayload(match.Profile, match.Project)), nil
			},
		},
		"codencer.list_project_locations": {
			Name:           "codencer.list_project_locations",
			Description:    "List safe machine/location metadata for a project across Gateway relay profiles.",
			InputSchema:    withSelectorSchema(objectSchema([]string{"project_id"}, map[string]any{"project_id": stringSchema("Project id.")})),
			ReadOnly:       true,
			RequiredScopes: []string{"projects:read"},
			Invoke: func(ctx context.Context, principal *authPrincipal, args map[string]any) (ToolResult, *apiError) {
				projectID, apiErr := requiredString(args, "project_id")
				if apiErr != nil {
					return ToolResult{}, apiErr
				}
				locations, relayErrors := server.projectLocations(ctx, principal, projectID, args)
				return successToolResult("Listed project locations.", map[string]any{"project_id": projectID, "locations": locations, "relay_errors": relayErrors}), nil
			},
		},
		"codencer.start_project_run":            server.projectForwardTool("codencer.start_project_run", "Start an async run for a shared project through the selected Gateway relay.", []string{"project_id"}, startProjectRunRoute),
		"codencer.list_project_runs":            server.projectForwardTool("codencer.list_project_runs", "List async runs for a shared project through the selected Gateway relay.", []string{"project_id"}, listProjectRunsRoute),
		"codencer.get_project_run":              server.projectForwardTool("codencer.get_project_run", "Get async run status for a shared project through the selected Gateway relay.", []string{"project_id", "run_id"}, getProjectRunRoute),
		"codencer.get_project_run_status":       server.projectForwardTool("codencer.get_project_run_status", "Alias for codencer.get_project_run.", []string{"project_id", "run_id"}, getProjectRunRoute),
		"codencer.submit_project_task":          server.projectForwardTool("codencer.submit_project_task", "Submit one approved task through the selected Gateway relay without waiting for terminal evidence.", []string{"project_id"}, submitProjectTaskRoute(false)),
		"codencer.run_project_manifest":         server.projectForwardTool("codencer.run_project_manifest", "Run a project manifest through the selected Gateway relay.", []string{"project_id"}, runProjectManifestRoute),
		"codencer.submit_project_task_and_wait": server.projectForwardTool("codencer.submit_project_task_and_wait", "Submit one approved task through the selected Gateway relay and wait for evidence.", []string{"project_id"}, submitProjectTaskRoute(true)),
		"codencer.get_run_report": server.projectForwardTool("codencer.get_run_report", "Get a normalized run-plan report through the selected Gateway relay.", []string{"project_id", "run_id"}, func(args map[string]any) (string, []byte, *apiError) {
			projectID, apiErr := requiredString(args, "project_id")
			if apiErr != nil {
				return "", nil, apiErr
			}
			runID, apiErr := requiredString(args, "run_id")
			if apiErr != nil {
				return "", nil, apiErr
			}
			return "/api/v2/projects/" + projectID + "/reports/run-plans/" + runID, nil, nil
		}),
		"codencer.get_gateway_run_events":     server.gatewayRunEventsTool(),
		"codencer.respond_to_human_interrupt": server.humanInterruptResponseTool(),
		"codencer.cancel_project_run":         server.projectForwardTool("codencer.cancel_project_run", "Cancel a shared project run through the selected Gateway relay.", []string{"project_id", "run_id"}, cancelProjectRunRoute),
		"codencer.resume_project_run":         server.unsupportedProjectLifecycleTool("codencer.resume_project_run", "Return an explicit capability blocker for project-run resume when the selected route cannot resume safely.", "resume_project_run"),
		"codencer.get_blocker": {
			Name:        "codencer.get_blocker",
			Description: "Read the blocker from a Gateway-routed project run report.",
			InputSchema: withSelectorSchema(objectSchema([]string{"project_id", "run_id"}, map[string]any{
				"project_id": stringSchema("Project id."),
				"run_id":     stringSchema("Run id."),
			})),
			ReadOnly:       true,
			RequiredScopes: []string{"reports:read", "runs:read"},
			Invoke: func(ctx context.Context, principal *authPrincipal, args map[string]any) (ToolResult, *apiError) {
				projectID, apiErr := requiredString(args, "project_id")
				if apiErr != nil {
					return ToolResult{}, apiErr
				}
				runID, apiErr := requiredString(args, "run_id")
				if apiErr != nil {
					return ToolResult{}, apiErr
				}
				match, apiErr := server.resolveProject(ctx, principal, projectID, args, false)
				if apiErr != nil {
					return ToolResult{}, apiErr
				}
				path := appendSelector("/api/v2/projects/"+projectID+"/reports/run-plans/"+runID, args)
				_, body, apiErr := server.callRelay(ctx, match.Profile, http.MethodGet, path, nil)
				if apiErr != nil {
					return ToolResult{}, apiErr
				}
				var report map[string]any
				_ = json.Unmarshal(body, &report)
				payload := map[string]any{"project_id": projectID, "run_id": runID, "relay_profile_id": match.Profile.ID, "blocker": nil}
				if report != nil {
					payload["report_status"] = report["status"]
					if blocker, ok := report["blocker"]; ok {
						payload["blocker"] = blocker
					}
				}
				return successToolResult("Fetched Gateway blocker.", payload), nil
			},
		},
	}
}

func (s *Server) projectForwardTool(name, description string, required []string, route func(map[string]any) (string, []byte, *apiError)) Tool {
	properties := map[string]any{
		"project_id": stringSchema("Project id."),
		"run_id":     stringSchema("Run id."),
	}
	if name == "codencer.run_project_manifest" {
		properties["manifest"] = objectSchema(nil, nil)
		properties["manifest_text"] = stringSchema("YAML or JSON manifest text.")
		properties["manifest_name"] = stringSchema("Manifest display name.")
		properties["wait"] = boolSchema("Wait for manifest completion.")
	}
	if name == "codencer.submit_project_task" || name == "codencer.submit_project_task_and_wait" {
		properties["goal"] = stringSchema("Direct task goal.")
		properties["prompt"] = stringSchema("Prompt text.")
		properties["task"] = objectSchema(nil, nil)
		properties["profile"] = stringSchema("Planner-facing profile id.")
		properties["adapter_profile"] = stringSchema("Daemon adapter profile.")
		properties["title"] = stringSchema("Task title.")
		properties["timeout_seconds"] = intSchema("Timeout in seconds.")
	}
	return Tool{
		Name:           name,
		Description:    description,
		InputSchema:    withSelectorSchema(objectSchema(required, properties)),
		ReadOnly:       isReadOnlyProjectForwardTool(name),
		RequiredScopes: forwardToolScopes(name),
		Invoke: func(ctx context.Context, principal *authPrincipal, args map[string]any) (ToolResult, *apiError) {
			projectID, apiErr := requiredString(args, "project_id")
			if apiErr != nil {
				return ToolResult{}, apiErr
			}
			runRecord := RunRecord{}
			auditMetadata := map[string]any{"project_id": projectID}
			recordRun := shouldRecordRunForTool(name)
			if recordRun {
				runRecord, auditMetadata = s.beginRunRecord(ctx, principal, projectID, name, args)
				s.recordGatewayAuditWithMetadata(ctx, principal, "task_submitted", "Submitted "+executionKindLabel(name)+" for project "+projectID, auditMetadata)
			}
			match, apiErr := s.resolveProject(ctx, principal, projectID, args, true)
			if apiErr != nil {
				if recordRun {
					runRecord.Status = "failed"
					runRecord.ResultSummary = "Route resolution failed: " + apiErr.Code
					runRecord, auditMetadata = s.finishRunRecord(ctx, runRecord, map[string]any{"status": "failed", "summary": runRecord.ResultSummary}, "unavailable")
				}
				s.recordGatewayAuditWithMetadata(ctx, principal, "run_failed", "Route resolution failed for project "+projectID+": "+apiErr.Code, auditMetadata)
				return ToolResult{}, apiErr
			}
			if recordRun {
				runRecord, auditMetadata = s.applyRouteToRunRecord(ctx, runRecord, principal, match, args)
				s.recordProjectRouteAudit(ctx, principal, match, args, auditMetadata)
			}
			path, body, apiErr := route(args)
			if apiErr != nil {
				if recordRun {
					runRecord.Status = "failed"
					runRecord.ResultSummary = "Run request validation failed: " + apiErr.Code
					runRecord, auditMetadata = s.finishRunRecord(ctx, runRecord, map[string]any{"status": "failed", "summary": runRecord.ResultSummary}, "unavailable")
				}
				s.recordGatewayAuditWithMetadata(ctx, principal, "run_failed", "Run request validation failed for project "+projectID+": "+apiErr.Code, auditMetadata)
				return ToolResult{}, apiErr
			}
			path = appendSelector(path, args)
			if name == "codencer.cancel_project_run" {
				auditMetadata = projectLifecycleAuditMetadata(match, args)
				s.recordGatewayAuditWithMetadata(ctx, principal, "cancel_project_run_requested", "Requested cancel_project_run for run "+requiredStringValue(args, "run_id")+" in project "+projectID, auditMetadata)
			}
			method := http.MethodGet
			if body != nil {
				method = http.MethodPost
			}
			if recordRun {
				s.recordGatewayAuditWithMetadata(ctx, principal, "run_started", "Started "+executionKindLabel(name)+" for project "+projectID, auditMetadata)
			}
			_, response, apiErr := s.callRelay(ctx, match.Profile, method, path, body)
			payload, apiErr := responsePayload(match.Profile, response, apiErr)
			if apiErr != nil {
				eventType := "run_failed"
				if name == "codencer.get_run_report" {
					eventType = "report_read"
				} else if name == "codencer.cancel_project_run" {
					auditMetadata = projectLifecycleAuditMetadata(match, args)
				} else {
					runRecord.Status = "failed"
					runRecord.ResultSummary = "Gateway relay call failed: " + apiErr.Code
					runRecord, auditMetadata = s.finishRunRecord(ctx, runRecord, map[string]any{"status": "failed", "summary": runRecord.ResultSummary}, "unavailable")
				}
				s.recordGatewayAuditWithMetadata(ctx, principal, eventType, "Gateway relay call failed for project "+projectID+": "+apiErr.Code, auditMetadata)
				return ToolResult{}, apiErr
			}
			if name == "codencer.get_run_report" {
				record, metadata := s.refreshRunRecordFromReport(ctx, principal, match, args, payload)
				_ = record
				s.recordTerminalRunAuditOnce(ctx, principal, projectID, payload, metadata)
				s.recordGatewayAuditWithMetadata(ctx, principal, "report_read", "Read run report "+requiredStringValue(args, "run_id")+" for project "+projectID, metadata)
			} else if name == "codencer.cancel_project_run" {
				record, metadata := s.refreshRunRecordFromLifecyclePayload(ctx, principal, match, args, payload, reportStatusForPayload(payload, "available"))
				_ = record
				s.recordGatewayAuditWithMetadata(ctx, principal, terminalAuditType(payload), terminalAuditSummary(projectID, payload), metadata)
			} else if recordRun {
				runRecord, auditMetadata = s.finishRunRecord(ctx, runRecord, payload, reportStatusForPayload(payload, "available"))
				if obj, ok := payload.(map[string]any); ok && runRecord.ID != "" {
					obj["run_history_id"] = runRecord.ID
					payload = obj
				}
				eventType := terminalAuditType(payload)
				if eventType != "run_started" {
					s.recordGatewayAuditWithMetadata(ctx, principal, eventType, terminalAuditSummary(projectID, payload), auditMetadata)
				}
				if eventType == "blocker" {
					s.recordHumanInterruptAudit(ctx, principal, projectID, payload, auditMetadata)
				}
			}
			return successToolResult(description, payload), nil
		},
	}
}

func (s *Server) gatewayRunEventsTool() Tool {
	return Tool{
		Name:        "codencer.get_gateway_run_events",
		Description: "List Gateway-observed audit events for a Gateway run history record.",
		InputSchema: objectSchema([]string{"run_history_id"}, map[string]any{
			"run_history_id": stringSchema("Gateway run history id returned by submit/start/report tools."),
			"limit":          intSchema("Maximum events to return."),
			"offset":         intSchema("Pagination offset."),
		}),
		ReadOnly:       true,
		RequiredScopes: []string{"runs:read", "reports:read"},
		Invoke: func(ctx context.Context, principal *authPrincipal, args map[string]any) (ToolResult, *apiError) {
			if s.store == nil || principal == nil || principal.WorkspaceID == "" {
				return ToolResult{}, &apiError{Status: http.StatusServiceUnavailable, Code: "gateway_store_unavailable", Message: "Gateway run events require the Gateway store"}
			}
			runHistoryID, apiErr := requiredString(args, "run_history_id")
			if apiErr != nil {
				return ToolResult{}, apiErr
			}
			record, err := s.store.GetRunRecord(ctx, principal.WorkspaceID, runHistoryID)
			if err == sql.ErrNoRows {
				return ToolResult{}, &apiError{Status: http.StatusNotFound, Code: "run_not_found", Message: "run history record not found"}
			}
			if err != nil {
				return ToolResult{}, &apiError{Status: http.StatusInternalServerError, Code: "gateway_store_error", Message: err.Error()}
			}
			limit := intArg(args, "limit", 100, 1, 200)
			offset := intArg(args, "offset", 0, 0, 1000000)
			events, err := s.store.ListAuditEvents(ctx, principal.WorkspaceID, AuditEventFilters{
				RunHistoryID: record.ID,
				Limit:        limit + 1,
				Offset:       offset,
			})
			if err != nil {
				return ToolResult{}, &apiError{Status: http.StatusInternalServerError, Code: "gateway_store_error", Message: err.Error()}
			}
			hasMore := len(events) > limit
			if hasMore {
				events = events[:limit]
			}
			sort.SliceStable(events, func(i, j int) bool {
				return events[i].CreatedAt.Before(events[j].CreatedAt)
			})
			return successToolResult("Listed Gateway run events.", map[string]any{
				"run_history_id": runHistoryID,
				"run_id":         record.RunID,
				"project_id":     record.ProjectID,
				"events":         events,
				"groups":         groupAuditEvents(events),
				"pagination":     buildPagination(limit, offset, hasMore),
			}), nil
		},
	}
}

func (s *Server) humanInterruptResponseTool() Tool {
	return Tool{
		Name:        "codencer.respond_to_human_interrupt",
		Description: "Record an explicit operator response to a Gateway-observed human interrupt without claiming automatic resume support.",
		InputSchema: objectSchema([]string{"run_history_id", "response"}, map[string]any{
			"run_history_id": stringSchema("Gateway run history id returned by submit/start/report tools."),
			"response":       stringSchema("Operator answer, approval, denial, or decision text."),
			"response_type":  stringSchema("Optional response type, such as answer, approve, deny, or decision."),
			"follow_up":      stringSchema("Optional requested follow-up, such as resume, cancel, or start_new_task."),
			"reason":         stringSchema("Optional operator/planner reason."),
		}),
		ReadOnly:       false,
		RequiredScopes: []string{"projects:read", "runs:write"},
		Invoke: func(ctx context.Context, principal *authPrincipal, args map[string]any) (ToolResult, *apiError) {
			if s.store == nil || principal == nil || principal.WorkspaceID == "" {
				return ToolResult{}, &apiError{Status: http.StatusServiceUnavailable, Code: "gateway_store_unavailable", Message: "Gateway human interrupt response requires the Gateway store"}
			}
			runHistoryID, apiErr := requiredString(args, "run_history_id")
			if apiErr != nil {
				return ToolResult{}, apiErr
			}
			record, err := s.store.GetRunRecord(ctx, principal.WorkspaceID, runHistoryID)
			if err == sql.ErrNoRows {
				return ToolResult{}, &apiError{Status: http.StatusNotFound, Code: "run_not_found", Message: "run history record not found"}
			}
			if err != nil {
				return ToolResult{}, &apiError{Status: http.StatusInternalServerError, Code: "gateway_store_error", Message: err.Error()}
			}
			payload, apiErr := s.recordHumanInterruptResponse(ctx, principal, record, args)
			if apiErr != nil {
				return ToolResult{}, apiErr
			}
			return successToolResult("Recorded human interrupt response.", payload), nil
		},
	}
}

func (s *Server) recordHumanInterruptResponse(ctx context.Context, principal *authPrincipal, record RunRecord, args map[string]any) (map[string]any, *apiError) {
	response := safeExcerpt(firstNonEmpty(
		stringValueFromAny(args["response"]),
		stringValueFromAny(args["answer"]),
		stringValueFromAny(args["decision"]),
		stringValueFromAny(args["action"]),
	), 1200)
	if response == "" {
		return nil, &apiError{Status: http.StatusBadRequest, Code: "human_response_required", Message: "response is required"}
	}
	responseType := safeExcerpt(firstNonEmpty(
		stringValueFromAny(args["response_type"]),
		stringValueFromAny(args["action"]),
		"response",
	), 120)
	followUp := safeExcerpt(firstNonEmpty(
		stringValueFromAny(args["follow_up"]),
		stringValueFromAny(args["next_action"]),
	), 120)
	reason := safeExcerpt(stringValueFromAny(args["reason"]), 600)

	metadata := runAuditMetadata(record)
	metadata["interrupt_status"] = "responded"
	metadata["response_type"] = responseType
	metadata["operator_response"] = response
	if followUp != "" {
		metadata["follow_up"] = followUp
	}
	if reason != "" {
		metadata["reason"] = reason
	}
	s.recordGatewayAuditWithMetadata(ctx, principal, "human_interrupt_responded", "Recorded human interrupt response for run "+firstNonEmpty(record.RunID, record.ID)+" in project "+record.ProjectID, metadata)

	nextActions := map[string]any{
		"resume_supported":          false,
		"resume_operation":          "codencer.resume_project_run",
		"cancel_supported":          true,
		"cancel_operation":          "codencer.cancel_project_run",
		"status_read_tool":          "codencer.get_project_run_status",
		"report_read_tool":          "codencer.get_run_report",
		"events_read_tool":          "codencer.get_gateway_run_events",
		"planner_decision_required": true,
	}
	payload := map[string]any{
		"ok":             true,
		"status":         "human_interrupt_response_recorded",
		"run_history_id": record.ID,
		"run_id":         record.RunID,
		"project_id":     record.ProjectID,
		"response": map[string]any{
			"type":              responseType,
			"operator_response": response,
		},
		"next_actions": nextActions,
	}
	if followUp != "" {
		payload["follow_up"] = followUp
	}
	if reason != "" {
		payload["reason"] = reason
	}
	return payload, nil
}

func (s *Server) unsupportedProjectLifecycleTool(name, description, operation string) Tool {
	return Tool{
		Name:        name,
		Description: description,
		InputSchema: withSelectorSchema(objectSchema([]string{"project_id", "run_id"}, map[string]any{
			"project_id":     stringSchema("Project id."),
			"run_id":         stringSchema("Run id."),
			"run_history_id": stringSchema("Optional Gateway run history id for audit correlation."),
			"reason":         stringSchema("Optional operator/planner reason."),
		})),
		ReadOnly:       false,
		RequiredScopes: []string{"projects:read", "runs:write"},
		Invoke: func(ctx context.Context, principal *authPrincipal, args map[string]any) (ToolResult, *apiError) {
			projectID, apiErr := requiredString(args, "project_id")
			if apiErr != nil {
				return ToolResult{}, apiErr
			}
			runID, apiErr := requiredString(args, "run_id")
			if apiErr != nil {
				return ToolResult{}, apiErr
			}
			match, apiErr := s.resolveProject(ctx, principal, projectID, args, true)
			if apiErr != nil {
				return ToolResult{}, apiErr
			}
			metadata := map[string]any{
				"project_id":       projectID,
				"run_id":           runID,
				"run_history_id":   strings.TrimSpace(stringArg(args, "run_history_id")),
				"operation":        operation,
				"relay_profile_id": match.Profile.ID,
			}
			s.recordGatewayAuditWithMetadata(ctx, principal, operation+"_requested", "Requested "+operation+" for run "+runID+" in project "+projectID, metadata)
			blocker := map[string]any{
				"type":                      "unsupported_operation",
				"operation":                 operation,
				"project_id":                projectID,
				"run_id":                    runID,
				"retryable":                 false,
				"planner_decision_required": true,
				"supported":                 false,
				"observed_facts": []string{
					"Gateway project-level " + operation + " is not supported by the selected Relay route yet.",
					"Use run status/events/report to inspect current state, or start a new run with explicit planner approval.",
				},
			}
			if reason := strings.TrimSpace(stringArg(args, "reason")); reason != "" {
				blocker["reason"] = reason
			}
			payload := map[string]any{
				"ok":      false,
				"status":  "blocked",
				"blocker": blocker,
			}
			return successToolResult("Returned structured lifecycle capability blocker.", payload), nil
		},
	}
}

func responsePayload(profile RelayProfile, response []byte, apiErr *apiError) (any, *apiError) {
	if apiErr != nil {
		return nil, apiErr
	}
	var payload any
	if len(response) == 0 {
		payload = map[string]any{"ok": true}
	} else if err := json.Unmarshal(response, &payload); err != nil {
		payload = map[string]any{"raw": string(response)}
	}
	if obj, ok := payload.(map[string]any); ok {
		obj["relay_profile_id"] = profile.ID
		payload = obj
	}
	return payload, nil
}

func executionKindLabel(value string) string {
	switch value {
	case "manifest", "codencer.run_project_manifest":
		return "manifest / run plan"
	default:
		return "simple task"
	}
}

func selectedProjectLocation(project relayProject, args map[string]any) projectLocation {
	locations := filterLocations(project.Locations, args)
	for _, location := range locations {
		if location.Online {
			return location
		}
	}
	if len(locations) > 0 {
		return locations[0]
	}
	return projectLocation{}
}

func projectLifecycleAuditMetadata(match relayProjectMatch, args map[string]any) map[string]any {
	metadata := map[string]any{
		"project_id":       match.Project.ProjectID,
		"run_id":           requiredStringValue(args, "run_id"),
		"relay_profile_id": match.Profile.ID,
	}
	if runHistoryID := strings.TrimSpace(stringArg(args, "run_history_id")); runHistoryID != "" {
		metadata["run_history_id"] = runHistoryID
	}
	location := selectedProjectLocation(match.Project, args)
	for key, value := range map[string]string{
		"connector_id": location.ConnectorID,
		"machine_id":   location.MachineID,
		"host_label":   location.HostLabel,
	} {
		if strings.TrimSpace(value) != "" {
			metadata[key] = strings.TrimSpace(value)
		}
	}
	return metadata
}

func resolvedExecutorLabel(project relayProject, args map[string]any) string {
	return firstNonEmpty(
		strings.TrimSpace(stringArg(args, "profile")),
		strings.TrimSpace(stringArg(args, "adapter_profile")),
		strings.TrimSpace(project.AdapterProfile),
		strings.TrimSpace(project.DefaultAdapter),
		"project default",
	)
}

func terminalAuditType(payload any) string {
	obj, _ := payload.(map[string]any)
	if obj == nil {
		return "run_completed"
	}
	if blocker, ok := obj["blocker"]; ok && blocker != nil {
		return "blocker"
	}
	if task, _ := obj["task"].(map[string]any); task != nil {
		if blocker, ok := task["blocker"]; ok && blocker != nil {
			return "blocker"
		}
	}
	status := strings.ToLower(firstNonEmpty(
		stringValueFromAny(obj["status"]),
		nestedString(obj, "task", "status"),
		nestedString(obj, "run", "state"),
	))
	switch status {
	case "submitted", "started", "starting", "queued", "pending", "running", "in_progress", "validating":
		return "run_started"
	case "cancel_requested", "cancelling", "canceling":
		return "run_cancel_requested"
	case "cancelled", "canceled":
		return "run_cancelled"
	case "failed", "failed_adapter", "failed_bridge", "failed_validation", "timeout", "adapter_error", "bridge_error":
		return "run_failed"
	case "blocked", "question", "manual_approval_required", "needs_approval", "needs_manual_attention", "permission_request_required", "unsafe_action", "validation_failed":
		return "blocker"
	default:
		return "run_completed"
	}
}

func terminalAuditSummary(projectID string, payload any) string {
	runID := runIDFromPayload(payload)
	outcome := "completed"
	switch terminalAuditType(payload) {
	case "run_started":
		outcome = "started"
	case "run_failed":
		outcome = "failed"
	case "blocker":
		outcome = "reached blocker"
	case "run_cancel_requested":
		outcome = "cancel requested"
	case "run_cancelled":
		outcome = "cancelled"
	}
	if runID == "" {
		return "Run " + outcome + " for project " + projectID
	}
	return "Run " + runID + " " + outcome + " for project " + projectID
}

func (s *Server) recordHumanInterruptAudit(ctx context.Context, principal *authPrincipal, projectID string, payload any, baseMetadata map[string]any) {
	interrupt := humanInterruptFromPayload(payload)
	if interrupt == nil {
		return
	}
	metadata := map[string]any{}
	for key, value := range baseMetadata {
		metadata[key] = value
	}
	for key, value := range interrupt {
		metadata[key] = value
	}
	summary := "Human interrupt for project " + projectID
	if prompt, _ := interrupt["prompt"].(string); strings.TrimSpace(prompt) != "" {
		summary = prompt
	}
	s.recordGatewayAuditWithMetadata(ctx, principal, "human_interrupt_created", summary, metadata)
}

func humanInterruptFromPayload(payload any) map[string]any {
	obj, _ := payload.(map[string]any)
	if obj == nil {
		return nil
	}
	blocker := blockerMapFromPayload(obj)
	if blocker == nil {
		status := strings.ToLower(firstNonEmpty(stringValueFromAny(obj["status"]), nestedString(obj, "task", "status")))
		if status != "blocked" && status != "question" && status != "manual_approval_required" && status != "needs_approval" && status != "needs_manual_attention" {
			return nil
		}
		blocker = map[string]any{"type": status}
	}
	blockerType := firstNonEmpty(
		stringValueFromAny(blocker["type"]),
		stringValueFromAny(blocker["blocker_type"]),
		stringValueFromAny(blocker["reason"]),
	)
	interruptType, action, responses := gatewayHumanInterruptContract(blockerType)
	if interruptType == "" {
		interruptType, action, responses = "executor_specific_human_decision_required", "choose_alternate_action", []string{"cancel", "start_new_task"}
	}
	prompt := firstNonEmpty(
		stringValueFromAny(blocker["message"]),
		stringValueFromAny(blocker["summary"]),
		resultSummaryFromPayload(obj),
	)
	out := map[string]any{
		"interrupt_type":    interruptType,
		"status":            "waiting_for_human",
		"requested_action":  action,
		"allowed_responses": responses,
	}
	if prompt != "" {
		out["prompt"] = safeExcerpt(prompt, 1200)
	}
	if questions := stringSliceValue(blocker["questions"]); len(questions) > 0 {
		out["questions"] = questions
	}
	return out
}

func blockerMapFromPayload(obj map[string]any) map[string]any {
	if blocker, _ := obj["blocker"].(map[string]any); blocker != nil {
		return blocker
	}
	if task, _ := obj["task"].(map[string]any); task != nil {
		if blocker, _ := task["blocker"].(map[string]any); blocker != nil {
			return blocker
		}
	}
	return nil
}

func gatewayHumanInterruptContract(blockerType string) (string, string, []string) {
	switch strings.TrimSpace(blockerType) {
	case "manual_approval_required", "needs_approval", "approval_required", "needs_planner_decision":
		return "planning_approval_required", "approve_or_reject", []string{"approve", "reject", "cancel"}
	case "question", "clarifying_question_required", "needs_manual_attention":
		return "clarifying_question_required", "answer_question", []string{"answer", "cancel"}
	case "unsafe_action", "permission_request_required":
		return "permission_request_required", "confirm_or_deny", []string{"confirm", "deny", "cancel"}
	case "daemon_not_running", "system_action_required", "os_system_human_action_required":
		return "os_system_human_action_required", "start_or_configure_daemon", []string{"retry", "cancel"}
	case "unsupported_operation", "executor_decision_required", "executor_specific_human_decision_required":
		return "executor_specific_human_decision_required", "choose_alternate_action", []string{"cancel", "start_new_task"}
	default:
		return "", "", nil
	}
}

func stringSliceValue(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		out := []string{}
		for _, item := range typed {
			if text := stringValueFromAny(item); text != "" {
				out = append(out, safeExcerpt(text, 600))
			}
		}
		return out
	default:
		return nil
	}
}

func runIDFromPayload(payload any) string {
	obj, _ := payload.(map[string]any)
	if obj == nil {
		return ""
	}
	return firstNonEmpty(
		stringValueFromAny(obj["run_id"]),
		nestedString(obj, "run", "id"),
		nestedString(obj, "task", "run_id"),
	)
}

func nestedString(obj map[string]any, keys ...string) string {
	var current any = obj
	for _, key := range keys {
		next, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = next[key]
	}
	return stringValueFromAny(current)
}

func stringValueFromAny(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func startProjectRunRoute(args map[string]any) (string, []byte, *apiError) {
	projectID, apiErr := requiredString(args, "project_id")
	if apiErr != nil {
		return "", nil, apiErr
	}
	return "/api/v2/projects/" + projectID + "/runs", []byte(`{}`), nil
}

func listProjectRunsRoute(args map[string]any) (string, []byte, *apiError) {
	projectID, apiErr := requiredString(args, "project_id")
	if apiErr != nil {
		return "", nil, apiErr
	}
	return "/api/v2/projects/" + projectID + "/runs", nil, nil
}

func getProjectRunRoute(args map[string]any) (string, []byte, *apiError) {
	projectID, apiErr := requiredString(args, "project_id")
	if apiErr != nil {
		return "", nil, apiErr
	}
	runID, apiErr := requiredString(args, "run_id")
	if apiErr != nil {
		return "", nil, apiErr
	}
	return "/api/v2/projects/" + projectID + "/runs/" + runID, nil, nil
}

func cancelProjectRunRoute(args map[string]any) (string, []byte, *apiError) {
	projectID, apiErr := requiredString(args, "project_id")
	if apiErr != nil {
		return "", nil, apiErr
	}
	runID, apiErr := requiredString(args, "run_id")
	if apiErr != nil {
		return "", nil, apiErr
	}
	payload := map[string]any{}
	copyOptional(payload, args, "reason")
	body, apiErr := jsonBody(payload)
	if apiErr != nil {
		return "", nil, apiErr
	}
	return "/api/v2/projects/" + projectID + "/runs/" + runID + "/cancel", body, nil
}

func submitProjectTaskRoute(wait bool) func(map[string]any) (string, []byte, *apiError) {
	return func(args map[string]any) (string, []byte, *apiError) {
		projectID, apiErr := requiredString(args, "project_id")
		if apiErr != nil {
			return "", nil, apiErr
		}
		payload := map[string]any{"wait": wait}
		copyOptional(payload, args, "run_id", "goal", "prompt", "task", "profile", "adapter_profile", "title", "timeout_seconds")
		body, apiErr := jsonBody(payload)
		if apiErr != nil {
			return "", nil, apiErr
		}
		return "/api/v2/projects/" + projectID + "/submit", body, nil
	}
}

func isReadOnlyProjectForwardTool(name string) bool {
	switch name {
	case "codencer.get_run_report", "codencer.list_project_runs", "codencer.get_project_run", "codencer.get_project_run_status":
		return true
	default:
		return false
	}
}

func shouldRecordRunForTool(name string) bool {
	switch name {
	case "codencer.start_project_run", "codencer.submit_project_task", "codencer.submit_project_task_and_wait", "codencer.run_project_manifest":
		return true
	default:
		return false
	}
}

func runProjectManifestRoute(args map[string]any) (string, []byte, *apiError) {
	projectID, apiErr := requiredString(args, "project_id")
	if apiErr != nil {
		return "", nil, apiErr
	}
	payload := map[string]any{}
	copyOptional(payload, args, "manifest", "manifest_text", "manifest_name", "wait")
	body, apiErr := jsonBody(payload)
	if apiErr != nil {
		return "", nil, apiErr
	}
	return "/api/v2/projects/" + projectID + "/run-plan", body, nil
}

func (s *Server) relayAvailability(ctx context.Context, profile RelayProfile) string {
	if !profile.Enabled {
		return "disabled"
	}
	_, _, apiErr := s.callRelay(ctx, profile, http.MethodGet, "/api/v2/status", nil)
	if apiErr != nil {
		return "unavailable"
	}
	return "available"
}

func (s *Server) relayProfiles(ctx context.Context, principal *authPrincipal) ([]RelayProfile, *apiError) {
	if s.store != nil && principal != nil && principal.WorkspaceID != "" {
		records, err := s.store.ListRelayProfiles(ctx, principal.WorkspaceID)
		if err != nil {
			return nil, &apiError{Status: http.StatusInternalServerError, Code: "gateway_store_error", Message: err.Error()}
		}
		profiles := make([]RelayProfile, 0, len(records))
		for _, record := range records {
			profiles = append(profiles, record.ToRelayProfile())
		}
		return profiles, nil
	}
	return append([]RelayProfile(nil), s.cfg.RelayProfiles...), nil
}

func (s *Server) aggregateProjects(ctx context.Context, principal *authPrincipal) ([]aggregatedProject, []map[string]any) {
	byID := map[string]*aggregatedProject{}
	order := []string{}
	relayErrors := []map[string]any{}
	profiles, apiErr := s.relayProfiles(ctx, principal)
	if apiErr != nil {
		relayErrors = append(relayErrors, map[string]any{"code": apiErr.Code, "message": apiErr.Message})
		return []aggregatedProject{}, relayErrors
	}
	for _, profile := range profiles {
		if !profile.Enabled {
			continue
		}
		projects, apiErr := s.fetchRelayProjects(ctx, profile)
		if apiErr != nil {
			relayErrors = append(relayErrors, map[string]any{"relay_profile_id": profile.ID, "code": apiErr.Code, "message": apiErr.Message})
			continue
		}
		for _, project := range projects {
			current := byID[project.ProjectID]
			if current == nil {
				current = &aggregatedProject{
					ProjectID:      project.ProjectID,
					Name:           project.Name,
					DefaultAdapter: project.DefaultAdapter,
					AdapterProfile: project.AdapterProfile,
				}
				byID[project.ProjectID] = current
				order = append(order, project.ProjectID)
			} else {
				current.DefaultAdapter = firstNonEmpty(current.DefaultAdapter, project.DefaultAdapter)
				current.AdapterProfile = firstNonEmpty(current.AdapterProfile, project.AdapterProfile)
			}
			current.RelayProfiles = append(current.RelayProfiles, projectRelayProfile{
				RelayProfileID: profile.ID,
				Name:           profile.Name,
				Status:         project.Status,
				DefaultAdapter: project.DefaultAdapter,
				AdapterProfile: project.AdapterProfile,
				Locations:      project.Locations,
			})
		}
	}
	sort.Strings(order)
	projects := make([]aggregatedProject, 0, len(order))
	for _, id := range order {
		current := *byID[id]
		sort.Slice(current.RelayProfiles, func(i, j int) bool {
			return current.RelayProfiles[i].RelayProfileID < current.RelayProfiles[j].RelayProfileID
		})
		projects = append(projects, current)
	}
	return projects, relayErrors
}

func (s *Server) fetchRelayProjects(ctx context.Context, profile RelayProfile) ([]relayProject, *apiError) {
	_, body, apiErr := s.callRelay(ctx, profile, http.MethodGet, "/api/v2/projects", nil)
	if apiErr != nil {
		return nil, apiErr
	}
	var projects []relayProject
	if err := json.Unmarshal(body, &projects); err != nil {
		return nil, &apiError{Status: http.StatusBadGateway, Code: "relay_response_invalid", Message: err.Error()}
	}
	return projects, nil
}

func (s *Server) fetchRelayProject(ctx context.Context, profile RelayProfile, projectID string) (relayProject, *apiError) {
	_, body, apiErr := s.callRelay(ctx, profile, http.MethodGet, "/api/v2/projects/"+projectID, nil)
	if apiErr != nil {
		return relayProject{}, apiErr
	}
	var project relayProject
	if err := json.Unmarshal(body, &project); err != nil {
		return relayProject{}, &apiError{Status: http.StatusBadGateway, Code: "relay_response_invalid", Message: err.Error()}
	}
	return project, nil
}

func (s *Server) resolveProject(ctx context.Context, principal *authPrincipal, projectID string, args map[string]any, requireLocationDisambiguation bool) (relayProjectMatch, *apiError) {
	if relayProfileID, _ := args["relay_profile_id"].(string); strings.TrimSpace(relayProfileID) != "" {
		profile, apiErr := s.profileByID(ctx, principal, relayProfileID)
		if apiErr != nil {
			return relayProjectMatch{}, apiErr
		}
		project, apiErr := s.fetchRelayProject(ctx, profile, projectID)
		if apiErr != nil {
			return relayProjectMatch{}, apiErr
		}
		if requireLocationDisambiguation {
			if apiErr := ambiguousLocationBlocker(projectID, profile, project, args); apiErr != nil {
				return relayProjectMatch{}, apiErr
			}
		}
		return relayProjectMatch{Profile: profile, Project: project}, nil
	}
	matches := []relayProjectMatch{}
	relayErrors := []map[string]any{}
	profiles, apiErr := s.relayProfiles(ctx, principal)
	if apiErr != nil {
		return relayProjectMatch{}, apiErr
	}
	for _, profile := range profiles {
		if !profile.Enabled {
			continue
		}
		project, apiErr := s.fetchRelayProject(ctx, profile, projectID)
		if apiErr != nil {
			if apiErr.Code == "project_not_found" {
				continue
			}
			relayErrors = append(relayErrors, map[string]any{"relay_profile_id": profile.ID, "code": apiErr.Code, "message": apiErr.Message})
			continue
		}
		matches = append(matches, relayProjectMatch{Profile: profile, Project: project})
	}
	switch len(matches) {
	case 0:
		if len(relayErrors) > 0 {
			first := relayErrors[0]
			return relayProjectMatch{}, relayUnavailable(RelayProfile{ID: fmt.Sprint(first["relay_profile_id"])}, fmt.Sprint(first["message"]))
		}
		return relayProjectMatch{}, &apiError{Status: http.StatusNotFound, Code: "project_not_found", Message: "project not found"}
	case 1:
		if requireLocationDisambiguation {
			if apiErr := ambiguousLocationBlocker(projectID, matches[0].Profile, matches[0].Project, args); apiErr != nil {
				return relayProjectMatch{}, apiErr
			}
		}
		return matches[0], nil
	default:
		return relayProjectMatch{}, ambiguousRelayProfileBlocker(projectID, matches)
	}
}

func (s *Server) projectLocations(ctx context.Context, principal *authPrincipal, projectID string, args map[string]any) ([]map[string]any, []map[string]any) {
	locations := []map[string]any{}
	relayErrors := []map[string]any{}
	profiles, apiErr := s.relayProfiles(ctx, principal)
	if apiErr != nil {
		relayErrors = append(relayErrors, map[string]any{"code": apiErr.Code, "message": apiErr.Message})
		return locations, relayErrors
	}
	if relayProfileID, _ := args["relay_profile_id"].(string); strings.TrimSpace(relayProfileID) != "" {
		if profile, apiErr := s.profileByID(ctx, principal, relayProfileID); apiErr == nil {
			profiles = []RelayProfile{profile}
		} else {
			relayErrors = append(relayErrors, map[string]any{"relay_profile_id": relayProfileID, "code": apiErr.Code, "message": apiErr.Message})
			return locations, relayErrors
		}
	}
	for _, profile := range profiles {
		if !profile.Enabled {
			continue
		}
		project, apiErr := s.fetchRelayProject(ctx, profile, projectID)
		if apiErr != nil {
			if apiErr.Code != "project_not_found" {
				relayErrors = append(relayErrors, map[string]any{"relay_profile_id": profile.ID, "code": apiErr.Code, "message": apiErr.Message})
			}
			continue
		}
		for _, location := range filterLocations(project.Locations, args) {
			item := map[string]any{
				"relay_profile_id": profile.ID,
				"machine_id":       location.MachineID,
				"host_label":       location.HostLabel,
				"hostname":         location.Hostname,
				"connector_id":     location.ConnectorID,
				"instance_id":      location.InstanceID,
				"repo_label":       location.RepoLabel,
				"repo_root_hash":   location.RepoRootHash,
				"online":           location.Online,
				"status":           location.Status,
			}
			locations = append(locations, item)
		}
	}
	return locations, relayErrors
}

func (s *Server) profileByID(ctx context.Context, principal *authPrincipal, id string) (RelayProfile, *apiError) {
	id = strings.TrimSpace(id)
	profiles, apiErr := s.relayProfiles(ctx, principal)
	if apiErr != nil {
		return RelayProfile{}, apiErr
	}
	for _, profile := range profiles {
		if profile.ID == id {
			if !profile.Enabled {
				return RelayProfile{}, &apiError{Status: http.StatusServiceUnavailable, Code: "relay_profile_disabled", Message: "relay profile is disabled"}
			}
			return profile, nil
		}
	}
	return RelayProfile{}, &apiError{Status: http.StatusNotFound, Code: "relay_profile_not_found", Message: "relay profile not found"}
}

func ambiguousRelayProfileBlocker(projectID string, matches []relayProjectMatch) *apiError {
	profiles := []map[string]any{}
	for _, match := range matches {
		profiles = append(profiles, map[string]any{"relay_profile_id": match.Profile.ID, "name": match.Profile.Name, "status": match.Project.Status})
	}
	sort.Slice(profiles, func(i, j int) bool {
		return fmt.Sprint(profiles[i]["relay_profile_id"]) < fmt.Sprint(profiles[j]["relay_profile_id"])
	})
	msg := fmt.Sprintf("project_id %s is available from multiple relay profiles; pass relay_profile_id", projectID)
	return &apiError{Status: http.StatusConflict, Code: "ambiguous_relay_profile", Message: msg, Blocker: map[string]any{
		"type":                      "ambiguous_relay_profile",
		"planner_decision_required": true,
		"project_id":                projectID,
		"relay_profiles":            profiles,
		"observed_facts":            []string{msg},
	}}
}

func ambiguousLocationBlocker(projectID string, profile RelayProfile, project relayProject, args map[string]any) *apiError {
	if strings.TrimSpace(stringArg(args, "machine_id")) != "" || strings.TrimSpace(stringArg(args, "host_label")) != "" {
		return nil
	}
	online := []projectLocation{}
	for _, location := range project.Locations {
		if location.Online {
			online = append(online, location)
		}
	}
	if len(online) <= 1 {
		return nil
	}
	locations := make([]map[string]any, 0, len(online))
	for _, location := range online {
		locations = append(locations, map[string]any{
			"relay_profile_id": profile.ID,
			"machine_id":       location.MachineID,
			"host_label":       location.HostLabel,
			"status":           location.Status,
		})
	}
	msg := fmt.Sprintf("project_id %s is available on multiple machines through relay profile %s; pass machine_id or host_label", projectID, profile.ID)
	return &apiError{Status: http.StatusConflict, Code: "ambiguous_project_location", Message: msg, Blocker: map[string]any{
		"type":                      "ambiguous_project_location",
		"planner_decision_required": true,
		"project_id":                projectID,
		"relay_profile_id":          profile.ID,
		"locations":                 locations,
		"observed_facts":            []string{msg},
	}}
}

func filterLocations(locations []projectLocation, args map[string]any) []projectLocation {
	machineID := strings.TrimSpace(stringArg(args, "machine_id"))
	hostLabel := strings.ToLower(strings.TrimSpace(stringArg(args, "host_label")))
	if machineID == "" && hostLabel == "" {
		return locations
	}
	out := []projectLocation{}
	for _, location := range locations {
		if machineID != "" && location.MachineID != machineID {
			continue
		}
		if hostLabel != "" && strings.ToLower(location.HostLabel) != hostLabel {
			continue
		}
		out = append(out, location)
	}
	return out
}

func gatewayProjectPayload(profile RelayProfile, project relayProject) map[string]any {
	return map[string]any{
		"project_id":      project.ProjectID,
		"name":            project.Name,
		"default_adapter": project.DefaultAdapter,
		"adapter_profile": project.AdapterProfile,
		"relay_profiles": []projectRelayProfile{{
			RelayProfileID: profile.ID,
			Name:           profile.Name,
			Status:         project.Status,
			DefaultAdapter: project.DefaultAdapter,
			AdapterProfile: project.AdapterProfile,
			Locations:      project.Locations,
		}},
	}
}

func relayStatusMap(profile RelayProfile) map[string]any {
	status := RelayStatus(profile)
	return map[string]any{
		"id":               status.ID,
		"relay_profile_id": status.ID,
		"name":             status.Name,
		"url":              status.URL,
		"enabled":          status.Enabled,
		"token_configured": profile.TokenEnv != "" || profile.TokenFile != "",
	}
}

func forwardToolScopes(name string) []string {
	switch name {
	case "codencer.get_run_report":
		return []string{"reports:read", "runs:read"}
	case "codencer.list_project_runs", "codencer.get_project_run", "codencer.get_project_run_status":
		return []string{"projects:read", "runs:read"}
	case "codencer.submit_project_task":
		return []string{"projects:read", "projects:write", "runs:write"}
	default:
		return []string{"projects:read", "projects:write", "runs:write"}
	}
}

func objectSchema(required []string, properties map[string]any) map[string]any {
	if properties == nil {
		properties = map[string]any{}
	}
	return map[string]any{"type": "object", "required": required, "properties": properties}
}

func withSelectorSchema(schema map[string]any) map[string]any {
	properties, _ := schema["properties"].(map[string]any)
	if properties == nil {
		properties = map[string]any{}
		schema["properties"] = properties
	}
	properties["relay_profile_id"] = stringSchema("Optional Gateway relay profile selector.")
	properties["machine_id"] = stringSchema("Optional machine_id selector for a project location.")
	properties["host_label"] = stringSchema("Optional host_label selector for a project location.")
	return schema
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

func requiredString(args map[string]any, key string) (string, *apiError) {
	value := requiredStringValue(args, key)
	if value == "" {
		return "", &apiError{Status: http.StatusBadRequest, Code: "malformed_request", Message: key + " is required"}
	}
	return value, nil
}

func requiredStringValue(args map[string]any, key string) string {
	return strings.TrimSpace(stringArg(args, key))
}

func stringArg(args map[string]any, key string) string {
	value, _ := args[key].(string)
	return value
}

func intArg(args map[string]any, key string, defaultValue, minValue, maxValue int) int {
	value, ok := args[key]
	if !ok {
		return defaultValue
	}
	var out int
	switch typed := value.(type) {
	case int:
		out = typed
	case int64:
		out = int(typed)
	case float64:
		out = int(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return defaultValue
		}
		out = int(parsed)
	default:
		return defaultValue
	}
	if out < minValue {
		return minValue
	}
	if out > maxValue {
		return maxValue
	}
	return out
}

func boolArg(args map[string]any, key string, defaultValue bool) bool {
	value, ok := args[key]
	if !ok {
		return defaultValue
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		default:
			return defaultValue
		}
	case float64:
		return typed != 0
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return defaultValue
		}
		return parsed != 0
	default:
		return defaultValue
	}
}

func copyOptional(dst, src map[string]any, keys ...string) {
	for _, key := range keys {
		if value, ok := src[key]; ok {
			dst[key] = value
		}
	}
}
