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
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("missing or invalid arguments: id, project_id")
	}

	run, err := s.runSvc.StartRun(ctx, id, projectID)
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
		_ = s.runSvc.DispatchStep(context.Background(), runID, step, "/tmp/codencer/artifacts/"+stepID)
	}()

	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": fmt.Sprintf("Step %s dispatched for run %s.", stepID, runID),
			},
		},
	}, nil
}

// ToolGetStepResult implements orchestrator.get_step_result
func (s *Server) ToolGetStepResult(ctx context.Context, args map[string]interface{}) (interface{}, error) {
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

	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": fmt.Sprintf("Step %s is in state: %s", step.ID, step.State),
			},
		},
	}, nil
}
