package account

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	SessionVersion    = 1
	DefaultGatewayURL = "https://mcp.codencer.dev"
)

type Session struct {
	Version     int       `json:"version"`
	GatewayURL  string    `json:"gateway_url"`
	MCPURL      string    `json:"mcp_url"`
	UserID      string    `json:"user_id"`
	WorkspaceID string    `json:"workspace_id"`
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	Scopes      []string  `json:"scopes"`
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type SafeSession struct {
	Version         int       `json:"version"`
	GatewayURL      string    `json:"gateway_url"`
	MCPURL          string    `json:"mcp_url"`
	UserID          string    `json:"user_id"`
	WorkspaceID     string    `json:"workspace_id"`
	TokenType       string    `json:"token_type"`
	Scopes          []string  `json:"scopes"`
	ExpiresAt       time.Time `json:"expires_at"`
	TokenConfigured bool      `json:"token_configured"`
	SessionPath     string    `json:"session_path,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func SessionPath(home string) string {
	return filepath.Join(strings.TrimSpace(home), "session.json")
}

func NormalizeGatewayURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = DefaultGatewayURL
	}
	value = strings.TrimRight(value, "/")
	value = strings.TrimSuffix(value, "/mcp")
	return strings.TrimRight(value, "/")
}

func NewSession(gatewayURL string, token TokenResponse, now time.Time) Session {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	scopes := strings.Fields(token.Scope)
	return Session{
		Version:     SessionVersion,
		GatewayURL:  NormalizeGatewayURL(gatewayURL),
		MCPURL:      strings.TrimRight(token.Resource, "/"),
		UserID:      token.UserID,
		WorkspaceID: token.WorkspaceID,
		AccessToken: token.AccessToken,
		TokenType:   firstNonEmpty(token.TokenType, "Bearer"),
		Scopes:      scopes,
		ExpiresAt:   token.ExpiresAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func LoadSession(path string) (Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Session{}, err
	}
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return Session{}, fmt.Errorf("decode Codencer session: %w", err)
	}
	session = normalizeSession(session)
	return session, nil
}

func SaveSession(path string, session Session) error {
	session = normalizeSession(session)
	if strings.TrimSpace(session.AccessToken) == "" {
		return fmt.Errorf("session access token is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create session parent: %w", err)
	}
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Codencer session: %w", err)
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-session-*.json")
	if err != nil {
		return fmt.Errorf("create session temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("write session temp file: %w", err)
	}
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("chmod session temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close session temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("replace session file: %w", err)
	}
	return nil
}

func RemoveSession(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s Session) Safe(path string) SafeSession {
	normalized := normalizeSession(s)
	return SafeSession{
		Version:         normalized.Version,
		GatewayURL:      normalized.GatewayURL,
		MCPURL:          normalized.MCPURL,
		UserID:          normalized.UserID,
		WorkspaceID:     normalized.WorkspaceID,
		TokenType:       normalized.TokenType,
		Scopes:          append([]string(nil), normalized.Scopes...),
		ExpiresAt:       normalized.ExpiresAt,
		TokenConfigured: normalized.AccessToken != "",
		SessionPath:     path,
		CreatedAt:       normalized.CreatedAt,
		UpdatedAt:       normalized.UpdatedAt,
	}
}

func normalizeSession(session Session) Session {
	if session.Version == 0 {
		session.Version = SessionVersion
	}
	session.GatewayURL = NormalizeGatewayURL(session.GatewayURL)
	session.MCPURL = strings.TrimRight(session.MCPURL, "/")
	if session.MCPURL == "" {
		session.MCPURL = session.GatewayURL + "/mcp"
	}
	if session.TokenType == "" {
		session.TokenType = "Bearer"
	}
	if session.CreatedAt.IsZero() {
		session.CreatedAt = time.Now().UTC()
	}
	if session.UpdatedAt.IsZero() {
		session.UpdatedAt = session.CreatedAt
	}
	return session
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
