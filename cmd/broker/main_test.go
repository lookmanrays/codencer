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
	registry := &BindingRegistry{path: path}

	// 1. Initial State
	if registry.Get() != nil {
		t.Errorf("Expected initial state to be nil, got %v", registry.Get())
	}

	// 2. Set Binding
	inst := Instance{PID: 12345, HTTPSPort: 8080, CSRFToken: "token"}
	registry.Set(inst)

	if registry.Get() == nil || registry.Get().PID != 12345 {
		t.Errorf("Expected PID 12345, got %v", registry.Get())
	}

	// 3. Verify Persistence on re-load
	registry2 := &BindingRegistry{path: path}
	registry2.load()
	if registry2.Get() == nil || registry2.Get().PID != 12345 {
		t.Errorf("Expected PID 12345 after re-load, got %v", registry2.Get())
	}

	// 4. Clear
	registry.Clear()
	if registry.Get() != nil {
		t.Errorf("Expected nil after clear, got %v", registry.Get())
	}

	// 5. Verify Persistence after clear
	registry3 := &BindingRegistry{path: path}
	registry3.load()
	if registry3.Get() != nil {
		t.Errorf("Expected nil after clear re-load, got %v", registry3.Get())
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
