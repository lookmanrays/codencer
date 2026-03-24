package app

import (
	"context"
	"encoding/json"
	"net/http"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/mcp"
	"agent-bridge/internal/service"
)

// APIHandler holds dependencies for exposing REST routes.
type APIHandler struct {
	RunSvc  *service.RunService
	GateSvc *service.GateService
}

// RegisterRoutes attaches the API to the given mux.
func (h *APIHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/runs", h.handleRuns)
	mux.HandleFunc("/api/v1/runs/", h.handleRunByID) // Also intercepts /runs/{id}/steps if matched manually
	mux.HandleFunc("/api/v1/steps/", h.handleStepByID)
	mux.HandleFunc("/api/v1/gates/", h.handleGateByID)
	
	mcpServer := mcp.NewServer(h.RunSvc, h.GateSvc)
	mux.HandleFunc("/mcp/call", mcpServer.HandleCall)
}

func (h *APIHandler) handleRuns(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		runs, err := h.RunSvc.List(r.Context())
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
		ID        string `json:"id"`
		ProjectID string `json:"project_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	run, err := h.RunSvc.StartRun(r.Context(), req.ID, req.ProjectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(run)
}

func (h *APIHandler) handleRunByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/v1/runs/"):]
	if id == "" {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodGet {
		isSteps := len(r.URL.Path) > len("/api/v1/runs/"+id+"/") && r.URL.Path[len("/api/v1/runs/"+id+"/"):] == "steps"
		isGates := len(r.URL.Path) > len("/api/v1/runs/"+id+"/") && r.URL.Path[len("/api/v1/runs/"+id+"/"):] == "gates"

		w.Header().Set("Content-Type", "application/json")
		if isSteps {
			steps, err := h.RunSvc.GetStepsByRun(r.Context(), id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(steps)
			return
		}
		if isGates {
			gates, err := h.RunSvc.GetGatesByRun(r.Context(), id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(gates)
			return
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
		return
	}

	if r.Method == http.MethodPatch { // Used for Abort conceptually
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
	}

	if r.Method == http.MethodPost {
		// Used to dispatch steps: /api/v1/runs/{id}/steps
		if len(r.URL.Path) > len("/api/v1/runs/"+id+"/") && r.URL.Path[len("/api/v1/runs/"+id+"/"):] == "steps" {
			var req struct {
				ID      string `json:"id"`
				PhaseID string `json:"phase_id"`
				Title   string `json:"title"`
				Goal    string `json:"goal"`
				Adapter string `json:"adapter"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			step := &domain.Step{
				ID:      req.ID,
				PhaseID: req.PhaseID,
				Title:   req.Title,
				Goal:    req.Goal,
				Adapter: req.Adapter,
			}

			// We dispatch asynchronously because RunService.DispatchStep blocks on adapter.Poll
			go func() {
				// In a real robust system, background contexts tied to daemon lifecycle should be used.
				_ = h.RunSvc.DispatchStep(context.Background(), id, step, "/tmp/codencer/artifacts/"+req.ID)
			}()

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(step)
			return
		}
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (h *APIHandler) handleStepByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/v1/steps/"):]
	if id == "" {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodGet {
		step, err := h.RunSvc.GetStep(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if step == nil {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(step)
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
