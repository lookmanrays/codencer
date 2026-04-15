package relay

import (
	"context"
	"net/http"
	"strings"
)

type plannerPrincipal struct {
	Name        string
	Scopes      []string
	InstanceIDs map[string]struct{}
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
				writeAPIError(w, err.Status, err.Code, err.Message)
				return
			}
			next(w, r)
			return
		}
		principal, err := s.authenticatePlanner(r, scope, instanceID)
		if err != nil {
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
	token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if token == "" {
		return nil, &apiError{Status: http.StatusUnauthorized, Code: "auth_failed", Message: "planner bearer token required"}
	}
	for _, candidate := range s.cfg.PlannerTokens {
		if token != candidate.Token {
			continue
		}
		principal := &plannerPrincipal{
			Name:        candidate.Name,
			Scopes:      candidate.Scopes,
			InstanceIDs: make(map[string]struct{}),
		}
		for _, allowed := range candidate.InstanceIDs {
			principal.InstanceIDs[allowed] = struct{}{}
		}
		if err := authorizePrincipal(principal, requiredScope, instanceID); err != nil {
			return nil, err
		}
		return principal, nil
	}
	return nil, &apiError{Status: http.StatusUnauthorized, Code: "auth_failed", Message: "planner authorization failed"}
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
