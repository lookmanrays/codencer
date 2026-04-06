package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBindingRegistry(t *testing.T) {
	// Setup tmp binding path
	tmpDir, _ := os.MkdirTemp("", "ag-broker-test")
	defer os.RemoveAll(tmpDir)

	path := filepath.Join(tmpDir, "binding.json")
	registry := &BindingRegistry{
		path:    path,
		current: make(map[string]*Instance),
	}

	repoA := "/path/to/repo/a"
	repoB := "/path/to/repo/b"

	// 1. Initial State
	if registry.Get(repoA) != nil {
		t.Errorf("Expected initial state to be nil, got %v", registry.Get(repoA))
	}

	// 2. Set Bindings for separate repos
	instA := Instance{PID: 111, HTTPSPort: 8081, CSRFToken: "tokenA"}
	instB := Instance{PID: 222, HTTPSPort: 8082, CSRFToken: "tokenB"}
	
	registry.Set(repoA, instA)
	registry.Set(repoB, instB)

	if registry.Get(repoA) == nil || registry.Get(repoA).PID != 111 {
		t.Errorf("Expected Repo A PID 111, got %v", registry.Get(repoA))
	}
	if registry.Get(repoB) == nil || registry.Get(repoB).PID != 222 {
		t.Errorf("Expected Repo B PID 222, got %v", registry.Get(repoB))
	}

	// 3. Verify Persistence on re-load
	registry2 := &BindingRegistry{path: path}
	registry2.load()
	if registry2.Get(repoA) == nil || registry2.Get(repoA).PID != 111 {
		t.Errorf("Expected Repo A PID 111 after re-load, got %v", registry2.Get(repoA))
	}
	if registry2.Get(repoB) == nil || registry2.Get(repoB).PID != 222 {
		t.Errorf("Expected Repo B PID 222 after re-load, got %v", registry2.Get(repoB))
	}

	// 4. Clear Repo A
	registry.Clear(repoA)
	if registry.Get(repoA) != nil {
		t.Errorf("Expected Repo A nil after clear, got %v", registry.Get(repoA))
	}
	// Verify Repo B still exists
	if registry.Get(repoB) == nil || registry.Get(repoB).PID != 222 {
		t.Errorf("Expected Repo B to still exist after clearing A, got %v", registry.Get(repoB))
	}

	// 5. Verify Persistence after clear
	registry3 := &BindingRegistry{path: path}
	registry3.load()
	if registry3.Get(repoA) != nil {
		t.Errorf("Expected Repo A nil after clear re-load, got %v", registry3.Get(repoA))
	}
	if registry3.Get(repoB) == nil {
		t.Errorf("Expected Repo B to still exist after clear re-load")
	}
}

func TestDiscovery_JSONParsing(t *testing.T) {
	// Simple test to ensure main.go data structures match our expectations
	data := `{"pid": 123, "https_port": 456, "csrf_token": "abc"}`
	var inst Instance
	if err := json.Unmarshal([]byte(data), &inst); err != nil {
		t.Fatalf("Unmarshalling failed: %v", err)
	}
	if inst.PID != 123 || inst.HTTPSPort != 456 || inst.CSRFToken != "abc" {
		t.Errorf("Mismatch in unmarshalled data: %+v", inst)
	}
}
