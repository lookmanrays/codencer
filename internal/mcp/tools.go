package mcp

import (
	"context"
	"fmt"
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

	run, err := s.runSvc.GetStatus(ctx, id)
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
