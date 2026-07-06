package localexec

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"agent-bridge/internal/domain"
)

type DaemonError struct {
	Kind       string
	Message    string
	StatusCode int
}

func (e *DaemonError) Error() string {
	if e == nil {
		return ""
	}
	if e.StatusCode != 0 {
		return fmt.Sprintf("%s: %s", e.Kind, e.Message)
	}
	return e.Message
}

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewClient(baseURL string, client *http.Client) *Client {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{BaseURL: baseURL, HTTPClient: client}
}

func (c *Client) StartRun(ctx context.Context, projectID string) (*domain.Run, error) {
	var run domain.Run
	body := map[string]string{
		"project_id": projectID,
		"planner_id": "codencer-cli",
	}
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/runs", nil, body, &run); err != nil {
		return nil, err
	}
	return &run, nil
}

func (c *Client) ListRuns(ctx context.Context, projectID string) ([]*domain.Run, error) {
	query := url.Values{}
	if strings.TrimSpace(projectID) != "" {
		query.Set("project_id", projectID)
	}
	var runs []*domain.Run
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/runs", query, nil, &runs); err != nil {
		return nil, err
	}
	return runs, nil
}

func (c *Client) GetRun(ctx context.Context, runID string) (*domain.Run, error) {
	var run domain.Run
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/runs/"+url.PathEscape(runID), nil, nil, &run); err != nil {
		return nil, err
	}
	return &run, nil
}

func (c *Client) GetRunSteps(ctx context.Context, runID string) ([]*domain.Step, error) {
	var steps []*domain.Step
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/runs/"+url.PathEscape(runID)+"/steps", nil, nil, &steps); err != nil {
		return nil, err
	}
	return steps, nil
}

func (c *Client) AbortRun(ctx context.Context, runID string) error {
	return c.doJSON(ctx, http.MethodPatch, "/api/v1/runs/"+url.PathEscape(runID), nil, map[string]string{"action": "abort"}, nil)
}

func (c *Client) ResumeRun(ctx context.Context, runID string) (*domain.Run, error) {
	var run domain.Run
	if err := c.doJSON(ctx, http.MethodPatch, "/api/v1/runs/"+url.PathEscape(runID), nil, map[string]string{"action": "resume"}, &run); err != nil {
		return nil, err
	}
	return &run, nil
}

func (c *Client) SubmitTask(ctx context.Context, runID string, task *domain.TaskSpec) (*domain.Step, error) {
	var step domain.Step
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/runs/"+url.PathEscape(runID)+"/steps", nil, task, &step); err != nil {
		return nil, err
	}
	return &step, nil
}

func (c *Client) GetStep(ctx context.Context, stepID string) (*domain.Step, error) {
	var step domain.Step
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/steps/"+url.PathEscape(stepID), nil, nil, &step); err != nil {
		return nil, err
	}
	return &step, nil
}

func (c *Client) GetResult(ctx context.Context, stepID string) (*domain.ResultSpec, error) {
	var result domain.ResultSpec
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/steps/"+url.PathEscape(stepID)+"/result", nil, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetArtifacts(ctx context.Context, stepID string) ([]*domain.Artifact, error) {
	var artifacts []*domain.Artifact
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/steps/"+url.PathEscape(stepID)+"/artifacts", nil, nil, &artifacts); err != nil {
		return nil, err
	}
	return artifacts, nil
}

func (c *Client) GetValidations(ctx context.Context, stepID string) (map[string][]*domain.ValidationResult, error) {
	var validations map[string][]*domain.ValidationResult
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/steps/"+url.PathEscape(stepID)+"/validations", nil, nil, &validations); err != nil {
		return nil, err
	}
	return validations, nil
}

func (c *Client) GetLogs(ctx context.Context, stepID string) (string, error) {
	body, err := c.do(ctx, http.MethodGet, "/api/v1/steps/"+url.PathEscape(stepID)+"/logs", nil, nil)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, query url.Values, input any, output any) error {
	body, err := c.do(ctx, method, path, query, input)
	if err != nil {
		return err
	}
	if output == nil || len(body) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, output); err != nil {
		return &DaemonError{Kind: BlockerBridgeError, Message: fmt.Sprintf("decode daemon response: %v", err)}
	}
	return nil
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, input any) ([]byte, error) {
	if c.BaseURL == "" {
		return nil, &DaemonError{Kind: BlockerDaemonNotRunning, Message: "daemon URL is not configured"}
	}
	target := c.BaseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			return nil, &DaemonError{Kind: BlockerInvalidInput, Message: fmt.Sprintf("encode request: %v", err)}
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, &DaemonError{Kind: BlockerInvalidInput, Message: err.Error()}
	}
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, &DaemonError{Kind: BlockerDaemonNotRunning, Message: err.Error()}
	}
	defer resp.Body.Close()

	data, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, &DaemonError{Kind: BlockerBridgeError, Message: fmt.Sprintf("read daemon response: %v", readErr), StatusCode: resp.StatusCode}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(data))
		if message == "" {
			message = resp.Status
		}
		return nil, &DaemonError{Kind: daemonErrorKind(resp.StatusCode), Message: message, StatusCode: resp.StatusCode}
	}
	return data, nil
}

func daemonErrorKind(status int) string {
	switch {
	case status == http.StatusBadRequest || status == http.StatusConflict:
		return BlockerInvalidInput
	case status == http.StatusNotFound:
		return BlockerBridgeError
	case status >= 500:
		return BlockerBridgeError
	default:
		return BlockerBridgeError
	}
}
