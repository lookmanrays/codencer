package antigravity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"agent-bridge/internal/domain"
)

func TestBrokerAdapter_IdentitySeparation(t *testing.T) {
	// 1. Setup a test server to capture the 'repo_root' identity in the request body
	var lastRepoRoot string
	var lastWorkspaceRoot string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/tasks" && r.Method == "POST" {
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			lastRepoRoot = body["repo_root"]
			lastWorkspaceRoot = body["workspace_root"]

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"id": "t1"})
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	// 2. Scenario: Multi-repo use on one machine
	// Repo A (stable root /home/user/repo-a)
	adapterA := NewBrokerAdapter(server.URL, "/home/user/repo-a")
	// Repo B (stable root /home/user/repo-b)
	adapterB := NewBrokerAdapter(server.URL, "/home/user/repo-b")

	ctx := context.Background()
	step := &domain.Step{ID: "s1", Goal: "test"}
	attempt := &domain.Attempt{ID: "att-1"}

	// 3. Execute Repo A in a specific worktree
	worktreeA := "/tmp/worktree-a-123"
	err := adapterA.Start(ctx, step, attempt, worktreeA, "/tmp/artifacts")
	if err != nil {
		t.Fatalf("Repo A start failed: %v", err)
	}

	if lastRepoRoot != "/home/user/repo-a" {
		t.Errorf("Repo A: Expected repo_root /home/user/repo-a, got %s", lastRepoRoot)
	}
	if lastWorkspaceRoot != worktreeA {
		t.Errorf("Repo A: Expected workspace_root %s, got %s", worktreeA, lastWorkspaceRoot)
	}

	// 4. Execute Repo B in a specific worktree
	worktreeB := "/tmp/worktree-b-456"
	err = adapterB.Start(ctx, step, attempt, worktreeB, "/tmp/artifacts")
	if err != nil {
		t.Fatalf("Repo B start failed: %v", err)
	}

	if lastRepoRoot != "/home/user/repo-b" {
		t.Errorf("Repo B: Expected repo_root /home/user/repo-b, got %s", lastRepoRoot)
	}
	if lastWorkspaceRoot != worktreeB {
		t.Errorf("Repo B: Expected workspace_root %s, got %s", worktreeB, lastWorkspaceRoot)
	}
}
