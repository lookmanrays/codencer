package cloud

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

var (
	ErrAPITokenNotFound  = errors.New("cloud api token not found")
	ErrAPITokenInvalid   = errors.New("cloud api token is invalid")
	ErrSecretBoxRequired = errors.New("cloud master key is required")
)

const apiTokenPrefix = "cct_"

var roleScopeAllowlist = map[string][]string{
	RoleOrgOwner: {
		"*",
	},
	RoleOrgAdmin: {
		"cloud:read",
		"orgs:read", "workspaces:read", "projects:read",
		"workspaces:write", "projects:write",
		"memberships:read", "memberships:write",
		"tokens:read", "tokens:write",
		"installations:read", "installations:write",
		"runtime_connectors:read", "runtime_connectors:write",
		"runtime_instances:read",
		"runs:read", "runs:write",
		"steps:read", "steps:write",
		"artifacts:read",
		"gates:read", "gates:write",
		"events:read", "audit:read",
	},
	RoleWorkspaceAdmin: {
		"cloud:read",
		"workspaces:read", "projects:read", "projects:write",
		"memberships:read", "memberships:write",
		"tokens:read", "tokens:write",
		"installations:read", "installations:write",
		"runtime_connectors:read", "runtime_connectors:write",
		"runtime_instances:read",
		"runs:read", "runs:write",
		"steps:read", "steps:write",
		"artifacts:read",
		"gates:read", "gates:write",
		"events:read", "audit:read",
	},
	RoleProjectOperator: {
		"cloud:read",
		"projects:read",
		"memberships:read",
		"tokens:read", "tokens:write",
		"installations:read", "installations:write",
		"runtime_connectors:read", "runtime_connectors:write",
		"runtime_instances:read",
		"runs:read", "runs:write",
		"steps:read", "steps:write",
		"artifacts:read",
		"gates:read", "gates:write",
		"events:read", "audit:read",
	},
	RoleProjectViewer: {
		"cloud:read",
		"projects:read",
		"memberships:read",
		"tokens:read",
		"installations:read",
		"runtime_connectors:read",
		"runtime_instances:read",
		"runs:read",
		"steps:read",
		"artifacts:read",
		"gates:read",
		"events:read", "audit:read",
	},
}

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
	if token.Role != "" && !roleAllowsScope(token.Role, required) {
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
	if token.MembershipWorkspaceID != "" && workspaceID != "" && token.MembershipWorkspaceID != workspaceID {
		return false
	}
	if token.MembershipWorkspaceID != "" && workspaceID == "" && projectID == "" {
		return false
	}
	if token.MembershipProjectID != "" && token.MembershipProjectID != projectID {
		return false
	}
	return true
}

func MembershipAllowsTarget(membership *Membership, orgID, workspaceID, projectID string) bool {
	if membership == nil {
		return false
	}
	if membership.OrgID != "" && orgID != "" && membership.OrgID != orgID {
		return false
	}
	if membership.WorkspaceID != "" && workspaceID != "" && membership.WorkspaceID != workspaceID {
		return false
	}
	if membership.ProjectID != "" && projectID != "" && membership.ProjectID != projectID {
		return false
	}
	if membership.WorkspaceID != "" && workspaceID == "" && projectID == "" {
		return false
	}
	if membership.ProjectID != "" && projectID == "" {
		return false
	}
	return true
}

func RoleAllowedScopes(role string) []string {
	values := roleScopeAllowlist[strings.TrimSpace(role)]
	if len(values) == 0 {
		return nil
	}
	return append([]string(nil), values...)
}

func ClampScopesForRole(role string, requested []string) ([]string, error) {
	if roleAllowsScope(role, "*") {
		if len(requested) == 0 {
			return []string{"*"}, nil
		}
		return uniqueScopes(requested), nil
	}
	allowed := RoleAllowedScopes(role)
	if len(allowed) == 0 {
		return nil, fmt.Errorf("unsupported membership role %q", role)
	}
	if len(requested) == 0 {
		return allowed, nil
	}
	filtered := make([]string, 0, len(requested))
	for _, scope := range uniqueScopes(requested) {
		if !scopeAllowed(allowed, scope) {
			return nil, fmt.Errorf("role %q is not allowed to grant scope %q", role, scope)
		}
		filtered = append(filtered, scope)
	}
	return filtered, nil
}

func roleAllowsScope(role, required string) bool {
	allowed := roleScopeAllowlist[strings.TrimSpace(role)]
	return scopeAllowed(allowed, required)
}

func uniqueScopes(values []string) []string {
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		seen[value] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for value := range maps.Keys(seen) {
		out = append(out, value)
	}
	slices.Sort(out)
	return out
}

func scopeAllowed(scopes []string, required string) bool {
	if required == "" {
		return true
	}
	for _, scope := range scopes {
		if scope == "*" || scope == required {
			return true
		}
		if strings.HasSuffix(scope, ":*") {
			prefix := strings.TrimSuffix(scope, "*")
			if strings.HasPrefix(required, prefix) {
				return true
			}
		}
	}
	return false
}
