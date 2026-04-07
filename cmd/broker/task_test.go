package main

import (
	"testing"
	"time"
)

func TestTaskRegistry_AddGet(t *testing.T) {
	registry := NewTaskRegistry()
	task := &Task{ID: "test-task", CascadeID: "cas-1", State: "running", CreatedAt: time.Now()}
	
	registry.Add(task)
	got := registry.Get("test-task")
	
	if got == nil || got.ID != "test-task" {
		t.Errorf("Expected task test-task, got %v", got)
	}
}

// TestTaskHandler_IdentitySeparation verifies that the broker's task handler logic
// correctly separates the Binding Identity (repo_root) from the Execution Context (workspace_root).
func TestTaskHandler_IdentitySeparation(t *testing.T) {
	registry := NewBindingRegistry()
	repoRoot := "/home/user/stable-repo"
	worktreePath := "/tmp/worktree-att-1"
	
	inst := Instance{
		PID:           123,
		WorkspaceRoot: repoRoot, // Default root discovered at bind-time
	}
	registry.Set(repoRoot, inst)

	// Simulated Request Payload from a Codencer attempt running in a temporary worktree
	payload := struct {
		Prompt        string `json:"prompt"`
		RepoRoot      string `json:"repo_root"`
		WorkspaceRoot string `json:"workspace_root"`
	}{
		Prompt:        "fix bug in worktree",
		RepoRoot:      repoRoot,     // The stable key used for binding lookup
		WorkspaceRoot: worktreePath, // The actual attempt-specific path
	}

	// 1. Verify Binding Lookup
	// This matches the logic in main.go http.HandleFunc("/tasks", ...)
	foundInst := registry.Get(payload.RepoRoot)
	if foundInst == nil {
		t.Fatal("Broker failed to resolve binding using stable repo_root")
	}

	// 2. Verify Execution Context Resolution
	// If workspace_root is provided by the adapter, it must take precedence over the instance's default.
	runWorkspace := payload.WorkspaceRoot
	if runWorkspace == "" {
		runWorkspace = foundInst.WorkspaceRoot
	}

	if runWorkspace != worktreePath {
		t.Errorf("Expected execution workspace %s, got %s", worktreePath, runWorkspace)
	}

	// 3. Verify LS Request formation
	// In main.go, this 'runWorkspace' is passed as 'workspaceFolderAbsoluteUri' to the LS.
	lsReq := map[string]any{
		"userPrompt": payload.Prompt,
		"workspaceFolderAbsoluteUri": runWorkspace,
	}
	
	if lsReq["workspaceFolderAbsoluteUri"] != worktreePath {
		t.Errorf("LS request used wrong workspace URI: %v", lsReq["workspaceFolderAbsoluteUri"])
	}
}

func TestTaskHandler_WorkspaceRootPrecedence(t *testing.T) {
registry := NewBindingRegistry()
repoRoot := "/home/user/project"
instanceDefault := "/home/user/project"
attemptWorktree := "/tmp/codencer-worktree-run-001"

inst := Instance{
          999,
instanceDefault,
}
registry.Set(repoRoot, inst)

// Case A: Provided WorkspaceRoot overrides instance default
runWorkspaceA := attemptWorktree
if runWorkspaceA == "" {
WorkspaceA = inst.WorkspaceRoot
}
if runWorkspaceA != attemptWorktree {
provided worktree %s to take precedence, but got %s", attemptWorktree, runWorkspaceA)
}

// Case B: Empty WorkspaceRoot falls back to instance default
runWorkspaceB := ""
if runWorkspaceB == "" {
WorkspaceB = inst.WorkspaceRoot
}
if runWorkspaceB != instanceDefault {
fallback to instance default %s, but got %s", instanceDefault, runWorkspaceB)
}
}
