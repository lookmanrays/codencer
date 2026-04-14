package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	cloudconnectors "agent-bridge/internal/cloud/connectors"
)

const defaultWorkerPollLimit = 50

// Worker runs cloud-side connector maintenance jobs. In this alpha pass the
// only polling-first provider is Jira; webhook-first providers continue to rely
// on direct webhook ingestion plus explicit validation/action calls.
type Worker struct {
	store      *Store
	httpClient *http.Client
	pollLimit  int
	now        func() time.Time
}

func NewWorker(store *Store, client *http.Client, pollLimit int) *Worker {
	if pollLimit <= 0 {
		pollLimit = defaultWorkerPollLimit
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &Worker{
		store:      store,
		httpClient: client,
		pollLimit:  pollLimit,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (w *Worker) Run(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		interval = 60 * time.Second
	}
	if err := w.RunOnce(ctx); err != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := w.RunOnce(ctx); err != nil && ctx.Err() != nil {
				return ctx.Err()
			}
		}
	}
}

func (w *Worker) RunOnce(ctx context.Context) error {
	if w == nil || w.store == nil {
		return fmt.Errorf("cloud worker store is required")
	}
	installations, err := w.store.ListConnectorInstallations(ctx, "", "", "")
	if err != nil {
		return err
	}
	var errs []error
	for _, installation := range installations {
		if !installation.Enabled {
			continue
		}
		if err := w.syncInstallation(ctx, installation); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (w *Worker) syncInstallation(ctx context.Context, installation ConnectorInstallation) error {
	switch cloudconnectors.Provider(installation.ConnectorKey) {
	case cloudconnectors.ProviderJira:
		return w.syncJiraInstallation(ctx, installation)
	default:
		return nil
	}
}

func (w *Worker) syncJiraInstallation(ctx context.Context, installation ConnectorInstallation) error {
	cfg, err := decodeInstallationConfig(installation.ConfigJSON)
	if err != nil {
		return w.failInstallation(ctx, installation, fmt.Errorf("decode jira installation config: %w", err))
	}
	username := strings.TrimSpace(cfg["username"])
	if username == "" {
		return w.failInstallation(ctx, installation, fmt.Errorf("jira installation requires config.username"))
	}
	token, err := w.store.GetInstallationSecret(ctx, installation.ID, "token")
	if err != nil {
		return w.failInstallation(ctx, installation, fmt.Errorf("load jira token: %w", err))
	}
	baseJQL := strings.TrimSpace(cfg["jql"])
	if baseJQL == "" {
		projectKey := strings.TrimSpace(cfg["project_key"])
		if projectKey == "" {
			return w.failInstallation(ctx, installation, fmt.Errorf("jira installation requires config.jql or config.project_key for polling"))
		}
		baseJQL = fmt.Sprintf(`project = "%s"`, projectKey)
	}
	client := &cloudconnectors.JiraClient{
		BaseURL:    cfg["api_base_url"],
		Email:      username,
		APIToken:   string(token),
		HTTPClient: w.httpClient,
	}
	since := time.Time{}
	if installation.LastSyncAt != nil {
		since = installation.LastSyncAt.UTC()
	}
	events, err := client.PollIssueSnapshots(ctx, baseJQL, since, w.pollLimit)
	if err != nil {
		return w.failInstallation(ctx, installation, err)
	}
	receivedAt := w.now()
	for _, event := range events {
		payload, err := json.Marshal(event)
		if err != nil {
			return w.failInstallation(ctx, installation, fmt.Errorf("encode jira event payload: %w", err))
		}
		sourceEventID := event.Issue.Key
		if !event.Issue.UpdatedAt.IsZero() {
			sourceEventID = fmt.Sprintf("%s:%d", event.Issue.Key, event.Issue.UpdatedAt.UTC().UnixMilli())
		}
		_, err = w.store.CreateConnectorEvent(ctx, ConnectorEvent{
			InstallationID: installation.ID,
			SourceEventID:  sourceEventID,
			EventType:      event.Kind,
			Action:         event.Action,
			Status:         "received",
			PayloadJSON:    payload,
			OccurredAt:     nonZeroTime(event.Issue.UpdatedAt.UTC(), receivedAt),
			ReceivedAt:     receivedAt,
		})
		if err != nil && !strings.Contains(err.Error(), "UNIQUE") {
			return w.failInstallation(ctx, installation, fmt.Errorf("persist jira event: %w", err))
		}
	}
	installation.Status = "active"
	installation.LastError = ""
	installation.Enabled = true
	installation.LastSeenAt = &receivedAt
	installation.LastSyncAt = &receivedAt
	if _, err := w.store.CreateConnectorInstallation(ctx, installation); err != nil {
		return fmt.Errorf("update jira installation status: %w", err)
	}
	_, _ = w.store.CreateCloudAuditEvent(ctx, CloudAuditEvent{
		ActorType:    "cloud_worker",
		Action:       "poll_installation",
		ResourceType: "installation",
		ResourceID:   installation.ID,
		OrgID:        installation.OrgID,
		WorkspaceID:  installation.WorkspaceID,
		ProjectID:    installation.ProjectID,
		Outcome:      "ok",
		DetailsJSON:  mustJSON(map[string]any{"provider": installation.ConnectorKey, "event_count": len(events)}),
		CreatedAt:    receivedAt,
	})
	return nil
}

func (w *Worker) failInstallation(ctx context.Context, installation ConnectorInstallation, err error) error {
	if err == nil {
		return nil
	}
	now := w.now()
	installation.Status = "error"
	installation.LastError = err.Error()
	installation.LastSeenAt = &now
	if _, updateErr := w.store.CreateConnectorInstallation(ctx, installation); updateErr != nil {
		return errors.Join(err, fmt.Errorf("update installation error state: %w", updateErr))
	}
	_, _ = w.store.CreateCloudAuditEvent(ctx, CloudAuditEvent{
		ActorType:    "cloud_worker",
		Action:       "poll_installation",
		ResourceType: "installation",
		ResourceID:   installation.ID,
		OrgID:        installation.OrgID,
		WorkspaceID:  installation.WorkspaceID,
		ProjectID:    installation.ProjectID,
		Outcome:      "error",
		DetailsJSON:  mustJSON(map[string]any{"provider": installation.ConnectorKey, "error": err.Error()}),
		CreatedAt:    now,
	})
	return err
}

func decodeInstallationConfig(raw json.RawMessage) (map[string]string, error) {
	if len(raw) == 0 {
		return map[string]string{}, nil
	}
	var cfg map[string]string
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
