package app

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/mcp"
	"agent-bridge/internal/service"
	"time"
	"fmt"
	"path/filepath"
)

// APIHandler holds dependencies for exposing REST routes.
type APIHandler struct {
	RunSvc  *service.RunService
	GateSvc *service.GateService
	AGSvc   *service.AntigravityService
	AppCtx  *AppContext
}

// RegisterRoutes attaches the API to the given mux.
func (h *APIHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/runs", h.handleRuns)
	mux.HandleFunc("/api/v1/runs/", h.handleRunByID) // Also intercepts /runs/{id}/steps if matched manually
	mux.HandleFunc("/api/v1/steps/", h.handleStepByID)
	mux.HandleFunc("/api/v1/gates/", h.handleGateByID)
	mux.HandleFunc("/api/v1/compatibility", h.handleCompatibility)
	mux.HandleFunc("/api/v1/benchmarks", h.handleBenchmarks)
	mux.HandleFunc("/api/v1/routing", h.handleRouting)
	mux.HandleFunc("/api/v1/instance", h.handleInstance)
	mux.HandleFunc("/api/v1/antigravity/instances", h.handleAGInstances)
	mux.HandleFunc("/api/v1/antigravity/status", h.handleAGStatus)
	mux.HandleFunc("/api/v1/antigravity/bind", h.handleAGBind)
	
	mcpServer := mcp.NewServer(h.RunSvc, h.GateSvc)
	mux.HandleFunc("/mcp/call", mcpServer.HandleCall)
}

func (h *APIHandler) handleRuns(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		filters := make(map[string]string)
		if p := r.URL.Query().Get("project_id"); p != "" {
			filters["project_id"] = p
		}
		if c := r.URL.Query().Get("conversation_id"); c != "" {
			filters["conversation_id"] = c
		}
		if s := r.URL.Query().Get("state"); s != "" {
			filters["state"] = s
		}

		runs, err := h.RunSvc.List(r.Context(), filters)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(runs)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID             string `json:"id"`
		ProjectID      string `json:"project_id"`
		ConversationID string `json:"conversation_id"`
		PlannerID      string `json:"planner_id"`
		ExecutorID     string `json:"executor_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.ID == "" {
		req.ID = fmt.Sprintf("run-%d", time.Now().Unix())
	}
	if req.ProjectID == "" {
		req.ProjectID = "default-project"
	}

	run, err := h.RunSvc.StartRun(r.Context(), req.ID, req.ProjectID, req.ConversationID, req.PlannerID, req.ExecutorID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(run)
}

func (h *APIHandler) handleRunByID(w http.ResponseWriter, r *http.Request) {
	fullPath := strings.TrimPrefix(r.URL.Path, "/api/v1/runs/")
	parts := strings.Split(strings.Trim(fullPath, "/"), "/")
	id := parts[0]
	if id == "" {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		if len(parts) > 1 {
			sub := parts[1]
			if sub == "steps" {
				steps, err := h.RunSvc.GetStepsByRun(r.Context(), id)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				json.NewEncoder(w).Encode(steps)
				return
			}
			if sub == "gates" {
				gates, err := h.RunSvc.GetGatesByRun(r.Context(), id)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				json.NewEncoder(w).Encode(gates)
				return
			}
		}

		run, err := h.RunSvc.GetRun(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if run == nil {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(run)

	case http.MethodPost:
		if len(parts) > 1 && parts[1] == "steps" {
			var spec domain.TaskSpec
			if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			if spec.StepID == "" {
				spec.StepID = fmt.Sprintf("step-%d", time.Now().Unix())
			}
			if spec.PhaseID == "" {
				spec.PhaseID = fmt.Sprintf("phase-execution-%s", id)
			}

			step := &domain.Step{
				ID:             spec.StepID,
				PhaseID:        spec.PhaseID,
				Title:          spec.Title,
				Goal:           spec.Goal,
				Adapter:        spec.AdapterProfile,
				Policy:         spec.PolicyBundle,
				TimeoutSeconds: spec.TimeoutSeconds,
				Validations:    spec.Validations,
			}

			go func() {
				if err := h.RunSvc.DispatchStep(context.Background(), id, step); err != nil {
					// Log omitted
				}
			}()

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(step)
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

	case http.MethodPatch:
		var req struct {
			Action string `json:"action"` // "abort"
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if req.Action == "abort" {
			if err := h.RunSvc.AbortRun(r.Context(), id); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, "Invalid action", http.StatusBadRequest)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *APIHandler) handleStepByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/v1/steps/"):]
	if id == "" {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}

	// Extract step ID if path is /api/v1/steps/{id}/anything
	parts := strings.Split(strings.Trim(id, "/"), "/")
	stepID := parts[0]

	if r.Method == http.MethodGet {
		isArtifacts := strings.HasSuffix(r.URL.Path, "/artifacts")
		isResult := strings.HasSuffix(r.URL.Path, "/result")
		isValidations := strings.HasSuffix(r.URL.Path, "/validations")
		isLogs := strings.HasSuffix(r.URL.Path, "/logs")

		if isLogs {
			result, err := h.RunSvc.GetResultByStep(r.Context(), stepID)
			if err != nil {
				http.Error(w, "Result not found: "+err.Error(), http.StatusNotFound)
				return
			}
			if result.RawOutputRef == "" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			content, err := os.ReadFile(result.RawOutputRef)
			if err != nil {
				http.Error(w, "Error reading logs: "+err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			w.Write(content)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		
		if isArtifacts {
			artifacts, err := h.RunSvc.GetArtifactsByStep(r.Context(), stepID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(artifacts)
			return
		}

		if isResult {
			result, err := h.RunSvc.GetResultByStep(r.Context(), stepID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			json.NewEncoder(w).Encode(result)
			return
		}

		if isValidations {
			validations, err := h.RunSvc.GetValidationsByStep(r.Context(), stepID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(validations)
			return
		}

		step, err := h.RunSvc.GetStep(r.Context(), stepID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if step == nil {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		
		_ = json.NewEncoder(w).Encode(step)
		return
	}

	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/retry") {
		if err := h.RunSvc.RetryStep(r.Context(), stepID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (h *APIHandler) handleGateByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/v1/gates/"):]
	if id == "" {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodPost {
		var req struct {
			Action string `json:"action"` // "approve", "reject"
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if req.Action == "approve" {
			if err := h.GateSvc.Approve(r.Context(), id); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}

		if req.Action == "reject" {
			if err := h.GateSvc.Reject(r.Context(), id); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		
		http.Error(w, "Unknown action", http.StatusBadRequest)
		return
	}
	
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}
func (h *APIHandler) handleCompatibility(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Dynamic detection of local IDE environment
	matrix := map[string]interface{}{
		"tier": 2, // Defaulting to Tier 2: Control features work via CLI/Daemon
		"adapters": []map[string]interface{}{
			{"id": "codex", "status": "active", "tier": 2},
			{"id": "claude", "status": "active", "tier": 2},
			{"id": "qwen", "status": "active", "tier": 2},
			{"id": "ide-chat", "status": "active", "tier": 3},
		},
		"environment": map[string]interface{}{
			"os": os.Getenv("OS"),
			"vscode_detected": false, // Simplified for MVP
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(matrix)
}

func (h *APIHandler) handleBenchmarks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	adapter := r.URL.Query().Get("adapter")
	scores, err := h.RunSvc.GetBenchmarks(r.Context(), adapter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(scores)
}

func (h *APIHandler) handleRouting(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	config := h.RunSvc.GetRoutingConfig(r.Context())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}
func (h *APIHandler) handleInstance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	simMode := os.Getenv("ALL_ADAPTERS_SIMULATION_MODE")
	execMode := "real"
	if simMode == "1" || simMode == "true" {
		execMode = "simulation"
	}

	info := domain.InstanceInfo{
		Version:       Version,
		RepoRoot:      h.AppCtx.RepoRoot,
		StateDir:      filepath.Join(h.AppCtx.RepoRoot, ".codencer"),
		WorkspaceRoot: h.AppCtx.Config.WorkspaceRoot,
		Host:          h.AppCtx.Config.Host,
		Port:          h.AppCtx.Config.Port,
		BaseURL:       fmt.Sprintf("http://%s:%d", h.AppCtx.Config.Host, h.AppCtx.Config.Port),
		ExecutionMode: execMode,
		PID:           os.Getpid(),
		StartedAt:     h.AppCtx.StartedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(info)
}

func (h *APIHandler) handleAGInstances(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	instances, err := h.AGSvc.ListInstances(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(instances)
}

func (h *APIHandler) handleAGStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	inst, err := h.AGSvc.GetBinding(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if inst == nil {
		w.Write([]byte("null"))
		return
	}
	json.NewEncoder(w).Encode(inst)
}

func (h *APIHandler) handleAGBind(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var req struct {
			PID int `json:"pid"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := h.AGSvc.Bind(r.Context(), req.PID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)

	case http.MethodDelete:
		if err := h.AGSvc.Unbind(r.Context()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
