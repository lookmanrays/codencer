package mcp

import (
	"context"
	"fmt"

	"agent-bridge/internal/domain"
)

// ToolStartRun implements `orchestrator.start_run` tool.
func (s *Server) ToolStartRun(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	id, ok1 := args["id"].(string)
	projectID, ok2 := args["project_id"].(string)
	conversationID, _ := args["conversation_id"].(string)
	plannerID, _ := args["planner_id"].(string)
	executorID, _ := args["executor_id"].(string)
	
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("missing or invalid arguments: id, project_id")
	}

	run, err := s.runSvc.StartRun(ctx, id, projectID, conversationID, plannerID, executorID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": fmt.Sprintf("Run %s created and started successfully.", run.ID),
			},
		},
		"run_id":     run.ID,
		"project_id": run.ProjectID,
		"state":      run.State,
		"recovery_notes": run.RecoveryNotes,
	}, nil
}

// ToolGetStatus implements `orchestrator.get_status` tool.
func (s *Server) ToolGetStatus(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	id, ok := args["id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing argument: id")
	}

	run, err := s.runSvc.GetRun(ctx, id)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, fmt.Errorf("run not found")
	}

	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": fmt.Sprintf("Run %s is in state: %s", run.ID, run.State),
			},
		},
		"run_id": run.ID,
		"project_id": run.ProjectID,
		"conversation_id": run.ConversationID,
		"planner_id": run.PlannerID,
		"executor_id": run.ExecutorID,
		"state":  run.State,
		"recovery_notes": run.RecoveryNotes,
	}, nil
}

// ToolListRuns implements `orchestrator.list_runs` tool.
func (s *Server) ToolListRuns(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	filters := make(map[string]string)
	if v, ok := args["project_id"].(string); ok {
		filters["project_id"] = v
	}
	if v, ok := args["state"].(string); ok {
		filters["state"] = v
	}

	runs, err := s.runSvc.List(ctx, filters)
	if err != nil {
		return nil, err
	}

	summary := fmt.Sprintf("Found %d runs", len(runs))
	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": summary,
			},
		},
		"runs": runs,
	}, nil
}

// ToolApproveGate implements `orchestrator.approve_gate` tool.
func (s *Server) ToolApproveGate(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	id, ok := args["id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing argument: id")
	}

	if err := s.gateSvc.Approve(ctx, id); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": fmt.Sprintf("Gate %s approved.", id),
			},
		},
		"gate_id": id,
		"status":  "approved",
	}, nil
}

// ToolRejectGate implements `orchestrator.reject_gate` tool.
func (s *Server) ToolRejectGate(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	id, ok := args["id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing argument: id")
	}

	if err := s.gateSvc.Reject(ctx, id); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": fmt.Sprintf("Gate %s rejected.", id),
			},
		},
		"gate_id": id,
		"status":  "rejected",
	}, nil
}

// ToolStartStep implements orchestrator.start_step
func (s *Server) ToolStartStep(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	runID, ok := args["run_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing argument: run_id")
	}
	stepID, ok := args["step_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing argument: step_id")
	}
	phaseID, ok := args["phase_id"].(string)
	adapter, okAdapter := args["adapter"].(string)
	if !okAdapter {
		adapter = "codex"
	}
	
	step := &domain.Step{
		ID:      stepID,
		PhaseID: phaseID,
		Title:   "MCP Dispatched Step",
		Goal:    "Execute task from MCP",
		Adapter: adapter,
	}

	go func() {
		_ = s.runSvc.DispatchStep(context.Background(), runID, step)
	}()
    
	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": fmt.Sprintf("Step %s dispatched for run %s.", stepID, runID),
			},
		},
		"step_id": step.ID,
		"run_id":  runID,
		"state":   domain.StepStateDispatching,
	}, nil
}

// ToolGetValidations implements orchestrator.get_validations tool.
func (s *Server) ToolGetValidations(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	stepID, ok := args["step_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing argument: step_id")
	}

	validations, err := s.runSvc.GetValidationsByStep(ctx, stepID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": fmt.Sprintf("Retrieved validations for step %s.", stepID),
			},
		},
		"step_id":     stepID,
		"validations": validations,
	}, nil
}

// ToolRetryStep implements orchestrator.retry_step
func (s *Server) ToolRetryStep(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	stepID, ok := args["step_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing argument: step_id")
	}

	step, err := s.runSvc.GetStep(ctx, stepID)
	if err != nil {
		return nil, err
	}
	if step == nil {
		return nil, fmt.Errorf("step not found")
	}

	phase, err := s.runSvc.GetPhase(ctx, step.PhaseID)
	runID := ""
	if err == nil && phase != nil {
		runID = phase.RunID
	}
	if runID == "" {
		// Fallback parse attempt if repository lookup is restricted
		var placeholder string
		if n, _ := fmt.Sscanf(step.PhaseID, "phase-01-%s", &placeholder); n != 1 {
			// If naming convention fails, we must rely on explicit run_id if we enhance the tool args later
			return nil, fmt.Errorf("could not resolve run identity for step %s", stepID)
		}
		runID = placeholder
	}

	// Just route to DispatchStep to execute another attempt lifecycle
	go func() {
		_ = s.runSvc.DispatchStep(context.Background(), runID, step)
	}()

	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": fmt.Sprintf("Step %s dispatched for retry under run %s.", stepID, runID),
			},
		},
		"step_id": step.ID,
		"run_id":  runID,
	}, nil
}

// ToolGetStepResult implements orchestrator.get_step_result
func (s *Server) ToolGetStepResult(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	stepID, ok := args["step_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing argument: step_id")
	}

	result, err := s.runSvc.GetResultByStep(ctx, stepID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": fmt.Sprintf("Step %s result state: %s\nSummary: %s", stepID, result.State, result.Summary),
			},
		},
		"step_id": stepID,
		"state":   result.State,
		"summary": result.Summary,
	}, nil
}

// ToolListArtifacts implements orchestrator.list_artifacts
func (s *Server) ToolListArtifacts(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	stepID, ok := args["step_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing argument: step_id")
	}

	artifacts, err := s.runSvc.GetArtifactsByStep(ctx, stepID)
	if err != nil {
		return nil, err
	}

	summary := fmt.Sprintf("Found %d artifacts for step %s:\n", len(artifacts), stepID)
	for _, a := range artifacts {
		summary += fmt.Sprintf("- [%s] %s (%d bytes)\n", a.Type, a.Path, a.Size)
	}

	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": summary,
			},
		},
		"step_id":   stepID,
		"artifacts": artifacts,
	}, nil
}

// ToolGetBenchmarks implements orchestrator.get_benchmarks
func (s *Server) ToolGetBenchmarks(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	adapter, _ := args["adapter"].(string)
	scores, err := s.runSvc.GetBenchmarks(ctx, adapter)
	if err != nil {
		return nil, err
	}

	summary := fmt.Sprintf("Retrieved %d benchmark scores", len(scores))
	if adapter != "" {
		summary += " for adapter " + adapter
	}

	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": summary,
			},
		},
		"scores": scores,
	}, nil
}

// ToolGetRoutingConfig implements orchestrator.get_routing_config
func (s *Server) ToolGetRoutingConfig(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	config := s.runSvc.GetRoutingConfig(ctx)
	
	summary := fmt.Sprintf("Routing Mode: %s\nFallback Chain: %v", config["mode"], config["chain"])

	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": summary,
			},
		},
		"config": config,
	}, nil
}
