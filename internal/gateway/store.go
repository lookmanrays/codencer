package gateway

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const (
	defaultDeviceTTL = 10 * time.Minute
	defaultTokenTTL  = 24 * time.Hour
)

type Store struct {
	db *sql.DB
}

type User struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Workspace struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	OwnerUserID string    `json:"owner_user_id"`
	Kind        string    `json:"kind"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type AccountContext struct {
	User      User      `json:"user"`
	Workspace Workspace `json:"workspace"`
}

type MachineRecord struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	UserID      string    `json:"user_id"`
	Hostname    string    `json:"hostname,omitempty"`
	HostLabel   string    `json:"host_label,omitempty"`
	OS          string    `json:"os,omitempty"`
	Arch        string    `json:"arch,omitempty"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ConnectorBinding struct {
	ID               string    `json:"id"`
	WorkspaceID      string    `json:"workspace_id"`
	MachineID        string    `json:"machine_id"`
	RelayProfileID   string    `json:"relay_profile_id"`
	RelayConnectorID string    `json:"relay_connector_id,omitempty"`
	RelayMachineID   string    `json:"relay_machine_id,omitempty"`
	PublicKey        string    `json:"public_key,omitempty"`
	Status           string    `json:"status"`
	LastSeenAt       time.Time `json:"last_seen_at,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type RelayProfileRecord struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	Type        string    `json:"type"`
	Name        string    `json:"name"`
	URL         string    `json:"url"`
	TokenEnv    string    `json:"token_env,omitempty"`
	TokenFile   string    `json:"token_file,omitempty"`
	Enabled     bool      `json:"enabled"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type AccessTokenRecord struct {
	TokenHash   string    `json:"token_hash"`
	UserID      string    `json:"user_id"`
	WorkspaceID string    `json:"workspace_id"`
	ClientID    string    `json:"client_id"`
	Resource    string    `json:"resource"`
	Scopes      []string  `json:"scopes"`
	ExpiresAt   time.Time `json:"expires_at"`
	RevokedAt   time.Time `json:"revoked_at,omitempty"`
}

type DeviceCodeRecord struct {
	DeviceCodeHash string    `json:"device_code_hash"`
	UserCodeHash   string    `json:"user_code_hash"`
	UserCode       string    `json:"user_code"`
	Status         string    `json:"status"`
	Email          string    `json:"email,omitempty"`
	DisplayName    string    `json:"display_name,omitempty"`
	UserID         string    `json:"user_id,omitempty"`
	WorkspaceID    string    `json:"workspace_id,omitempty"`
	ExpiresAt      time.Time `json:"expires_at"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type AuditEvent struct {
	ID          string         `json:"id"`
	WorkspaceID string         `json:"workspace_id,omitempty"`
	ActorUserID string         `json:"actor_user_id,omitempty"`
	Type        string         `json:"type"`
	Summary     string         `json:"summary"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

type RunRecord struct {
	ID              string         `json:"id"`
	WorkspaceID     string         `json:"workspace_id,omitempty"`
	ActorUserID     string         `json:"actor_user_id,omitempty"`
	ProjectID       string         `json:"project_id"`
	ProjectName     string         `json:"project_name,omitempty"`
	RunID           string         `json:"run_id,omitempty"`
	StepID          string         `json:"step_id,omitempty"`
	Title           string         `json:"title,omitempty"`
	Goal            string         `json:"goal,omitempty"`
	Mode            string         `json:"mode,omitempty"`
	ExecutorProfile string         `json:"executor_profile,omitempty"`
	Status          string         `json:"status,omitempty"`
	ReportStatus    string         `json:"report_status,omitempty"`
	ResultSummary   string         `json:"result_summary,omitempty"`
	ResultDetails   string         `json:"result_details,omitempty"`
	RelayProfileID  string         `json:"relay_profile_id,omitempty"`
	ConnectorID     string         `json:"connector_id,omitempty"`
	MachineID       string         `json:"machine_id,omitempty"`
	HostLabel       string         `json:"host_label,omitempty"`
	StartedAt       time.Time      `json:"started_at,omitempty"`
	CompletedAt     time.Time      `json:"completed_at,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	Report          map[string]any `json:"report,omitempty"`
}

type RunRecordFilters struct {
	ProjectID string
	Status    string
	Limit     int
}

type DeviceAuthorization struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type TokenResponse struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresIn   int       `json:"expires_in"`
	ExpiresAt   time.Time `json:"expires_at"`
	Scope       string    `json:"scope"`
	Resource    string    `json:"resource"`
	UserID      string    `json:"user_id"`
	WorkspaceID string    `json:"workspace_id"`
}

func OpenStore(path string) (*Store, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("gateway store path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create gateway store parent: %w", err)
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("open gateway store: %w", err)
	}
	store := &Store{db: db}
	if err := store.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			display_name TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS workspaces (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			owner_user_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS workspace_members (
			workspace_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			role TEXT NOT NULL,
			created_at TEXT NOT NULL,
			PRIMARY KEY (workspace_id, user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS machines (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			hostname TEXT NOT NULL DEFAULT '',
			host_label TEXT NOT NULL DEFAULT '',
			os TEXT NOT NULL DEFAULT '',
			arch TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS connector_bindings (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			machine_id TEXT NOT NULL,
			relay_profile_id TEXT NOT NULL,
			relay_connector_id TEXT NOT NULL DEFAULT '',
			relay_machine_id TEXT NOT NULL DEFAULT '',
			public_key TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			last_seen_at TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS relay_profiles (
			id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			type TEXT NOT NULL,
			name TEXT NOT NULL,
			url TEXT NOT NULL,
			token_env TEXT NOT NULL DEFAULT '',
			token_file TEXT NOT NULL DEFAULT '',
			enabled INTEGER NOT NULL,
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (workspace_id, id)
		)`,
		`CREATE TABLE IF NOT EXISTS access_tokens (
			token_hash TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			client_id TEXT NOT NULL,
			resource TEXT NOT NULL,
			scopes TEXT NOT NULL,
			expires_at TEXT NOT NULL,
			revoked_at TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS device_codes (
			device_code_hash TEXT PRIMARY KEY,
			user_code_hash TEXT NOT NULL UNIQUE,
			user_code TEXT NOT NULL,
			status TEXT NOT NULL,
			email TEXT NOT NULL DEFAULT '',
			display_name TEXT NOT NULL DEFAULT '',
			user_id TEXT NOT NULL DEFAULT '',
			workspace_id TEXT NOT NULL DEFAULT '',
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS audit_events (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL DEFAULT '',
			actor_user_id TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL,
			summary TEXT NOT NULL,
			metadata_json TEXT NOT NULL DEFAULT '{}',
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS gateway_run_records (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL DEFAULT '',
			actor_user_id TEXT NOT NULL DEFAULT '',
			project_id TEXT NOT NULL DEFAULT '',
			project_name TEXT NOT NULL DEFAULT '',
			run_id TEXT NOT NULL DEFAULT '',
			step_id TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL DEFAULT '',
			goal TEXT NOT NULL DEFAULT '',
			mode TEXT NOT NULL DEFAULT '',
			executor_profile TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT '',
			report_status TEXT NOT NULL DEFAULT '',
			result_summary TEXT NOT NULL DEFAULT '',
			result_details TEXT NOT NULL DEFAULT '',
			relay_profile_id TEXT NOT NULL DEFAULT '',
			connector_id TEXT NOT NULL DEFAULT '',
			machine_id TEXT NOT NULL DEFAULT '',
			host_label TEXT NOT NULL DEFAULT '',
			started_at TEXT NOT NULL DEFAULT '',
			completed_at TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			report_json TEXT NOT NULL DEFAULT '{}'
		)`,
		`CREATE INDEX IF NOT EXISTS idx_gateway_run_records_workspace_updated ON gateway_run_records (workspace_id, updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_gateway_run_records_run_id ON gateway_run_records (workspace_id, run_id)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migrate gateway store: %w", err)
		}
	}
	if _, err := s.db.ExecContext(ctx, `ALTER TABLE audit_events ADD COLUMN metadata_json TEXT NOT NULL DEFAULT '{}'`); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		return fmt.Errorf("migrate gateway audit metadata: %w", err)
	}
	return nil
}

func (s *Store) EnsureUserWorkspace(ctx context.Context, email, displayName string, defaults DefaultRelay) (AccountContext, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		email = "dev@codencer.local"
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		displayName = email
	}
	now := time.Now().UTC()
	var user User
	row := s.db.QueryRowContext(ctx, `SELECT id, email, display_name, created_at, updated_at FROM users WHERE email = ?`, email)
	if err := scanUser(row, &user); err != nil {
		if err != sql.ErrNoRows {
			return AccountContext{}, err
		}
		id, genErr := randomStoreID("usr")
		if genErr != nil {
			return AccountContext{}, genErr
		}
		user = User{ID: id, Email: email, DisplayName: displayName, CreatedAt: now, UpdatedAt: now}
		if _, err := s.db.ExecContext(ctx, `INSERT INTO users (id, email, display_name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
			user.ID, user.Email, user.DisplayName, formatStoreTime(user.CreatedAt), formatStoreTime(user.UpdatedAt)); err != nil {
			return AccountContext{}, fmt.Errorf("create gateway user: %w", err)
		}
	} else if displayName != "" && user.DisplayName != displayName {
		user.DisplayName = displayName
		user.UpdatedAt = now
		if _, err := s.db.ExecContext(ctx, `UPDATE users SET display_name = ?, updated_at = ? WHERE id = ?`, user.DisplayName, formatStoreTime(user.UpdatedAt), user.ID); err != nil {
			return AccountContext{}, fmt.Errorf("update gateway user: %w", err)
		}
	}

	workspace, err := s.ensurePersonalWorkspace(ctx, user, now)
	if err != nil {
		return AccountContext{}, err
	}
	if _, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO workspace_members (workspace_id, user_id, role, created_at) VALUES (?, ?, ?, ?)`,
		workspace.ID, user.ID, "owner", formatStoreTime(now)); err != nil {
		return AccountContext{}, fmt.Errorf("create workspace member: %w", err)
	}
	if _, err := s.EnsureDefaultRelayProfile(ctx, workspace.ID, defaults); err != nil {
		return AccountContext{}, err
	}
	return AccountContext{User: user, Workspace: workspace}, nil
}

func (s *Store) ensurePersonalWorkspace(ctx context.Context, user User, now time.Time) (Workspace, error) {
	var workspace Workspace
	row := s.db.QueryRowContext(ctx, `SELECT id, name, owner_user_id, kind, created_at, updated_at FROM workspaces WHERE owner_user_id = ? AND kind = 'personal' LIMIT 1`, user.ID)
	if err := scanWorkspace(row, &workspace); err != nil {
		if err != sql.ErrNoRows {
			return Workspace{}, err
		}
		id, genErr := randomStoreID("ws")
		if genErr != nil {
			return Workspace{}, genErr
		}
		workspace = Workspace{ID: id, Name: "Personal", OwnerUserID: user.ID, Kind: "personal", CreatedAt: now, UpdatedAt: now}
		if _, err := s.db.ExecContext(ctx, `INSERT INTO workspaces (id, name, owner_user_id, kind, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
			workspace.ID, workspace.Name, workspace.OwnerUserID, workspace.Kind, formatStoreTime(workspace.CreatedAt), formatStoreTime(workspace.UpdatedAt)); err != nil {
			return Workspace{}, fmt.Errorf("create workspace: %w", err)
		}
	}
	return workspace, nil
}

func (s *Store) EnsureDefaultRelayProfile(ctx context.Context, workspaceID string, defaults DefaultRelay) (RelayProfileRecord, error) {
	defaults.URL = strings.TrimRight(strings.TrimSpace(firstNonEmpty(defaults.URL, DefaultRelayURL)), "/")
	defaults.TokenEnv = strings.TrimSpace(firstNonEmpty(defaults.TokenEnv, DefaultRelayToken))
	defaults.TokenFile = strings.TrimSpace(defaults.TokenFile)
	current, err := s.GetRelayProfile(ctx, workspaceID, "default")
	if err == nil {
		return current, nil
	}
	if err != sql.ErrNoRows {
		return RelayProfileRecord{}, err
	}
	return s.UpsertRelayProfile(ctx, RelayProfileRecord{
		ID:          "default",
		WorkspaceID: workspaceID,
		Type:        "managed",
		Name:        "Default Codencer Relay",
		URL:         defaults.URL,
		TokenEnv:    defaults.TokenEnv,
		TokenFile:   defaults.TokenFile,
		Enabled:     true,
		Status:      "available",
	})
}

func (s *Store) CreateDeviceAuthorization(ctx context.Context, email, displayName, verificationURI string) (DeviceAuthorization, error) {
	deviceCode, err := randomOpaqueToken(32)
	if err != nil {
		return DeviceAuthorization{}, err
	}
	userCode, err := randomUserCode()
	if err != nil {
		return DeviceAuthorization{}, err
	}
	now := time.Now().UTC()
	expiresAt := now.Add(defaultDeviceTTL)
	if _, err := s.db.ExecContext(ctx, `INSERT INTO device_codes (device_code_hash, user_code_hash, user_code, status, email, display_name, expires_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tokenHash(deviceCode), tokenHash(userCode), userCode, "pending", strings.ToLower(strings.TrimSpace(email)), strings.TrimSpace(displayName), formatStoreTime(expiresAt), formatStoreTime(now), formatStoreTime(now)); err != nil {
		return DeviceAuthorization{}, fmt.Errorf("create device code: %w", err)
	}
	return DeviceAuthorization{DeviceCode: deviceCode, UserCode: userCode, VerificationURI: verificationURI, ExpiresIn: int(defaultDeviceTTL.Seconds()), Interval: 1}, nil
}

func (s *Store) ApproveDeviceCode(ctx context.Context, userCode string, defaults DefaultRelay) (AccountContext, error) {
	userCode = strings.ToUpper(strings.TrimSpace(userCode))
	record, err := s.deviceByUserCode(ctx, userCode)
	if err != nil {
		return AccountContext{}, err
	}
	if record.Status != "pending" {
		return AccountContext{}, fmt.Errorf("device code is %s", record.Status)
	}
	if time.Now().UTC().After(record.ExpiresAt) {
		_, _ = s.db.ExecContext(ctx, `UPDATE device_codes SET status = ?, updated_at = ? WHERE user_code_hash = ?`, "expired", formatStoreTime(time.Now().UTC()), tokenHash(userCode))
		return AccountContext{}, fmt.Errorf("device code expired")
	}
	account, err := s.EnsureUserWorkspace(ctx, record.Email, record.DisplayName, defaults)
	if err != nil {
		return AccountContext{}, err
	}
	now := time.Now().UTC()
	if _, err := s.db.ExecContext(ctx, `UPDATE device_codes SET status = ?, user_id = ?, workspace_id = ?, updated_at = ? WHERE user_code_hash = ?`,
		"approved", account.User.ID, account.Workspace.ID, formatStoreTime(now), tokenHash(userCode)); err != nil {
		return AccountContext{}, fmt.Errorf("approve device code: %w", err)
	}
	_ = s.RecordAudit(ctx, AuditEvent{WorkspaceID: account.Workspace.ID, ActorUserID: account.User.ID, Type: "login.device_approve", Summary: "Approved Codencer device login"})
	return account, nil
}

func (s *Store) PollDeviceToken(ctx context.Context, deviceCode string, cfg *Config) (TokenResponse, *apiError) {
	record, err := s.deviceByDeviceCode(ctx, deviceCode)
	if err != nil {
		return TokenResponse{}, &apiError{Status: 400, Code: "invalid_grant", Message: "device_code is invalid"}
	}
	now := time.Now().UTC()
	if now.After(record.ExpiresAt) {
		_, _ = s.db.ExecContext(ctx, `UPDATE device_codes SET status = ?, updated_at = ? WHERE device_code_hash = ?`, "expired", formatStoreTime(now), tokenHash(deviceCode))
		return TokenResponse{}, &apiError{Status: 400, Code: "expired_token", Message: "device_code is expired"}
	}
	if record.Status == "pending" {
		return TokenResponse{}, &apiError{Status: 428, Code: "authorization_pending", Message: "device authorization is pending"}
	}
	if record.Status != "approved" {
		return TokenResponse{}, &apiError{Status: 400, Code: "invalid_grant", Message: "device authorization is not approved"}
	}
	scopes := defaultGatewayScopes()
	resource := strings.TrimRight(firstNonEmpty(cfg.MCPURL, cfg.PublicBaseURL+"/mcp"), "/")
	token, err := s.CreateAccessToken(ctx, record.UserID, record.WorkspaceID, "codencer-cli-device", resource, scopes, defaultTokenTTL)
	if err != nil {
		return TokenResponse{}, &apiError{Status: 500, Code: "gateway_internal_error", Message: err.Error()}
	}
	return token, nil
}

func (s *Store) CreateAccessToken(ctx context.Context, userID, workspaceID, clientID, resource string, scopes []string, ttl time.Duration) (TokenResponse, error) {
	if ttl <= 0 {
		ttl = defaultTokenTTL
	}
	token, err := randomOpaqueToken(32)
	if err != nil {
		return TokenResponse{}, err
	}
	expiresAt := time.Now().UTC().Add(ttl)
	scope := strings.Join(cleanList(scopes), " ")
	if _, err := s.db.ExecContext(ctx, `INSERT INTO access_tokens (token_hash, user_id, workspace_id, client_id, resource, scopes, expires_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		tokenHash(token), userID, workspaceID, clientID, strings.TrimRight(resource, "/"), scope, formatStoreTime(expiresAt), formatStoreTime(time.Now().UTC())); err != nil {
		return TokenResponse{}, fmt.Errorf("create access token: %w", err)
	}
	return TokenResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   int(ttl.Seconds()),
		ExpiresAt:   expiresAt,
		Scope:       scope,
		Resource:    strings.TrimRight(resource, "/"),
		UserID:      userID,
		WorkspaceID: workspaceID,
	}, nil
}

func (s *Store) LookupAccessToken(ctx context.Context, token string) (AccessTokenRecord, error) {
	var rec AccessTokenRecord
	var scopes, revokedAt string
	row := s.db.QueryRowContext(ctx, `SELECT token_hash, user_id, workspace_id, client_id, resource, scopes, expires_at, revoked_at FROM access_tokens WHERE token_hash = ?`, tokenHash(token))
	var expiresAt string
	if err := row.Scan(&rec.TokenHash, &rec.UserID, &rec.WorkspaceID, &rec.ClientID, &rec.Resource, &scopes, &expiresAt, &revokedAt); err != nil {
		return AccessTokenRecord{}, err
	}
	rec.Scopes = strings.Fields(scopes)
	rec.ExpiresAt = parseStoreTime(expiresAt)
	rec.RevokedAt = parseStoreTime(revokedAt)
	if !rec.RevokedAt.IsZero() {
		return AccessTokenRecord{}, fmt.Errorf("access token revoked")
	}
	if time.Now().UTC().After(rec.ExpiresAt) {
		return AccessTokenRecord{}, fmt.Errorf("access token expired")
	}
	return rec, nil
}

func (s *Store) RevokeAccessToken(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE access_tokens SET revoked_at = ? WHERE token_hash = ?`, formatStoreTime(time.Now().UTC()), tokenHash(token))
	return err
}

func (s *Store) UpsertRelayProfile(ctx context.Context, profile RelayProfileRecord) (RelayProfileRecord, error) {
	now := time.Now().UTC()
	profile.ID = sanitizeRelayProfileID(firstNonEmpty(profile.ID, profile.Name))
	if profile.ID == "" {
		id, err := randomStoreID("relay")
		if err != nil {
			return RelayProfileRecord{}, err
		}
		profile.ID = id
	}
	profile.Name = strings.TrimSpace(firstNonEmpty(profile.Name, profile.ID))
	profile.URL = strings.TrimRight(strings.TrimSpace(profile.URL), "/")
	profile.Type = strings.TrimSpace(firstNonEmpty(profile.Type, "self_host"))
	profile.Status = strings.TrimSpace(firstNonEmpty(profile.Status, "available"))
	if profile.WorkspaceID == "" {
		return RelayProfileRecord{}, fmt.Errorf("workspace_id is required")
	}
	if err := validatePublicURL("relay.url", profile.URL); err != nil {
		return RelayProfileRecord{}, err
	}
	var createdAt string
	_ = s.db.QueryRowContext(ctx, `SELECT created_at FROM relay_profiles WHERE workspace_id = ? AND id = ?`, profile.WorkspaceID, profile.ID).Scan(&createdAt)
	if createdAt == "" {
		createdAt = formatStoreTime(now)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO relay_profiles (id, workspace_id, type, name, url, token_env, token_file, enabled, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(workspace_id, id) DO UPDATE SET type = excluded.type, name = excluded.name, url = excluded.url, token_env = excluded.token_env, token_file = excluded.token_file, enabled = excluded.enabled, status = excluded.status, updated_at = excluded.updated_at`,
		profile.ID, profile.WorkspaceID, profile.Type, profile.Name, profile.URL, strings.TrimSpace(profile.TokenEnv), strings.TrimSpace(profile.TokenFile), boolInt(profile.Enabled), profile.Status, createdAt, formatStoreTime(now)); err != nil {
		return RelayProfileRecord{}, fmt.Errorf("upsert relay profile: %w", err)
	}
	return s.GetRelayProfile(ctx, profile.WorkspaceID, profile.ID)
}

func (s *Store) GetRelayProfile(ctx context.Context, workspaceID, id string) (RelayProfileRecord, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, workspace_id, type, name, url, token_env, token_file, enabled, status, created_at, updated_at FROM relay_profiles WHERE workspace_id = ? AND id = ?`, workspaceID, id)
	return scanRelayProfile(row)
}

func (s *Store) ListRelayProfiles(ctx context.Context, workspaceID string) ([]RelayProfileRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, workspace_id, type, name, url, token_env, token_file, enabled, status, created_at, updated_at FROM relay_profiles WHERE workspace_id = ? ORDER BY id`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RelayProfileRecord
	for rows.Next() {
		profile, err := scanRelayProfile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, profile)
	}
	return out, rows.Err()
}

func (s *Store) RemoveRelayProfile(ctx context.Context, workspaceID, id string) error {
	if id == "default" {
		return fmt.Errorf("default relay profile cannot be removed")
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM relay_profiles WHERE workspace_id = ? AND id = ?`, workspaceID, id)
	return err
}

func (s *Store) UpsertMachine(ctx context.Context, machine MachineRecord) (MachineRecord, error) {
	now := time.Now().UTC()
	if machine.ID == "" || machine.WorkspaceID == "" || machine.UserID == "" {
		return MachineRecord{}, fmt.Errorf("machine id, workspace_id, and user_id are required")
	}
	machine.Status = firstNonEmpty(machine.Status, "active")
	var createdAt string
	_ = s.db.QueryRowContext(ctx, `SELECT created_at FROM machines WHERE id = ?`, machine.ID).Scan(&createdAt)
	if createdAt == "" {
		createdAt = formatStoreTime(now)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO machines (id, workspace_id, user_id, hostname, host_label, os, arch, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET workspace_id = excluded.workspace_id, user_id = excluded.user_id, hostname = excluded.hostname, host_label = excluded.host_label, os = excluded.os, arch = excluded.arch, status = excluded.status, updated_at = excluded.updated_at`,
		machine.ID, machine.WorkspaceID, machine.UserID, machine.Hostname, machine.HostLabel, machine.OS, machine.Arch, machine.Status, createdAt, formatStoreTime(now)); err != nil {
		return MachineRecord{}, fmt.Errorf("upsert machine: %w", err)
	}
	return s.GetMachine(ctx, machine.ID, machine.WorkspaceID)
}

func (s *Store) GetMachine(ctx context.Context, machineID, workspaceID string) (MachineRecord, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, workspace_id, user_id, hostname, host_label, os, arch, status, created_at, updated_at FROM machines WHERE id = ? AND workspace_id = ?`, machineID, workspaceID)
	var rec MachineRecord
	var created, updated string
	if err := row.Scan(&rec.ID, &rec.WorkspaceID, &rec.UserID, &rec.Hostname, &rec.HostLabel, &rec.OS, &rec.Arch, &rec.Status, &created, &updated); err != nil {
		return MachineRecord{}, err
	}
	rec.CreatedAt = parseStoreTime(created)
	rec.UpdatedAt = parseStoreTime(updated)
	return rec, nil
}

func (s *Store) GetUser(ctx context.Context, userID string) (User, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, email, display_name, created_at, updated_at FROM users WHERE id = ?`, userID)
	var user User
	if err := scanUser(row, &user); err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Store) GetWorkspace(ctx context.Context, workspaceID string) (Workspace, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, name, owner_user_id, kind, created_at, updated_at FROM workspaces WHERE id = ?`, workspaceID)
	var workspace Workspace
	if err := scanWorkspace(row, &workspace); err != nil {
		return Workspace{}, err
	}
	return workspace, nil
}

func (s *Store) ListMachines(ctx context.Context, workspaceID string) ([]MachineRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, workspace_id, user_id, hostname, host_label, os, arch, status, created_at, updated_at FROM machines WHERE workspace_id = ? ORDER BY host_label, id`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MachineRecord{}
	for rows.Next() {
		var rec MachineRecord
		var created, updated string
		if err := rows.Scan(&rec.ID, &rec.WorkspaceID, &rec.UserID, &rec.Hostname, &rec.HostLabel, &rec.OS, &rec.Arch, &rec.Status, &created, &updated); err != nil {
			return nil, err
		}
		rec.CreatedAt = parseStoreTime(created)
		rec.UpdatedAt = parseStoreTime(updated)
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (s *Store) CreateConnectorBinding(ctx context.Context, workspaceID, machineID, relayProfileID string) (ConnectorBinding, error) {
	id, err := randomStoreID("conn")
	if err != nil {
		return ConnectorBinding{}, err
	}
	now := time.Now().UTC()
	rec := ConnectorBinding{ID: id, WorkspaceID: workspaceID, MachineID: machineID, RelayProfileID: relayProfileID, Status: "pending", CreatedAt: now, UpdatedAt: now}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO connector_bindings (id, workspace_id, machine_id, relay_profile_id, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		rec.ID, rec.WorkspaceID, rec.MachineID, rec.RelayProfileID, rec.Status, formatStoreTime(rec.CreatedAt), formatStoreTime(rec.UpdatedAt)); err != nil {
		return ConnectorBinding{}, fmt.Errorf("create connector binding: %w", err)
	}
	return rec, nil
}

func (s *Store) CompleteConnectorBinding(ctx context.Context, bindingID, relayConnectorID, relayMachineID, publicKey string) (ConnectorBinding, error) {
	now := time.Now().UTC()
	if _, err := s.db.ExecContext(ctx, `UPDATE connector_bindings SET relay_connector_id = ?, relay_machine_id = ?, public_key = ?, status = ?, last_seen_at = ?, updated_at = ? WHERE id = ?`,
		relayConnectorID, relayMachineID, publicKey, "online", formatStoreTime(now), formatStoreTime(now), bindingID); err != nil {
		return ConnectorBinding{}, fmt.Errorf("complete connector binding: %w", err)
	}
	return s.GetConnectorBinding(ctx, bindingID)
}

func (s *Store) GetConnectorBinding(ctx context.Context, id string) (ConnectorBinding, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, workspace_id, machine_id, relay_profile_id, relay_connector_id, relay_machine_id, public_key, status, last_seen_at, created_at, updated_at FROM connector_bindings WHERE id = ?`, id)
	var rec ConnectorBinding
	var lastSeen, created, updated string
	if err := row.Scan(&rec.ID, &rec.WorkspaceID, &rec.MachineID, &rec.RelayProfileID, &rec.RelayConnectorID, &rec.RelayMachineID, &rec.PublicKey, &rec.Status, &lastSeen, &created, &updated); err != nil {
		return ConnectorBinding{}, err
	}
	rec.LastSeenAt = parseStoreTime(lastSeen)
	rec.CreatedAt = parseStoreTime(created)
	rec.UpdatedAt = parseStoreTime(updated)
	return rec, nil
}

func (s *Store) ListConnectorBindings(ctx context.Context, workspaceID string) ([]ConnectorBinding, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, workspace_id, machine_id, relay_profile_id, relay_connector_id, relay_machine_id, public_key, status, last_seen_at, created_at, updated_at FROM connector_bindings WHERE workspace_id = ? ORDER BY updated_at DESC, id`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ConnectorBinding{}
	for rows.Next() {
		var rec ConnectorBinding
		var lastSeen, created, updated string
		if err := rows.Scan(&rec.ID, &rec.WorkspaceID, &rec.MachineID, &rec.RelayProfileID, &rec.RelayConnectorID, &rec.RelayMachineID, &rec.PublicKey, &rec.Status, &lastSeen, &created, &updated); err != nil {
			return nil, err
		}
		rec.LastSeenAt = parseStoreTime(lastSeen)
		rec.CreatedAt = parseStoreTime(created)
		rec.UpdatedAt = parseStoreTime(updated)
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (s *Store) RecordAudit(ctx context.Context, event AuditEvent) error {
	if strings.TrimSpace(event.Type) == "" {
		return nil
	}
	if event.ID == "" {
		id, err := randomStoreID("evt")
		if err != nil {
			return err
		}
		event.ID = id
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	metadataJSON := "{}"
	if len(event.Metadata) > 0 {
		if data, err := json.Marshal(event.Metadata); err == nil {
			metadataJSON = string(data)
		}
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO audit_events (id, workspace_id, actor_user_id, type, summary, metadata_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		event.ID, event.WorkspaceID, event.ActorUserID, event.Type, event.Summary, metadataJSON, formatStoreTime(event.CreatedAt))
	return err
}

func (s *Store) ListAuditEvents(ctx context.Context, workspaceID string, limit int) ([]AuditEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, workspace_id, actor_user_id, type, summary, metadata_json, created_at FROM audit_events WHERE workspace_id = ? ORDER BY created_at DESC LIMIT ?`, workspaceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AuditEvent{}
	for rows.Next() {
		var event AuditEvent
		var created, metadataJSON string
		if err := rows.Scan(&event.ID, &event.WorkspaceID, &event.ActorUserID, &event.Type, &event.Summary, &metadataJSON, &created); err != nil {
			return nil, err
		}
		event.Metadata = decodeStoreMap(metadataJSON)
		event.CreatedAt = parseStoreTime(created)
		out = append(out, event)
	}
	return out, rows.Err()
}

func (s *Store) ListAuditEventsByRunHistoryID(ctx context.Context, workspaceID, runHistoryID string, limit int) ([]AuditEvent, error) {
	runHistoryID = strings.TrimSpace(runHistoryID)
	if runHistoryID == "" {
		return []AuditEvent{}, nil
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, workspace_id, actor_user_id, type, summary, metadata_json, created_at FROM audit_events WHERE workspace_id = ? AND metadata_json LIKE ? ORDER BY created_at ASC LIMIT ?`, workspaceID, "%\"run_history_id\":\""+runHistoryID+"\"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AuditEvent{}
	for rows.Next() {
		var event AuditEvent
		var created, metadataJSON string
		if err := rows.Scan(&event.ID, &event.WorkspaceID, &event.ActorUserID, &event.Type, &event.Summary, &metadataJSON, &created); err != nil {
			return nil, err
		}
		event.Metadata = decodeStoreMap(metadataJSON)
		event.CreatedAt = parseStoreTime(created)
		out = append(out, event)
	}
	return out, rows.Err()
}

func (s *Store) UpsertRunRecord(ctx context.Context, record RunRecord) (RunRecord, error) {
	now := time.Now().UTC()
	record.ID = strings.TrimSpace(record.ID)
	if record.ID == "" && strings.TrimSpace(record.RunID) != "" {
		if existing, err := s.FindRunRecord(ctx, record.WorkspaceID, record.ProjectID, record.RunID, record.RelayProfileID, record.MachineID, record.HostLabel); err == nil {
			record.ID = existing.ID
			if record.CreatedAt.IsZero() {
				record.CreatedAt = existing.CreatedAt
			}
		}
	}
	if record.ID == "" {
		id, err := randomStoreID("runhist")
		if err != nil {
			return RunRecord{}, err
		}
		record.ID = id
	}
	if record.WorkspaceID == "" {
		return RunRecord{}, fmt.Errorf("workspace_id is required")
	}
	if record.CreatedAt.IsZero() {
		var created string
		_ = s.db.QueryRowContext(ctx, `SELECT created_at FROM gateway_run_records WHERE id = ? AND workspace_id = ?`, record.ID, record.WorkspaceID).Scan(&created)
		record.CreatedAt = parseStoreTime(created)
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	record.UpdatedAt = now
	reportJSON := "{}"
	if len(record.Report) > 0 {
		if data, err := json.Marshal(record.Report); err == nil {
			reportJSON = string(data)
		}
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO gateway_run_records (
			id, workspace_id, actor_user_id, project_id, project_name, run_id, step_id, title, goal, mode,
			executor_profile, status, report_status, result_summary, result_details, relay_profile_id,
			connector_id, machine_id, host_label, started_at, completed_at, created_at, updated_at, report_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			actor_user_id = excluded.actor_user_id,
			project_id = excluded.project_id,
			project_name = excluded.project_name,
			run_id = excluded.run_id,
			step_id = excluded.step_id,
			title = excluded.title,
			goal = excluded.goal,
			mode = excluded.mode,
			executor_profile = excluded.executor_profile,
			status = excluded.status,
			report_status = excluded.report_status,
			result_summary = excluded.result_summary,
			result_details = excluded.result_details,
			relay_profile_id = excluded.relay_profile_id,
			connector_id = excluded.connector_id,
			machine_id = excluded.machine_id,
			host_label = excluded.host_label,
			started_at = excluded.started_at,
			completed_at = excluded.completed_at,
			updated_at = excluded.updated_at,
			report_json = excluded.report_json`,
		record.ID, record.WorkspaceID, record.ActorUserID, record.ProjectID, record.ProjectName, record.RunID, record.StepID, record.Title, record.Goal, record.Mode,
		record.ExecutorProfile, record.Status, record.ReportStatus, record.ResultSummary, record.ResultDetails, record.RelayProfileID,
		record.ConnectorID, record.MachineID, record.HostLabel, formatStoreTime(record.StartedAt), formatStoreTime(record.CompletedAt), formatStoreTime(record.CreatedAt), formatStoreTime(record.UpdatedAt), reportJSON)
	if err != nil {
		return RunRecord{}, fmt.Errorf("upsert gateway run record: %w", err)
	}
	return s.GetRunRecord(ctx, record.WorkspaceID, record.ID)
}

func (s *Store) FindRunRecord(ctx context.Context, workspaceID, projectID, runID, relayProfileID, machineID, hostLabel string) (RunRecord, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, workspace_id, actor_user_id, project_id, project_name, run_id, step_id, title, goal, mode, executor_profile, status, report_status, result_summary, result_details, relay_profile_id, connector_id, machine_id, host_label, started_at, completed_at, created_at, updated_at, report_json
		FROM gateway_run_records
		WHERE workspace_id = ? AND project_id = ? AND run_id = ? AND relay_profile_id = ? AND machine_id = ? AND host_label = ?
		ORDER BY updated_at DESC LIMIT 1`,
		workspaceID, projectID, runID, relayProfileID, machineID, hostLabel)
	return scanRunRecord(row)
}

func (s *Store) FindRunRecordByRunID(ctx context.Context, workspaceID, projectID, runID string) (RunRecord, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, workspace_id, actor_user_id, project_id, project_name, run_id, step_id, title, goal, mode, executor_profile, status, report_status, result_summary, result_details, relay_profile_id, connector_id, machine_id, host_label, started_at, completed_at, created_at, updated_at, report_json
		FROM gateway_run_records
		WHERE workspace_id = ? AND project_id = ? AND run_id = ?
		ORDER BY updated_at DESC LIMIT 1`, workspaceID, projectID, runID)
	return scanRunRecord(row)
}

func (s *Store) GetRunRecord(ctx context.Context, workspaceID, id string) (RunRecord, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, workspace_id, actor_user_id, project_id, project_name, run_id, step_id, title, goal, mode, executor_profile, status, report_status, result_summary, result_details, relay_profile_id, connector_id, machine_id, host_label, started_at, completed_at, created_at, updated_at, report_json
		FROM gateway_run_records WHERE workspace_id = ? AND id = ?`, workspaceID, id)
	return scanRunRecord(row)
}

func (s *Store) ListRunRecords(ctx context.Context, workspaceID string, filters RunRecordFilters) ([]RunRecord, error) {
	limit := filters.Limit
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	query := `SELECT id, workspace_id, actor_user_id, project_id, project_name, run_id, step_id, title, goal, mode, executor_profile, status, report_status, result_summary, result_details, relay_profile_id, connector_id, machine_id, host_label, started_at, completed_at, created_at, updated_at, report_json FROM gateway_run_records WHERE workspace_id = ?`
	args := []any{workspaceID}
	if strings.TrimSpace(filters.ProjectID) != "" {
		query += ` AND project_id = ?`
		args = append(args, strings.TrimSpace(filters.ProjectID))
	}
	if strings.TrimSpace(filters.Status) != "" {
		query += ` AND status = ?`
		args = append(args, strings.TrimSpace(filters.Status))
	}
	query += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RunRecord{}
	for rows.Next() {
		record, err := scanRunRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	return out, rows.Err()
}

func (p RelayProfileRecord) ToRelayProfile() RelayProfile {
	return RelayProfile{
		ID:        p.ID,
		Name:      p.Name,
		URL:       p.URL,
		TokenEnv:  p.TokenEnv,
		TokenFile: p.TokenFile,
		Enabled:   p.Enabled,
	}
}

func (p RelayProfileRecord) SafeMap(status string) map[string]any {
	return map[string]any{
		"id":               p.ID,
		"relay_profile_id": p.ID,
		"type":             p.Type,
		"name":             p.Name,
		"url":              p.URL,
		"enabled":          p.Enabled,
		"status":           firstNonEmpty(status, p.Status),
		"token_configured": p.TokenEnv != "" || p.TokenFile != "",
	}
}

func defaultGatewayScopes() []string {
	return []string{"projects:read", "projects:write", "runs:read", "runs:write", "steps:read", "steps:write", "artifacts:read", "reports:read"}
}

func sanitizeRelayProfileID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-'
		if !ok {
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
			continue
		}
		b.WriteRune(r)
		lastDash = r == '-'
	}
	return strings.Trim(b.String(), "-_")
}

func randomStoreID(prefix string) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(buf), nil
}

func randomUserCode() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	chars := make([]byte, 8)
	for i := range chars {
		chars[i] = alphabet[int(buf[i%len(buf)])%len(alphabet)]
	}
	return string(chars[:4]) + "-" + string(chars[4:]), nil
}

func scanUser(row interface{ Scan(dest ...any) error }, user *User) error {
	var created, updated string
	if err := row.Scan(&user.ID, &user.Email, &user.DisplayName, &created, &updated); err != nil {
		return err
	}
	user.CreatedAt = parseStoreTime(created)
	user.UpdatedAt = parseStoreTime(updated)
	return nil
}

func scanWorkspace(row interface{ Scan(dest ...any) error }, workspace *Workspace) error {
	var created, updated string
	if err := row.Scan(&workspace.ID, &workspace.Name, &workspace.OwnerUserID, &workspace.Kind, &created, &updated); err != nil {
		return err
	}
	workspace.CreatedAt = parseStoreTime(created)
	workspace.UpdatedAt = parseStoreTime(updated)
	return nil
}

func scanRelayProfile(row interface{ Scan(dest ...any) error }) (RelayProfileRecord, error) {
	var profile RelayProfileRecord
	var enabled int
	var created, updated string
	if err := row.Scan(&profile.ID, &profile.WorkspaceID, &profile.Type, &profile.Name, &profile.URL, &profile.TokenEnv, &profile.TokenFile, &enabled, &profile.Status, &created, &updated); err != nil {
		return RelayProfileRecord{}, err
	}
	profile.Enabled = enabled != 0
	profile.CreatedAt = parseStoreTime(created)
	profile.UpdatedAt = parseStoreTime(updated)
	return profile, nil
}

func scanRunRecord(row interface{ Scan(dest ...any) error }) (RunRecord, error) {
	var record RunRecord
	var started, completed, created, updated, reportJSON string
	if err := row.Scan(
		&record.ID,
		&record.WorkspaceID,
		&record.ActorUserID,
		&record.ProjectID,
		&record.ProjectName,
		&record.RunID,
		&record.StepID,
		&record.Title,
		&record.Goal,
		&record.Mode,
		&record.ExecutorProfile,
		&record.Status,
		&record.ReportStatus,
		&record.ResultSummary,
		&record.ResultDetails,
		&record.RelayProfileID,
		&record.ConnectorID,
		&record.MachineID,
		&record.HostLabel,
		&started,
		&completed,
		&created,
		&updated,
		&reportJSON,
	); err != nil {
		return RunRecord{}, err
	}
	record.StartedAt = parseStoreTime(started)
	record.CompletedAt = parseStoreTime(completed)
	record.CreatedAt = parseStoreTime(created)
	record.UpdatedAt = parseStoreTime(updated)
	record.Report = decodeStoreMap(reportJSON)
	return record, nil
}

func decodeStoreMap(value string) map[string]any {
	out := map[string]any{}
	if strings.TrimSpace(value) == "" {
		return out
	}
	_ = json.Unmarshal([]byte(value), &out)
	if out == nil {
		return map[string]any{}
	}
	return out
}

func (s *Store) deviceByUserCode(ctx context.Context, userCode string) (DeviceCodeRecord, error) {
	row := s.db.QueryRowContext(ctx, `SELECT device_code_hash, user_code_hash, user_code, status, email, display_name, user_id, workspace_id, expires_at, created_at, updated_at FROM device_codes WHERE user_code_hash = ?`, tokenHash(userCode))
	return scanDeviceCode(row)
}

func (s *Store) deviceByDeviceCode(ctx context.Context, deviceCode string) (DeviceCodeRecord, error) {
	row := s.db.QueryRowContext(ctx, `SELECT device_code_hash, user_code_hash, user_code, status, email, display_name, user_id, workspace_id, expires_at, created_at, updated_at FROM device_codes WHERE device_code_hash = ?`, tokenHash(deviceCode))
	return scanDeviceCode(row)
}

func scanDeviceCode(row interface{ Scan(dest ...any) error }) (DeviceCodeRecord, error) {
	var rec DeviceCodeRecord
	var expires, created, updated string
	if err := row.Scan(&rec.DeviceCodeHash, &rec.UserCodeHash, &rec.UserCode, &rec.Status, &rec.Email, &rec.DisplayName, &rec.UserID, &rec.WorkspaceID, &expires, &created, &updated); err != nil {
		return DeviceCodeRecord{}, err
	}
	rec.ExpiresAt = parseStoreTime(expires)
	rec.CreatedAt = parseStoreTime(created)
	rec.UpdatedAt = parseStoreTime(updated)
	return rec, nil
}

func formatStoreTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func parseStoreTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339Nano, value)
	return t
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func sortRelayProfiles(profiles []RelayProfileRecord) []RelayProfileRecord {
	out := append([]RelayProfileRecord(nil), profiles...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
