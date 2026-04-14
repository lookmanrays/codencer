package cloud

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
)

var (
	ErrAPITokenNotFound  = errors.New("cloud api token not found")
	ErrAPITokenInvalid   = errors.New("cloud api token is invalid")
	ErrSecretBoxRequired = errors.New("cloud master key is required")
)

const apiTokenPrefix = "cct_"

// GenerateAPIToken creates a new opaque bearer token suitable for cloud auth.
func GenerateAPIToken() (string, error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("generate api token: %w", err)
	}
	return apiTokenPrefix + base64.RawURLEncoding.EncodeToString(buf[:]), nil
}

// HashAPIToken returns the deterministic storage hash for a raw bearer token.
func HashAPIToken(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:])
}

// TokenPrefix gives operators a short, non-sensitive preview of a token.
func TokenPrefix(raw string) string {
	raw = strings.TrimSpace(raw)
	if len(raw) <= 8 {
		return raw
	}
	return raw[:8]
}

// VerifyAPIToken compares a raw token to a stored hash using constant-time compare.
func VerifyAPIToken(raw, storedHash string) bool {
	return subtle.ConstantTimeCompare([]byte(HashAPIToken(raw)), []byte(strings.TrimSpace(storedHash))) == 1
}

func TokenHasScope(token *APIToken, required string) bool {
	if token == nil {
		return false
	}
	if required == "" {
		return true
	}
	if slices.Contains(token.Scopes, "*") || slices.Contains(token.Scopes, "cloud:admin") {
		return true
	}
	return slices.Contains(token.Scopes, required)
}

func TokenAllowsTarget(token *APIToken, orgID, workspaceID, projectID string) bool {
	if token == nil {
		return false
	}
	if token.OrgID != "" && orgID != "" && token.OrgID != orgID {
		return false
	}
	if token.WorkspaceID != "" && workspaceID != "" && token.WorkspaceID != workspaceID {
		return false
	}
	if token.ProjectID != "" && projectID != "" && token.ProjectID != projectID {
		return false
	}
	if token.WorkspaceID != "" && workspaceID == "" && projectID == "" {
		return false
	}
	if token.ProjectID != "" && projectID == "" {
		return false
	}
	return true
}
