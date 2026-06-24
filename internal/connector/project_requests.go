package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/local"
	"agent-bridge/internal/localexec"
	manifestpkg "agent-bridge/internal/manifest"
	projectpkg "agent-bridge/internal/project"
	"agent-bridge/internal/relayproto"
)

func (c *Client) handleProjectRequest(ctx context.Context, cfg *Config, request relayproto.CommandRequest) relayproto.CommandResponse {
	response := relayproto.CommandResponse{
		Type:            "response",
		RequestID:       request.RequestID,
		ContentType:     "application/json",
		ContentEncoding: "json",
	}
	projectID, tail, err := parseProjectCommandPath(request.Path)
	if err != nil {
		return projectCommandError(response, http.StatusBadRequest, err)
	}
	project, err := c.registryForConfig(cfg).ConfiguredSharedProject(projectID)
	if err != nil {
		if c.status != nil {
			_ = c.status.MarkFailure(cfg, err.Error(), nowUTC())
		}
		return projectCommandError(response, http.StatusForbidden, err)
	}

	switch {
	case len(tail) == 1 && tail[0] == "runs" && request.Method == http.MethodPost:
		service := localexec.NewService()
		report, err := service.StartRun(ctx, localexec.RunOptions{BaseOptions: projectBaseOptions(cfg, project)})
		return projectExecutionResponse(response, report, err)
	case len(tail) == 1 && tail[0] == "runs" && request.Method == http.MethodGet:
		service := localexec.NewService()
		report, err := service.ListRuns(ctx, localexec.RunOptions{BaseOptions: projectBaseOptions(cfg, project)})
		return projectExecutionResponse(response, report, err)
	case len(tail) == 2 && tail[0] == "runs" && request.Method == http.MethodGet:
		service := localexec.NewService()
		report, err := service.GetRun(ctx, localexec.RunOptions{BaseOptions: projectBaseOptions(cfg, project), RunID: tail[1]})
		return projectExecutionResponse(response, report, err)
	case len(tail) == 3 && tail[0] == "runs" && tail[2] == "cancel" && request.Method == http.MethodPost:
		service := localexec.NewService()
		report, err := service.CancelRun(ctx, localexec.RunOptions{BaseOptions: projectBaseOptions(cfg, project), RunID: tail[1]})
		return projectExecutionResponse(response, report, err)
	case len(tail) == 1 && tail[0] == "submit" && request.Method == http.MethodPost:
		opts, err := decodeProjectSubmit(projectBaseOptions(cfg, project), request.Body)
		if err != nil {
			return projectCommandError(response, http.StatusBadRequest, err)
		}
		service := localexec.NewService()
		report, err := service.Submit(ctx, opts)
		return projectExecutionResponse(response, report, err)
	case len(tail) == 1 && tail[0] == "run-plan" && request.Method == http.MethodPost:
		opts, err := decodeProjectRunPlan(projectBaseOptions(cfg, project), request.Body)
		if err != nil {
			return projectCommandError(response, http.StatusBadRequest, err)
		}
		service := localexec.NewService()
		report, err := service.RunPlan(ctx, opts)
		return projectRunPlanResponse(response, report, err)
	case len(tail) == 3 && tail[0] == "reports" && tail[1] == "run-plans" && request.Method == http.MethodGet:
		service := localexec.NewService()
		report, err := service.GetRunPlanReport(ctx, localexec.RunPlanReportOptions{BaseOptions: projectBaseOptions(cfg, project), RunID: tail[2]})
		return projectRunPlanResponse(response, report, err)
	case len(tail) == 3 && tail[0] == "steps" && request.Method == http.MethodGet:
		return c.proxyProjectEvidence(ctx, cfg, project, request, tail[1], tail[2])
	default:
		return projectCommandError(response, http.StatusNotFound, fmt.Errorf("unsupported project command route"))
	}
}

func parseProjectCommandPath(path string) (string, []string, error) {
	trimmed := strings.Trim(strings.TrimPrefix(path, "/codencer/v1/projects/"), "/")
	parts := splitProjectPath(trimmed)
	if len(parts) == 0 || parts[0] == "" {
		return "", nil, fmt.Errorf("project id is required")
	}
	return parts[0], parts[1:], nil
}

func splitProjectPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	parts := strings.Split(path, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			out = append(out, part)
		}
	}
	return out
}

func projectBaseOptions(cfg *Config, project projectpkg.Project) localexec.BaseOptions {
	home := ""
	if cfg != nil {
		home = cfg.CodencerHome
	}
	return localexec.BaseOptions{
		ProjectID:    project.ID,
		RepoRoot:     project.RepoRoot,
		CodencerHome: home,
	}
}

type projectSubmitRequest struct {
	RunID          string          `json:"run_id"`
	Goal           string          `json:"goal"`
	Prompt         string          `json:"prompt"`
	Task           json.RawMessage `json:"task"`
	Wait           bool            `json:"wait"`
	Profile        string          `json:"profile"`
	AdapterProfile string          `json:"adapter_profile"`
	Title          string          `json:"title"`
	TimeoutSeconds int             `json:"timeout_seconds"`
}

func decodeProjectSubmit(base localexec.BaseOptions, body json.RawMessage) (localexec.SubmitOptions, error) {
	var req projectSubmitRequest
	if len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			return localexec.SubmitOptions{}, err
		}
	}
	opts := localexec.SubmitOptions{
		BaseOptions:    base,
		RunID:          req.RunID,
		Goal:           req.Goal,
		Wait:           req.Wait,
		Profile:        req.Profile,
		AdapterProfile: req.AdapterProfile,
		Title:          req.Title,
		TimeoutSeconds: req.TimeoutSeconds,
	}
	if len(req.Task) > 0 && string(req.Task) != "null" {
		opts.SourceKind = domain.SubmissionSourceTaskJSON
		opts.SourceName = "relay-task.json"
		opts.Content = req.Task
		opts.Goal = ""
		return opts, nil
	}
	if strings.TrimSpace(req.Prompt) != "" {
		opts.SourceKind = domain.SubmissionSourceStdin
		opts.SourceName = "relay-prompt"
		opts.Content = []byte(req.Prompt)
		opts.Goal = ""
	}
	return opts, nil
}

type projectRunPlanRequest struct {
	Manifest     json.RawMessage `json:"manifest"`
	ManifestText string          `json:"manifest_text"`
	ManifestName string          `json:"manifest_name"`
	Wait         bool            `json:"wait"`
}

func decodeProjectRunPlan(base localexec.BaseOptions, body json.RawMessage) (localexec.RunPlanOptions, error) {
	var req projectRunPlanRequest
	if len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			return localexec.RunPlanOptions{}, err
		}
	}
	var data []byte
	switch {
	case len(req.Manifest) > 0 && string(req.Manifest) != "null":
		data = req.Manifest
	case strings.TrimSpace(req.ManifestText) != "":
		data = []byte(req.ManifestText)
	default:
		return localexec.RunPlanOptions{}, fmt.Errorf("manifest or manifest_text is required")
	}
	manifest, err := manifestpkg.Parse(data)
	if err != nil {
		return localexec.RunPlanOptions{}, err
	}
	for _, task := range manifest.Tasks {
		if strings.TrimSpace(task.PromptFile) != "" {
			return localexec.RunPlanOptions{}, fmt.Errorf("remote run-plan does not allow prompt_file for task %q", task.ID)
		}
	}
	return localexec.RunPlanOptions{
		BaseOptions:  base,
		Manifest:     manifest,
		ManifestName: firstNonEmpty(req.ManifestName, "relay-manifest"),
		Wait:         req.Wait,
	}, nil
}

func (c *Client) proxyProjectEvidence(ctx context.Context, cfg *Config, project projectpkg.Project, request relayproto.CommandRequest, stepID, resource string) relayproto.CommandResponse {
	daemonURL, err := projectDaemonURL(cfg, project)
	if err != nil {
		return projectCommandError(relayproto.CommandResponse{
			Type:            "response",
			RequestID:       request.RequestID,
			ContentType:     "application/json",
			ContentEncoding: "json",
		}, http.StatusBadRequest, err)
	}
	targetPath := "/api/v1/steps/" + stepID
	switch resource {
	case "result", "artifacts", "logs", "validations":
		targetPath += "/" + resource
	default:
		return projectCommandError(relayproto.CommandResponse{
			Type:            "response",
			RequestID:       request.RequestID,
			ContentType:     "application/json",
			ContentEncoding: "json",
		}, http.StatusNotFound, fmt.Errorf("unsupported project evidence route"))
	}
	proxyReq := request
	proxyReq.Path = targetPath
	return NewCodencerClient(daemonURL).WithStatusStore(c.status, cfg).Proxy(ctx, proxyReq)
}

func projectDaemonURL(cfg *Config, project projectpkg.Project) (string, error) {
	if strings.TrimSpace(project.DaemonURL) != "" {
		return strings.TrimRight(strings.TrimSpace(project.DaemonURL), "/"), nil
	}
	home := ""
	if cfg != nil {
		home = cfg.CodencerHome
	}
	paths, err := local.ResolvePathsForHome("", "", home)
	if err != nil {
		return "", err
	}
	localCfg, err := local.LoadConfig(paths.ConfigFile)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(localCfg.DefaultDaemonURL) == "" {
		return "", fmt.Errorf("daemon URL is not configured")
	}
	return strings.TrimRight(strings.TrimSpace(localCfg.DefaultDaemonURL), "/"), nil
}

func projectExecutionResponse(response relayproto.CommandResponse, report localexec.ExecutionReport, err error) relayproto.CommandResponse {
	if err != nil {
		return projectLocalexecError(response, err)
	}
	return projectJSONResponse(response, http.StatusOK, report)
}

func projectRunPlanResponse(response relayproto.CommandResponse, report localexec.RunPlanReport, err error) relayproto.CommandResponse {
	if err != nil {
		return projectLocalexecError(response, err)
	}
	return projectJSONResponse(response, http.StatusOK, report)
}

func projectLocalexecError(response relayproto.CommandResponse, err error) relayproto.CommandResponse {
	return projectJSONResponse(response, http.StatusBadRequest, localexec.ErrorReportFor(err))
}

func projectCommandError(response relayproto.CommandResponse, status int, err error) relayproto.CommandResponse {
	return projectJSONResponse(response, status, map[string]any{
		"ok":     false,
		"status": "error",
		"error":  err.Error(),
	})
}

func projectJSONResponse(response relayproto.CommandResponse, status int, payload any) relayproto.CommandResponse {
	body, err := json.Marshal(payload)
	if err != nil {
		response.StatusCode = http.StatusInternalServerError
		response.Error = err.Error()
		response.Body = []byte(`{"ok":false,"status":"error","error":"encode response failed"}`)
		return response
	}
	response.StatusCode = status
	response.Body = body
	if status >= 400 {
		response.Error = string(body)
	}
	return response
}

func nowUTC() time.Time {
	return time.Now().UTC()
}
