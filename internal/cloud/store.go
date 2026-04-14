package cloud

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db        *sql.DB
	secretBox *SecretBox
}

type migration struct {
	version int
	sql     string
}

var migrations = []migration{
	{
		version: 1,
		sql: `
CREATE TABLE IF NOT EXISTS orgs (
	id TEXT PRIMARY KEY,
	slug TEXT NOT NULL UNIQUE,
	name TEXT NOT NULL,
	status TEXT NOT NULL,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS workspaces (
	id TEXT PRIMARY KEY,
	org_id TEXT NOT NULL,
	slug TEXT NOT NULL,
	name TEXT NOT NULL,
	status TEXT NOT NULL,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	UNIQUE(org_id, slug),
	FOREIGN KEY (org_id) REFERENCES orgs(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS projects (
	id TEXT PRIMARY KEY,
	org_id TEXT NOT NULL,
	workspace_id TEXT NOT NULL,
	slug TEXT NOT NULL,
	name TEXT NOT NULL,
	status TEXT NOT NULL,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	UNIQUE(workspace_id, slug),
	FOREIGN KEY (org_id) REFERENCES orgs(id) ON DELETE CASCADE,
	FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS api_tokens (
	id TEXT PRIMARY KEY,
	org_id TEXT NOT NULL,
	workspace_id TEXT,
	project_id TEXT,
	name TEXT NOT NULL,
	kind TEXT NOT NULL,
	token_hash TEXT NOT NULL UNIQUE,
	token_prefix TEXT NOT NULL,
	scopes_json TEXT NOT NULL,
	disabled INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	last_used_at DATETIME,
	revoked_at DATETIME,
	FOREIGN KEY (org_id) REFERENCES orgs(id) ON DELETE CASCADE,
	FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE,
	FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS connector_installations (
	id TEXT PRIMARY KEY,
	org_id TEXT NOT NULL,
	workspace_id TEXT,
	project_id TEXT,
	connector_key TEXT NOT NULL,
	external_installation_id TEXT,
	external_account TEXT,
	name TEXT,
	status TEXT NOT NULL,
	enabled INTEGER NOT NULL DEFAULT 1,
	config_json TEXT,
	metadata_json TEXT,
	last_seen_at DATETIME,
	last_sync_at DATETIME,
	last_error TEXT,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	FOREIGN KEY (org_id) REFERENCES orgs(id) ON DELETE CASCADE,
	FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE,
	FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS installation_secrets (
	id TEXT PRIMARY KEY,
	installation_id TEXT NOT NULL,
	secret_name TEXT NOT NULL,
	ciphertext TEXT NOT NULL,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	UNIQUE(installation_id, secret_name),
	FOREIGN KEY (installation_id) REFERENCES connector_installations(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS connector_events (
	id TEXT PRIMARY KEY,
	installation_id TEXT NOT NULL,
	source_event_id TEXT,
	event_type TEXT NOT NULL,
	action TEXT,
	status TEXT NOT NULL,
	payload_json TEXT,
	metadata_json TEXT,
	occurred_at DATETIME NOT NULL,
	received_at DATETIME NOT NULL,
	processed_at DATETIME,
	error_message TEXT,
	UNIQUE(installation_id, source_event_id),
	FOREIGN KEY (installation_id) REFERENCES connector_installations(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS connector_action_logs (
	id TEXT PRIMARY KEY,
	installation_id TEXT NOT NULL,
	action_name TEXT NOT NULL,
	status TEXT NOT NULL,
	request_json TEXT,
	response_json TEXT,
	error_message TEXT,
	started_at DATETIME NOT NULL,
	completed_at DATETIME,
	FOREIGN KEY (installation_id) REFERENCES connector_installations(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS cloud_audit_events (
	id TEXT PRIMARY KEY,
	actor_type TEXT NOT NULL,
	actor_id TEXT,
	action TEXT NOT NULL,
	resource_type TEXT,
	resource_id TEXT,
	org_id TEXT,
	workspace_id TEXT,
	project_id TEXT,
	outcome TEXT NOT NULL,
	details_json TEXT,
	created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS cloud_schema_migrations (
	version INTEGER PRIMARY KEY,
	applied_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_api_tokens_org_id ON api_tokens(org_id);
CREATE INDEX IF NOT EXISTS idx_connector_installations_org_id ON connector_installations(org_id);
CREATE INDEX IF NOT EXISTS idx_connector_installations_workspace_id ON connector_installations(workspace_id);
CREATE INDEX IF NOT EXISTS idx_connector_installations_project_id ON connector_installations(project_id);
CREATE INDEX IF NOT EXISTS idx_connector_events_installation_id ON connector_events(installation_id);
CREATE INDEX IF NOT EXISTS idx_connector_action_logs_installation_id ON connector_action_logs(installation_id);
CREATE INDEX IF NOT EXISTS idx_audit_events_org_id ON cloud_audit_events(org_id);
`,
	},
}

// OpenStore opens or creates the cloud SQLite store and applies code-defined migrations.
func OpenStore(path string, masterKey string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("cloud database path is required")
	}
	if dir := filepath.Dir(path); dir != "." && dir != string(filepath.Separator) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create cloud db directory: %w", err)
		}
	}
	db, err := sql.Open("sqlite3", sqliteDSN(path))
	if err != nil {
		return nil, fmt.Errorf("open cloud database: %w", err)
	}
	store := &Store{db: db}
	if strings.TrimSpace(masterKey) != "" {
		box, err := NewSecretBox(masterKey)
		if err != nil {
			_ = db.Close()
			return nil, err
		}
		store.secretBox = box
	}
	if err := store.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

// Close releases the underlying database handle.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func sqliteDSN(path string) string {
	if strings.Contains(path, "?") {
		return path + "&_busy_timeout=5000&_foreign_keys=on"
	}
	return path + "?_busy_timeout=5000&_foreign_keys=on&_journal=WAL"
}

func (s *Store) migrate(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("cloud store is not open")
	}
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS cloud_schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at DATETIME NOT NULL
	)`); err != nil {
		return fmt.Errorf("create cloud migration table: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin cloud migration transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	applied := map[int]struct{}{}
	rows, err := tx.QueryContext(ctx, `SELECT version FROM cloud_schema_migrations`)
	if err != nil {
		return fmt.Errorf("read cloud migration state: %w", err)
	}
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan cloud migration state: %w", err)
		}
		applied[version] = struct{}{}
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close cloud migration rows: %w", err)
	}

	for _, mig := range migrations {
		if _, ok := applied[mig.version]; ok {
			continue
		}
		if _, err := tx.ExecContext(ctx, mig.sql); err != nil {
			return fmt.Errorf("apply cloud migration %d: %w", mig.version, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO cloud_schema_migrations(version, applied_at) VALUES (?, ?)`, mig.version, time.Now().UTC()); err != nil {
			return fmt.Errorf("record cloud migration %d: %w", mig.version, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit cloud migrations: %w", err)
	}
	return nil
}

// CreateOrg inserts a new org row.
func (s *Store) CreateOrg(ctx context.Context, org Org) (*Org, error) {
	if strings.TrimSpace(org.Slug) == "" {
		return nil, fmt.Errorf("org slug is required")
	}
	if strings.TrimSpace(org.Name) == "" {
		return nil, fmt.Errorf("org name is required")
	}
	if org.ID == "" {
		id, err := newID("org")
		if err != nil {
			return nil, err
		}
		org.ID = id
	}
	if org.Status == "" {
		org.Status = DefaultOrgStatus
	}
	now := time.Now().UTC()
	if org.CreatedAt.IsZero() {
		org.CreatedAt = now
	}
	org.UpdatedAt = now
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO orgs (id, slug, name, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, org.ID, org.Slug, org.Name, org.Status, org.CreatedAt, org.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert org: %w", err)
	}
	return &org, nil
}

// GetOrg loads an org by ID.
func (s *Store) GetOrg(ctx context.Context, id string) (*Org, error) {
	var org Org
	if err := s.db.QueryRowContext(ctx, `
		SELECT id, slug, name, status, created_at, updated_at
		FROM orgs
		WHERE id = ?
	`, id).Scan(&org.ID, &org.Slug, &org.Name, &org.Status, &org.CreatedAt, &org.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound("org", id)
		}
		return nil, fmt.Errorf("get org: %w", err)
	}
	return &org, nil
}

// ListOrgs returns all orgs ordered by creation time.
func (s *Store) ListOrgs(ctx context.Context) ([]Org, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, slug, name, status, created_at, updated_at
		FROM orgs
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list orgs: %w", err)
	}
	defer rows.Close()
	var out []Org
	for rows.Next() {
		var org Org
		if err := rows.Scan(&org.ID, &org.Slug, &org.Name, &org.Status, &org.CreatedAt, &org.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan org: %w", err)
		}
		out = append(out, org)
	}
	return out, rows.Err()
}

// CreateWorkspace inserts a new workspace row.
func (s *Store) CreateWorkspace(ctx context.Context, workspace Workspace) (*Workspace, error) {
	if strings.TrimSpace(workspace.OrgID) == "" {
		return nil, fmt.Errorf("workspace org_id is required")
	}
	if strings.TrimSpace(workspace.Slug) == "" {
		return nil, fmt.Errorf("workspace slug is required")
	}
	if strings.TrimSpace(workspace.Name) == "" {
		return nil, fmt.Errorf("workspace name is required")
	}
	if workspace.ID == "" {
		id, err := newID("ws")
		if err != nil {
			return nil, err
		}
		workspace.ID = id
	}
	if workspace.Status == "" {
		workspace.Status = DefaultWorkspaceStatus
	}
	now := time.Now().UTC()
	if workspace.CreatedAt.IsZero() {
		workspace.CreatedAt = now
	}
	workspace.UpdatedAt = now
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO workspaces (id, org_id, slug, name, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, workspace.ID, workspace.OrgID, workspace.Slug, workspace.Name, workspace.Status, workspace.CreatedAt, workspace.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert workspace: %w", err)
	}
	return &workspace, nil
}

// GetWorkspace loads a workspace by ID.
func (s *Store) GetWorkspace(ctx context.Context, id string) (*Workspace, error) {
	var workspace Workspace
	if err := s.db.QueryRowContext(ctx, `
		SELECT id, org_id, slug, name, status, created_at, updated_at
		FROM workspaces
		WHERE id = ?
	`, id).Scan(&workspace.ID, &workspace.OrgID, &workspace.Slug, &workspace.Name, &workspace.Status, &workspace.CreatedAt, &workspace.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound("workspace", id)
		}
		return nil, fmt.Errorf("get workspace: %w", err)
	}
	return &workspace, nil
}

// ListWorkspaces returns workspaces optionally filtered by org.
func (s *Store) ListWorkspaces(ctx context.Context, orgID string) ([]Workspace, error) {
	query := `
		SELECT id, org_id, slug, name, status, created_at, updated_at
		FROM workspaces
	`
	args := []any{}
	if strings.TrimSpace(orgID) != "" {
		query += ` WHERE org_id = ?`
		args = append(args, orgID)
	}
	query += ` ORDER BY created_at ASC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}
	defer rows.Close()
	var out []Workspace
	for rows.Next() {
		var workspace Workspace
		if err := rows.Scan(&workspace.ID, &workspace.OrgID, &workspace.Slug, &workspace.Name, &workspace.Status, &workspace.CreatedAt, &workspace.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan workspace: %w", err)
		}
		out = append(out, workspace)
	}
	return out, rows.Err()
}

// CreateProject inserts a new project row.
func (s *Store) CreateProject(ctx context.Context, project Project) (*Project, error) {
	if strings.TrimSpace(project.OrgID) == "" {
		return nil, fmt.Errorf("project org_id is required")
	}
	if strings.TrimSpace(project.WorkspaceID) == "" {
		return nil, fmt.Errorf("project workspace_id is required")
	}
	if strings.TrimSpace(project.Slug) == "" {
		return nil, fmt.Errorf("project slug is required")
	}
	if strings.TrimSpace(project.Name) == "" {
		return nil, fmt.Errorf("project name is required")
	}
	if project.ID == "" {
		id, err := newID("proj")
		if err != nil {
			return nil, err
		}
		project.ID = id
	}
	if project.Status == "" {
		project.Status = DefaultProjectStatus
	}
	now := time.Now().UTC()
	if project.CreatedAt.IsZero() {
		project.CreatedAt = now
	}
	project.UpdatedAt = now
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO projects (id, org_id, workspace_id, slug, name, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, project.ID, project.OrgID, project.WorkspaceID, project.Slug, project.Name, project.Status, project.CreatedAt, project.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert project: %w", err)
	}
	return &project, nil
}

// GetProject loads a project by ID.
func (s *Store) GetProject(ctx context.Context, id string) (*Project, error) {
	var project Project
	if err := s.db.QueryRowContext(ctx, `
		SELECT id, org_id, workspace_id, slug, name, status, created_at, updated_at
		FROM projects
		WHERE id = ?
	`, id).Scan(&project.ID, &project.OrgID, &project.WorkspaceID, &project.Slug, &project.Name, &project.Status, &project.CreatedAt, &project.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound("project", id)
		}
		return nil, fmt.Errorf("get project: %w", err)
	}
	return &project, nil
}

// ListProjects returns projects optionally filtered by workspace.
func (s *Store) ListProjects(ctx context.Context, workspaceID string) ([]Project, error) {
	query := `
		SELECT id, org_id, workspace_id, slug, name, status, created_at, updated_at
		FROM projects
	`
	args := []any{}
	if strings.TrimSpace(workspaceID) != "" {
		query += ` WHERE workspace_id = ?`
		args = append(args, workspaceID)
	}
	query += ` ORDER BY created_at ASC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		var project Project
		if err := rows.Scan(&project.ID, &project.OrgID, &project.WorkspaceID, &project.Slug, &project.Name, &project.Status, &project.CreatedAt, &project.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		out = append(out, project)
	}
	return out, rows.Err()
}

// CreateAPIToken stores a hashed API token record from a raw bearer token.
func (s *Store) CreateAPIToken(ctx context.Context, token APIToken, rawToken string) (*APIToken, error) {
	if strings.TrimSpace(token.OrgID) == "" {
		return nil, fmt.Errorf("api token org_id is required")
	}
	if strings.TrimSpace(token.Name) == "" {
		return nil, fmt.Errorf("api token name is required")
	}
	if strings.TrimSpace(rawToken) == "" {
		return nil, ErrAPITokenInvalid
	}
	if token.ID == "" {
		id, err := newID("tok")
		if err != nil {
			return nil, err
		}
		token.ID = id
	}
	if token.Kind == "" {
		token.Kind = DefaultAPITokenKind
	}
	now := time.Now().UTC()
	if token.CreatedAt.IsZero() {
		token.CreatedAt = now
	}
	token.UpdatedAt = now
	token.TokenHash = HashAPIToken(rawToken)
	token.TokenPrefix = TokenPrefix(rawToken)
	scopesJSON, err := json.Marshal(token.Scopes)
	if err != nil {
		return nil, fmt.Errorf("marshal api token scopes: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO api_tokens (
			id, org_id, workspace_id, project_id, name, kind, token_hash, token_prefix,
			scopes_json, disabled, created_at, updated_at, last_used_at, revoked_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, token.ID, token.OrgID, nullString(token.WorkspaceID), nullString(token.ProjectID), token.Name, token.Kind, token.TokenHash, token.TokenPrefix, string(scopesJSON), boolToInt(token.Disabled), token.CreatedAt, token.UpdatedAt, nullTime(token.LastUsedAt), nullTime(token.RevokedAt))
	if err != nil {
		return nil, fmt.Errorf("insert api token: %w", err)
	}
	return &token, nil
}

// LookupAPIToken resolves a raw bearer token back to its stored record.
func (s *Store) LookupAPIToken(ctx context.Context, rawToken string) (*APIToken, error) {
	if strings.TrimSpace(rawToken) == "" {
		return nil, ErrAPITokenInvalid
	}
	hash := HashAPIToken(rawToken)
	var token APIToken
	var disabled int
	var workspaceID sql.NullString
	var projectID sql.NullString
	var scopesJSON sql.NullString
	var lastUsed sql.NullTime
	var revoked sql.NullTime
	if err := s.db.QueryRowContext(ctx, `
		SELECT id, org_id, workspace_id, project_id, name, kind, token_hash, token_prefix, scopes_json, disabled, created_at, updated_at, last_used_at, revoked_at
		FROM api_tokens
		WHERE token_hash = ?
	`, hash).Scan(&token.ID, &token.OrgID, &workspaceID, &projectID, &token.Name, &token.Kind, &token.TokenHash, &token.TokenPrefix, &scopesJSON, &disabled, &token.CreatedAt, &token.UpdatedAt, &lastUsed, &revoked); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAPITokenNotFound
		}
		return nil, fmt.Errorf("lookup api token: %w", err)
	}
	if err := json.Unmarshal([]byte(scopesJSON.String), &token.Scopes); err != nil && scopesJSON.Valid {
		return nil, fmt.Errorf("decode api token scopes: %w", err)
	}
	token.Disabled = disabled != 0
	token.WorkspaceID = workspaceID.String
	token.ProjectID = projectID.String
	token.LastUsedAt = scanTime(lastUsed)
	token.RevokedAt = scanTime(revoked)
	return &token, nil
}

// TouchAPITokenUsage updates the last-used timestamp for auditability.
func (s *Store) TouchAPITokenUsage(ctx context.Context, tokenID string) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		UPDATE api_tokens
		SET last_used_at = ?, updated_at = ?
		WHERE id = ?
	`, now, now, tokenID)
	if err != nil {
		return fmt.Errorf("touch api token usage: %w", err)
	}
	return nil
}

// ListAPITokens returns token metadata for the requested scope.
func (s *Store) ListAPITokens(ctx context.Context, orgID, workspaceID, projectID string) ([]APIToken, error) {
	query := `
		SELECT id, org_id, workspace_id, project_id, name, kind, token_hash, token_prefix, scopes_json, disabled, created_at, updated_at, last_used_at, revoked_at
		FROM api_tokens
	`
	args := []any{}
	clauses := make([]string, 0, 3)
	if strings.TrimSpace(orgID) != "" {
		clauses = append(clauses, "org_id = ?")
		args = append(args, orgID)
	}
	if strings.TrimSpace(workspaceID) != "" {
		clauses = append(clauses, "workspace_id = ?")
		args = append(args, workspaceID)
	}
	if strings.TrimSpace(projectID) != "" {
		clauses = append(clauses, "project_id = ?")
		args = append(args, projectID)
	}
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += ` ORDER BY created_at ASC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list api tokens: %w", err)
	}
	defer rows.Close()
	var out []APIToken
	for rows.Next() {
		var token APIToken
		var disabled int
		var workspaceID sql.NullString
		var projectID sql.NullString
		var scopesJSON sql.NullString
		var lastUsed sql.NullTime
		var revoked sql.NullTime
		if err := rows.Scan(&token.ID, &token.OrgID, &workspaceID, &projectID, &token.Name, &token.Kind, &token.TokenHash, &token.TokenPrefix, &scopesJSON, &disabled, &token.CreatedAt, &token.UpdatedAt, &lastUsed, &revoked); err != nil {
			return nil, fmt.Errorf("scan api token: %w", err)
		}
		if scopesJSON.Valid {
			if err := json.Unmarshal([]byte(scopesJSON.String), &token.Scopes); err != nil {
				return nil, fmt.Errorf("decode api token scopes: %w", err)
			}
		}
		token.Disabled = disabled != 0
		token.WorkspaceID = workspaceID.String
		token.ProjectID = projectID.String
		token.LastUsedAt = scanTime(lastUsed)
		token.RevokedAt = scanTime(revoked)
		out = append(out, token)
	}
	return out, rows.Err()
}

// CreateConnectorInstallation stores or updates a connector installation record.
func (s *Store) CreateConnectorInstallation(ctx context.Context, installation ConnectorInstallation) (*ConnectorInstallation, error) {
	if strings.TrimSpace(installation.OrgID) == "" {
		return nil, fmt.Errorf("connector installation org_id is required")
	}
	if strings.TrimSpace(installation.ConnectorKey) == "" {
		return nil, fmt.Errorf("connector installation connector_key is required")
	}
	if installation.ID == "" {
		id, err := newID("inst")
		if err != nil {
			return nil, err
		}
		installation.ID = id
	}
	if installation.Status == "" {
		installation.Status = DefaultInstallationStatus
	}
	if !installation.Enabled && installation.Status == DefaultInstallationStatus {
		installation.Enabled = true
	}
	now := time.Now().UTC()
	if installation.CreatedAt.IsZero() {
		installation.CreatedAt = now
	}
	installation.UpdatedAt = now
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO connector_installations (
			id, org_id, workspace_id, project_id, connector_key, external_installation_id,
			external_account, name, status, enabled, config_json, metadata_json, last_seen_at,
			last_sync_at, last_error, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			org_id = excluded.org_id,
			workspace_id = excluded.workspace_id,
			project_id = excluded.project_id,
			connector_key = excluded.connector_key,
			external_installation_id = excluded.external_installation_id,
			external_account = excluded.external_account,
			name = excluded.name,
			status = excluded.status,
			enabled = excluded.enabled,
			config_json = excluded.config_json,
			metadata_json = excluded.metadata_json,
			last_seen_at = excluded.last_seen_at,
			last_sync_at = excluded.last_sync_at,
			last_error = excluded.last_error,
			updated_at = excluded.updated_at
	`, installation.ID, installation.OrgID, nullString(installation.WorkspaceID), nullString(installation.ProjectID), installation.ConnectorKey, nullString(installation.ExternalInstallationID), nullString(installation.ExternalAccount), nullString(installation.Name), installation.Status, boolToInt(installation.Enabled), rawMessage(installation.ConfigJSON), rawMessage(installation.MetadataJSON), nullTime(installation.LastSeenAt), nullTime(installation.LastSyncAt), nullString(installation.LastError), installation.CreatedAt, installation.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert connector installation: %w", err)
	}
	return &installation, nil
}

// GetConnectorInstallation loads an installation by ID.
func (s *Store) GetConnectorInstallation(ctx context.Context, id string) (*ConnectorInstallation, error) {
	var installation ConnectorInstallation
	var enabled int
	var workspaceID sql.NullString
	var projectID sql.NullString
	var externalInstallationID sql.NullString
	var externalAccount sql.NullString
	var name sql.NullString
	var configJSON sql.NullString
	var metadataJSON sql.NullString
	var lastSeen sql.NullTime
	var lastSync sql.NullTime
	var lastError sql.NullString
	if err := s.db.QueryRowContext(ctx, `
		SELECT id, org_id, workspace_id, project_id, connector_key, external_installation_id,
		       external_account, name, status, enabled, config_json, metadata_json,
		       last_seen_at, last_sync_at, last_error, created_at, updated_at
		FROM connector_installations
		WHERE id = ?
	`, id).Scan(&installation.ID, &installation.OrgID, &workspaceID, &projectID, &installation.ConnectorKey, &externalInstallationID, &externalAccount, &name, &installation.Status, &enabled, &configJSON, &metadataJSON, &lastSeen, &lastSync, &lastError, &installation.CreatedAt, &installation.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound("connector installation", id)
		}
		return nil, fmt.Errorf("get connector installation: %w", err)
	}
	installation.Enabled = enabled != 0
	installation.WorkspaceID = workspaceID.String
	installation.ProjectID = projectID.String
	installation.ExternalInstallationID = externalInstallationID.String
	installation.ExternalAccount = externalAccount.String
	installation.Name = name.String
	installation.ConfigJSON = rawMessageFromNull(configJSON)
	installation.MetadataJSON = rawMessageFromNull(metadataJSON)
	installation.LastSeenAt = scanTime(lastSeen)
	installation.LastSyncAt = scanTime(lastSync)
	installation.LastError = lastError.String
	return &installation, nil
}

// DeleteConnectorInstallation removes an installation and its dependent records.
func (s *Store) DeleteConnectorInstallation(ctx context.Context, installationID string) error {
	if strings.TrimSpace(installationID) == "" {
		return fmt.Errorf("installation id is required")
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM connector_installations WHERE id = ?`, installationID)
	if err != nil {
		return fmt.Errorf("delete connector installation: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete connector installation rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound("connector installation", installationID)
	}
	return nil
}

// ListConnectorInstallations returns installations optionally filtered by org/workspace/project.
func (s *Store) ListConnectorInstallations(ctx context.Context, orgID, workspaceID, projectID string) ([]ConnectorInstallation, error) {
	query := `
		SELECT id, org_id, workspace_id, project_id, connector_key, external_installation_id,
		       external_account, name, status, enabled, config_json, metadata_json,
		       last_seen_at, last_sync_at, last_error, created_at, updated_at
		FROM connector_installations
	`
	args := []any{}
	clauses := make([]string, 0, 3)
	if strings.TrimSpace(orgID) != "" {
		clauses = append(clauses, "org_id = ?")
		args = append(args, orgID)
	}
	if strings.TrimSpace(workspaceID) != "" {
		clauses = append(clauses, "workspace_id = ?")
		args = append(args, workspaceID)
	}
	if strings.TrimSpace(projectID) != "" {
		clauses = append(clauses, "project_id = ?")
		args = append(args, projectID)
	}
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += ` ORDER BY created_at ASC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list connector installations: %w", err)
	}
	defer rows.Close()
	var out []ConnectorInstallation
	for rows.Next() {
		var installation ConnectorInstallation
		var enabled int
		var workspaceID sql.NullString
		var projectID sql.NullString
		var externalInstallationID sql.NullString
		var externalAccount sql.NullString
		var name sql.NullString
		var configJSON sql.NullString
		var metadataJSON sql.NullString
		var lastSeen sql.NullTime
		var lastSync sql.NullTime
		var lastError sql.NullString
		if err := rows.Scan(&installation.ID, &installation.OrgID, &workspaceID, &projectID, &installation.ConnectorKey, &externalInstallationID, &externalAccount, &name, &installation.Status, &enabled, &configJSON, &metadataJSON, &lastSeen, &lastSync, &lastError, &installation.CreatedAt, &installation.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan connector installation: %w", err)
		}
		installation.Enabled = enabled != 0
		installation.WorkspaceID = workspaceID.String
		installation.ProjectID = projectID.String
		installation.ExternalInstallationID = externalInstallationID.String
		installation.ExternalAccount = externalAccount.String
		installation.Name = name.String
		installation.ConfigJSON = rawMessageFromNull(configJSON)
		installation.MetadataJSON = rawMessageFromNull(metadataJSON)
		installation.LastSeenAt = scanTime(lastSeen)
		installation.LastSyncAt = scanTime(lastSync)
		installation.LastError = lastError.String
		out = append(out, installation)
	}
	return out, rows.Err()
}

// PutInstallationSecret encrypts and persists a secret for an installation.
func (s *Store) PutInstallationSecret(ctx context.Context, installationID, secretName string, plaintext []byte) (*InstallationSecret, error) {
	if strings.TrimSpace(installationID) == "" {
		return nil, fmt.Errorf("installation_id is required")
	}
	if strings.TrimSpace(secretName) == "" {
		return nil, fmt.Errorf("secret_name is required")
	}
	if s.secretBox == nil {
		return nil, ErrSecretBoxRequired
	}
	ciphertext, err := s.secretBox.Encrypt(plaintext)
	if err != nil {
		return nil, err
	}
	record := InstallationSecret{
		ID:             "",
		InstallationID: installationID,
		SecretName:     secretName,
		Ciphertext:     ciphertext,
	}
	id, err := newID("sec")
	if err != nil {
		return nil, err
	}
	record.ID = id
	now := time.Now().UTC()
	record.CreatedAt = now
	record.UpdatedAt = now
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO installation_secrets (id, installation_id, secret_name, ciphertext, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(installation_id, secret_name) DO UPDATE SET
			ciphertext = excluded.ciphertext,
			updated_at = excluded.updated_at
	`, record.ID, record.InstallationID, record.SecretName, record.Ciphertext, record.CreatedAt, record.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("store installation secret: %w", err)
	}
	return &record, nil
}

// GetInstallationSecret fetches and decrypts an installation secret.
func (s *Store) GetInstallationSecret(ctx context.Context, installationID, secretName string) ([]byte, error) {
	if s.secretBox == nil {
		return nil, ErrSecretBoxRequired
	}
	var ciphertext string
	if err := s.db.QueryRowContext(ctx, `
		SELECT ciphertext
		FROM installation_secrets
		WHERE installation_id = ? AND secret_name = ?
	`, installationID, secretName).Scan(&ciphertext); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound("installation secret", installationID+"/"+secretName)
		}
		return nil, fmt.Errorf("get installation secret: %w", err)
	}
	plaintext, err := s.secretBox.Decrypt(ciphertext)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}

// CreateConnectorEvent records a normalized connector ingest event.
func (s *Store) CreateConnectorEvent(ctx context.Context, event ConnectorEvent) (*ConnectorEvent, error) {
	if strings.TrimSpace(event.InstallationID) == "" {
		return nil, fmt.Errorf("connector event installation_id is required")
	}
	if strings.TrimSpace(event.EventType) == "" {
		return nil, fmt.Errorf("connector event event_type is required")
	}
	if event.ID == "" {
		id, err := newID("evt")
		if err != nil {
			return nil, err
		}
		event.ID = id
	}
	if event.Status == "" {
		event.Status = "received"
	}
	if event.ReceivedAt.IsZero() {
		event.ReceivedAt = time.Now().UTC()
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = event.ReceivedAt
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO connector_events (
			id, installation_id, source_event_id, event_type, action, status, payload_json, metadata_json,
			occurred_at, received_at, processed_at, error_message
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(installation_id, source_event_id) DO UPDATE SET
			event_type = excluded.event_type,
			action = excluded.action,
			status = excluded.status,
			payload_json = excluded.payload_json,
			metadata_json = excluded.metadata_json,
			occurred_at = excluded.occurred_at,
			received_at = excluded.received_at,
			processed_at = excluded.processed_at,
			error_message = excluded.error_message
	`, event.ID, event.InstallationID, nullString(event.SourceEventID), event.EventType, nullString(event.Action), event.Status, rawMessage(event.PayloadJSON), rawMessage(event.MetadataJSON), event.OccurredAt, event.ReceivedAt, nullTime(event.ProcessedAt), nullString(event.ErrorMessage))
	if err != nil {
		return nil, fmt.Errorf("insert connector event: %w", err)
	}
	return &event, nil
}

// ListConnectorEvents returns recent events for an installation.
func (s *Store) ListConnectorEvents(ctx context.Context, installationID string, limit int) ([]ConnectorEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	query := `
		SELECT id, installation_id, source_event_id, event_type, action, status, payload_json, metadata_json, occurred_at, received_at, processed_at, error_message
		FROM connector_events
	`
	args := []any{}
	if strings.TrimSpace(installationID) != "" {
		query += ` WHERE installation_id = ?`
		args = append(args, installationID)
	}
	query += ` ORDER BY received_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list connector events: %w", err)
	}
	defer rows.Close()
	var out []ConnectorEvent
	for rows.Next() {
		var event ConnectorEvent
		var payloadJSON sql.NullString
		var metadataJSON sql.NullString
		var processed sql.NullTime
		var sourceID sql.NullString
		var action sql.NullString
		var errorMessage sql.NullString
		if err := rows.Scan(&event.ID, &event.InstallationID, &sourceID, &event.EventType, &action, &event.Status, &payloadJSON, &metadataJSON, &event.OccurredAt, &event.ReceivedAt, &processed, &errorMessage); err != nil {
			return nil, fmt.Errorf("scan connector event: %w", err)
		}
		event.SourceEventID = sourceID.String
		event.Action = action.String
		event.PayloadJSON = rawMessageFromNull(payloadJSON)
		event.MetadataJSON = rawMessageFromNull(metadataJSON)
		event.ProcessedAt = scanTime(processed)
		event.ErrorMessage = errorMessage.String
		out = append(out, event)
	}
	return out, rows.Err()
}

// CreateConnectorActionLog records a connector action dispatch.
func (s *Store) CreateConnectorActionLog(ctx context.Context, log ConnectorActionLog) (*ConnectorActionLog, error) {
	if strings.TrimSpace(log.InstallationID) == "" {
		return nil, fmt.Errorf("connector action installation_id is required")
	}
	if strings.TrimSpace(log.ActionName) == "" {
		return nil, fmt.Errorf("connector action name is required")
	}
	if log.ID == "" {
		id, err := newID("act")
		if err != nil {
			return nil, err
		}
		log.ID = id
	}
	if log.Status == "" {
		log.Status = "started"
	}
	if log.StartedAt.IsZero() {
		log.StartedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO connector_action_logs (
			id, installation_id, action_name, status, request_json, response_json, error_message, started_at, completed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			status = excluded.status,
			request_json = excluded.request_json,
			response_json = excluded.response_json,
			error_message = excluded.error_message,
			completed_at = excluded.completed_at
	`, log.ID, log.InstallationID, log.ActionName, log.Status, rawMessage(log.RequestJSON), rawMessage(log.ResponseJSON), nullString(log.ErrorMessage), log.StartedAt, nullTime(log.CompletedAt))
	if err != nil {
		return nil, fmt.Errorf("insert connector action log: %w", err)
	}
	return &log, nil
}

// CreateCloudAuditEvent appends an audit event to the cloud audit trail.
func (s *Store) CreateCloudAuditEvent(ctx context.Context, event CloudAuditEvent) (*CloudAuditEvent, error) {
	if strings.TrimSpace(event.ActorType) == "" {
		return nil, fmt.Errorf("cloud audit actor_type is required")
	}
	if strings.TrimSpace(event.Action) == "" {
		return nil, fmt.Errorf("cloud audit action is required")
	}
	if strings.TrimSpace(event.Outcome) == "" {
		return nil, fmt.Errorf("cloud audit outcome is required")
	}
	if event.ID == "" {
		id, err := newID("audit")
		if err != nil {
			return nil, err
		}
		event.ID = id
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO cloud_audit_events (
			id, actor_type, actor_id, action, resource_type, resource_id, org_id, workspace_id, project_id, outcome, details_json, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, event.ID, event.ActorType, nullString(event.ActorID), event.Action, nullString(event.ResourceType), nullString(event.ResourceID), nullString(event.OrgID), nullString(event.WorkspaceID), nullString(event.ProjectID), event.Outcome, rawMessage(event.DetailsJSON), event.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert cloud audit event: %w", err)
	}
	return &event, nil
}

// ListCloudAuditEvents returns recent audit events newest first.
func (s *Store) ListCloudAuditEvents(ctx context.Context, limit int) ([]CloudAuditEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, actor_type, actor_id, action, resource_type, resource_id, org_id, workspace_id, project_id, outcome, details_json, created_at
		FROM cloud_audit_events
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list cloud audit events: %w", err)
	}
	defer rows.Close()
	var out []CloudAuditEvent
	for rows.Next() {
		var event CloudAuditEvent
		var actorID sql.NullString
		var resourceType sql.NullString
		var resourceID sql.NullString
		var orgID sql.NullString
		var workspaceID sql.NullString
		var projectID sql.NullString
		var detailsJSON sql.NullString
		if err := rows.Scan(&event.ID, &event.ActorType, &actorID, &event.Action, &resourceType, &resourceID, &orgID, &workspaceID, &projectID, &event.Outcome, &detailsJSON, &event.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan cloud audit event: %w", err)
		}
		event.ActorID = actorID.String
		event.ResourceType = resourceType.String
		event.ResourceID = resourceID.String
		event.OrgID = orgID.String
		event.WorkspaceID = workspaceID.String
		event.ProjectID = projectID.String
		event.DetailsJSON = rawMessageFromNull(detailsJSON)
		out = append(out, event)
	}
	return out, rows.Err()
}

// RevokeAPIToken marks an API token as revoked without deleting its historical row.
func (s *Store) RevokeAPIToken(ctx context.Context, tokenID string) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		UPDATE api_tokens
		SET disabled = 1, revoked_at = ?, updated_at = ?
		WHERE id = ?
	`, now, now, tokenID)
	if err != nil {
		return fmt.Errorf("revoke api token: %w", err)
	}
	return nil
}

// ErrNotFound returns a stable not-found error for the requested entity.
func ErrNotFound(entity, id string) error {
	return fmt.Errorf("%s %q not found", entity, id)
}

func rawMessage(value json.RawMessage) any {
	if len(value) == 0 {
		return nil
	}
	return string(value)
}

func rawMessageFromNull(value sql.NullString) json.RawMessage {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return nil
	}
	return json.RawMessage(value.String)
}

func nullString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func nullTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	t := value.UTC()
	return t
}

func scanTime(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time.UTC()
	return &t
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
