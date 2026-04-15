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
	"strconv"
	"strings"
	"time"
)

// SlackClient is a minimal Slack Web API client plus request verification helpers.
type SlackClient struct {
	BaseURL       string
	BotToken      string
	SigningSecret string
	HTTPClient    *http.Client
}

// Name returns the provider name used by the cloud connector registry.
func (c *SlackClient) Name() string { return "slack" }

// SupportsWebhook reports that Slack request-signature verification is implemented.
func (c *SlackClient) SupportsWebhook() bool { return true }

// SlackMessageResult captures a postMessage response.
type SlackMessageResult struct {
	OK      bool   `json:"ok"`
	Channel string `json:"channel"`
	TS      string `json:"ts"`
	Text    string `json:"text,omitempty"`
}

// SlackPostMessageOptions carries the small supported write surface.
type SlackPostMessageOptions struct {
	ThreadTS       string `json:"thread_ts,omitempty"`
	ReplyBroadcast bool   `json:"reply_broadcast,omitempty"`
}

// SlackNormalizedEvent is a practical normalized Slack event for notifications/approvals.
type SlackNormalizedEvent struct {
	Provider       string         `json:"provider"`
	Kind           string         `json:"kind"`
	Action         string         `json:"action"`
	ChannelID      string         `json:"channel_id,omitempty"`
	UserID         string         `json:"user_id,omitempty"`
	Text           string         `json:"text,omitempty"`
	MessageTS      string         `json:"message_ts,omitempty"`
	ThreadTS       string         `json:"thread_ts,omitempty"`
	CallbackID     string         `json:"callback_id,omitempty"`
	ApprovalAction string         `json:"approval_action,omitempty"`
	Command        string         `json:"command,omitempty"`
	WebhookType    string         `json:"webhook_type,omitempty"`
	ResponseURL    string         `json:"response_url,omitempty"`
	Raw            map[string]any `json:"raw,omitempty"`
}

// Validate checks the token by calling auth.test.
func (c *SlackClient) Validate(ctx context.Context) error {
	if c == nil {
		return fmt.Errorf("slack client is nil")
	}
	if strings.TrimSpace(c.BaseURL) == "" {
		return fmt.Errorf("slack base url is required")
	}
	if strings.TrimSpace(c.BotToken) == "" {
		return fmt.Errorf("slack bot token is required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, slackResolveURL(c.BaseURL, "/api/auth.test"), bytes.NewReader(nil))
	if err != nil {
		return fmt.Errorf("create slack auth.test request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.BotToken))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("validate slack token: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read slack auth.test response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("validate slack token: %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	var decoded struct {
		OK     bool   `json:"ok"`
		TeamID string `json:"team_id"`
		Team   string `json:"team"`
		UserID string `json:"user_id"`
		User   string `json:"user"`
		URL    string `json:"url"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return fmt.Errorf("decode slack auth.test response: %w", err)
	}
	if !decoded.OK {
		return errors.New("slack auth.test returned ok=false")
	}
	if strings.TrimSpace(decoded.TeamID) == "" && strings.TrimSpace(decoded.Team) == "" && strings.TrimSpace(decoded.UserID) == "" {
		return errors.New("slack auth.test response missing identity fields")
	}
	return nil
}

// PostMessage sends a real chat.postMessage request.
func (c *SlackClient) PostMessage(ctx context.Context, channel, text string, opts SlackPostMessageOptions) (*SlackMessageResult, error) {
	if c == nil {
		return nil, fmt.Errorf("slack client is nil")
	}
	channel = strings.TrimSpace(channel)
	text = strings.TrimSpace(text)
	if channel == "" {
		return nil, fmt.Errorf("channel is required")
	}
	if text == "" {
		return nil, fmt.Errorf("text is required")
	}

	payload := map[string]any{
		"channel": channel,
		"text":    text,
	}
	if strings.TrimSpace(opts.ThreadTS) != "" {
		payload["thread_ts"] = strings.TrimSpace(opts.ThreadTS)
	}
	if opts.ReplyBroadcast {
		payload["reply_broadcast"] = true
	}
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode slack message payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, slackResolveURL(c.BaseURL, "/api/chat.postMessage"), bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("create slack chat.postMessage request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.BotToken))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("post slack message: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read slack postMessage response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("post slack message: %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	var decoded SlackMessageResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("decode slack postMessage response: %w", err)
	}
	if !decoded.OK {
		return nil, fmt.Errorf("slack chat.postMessage returned ok=false")
	}
	return &decoded, nil
}

// UpdateMessage updates an existing Slack message through chat.update.
func (c *SlackClient) UpdateMessage(ctx context.Context, channel, ts, text string) (*SlackMessageResult, error) {
	if c == nil {
		return nil, fmt.Errorf("slack client is nil")
	}
	channel = strings.TrimSpace(channel)
	ts = strings.TrimSpace(ts)
	text = strings.TrimSpace(text)
	if channel == "" {
		return nil, fmt.Errorf("channel is required")
	}
	if ts == "" {
		return nil, fmt.Errorf("message ts is required")
	}
	if text == "" {
		return nil, fmt.Errorf("text is required")
	}

	payload := map[string]any{
		"channel": channel,
		"ts":      ts,
		"text":    text,
	}
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode slack update payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, slackResolveURL(c.BaseURL, "/api/chat.update"), bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("create slack chat.update request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.BotToken))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("update slack message: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read slack chat.update response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("update slack message: %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	var decoded SlackMessageResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("decode slack chat.update response: %w", err)
	}
	if !decoded.OK {
		return nil, fmt.Errorf("slack chat.update returned ok=false")
	}
	return &decoded, nil
}

// NormalizeSlackRequest verifies a Slack request and normalizes either an event callback,
// an interactive payload, or a slash command payload.
func NormalizeSlackRequest(req *http.Request, signingSecret string, now time.Time) ([]SlackNormalizedEvent, error) {
	if req == nil {
		return nil, fmt.Errorf("slack request is nil")
	}
	raw, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("read slack request body: %w", err)
	}
	req.Body = io.NopCloser(bytes.NewReader(raw))
	if err := verifySlackRequest(req.Header, raw, signingSecret, now); err != nil {
		return nil, err
	}

	contentType := req.Header.Get("Content-Type")
	if strings.Contains(strings.ToLower(contentType), "application/x-www-form-urlencoded") {
		form, err := url.ParseQuery(string(raw))
		if err != nil {
			return nil, fmt.Errorf("decode slack form payload: %w", err)
		}
		if payload := form.Get("payload"); payload != "" {
			return normalizeSlackInteractivePayload([]byte(payload))
		}
		return normalizeSlackSlashCommand(form), nil
	}
	return normalizeSlackEventPayload(raw)
}

func verifySlackRequest(header http.Header, raw []byte, signingSecret string, now time.Time) error {
	signingSecret = strings.TrimSpace(signingSecret)
	if signingSecret == "" {
		return fmt.Errorf("slack signing secret is required")
	}
	ts := strings.TrimSpace(header.Get("X-Slack-Request-Timestamp"))
	if ts == "" {
		return fmt.Errorf("slack request timestamp is required")
	}
	tsUnix, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return fmt.Errorf("parse slack request timestamp: %w", err)
	}
	if now.IsZero() {
		now = time.Now()
	}
	if delta := now.Sub(time.Unix(tsUnix, 0)); delta > 5*time.Minute || delta < -5*time.Minute {
		return fmt.Errorf("slack request timestamp outside acceptable window")
	}
	got := strings.TrimSpace(header.Get("X-Slack-Signature"))
	if got == "" {
		return fmt.Errorf("slack request signature is required")
	}
	basestring := "v0:" + ts + ":" + string(raw)
	mac := hmac.New(sha256.New, []byte(signingSecret))
	_, _ = mac.Write([]byte(basestring))
	want := "v0=" + hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(strings.ToLower(got)), []byte(strings.ToLower(want))) {
		return fmt.Errorf("slack request signature mismatch")
	}
	return nil
}

func normalizeSlackEventPayload(raw []byte) ([]SlackNormalizedEvent, error) {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode slack event payload: %w", err)
	}
	eventType := stringFromMap(payload, "type")
	switch eventType {
	case "url_verification":
		return []SlackNormalizedEvent{{
			Provider:    "slack",
			Kind:        "challenge",
			Action:      "verify",
			WebhookType: eventType,
			Raw:         payload,
		}}, nil
	case "event_callback":
		nested, _ := payload["event"].(map[string]any)
		event := SlackNormalizedEvent{
			Provider:    "slack",
			Kind:        strings.ToLower(stringFromMap(nested, "type")),
			Action:      slackActionForEventType(stringFromMap(nested, "type")),
			ChannelID:   stringFromMap(nested, "channel"),
			UserID:      stringFromMap(nested, "user"),
			Text:        stringFromMap(nested, "text"),
			MessageTS:   stringFromMap(nested, "ts"),
			ThreadTS:    stringFromMap(nested, "thread_ts"),
			CallbackID:  stringFromMap(payload, "event_id"),
			WebhookType: eventType,
			Raw:         payload,
		}
		if event.Kind == "" {
			event.Kind = "event"
		}
		if event.Action == "" {
			event.Action = "notify"
		}
		return []SlackNormalizedEvent{event}, nil
	default:
		return []SlackNormalizedEvent{{
			Provider:    "slack",
			Kind:        "event",
			Action:      "notify",
			WebhookType: eventType,
			Raw:         payload,
		}}, nil
	}
}

func normalizeSlackInteractivePayload(raw []byte) ([]SlackNormalizedEvent, error) {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode slack interactive payload: %w", err)
	}
	kind := stringFromMap(payload, "type")
	actions, _ := payload["actions"].([]any)
	actionID := ""
	actionValue := ""
	if len(actions) > 0 {
		if first, ok := actions[0].(map[string]any); ok {
			actionID = stringFromMap(first, "action_id")
			actionValue = stringFromMap(first, "value")
		}
	}
	approvalAction := firstNonEmpty(actionID, actionValue)
	if approvalAction == "" {
		approvalAction = "interactive"
	}
	return []SlackNormalizedEvent{{
		Provider:       "slack",
		Kind:           kind,
		Action:         approvalAction,
		ChannelID:      stringFromMap(nestedMapValue(payload, "channel"), "id"),
		UserID:         stringFromMap(nestedMapValue(payload, "user"), "id"),
		ThreadTS:       stringFromMap(nestedMapValue(payload, "message"), "thread_ts"),
		MessageTS:      stringFromMap(nestedMapValue(payload, "message"), "ts"),
		CallbackID:     stringFromMap(payload, "callback_id"),
		ApprovalAction: approvalAction,
		WebhookType:    "interactive",
		Raw:            payload,
	}}, nil
}

func normalizeSlackSlashCommand(form url.Values) []SlackNormalizedEvent {
	return []SlackNormalizedEvent{{
		Provider:    "slack",
		Kind:        "slash_command",
		Action:      "invoke",
		ChannelID:   form.Get("channel_id"),
		UserID:      form.Get("user_id"),
		Text:        form.Get("text"),
		Command:     form.Get("command"),
		ResponseURL: form.Get("response_url"),
		WebhookType: "slash_command",
		Raw: map[string]any{
			"channel_id":   form.Get("channel_id"),
			"user_id":      form.Get("user_id"),
			"text":         form.Get("text"),
			"command":      form.Get("command"),
			"response_url": form.Get("response_url"),
		},
	}}
}

func slackActionForEventType(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "app_mention":
		return "mention"
	case "message":
		return "message"
	case "reaction_added":
		return "reaction_added"
	case "reaction_removed":
		return "reaction_removed"
	default:
		return "notify"
	}
}

func slackResolveURL(baseURL, path string) string {
	return strings.TrimRight(strings.TrimSpace(baseURL), "/") + path
}

func (c *SlackClient) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

type SlackConnector struct {
	client *http.Client
}

func NewSlackConnector(client *http.Client) *SlackConnector {
	return &SlackConnector{client: defaultHTTPClient(client)}
}

func (c *SlackConnector) Provider() Provider {
	return ProviderSlack
}

func (c *SlackConnector) ValidateInstallation(ctx context.Context, cfg InstallationConfig) (ValidationResult, error) {
	client := &SlackClient{
		BaseURL:    cfg.APIBaseURL,
		BotToken:   cfg.Token,
		HTTPClient: c.client,
	}
	err := client.Validate(ctx)
	result := ValidationResult{
		Provider:  ProviderSlack,
		OK:        err == nil,
		CheckedAt: nowUTC(),
		Message:   "slack bot token validated via auth.test",
		Details:   map[string]string{},
	}
	if err != nil {
		result.Message = err.Error()
	} else {
		result.Details["provider"] = "slack"
		result.Details["base_url"] = strings.TrimSpace(cfg.APIBaseURL)
	}
	return result, err
}

func (c *SlackConnector) VerifyWebhook(headers http.Header, body []byte, cfg InstallationConfig) (WebhookVerification, error) {
	req, err := http.NewRequest(http.MethodPost, "http://slack.local/webhook", bytes.NewReader(body))
	if err != nil {
		return WebhookVerification{Provider: ProviderSlack, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	req.Header = headers.Clone()
	events, err := NormalizeSlackRequest(req, cfg.WebhookSecret, time.Now().UTC())
	if err != nil {
		return WebhookVerification{Provider: ProviderSlack, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	eventType := ""
	if len(events) > 0 {
		eventType = events[0].Kind
	}
	return WebhookVerification{
		Provider:   ProviderSlack,
		Verified:   true,
		EventType:  eventType,
		DeliveryID: headers.Get("X-Slack-Request-Timestamp"),
		CheckedAt:  nowUTC(),
		Message:    "slack request signature verified",
	}, nil
}

func (c *SlackConnector) NormalizeEvent(headers http.Header, body []byte, cfg InstallationConfig) ([]Event, error) {
	req, err := http.NewRequest(http.MethodPost, "http://slack.local/webhook", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header = headers.Clone()
	normalized, err := NormalizeSlackRequest(req, cfg.WebhookSecret, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	events := make([]Event, 0, len(normalized))
	for _, item := range normalized {
		raw, _ := json.Marshal(item.Raw)
		events = append(events, Event{
			Provider:    ProviderSlack,
			Kind:        item.Kind,
			Action:      item.Action,
			SubjectType: "slack_event",
			SubjectID:   item.CallbackID,
			Actor:       item.UserID,
			Title:       item.Text,
			URL:         item.ResponseURL,
			Details: map[string]string{
				"channel_id":      item.ChannelID,
				"message_ts":      item.MessageTS,
				"thread_ts":       item.ThreadTS,
				"command":         item.Command,
				"approval_action": item.ApprovalAction,
				"webhook_type":    item.WebhookType,
			},
			Raw: raw,
		})
	}
	return events, nil
}

func (c *SlackConnector) ExecuteAction(ctx context.Context, req ActionRequest, cfg InstallationConfig) (ActionResult, error) {
	client := &SlackClient{
		BaseURL:    cfg.APIBaseURL,
		BotToken:   cfg.Token,
		HTTPClient: c.client,
	}
	switch req.Action {
	case ActionSlackPostMessage:
		result, err := client.PostMessage(ctx, req.Channel, req.Body, SlackPostMessageOptions{ThreadTS: req.ThreadTS})
		if err != nil {
			return ActionResult{Provider: ProviderSlack, Action: req.Action, CheckedAt: nowUTC(), Message: err.Error()}, err
		}
		return ActionResult{
			Provider:   ProviderSlack,
			Action:     req.Action,
			OK:         result.OK,
			ExternalID: result.TS,
			CheckedAt:  nowUTC(),
			Message:    "slack message posted",
			Details: map[string]string{
				"channel": result.Channel,
				"text":    result.Text,
			},
		}, nil
	case ActionSlackUpdateMessage:
		result, err := client.UpdateMessage(ctx, req.Channel, firstNonEmpty(req.MessageTS, req.ThreadTS), req.Body)
		if err != nil {
			return ActionResult{Provider: ProviderSlack, Action: req.Action, CheckedAt: nowUTC(), Message: err.Error()}, err
		}
		return ActionResult{
			Provider:   ProviderSlack,
			Action:     req.Action,
			OK:         result.OK,
			ExternalID: result.TS,
			CheckedAt:  nowUTC(),
			Message:    "slack message updated",
			Details: map[string]string{
				"channel": result.Channel,
				"text":    result.Text,
			},
		}, nil
	default:
		err := fmt.Errorf("unsupported slack action %q", req.Action)
		return ActionResult{Provider: ProviderSlack, Action: req.Action, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
}

func (c *SlackConnector) DeriveStatus(validation ValidationResult, webhook WebhookVerification) ConnectorStatus {
	return statusFromResults(ProviderSlack, validation, webhook)
}
