package antigravity

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"agent-bridge/internal/domain"
)

var (
	ErrInstanceUnreachable = errors.New("antigravity instance unreachable")
	ErrAuthFailed          = errors.New("antigravity authentication failed (invalid CSRF or forbidden)")
)

// Client handles communication with the Antigravity Language Server RPC.
type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

// Call executes a Connect RPC method on the specified instance.
func (c *Client) Call(ctx context.Context, inst *domain.AGInstance, method string, req interface{}, resp interface{}) error {
	url := fmt.Sprintf("https://127.0.0.1:%d/%s/%s", inst.HTTPSPort, servicePrefix, method)
	
	payload, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("Antigravity RPC: failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("Antigravity RPC: failed to create http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-codeium-csrf-token", inst.CSRFToken)

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInstanceUnreachable, err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == http.StatusUnauthorized || httpResp.StatusCode == http.StatusForbidden {
		return ErrAuthFailed
	}

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return fmt.Errorf("rpc error (status %d): %s", httpResp.StatusCode, string(body))
	}

	if err := json.Unmarshal(body, resp); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w (body: %s)", err, string(body))
	}

	return nil
}
