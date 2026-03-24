package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"agent-bridge/internal/service"
)

// Server provides the MCP bridge over HTTP (mimicking MCP's basic transport).
type Server struct {
	runSvc  *service.RunService
	gateSvc *service.GateService
}

// NewServer initializes the MCP bridge.
func NewServer(runSvc *service.RunService, gateSvc *service.GateService) *Server {
	return &Server{
		runSvc:  runSvc,
		gateSvc: gateSvc,
	}
}

// HandleCall mimics an MCP CallTool endpoint.
func (s *Server) HandleCall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.errorResponse(w, "Invalid input", err.Error())
		return
	}

	resp, err := s.routeTool(r.Context(), req.Name, req.Arguments)
	if err != nil {
		s.errorResponse(w, "Tool execution failed", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) routeTool(ctx context.Context, name string, args map[string]interface{}) (interface{}, error) {
	switch name {
	case "orchestrator.start_run":
		return s.ToolStartRun(ctx, args)
	case "orchestrator.start_step":
		return s.ToolStartStep(ctx, args)
	case "orchestrator.get_status":
		return s.ToolGetStatus(ctx, args)
	case "orchestrator.get_step_result":
		return s.ToolGetStepResult(ctx, args)
	case "orchestrator.approve_gate":
		return s.ToolApproveGate(ctx, args)
	case "orchestrator.reject_gate":
		return s.ToolRejectGate(ctx, args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func (s *Server) errorResponse(w http.ResponseWriter, msg, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"isError": true,
		"content": []map[string]string{
			{"type": "text", "text": fmt.Sprintf("%s: %s", msg, details)},
		},
	})
}
