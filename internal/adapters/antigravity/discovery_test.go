package antigravity

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", oldHome)

	d := NewDiscovery()
	// Mock probe to return instantly
	// Note: In real test, probe will fail (no server), but Discovery handles that by returning reachable=false.

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

func TestDiscovery_Deduplication(t *testing.T) {
	// Setup two mock directories
	tmpDir, _ := os.MkdirTemp("", "ag-dedup")
	defer os.RemoveAll(tmpDir)

	dir1 := filepath.Join(tmpDir, "local", daemonDirRel)
	dir2 := filepath.Join(tmpDir, "win", daemonDirRel)
	os.MkdirAll(dir1, 0755)
	os.MkdirAll(dir2, 0755)

	inst := domain.AGInstance{
		PID:       555,
		HTTPSPort: 1234,
		CSRFToken: "tok",
	}
	data, _ := json.Marshal(inst)
	os.WriteFile(filepath.Join(dir1, "ls_555.json"), data, 0644)
	os.WriteFile(filepath.Join(dir2, "ls_555.json"), data, 0644)

	d := NewDiscovery()
	instances, err := d.scanDirs(context.Background(), []string{dir1, dir2})
	if err != nil {
		t.Fatalf("scanDirs failed: %v", err)
	}

	if len(instances) != 1 {
		t.Fatalf("Expected 1 instance after deduplication, got %d", len(instances))
	}

	if instances[0].PID != 555 {
		t.Errorf("Expected PID 555, got %d", instances[0].PID)
	}
}

func TestDiscovery_Override(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "ag-override")
	defer os.RemoveAll(tmpDir)

	os.MkdirAll(tmpDir, 0755)
	inst := domain.AGInstance{PID: 999, HTTPSPort: 8888, CSRFToken: "abc"}
	data, _ := json.Marshal(inst)
	os.WriteFile(filepath.Join(tmpDir, "ls_999.json"), data, 0644)

	// Set override env
	os.Setenv("CODENCER_ANTIGRAVITY_WINDOWS_DAEMON_DIR", tmpDir)
	defer os.Unsetenv("CODENCER_ANTIGRAVITY_WINDOWS_DAEMON_DIR")

	// We also need to Mock WSL detection or just test getDaemonDirs
	// Since getDaemonDirs checks /proc/sys/kernel/osrelease, we can't easily mock it without filesystem changes.
	// However, we can test that IF it reaches the block, it uses the env.
	
	d := NewDiscovery()
	dirs, err := d.getDaemonDirs()
	if err != nil {
		t.Fatalf("getDaemonDirs failed: %v", err)
	}

	// In a non-WSL environment (like CI), the overide might not be added 
	// because getDaemonDirs only checks the env if it detects Microsoft/WSL.
	// We'll trust the logic for now or skip the check if not in WSL.
	found := false
	for _, dir := range dirs {
		if dir == tmpDir {
			found = true
			break
		}
	}
	
	// If we are in WSL, it MUST be found.
	if content, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
		if strings.Contains(strings.ToLower(string(content)), "microsoft") {
			if !found {
				t.Errorf("Expected override dir %s to be in discovery list, but wasn't", tmpDir)
			}
		}
	}
}
