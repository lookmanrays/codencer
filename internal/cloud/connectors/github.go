package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const githubDefaultAPIBaseURL = "https://api.github.com"

type GitHubConnector struct {
	client *http.Client
}

func NewGitHubConnector(client *http.Client) *GitHubConnector {
	return &GitHubConnector{client: defaultHTTPClient(client)}
}

func (c *GitHubConnector) Provider() Provider {
	return ProviderGitHub
}

func (c *GitHubConnector) ValidateInstallation(ctx context.Context, cfg InstallationConfig) (ValidationResult, error) {
	if err := nonEmpty(cfg.Token, "token"); err != nil {
		return ValidationResult{Provider: ProviderGitHub, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	baseURL, err := resolveAPIBase(githubDefaultAPIBaseURL, cfg.APIBaseURL)
	if err != nil {
		return ValidationResult{Provider: ProviderGitHub, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	endpoint, err := apiURL(baseURL, "user")
	if err != nil {
		return ValidationResult{Provider: ProviderGitHub, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	req, err := newJSONRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ValidationResult{Provider: ProviderGitHub, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	var response struct {
		Login   string `json:"login"`
		ID      int64  `json:"id"`
		HTMLURL string `json:"html_url"`
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return ValidationResult{Provider: ProviderGitHub, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	if err := readJSONResponse(resp, &response); err != nil {
		return ValidationResult{Provider: ProviderGitHub, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	return ValidationResult{
		Provider:  ProviderGitHub,
		OK:        true,
		Identity:  response.Login,
		CheckedAt: nowUTC(),
		Details: map[string]string{
			"user_id":  strconv.FormatInt(response.ID, 10),
			"html_url": response.HTMLURL,
		},
		Message: "token validated against GitHub user endpoint",
	}, nil
}

func (c *GitHubConnector) VerifyWebhook(headers http.Header, body []byte, cfg InstallationConfig) (WebhookVerification, error) {
	if err := nonEmpty(cfg.WebhookSecret, "webhook_secret"); err != nil {
		return WebhookVerification{Provider: ProviderGitHub, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	signature := headers.Get("X-Hub-Signature-256")
	if !verifyHMACSignature(cfg.WebhookSecret, signature, body) {
		err := fmt.Errorf("github webhook signature mismatch")
		return WebhookVerification{Provider: ProviderGitHub, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	return WebhookVerification{
		Provider:   ProviderGitHub,
		Verified:   true,
		EventType:  headers.Get("X-GitHub-Event"),
		DeliveryID: headers.Get("X-GitHub-Delivery"),
		CheckedAt:  nowUTC(),
		Message:    "github webhook signature verified",
	}, nil
}

func (c *GitHubConnector) NormalizeEvent(headers http.Header, body []byte, cfg InstallationConfig) ([]Event, error) {
	if len(body) == 0 {
		return nil, fmt.Errorf("github webhook payload is empty")
	}
	eventType := headers.Get("X-GitHub-Event")
	if eventType == "" {
		return nil, fmt.Errorf("missing X-GitHub-Event header")
	}
	switch eventType {
	case "issues":
		return []Event{normalizeGitHubIssue(body)}, nil
	case "pull_request":
		return []Event{normalizeGitHubPullRequest(body)}, nil
	case "push":
		return []Event{normalizeGitHubPush(body)}, nil
	default:
		return nil, fmt.Errorf("unsupported github event %q", eventType)
	}
}

func (c *GitHubConnector) ExecuteAction(ctx context.Context, req ActionRequest, cfg InstallationConfig) (ActionResult, error) {
	if err := nonEmpty(cfg.Token, "token"); err != nil {
		return ActionResult{Provider: ProviderGitHub, Action: req.Action, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	if req.Action != ActionGitHubCreateIssueComment {
		err := fmt.Errorf("unsupported github action %q", req.Action)
		return ActionResult{Provider: ProviderGitHub, Action: req.Action, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	if err := nonEmpty(req.Repository, "repository"); err != nil {
		return ActionResult{Provider: ProviderGitHub, Action: req.Action, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	if req.IssueNumber <= 0 {
		err := fmt.Errorf("issue_number must be greater than zero")
		return ActionResult{Provider: ProviderGitHub, Action: req.Action, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	if err := nonEmpty(req.Body, "body"); err != nil {
		return ActionResult{Provider: ProviderGitHub, Action: req.Action, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	baseURL, err := resolveAPIBase(githubDefaultAPIBaseURL, cfg.APIBaseURL)
	if err != nil {
		return ActionResult{Provider: ProviderGitHub, Action: req.Action, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	owner, repo, err := splitGitHubRepository(req.Repository)
	if err != nil {
		return ActionResult{Provider: ProviderGitHub, Action: req.Action, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	endpoint, err := apiURL(baseURL, "repos", owner, repo, "issues", strconv.Itoa(req.IssueNumber), "comments")
	if err != nil {
		return ActionResult{Provider: ProviderGitHub, Action: req.Action, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	payload := map[string]string{"body": req.Body}
	httpReq, err := newJSONRequest(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return ActionResult{Provider: ProviderGitHub, Action: req.Action, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+cfg.Token)
	httpReq.Header.Set("Accept", "application/vnd.github+json")
	var response struct {
		ID      int64  `json:"id"`
		HTMLURL string `json:"html_url"`
	}
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return ActionResult{Provider: ProviderGitHub, Action: req.Action, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	if err := readJSONResponse(resp, &response); err != nil {
		return ActionResult{Provider: ProviderGitHub, Action: req.Action, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	return ActionResult{
		Provider:   ProviderGitHub,
		Action:     req.Action,
		OK:         true,
		StatusCode: resp.StatusCode,
		ExternalID: strconv.FormatInt(response.ID, 10),
		URL:        response.HTMLURL,
		CheckedAt:  nowUTC(),
		Message:    "github issue comment created",
	}, nil
}

func (c *GitHubConnector) DeriveStatus(validation ValidationResult, webhook WebhookVerification) ConnectorStatus {
	return statusFromResults(ProviderGitHub, validation, webhook)
}

func nowUTC() time.Time {
	return time.Now().UTC()
}

type githubIssuePayload struct {
	Action     string         `json:"action"`
	Issue      githubIssueRef `json:"issue"`
	Repository githubRepoRef  `json:"repository"`
	Sender     githubUserRef  `json:"sender"`
}

type githubIssueRef struct {
	ID      int64  `json:"id"`
	Number  int    `json:"number"`
	Title   string `json:"title"`
	HTMLURL string `json:"html_url"`
}

type githubRepoRef struct {
	FullName string `json:"full_name"`
}

type githubUserRef struct {
	Login string `json:"login"`
}

type githubPullRequestPayload struct {
	Action      string        `json:"action"`
	PullRequest githubPRRef   `json:"pull_request"`
	Repository  githubRepoRef `json:"repository"`
	Sender      githubUserRef `json:"sender"`
}

type githubPRRef struct {
	ID      int64  `json:"id"`
	Number  int    `json:"number"`
	Title   string `json:"title"`
	HTMLURL string `json:"html_url"`
	Merged  bool   `json:"merged"`
}

type githubPushPayload struct {
	Ref        string        `json:"ref"`
	Before     string        `json:"before"`
	After      string        `json:"after"`
	Repository githubRepoRef `json:"repository"`
	Sender     githubUserRef `json:"sender"`
}

func normalizeGitHubIssue(body []byte) Event {
	var payload githubIssuePayload
	_ = json.Unmarshal(body, &payload)
	action := payload.Action
	if action == "" {
		action = "opened"
	}
	return Event{
		Provider:    ProviderGitHub,
		Kind:        "issue." + action,
		Action:      action,
		SubjectType: "issue",
		SubjectID:   strconv.Itoa(payload.Issue.Number),
		Repository:  payload.Repository.FullName,
		Actor:       payload.Sender.Login,
		Title:       payload.Issue.Title,
		URL:         payload.Issue.HTMLURL,
		ExternalID:  strconv.FormatInt(payload.Issue.ID, 10),
		OccurredAt:  nowUTC(),
		Raw:         append(json.RawMessage(nil), body...),
	}
}

func normalizeGitHubPullRequest(body []byte) Event {
	var payload githubPullRequestPayload
	_ = json.Unmarshal(body, &payload)
	action := payload.Action
	if action == "" {
		action = "opened"
	}
	kind := "pull_request." + action
	if action == "closed" && payload.PullRequest.Merged {
		kind = "pull_request.merged"
		action = "merged"
	}
	return Event{
		Provider:    ProviderGitHub,
		Kind:        kind,
		Action:      action,
		SubjectType: "pull_request",
		SubjectID:   strconv.Itoa(payload.PullRequest.Number),
		Repository:  payload.Repository.FullName,
		Actor:       payload.Sender.Login,
		Title:       payload.PullRequest.Title,
		URL:         payload.PullRequest.HTMLURL,
		ExternalID:  strconv.FormatInt(payload.PullRequest.ID, 10),
		OccurredAt:  nowUTC(),
		Raw:         append(json.RawMessage(nil), body...),
	}
}

func normalizeGitHubPush(body []byte) Event {
	var payload githubPushPayload
	_ = json.Unmarshal(body, &payload)
	ref := strings.TrimPrefix(payload.Ref, "refs/heads/")
	return Event{
		Provider:    ProviderGitHub,
		Kind:        "push.pushed",
		Action:      "pushed",
		SubjectType: "push",
		SubjectID:   payload.After,
		Repository:  payload.Repository.FullName,
		Actor:       payload.Sender.Login,
		ExternalID:  payload.After,
		OccurredAt:  nowUTC(),
		Details: map[string]string{
			"before": payload.Before,
			"after":  payload.After,
			"ref":    ref,
		},
		Raw: append(json.RawMessage(nil), body...),
	}
}

func splitGitHubRepository(repository string) (string, string, error) {
	parts := strings.Split(strings.TrimSpace(repository), "/")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", fmt.Errorf("repository must be in owner/repo form")
	}
	return parts[0], parts[1], nil
}
