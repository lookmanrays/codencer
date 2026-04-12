package relay

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type mcpServer struct {
	relay *Server
	tools map[string]mcpTool
}

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Name    string          `json:"name,omitempty"`
	Args    json.RawMessage `json:"arguments,omitempty"`
}

type mcpResponse struct {
	JSONRPC string       `json:"jsonrpc"`
	ID      any          `json:"id,omitempty"`
	Result  any          `json:"result,omitempty"`
	Error   *mcpRPCError `json:"error,omitempty"`
}

type mcpRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type mcpToolResult struct {
	Content           []map[string]string `json:"content,omitempty"`
	StructuredContent any                 `json:"structuredContent,omitempty"`
	IsError           bool                `json:"isError,omitempty"`
}

func newMCPServer(relayServer *Server) *mcpServer {
	server := &mcpServer{relay: relayServer}
	server.tools = buildMCPTools(server)
	return server
}

func (s *mcpServer) Handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Codencer-MCP-Surface", "relay-public")
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var req mcpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeRPC(w, mcpResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error: &mcpRPCError{
				Code:    -32700,
				Message: "parse error",
				Data:    err.Error(),
			},
		})
		return
	}
	if req.JSONRPC == "" {
		req.JSONRPC = "2.0"
	}
	if req.Method == "" && req.Name != "" {
		params, _ := json.Marshal(map[string]any{
			"name":      req.Name,
			"arguments": rawObjectOrEmpty(req.Args),
		})
		req.Method = "tools/call"
		req.Params = params
	}

	switch req.Method {
	case "initialize":
		s.writeRPC(w, mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"protocolVersion": "2025-03-26",
				"capabilities": map[string]any{
					"tools": map[string]any{"listChanged": false},
				},
				"serverInfo": map[string]any{
					"name":    "codencer-relay",
					"version": "v2",
				},
			},
		})
	case "notifications/initialized":
		w.WriteHeader(http.StatusAccepted)
	case "tools/list":
		s.writeRPC(w, mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"tools": s.listTools(),
			},
		})
	case "tools/call":
		s.handleToolCall(w, r, req)
	default:
		s.writeRPC(w, mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &mcpRPCError{
				Code:    -32601,
				Message: "method not found",
				Data:    req.Method,
			},
		})
	}
}

func (s *mcpServer) handleToolCall(w http.ResponseWriter, r *http.Request, req mcpRequest) {
	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.writeRPC(w, mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &mcpRPCError{
				Code:    -32602,
				Message: "invalid params",
				Data:    err.Error(),
			},
		})
		return
	}
	tool, ok := s.tools[params.Name]
	if !ok {
		s.writeRPC(w, mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  errorToolResult("tool_not_found", fmt.Sprintf("unknown tool: %s", params.Name)),
		})
		return
	}
	if params.Arguments == nil {
		params.Arguments = map[string]any{}
	}

	principal := plannerFromContext(r.Context())
	if err := authorizePrincipal(principal, tool.Scope, tool.instanceID(params.Arguments)); err != nil {
		s.writeRPC(w, mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  errorToolResult(err.Code, err.Message),
		})
		return
	}

	result, apiErr := tool.Invoke(r.Context(), principal, params.Arguments)
	if apiErr != nil {
		s.writeRPC(w, mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  errorToolResult(apiErr.Code, apiErr.Message),
		})
		return
	}
	s.writeRPC(w, mcpResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	})
}

func (s *mcpServer) listTools() []map[string]any {
	tools := make([]map[string]any, 0, len(s.tools))
	for _, tool := range toolOrder(s.tools) {
		tools = append(tools, map[string]any{
			"name":        tool.Name,
			"description": tool.Description,
			"inputSchema": tool.InputSchema,
		})
	}
	return tools
}

func (s *mcpServer) writeRPC(w http.ResponseWriter, response mcpResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

func errorToolResult(code, message string) mcpToolResult {
	return mcpToolResult{
		IsError: true,
		Content: []map[string]string{{
			"type": "text",
			"text": message,
		}},
		StructuredContent: map[string]any{
			"error": map[string]any{
				"code":    code,
				"message": message,
			},
		},
	}
}

func successToolResult(summary string, payload any) mcpToolResult {
	result := mcpToolResult{StructuredContent: payload}
	if summary != "" {
		result.Content = []map[string]string{{
			"type": "text",
			"text": summary,
		}}
	}
	return result
}

func (s *mcpServer) callPlannerRoute(ctx context.Context, authHeader, method, path string, body []byte) (int, http.Header, []byte, *apiError) {
	req, err := http.NewRequest(method, path, bytes.NewReader(body))
	if err != nil {
		return 0, nil, nil, &apiError{Status: http.StatusInternalServerError, Code: "relay_internal_error", Message: err.Error()}
	}
	req = req.WithContext(ctx)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	recorder := newResponseRecorder()
	handler, _ := s.relay.server.Handler.(*http.ServeMux)
	handler.ServeHTTP(recorder, req)
	bodyBytes := recorder.body.Bytes()
	if recorder.statusCode >= 400 {
		return recorder.statusCode, recorder.header, bodyBytes, decodeAPIError(recorder.statusCode, bodyBytes)
	}
	return recorder.statusCode, recorder.header, bodyBytes, nil
}

func decodeAPIError(status int, body []byte) *apiError {
	var payload struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err == nil && payload.Error.Code != "" {
		return &apiError{Status: status, Code: payload.Error.Code, Message: payload.Error.Message}
	}
	return &apiError{Status: status, Code: "upstream_error", Message: strings.TrimSpace(string(body))}
}

func rawObjectOrEmpty(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		return map[string]any{}
	}
	return value
}

func artifactContentPayload(contentType string, body []byte) map[string]any {
	payload := map[string]any{
		"content_type": contentType,
	}
	if strings.HasPrefix(contentType, "text/") || strings.Contains(contentType, "json") {
		payload["encoding"] = "utf-8"
		payload["text"] = string(body)
		return payload
	}
	payload["encoding"] = "base64"
	payload["base64"] = base64.StdEncoding.EncodeToString(body)
	return payload
}
