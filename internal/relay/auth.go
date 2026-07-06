package relay

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
)

type plannerPrincipal struct {
	Name        string
	TokenHash   string
	Scopes      []string
	InstanceIDs map[string]struct{}
	ProjectIDs  map[string]struct{}
}

type plannerPrincipalKey struct{}

func (s *Server) withPlannerScope(scope string, instanceIDFromRequest func(*http.Request) string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		instanceID := ""
		if instanceIDFromRequest != nil {
			instanceID = instanceIDFromRequest(r)
		}
		principal := plannerFromContext(r.Context())
		if principal != nil {
			if err := authorizePrincipal(principal, scope, instanceID); err != nil {
				s.addPlannerAuthChallenge(w, r, scope)
				writeAPIError(w, err.Status, err.Code, err.Message)
				return
			}
			next(w, r)
			return
		}
		principal, err := s.authenticatePlanner(r, scope, instanceID)
		if err != nil {
			s.addPlannerAuthChallenge(w, r, scope)
			writeAPIError(w, err.Status, err.Code, err.Message)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), plannerPrincipalKey{}, principal)))
	}
}

func plannerFromContext(ctx context.Context) *plannerPrincipal {
	principal, _ := ctx.Value(plannerPrincipalKey{}).(*plannerPrincipal)
	return principal
}

func (s *Server) authenticatePlanner(r *http.Request, requiredScope, instanceID string) (*plannerPrincipal, *apiError) {
	token := bearerToken(r.Header.Get("Authorization"))
	if token == "" {
		if principal := s.devNoAuthPrincipal(); principal != nil {
			if err := authorizePrincipal(principal, requiredScope, instanceID); err != nil {
				return nil, err
			}
			return principal, nil
		}
		return nil, &apiError{Status: http.StatusUnauthorized, Code: "auth_failed", Message: "planner bearer token required"}
	}
	for _, candidate := range s.cfg.PlannerTokens {
		if token != candidate.Token {
			continue
		}
		principal := &plannerPrincipal{
			Name:        candidate.Name,
			TokenHash:   plannerTokenHash(token),
			Scopes:      candidate.Scopes,
			InstanceIDs: make(map[string]struct{}),
			ProjectIDs:  make(map[string]struct{}),
		}
		for _, allowed := range candidate.InstanceIDs {
			principal.InstanceIDs[allowed] = struct{}{}
		}
		for _, allowed := range candidate.ProjectIDs {
			principal.ProjectIDs[allowed] = struct{}{}
		}
		if err := authorizePrincipal(principal, requiredScope, instanceID); err != nil {
			return nil, err
		}
		return principal, nil
	}
	if s.oauthDev != nil {
		principal, apiErr := s.oauthDev.Authenticate(token)
		if apiErr == nil && principal != nil {
			if err := authorizePrincipal(principal, requiredScope, instanceID); err != nil {
				return nil, err
			}
			return principal, nil
		}
	}
	return nil, &apiError{Status: http.StatusUnauthorized, Code: "auth_failed", Message: "planner authorization failed"}
}

func (s *Server) devNoAuthPrincipal() *plannerPrincipal {
	if s == nil || s.cfg == nil || !s.cfg.ChatGPTDevNoAuth.Enabled {
		return nil
	}
	principal := &plannerPrincipal{
		Name:        "chatgpt-dev-noauth",
		TokenHash:   plannerTokenHash("chatgpt-dev-noauth"),
		Scopes:      append([]string(nil), s.cfg.ChatGPTDevNoAuth.Scopes...),
		InstanceIDs: make(map[string]struct{}),
		ProjectIDs:  make(map[string]struct{}),
	}
	for _, projectID := range s.cfg.ChatGPTDevNoAuth.ProjectIDs {
		principal.ProjectIDs[projectID] = struct{}{}
	}
	return principal
}

func bearerToken(header string) string {
	header = strings.TrimSpace(header)
	if len(header) < len("Bearer ") || !strings.EqualFold(header[:len("Bearer ")], "Bearer ") {
		return ""
	}
	return strings.TrimSpace(header[len("Bearer "):])
}

func plannerTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func authorizePrincipal(principal *plannerPrincipal, requiredScope, instanceID string) *apiError {
	if principal == nil {
		return &apiError{Status: http.StatusUnauthorized, Code: "auth_failed", Message: "planner authorization required"}
	}
	if !scopeAllowed(principal.Scopes, requiredScope) {
		return &apiError{Status: http.StatusForbidden, Code: "scope_denied", Message: "planner token lacks required scope"}
	}
	if instanceID != "" && len(principal.InstanceIDs) > 0 {
		if _, ok := principal.InstanceIDs[instanceID]; !ok {
			return &apiError{Status: http.StatusForbidden, Code: "instance_denied", Message: "planner token is not authorized for this instance"}
		}
	}
	return nil
}

func authorizeProject(principal *plannerPrincipal, requiredScope string, project *ProjectRecord) *apiError {
	if project == nil {
		return &apiError{Status: http.StatusNotFound, Code: "project_not_found", Message: "project not found"}
	}
	if err := authorizePrincipal(principal, requiredScope, project.InstanceID); err != nil {
		return err
	}
	if len(principal.ProjectIDs) > 0 {
		if _, ok := principal.ProjectIDs[project.ProjectID]; !ok {
			return &apiError{Status: http.StatusForbidden, Code: "project_denied", Message: "planner token is not authorized for this project"}
		}
	}
	return nil
}

func projectAllowed(principal *plannerPrincipal, requiredScope string, project ProjectRecord) bool {
	return authorizeProject(principal, requiredScope, &project) == nil
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

// ServeAsPlanner routes an HTTP request through the relay using an injected
// planner principal. This is intended for trusted in-process callers such as the
// composed cloud control plane; it does not change the public relay auth model.
func (s *Server) ServeAsPlanner(w http.ResponseWriter, r *http.Request, name string, scopes []string, instanceIDs []string) {
	if s == nil || s.server == nil || s.server.Handler == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "relay_unavailable", "relay handler is not available")
		return
	}
	principal := &plannerPrincipal{
		Name:        name,
		Scopes:      append([]string(nil), scopes...),
		InstanceIDs: make(map[string]struct{}, len(instanceIDs)),
		ProjectIDs:  make(map[string]struct{}),
	}
	for _, instanceID := range instanceIDs {
		instanceID = strings.TrimSpace(instanceID)
		if instanceID == "" {
			continue
		}
		principal.InstanceIDs[instanceID] = struct{}{}
	}
	s.server.Handler.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), plannerPrincipalKey{}, principal)))
}
