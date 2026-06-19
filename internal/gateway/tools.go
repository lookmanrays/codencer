package gateway

import (
	"context"
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
	ProjectID     string                `json:"project_id"`
	Name          string                `json:"name,omitempty"`
	RelayProfiles []projectRelayProfile `json:"relay_profiles"`
}

type projectRelayProfile struct {
	RelayProfileID string            `json:"relay_profile_id"`
	Name           string            `json:"name,omitempty"`
	Status         string            `json:"status"`
	Locations      []projectLocation `json:"locations,omitempty"`
}

type relayProjectMatch struct {
	Profile RelayProfile
	Project relayProject
}

func buildTools(server *Server) map[string]Tool {
	return map[string]Tool{
		"codencer.list_relays": {
			Name:        "codencer.list_relays",
			Description: "List Gateway relay profiles available to the official Codencer connector.",
			InputSchema: objectSchema(nil, nil),
			ReadOnly:    true,
			Invoke: func(ctx context.Context, args map[string]any) (ToolResult, *apiError) {
				relays := make([]map[string]any, 0, len(server.cfg.RelayProfiles))
				for _, profile := range server.cfg.RelayProfiles {
					payload := relayStatusMap(profile)
					payload["status"] = server.relayAvailability(ctx, profile)
					relays = append(relays, payload)
				}
				return successToolResult("Listed Gateway relay profiles.", map[string]any{"relays": relays}), nil
			},
		},
		"codencer.get_relay": {
			Name:        "codencer.get_relay",
			Description: "Get one Gateway relay profile without exposing backend bearer tokens.",
			InputSchema: objectSchema([]string{"relay_profile_id"}, map[string]any{"relay_profile_id": stringSchema("Gateway relay profile id.")}),
			ReadOnly:    true,
			Invoke: func(ctx context.Context, args map[string]any) (ToolResult, *apiError) {
				profile, apiErr := server.profileByID(requiredStringValue(args, "relay_profile_id"))
				if apiErr != nil {
					return ToolResult{}, apiErr
				}
				payload := relayStatusMap(profile)
				payload["status"] = server.relayAvailability(ctx, profile)
				return successToolResult("Fetched Gateway relay profile.", payload), nil
			},
		},
		"codencer.list_projects": {
			Name:        "codencer.list_projects",
			Description: "Aggregate shared Codencer projects across enabled backend Relays.",
			InputSchema: objectSchema(nil, nil),
			ReadOnly:    true,
			Invoke: func(ctx context.Context, args map[string]any) (ToolResult, *apiError) {
				projects, relayErrors := server.aggregateProjects(ctx)
				return successToolResult("Listed projects through Codencer Gateway.", map[string]any{"projects": projects, "relay_errors": relayErrors}), nil
			},
		},
		"codencer.get_project": {
			Name:        "codencer.get_project",
			Description: "Get a shared project through the Gateway, selecting a relay profile when needed.",
			InputSchema: withSelectorSchema(objectSchema([]string{"project_id"}, map[string]any{"project_id": stringSchema("Project id.")})),
			ReadOnly:    true,
			Invoke: func(ctx context.Context, args map[string]any) (ToolResult, *apiError) {
				projectID, apiErr := requiredString(args, "project_id")
				if apiErr != nil {
					return ToolResult{}, apiErr
				}
				match, apiErr := server.resolveProject(ctx, projectID, args, false)
				if apiErr != nil {
					return ToolResult{}, apiErr
				}
				return successToolResult("Fetched Gateway project.", gatewayProjectPayload(match.Profile, match.Project)), nil
			},
		},
		"codencer.list_project_locations": {
			Name:        "codencer.list_project_locations",
			Description: "List safe machine/location metadata for a project across Gateway relay profiles.",
			InputSchema: withSelectorSchema(objectSchema([]string{"project_id"}, map[string]any{"project_id": stringSchema("Project id.")})),
			ReadOnly:    true,
			Invoke: func(ctx context.Context, args map[string]any) (ToolResult, *apiError) {
				projectID, apiErr := requiredString(args, "project_id")
				if apiErr != nil {
					return ToolResult{}, apiErr
				}
				locations, relayErrors := server.projectLocations(ctx, projectID, args)
				return successToolResult("Listed project locations.", map[string]any{"project_id": projectID, "locations": locations, "relay_errors": relayErrors}), nil
			},
		},
		"codencer.run_project_manifest": server.projectForwardTool("codencer.run_project_manifest", "Run a project manifest through the selected Gateway relay.", []string{"project_id"}, func(args map[string]any) (string, []byte, *apiError) {
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
		}),
		"codencer.submit_project_task_and_wait": server.projectForwardTool("codencer.submit_project_task_and_wait", "Submit one approved task through the selected Gateway relay and wait for evidence.", []string{"project_id"}, func(args map[string]any) (string, []byte, *apiError) {
			projectID, apiErr := requiredString(args, "project_id")
			if apiErr != nil {
				return "", nil, apiErr
			}
			payload := map[string]any{"wait": true}
			copyOptional(payload, args, "run_id", "goal", "prompt", "task", "profile", "adapter_profile", "title", "timeout_seconds")
			body, apiErr := jsonBody(payload)
			if apiErr != nil {
				return "", nil, apiErr
			}
			return "/api/v2/projects/" + projectID + "/submit", body, nil
		}),
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
		"codencer.get_blocker": {
			Name:        "codencer.get_blocker",
			Description: "Read the blocker from a Gateway-routed project run report.",
			InputSchema: withSelectorSchema(objectSchema([]string{"project_id", "run_id"}, map[string]any{
				"project_id": stringSchema("Project id."),
				"run_id":     stringSchema("Run id."),
			})),
			ReadOnly: true,
			Invoke: func(ctx context.Context, args map[string]any) (ToolResult, *apiError) {
				projectID, apiErr := requiredString(args, "project_id")
				if apiErr != nil {
					return ToolResult{}, apiErr
				}
				runID, apiErr := requiredString(args, "run_id")
				if apiErr != nil {
					return ToolResult{}, apiErr
				}
				match, apiErr := server.resolveProject(ctx, projectID, args, false)
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
	if name == "codencer.submit_project_task_and_wait" {
		properties["goal"] = stringSchema("Direct task goal.")
		properties["prompt"] = stringSchema("Prompt text.")
		properties["task"] = objectSchema(nil, nil)
		properties["profile"] = stringSchema("Planner-facing profile id.")
		properties["adapter_profile"] = stringSchema("Daemon adapter profile.")
		properties["title"] = stringSchema("Task title.")
		properties["timeout_seconds"] = intSchema("Timeout in seconds.")
	}
	return Tool{
		Name:        name,
		Description: description,
		InputSchema: withSelectorSchema(objectSchema(required, properties)),
		ReadOnly:    name == "codencer.get_run_report",
		Invoke: func(ctx context.Context, args map[string]any) (ToolResult, *apiError) {
			projectID, apiErr := requiredString(args, "project_id")
			if apiErr != nil {
				return ToolResult{}, apiErr
			}
			match, apiErr := s.resolveProject(ctx, projectID, args, true)
			if apiErr != nil {
				return ToolResult{}, apiErr
			}
			path, body, apiErr := route(args)
			if apiErr != nil {
				return ToolResult{}, apiErr
			}
			path = appendSelector(path, args)
			method := http.MethodGet
			if body != nil {
				method = http.MethodPost
			}
			_, response, apiErr := s.callRelay(ctx, match.Profile, method, path, body)
			if apiErr != nil {
				return ToolResult{}, apiErr
			}
			var payload any
			if len(response) == 0 {
				payload = map[string]any{"ok": true}
			} else if err := json.Unmarshal(response, &payload); err != nil {
				payload = map[string]any{"raw": string(response)}
			}
			if obj, ok := payload.(map[string]any); ok {
				obj["relay_profile_id"] = match.Profile.ID
				payload = obj
			}
			return successToolResult(description, payload), nil
		},
	}
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

func (s *Server) aggregateProjects(ctx context.Context) ([]aggregatedProject, []map[string]any) {
	byID := map[string]*aggregatedProject{}
	order := []string{}
	relayErrors := []map[string]any{}
	for _, profile := range s.cfg.RelayProfiles {
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
				current = &aggregatedProject{ProjectID: project.ProjectID, Name: project.Name}
				byID[project.ProjectID] = current
				order = append(order, project.ProjectID)
			}
			current.RelayProfiles = append(current.RelayProfiles, projectRelayProfile{
				RelayProfileID: profile.ID,
				Name:           profile.Name,
				Status:         project.Status,
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

func (s *Server) resolveProject(ctx context.Context, projectID string, args map[string]any, requireLocationDisambiguation bool) (relayProjectMatch, *apiError) {
	if relayProfileID, _ := args["relay_profile_id"].(string); strings.TrimSpace(relayProfileID) != "" {
		profile, apiErr := s.profileByID(relayProfileID)
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
	for _, profile := range s.cfg.RelayProfiles {
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

func (s *Server) projectLocations(ctx context.Context, projectID string, args map[string]any) ([]map[string]any, []map[string]any) {
	locations := []map[string]any{}
	relayErrors := []map[string]any{}
	profiles := s.cfg.RelayProfiles
	if relayProfileID, _ := args["relay_profile_id"].(string); strings.TrimSpace(relayProfileID) != "" {
		if profile, apiErr := s.profileByID(relayProfileID); apiErr == nil {
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

func (s *Server) profileByID(id string) (RelayProfile, *apiError) {
	id = strings.TrimSpace(id)
	for _, profile := range s.cfg.RelayProfiles {
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
		"project_id": project.ProjectID,
		"name":       project.Name,
		"relay_profiles": []projectRelayProfile{{
			RelayProfileID: profile.ID,
			Name:           profile.Name,
			Status:         project.Status,
			Locations:      project.Locations,
		}},
	}
}

func relayStatusMap(profile RelayProfile) map[string]any {
	status := RelayStatus(profile)
	return map[string]any{
		"id":        status.ID,
		"name":      status.Name,
		"url":       status.URL,
		"token_env": status.TokenEnv,
		"enabled":   status.Enabled,
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

func copyOptional(dst, src map[string]any, keys ...string) {
	for _, key := range keys {
		if value, ok := src[key]; ok {
			dst[key] = value
		}
	}
}
