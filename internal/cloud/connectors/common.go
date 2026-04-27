package connectors

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const userAgent = "codencer-cloud-connectors/1"

func defaultHTTPClient(client *http.Client) *http.Client {
	if client != nil {
		return client
	}
	return http.DefaultClient
}

func resolveAPIBase(defaultBaseURL, configured string) (string, error) {
	baseURL := strings.TrimSpace(configured)
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if _, err := url.Parse(baseURL); err != nil {
		return "", err
	}
	return baseURL, nil
}

func apiURL(baseURL string, segments ...string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", err
	}
	base := parsed.Scheme + "://"
	if parsed.User != nil {
		base += parsed.User.String() + "@"
	}
	base += parsed.Host
	basePath := strings.TrimRight(parsed.EscapedPath(), "/")
	encoded := make([]string, 0, len(segments))
	for _, segment := range segments {
		if strings.TrimSpace(segment) == "" {
			continue
		}
		encoded = append(encoded, url.PathEscape(segment))
	}
	joined := strings.Join(encoded, "/")
	endpoint := base + basePath
	if joined != "" {
		if !strings.HasSuffix(endpoint, "/") {
			endpoint += "/"
		}
		endpoint += joined
	}
	if parsed.RawQuery != "" {
		endpoint += "?" + parsed.RawQuery
	}
	return endpoint, nil
}

func newJSONRequest(ctx context.Context, method, endpoint string, body any) (*http.Request, error) {
	var payload io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		payload = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, payload)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	return req, nil
}

func readJSONResponse(resp *http.Response, out any) error {
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		text := strings.TrimSpace(string(body))
		if text == "" {
			text = resp.Status
		}
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, text)
	}
	if out == nil {
		io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func readBodyLimited(body io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, 1<<20))
	if err != nil {
		return nil, err
	}
	return data, nil
}

func verifyHMACSignature(secret, signature string, body []byte) bool {
	if secret == "" || signature == "" {
		return false
	}
	const prefix = "sha256="
	if !strings.HasPrefix(signature, prefix) {
		return false
	}
	sum := hmac.New(sha256.New, []byte(secret))
	_, _ = sum.Write(body)
	expected := sum.Sum(nil)
	got, err := hex.DecodeString(strings.TrimPrefix(signature, prefix))
	if err != nil {
		return false
	}
	return hmac.Equal(expected, got)
}

func statusFromResults(provider Provider, validation ValidationResult, webhook WebhookVerification) ConnectorStatus {
	status := ConnectorStatus{
		Provider:  provider,
		CheckedAt: time.Now().UTC(),
		Details:   map[string]string{},
	}
	if validation.Message != "" {
		status.Details["validation_message"] = validation.Message
	}
	if validation.Identity != "" {
		status.Details["identity"] = validation.Identity
	}
	if webhook.EventType != "" {
		status.Details["last_event_type"] = webhook.EventType
	}
	if webhook.Message != "" {
		status.Details["webhook_message"] = webhook.Message
	}
	switch {
	case !validation.OK:
		status.State = "invalid"
		status.Message = validation.Message
		status.Ready = false
	case webhook.Verified:
		status.State = "healthy"
		status.Message = "installation validated and webhook verified"
		status.Ready = true
	case webhook.Message != "":
		status.State = "degraded"
		status.Message = webhook.Message
		status.Ready = true
	default:
		status.State = "healthy"
		status.Message = "installation validated"
		status.Ready = true
	}
	if len(status.Details) == 0 {
		status.Details = nil
	}
	return status
}

func validationOnlyStatus(provider Provider, validation ValidationResult, message string) ConnectorStatus {
	status := ConnectorStatus{
		Provider:  provider,
		CheckedAt: time.Now().UTC(),
		Details:   map[string]string{},
	}
	if validation.Identity != "" {
		status.Details["identity"] = validation.Identity
	}
	if validation.Message != "" {
		status.Details["validation_message"] = validation.Message
	}
	switch {
	case !validation.OK:
		status.State = "invalid"
		status.Ready = false
		status.Message = validation.Message
	default:
		status.State = "healthy"
		status.Ready = true
		status.Message = message
	}
	if len(status.Details) == 0 {
		status.Details = nil
	}
	return status
}

func nonEmpty(value, name string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New(name + " is required")
	}
	return nil
}
