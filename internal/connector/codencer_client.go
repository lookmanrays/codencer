package connector

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/relayproto"
)

const maxArtifactContentBytes = 8 << 20

type CodencerClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewCodencerClient(baseURL string) *CodencerClient {
	return &CodencerClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *CodencerClient) GetInstance(ctx context.Context) (*domain.InstanceInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/instance", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("local daemon unavailable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("local daemon instance lookup failed: %s", string(body))
	}
	var info domain.InstanceInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

func (c *CodencerClient) Proxy(ctx context.Context, request relayproto.CommandRequest) relayproto.CommandResponse {
	response := relayproto.CommandResponse{
		Type:      "response",
		RequestID: request.RequestID,
	}
	if !AllowedLocalProxy(request.Method, request.Path) {
		response.StatusCode = http.StatusForbidden
		response.Error = "connector denied an unsafe proxy request"
		return response
	}

	if strings.HasSuffix(request.Path, "/wait") && request.Method == http.MethodPost {
		return c.waitStep(ctx, request)
	}

	target := c.baseURL + request.Path
	if request.Query != "" {
		target += "?" + request.Query
	}

	var bodyReader io.Reader
	if len(request.Body) > 0 {
		bodyReader = bytes.NewReader(request.Body)
	}
	req, err := http.NewRequestWithContext(ctx, request.Method, target, bodyReader)
	if err != nil {
		response.StatusCode = http.StatusInternalServerError
		response.Error = err.Error()
		return response
	}
	if request.ContentType != "" {
		req.Header.Set("Content-Type", request.ContentType)
	} else if len(request.Body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		response.StatusCode = http.StatusBadGateway
		response.Error = fmt.Sprintf("local daemon unavailable: %v", err)
		return response
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxArtifactContentBytes+1))
	if err != nil {
		response.StatusCode = http.StatusBadGateway
		response.Error = err.Error()
		return response
	}
	if len(body) > maxArtifactContentBytes {
		response.StatusCode = http.StatusRequestEntityTooLarge
		response.Error = "artifact too large for connector transport"
		return response
	}

	response.StatusCode = resp.StatusCode
	response.ContentType = resp.Header.Get("Content-Type")
	response.ContentEncoding, response.Body = encodeBody(response.ContentType, body)
	if resp.StatusCode >= 400 {
		response.Error = string(body)
	}
	return response
}

func (c *CodencerClient) waitStep(ctx context.Context, request relayproto.CommandRequest) relayproto.CommandResponse {
	response := relayproto.CommandResponse{
		Type:            "response",
		RequestID:       request.RequestID,
		ContentType:     "application/json",
		ContentEncoding: "json",
	}
	stepID := path.Base(path.Dir(request.Path))
	var waitReq struct {
		IntervalMS    int  `json:"interval_ms"`
		TimeoutMS     int  `json:"timeout_ms"`
		IncludeResult bool `json:"include_result"`
	}
	if len(request.Body) > 0 {
		if err := json.Unmarshal(request.Body, &waitReq); err != nil {
			response.StatusCode = http.StatusBadRequest
			response.Error = err.Error()
			return response
		}
	}
	if waitReq.IntervalMS <= 0 {
		waitReq.IntervalMS = 1000
	}
	if waitReq.TimeoutMS <= 0 {
		waitReq.TimeoutMS = 300000
	}

	timeout := time.NewTimer(time.Duration(waitReq.TimeoutMS) * time.Millisecond)
	defer timeout.Stop()
	ticker := time.NewTicker(time.Duration(waitReq.IntervalMS) * time.Millisecond)
	defer ticker.Stop()

	buildPayload := func(step *domain.Step, result *domain.ResultSpec, timedOut bool) relayproto.CommandResponse {
		payload := map[string]any{
			"step_id":   stepID,
			"state":     step.State,
			"terminal":  step.State.IsTerminal(),
			"timed_out": timedOut,
			"step":      step,
		}
		if result != nil {
			payload["result"] = result
		}
		data, _ := json.Marshal(payload)
		response.StatusCode = http.StatusOK
		response.Body = data
		return response
	}

	for {
		step, err := c.fetchStep(ctx, stepID)
		if err != nil {
			response.StatusCode = http.StatusBadGateway
			response.Error = err.Error()
			return response
		}
		if step.State.IsTerminal() {
			var result *domain.ResultSpec
			if waitReq.IncludeResult {
				result, _ = c.fetchResult(ctx, stepID)
			}
			return buildPayload(step, result, false)
		}

		select {
		case <-ctx.Done():
			response.StatusCode = http.StatusGatewayTimeout
			response.Error = ctx.Err().Error()
			return response
		case <-timeout.C:
			return buildPayload(step, nil, true)
		case <-ticker.C:
		}
	}
}

func (c *CodencerClient) fetchStep(ctx context.Context, stepID string) (*domain.Step, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/steps/"+stepID, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get step failed: %s", string(body))
	}
	var step domain.Step
	if err := json.NewDecoder(resp.Body).Decode(&step); err != nil {
		return nil, err
	}
	return &step, nil
}

func (c *CodencerClient) fetchResult(ctx context.Context, stepID string) (*domain.ResultSpec, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/steps/"+stepID+"/result", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get result failed: %s", string(body))
	}
	var result domain.ResultSpec
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func encodeBody(contentType string, body []byte) (string, json.RawMessage) {
	if strings.Contains(contentType, "json") {
		return "json", body
	}
	if utf8.Valid(body) {
		encoded, _ := json.Marshal(string(body))
		return "utf-8", encoded
	}
	encoded, _ := json.Marshal(base64.StdEncoding.EncodeToString(body))
	return "base64", encoded
}
