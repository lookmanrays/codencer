package cloud

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

const (
	DefaultOrgStatus          = "active"
	DefaultWorkspaceStatus    = "active"
	DefaultProjectStatus      = "active"
	DefaultMembershipStatus   = "active"
	DefaultAPITokenKind       = "api"
	DefaultInstallationStatus = "active"
	DefaultInstallationHealth = "unknown"
	DefaultRuntimeStatus      = "active"
	DefaultRuntimeHealth      = "unknown"
)

const (
	RoleOrgOwner        = "org_owner"
	RoleOrgAdmin        = "org_admin"
	RoleWorkspaceAdmin  = "workspace_admin"
	RoleProjectOperator = "project_operator"
	RoleProjectViewer   = "project_viewer"
)

// Org is the top-level tenant boundary for the cloud control plane.
type Org struct {
	ID        string    `json:"id"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Workspace scopes work under an org.
type Workspace struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"org_id"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Project scopes runs, integrations, and tokens under a workspace.
type Project struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"org_id"`
	WorkspaceID string    `json:"workspace_id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Membership is the minimal first-class operator/service membership record used
// to attribute cloud control-plane actions and constrain token issuance.
type Membership struct {
	ID          string     `json:"id"`
	OrgID       string     `json:"org_id"`
	WorkspaceID string     `json:"workspace_id,omitempty"`
	ProjectID   string     `json:"project_id,omitempty"`
	Name        string     `json:"name"`
	Email       string     `json:"email,omitempty"`
	Role        string     `json:"role"`
	Status      string     `json:"status"`
	DisabledAt  *time.Time `json:"disabled_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// APIToken is a hashed bearer token record for cloud auth.
type APIToken struct {
	ID                    string     `json:"id"`
	OrgID                 string     `json:"org_id"`
	WorkspaceID           string     `json:"workspace_id,omitempty"`
	ProjectID             string     `json:"project_id,omitempty"`
	MembershipID          string     `json:"membership_id,omitempty"`
	Role                  string     `json:"role,omitempty"`
	Name                  string     `json:"name"`
	Kind                  string     `json:"kind"`
	SubjectType           string     `json:"subject_type,omitempty"`
	SubjectName           string     `json:"subject_name,omitempty"`
	TokenHash             string     `json:"token_hash"`
	TokenPrefix           string     `json:"token_prefix,omitempty"`
	Scopes                []string   `json:"scopes,omitempty"`
	Disabled              bool       `json:"disabled"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
	LastUsedAt            *time.Time `json:"last_used_at,omitempty"`
	RevokedAt             *time.Time `json:"revoked_at,omitempty"`
	MembershipWorkspaceID string     `json:"-"`
	MembershipProjectID   string     `json:"-"`
}

// ConnectorInstallation tracks an installation of a provider connector.
type ConnectorInstallation struct {
	ID                     string          `json:"id"`
	OrgID                  string          `json:"org_id"`
	WorkspaceID            string          `json:"workspace_id,omitempty"`
	ProjectID              string          `json:"project_id,omitempty"`
	OwnerMembershipID      string          `json:"owner_membership_id,omitempty"`
	ConnectorKey           string          `json:"connector_key"`
	ExternalInstallationID string          `json:"external_installation_id,omitempty"`
	ExternalAccount        string          `json:"external_account,omitempty"`
	Name                   string          `json:"name,omitempty"`
	Status                 string          `json:"status"`
	Enabled                bool            `json:"enabled"`
	Health                 string          `json:"health"`
	ConfigJSON             json.RawMessage `json:"config_json,omitempty"`
	MetadataJSON           json.RawMessage `json:"metadata_json,omitempty"`
	LastSeenAt             *time.Time      `json:"last_seen_at,omitempty"`
	LastSyncAt             *time.Time      `json:"last_sync_at,omitempty"`
	LastValidatedAt        *time.Time      `json:"last_validated_at,omitempty"`
	LastWebhookAt          *time.Time      `json:"last_webhook_at,omitempty"`
	LastActionAt           *time.Time      `json:"last_action_at,omitempty"`
	LastError              string          `json:"last_error,omitempty"`
	CreatedAt              time.Time       `json:"created_at"`
	UpdatedAt              time.Time       `json:"updated_at"`
}

// RuntimeConnectorInstallation tracks a Codencer runtime connector/node inside a tenant scope.
type RuntimeConnectorInstallation struct {
	ID                string          `json:"id"`
	OrgID             string          `json:"org_id"`
	WorkspaceID       string          `json:"workspace_id,omitempty"`
	ProjectID         string          `json:"project_id,omitempty"`
	OwnerMembershipID string          `json:"owner_membership_id,omitempty"`
	ConnectorID       string          `json:"connector_id"`
	MachineID         string          `json:"machine_id"`
	Label             string          `json:"label,omitempty"`
	PublicKey         string          `json:"public_key,omitempty"`
	Status            string          `json:"status"`
	Enabled           bool            `json:"enabled"`
	Health            string          `json:"health"`
	MetadataJSON      json.RawMessage `json:"metadata_json,omitempty"`
	LastSeenAt        *time.Time      `json:"last_seen_at,omitempty"`
	LastError         string          `json:"last_error,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

// RuntimeInstance tracks a shared Codencer runtime instance owned by a runtime connector installation.
type RuntimeInstance struct {
	ID                             string          `json:"instance_id"`
	OrgID                          string          `json:"org_id"`
	WorkspaceID                    string          `json:"workspace_id,omitempty"`
	ProjectID                      string          `json:"project_id,omitempty"`
	RuntimeConnectorInstallationID string          `json:"runtime_connector_installation_id"`
	RepoRoot                       string          `json:"repo_root"`
	InstanceJSON                   json.RawMessage `json:"instance_json,omitempty"`
	Status                         string          `json:"status"`
	Enabled                        bool            `json:"enabled"`
	Health                         string          `json:"health"`
	Shared                         bool            `json:"shared"`
	LastSeenAt                     *time.Time      `json:"last_seen_at,omitempty"`
	LastError                      string          `json:"last_error,omitempty"`
	CreatedAt                      time.Time       `json:"created_at"`
	UpdatedAt                      time.Time       `json:"updated_at"`
}

// InstallationSecret stores encrypted provider secrets for an installation.
type InstallationSecret struct {
	ID             string    `json:"id"`
	InstallationID string    `json:"installation_id"`
	SecretName     string    `json:"secret_name"`
	Ciphertext     string    `json:"ciphertext"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ConnectorEvent is a normalized ingest event record.
type ConnectorEvent struct {
	ID             string          `json:"id"`
	InstallationID string          `json:"installation_id"`
	SourceEventID  string          `json:"source_event_id,omitempty"`
	EventType      string          `json:"event_type"`
	Action         string          `json:"action,omitempty"`
	Status         string          `json:"status"`
	PayloadJSON    json.RawMessage `json:"payload_json,omitempty"`
	MetadataJSON   json.RawMessage `json:"metadata_json,omitempty"`
	OccurredAt     time.Time       `json:"occurred_at"`
	ReceivedAt     time.Time       `json:"received_at"`
	ProcessedAt    *time.Time      `json:"processed_at,omitempty"`
	ErrorMessage   string          `json:"error_message,omitempty"`
}

// ConnectorActionLog records normalized action dispatches against a connector.
type ConnectorActionLog struct {
	ID             string          `json:"id"`
	InstallationID string          `json:"installation_id"`
	ActionName     string          `json:"action_name"`
	Status         string          `json:"status"`
	RequestJSON    json.RawMessage `json:"request_json,omitempty"`
	ResponseJSON   json.RawMessage `json:"response_json,omitempty"`
	ErrorMessage   string          `json:"error_message,omitempty"`
	StartedAt      time.Time       `json:"started_at"`
	CompletedAt    *time.Time      `json:"completed_at,omitempty"`
}

// CloudAuditEvent is the append-only audit trail for operator and API actions.
type CloudAuditEvent struct {
	ID           string          `json:"id"`
	ActorType    string          `json:"actor_type"`
	ActorID      string          `json:"actor_id,omitempty"`
	Action       string          `json:"action"`
	ResourceType string          `json:"resource_type,omitempty"`
	ResourceID   string          `json:"resource_id,omitempty"`
	OrgID        string          `json:"org_id,omitempty"`
	WorkspaceID  string          `json:"workspace_id,omitempty"`
	ProjectID    string          `json:"project_id,omitempty"`
	Outcome      string          `json:"outcome"`
	DetailsJSON  json.RawMessage `json:"details_json,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
}

func newID(prefix string) (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("generate %s id: %w", prefix, err)
	}
	return prefix + "_" + hex.EncodeToString(buf[:]), nil
}
