package connectors

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// JiraClient is a minimal Jira Cloud/Server REST client for validation, polling,
// and a small write surface.
type JiraClient struct {
	BaseURL    string
	Email      string
	APIToken   string
	HTTPClient *http.Client
}

// Name returns the provider name used by the cloud connector registry.
func (c *JiraClient) Name() string { return "jira" }

// SupportsWebhook stays false in this run; Jira is implemented polling-first and
// the connector matrix should describe that truthfully.
func (c *JiraClient) SupportsWebhook() bool { return false }

// JiraIssueSnapshot is the polling-friendly normalized view for an issue.
type JiraIssueSnapshot struct {
	Key       string    `json:"key"`
	Summary   string    `json:"summary,omitempty"`
	Status    string    `json:"status,omitempty"`
	Assignee  string    `json:"assignee,omitempty"`
	URL       string    `json:"url,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// JiraNormalizedEvent is a small normalized record for polling or webhook-style use.
type JiraNormalizedEvent struct {
	Provider string            `json:"provider"`
	Kind     string            `json:"kind"`
	Action   string            `json:"action"`
	Issue    JiraIssueSnapshot `json:"issue"`
	Comment  string            `json:"comment,omitempty"`
	Author   string            `json:"author,omitempty"`
	Raw      map[string]any    `json:"raw,omitempty"`
}

// JiraCommentResult captures a real write result from Jira's comment endpoint.
type JiraCommentResult struct {
	IssueKey   string    `json:"issue_key"`
	CommentID  string    `json:"comment_id"`
	Self       string    `json:"self,omitempty"`
	CreatedAt  time.Time `json:"created_at,omitempty"`
	Body       string    `json:"body,omitempty"`
	Author     string    `json:"author,omitempty"`
	Visibility string    `json:"visibility,omitempty"`
}

// Validate checks credentials by calling the authenticated /myself endpoint.
func (c *JiraClient) Validate(ctx context.Context) error {
	if c == nil {
		return fmt.Errorf("jira client is nil")
	}
	if strings.TrimSpace(c.BaseURL) == "" {
		return fmt.Errorf("jira base url is required")
	}
	if strings.TrimSpace(c.Email) == "" {
		return fmt.Errorf("jira email is required for basic auth")
	}
	if strings.TrimSpace(c.APIToken) == "" {
		return fmt.Errorf("jira api token is required for basic auth")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jiraResolveURL(c.BaseURL, "/rest/api/3/myself"), nil)
	if err != nil {
		return fmt.Errorf("create jira validation request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", jiraBasicAuth(c.Email, c.APIToken))

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("validate jira credentials: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read jira validation response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("validate jira credentials: %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	var decoded jiraAccountResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		return fmt.Errorf("decode jira validation response: %w", err)
	}
	if strings.TrimSpace(decoded.AccountID) == "" && strings.TrimSpace(decoded.DisplayName) == "" && strings.TrimSpace(decoded.Email) == "" {
		return fmt.Errorf("jira validation response missing identity fields")
	}
	return nil
}

// AddComment posts a real comment to an issue.
func (c *JiraClient) AddComment(ctx context.Context, issueKey, body string) (*JiraCommentResult, error) {
	if c == nil {
		return nil, fmt.Errorf("jira client is nil")
	}
	issueKey = strings.TrimSpace(issueKey)
	if issueKey == "" {
		return nil, fmt.Errorf("issue key is required")
	}
	if strings.TrimSpace(body) == "" {
		return nil, fmt.Errorf("comment body is required")
	}

	payload := map[string]any{"body": body}
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode jira comment payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, jiraResolveURL(c.BaseURL, "/rest/api/3/issue/"+url.PathEscape(issueKey)+"/comment"), bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("create jira comment request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", jiraBasicAuth(c.Email, c.APIToken))

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("post jira comment: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read jira comment response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("post jira comment: %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}

	var decoded jiraCommentResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("decode jira comment response: %w", err)
	}
	result := &JiraCommentResult{
		IssueKey:  issueKey,
		CommentID: decoded.ID,
		Self:      decoded.Self,
		Body:      decoded.Body.Text(),
		Author:    decoded.Author.DisplayName,
	}
	if decoded.Created != "" {
		if ts, err := jiraParseTime(decoded.Created); err == nil {
			result.CreatedAt = ts
		}
	}
	if decoded.Visibility != nil {
		result.Visibility = decoded.Visibility.Value
	}
	return result, nil
}

// TransitionIssue moves an issue to a named transition ID.
func (c *JiraClient) TransitionIssue(ctx context.Context, issueKey, transitionID string) (*JiraTransitionResult, error) {
	if c == nil {
		return nil, fmt.Errorf("jira client is nil")
	}
	issueKey = strings.TrimSpace(issueKey)
	transitionID = strings.TrimSpace(transitionID)
	if issueKey == "" {
		return nil, fmt.Errorf("issue key is required")
	}
	if transitionID == "" {
		return nil, fmt.Errorf("transition id is required")
	}

	payload := map[string]any{"transition": map[string]any{"id": transitionID}}
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode jira transition payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, jiraResolveURL(c.BaseURL, "/rest/api/3/issue/"+url.PathEscape(issueKey)+"/transitions"), bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("create jira transition request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", jiraBasicAuth(c.Email, c.APIToken))

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("transition jira issue: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read jira transition response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("transition jira issue: %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	result := &JiraTransitionResult{
		IssueKey:     issueKey,
		TransitionID: transitionID,
	}
	if len(data) > 0 {
		var decoded jiraTransitionResponse
		if err := json.Unmarshal(data, &decoded); err == nil {
			result.Status = decoded.ToString()
		}
	}
	return result, nil
}

// PollIssueSnapshots queries Jira search for issue snapshots updated since the
// provided timestamp. This is the polling-first ingest path for Jira in the
// cloud alpha backend.
func (c *JiraClient) PollIssueSnapshots(ctx context.Context, baseJQL string, since time.Time, maxResults int) ([]JiraNormalizedEvent, error) {
	if c == nil {
		return nil, fmt.Errorf("jira client is nil")
	}
	query := strings.TrimSpace(baseJQL)
	if query == "" {
		return nil, fmt.Errorf("jira polling requires jql or project_key")
	}
	if !since.IsZero() {
		clause := fmt.Sprintf(`updated >= "%s"`, since.UTC().Format("2006-01-02 15:04"))
		query = "(" + query + ") AND " + clause
	}
	if !strings.Contains(strings.ToUpper(query), "ORDER BY") {
		query += " ORDER BY updated ASC"
	}
	if maxResults <= 0 {
		maxResults = 50
	}

	endpoint := jiraResolveURL(c.BaseURL, "/rest/api/3/search")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create jira search request: %w", err)
	}
	values := req.URL.Query()
	values.Set("jql", query)
	values.Set("maxResults", strconv.Itoa(maxResults))
	values.Set("fields", "summary,status,assignee,updated")
	req.URL.RawQuery = values.Encode()
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", jiraBasicAuth(c.Email, c.APIToken))

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("poll jira issues: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("read jira search response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("poll jira issues: %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}

	var payload struct {
		Issues []jiraIssueEnvelope `json:"issues"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("decode jira search response: %w", err)
	}
	events := make([]JiraNormalizedEvent, 0, len(payload.Issues))
	for _, issue := range payload.Issues {
		raw, err := json.Marshal(issue)
		if err != nil {
			return nil, fmt.Errorf("encode jira issue snapshot: %w", err)
		}
		event, err := NormalizeJiraIssuePayload(raw)
		if err != nil {
			return nil, err
		}
		events = append(events, *event)
	}
	return events, nil
}

// NormalizeJiraIssuePayload normalizes a polling issue snapshot or an issue/comment webhook payload.
func NormalizeJiraIssuePayload(raw []byte) (*JiraNormalizedEvent, error) {
	var envelope jiraIssueEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decode jira payload: %w", err)
	}

	event := &JiraNormalizedEvent{
		Provider: "jira",
		Kind:     "issue_snapshot",
		Action:   "snapshot",
		Raw:      map[string]any{},
	}
	_ = json.Unmarshal(raw, &event.Raw)

	target := &envelope
	if envelope.Issue != nil {
		target = envelope.Issue
	}

	event.Issue = JiraIssueSnapshot{
		Key:      firstNonEmpty(target.Key, envelope.Key),
		Summary:  target.Fields.Summary,
		Status:   target.Fields.Status.Name,
		Assignee: target.Fields.Assignee.DisplayName,
		URL:      firstNonEmpty(target.Self, envelope.Self),
	}
	if target.Fields.Updated != "" {
		if ts, err := jiraParseTime(target.Fields.Updated); err == nil {
			event.Issue.UpdatedAt = ts
		}
	}

	if envelope.Comment != nil && (envelope.Comment.Body.Text() != "" || envelope.Comment.Author.DisplayName != "") {
		event.Kind = "issue_comment"
		event.Action = "commented"
		event.Comment = envelope.Comment.Body.Text()
		event.Author = envelope.Comment.Author.DisplayName
		if event.Issue.Key == "" && target.Key != "" {
			event.Issue.Key = target.Key
		}
	}
	return event, nil
}

func (c *JiraClient) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func jiraResolveURL(baseURL, path string) string {
	return strings.TrimRight(strings.TrimSpace(baseURL), "/") + path
}

func jiraBasicAuth(email, token string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(strings.TrimSpace(email)+":"+token))
}

func jiraParseTime(value string) (time.Time, error) {
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000-0700",
		"2006-01-02T15:04:05-0700",
	}
	var lastErr error
	for _, layout := range layouts {
		if ts, err := time.Parse(layout, strings.TrimSpace(value)); err == nil {
			return ts, nil
		} else {
			lastErr = err
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("invalid jira time")
	}
	return time.Time{}, lastErr
}

type jiraIssueEnvelope struct {
	Key     string             `json:"key"`
	Self    string             `json:"self"`
	Fields  jiraIssueFields    `json:"fields"`
	Issue   *jiraIssueEnvelope `json:"issue"`
	Comment *jiraCommentBody   `json:"comment"`
}

type jiraIssueFields struct {
	Summary  string       `json:"summary"`
	Status   jiraStatus   `json:"status"`
	Assignee jiraAssignee `json:"assignee"`
	Updated  string       `json:"updated"`
}

type jiraStatus struct {
	Name string `json:"name"`
}

type jiraAssignee struct {
	DisplayName string `json:"displayName"`
}

type jiraCommentBody struct {
	Body   jiraTextBody `json:"body"`
	Author jiraAuthor   `json:"author"`
}

type jiraAuthor struct {
	DisplayName string `json:"displayName"`
}

type jiraTextBody struct {
	Type  string `json:"type"`
	Value string `json:"text"`
}

func (t jiraTextBody) TextValue() string {
	if t.Type == "" {
		return t.Value
	}
	return t.Value
}

func (t jiraTextBody) Text() string {
	return t.TextValue()
}

type jiraCommentResponse struct {
	ID         string          `json:"id"`
	Self       string          `json:"self"`
	Body       jiraTextBody    `json:"body"`
	Author     jiraAuthor      `json:"author"`
	Created    string          `json:"created"`
	Visibility *jiraVisibility `json:"visibility"`
}

type jiraVisibility struct {
	Value string `json:"value"`
}

type jiraAccountResponse struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
	Email       string `json:"emailAddress"`
	AccountType string `json:"accountType"`
	Self        string `json:"self"`
}

type JiraTransitionResult struct {
	IssueKey     string `json:"issue_key"`
	TransitionID string `json:"transition_id"`
	Status       string `json:"status,omitempty"`
}

type jiraTransitionResponse struct {
	To jiraTransitionState `json:"to"`
}

type jiraTransitionState struct {
	Name string `json:"name"`
}

func (r jiraTransitionResponse) ToString() string {
	return r.To.Name
}

type JiraConnector struct {
	client *http.Client
}

func NewJiraConnector(client *http.Client) *JiraConnector {
	return &JiraConnector{client: defaultHTTPClient(client)}
}

func (c *JiraConnector) Provider() Provider {
	return ProviderJira
}

func (c *JiraConnector) ValidateConfig(cfg InstallationConfig) error {
	if err := nonEmpty(cfg.APIBaseURL, "api_base_url"); err != nil {
		return err
	}
	if err := nonEmpty(cfg.Username, "username"); err != nil {
		return err
	}
	if err := nonEmpty(cfg.Token, "token"); err != nil {
		return err
	}
	if cfg.Extras == nil || (strings.TrimSpace(cfg.Extras["jql"]) == "" && strings.TrimSpace(cfg.Extras["project_key"]) == "") {
		return fmt.Errorf("jira installation requires config.jql or config.project_key")
	}
	return nil
}

func (c *JiraConnector) ValidateInstallation(ctx context.Context, cfg InstallationConfig) (ValidationResult, error) {
	client := &JiraClient{
		BaseURL:    cfg.APIBaseURL,
		Email:      cfg.Username,
		APIToken:   cfg.Token,
		HTTPClient: c.client,
	}
	err := client.Validate(ctx)
	result := ValidationResult{
		Provider:  ProviderJira,
		OK:        err == nil,
		CheckedAt: nowUTC(),
		Message:   "jira credentials validated via /rest/api/3/myself",
		Details:   map[string]string{},
	}
	if strings.TrimSpace(cfg.APIBaseURL) != "" {
		result.Details["api_base_url"] = strings.TrimSpace(cfg.APIBaseURL)
	}
	if strings.TrimSpace(cfg.Username) != "" {
		result.Details["username"] = strings.TrimSpace(cfg.Username)
	}
	if err != nil {
		result.Message = err.Error()
	} else {
		result.Identity = cfg.Username
	}
	return result, err
}

func (c *JiraConnector) VerifyWebhook(_ http.Header, _ []byte, _ InstallationConfig) (WebhookVerification, error) {
	return WebhookVerification{
		Provider:  ProviderJira,
		Verified:  false,
		CheckedAt: nowUTC(),
		Message:   "jira is polling-first in this alpha cloud pass; webhook verification is not implemented",
	}, nil
}

func (c *JiraConnector) NormalizeEvent(_ http.Header, body []byte, _ InstallationConfig) ([]Event, error) {
	normalized, err := NormalizeJiraIssuePayload(body)
	if err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(normalized.Raw)
	return []Event{{
		Provider:    ProviderJira,
		Kind:        normalized.Kind,
		Action:      normalized.Action,
		SubjectType: "issue",
		SubjectID:   normalized.Issue.Key,
		Actor:       normalized.Author,
		Title:       normalized.Issue.Summary,
		URL:         normalized.Issue.URL,
		OccurredAt:  normalized.Issue.UpdatedAt,
		Details: map[string]string{
			"status":   normalized.Issue.Status,
			"assignee": normalized.Issue.Assignee,
			"comment":  normalized.Comment,
		},
		Raw: raw,
	}}, nil
}

func (c *JiraConnector) ExecuteAction(ctx context.Context, req ActionRequest, cfg InstallationConfig) (ActionResult, error) {
	client := &JiraClient{
		BaseURL:    cfg.APIBaseURL,
		Email:      cfg.Username,
		APIToken:   cfg.Token,
		HTTPClient: c.client,
	}
	switch req.Action {
	case ActionJiraAddIssueComment:
		result, err := client.AddComment(ctx, req.IssueKey, req.Body)
		if err != nil {
			return ActionResult{Provider: ProviderJira, Action: req.Action, CheckedAt: nowUTC(), Message: err.Error()}, err
		}
		return ActionResult{
			Provider:   ProviderJira,
			Action:     req.Action,
			OK:         true,
			ExternalID: result.CommentID,
			URL:        result.Self,
			CheckedAt:  nowUTC(),
			Message:    "jira issue comment created",
			Details: map[string]string{
				"issue_key": result.IssueKey,
				"author":    result.Author,
			},
		}, nil
	case ActionJiraTransitionIssue:
		result, err := client.TransitionIssue(ctx, req.IssueKey, req.TransitionID)
		if err != nil {
			return ActionResult{Provider: ProviderJira, Action: req.Action, CheckedAt: nowUTC(), Message: err.Error()}, err
		}
		details := map[string]string{
			"issue_key":     result.IssueKey,
			"transition_id": result.TransitionID,
		}
		if result.Status != "" {
			details["status"] = result.Status
		}
		return ActionResult{
			Provider:   ProviderJira,
			Action:     req.Action,
			OK:         true,
			ExternalID: result.TransitionID,
			CheckedAt:  nowUTC(),
			Message:    "jira issue transitioned",
			Details:    details,
		}, nil
	default:
		err := fmt.Errorf("unsupported jira action %q", req.Action)
		return ActionResult{Provider: ProviderJira, Action: req.Action, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
}

func (c *JiraConnector) DeriveStatus(validation ValidationResult, _ WebhookVerification) ConnectorStatus {
	status := validationOnlyStatus(ProviderJira, validation, "installation validated; jira webhook ingest is intentionally polling-first")
	if status.Details == nil {
		status.Details = map[string]string{}
	}
	status.Details["polling_mode"] = "polling-first"
	status.Details["webhook_ingest"] = "disabled"
	return status
}
