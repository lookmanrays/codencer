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
		JSONRPC   string                 `json:"jsonrpc"`
		ID        interface{}            `json:"id"`
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.errorResponse(w, req.ID, -32700, "Parse error", err.Error())
		return
	}

	// Tool mapping
	resp, err := s.routeTool(r.Context(), req.Name, req.Arguments)
	if err != nil {
		// Tool execution errors within MCP return as normal outputs but with isError=true
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result": map[string]interface{}{
				"isError": true,
				"content": []map[string]string{
					{"type": "text", "text": fmt.Sprintf("Tool execution failed: %v", err)},
				},
			},
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      req.ID,
		"result":  resp,
	})
}

func (s *Server) routeTool(ctx context.Context, name string, args map[string]interface{}) (interface{}, error) {
	switch name {
	case "orchestrator.start_run":
		return s.ToolStartRun(ctx, args)
	case "orchestrator.start_step":
		return s.ToolStartStep(ctx, args)
	case "orchestrator.retry_step":
		return s.ToolRetryStep(ctx, args)
	case "orchestrator.get_status":
		return s.ToolGetStatus(ctx, args)
	case "orchestrator.get_step_result":
		return s.ToolGetStepResult(ctx, args)
	case "orchestrator.get_validations":
		return s.ToolGetValidations(ctx, args)
	case "orchestrator.list_artifacts":
		return s.ToolListArtifacts(ctx, args)
	case "orchestrator.approve_gate":
		return s.ToolApproveGate(ctx, args)
	case "orchestrator.reject_gate":
		return s.ToolRejectGate(ctx, args)
	case "orchestrator.get_benchmarks":
		return s.ToolGetBenchmarks(ctx, args)
	case "orchestrator.get_routing_config":
		return s.ToolGetRoutingConfig(ctx, args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func (s *Server) errorResponse(w http.ResponseWriter, id interface{}, code int, msg, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // JSON-RPC dictates HTTP 200 even for RPC errors
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]interface{}{
			"code":    code,
			"message": fmt.Sprintf("%s: %s", msg, details),
		},
	})
}
