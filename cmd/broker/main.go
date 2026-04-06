package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	Version       = "0.1.0-alpha"
	DefaultPort   = "8088"
	DefaultHost   = "127.0.0.1"
	ServicePrefix = "AntigravityConnectService"
)

// Instance represents a discovered Antigravity instance.
type Instance struct {
	PID           int    `json:"pid"`
	HTTPSPort     int    `json:"https_port"`
	CSRFToken     string `json:"csrf_token"`
	WorkspaceRoot string `json:"workspace_root,omitempty"`
	IsReachable   bool   `json:"is_reachable"`
}

// Task represents an execution session tracked by the broker.
type Task struct {
	ID        string    `json:"id"`
	CascadeID string    `json:"cascade_id"`
	State     string    `json:"state"` // running, completed, failed, error
	Summary   string    `json:"summary"`
	Instance  Instance  `json:"instance"`
	CreatedAt time.Time `json:"created_at"`
}

// ProxyClient handles Antigravity LS RPC calls.
type ProxyClient struct {
	httpClient *http.Client
}

func NewProxyClient() *ProxyClient {
	return &ProxyClient{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

func (c *ProxyClient) Call(ctx context.Context, inst *Instance, method string, req, resp any) error {
	url := fmt.Sprintf("https://127.0.0.1:%d/%s/%s", inst.HTTPSPort, ServicePrefix, method)
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-codeium-csrf-token", inst.CSRFToken)

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("transport failure: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		return fmt.Errorf("RPC failed (%d): %s", httpResp.StatusCode, string(body))
	}

	return json.NewDecoder(httpResp.Body).Decode(resp)
}

// Discovery handles instance scanning.
type Discovery struct {
	client *ProxyClient
}

func NewDiscovery(client *ProxyClient) *Discovery {
	return &Discovery{client: client}
}

func (d *Discovery) GetInstances(ctx context.Context) ([]Instance, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	daemonDir := filepath.Join(home, ".gemini", "antigravity", "daemon")
	entries, err := os.ReadDir(daemonDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	instances := []Instance{}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "ls_") || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(daemonDir, entry.Name())
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			data, err := os.ReadFile(p)
			if err != nil {
				return
			}

			var inst Instance
			if err := json.Unmarshal(data, &inst); err != nil {
				return
			}

			// Authoritative Probe
			root, ok := d.probeInstance(ctx, inst)
			inst.WorkspaceRoot = root
			inst.IsReachable = ok

			mu.Lock()
			instances = append(instances, inst)
			mu.Unlock()
		}(path)
	}

	wg.Wait()
	return instances, nil
}

func (d *Discovery) probeInstance(ctx context.Context, inst Instance) (string, bool) {
	var info struct {
		WorkspaceInfos []struct {
			WorkspaceUri string `json:"workspaceUri"`
		} `json:"workspaceInfos"`
	}
	if err := d.client.Call(ctx, &inst, "GetWorkspaceInfos", struct{}{}, &info); err != nil {
		return "", false
	}
	if len(info.WorkspaceInfos) > 0 {
		return info.WorkspaceInfos[0].WorkspaceUri, true
	}
	return "", true
}

// BindingRegistry handles persistence of active instances keyed by repo_root.
type BindingRegistry struct {
	mu      sync.RWMutex
	current map[string]*Instance
	path    string
}

func NewBindingRegistry() *BindingRegistry {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".gemini", "antigravity", "broker_binding.json")
	registry := &BindingRegistry{
		path:    path,
		current: make(map[string]*Instance),
	}
	registry.load()
	return registry
}

func (r *BindingRegistry) Set(repoRoot string, inst Instance) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.current[repoRoot] = &inst
	r.save()
}

func (r *BindingRegistry) Get(repoRoot string) *Instance {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.current[repoRoot]
}

func (r *BindingRegistry) Clear(repoRoot string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.current, repoRoot)
	r.save()
}

func (r *BindingRegistry) save() {
	data, _ := json.Marshal(r.current)
	_ = os.MkdirAll(filepath.Dir(r.path), 0755)
	_ = os.WriteFile(r.path, data, 0644)
}

func (r *BindingRegistry) load() {
	data, err := os.ReadFile(r.path)
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, &r.current)
}

// Global Task Registry for Phase 4 (In-memory)
type TaskRegistry struct {
	mu    sync.RWMutex
	tasks map[string]*Task
}

func NewTaskRegistry() *TaskRegistry {
	return &TaskRegistry{tasks: make(map[string]*Task)}
}

func (r *TaskRegistry) Add(t *Task) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tasks[t.ID] = t
}

func (r *TaskRegistry) Get(id string) *Task {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tasks[id]
}

func logger(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next(w, r)
		log.Printf("[Broker] %s %s %v", r.Method, r.URL.Path, time.Since(start))
	}
}

func main() {
	host := os.Getenv("BROKER_HOST")
	if host == "" { host = DefaultHost }
	port := os.Getenv("BROKER_PORT")
	if port == "" { port = DefaultPort }

	client := NewProxyClient()
	discovery := NewDiscovery(client)
	registry := NewBindingRegistry()
	tasks := NewTaskRegistry()

	// Metadata API
	http.HandleFunc("/health", logger(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	http.HandleFunc("/version", logger(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"version": Version})
	}))

	// Binding API
	http.HandleFunc("/instances", logger(func(w http.ResponseWriter, r *http.Request) {
		instances, err := discovery.GetInstances(r.Context())
		if err != nil { http.Error(w, err.Error(), 500); return }
		json.NewEncoder(w).Encode(instances)
	}))

	http.HandleFunc("/binding", logger(func(w http.ResponseWriter, r *http.Request) {
		repoRoot := r.URL.Query().Get("repo_root")
		switch r.Method {
		case "GET":
			if repoRoot == "" { http.Error(w, "repo_root is required", 400); return }
			inst := registry.Get(repoRoot)
			if inst == nil { json.NewEncoder(w).Encode(map[string]string{"status": "unbound"}); return }
			json.NewEncoder(w).Encode(inst)
		case "POST":
			var b struct {
				PID      int    `json:"pid"`
				RepoRoot string `json:"repo_root"`
			}
			if err := json.NewDecoder(r.Body).Decode(&b); err != nil { http.Error(w, "invalid JSON", 400); return }
			if b.RepoRoot == "" { http.Error(w, "repo_root is required", 400); return }
			instances, _ := discovery.GetInstances(r.Context())
			var chosen *Instance
			for _, inst := range instances {
				if inst.PID == b.PID { chosen = &inst; break }
			}
			if chosen == nil { http.Error(w, "instance not found", 404); return }
			registry.Set(b.RepoRoot, *chosen)
			json.NewEncoder(w).Encode(chosen)
		case "DELETE":
			if repoRoot == "" { http.Error(w, "repo_root is required", 400); return }
			registry.Clear(repoRoot)
			w.WriteHeader(204)
		default:
			http.Error(w, "method not allowed", 405)
		}
	}))

	// Task API (Experimental Phase 4)
	http.HandleFunc("/tasks", logger(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" { http.Error(w, "method not allowed", 405); return }
		var b struct {
			Prompt   string `json:"prompt"`
			RepoRoot string `json:"repo_root"`
		}
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil { http.Error(w, "invalid JSON", 400); return }
		if b.RepoRoot == "" { http.Error(w, "repo_root is required", 400); return }

		inst := registry.Get(b.RepoRoot)
		if inst == nil { http.Error(w, "no instance bound for this repo", 400); return }

		req := map[string]any{
			"userPrompt": b.Prompt,
			"workspaceFolderAbsoluteUri": inst.WorkspaceRoot,
			"metadata": map[string]any{"fileAccessGranted": true},
			"cascadeConfig": map[string]any{"plannerConfig": map[string]any{"plannerTypeConfig": map[string]any{"planning": map[string]any{}}}},
		}

		var resp struct{ CascadeId string `json:"cascadeId"` }
		if err := client.Call(r.Context(), inst, "StartCascade", req, &resp); err != nil {
			http.Error(w, fmt.Sprintf("execution start failed: %v", err), 500)
			return
		}

		taskID := strconv.FormatInt(time.Now().UnixNano(), 10)
		task := &Task{
			ID:        taskID,
			CascadeID: resp.CascadeId,
			State:     "running",
			Instance:  *inst,
			CreatedAt: time.Now(),
		}
		tasks.Add(task)
		json.NewEncoder(w).Encode(task)
	}))

	http.HandleFunc("/tasks/", logger(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) < 3 { http.Error(w, "invalid task ID", 400); return }
		taskID := parts[2]
		task := tasks.Get(taskID)
		if task == nil { http.Error(w, "task not found", 404); return }

		// Poll logic
		var poll struct{ Status string `json:"status"` }
		err := client.Call(r.Context(), &task.Instance, "GetCascadeTrajectory", map[string]any{"cascadeId": task.CascadeID}, &poll)
		
		if err != nil {
			task.State = "error"
			task.Summary = fmt.Sprintf("Transport/Poll failure: %v", err)
		} else {
			switch poll.Status {
			case "COMPLETED": task.State = "completed"
			case "FAILED":    task.State = "failed"
			case "ABORTED":   task.State = "cancelled"
			default:           task.State = "running"
			}
		}

		if len(parts) >= 4 && parts[3] == "result" {
			// Request full trajectory
			var result json.RawMessage
			err := client.Call(r.Context(), &task.Instance, "GetCascadeTrajectory", map[string]any{"cascadeId": task.CascadeID}, &result)
			if err != nil {
				http.Error(w, fmt.Sprintf("failed to fetch trajectory: %v", err), 500)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(result)
			return
		}

		json.NewEncoder(w).Encode(task)
	}))

	addr := host + ":" + port
	log.Printf("Antigravity Broker v%s starting on %s\n", Version, addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Broker failed to start: %v (Is another instance running?)", err)
	}
}
