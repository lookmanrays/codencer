package antigravity

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"agent-bridge/internal/domain"
)

func TestDiscovery_Discover(t *testing.T) {
	// Setup mock daemon dir
	tmpHome, _ := os.MkdirTemp("", "ag-home")
	defer os.RemoveAll(tmpHome)

	daemonDir := filepath.Join(tmpHome, daemonDirRel)
	os.MkdirAll(daemonDir, 0755)

	// Create mock instance file
	inst := domain.AGInstance{
		PID:       12345,
		HTTPSPort: 9999,
		CSRFToken: "mock-token",
	}
	data, _ := json.Marshal(inst)
	os.WriteFile(filepath.Join(daemonDir, "ls_12345.json"), data, 0644)

	// Point discovery to tmp home
	// We need to modify Discovery to accept a base path or just set HOME
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", oldHome)

	d := NewDiscovery()
	instances, err := d.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	if len(instances) != 1 {
		t.Fatalf("Expected 1 instance, got %d", len(instances))
	}

	if instances[0].PID != 12345 {
		t.Errorf("Expected PID 12345, got %d", instances[0].PID)
	}
}
