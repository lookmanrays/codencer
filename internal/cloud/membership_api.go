package cloud

import (
	"net/http"
	"strings"
	"time"
)

func (s *Server) handleMemberships(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		token, ok := s.requireToken(w, r, "memberships:read")
		if !ok {
			return
		}
		orgID := firstNonEmpty(r.URL.Query().Get("org_id"), token.OrgID)
		workspaceID := firstNonEmpty(r.URL.Query().Get("workspace_id"), token.WorkspaceID)
		projectID := firstNonEmpty(r.URL.Query().Get("project_id"), token.ProjectID)
		if !TokenAllowsTarget(token, orgID, workspaceID, projectID) {
			writeAPIError(w, http.StatusForbidden, "scope_denied", "token is not allowed to read these memberships")
			return
		}
		memberships, err := s.store.ListMemberships(r.Context(), orgID, workspaceID, projectID)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "list_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, memberships)
	case http.MethodPost:
		token, ok := s.requireToken(w, r, "memberships:write")
		if !ok {
			return
		}
		var req Membership
		if err := decodeJSON(r.Body, &req); err != nil {
			writeAPIError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		if !TokenAllowsTarget(token, req.OrgID, req.WorkspaceID, req.ProjectID) {
			writeAPIError(w, http.StatusForbidden, "scope_denied", "token is not allowed to manage this membership scope")
			return
		}
		if !canManageMembershipRole(token, req.Role, req.WorkspaceID, req.ProjectID) {
			writeAPIError(w, http.StatusForbidden, "role_denied", "token is not allowed to assign this membership role")
			return
		}
		req.Status = DefaultMembershipStatus
		req.DisabledAt = nil
		membership, err := s.store.CreateMembership(r.Context(), req)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "create_failed", err.Error())
			return
		}
		s.recordAudit(r, token, "create_membership", "membership", membership.ID, membership.OrgID, membership.WorkspaceID, membership.ProjectID, "ok", map[string]any{
			"name": membership.Name,
			"role": membership.Role,
		})
		writeJSON(w, http.StatusCreated, membership)
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (s *Server) handleMembershipByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/cloud/v1/memberships/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	membershipID := parts[0]
	sub := ""
	if len(parts) > 1 {
		sub = parts[1]
	}
	switch {
	case sub == "" && r.Method == http.MethodGet:
		token, ok := s.requireToken(w, r, "memberships:read")
		if !ok {
			return
		}
		membership, err := s.store.GetMembership(r.Context(), membershipID)
		if err != nil {
			writeAPIError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		if !TokenAllowsTarget(token, membership.OrgID, membership.WorkspaceID, membership.ProjectID) {
			writeAPIError(w, http.StatusForbidden, "scope_denied", "token is not allowed to read this membership")
			return
		}
		writeJSON(w, http.StatusOK, membership)
	case (sub == "enable" || sub == "disable") && r.Method == http.MethodPost:
		token, ok := s.requireToken(w, r, "memberships:write")
		if !ok {
			return
		}
		membership, err := s.store.GetMembership(r.Context(), membershipID)
		if err != nil {
			writeAPIError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		if !TokenAllowsTarget(token, membership.OrgID, membership.WorkspaceID, membership.ProjectID) || !canManageMembershipRole(token, membership.Role, membership.WorkspaceID, membership.ProjectID) {
			writeAPIError(w, http.StatusForbidden, "scope_denied", "token is not allowed to update this membership")
			return
		}
		if sub == "enable" {
			membership.Status = DefaultMembershipStatus
			membership.DisabledAt = nil
		} else {
			now := time.Now().UTC()
			membership.Status = "disabled"
			membership.DisabledAt = &now
		}
		membership, err = s.store.CreateMembership(r.Context(), *membership)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "update_failed", err.Error())
			return
		}
		action := "disable_membership"
		if sub == "enable" {
			action = "enable_membership"
		}
		s.recordAudit(r, token, action, "membership", membership.ID, membership.OrgID, membership.WorkspaceID, membership.ProjectID, "ok", map[string]any{"role": membership.Role})
		writeJSON(w, http.StatusOK, membership)
	default:
		http.NotFound(w, r)
	}
}

func canManageMembershipRole(token *APIToken, targetRole, workspaceID, projectID string) bool {
	if token == nil {
		return false
	}
	switch token.Role {
	case "":
		return TokenHasScope(token, "cloud:admin")
	case RoleOrgOwner:
		return true
	case RoleOrgAdmin:
		return targetRole != RoleOrgOwner
	case RoleWorkspaceAdmin:
		return workspaceID != "" && projectID != "" && (targetRole == RoleProjectOperator || targetRole == RoleProjectViewer)
	default:
		return false
	}
}
