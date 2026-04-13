package relaystore

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

type InstanceRecord struct {
	InstanceID   string    `json:"instance_id"`
	ConnectorID  string    `json:"connector_id"`
	RepoRoot     string    `json:"repo_root"`
	BaseURL      string    `json:"base_url"`
	InstanceJSON string    `json:"instance_json,omitempty"`
	LastSeenAt   time.Time `json:"last_seen_at"`
}

type ConnectorRecord struct {
	ConnectorID         string
	MachineID           string
	PublicKey           string
	Label               string
	MachineMetadataJSON string
	Disabled            bool
	CreatedAt           time.Time
	UpdatedAt           time.Time
	LastSeenAt          time.Time
}

type EnrollmentTokenRecord struct {
	TokenID     string
	TokenHash   string
	Label       string
	CreatedBy   string
	CreatedAt   time.Time
	ExpiresAt   *time.Time
	ConsumedAt  *time.Time
	ConnectorID string
}

type ChallengeRecord struct {
	ChallengeID string
	ConnectorID string
	MachineID   string
	Nonce       string
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

type AuditEvent struct {
	ID                int64     `json:"id"`
	ActorType         string    `json:"actor_type"`
	ActorID           string    `json:"actor_id,omitempty"`
	Action            string    `json:"action"`
	Method            string    `json:"method,omitempty"`
	Scope             string    `json:"scope,omitempty"`
	ResourceKind      string    `json:"resource_kind,omitempty"`
	ResourceID        string    `json:"resource_id,omitempty"`
	TargetConnectorID string    `json:"target_connector_id,omitempty"`
	TargetInstanceID  string    `json:"target_instance_id,omitempty"`
	Outcome           string    `json:"outcome"`
	ErrorCode         string    `json:"error_code,omitempty"`
	Details           string    `json:"details,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", path+"?_journal=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS connectors (
  connector_id TEXT PRIMARY KEY,
  machine_id TEXT NOT NULL,
  public_key TEXT NOT NULL,
  label TEXT,
  machine_metadata_json TEXT,
  disabled INTEGER NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  last_seen_at DATETIME
);
CREATE TABLE IF NOT EXISTS enrollment_tokens (
  token_id TEXT PRIMARY KEY,
  token_hash TEXT NOT NULL UNIQUE,
  label TEXT,
  created_by TEXT,
  created_at DATETIME NOT NULL,
  expires_at DATETIME,
  consumed_at DATETIME,
  connector_id TEXT
);
CREATE TABLE IF NOT EXISTS connector_challenges (
  challenge_id TEXT PRIMARY KEY,
  connector_id TEXT NOT NULL,
  machine_id TEXT NOT NULL,
  nonce TEXT NOT NULL,
  created_at DATETIME NOT NULL,
  expires_at DATETIME NOT NULL
);
CREATE TABLE IF NOT EXISTS instances (
  instance_id TEXT PRIMARY KEY,
  connector_id TEXT NOT NULL,
  repo_root TEXT NOT NULL,
  base_url TEXT NOT NULL,
  instance_json TEXT,
  last_seen_at DATETIME NOT NULL
);
CREATE TABLE IF NOT EXISTS resource_routes (
  resource_kind TEXT NOT NULL,
  resource_id TEXT NOT NULL,
  instance_id TEXT NOT NULL,
  updated_at DATETIME NOT NULL,
  PRIMARY KEY (resource_kind, resource_id)
);
CREATE TABLE IF NOT EXISTS audit_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  actor_type TEXT NOT NULL,
  actor_id TEXT,
  action TEXT NOT NULL,
  method TEXT,
  scope TEXT,
  resource_kind TEXT,
  resource_id TEXT,
  target_connector_id TEXT,
  target_instance_id TEXT,
  outcome TEXT NOT NULL,
  error_code TEXT,
  details TEXT,
  created_at DATETIME NOT NULL
);`
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}
	extras := []string{
		"ALTER TABLE connectors ADD COLUMN machine_id TEXT",
		"ALTER TABLE connectors ADD COLUMN public_key TEXT",
		"ALTER TABLE connectors ADD COLUMN label TEXT",
		"ALTER TABLE connectors ADD COLUMN machine_metadata_json TEXT",
		"ALTER TABLE connectors ADD COLUMN disabled INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE connectors ADD COLUMN updated_at DATETIME",
		"ALTER TABLE connectors ADD COLUMN last_seen_at DATETIME",
		"ALTER TABLE instances ADD COLUMN instance_json TEXT",
		"ALTER TABLE audit_events ADD COLUMN actor_type TEXT",
		"ALTER TABLE audit_events ADD COLUMN actor_id TEXT",
		"ALTER TABLE audit_events ADD COLUMN method TEXT",
		"ALTER TABLE audit_events ADD COLUMN scope TEXT",
		"ALTER TABLE audit_events ADD COLUMN target_connector_id TEXT",
		"ALTER TABLE audit_events ADD COLUMN target_instance_id TEXT",
		"ALTER TABLE audit_events ADD COLUMN outcome TEXT",
		"ALTER TABLE audit_events ADD COLUMN error_code TEXT",
	}
	for _, stmt := range extras {
		_, _ = s.db.Exec(stmt)
	}
	return nil
}

func (s *Store) SaveConnector(ctx context.Context, connectorID, machineID, publicKey, label string) error {
	return s.SaveConnectorRecord(ctx, ConnectorRecord{
		ConnectorID: connectorID,
		MachineID:   machineID,
		PublicKey:   publicKey,
		Label:       label,
	})
}

func (s *Store) SaveConnectorRecord(ctx context.Context, record ConnectorRecord) error {
	now := time.Now().UTC()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	record.UpdatedAt = now
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO connectors (connector_id, machine_id, public_key, label, machine_metadata_json, disabled, created_at, updated_at, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(connector_id) DO UPDATE SET
			machine_id = excluded.machine_id,
			public_key = excluded.public_key,
			label = excluded.label,
			machine_metadata_json = excluded.machine_metadata_json,
			disabled = excluded.disabled,
			updated_at = excluded.updated_at,
			last_seen_at = excluded.last_seen_at
	`, record.ConnectorID, record.MachineID, record.PublicKey, record.Label, record.MachineMetadataJSON, boolToInt(record.Disabled), record.CreatedAt, record.UpdatedAt, nullTime(record.LastSeenAt))
	return err
}

func (s *Store) GetConnector(ctx context.Context, connectorID string) (*ConnectorRecord, error) {
	record := &ConnectorRecord{}
	var disabled int
	var lastSeen sql.NullTime
	if err := s.db.QueryRowContext(ctx, `
		SELECT connector_id, machine_id, public_key, label, machine_metadata_json, disabled, created_at, updated_at, last_seen_at
		FROM connectors WHERE connector_id = ?
	`, connectorID).Scan(
		&record.ConnectorID,
		&record.MachineID,
		&record.PublicKey,
		&record.Label,
		&record.MachineMetadataJSON,
		&disabled,
		&record.CreatedAt,
		&record.UpdatedAt,
		&lastSeen,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	record.Disabled = disabled != 0
	if lastSeen.Valid {
		record.LastSeenAt = lastSeen.Time
	}
	return record, nil
}

func (s *Store) ListConnectors(ctx context.Context) ([]ConnectorRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT connector_id, machine_id, public_key, label, machine_metadata_json, disabled, created_at, updated_at, last_seen_at
		FROM connectors
		ORDER BY connector_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []ConnectorRecord
	for rows.Next() {
		var record ConnectorRecord
		var disabled int
		var lastSeen sql.NullTime
		if err := rows.Scan(
			&record.ConnectorID,
			&record.MachineID,
			&record.PublicKey,
			&record.Label,
			&record.MachineMetadataJSON,
			&disabled,
			&record.CreatedAt,
			&record.UpdatedAt,
			&lastSeen,
		); err != nil {
			return nil, err
		}
		record.Disabled = disabled != 0
		if lastSeen.Valid {
			record.LastSeenAt = lastSeen.Time
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *Store) MarkConnectorSeen(ctx context.Context, connectorID string, seenAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE connectors SET last_seen_at = ?, updated_at = ? WHERE connector_id = ?`, seenAt.UTC(), time.Now().UTC(), connectorID)
	return err
}

func (s *Store) SetConnectorDisabled(ctx context.Context, connectorID string, disabled bool) error {
	_, err := s.db.ExecContext(ctx, `UPDATE connectors SET disabled = ?, updated_at = ? WHERE connector_id = ?`, boolToInt(disabled), time.Now().UTC(), connectorID)
	return err
}

func (s *Store) CreateEnrollmentToken(ctx context.Context, label, createdBy string, expiresAt *time.Time) (*EnrollmentTokenRecord, string, error) {
	tokenID, err := randomID("enroll")
	if err != nil {
		return nil, "", err
	}
	secret, err := randomID("secret")
	if err != nil {
		return nil, "", err
	}
	record := &EnrollmentTokenRecord{
		TokenID:   tokenID,
		TokenHash: hashSecret(secret),
		Label:     label,
		CreatedBy: createdBy,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: expiresAt,
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO enrollment_tokens (token_id, token_hash, label, created_by, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, record.TokenID, record.TokenHash, record.Label, record.CreatedBy, record.CreatedAt, nullableTime(record.ExpiresAt))
	if err != nil {
		return nil, "", err
	}
	return record, secret, nil
}

func (s *Store) ConsumeEnrollmentToken(ctx context.Context, secret, connectorID string) (*EnrollmentTokenRecord, error) {
	hash := hashSecret(secret)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	record := &EnrollmentTokenRecord{}
	var expiresAt sql.NullTime
	var consumedAt sql.NullTime
	var existingConnectorID sql.NullString
	queryErr := tx.QueryRowContext(ctx, `
		SELECT token_id, token_hash, label, created_by, created_at, expires_at, consumed_at, connector_id
		FROM enrollment_tokens WHERE token_hash = ?
	`, hash).Scan(&record.TokenID, &record.TokenHash, &record.Label, &record.CreatedBy, &record.CreatedAt, &expiresAt, &consumedAt, &existingConnectorID)
	if queryErr != nil {
		if queryErr == sql.ErrNoRows {
			err = fmt.Errorf("invalid enrollment token")
			return nil, err
		}
		err = queryErr
		return nil, err
	}
	if expiresAt.Valid {
		record.ExpiresAt = &expiresAt.Time
		if time.Now().UTC().After(expiresAt.Time) {
			err = fmt.Errorf("enrollment token expired")
			return nil, err
		}
	}
	if consumedAt.Valid {
		record.ConsumedAt = &consumedAt.Time
		err = fmt.Errorf("enrollment token already used")
		return nil, err
	}
	if existingConnectorID.Valid {
		record.ConnectorID = existingConnectorID.String
	}
	now := time.Now().UTC()
	record.ConsumedAt = &now
	record.ConnectorID = connectorID
	if _, execErr := tx.ExecContext(ctx, `UPDATE enrollment_tokens SET consumed_at = ?, connector_id = ? WHERE token_id = ?`, now, connectorID, record.TokenID); execErr != nil {
		err = execErr
		return nil, err
	}
	err = tx.Commit()
	return record, err
}

func (s *Store) SaveChallenge(ctx context.Context, record ChallengeRecord) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO connector_challenges (challenge_id, connector_id, machine_id, nonce, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(challenge_id) DO UPDATE SET connector_id = excluded.connector_id, machine_id = excluded.machine_id, nonce = excluded.nonce, created_at = excluded.created_at, expires_at = excluded.expires_at
	`, record.ChallengeID, record.ConnectorID, record.MachineID, record.Nonce, record.CreatedAt, record.ExpiresAt)
	return err
}

func (s *Store) ConsumeChallenge(ctx context.Context, challengeID string) (*ChallengeRecord, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	record := &ChallengeRecord{}
	queryErr := tx.QueryRowContext(ctx, `
		SELECT challenge_id, connector_id, machine_id, nonce, created_at, expires_at
		FROM connector_challenges WHERE challenge_id = ?
	`, challengeID).Scan(&record.ChallengeID, &record.ConnectorID, &record.MachineID, &record.Nonce, &record.CreatedAt, &record.ExpiresAt)
	if queryErr != nil {
		if queryErr == sql.ErrNoRows {
			err = nil
			return nil, nil
		}
		err = queryErr
		return nil, err
	}
	if _, execErr := tx.ExecContext(ctx, `DELETE FROM connector_challenges WHERE challenge_id = ?`, challengeID); execErr != nil {
		err = execErr
		return nil, err
	}
	err = tx.Commit()
	return record, err
}

func (s *Store) SaveInstance(ctx context.Context, record InstanceRecord) error {
	if record.LastSeenAt.IsZero() {
		record.LastSeenAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO instances (instance_id, connector_id, repo_root, base_url, instance_json, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(instance_id) DO UPDATE SET
			connector_id = excluded.connector_id,
			repo_root = excluded.repo_root,
			base_url = excluded.base_url,
			instance_json = excluded.instance_json,
			last_seen_at = excluded.last_seen_at
	`, record.InstanceID, record.ConnectorID, record.RepoRoot, record.BaseURL, record.InstanceJSON, record.LastSeenAt.UTC())
	return err
}

func (s *Store) GetInstance(ctx context.Context, instanceID string) (*InstanceRecord, error) {
	record := &InstanceRecord{}
	if err := s.db.QueryRowContext(ctx, `
		SELECT instance_id, connector_id, repo_root, base_url, instance_json, last_seen_at
		FROM instances WHERE instance_id = ?
	`, instanceID).Scan(&record.InstanceID, &record.ConnectorID, &record.RepoRoot, &record.BaseURL, &record.InstanceJSON, &record.LastSeenAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return record, nil
}

func (s *Store) TouchInstance(ctx context.Context, instanceID string, touchedAt ...time.Time) error {
	now := time.Now().UTC()
	if len(touchedAt) > 0 && !touchedAt[0].IsZero() {
		now = touchedAt[0].UTC()
	}
	_, err := s.db.ExecContext(ctx, `UPDATE instances SET last_seen_at = ? WHERE instance_id = ?`, now, instanceID)
	return err
}

func (s *Store) ListInstances(ctx context.Context) ([]InstanceRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT instance_id, connector_id, repo_root, base_url, instance_json, last_seen_at
		FROM instances
		ORDER BY instance_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []InstanceRecord
	for rows.Next() {
		var record InstanceRecord
		if err := rows.Scan(&record.InstanceID, &record.ConnectorID, &record.RepoRoot, &record.BaseURL, &record.InstanceJSON, &record.LastSeenAt); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *Store) ListInstancesByConnector(ctx context.Context, connectorID string) ([]InstanceRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT instance_id, connector_id, repo_root, base_url, instance_json, last_seen_at
		FROM instances
		WHERE connector_id = ?
		ORDER BY instance_id
	`, connectorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []InstanceRecord
	for rows.Next() {
		var record InstanceRecord
		if err := rows.Scan(&record.InstanceID, &record.ConnectorID, &record.RepoRoot, &record.BaseURL, &record.InstanceJSON, &record.LastSeenAt); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *Store) SaveResourceRoute(ctx context.Context, kind, id, instanceID string) error {
	if kind == "" || id == "" || instanceID == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO resource_routes (resource_kind, resource_id, instance_id, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(resource_kind, resource_id) DO UPDATE SET instance_id = excluded.instance_id, updated_at = excluded.updated_at
	`, kind, id, instanceID, time.Now().UTC())
	return err
}

func (s *Store) LookupResourceRoute(ctx context.Context, kind, id string) (string, error) {
	var instanceID string
	if err := s.db.QueryRowContext(ctx, `SELECT instance_id FROM resource_routes WHERE resource_kind = ? AND resource_id = ?`, kind, id).Scan(&instanceID); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return instanceID, nil
}

func (s *Store) AppendAudit(ctx context.Context, event AuditEvent) error {
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	if event.Outcome == "" {
		event.Outcome = "ok"
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO audit_events (
			actor_type, actor_id, action, method, scope, resource_kind, resource_id,
			target_connector_id, target_instance_id, outcome, error_code, details, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, event.ActorType, event.ActorID, event.Action, event.Method, event.Scope, event.ResourceKind, event.ResourceID, event.TargetConnectorID, event.TargetInstanceID, event.Outcome, event.ErrorCode, event.Details, event.CreatedAt)
	return err
}

func (s *Store) ListAuditEvents(ctx context.Context) ([]AuditEvent, error) {
	return s.ListAuditEventsLimit(ctx, 0)
}

func (s *Store) ListAuditEventsLimit(ctx context.Context, limit int) ([]AuditEvent, error) {
	query := `
		SELECT id, actor_type, actor_id, action, method, scope, resource_kind, resource_id,
		       target_connector_id, target_instance_id, outcome, error_code, details, created_at
		FROM audit_events
		ORDER BY id DESC
	`
	var rows *sql.Rows
	var err error
	if limit > 0 {
		rows, err = s.db.QueryContext(ctx, query+` LIMIT ?`, limit)
	} else {
		rows, err = s.db.QueryContext(ctx, query)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []AuditEvent
	for rows.Next() {
		var event AuditEvent
		if err := rows.Scan(&event.ID, &event.ActorType, &event.ActorID, &event.Action, &event.Method, &event.Scope, &event.ResourceKind, &event.ResourceID, &event.TargetConnectorID, &event.TargetInstanceID, &event.Outcome, &event.ErrorCode, &event.Details, &event.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func hashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func randomID(prefix string) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(buf)), nil
}

func nullableTime(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return value.UTC()
}

func nullTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value.UTC()
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
