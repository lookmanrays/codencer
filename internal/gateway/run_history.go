package gateway

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"agent-bridge/internal/security"
)

func (s *Server) beginRunRecord(ctx context.Context, principal *authPrincipal, projectID, mode string, args map[string]any) (RunRecord, map[string]any) {
	if s.store == nil || principal == nil || principal.WorkspaceID == "" {
		return RunRecord{}, map[string]any{"project_id": projectID}
	}
	record := RunRecord{
		WorkspaceID:     principal.WorkspaceID,
		ActorUserID:     principal.UserID,
		ProjectID:       projectID,
		Title:           firstNonEmpty(stringArg(args, "title"), stringArg(args, "manifest_name"), "Untitled run"),
		Goal:            runGoalFromArgs(args),
		Mode:            normalizeRunMode(mode),
		ExecutorProfile: firstNonEmpty(stringArg(args, "profile"), stringArg(args, "adapter_profile")),
		Status:          "submitted",
		ReportStatus:    "pending",
		StartedAt:       time.Now().UTC(),
	}
	saved, err := s.store.UpsertRunRecord(ctx, record)
	if err == nil {
		record = saved
	}
	return record, runAuditMetadata(record)
}

func (s *Server) applyRouteToRunRecord(ctx context.Context, record RunRecord, principal *authPrincipal, match relayProjectMatch, args map[string]any) (RunRecord, map[string]any) {
	if record.ID == "" {
		return record, map[string]any{"project_id": match.Project.ProjectID, "relay_profile_id": match.Profile.ID}
	}
	location := selectedProjectLocation(match.Project, args)
	record.ProjectID = match.Project.ProjectID
	record.ProjectName = match.Project.Name
	record.RelayProfileID = match.Profile.ID
	record.ConnectorID = location.ConnectorID
	record.MachineID = location.MachineID
	record.HostLabel = location.HostLabel
	record.ExecutorProfile = resolvedExecutorLabel(match.Project, args)
	if record.ActorUserID == "" && principal != nil {
		record.ActorUserID = principal.UserID
	}
	if s.store != nil {
		if saved, err := s.store.UpsertRunRecord(ctx, record); err == nil {
			record = saved
		}
	}
	return record, runAuditMetadata(record)
}

func (s *Server) finishRunRecord(ctx context.Context, record RunRecord, payload any, reportStatus string) (RunRecord, map[string]any) {
	if record.ID == "" {
		return record, map[string]any{}
	}
	applyPayloadToRunRecord(&record, payload, reportStatus)
	if s.store != nil {
		if saved, err := s.store.UpsertRunRecord(ctx, record); err == nil {
			record = saved
		}
	}
	return record, runAuditMetadata(record)
}

func (s *Server) refreshRunRecordFromReport(ctx context.Context, principal *authPrincipal, match relayProjectMatch, args map[string]any, payload any) (RunRecord, map[string]any) {
	return s.refreshRunRecordFromLifecyclePayload(ctx, principal, match, args, payload, reportStatusForPayload(payload, "completed"))
}

func (s *Server) refreshRunRecordFromLifecyclePayload(ctx context.Context, principal *authPrincipal, match relayProjectMatch, args map[string]any, payload any, reportStatus string) (RunRecord, map[string]any) {
	if s.store == nil || principal == nil || principal.WorkspaceID == "" {
		return RunRecord{}, map[string]any{"project_id": match.Project.ProjectID, "run_id": stringArg(args, "run_id")}
	}
	runID := firstNonEmpty(stringArg(args, "run_id"), runIDFromPayload(payload))
	record, err := s.store.FindRunRecordByRunID(ctx, principal.WorkspaceID, match.Project.ProjectID, runID)
	if err != nil && err != sql.ErrNoRows {
		return RunRecord{}, map[string]any{"project_id": match.Project.ProjectID, "run_id": runID}
	}
	if err == sql.ErrNoRows || record.ID == "" {
		record = RunRecord{
			WorkspaceID:     principal.WorkspaceID,
			ActorUserID:     principal.UserID,
			ProjectID:       match.Project.ProjectID,
			ProjectName:     match.Project.Name,
			RunID:           runID,
			Title:           firstNonEmpty(stringArg(args, "title"), "Run "+runID),
			Mode:            normalizeRunMode(stringArg(args, "mode")),
			ExecutorProfile: resolvedExecutorLabel(match.Project, args),
			ReportStatus:    "pending",
			StartedAt:       time.Now().UTC(),
		}
	}
	record, _ = s.applyRouteToRunRecord(ctx, record, principal, match, args)
	record.RunID = firstNonEmpty(record.RunID, runID)
	return s.finishRunRecord(ctx, record, payload, reportStatus)
}

func applyPayloadToRunRecord(record *RunRecord, payload any, reportStatus string) {
	obj, _ := sanitizeAny(payload).(map[string]any)
	if obj == nil {
		obj = map[string]any{}
	}
	record.Report = obj
	record.RunID = firstNonEmpty(record.RunID, runIDFromPayload(obj))
	record.StepID = firstNonEmpty(record.StepID, stringValueFromAny(obj["step_id"]), nestedString(obj, "task", "step_id"))
	record.Status = firstNonEmpty(stringValueFromAny(obj["status"]), nestedString(obj, "task", "status"), record.Status)
	if record.Status == "" {
		record.Status = "completed"
	}
	if reportStatus != "" {
		record.ReportStatus = reportStatus
	} else if record.ReportStatus == "" || record.ReportStatus == "pending" {
		record.ReportStatus = "available"
	}
	record.ResultSummary = firstNonEmpty(resultSummaryFromPayload(obj), record.ResultSummary)
	record.ResultDetails = firstNonEmpty(resultDetailsFromPayload(obj), record.ResultDetails, record.ResultSummary)
	eventType := terminalAuditType(obj)
	if eventType == "run_completed" || eventType == "run_failed" || eventType == "blocker" || eventType == "run_cancelled" {
		record.CompletedAt = time.Now().UTC()
	}
}

func reportStatusForPayload(payload any, terminalStatus string) string {
	eventType := terminalAuditType(payload)
	if eventType == "run_started" || eventType == "run_resumed" {
		return "pending"
	}
	return terminalStatus
}

func runAuditMetadata(record RunRecord) map[string]any {
	metadata := map[string]any{}
	for key, value := range map[string]string{
		"run_history_id":   record.ID,
		"run_id":           record.RunID,
		"step_id":          record.StepID,
		"project_id":       record.ProjectID,
		"relay_profile_id": record.RelayProfileID,
		"connector_id":     record.ConnectorID,
		"machine_id":       record.MachineID,
		"host_label":       record.HostLabel,
		"executor_profile": record.ExecutorProfile,
	} {
		if strings.TrimSpace(value) != "" {
			metadata[key] = security.Redact(value)
		}
	}
	return metadata
}

func runGoalFromArgs(args map[string]any) string {
	if goal := firstNonEmpty(stringArg(args, "goal"), stringArg(args, "prompt")); goal != "" {
		return goal
	}
	if task, _ := args["task"].(map[string]any); task != nil {
		return firstNonEmpty(stringValueFromAny(task["goal"]), stringValueFromAny(task["prompt"]))
	}
	return ""
}

func normalizeRunMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case "manifest", "codencer.run_project_manifest":
		return "manifest"
	default:
		return "task"
	}
}

func resultSummaryFromPayload(payload map[string]any) string {
	for _, candidate := range []string{
		stringValueFromAny(payload["summary"]),
		nestedString(payload, "task", "summary"),
		nestedString(payload, "evidence", "result", "summary"),
		nestedString(payload, "task", "evidence", "result", "summary"),
		firstTaskString(payload, "summary"),
		firstTaskNestedString(payload, "evidence", "result", "summary"),
		nestedString(payload, "blocker", "message"),
		nestedString(payload, "task", "blocker", "message"),
	} {
		if text := safeExcerpt(candidate, 1200); text != "" {
			return text
		}
	}
	if text := firstRawOutput(payload); text != "" {
		return safeExcerpt(text, 1200)
	}
	if text := firstValidationSummary(payload); text != "" {
		return safeExcerpt(text, 1200)
	}
	if text := artifactSummary(payload); text != "" {
		return text
	}
	return ""
}

func resultDetailsFromPayload(payload map[string]any) string {
	if raw := firstRawOutput(payload); raw != "" {
		return safeExcerpt(raw, 4000)
	}
	parts := []string{}
	if validations := firstValidationSummary(payload); validations != "" {
		parts = append(parts, validations)
	}
	if artifacts := artifactSummary(payload); artifacts != "" {
		parts = append(parts, artifacts)
	}
	if blocker := firstNonEmpty(nestedString(payload, "blocker", "message"), nestedString(payload, "task", "blocker", "message")); blocker != "" {
		parts = append(parts, blocker)
	}
	if len(parts) == 0 {
		return resultSummaryFromPayload(payload)
	}
	return safeExcerpt(strings.Join(parts, "\n"), 4000)
}

func firstRawOutput(payload map[string]any) string {
	return firstNonEmpty(
		stringValueFromAny(payload["raw_output"]),
		nestedString(payload, "evidence", "result", "raw_output"),
		nestedString(payload, "task", "evidence", "result", "raw_output"),
		firstTaskNestedString(payload, "evidence", "result", "raw_output"),
	)
}

func firstTaskString(payload map[string]any, key string) string {
	tasks, _ := payload["tasks"].([]any)
	for _, item := range tasks {
		task, _ := item.(map[string]any)
		if value := stringValueFromAny(task[key]); value != "" {
			return value
		}
	}
	return ""
}

func firstTaskNestedString(payload map[string]any, keys ...string) string {
	tasks, _ := payload["tasks"].([]any)
	for _, item := range tasks {
		task, _ := item.(map[string]any)
		if value := nestedString(task, keys...); value != "" {
			return value
		}
	}
	return ""
}

func firstValidationSummary(payload map[string]any) string {
	data, _ := json.Marshal(payload)
	text := string(data)
	if !strings.Contains(text, "validations") {
		return ""
	}
	var names []string
	var walk func(any)
	walk = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			if summary := firstNonEmpty(stringValueFromAny(typed["summary"]), stringValueFromAny(typed["status"]), stringValueFromAny(typed["state"])); summary != "" {
				if name := firstNonEmpty(stringValueFromAny(typed["name"]), stringValueFromAny(typed["id"])); name != "" {
					names = append(names, name+": "+summary)
				}
			}
			for _, item := range typed {
				walk(item)
			}
		case []any:
			for _, item := range typed {
				walk(item)
			}
		}
	}
	walk(payload["validations"])
	walk(nestedAny(payload, "evidence", "validations"))
	walk(nestedAny(payload, "task", "evidence", "validations"))
	if len(names) == 0 {
		return ""
	}
	return "Validations: " + strings.Join(names, "; ")
}

func artifactSummary(payload map[string]any) string {
	var names []string
	var walk func(any)
	walk = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			if name := stringValueFromAny(typed["name"]); name != "" {
				names = append(names, name)
			}
			for _, item := range typed {
				walk(item)
			}
		case []any:
			for _, item := range typed {
				walk(item)
			}
		}
	}
	walk(payload["artifacts"])
	walk(nestedAny(payload, "evidence", "artifacts"))
	walk(nestedAny(payload, "task", "evidence", "artifacts"))
	walk(payload["tasks"])
	if len(names) == 0 {
		return ""
	}
	return "Artifacts: " + strings.Join(uniqueStrings(names), ", ")
}

func nestedAny(obj map[string]any, keys ...string) any {
	var current any = obj
	for _, key := range keys {
		next, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = next[key]
	}
	return current
}

func safeExcerpt(value string, limit int) string {
	value = strings.TrimSpace(security.Redact(value))
	if value == "" {
		return ""
	}
	data, _ := json.Marshal(map[string]any{"value": value})
	sanitized := string(security.SanitizeRemoteJSON(data))
	var decoded map[string]string
	if json.Unmarshal([]byte(sanitized), &decoded) == nil {
		value = strings.TrimSpace(decoded["value"])
	}
	value = strings.TrimSpace(value)
	if limit > 0 && len(value) > limit {
		return value[:limit] + "..."
	}
	return value
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func runRecordNotFound(id string) *apiError {
	return &apiError{Status: 404, Code: "run_not_found", Message: fmt.Sprintf("run not found: %s", id)}
}
