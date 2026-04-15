package connectors

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// LinearClient is a minimal Linear GraphQL client for validation, event ingestion,
// and a small write surface.
type LinearClient struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// Name returns the provider name used by the cloud connector registry.
func (c *LinearClient) Name() string { return "linear" }

// SupportsWebhook reports that Linear webhook verification is implemented.
func (c *LinearClient) SupportsWebhook() bool { return true }

// LinearIssue is the normalized write result for issue creation.
type LinearIssue struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url,omitempty"`
}

// LinearComment captures the normalized write result for comment creation.
type LinearComment struct {
	ID   string `json:"id"`
	Body string `json:"body,omitempty"`
	URL  string `json:"url,omitempty"`
}

// LinearNormalizedEvent is a provider-specific normalized event for issues/comments.
type LinearNormalizedEvent struct {
	Provider         string         `json:"provider"`
	Kind             string         `json:"kind"`
	Action           string         `json:"action"`
	EntityType       string         `json:"entity_type"`
	EntityID         string         `json:"entity_id,omitempty"`
	Title            string         `json:"title,omitempty"`
	Comment          string         `json:"comment,omitempty"`
	URL              string         `json:"url,omitempty"`
	Actor            string         `json:"actor,omitempty"`
	WebhookID        string         `json:"webhook_id,omitempty"`
	WebhookTimestamp time.Time      `json:"webhook_timestamp,omitempty"`
	UpdatedFrom      map[string]any `json:"updated_from,omitempty"`
	Raw              map[string]any `json:"raw,omitempty"`
}

// Validate checks the token by querying viewer through GraphQL.
func (c *LinearClient) Validate(ctx context.Context) error {
	if c == nil {
		return fmt.Errorf("linear client is nil")
	}
	if strings.TrimSpace(c.BaseURL) == "" {
		return fmt.Errorf("linear base url is required")
	}
	if strings.TrimSpace(c.Token) == "" {
		return fmt.Errorf("linear token is required")
	}
	res, err := c.graphQL(ctx, map[string]any{
		"query": `query Me { viewer { id name email } }`,
	})
	if err != nil {
		return err
	}
	viewer, ok := nestedMap(res, "data", "viewer")
	if !ok {
		return fmt.Errorf("linear viewer response missing viewer")
	}
	if strings.TrimSpace(stringFromMap(viewer, "id")) == "" && strings.TrimSpace(stringFromMap(viewer, "name")) == "" {
		return fmt.Errorf("linear viewer response missing identity fields")
	}
	return nil
}

// CreateIssue creates a real Linear issue using GraphQL.
func (c *LinearClient) CreateIssue(ctx context.Context, teamID, title, description string) (*LinearIssue, error) {
	if c == nil {
		return nil, fmt.Errorf("linear client is nil")
	}
	teamID = strings.TrimSpace(teamID)
	title = strings.TrimSpace(title)
	if teamID == "" {
		return nil, fmt.Errorf("team id is required")
	}
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}

	res, err := c.graphQL(ctx, map[string]any{
		"query": `mutation IssueCreate($input: IssueCreateInput!) {
			issueCreate(input: $input) {
				success
				issue { id title url }
			}
		}`,
		"variables": map[string]any{
			"input": map[string]any{
				"teamId":      teamID,
				"title":       title,
				"description": description,
			},
		},
	})
	if err != nil {
		return nil, err
	}

	issueMap, ok := nestedMap(res, "data", "issueCreate", "issue")
	if !ok {
		return nil, fmt.Errorf("linear issueCreate response missing issue")
	}
	return &LinearIssue{
		ID:    stringFromMap(issueMap, "id"),
		Title: stringFromMap(issueMap, "title"),
		URL:   stringFromMap(issueMap, "url"),
	}, nil
}

// AddComment posts a real comment to an existing Linear issue.
func (c *LinearClient) AddComment(ctx context.Context, issueID, body string) (*LinearComment, error) {
	if c == nil {
		return nil, fmt.Errorf("linear client is nil")
	}
	issueID = strings.TrimSpace(issueID)
	body = strings.TrimSpace(body)
	if issueID == "" {
		return nil, fmt.Errorf("issue id is required")
	}
	if body == "" {
		return nil, fmt.Errorf("comment body is required")
	}

	res, err := c.graphQL(ctx, map[string]any{
		"query": `mutation CommentCreate($input: CommentCreateInput!) {
			commentCreate(input: $input) {
				success
				comment { id body url }
			}
		}`,
		"variables": map[string]any{
			"input": map[string]any{
				"issueId": issueID,
				"body":    body,
			},
		},
	})
	if err != nil {
		return nil, err
	}

	commentMap, ok := nestedMap(res, "data", "commentCreate", "comment")
	if !ok {
		return nil, fmt.Errorf("linear commentCreate response missing comment")
	}
	return &LinearComment{
		ID:   stringFromMap(commentMap, "id"),
		Body: stringFromMap(commentMap, "body"),
		URL:  stringFromMap(commentMap, "url"),
	}, nil
}

// NormalizeLinearWebhookRequest verifies the request signature and normalizes issue/comment events.
func NormalizeLinearWebhookRequest(req *http.Request, signingSecret string, now time.Time) (*LinearNormalizedEvent, error) {
	if req == nil {
		return nil, fmt.Errorf("linear request is nil")
	}
	raw, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("read linear webhook body: %w", err)
	}
	req.Body = io.NopCloser(bytes.NewReader(raw))
	if err := verifyLinearSignature(req.Header, raw, signingSecret, now); err != nil {
		return nil, err
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode linear webhook body: %w", err)
	}

	event := &LinearNormalizedEvent{
		Provider: "linear",
		Raw:      payload,
	}
	event.Action = stringFromMap(payload, "action")
	event.EntityType = stringFromMap(payload, "type")
	event.Kind = strings.ToLower(strings.TrimSpace(event.EntityType))
	event.EntityID = stringFromMap(nestedMapValue(payload, "data"), "id")
	event.Title = stringFromMap(nestedMapValue(payload, "data"), "title")
	event.URL = stringFromMap(payload, "url")
	event.Actor = stringFromMap(nestedMapValue(payload, "actor"), "name")
	event.WebhookID = stringFromMap(payload, "webhookId")
	if ts := int64FromMap(payload, "webhookTimestamp"); ts > 0 {
		event.WebhookTimestamp = time.UnixMilli(ts)
	}
	if updatedFrom, ok := payload["updatedFrom"].(map[string]any); ok {
		event.UpdatedFrom = updatedFrom
	}
	if comment := stringFromMap(nestedMapValue(payload, "data"), "body"); comment != "" {
		event.Comment = comment
	}
	if event.Kind == "" {
		event.Kind = "event"
	}
	if event.Action == "" {
		event.Action = "update"
	}
	return event, nil
}

func (c *LinearClient) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c *LinearClient) graphQL(ctx context.Context, payload map[string]any) (map[string]any, error) {
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode linear graphql payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, linearResolveURL(c.BaseURL, "/graphql"), bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("create linear graphql request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.Token))

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("do linear graphql request: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read linear graphql response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("linear graphql request: %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("decode linear graphql response: %w", err)
	}
	if errs, ok := decoded["errors"].([]any); ok && len(errs) > 0 {
		return nil, fmt.Errorf("linear graphql error: %v", errs[0])
	}
	return decoded, nil
}

func linearResolveURL(baseURL, path string) string {
	return strings.TrimRight(strings.TrimSpace(baseURL), "/") + path
}

func verifyLinearSignature(header http.Header, raw []byte, signingSecret string, now time.Time) error {
	if strings.TrimSpace(signingSecret) == "" {
		return fmt.Errorf("linear signing secret is required")
	}
	tsMillis, err := timeFromLinearWebhook(header, raw)
	if err != nil {
		return err
	}
	if now.IsZero() {
		now = time.Now()
	}
	if delta := now.Sub(time.UnixMilli(tsMillis)); delta > time.Minute || delta < -time.Minute {
		return fmt.Errorf("linear webhook timestamp outside acceptable window")
	}
	got := strings.TrimSpace(header.Get("Linear-Signature"))
	if got == "" {
		return fmt.Errorf("linear webhook signature is required")
	}
	mac := hmac.New(sha256.New, []byte(signingSecret))
	_, _ = mac.Write(raw)
	want := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(strings.ToLower(got)), []byte(strings.ToLower(want))) {
		return fmt.Errorf("linear webhook signature mismatch")
	}
	return nil
}

func timeFromLinearWebhook(_ http.Header, raw []byte) (int64, error) {
	var payload struct {
		WebhookTimestamp int64 `json:"webhookTimestamp"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return 0, fmt.Errorf("decode linear webhook timestamp: %w", err)
	}
	if payload.WebhookTimestamp <= 0 {
		return 0, fmt.Errorf("linear webhook timestamp is required")
	}
	return payload.WebhookTimestamp, nil
}

func nestedMapValue(m map[string]any, key string) map[string]any {
	if m == nil {
		return nil
	}
	if v, ok := m[key].(map[string]any); ok {
		return v
	}
	return nil
}

type LinearConnector struct {
	client *http.Client
}

func NewLinearConnector(client *http.Client) *LinearConnector {
	return &LinearConnector{client: defaultHTTPClient(client)}
}

func (c *LinearConnector) Provider() Provider {
	return ProviderLinear
}

func (c *LinearConnector) ValidateInstallation(ctx context.Context, cfg InstallationConfig) (ValidationResult, error) {
	client := &LinearClient{
		BaseURL:    cfg.APIBaseURL,
		Token:      cfg.Token,
		HTTPClient: c.client,
	}
	err := client.Validate(ctx)
	details := map[string]string{}
	if strings.TrimSpace(cfg.APIBaseURL) != "" {
		details["api_base_url"] = strings.TrimSpace(cfg.APIBaseURL)
	}
	if strings.TrimSpace(cfg.Token) != "" {
		details["token_present"] = "true"
	}
	result := ValidationResult{
		Provider:  ProviderLinear,
		OK:        err == nil,
		CheckedAt: nowUTC(),
		Message:   "linear token validated via viewer query",
		Details:   details,
	}
	if err != nil {
		result.Message = err.Error()
	}
	return result, err
}

func (c *LinearConnector) VerifyWebhook(headers http.Header, body []byte, cfg InstallationConfig) (WebhookVerification, error) {
	req, err := http.NewRequest(http.MethodPost, "http://linear.local/webhook", bytes.NewReader(body))
	if err != nil {
		return WebhookVerification{Provider: ProviderLinear, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	req.Header = headers.Clone()
	event, err := NormalizeLinearWebhookRequest(req, cfg.WebhookSecret, time.Now().UTC())
	if err != nil {
		return WebhookVerification{Provider: ProviderLinear, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	return WebhookVerification{
		Provider:   ProviderLinear,
		Verified:   true,
		EventType:  event.Kind,
		DeliveryID: event.WebhookID,
		CheckedAt:  nowUTC(),
		Message:    "linear webhook signature verified",
	}, nil
}

func (c *LinearConnector) NormalizeEvent(headers http.Header, body []byte, cfg InstallationConfig) ([]Event, error) {
	req, err := http.NewRequest(http.MethodPost, "http://linear.local/webhook", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header = headers.Clone()
	normalized, err := NormalizeLinearWebhookRequest(req, cfg.WebhookSecret, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(normalized.Raw)
	return []Event{{
		Provider:    ProviderLinear,
		Kind:        normalized.Kind,
		Action:      normalized.Action,
		SubjectType: normalized.EntityType,
		SubjectID:   normalized.EntityID,
		Actor:       normalized.Actor,
		Title:       normalized.Title,
		URL:         normalized.URL,
		OccurredAt:  normalized.WebhookTimestamp,
		Details: map[string]string{
			"comment": normalized.Comment,
		},
		Raw: raw,
	}}, nil
}

func (c *LinearConnector) ExecuteAction(ctx context.Context, req ActionRequest, cfg InstallationConfig) (ActionResult, error) {
	client := &LinearClient{
		BaseURL:    cfg.APIBaseURL,
		Token:      cfg.Token,
		HTTPClient: c.client,
	}
	switch req.Action {
	case ActionLinearCreateIssue:
		issue, err := client.CreateIssue(ctx, req.TeamID, req.Title, req.Description)
		if err != nil {
			return ActionResult{Provider: ProviderLinear, Action: req.Action, CheckedAt: nowUTC(), Message: err.Error()}, err
		}
		return ActionResult{
			Provider:   ProviderLinear,
			Action:     req.Action,
			OK:         true,
			ExternalID: issue.ID,
			URL:        issue.URL,
			CheckedAt:  nowUTC(),
			Message:    "linear issue created",
			Details: map[string]string{
				"title": issue.Title,
			},
		}, nil
	case ActionLinearAddComment:
		comment, err := client.AddComment(ctx, req.IssueID, req.Body)
		if err != nil {
			return ActionResult{Provider: ProviderLinear, Action: req.Action, CheckedAt: nowUTC(), Message: err.Error()}, err
		}
		return ActionResult{
			Provider:   ProviderLinear,
			Action:     req.Action,
			OK:         true,
			ExternalID: comment.ID,
			URL:        comment.URL,
			CheckedAt:  nowUTC(),
			Message:    "linear issue comment created",
			Details: map[string]string{
				"issue_id": req.IssueID,
				"body":     comment.Body,
			},
		}, nil
	default:
		err := fmt.Errorf("unsupported linear action %q", req.Action)
		return ActionResult{Provider: ProviderLinear, Action: req.Action, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
}

func (c *LinearConnector) DeriveStatus(validation ValidationResult, webhook WebhookVerification) ConnectorStatus {
	return statusFromResults(ProviderLinear, validation, webhook)
}

func nestedMap(m map[string]any, keys ...string) (map[string]any, bool) {
	current := m
	for _, key := range keys {
		next, ok := current[key].(map[string]any)
		if !ok {
			return nil, false
		}
		current = next
	}
	return current, true
}

func stringFromMap(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key]; ok {
		switch x := v.(type) {
		case string:
			return x
		case fmt.Stringer:
			return x.String()
		}
	}
	return ""
}

func int64FromMap(m map[string]any, key string) int64 {
	if m == nil {
		return 0
	}
	switch v := m[key].(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case json.Number:
		n, _ := v.Int64()
		return n
	}
	return 0
}
