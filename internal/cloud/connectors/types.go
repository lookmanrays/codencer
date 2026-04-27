package connectors

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type Provider string

const (
	ProviderGitHub Provider = "github"
	ProviderGitLab Provider = "gitlab"
	ProviderJira   Provider = "jira"
	ProviderLinear Provider = "linear"
	ProviderSlack  Provider = "slack"
)

type ActionName string

const (
	ActionGitHubCreateIssueComment ActionName = "create_issue_comment"
	ActionGitHubCreateIssue        ActionName = "create_issue"
	ActionGitLabCreateIssueNote    ActionName = "create_issue_note"
	ActionGitLabCreateIssue        ActionName = "create_issue"
	ActionJiraAddIssueComment      ActionName = "add_issue_comment"
	ActionJiraTransitionIssue      ActionName = "transition_issue"
	ActionLinearCreateIssue        ActionName = "create_issue"
	ActionLinearAddComment         ActionName = "add_comment"
	ActionSlackPostMessage         ActionName = "post_message"
	ActionSlackUpdateMessage       ActionName = "update_message"
)

type InstallationConfig struct {
	APIBaseURL    string            `json:"api_base_url,omitempty"`
	Token         string            `json:"token,omitempty"`
	WebhookSecret string            `json:"webhook_secret,omitempty"`
	Username      string            `json:"username,omitempty"`
	Extras        map[string]string `json:"extras,omitempty"`
}

type ValidationResult struct {
	Provider  Provider          `json:"provider,omitempty"`
	OK        bool              `json:"ok"`
	Identity  string            `json:"identity,omitempty"`
	Details   map[string]string `json:"details,omitempty"`
	CheckedAt time.Time         `json:"checked_at,omitempty"`
	Message   string            `json:"message,omitempty"`
}

type WebhookVerification struct {
	Provider   Provider          `json:"provider,omitempty"`
	Verified   bool              `json:"verified"`
	EventType  string            `json:"event_type,omitempty"`
	DeliveryID string            `json:"delivery_id,omitempty"`
	Message    string            `json:"message,omitempty"`
	CheckedAt  time.Time         `json:"checked_at,omitempty"`
	Details    map[string]string `json:"details,omitempty"`
}

type Event struct {
	Provider    Provider          `json:"provider,omitempty"`
	Kind        string            `json:"kind"`
	Action      string            `json:"action,omitempty"`
	SubjectType string            `json:"subject_type,omitempty"`
	SubjectID   string            `json:"subject_id,omitempty"`
	Repository  string            `json:"repository,omitempty"`
	Project     string            `json:"project,omitempty"`
	Actor       string            `json:"actor,omitempty"`
	Title       string            `json:"title,omitempty"`
	URL         string            `json:"url,omitempty"`
	ExternalID  string            `json:"external_id,omitempty"`
	OccurredAt  time.Time         `json:"occurred_at,omitempty"`
	Details     map[string]string `json:"details,omitempty"`
	Raw         json.RawMessage   `json:"raw,omitempty"`
}

type ActionRequest struct {
	Action       ActionName `json:"action"`
	Repository   string     `json:"repository,omitempty"`
	Project      string     `json:"project,omitempty"`
	IssueNumber  int        `json:"issue_number,omitempty"`
	IssueKey     string     `json:"issue_key,omitempty"`
	IssueID      string     `json:"issue_id,omitempty"`
	Channel      string     `json:"channel,omitempty"`
	ThreadTS     string     `json:"thread_ts,omitempty"`
	MessageTS    string     `json:"message_ts,omitempty"`
	TeamID       string     `json:"team_id,omitempty"`
	TransitionID string     `json:"transition_id,omitempty"`
	Title        string     `json:"title,omitempty"`
	Description  string     `json:"description,omitempty"`
	Body         string     `json:"body,omitempty"`
}

type ActionResult struct {
	Provider   Provider          `json:"provider,omitempty"`
	Action     ActionName        `json:"action,omitempty"`
	OK         bool              `json:"ok"`
	StatusCode int               `json:"status_code,omitempty"`
	ExternalID string            `json:"external_id,omitempty"`
	URL        string            `json:"url,omitempty"`
	Message    string            `json:"message,omitempty"`
	Details    map[string]string `json:"details,omitempty"`
	CheckedAt  time.Time         `json:"checked_at,omitempty"`
}

type ConnectorStatus struct {
	Provider  Provider          `json:"provider,omitempty"`
	State     string            `json:"state"`
	Ready     bool              `json:"ready"`
	Message   string            `json:"message,omitempty"`
	CheckedAt time.Time         `json:"checked_at,omitempty"`
	Details   map[string]string `json:"details,omitempty"`
}

type Connector interface {
	Provider() Provider
	ValidateConfig(cfg InstallationConfig) error
	ValidateInstallation(ctx context.Context, cfg InstallationConfig) (ValidationResult, error)
	VerifyWebhook(headers http.Header, body []byte, cfg InstallationConfig) (WebhookVerification, error)
	NormalizeEvent(headers http.Header, body []byte, cfg InstallationConfig) ([]Event, error)
	ExecuteAction(ctx context.Context, req ActionRequest, cfg InstallationConfig) (ActionResult, error)
	DeriveStatus(validation ValidationResult, webhook WebhookVerification) ConnectorStatus
}
