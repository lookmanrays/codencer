package app

import (
	"encoding/json"
	"net/http"

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
	mux.HandleFunc("/api/v1/runs/", h.handleRunByID)
	mux.HandleFunc("/api/v1/gates/", h.handleGateByID)
	
	mcpServer := mcp.NewServer(h.RunSvc, h.GateSvc)
	mux.HandleFunc("/mcp/call", mcpServer.HandleCall)
}

func (h *APIHandler) handleRuns(w http.ResponseWriter, r *http.Request) {
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
		run, err := h.RunSvc.GetStatus(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if run == nil {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
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
			if err := h.RunSvc.Abort(r.Context(), id); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
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
