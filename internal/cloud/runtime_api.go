package cloud

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func (s *Server) handleRuntimeConnectors(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		token, ok := s.requireToken(w, r, "runtime_connectors:read")
		if !ok {
			return
		}
		orgID := firstNonEmpty(r.URL.Query().Get("org_id"), token.OrgID)
		workspaceID := firstNonEmpty(r.URL.Query().Get("workspace_id"), token.WorkspaceID)
		projectID := firstNonEmpty(r.URL.Query().Get("project_id"), token.ProjectID)
		if !TokenAllowsTarget(token, orgID, workspaceID, projectID) {
			writeAPIError(w, http.StatusForbidden, "scope_denied", "token is not allowed to read these runtime connectors")
			return
		}
		if err := s.syncRuntimeScope(r.Context(), orgID, workspaceID, projectID); err != nil {
			writeAPIError(w, http.StatusInternalServerError, "runtime_sync_failed", err.Error())
			return
		}
		installations, err := s.store.ListRuntimeConnectorInstallations(r.Context(), orgID, workspaceID, projectID)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "list_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, installations)
	case http.MethodPost:
		token, ok := s.requireToken(w, r, "runtime_connectors:write")
		if !ok {
			return
		}
		if !s.requireRuntimeBridge(w) {
			return
		}
		var req struct {
			OrgID       string `json:"org_id"`
			WorkspaceID string `json:"workspace_id,omitempty"`
			ProjectID   string `json:"project_id,omitempty"`
			ConnectorID string `json:"connector_id"`
		}
		if err := decodeJSON(r.Body, &req); err != nil {
			writeAPIError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		if !TokenAllowsTarget(token, req.OrgID, req.WorkspaceID, req.ProjectID) {
			writeAPIError(w, http.StatusForbidden, "scope_denied", "token is not allowed to claim this runtime connector")
			return
		}
		existing, err := s.findRuntimeConnectorByConnectorID(r.Context(), req.ConnectorID)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "lookup_failed", err.Error())
			return
		}
		if existing != nil && (existing.OrgID != req.OrgID || existing.WorkspaceID != req.WorkspaceID || existing.ProjectID != req.ProjectID) {
			writeAPIError(w, http.StatusConflict, "connector_already_claimed", "runtime connector is already claimed by another scope")
			return
		}
		relayRecord, err := s.runtime.Store.GetConnector(r.Context(), strings.TrimSpace(req.ConnectorID))
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "relay_lookup_failed", err.Error())
			return
		}
		if relayRecord == nil {
			writeAPIError(w, http.StatusNotFound, "connector_not_found", "relay connector not found")
			return
		}
		installation := RuntimeConnectorInstallation{
			OrgID:       req.OrgID,
			WorkspaceID: req.WorkspaceID,
			ProjectID:   req.ProjectID,
			ConnectorID: relayRecord.ConnectorID,
			MachineID:   relayRecord.MachineID,
			Label:       relayRecord.Label,
			PublicKey:   relayRecord.PublicKey,
			Enabled:     true,
		}
		if existing != nil {
			installation = *existing
			installation.OrgID = req.OrgID
			installation.WorkspaceID = req.WorkspaceID
			installation.ProjectID = req.ProjectID
			installation.ConnectorID = relayRecord.ConnectorID
			installation.MachineID = relayRecord.MachineID
			installation.Label = relayRecord.Label
			installation.PublicKey = relayRecord.PublicKey
		}
		synced, _, err := s.syncRuntimeConnectorFromRelay(r.Context(), installation)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "runtime_sync_failed", err.Error())
			return
		}
		s.recordAudit(r, token, "claim_runtime_connector", "runtime_connector", synced.ID, synced.OrgID, synced.WorkspaceID, synced.ProjectID, "ok", map[string]any{"connector_id": synced.ConnectorID})
		writeJSON(w, http.StatusCreated, synced)
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (s *Server) handleRuntimeConnectorByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/cloud/v1/runtime/connectors/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	connectorRecordID := parts[0]
	sub := ""
	if len(parts) > 1 {
		sub = parts[1]
	}
	switch {
	case sub == "" && r.Method == http.MethodGet:
		token, ok := s.requireToken(w, r, "runtime_connectors:read")
		if !ok {
			return
		}
		installation, err := s.store.GetRuntimeConnectorInstallation(r.Context(), connectorRecordID)
		if err != nil {
			writeAPIError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		if !TokenAllowsTarget(token, installation.OrgID, installation.WorkspaceID, installation.ProjectID) {
			writeAPIError(w, http.StatusForbidden, "scope_denied", "token is not allowed to read this runtime connector")
			return
		}
		if _, _, err := s.syncRuntimeConnectorFromRelay(r.Context(), *installation); err == nil {
			installation, _ = s.store.GetRuntimeConnectorInstallation(r.Context(), connectorRecordID)
		}
		writeJSON(w, http.StatusOK, installation)
	case sub == "instances" && r.Method == http.MethodGet:
		token, ok := s.requireToken(w, r, "runtime_instances:read")
		if !ok {
			return
		}
		installation, err := s.store.GetRuntimeConnectorInstallation(r.Context(), connectorRecordID)
		if err != nil {
			writeAPIError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		if !TokenAllowsTarget(token, installation.OrgID, installation.WorkspaceID, installation.ProjectID) {
			writeAPIError(w, http.StatusForbidden, "scope_denied", "token is not allowed to read this runtime connector's instances")
			return
		}
		if _, _, err := s.syncRuntimeConnectorFromRelay(r.Context(), *installation); err != nil {
			writeAPIError(w, http.StatusInternalServerError, "runtime_sync_failed", err.Error())
			return
		}
		instances, err := s.store.ListRuntimeInstances(r.Context(), installation.OrgID, installation.WorkspaceID, installation.ProjectID, installation.ID)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "list_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, filterRuntimeInstances(instances, includeUnshared(r)))
	case (sub == "enable" || sub == "disable") && r.Method == http.MethodPost:
		token, ok := s.requireToken(w, r, "runtime_connectors:write")
		if !ok {
			return
		}
		if !s.requireRuntimeBridge(w) {
			return
		}
		installation, err := s.store.GetRuntimeConnectorInstallation(r.Context(), connectorRecordID)
		if err != nil {
			writeAPIError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		if !TokenAllowsTarget(token, installation.OrgID, installation.WorkspaceID, installation.ProjectID) {
			writeAPIError(w, http.StatusForbidden, "scope_denied", "token is not allowed to update this runtime connector")
			return
		}
		enabled := sub == "enable"
		installation.Enabled = enabled
		if err := s.runtime.Store.SetConnectorDisabled(r.Context(), installation.ConnectorID, !enabled); err != nil {
			writeAPIError(w, http.StatusInternalServerError, "relay_update_failed", err.Error())
			return
		}
		if _, _, err := s.syncRuntimeConnectorFromRelay(r.Context(), *installation); err != nil {
			writeAPIError(w, http.StatusInternalServerError, "runtime_sync_failed", err.Error())
			return
		}
		installation, _ = s.store.GetRuntimeConnectorInstallation(r.Context(), connectorRecordID)
		action := "disable_runtime_connector"
		if enabled {
			action = "enable_runtime_connector"
		}
		s.recordAudit(r, token, action, "runtime_connector", installation.ID, installation.OrgID, installation.WorkspaceID, installation.ProjectID, "ok", map[string]any{"connector_id": installation.ConnectorID, "enabled": enabled})
		writeJSON(w, http.StatusOK, installation)
	case sub == "sync" && r.Method == http.MethodPost:
		token, ok := s.requireToken(w, r, "runtime_connectors:write")
		if !ok {
			return
		}
		if !s.requireRuntimeBridge(w) {
			return
		}
		installation, err := s.store.GetRuntimeConnectorInstallation(r.Context(), connectorRecordID)
		if err != nil {
			writeAPIError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		if !TokenAllowsTarget(token, installation.OrgID, installation.WorkspaceID, installation.ProjectID) {
			writeAPIError(w, http.StatusForbidden, "scope_denied", "token is not allowed to sync this runtime connector")
			return
		}
		synced, instances, err := s.syncRuntimeConnectorFromRelay(r.Context(), *installation)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "runtime_sync_failed", err.Error())
			return
		}
		s.recordAudit(r, token, "sync_runtime_connector", "runtime_connector", synced.ID, synced.OrgID, synced.WorkspaceID, synced.ProjectID, "ok", map[string]any{"connector_id": synced.ConnectorID, "instance_count": len(instances)})
		writeJSON(w, http.StatusOK, map[string]any{
			"connector": synced,
			"instances": filterRuntimeInstances(instances, includeUnshared(r)),
		})
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleRuntimeInstances(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	token, ok := s.requireToken(w, r, "runtime_instances:read")
	if !ok {
		return
	}
	orgID := firstNonEmpty(r.URL.Query().Get("org_id"), token.OrgID)
	workspaceID := firstNonEmpty(r.URL.Query().Get("workspace_id"), token.WorkspaceID)
	projectID := firstNonEmpty(r.URL.Query().Get("project_id"), token.ProjectID)
	if !TokenAllowsTarget(token, orgID, workspaceID, projectID) {
		writeAPIError(w, http.StatusForbidden, "scope_denied", "token is not allowed to read these runtime instances")
		return
	}
	if err := s.syncRuntimeScope(r.Context(), orgID, workspaceID, projectID); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "runtime_sync_failed", err.Error())
		return
	}
	instances, err := s.store.ListRuntimeInstances(r.Context(), orgID, workspaceID, projectID, strings.TrimSpace(r.URL.Query().Get("runtime_connector_id")))
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, filterRuntimeInstances(instances, includeUnshared(r)))
}

func (s *Server) handleRuntimeInstanceByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/cloud/v1/runtime/instances/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	instanceID := parts[0]
	token, ok := s.requireToken(w, r, requiredRuntimeScope(parts, r.Method))
	if !ok {
		return
	}
	instance, connector, ok := s.loadAuthorizedRuntimeInstance(w, r, token, instanceID)
	if !ok {
		return
	}
	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{
			"instance":           instance,
			"runtime_connector": connector,
		})
	case len(parts) >= 2:
		s.proxyRuntimeInstanceOperation(w, r, token, *instance, parts[1:])
	default:
		http.NotFound(w, r)
	}
}

func requiredRuntimeScope(parts []string, method string) string {
	if len(parts) <= 1 {
		return "runtime_instances:read"
	}
	switch parts[1] {
	case "runs":
		if method == http.MethodGet {
			return "runs:read"
		}
		return "runs:write"
	case "steps":
		if len(parts) >= 4 && parts[3] == "artifacts" {
			return "artifacts:read"
		}
		return "steps:read"
	case "artifacts":
		return "artifacts:read"
	case "gates":
		return "gates:write"
	default:
		return "runtime_instances:read"
	}
}

func (s *Server) loadAuthorizedRuntimeInstance(w http.ResponseWriter, r *http.Request, token *APIToken, instanceID string) (*RuntimeInstance, *RuntimeConnectorInstallation, bool) {
	instance, err := s.store.GetRuntimeInstance(r.Context(), instanceID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, "not_found", err.Error())
		return nil, nil, false
	}
	connector, err := s.store.GetRuntimeConnectorInstallation(r.Context(), instance.RuntimeConnectorInstallationID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "runtime_connector_lookup_failed", err.Error())
		return nil, nil, false
	}
	if !TokenAllowsTarget(token, instance.OrgID, instance.WorkspaceID, instance.ProjectID) {
		writeAPIError(w, http.StatusForbidden, "scope_denied", "token is not allowed to access this runtime instance")
		return nil, nil, false
	}
	if connector != nil {
		if _, _, err := s.syncRuntimeConnectorFromRelay(r.Context(), *connector); err == nil {
			instance, _ = s.store.GetRuntimeInstance(r.Context(), instanceID)
			connector, _ = s.store.GetRuntimeConnectorInstallation(r.Context(), connector.ID)
		}
	}
	if instance == nil || connector == nil {
		writeAPIError(w, http.StatusNotFound, "not_found", "runtime instance is not available")
		return nil, nil, false
	}
	if !instance.Shared {
		writeAPIError(w, http.StatusConflict, "instance_not_shared", "runtime instance is not currently shared")
		return nil, nil, false
	}
	if !instance.Enabled {
		writeAPIError(w, http.StatusConflict, "instance_disabled", "runtime instance is disabled")
		return nil, nil, false
	}
	if !connector.Enabled {
		writeAPIError(w, http.StatusConflict, "runtime_connector_disabled", "runtime connector is disabled")
		return nil, nil, false
	}
	return instance, connector, true
}

func (s *Server) proxyRuntimeInstanceOperation(w http.ResponseWriter, r *http.Request, token *APIToken, instance RuntimeInstance, rest []string) {
	if !s.requireRuntimeBridge(w) {
		return
	}
	relayPath := ""
	method := r.Method
	scope := requiredRuntimeScope(append([]string{instance.ID}, rest...), r.Method)
	var bodyOverride []byte

	switch {
	case len(rest) == 1 && rest[0] == "runs" && r.Method == http.MethodGet:
		relayPath = fmt.Sprintf("/api/v2/instances/%s/runs", instance.ID)
	case len(rest) == 1 && rest[0] == "runs" && r.Method == http.MethodPost:
		relayPath = fmt.Sprintf("/api/v2/instances/%s/runs", instance.ID)
	case len(rest) == 2 && rest[0] == "runs" && r.Method == http.MethodGet:
		relayPath = fmt.Sprintf("/api/v2/instances/%s/runs/%s", instance.ID, rest[1])
	case len(rest) == 3 && rest[0] == "runs" && rest[2] == "steps" && r.Method == http.MethodPost:
		relayPath = fmt.Sprintf("/api/v2/instances/%s/runs/%s/steps", instance.ID, rest[1])
		scope = "steps:write"
	case len(rest) == 3 && rest[0] == "runs" && rest[2] == "gates" && r.Method == http.MethodGet:
		relayPath = fmt.Sprintf("/api/v2/instances/%s/runs/%s/gates", instance.ID, rest[1])
		scope = "gates:read"
	case len(rest) == 3 && rest[0] == "runs" && rest[2] == "abort" && r.Method == http.MethodPost:
		relayPath = fmt.Sprintf("/api/v2/instances/%s/runs/%s/abort", instance.ID, rest[1])
		scope = "runs:write"
	case len(rest) == 2 && rest[0] == "steps" && r.Method == http.MethodGet:
		relayPath = fmt.Sprintf("/api/v2/steps/%s", rest[1])
		scope = "steps:read"
	case len(rest) == 3 && rest[0] == "steps" && r.Method == http.MethodGet:
		relayPath = fmt.Sprintf("/api/v2/steps/%s/%s", rest[1], rest[2])
		scope = "steps:read"
		if rest[2] == "artifacts" {
			scope = "artifacts:read"
		}
	case len(rest) == 3 && rest[0] == "artifacts" && rest[2] == "content" && r.Method == http.MethodGet:
		relayPath = fmt.Sprintf("/api/v2/artifacts/%s/content", rest[1])
		scope = "artifacts:read"
	case len(rest) == 3 && rest[0] == "gates" && r.Method == http.MethodPost && (rest[2] == "approve" || rest[2] == "reject"):
		relayPath = fmt.Sprintf("/api/v2/gates/%s", rest[1])
		scope = "gates:write"
		bodyOverride = mustJSON(map[string]string{"action": rest[2]})
	default:
		http.NotFound(w, r)
		return
	}
	s.serveRelayProxy(w, r, token, []string{scope}, []string{instance.ID}, relayPath, method, bodyOverride)
}

func (s *Server) serveRelayProxy(w http.ResponseWriter, r *http.Request, token *APIToken, scopes []string, instanceIDs []string, relayPath, method string, bodyOverride []byte) {
	if !s.requireRuntimeBridge(w) {
		return
	}
	relayReq := r.Clone(r.Context())
	urlCopy := *r.URL
	urlCopy.Path = relayPath
	relayReq.URL = &urlCopy
	relayReq.Method = method
	relayReq.Header = r.Header.Clone()
	relayReq.Header.Del("Authorization")
	if bodyOverride != nil {
		relayReq.Body = io.NopCloser(bytes.NewReader(bodyOverride))
		relayReq.ContentLength = int64(len(bodyOverride))
		relayReq.Header.Set("Content-Type", "application/json")
	}
	name := "cloud"
	if token != nil {
		name = "cloud:" + firstNonEmpty(token.Name, token.ID)
	}
	s.runtime.Server.ServeAsPlanner(w, relayReq, name, scopes, instanceIDs)
}

func (s *Server) requireRuntimeBridge(w http.ResponseWriter) bool {
	if s.runtime == nil || s.runtime.Server == nil || s.runtime.Store == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "runtime_bridge_missing", "cloud runtime bridge is not configured")
		return false
	}
	return true
}

func (s *Server) syncRuntimeScope(ctx context.Context, orgID, workspaceID, projectID string) error {
	if s.runtime == nil || s.runtime.Server == nil || s.runtime.Store == nil {
		return nil
	}
	installations, err := s.store.ListRuntimeConnectorInstallations(ctx, orgID, workspaceID, projectID)
	if err != nil {
		return err
	}
	for _, installation := range installations {
		if _, _, err := s.syncRuntimeConnectorFromRelay(ctx, installation); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) syncRuntimeConnectorFromRelay(ctx context.Context, installation RuntimeConnectorInstallation) (*RuntimeConnectorInstallation, []RuntimeInstance, error) {
	if s.runtime == nil || s.runtime.Server == nil || s.runtime.Store == nil {
		if installation.ID == "" {
			updated, err := s.store.UpsertRuntimeConnectorInstallation(ctx, installation)
			return updated, nil, err
		}
		updated, err := s.store.UpdateRuntimeConnectorInstallation(ctx, installation)
		return updated, nil, err
	}

	record, err := s.runtime.Store.GetConnector(ctx, installation.ConnectorID)
	if err != nil {
		return nil, nil, fmt.Errorf("get relay connector: %w", err)
	}
	if record == nil {
		installation.Status = "missing"
		installation.Health = "missing"
		installation.LastError = "relay connector not found"
		updated, err := s.persistRuntimeConnectorInstallation(ctx, installation)
		if err != nil {
			return nil, nil, err
		}
		storedInstances, err := s.store.ListRuntimeInstances(ctx, installation.OrgID, installation.WorkspaceID, installation.ProjectID, installation.ID)
		if err != nil {
			return updated, nil, err
		}
		var changed []RuntimeInstance
		for _, instance := range storedInstances {
			instance.Shared = false
			instance.Status = "offline"
			instance.Health = "missing"
			instance.LastError = "relay instance no longer shared"
			next, err := s.store.UpdateRuntimeInstance(ctx, instance)
			if err != nil {
				return updated, nil, err
			}
			changed = append(changed, *next)
		}
		return updated, changed, nil
	}

	connectorStatus := s.runtime.Server.ConnectorStatus(record)
	installation.ConnectorID = record.ConnectorID
	installation.MachineID = record.MachineID
	installation.Label = record.Label
	installation.PublicKey = record.PublicKey
	installation.Status = connectorStatus.Status
	installation.Health = relayHealth(connectorStatus.Status)
	installation.MetadataJSON = mustJSON(map[string]any{"machine_metadata_json": record.MachineMetadataJSON})
	if !record.LastSeenAt.IsZero() {
		installation.LastSeenAt = timePtr(record.LastSeenAt.UTC())
	}
	installation.LastError = ""
	updatedInstallation, err := s.persistRuntimeConnectorInstallation(ctx, installation)
	if err != nil {
		return nil, nil, err
	}

	relayInstances, err := s.runtime.Store.ListInstancesByConnector(ctx, record.ConnectorID)
	if err != nil {
		return nil, nil, fmt.Errorf("list relay instances: %w", err)
	}
	storedInstances, err := s.store.ListRuntimeInstances(ctx, updatedInstallation.OrgID, updatedInstallation.WorkspaceID, updatedInstallation.ProjectID, updatedInstallation.ID)
	if err != nil {
		return nil, nil, err
	}
	current := make(map[string]RuntimeInstance, len(storedInstances))
	for _, instance := range storedInstances {
		current[instance.ID] = instance
	}

	synced := make([]RuntimeInstance, 0, len(relayInstances))
	for _, relayInstance := range relayInstances {
		status, err := s.runtime.Server.InstanceStatus(ctx, &relayInstance)
		if err != nil {
			return nil, nil, err
		}
		next, ok := current[relayInstance.InstanceID]
		if !ok {
			next = RuntimeInstance{
				ID:                             relayInstance.InstanceID,
				OrgID:                          updatedInstallation.OrgID,
				WorkspaceID:                    updatedInstallation.WorkspaceID,
				ProjectID:                      updatedInstallation.ProjectID,
				RuntimeConnectorInstallationID: updatedInstallation.ID,
				Enabled:                        true,
			}
		}
		delete(current, relayInstance.InstanceID)
		next.RepoRoot = relayInstance.RepoRoot
		next.InstanceJSON = []byte(relayInstance.InstanceJSON)
		next.Status = status.Status
		next.Health = relayHealth(status.Status)
		next.Shared = true
		next.LastError = ""
		if !relayInstance.LastSeenAt.IsZero() {
			next.LastSeenAt = timePtr(relayInstance.LastSeenAt.UTC())
		}
		persisted, err := s.persistRuntimeInstance(ctx, next)
		if err != nil {
			return nil, nil, err
		}
		synced = append(synced, *persisted)
	}

	for _, orphan := range current {
		orphan.Shared = false
		orphan.Status = "offline"
		orphan.Health = "missing"
		orphan.LastError = "relay instance no longer shared"
		persisted, err := s.store.UpdateRuntimeInstance(ctx, orphan)
		if err != nil {
			return nil, nil, err
		}
		synced = append(synced, *persisted)
	}
	return updatedInstallation, synced, nil
}

func (s *Server) persistRuntimeConnectorInstallation(ctx context.Context, installation RuntimeConnectorInstallation) (*RuntimeConnectorInstallation, error) {
	if installation.ID == "" {
		return s.store.CreateRuntimeConnectorInstallation(ctx, installation)
	}
	return s.store.UpdateRuntimeConnectorInstallation(ctx, installation)
}

func (s *Server) persistRuntimeInstance(ctx context.Context, instance RuntimeInstance) (*RuntimeInstance, error) {
	existing, err := s.store.GetRuntimeInstance(ctx, instance.ID)
	if err == nil && existing != nil {
		instance.CreatedAt = existing.CreatedAt
		return s.store.UpdateRuntimeInstance(ctx, instance)
	}
	return s.store.CreateRuntimeInstance(ctx, instance)
}

func (s *Server) findRuntimeConnectorByConnectorID(ctx context.Context, connectorID string) (*RuntimeConnectorInstallation, error) {
	installations, err := s.store.ListRuntimeConnectorInstallations(ctx, "", "", "")
	if err != nil {
		return nil, err
	}
	var match *RuntimeConnectorInstallation
	for i := range installations {
		if installations[i].ConnectorID != connectorID {
			continue
		}
		if match != nil {
			return nil, fmt.Errorf("runtime connector %q is claimed multiple times", connectorID)
		}
		copy := installations[i]
		match = &copy
	}
	return match, nil
}

func filterRuntimeInstances(instances []RuntimeInstance, includeUnshared bool) []RuntimeInstance {
	if includeUnshared {
		return instances
	}
	filtered := make([]RuntimeInstance, 0, len(instances))
	for _, instance := range instances {
		if instance.Shared {
			filtered = append(filtered, instance)
		}
	}
	return filtered
}

func includeUnshared(r *http.Request) bool {
	value := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("include_unshared")))
	return value == "1" || value == "true" || value == "yes"
}

func relayHealth(status string) string {
	switch status {
	case "online", "active":
		return "healthy"
	case "disabled":
		return "disabled"
	case "missing":
		return "missing"
	default:
		return "degraded"
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}
