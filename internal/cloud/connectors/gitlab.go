package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const gitlabDefaultAPIBaseURL = "https://gitlab.com/api/v4"

type GitLabConnector struct {
	client *http.Client
}

func NewGitLabConnector(client *http.Client) *GitLabConnector {
	return &GitLabConnector{client: defaultHTTPClient(client)}
}

func (c *GitLabConnector) Provider() Provider {
	return ProviderGitLab
}

func (c *GitLabConnector) ValidateInstallation(ctx context.Context, cfg InstallationConfig) (ValidationResult, error) {
	if err := nonEmpty(cfg.Token, "token"); err != nil {
		return ValidationResult{Provider: ProviderGitLab, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	baseURL, err := resolveAPIBase(gitlabDefaultAPIBaseURL, cfg.APIBaseURL)
	if err != nil {
		return ValidationResult{Provider: ProviderGitLab, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	endpoint, err := apiURL(baseURL, "user")
	if err != nil {
		return ValidationResult{Provider: ProviderGitLab, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	req, err := newJSONRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ValidationResult{Provider: ProviderGitLab, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	req.Header.Set("PRIVATE-TOKEN", cfg.Token)
	var response struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
		Name     string `json:"name"`
		WebURL   string `json:"web_url"`
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return ValidationResult{Provider: ProviderGitLab, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	if err := readJSONResponse(resp, &response); err != nil {
		return ValidationResult{Provider: ProviderGitLab, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	return ValidationResult{
		Provider:  ProviderGitLab,
		OK:        true,
		Identity:  response.Username,
		CheckedAt: nowUTC(),
		Details: map[string]string{
			"user_id": strconv.FormatInt(response.ID, 10),
			"name":    response.Name,
			"web_url": response.WebURL,
		},
		Message: "token validated against GitLab user endpoint",
	}, nil
}

func (c *GitLabConnector) VerifyWebhook(headers http.Header, body []byte, cfg InstallationConfig) (WebhookVerification, error) {
	if err := nonEmpty(cfg.WebhookSecret, "webhook_secret"); err != nil {
		return WebhookVerification{Provider: ProviderGitLab, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	token := headers.Get("X-Gitlab-Token")
	if token != cfg.WebhookSecret {
		err := fmt.Errorf("gitlab webhook token mismatch")
		return WebhookVerification{Provider: ProviderGitLab, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	return WebhookVerification{
		Provider:   ProviderGitLab,
		Verified:   true,
		EventType:  headers.Get("X-Gitlab-Event"),
		DeliveryID: headers.Get("X-Request-Id"),
		CheckedAt:  nowUTC(),
		Message:    "gitlab webhook token verified",
	}, nil
}

func (c *GitLabConnector) NormalizeEvent(headers http.Header, body []byte, cfg InstallationConfig) ([]Event, error) {
	if len(body) == 0 {
		return nil, fmt.Errorf("gitlab webhook payload is empty")
	}
	var envelope struct {
		ObjectKind string `json:"object_kind"`
		EventType  string `json:"event_type"`
		UserName   string `json:"user_username"`
		User       struct {
			Username string `json:"username"`
			Name     string `json:"name"`
		} `json:"user"`
		Project struct {
			PathWithNamespace string `json:"path_with_namespace"`
			WebURL            string `json:"web_url"`
		} `json:"project"`
		ObjectAttributes struct {
			Action string `json:"action"`
			IID    int    `json:"iid"`
			Title  string `json:"title"`
			URL    string `json:"url"`
			State  string `json:"state"`
		} `json:"object_attributes"`
		Ref         string `json:"ref"`
		CheckoutSHA string `json:"checkout_sha"`
		BeforeSHA   string `json:"before"`
		AfterSHA    string `json:"after"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	kind := envelope.ObjectKind
	if kind == "" {
		kind = headers.Get("X-Gitlab-Event")
		kind = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(kind)), " event")
	}
	switch kind {
	case "issue":
		return []Event{normalizeGitLabIssue(body, envelope)}, nil
	case "merge_request":
		return []Event{normalizeGitLabMergeRequest(body, envelope)}, nil
	case "push":
		return []Event{normalizeGitLabPush(body, envelope)}, nil
	default:
		return nil, fmt.Errorf("unsupported gitlab event %q", kind)
	}
}

func (c *GitLabConnector) ExecuteAction(ctx context.Context, req ActionRequest, cfg InstallationConfig) (ActionResult, error) {
	if err := nonEmpty(cfg.Token, "token"); err != nil {
		return ActionResult{Provider: ProviderGitLab, Action: req.Action, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	if req.Action != ActionGitLabCreateIssueNote {
		err := fmt.Errorf("unsupported gitlab action %q", req.Action)
		return ActionResult{Provider: ProviderGitLab, Action: req.Action, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	if err := nonEmpty(req.Project, "project"); err != nil {
		return ActionResult{Provider: ProviderGitLab, Action: req.Action, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	if req.IssueNumber <= 0 {
		err := fmt.Errorf("issue_number must be greater than zero")
		return ActionResult{Provider: ProviderGitLab, Action: req.Action, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	if err := nonEmpty(req.Body, "body"); err != nil {
		return ActionResult{Provider: ProviderGitLab, Action: req.Action, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	baseURL, err := resolveAPIBase(gitlabDefaultAPIBaseURL, cfg.APIBaseURL)
	if err != nil {
		return ActionResult{Provider: ProviderGitLab, Action: req.Action, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	endpoint, err := apiURL(baseURL, "projects", req.Project, "issues", strconv.Itoa(req.IssueNumber), "notes")
	if err != nil {
		return ActionResult{Provider: ProviderGitLab, Action: req.Action, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	form := url.Values{}
	form.Set("body", req.Body)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return ActionResult{Provider: ProviderGitLab, Action: req.Action, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	httpReq.Header.Set("PRIVATE-TOKEN", cfg.Token)
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", userAgent)
	var response struct {
		ID     int64  `json:"id"`
		Body   string `json:"body"`
		WebURL string `json:"web_url"`
	}
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return ActionResult{Provider: ProviderGitLab, Action: req.Action, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	if err := readJSONResponse(resp, &response); err != nil {
		return ActionResult{Provider: ProviderGitLab, Action: req.Action, CheckedAt: nowUTC(), Message: err.Error()}, err
	}
	return ActionResult{
		Provider:   ProviderGitLab,
		Action:     req.Action,
		OK:         true,
		StatusCode: resp.StatusCode,
		ExternalID: strconv.FormatInt(response.ID, 10),
		URL:        response.WebURL,
		CheckedAt:  nowUTC(),
		Message:    "gitlab issue note created",
	}, nil
}

func (c *GitLabConnector) DeriveStatus(validation ValidationResult, webhook WebhookVerification) ConnectorStatus {
	return statusFromResults(ProviderGitLab, validation, webhook)
}

type gitLabIssuePayload struct {
	ObjectAttributes struct {
		Action string `json:"action"`
		IID    int    `json:"iid"`
		Title  string `json:"title"`
		URL    string `json:"url"`
	} `json:"object_attributes"`
	Project struct {
		PathWithNamespace string `json:"path_with_namespace"`
		WebURL            string `json:"web_url"`
	} `json:"project"`
	User struct {
		Username string `json:"username"`
		Name     string `json:"name"`
	} `json:"user"`
}

type gitLabMergeRequestPayload struct {
	ObjectAttributes struct {
		Action string `json:"action"`
		IID    int    `json:"iid"`
		Title  string `json:"title"`
		URL    string `json:"url"`
		State  string `json:"state"`
	} `json:"object_attributes"`
	Project struct {
		PathWithNamespace string `json:"path_with_namespace"`
		WebURL            string `json:"web_url"`
	} `json:"project"`
	User struct {
		Username string `json:"username"`
		Name     string `json:"name"`
	} `json:"user"`
}

type gitLabPushPayload struct {
	Ref         string `json:"ref"`
	CheckoutSHA string `json:"checkout_sha"`
	BeforeSHA   string `json:"before"`
	AfterSHA    string `json:"after"`
	UserName    string `json:"user_username"`
	Project     struct {
		PathWithNamespace string `json:"path_with_namespace"`
		WebURL            string `json:"web_url"`
	} `json:"project"`
}

func normalizeGitLabIssue(body []byte, envelope struct {
	ObjectKind string `json:"object_kind"`
	EventType  string `json:"event_type"`
	UserName   string `json:"user_username"`
	User       struct {
		Username string `json:"username"`
		Name     string `json:"name"`
	} `json:"user"`
	Project struct {
		PathWithNamespace string `json:"path_with_namespace"`
		WebURL            string `json:"web_url"`
	} `json:"project"`
	ObjectAttributes struct {
		Action string `json:"action"`
		IID    int    `json:"iid"`
		Title  string `json:"title"`
		URL    string `json:"url"`
		State  string `json:"state"`
	} `json:"object_attributes"`
	Ref         string `json:"ref"`
	CheckoutSHA string `json:"checkout_sha"`
	BeforeSHA   string `json:"before"`
	AfterSHA    string `json:"after"`
}) Event {
	action := envelope.ObjectAttributes.Action
	if action == "" {
		action = "open"
	}
	return Event{
		Provider:    ProviderGitLab,
		Kind:        "issue." + gitLabActionSuffix(action),
		Action:      action,
		SubjectType: "issue",
		SubjectID:   strconv.Itoa(envelope.ObjectAttributes.IID),
		Project:     envelope.Project.PathWithNamespace,
		Actor:       firstNonEmpty(envelope.User.Username, envelope.UserName, envelope.User.Name),
		Title:       envelope.ObjectAttributes.Title,
		URL:         envelope.ObjectAttributes.URL,
		OccurredAt:  nowUTC(),
		Raw:         append(json.RawMessage(nil), body...),
	}
}

func normalizeGitLabMergeRequest(body []byte, envelope struct {
	ObjectKind string `json:"object_kind"`
	EventType  string `json:"event_type"`
	UserName   string `json:"user_username"`
	User       struct {
		Username string `json:"username"`
		Name     string `json:"name"`
	} `json:"user"`
	Project struct {
		PathWithNamespace string `json:"path_with_namespace"`
		WebURL            string `json:"web_url"`
	} `json:"project"`
	ObjectAttributes struct {
		Action string `json:"action"`
		IID    int    `json:"iid"`
		Title  string `json:"title"`
		URL    string `json:"url"`
		State  string `json:"state"`
	} `json:"object_attributes"`
	Ref         string `json:"ref"`
	CheckoutSHA string `json:"checkout_sha"`
	BeforeSHA   string `json:"before"`
	AfterSHA    string `json:"after"`
}) Event {
	action := envelope.ObjectAttributes.Action
	if action == "" {
		action = "open"
	}
	kind := "merge_request." + gitLabActionSuffix(action)
	if action == "merge" {
		kind = "merge_request.merged"
	}
	return Event{
		Provider:    ProviderGitLab,
		Kind:        kind,
		Action:      action,
		SubjectType: "merge_request",
		SubjectID:   strconv.Itoa(envelope.ObjectAttributes.IID),
		Project:     envelope.Project.PathWithNamespace,
		Actor:       firstNonEmpty(envelope.User.Username, envelope.UserName, envelope.User.Name),
		Title:       envelope.ObjectAttributes.Title,
		URL:         envelope.ObjectAttributes.URL,
		OccurredAt:  nowUTC(),
		Raw:         append(json.RawMessage(nil), body...),
	}
}

func normalizeGitLabPush(body []byte, envelope struct {
	ObjectKind string `json:"object_kind"`
	EventType  string `json:"event_type"`
	UserName   string `json:"user_username"`
	User       struct {
		Username string `json:"username"`
		Name     string `json:"name"`
	} `json:"user"`
	Project struct {
		PathWithNamespace string `json:"path_with_namespace"`
		WebURL            string `json:"web_url"`
	} `json:"project"`
	ObjectAttributes struct {
		Action string `json:"action"`
		IID    int    `json:"iid"`
		Title  string `json:"title"`
		URL    string `json:"url"`
		State  string `json:"state"`
	} `json:"object_attributes"`
	Ref         string `json:"ref"`
	CheckoutSHA string `json:"checkout_sha"`
	BeforeSHA   string `json:"before"`
	AfterSHA    string `json:"after"`
}) Event {
	ref := strings.TrimPrefix(envelope.Ref, "refs/heads/")
	return Event{
		Provider:    ProviderGitLab,
		Kind:        "push.pushed",
		Action:      "pushed",
		SubjectType: "push",
		SubjectID:   envelope.AfterSHA,
		Project:     envelope.Project.PathWithNamespace,
		Actor:       envelope.UserName,
		ExternalID:  envelope.AfterSHA,
		OccurredAt:  nowUTC(),
		Details: map[string]string{
			"before": envelope.BeforeSHA,
			"after":  envelope.AfterSHA,
			"ref":    ref,
		},
		Raw: append(json.RawMessage(nil), body...),
	}
}

func gitLabActionSuffix(action string) string {
	switch action {
	case "open", "opened":
		return "opened"
	case "reopen", "reopened":
		return "reopened"
	case "close", "closed":
		return "closed"
	case "update", "updated":
		return "updated"
	case "merge", "merged":
		return "merged"
	default:
		return action
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
