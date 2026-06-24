package account

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"agent-bridge/internal/security"
)

type Client struct {
	GatewayURL  string
	AccessToken string
	HTTPClient  *http.Client
}

type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	if e.Code != "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return e.Message
}

type DeviceAuthorization struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type TokenResponse struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresIn   int       `json:"expires_in"`
	ExpiresAt   time.Time `json:"expires_at"`
	Scope       string    `json:"scope"`
	Resource    string    `json:"resource"`
	UserID      string    `json:"user_id"`
	WorkspaceID string    `json:"workspace_id"`
}

type WhoamiResponse struct {
	UserID      string   `json:"user_id"`
	WorkspaceID string   `json:"workspace_id"`
	Scopes      []string `json:"scopes"`
	MCPURL      string   `json:"mcp_url"`
}

type RelayProfile struct {
	ID              string `json:"id"`
	RelayProfileID  string `json:"relay_profile_id,omitempty"`
	Type            string `json:"type,omitempty"`
	Name            string `json:"name,omitempty"`
	URL             string `json:"url"`
	Enabled         bool   `json:"enabled"`
	Status          string `json:"status,omitempty"`
	TokenConfigured bool   `json:"token_configured"`
}

type RelayProfileInput struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	URL       string `json:"url"`
	TokenEnv  string `json:"token_env,omitempty"`
	TokenFile string `json:"token_file,omitempty"`
	Type      string `json:"type,omitempty"`
	Enabled   *bool  `json:"enabled,omitempty"`
}

type MachineInput struct {
	MachineID string `json:"machine_id"`
	Hostname  string `json:"hostname,omitempty"`
	HostLabel string `json:"host_label,omitempty"`
	OS        string `json:"os,omitempty"`
	Arch      string `json:"arch,omitempty"`
}

type ConnectorLoginRequest struct {
	RelayProfileID string       `json:"relay_profile_id,omitempty"`
	Relay          string       `json:"relay,omitempty"`
	Machine        MachineInput `json:"machine"`
	Label          string       `json:"label,omitempty"`
}

type ConnectorLoginResponse struct {
	BindingID        string       `json:"binding_id"`
	WorkspaceID      string       `json:"workspace_id"`
	Machine          MachineInput `json:"machine"`
	RelayProfile     RelayProfile `json:"relay_profile"`
	RelayURL         string       `json:"relay_url"`
	EnrollmentToken  string       `json:"enrollment_token"`
	ExpiresInSeconds int          `json:"expires_in_seconds"`
}

type ConnectorCompleteRequest struct {
	BindingID        string `json:"binding_id"`
	RelayConnectorID string `json:"relay_connector_id"`
	RelayMachineID   string `json:"relay_machine_id"`
	PublicKey        string `json:"public_key,omitempty"`
}

type ConnectorCompleteResponse struct {
	OK bool `json:"ok"`
}

type SyncProjectRecord struct {
	ID             string `json:"id"`
	Name           string `json:"name,omitempty"`
	DefaultAdapter string `json:"default_adapter,omitempty"`
	Profile        string `json:"profile,omitempty"`
	SharedToRelay  bool   `json:"shared_to_relay"`
	MachineID      string `json:"machine_id,omitempty"`
	HostLabel      string `json:"host_label,omitempty"`
}

type SyncRunRecord struct {
	RunID            string   `json:"run_id"`
	ProjectID        string   `json:"project_id"`
	Status           string   `json:"status,omitempty"`
	Title            string   `json:"title,omitempty"`
	Summary          string   `json:"summary,omitempty"`
	ExecutorProfile  string   `json:"executor_profile,omitempty"`
	Mode             string   `json:"mode,omitempty"`
	Scope            string   `json:"scope"`
	ReportStatus     string   `json:"report_status"`
	ExecutionMode    string   `json:"execution_mode,omitempty"`
	SafeArtifactRefs []string `json:"safe_artifact_refs,omitempty"`
}

type SyncRunsRequest struct {
	Mode     string              `json:"mode"`
	Scope    string              `json:"scope"`
	Projects []SyncProjectRecord `json:"projects"`
	Runs     []SyncRunRecord     `json:"runs"`
}

type SyncRunsResponse struct {
	OK            bool     `json:"ok"`
	Mode          string   `json:"mode"`
	Scope         string   `json:"scope"`
	SyncedRuns    int      `json:"synced_runs"`
	RunHistoryIDs []string `json:"run_history_ids"`
}

func NewClient(gatewayURL, accessToken string) *Client {
	return &Client{
		GatewayURL:  NormalizeGatewayURL(gatewayURL),
		AccessToken: strings.TrimSpace(accessToken),
		HTTPClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) DeviceAuthorize(ctx context.Context, email, displayName string) (DeviceAuthorization, error) {
	var out DeviceAuthorization
	err := c.do(ctx, http.MethodPost, "/api/gateway/v1/device/authorize", map[string]any{
		"email":        strings.TrimSpace(email),
		"display_name": strings.TrimSpace(displayName),
	}, &out)
	return out, err
}

func (c *Client) DeviceApprove(ctx context.Context, userCode string) error {
	var out map[string]any
	return c.do(ctx, http.MethodPost, "/api/gateway/v1/device/approve", map[string]any{"user_code": strings.TrimSpace(userCode)}, &out)
}

func (c *Client) DeviceToken(ctx context.Context, deviceCode string) (TokenResponse, error) {
	var out TokenResponse
	err := c.do(ctx, http.MethodPost, "/api/gateway/v1/device/token", map[string]any{"device_code": strings.TrimSpace(deviceCode)}, &out)
	return out, err
}

func (c *Client) Whoami(ctx context.Context) (WhoamiResponse, error) {
	var out WhoamiResponse
	err := c.do(ctx, http.MethodGet, "/api/gateway/v1/whoami", nil, &out)
	return out, err
}

func (c *Client) Logout(ctx context.Context) error {
	var out map[string]any
	return c.do(ctx, http.MethodPost, "/api/gateway/v1/logout", map[string]any{}, &out)
}

func (c *Client) ListRelays(ctx context.Context) ([]RelayProfile, error) {
	var out struct {
		Relays []RelayProfile `json:"relays"`
	}
	err := c.do(ctx, http.MethodGet, "/api/gateway/v1/relays", nil, &out)
	return out.Relays, err
}

func (c *Client) AddRelay(ctx context.Context, input RelayProfileInput) (RelayProfile, error) {
	var out struct {
		Relay RelayProfile `json:"relay"`
	}
	err := c.do(ctx, http.MethodPost, "/api/gateway/v1/relays", input, &out)
	return out.Relay, err
}

func (c *Client) GetRelay(ctx context.Context, id string) (RelayProfile, error) {
	var out struct {
		Relay RelayProfile `json:"relay"`
	}
	err := c.do(ctx, http.MethodGet, "/api/gateway/v1/relays/"+strings.Trim(strings.TrimSpace(id), "/"), nil, &out)
	return out.Relay, err
}

func (c *Client) RemoveRelay(ctx context.Context, id string) error {
	var out map[string]any
	return c.do(ctx, http.MethodDelete, "/api/gateway/v1/relays/"+strings.Trim(strings.TrimSpace(id), "/"), nil, &out)
}

func (c *Client) ConnectorLogin(ctx context.Context, input ConnectorLoginRequest) (ConnectorLoginResponse, error) {
	var out ConnectorLoginResponse
	err := c.do(ctx, http.MethodPost, "/api/gateway/v1/connectors/login", input, &out)
	return out, err
}

func (c *Client) ConnectorComplete(ctx context.Context, input ConnectorCompleteRequest) error {
	var out ConnectorCompleteResponse
	return c.do(ctx, http.MethodPost, "/api/gateway/v1/connectors/complete", input, &out)
}

func (c *Client) SyncRuns(ctx context.Context, input SyncRunsRequest) (SyncRunsResponse, error) {
	var out SyncRunsResponse
	err := c.do(ctx, http.MethodPost, "/api/gateway/v1/sync/runs", input, &out)
	return out, err
}

func (c *Client) do(ctx context.Context, method, path string, payload any, out any) error {
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.GatewayURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		return decodeAPIError(resp.StatusCode, data)
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode Gateway response: %w", err)
	}
	return nil
}

func decodeAPIError(status int, data []byte) error {
	var decoded struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &decoded); err == nil && (decoded.Error.Code != "" || decoded.Error.Message != "") {
		return &APIError{Status: status, Code: decoded.Error.Code, Message: security.Redact(decoded.Error.Message)}
	}
	return &APIError{Status: status, Code: "gateway_request_failed", Message: security.Redact(string(data))}
}
