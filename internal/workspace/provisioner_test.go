package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"agent-bridge/internal/domain"
)

func TestProvisioner_Copy(t *testing.T) {
	base := t.TempDir()
	work := t.TempDir()
	
	// Setup source
	envContent := "API_KEY=123"
	if err := os.WriteFile(filepath.Join(base, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatal(err)
	}

	p := NewLocalProvisioner()
	spec := &domain.ProvisioningSpec{
		Copy: []string{".env"},
	}

	res, err := p.Provision(context.Background(), spec, base, work)
	if err != nil {
		t.Fatalf("Provision failed: %v", err)
	}
	if !res.Success {
		t.Errorf("Expected success, got failure: %s", res.Summary)
	}

	// Verify
	got, err := os.ReadFile(filepath.Join(work, ".env"))
	if err != nil {
		t.Fatalf("Could not read copied .env: %v", err)
	}
	if string(got) != envContent {
		t.Errorf("Expected %s, got %s", envContent, string(got))
	}
}

func TestProvisioner_Symlink(t *testing.T) {
	base := t.TempDir()
	work := t.TempDir()
	
	// Setup source dir
	nodeDir := filepath.Join(base, "node_modules")
	if err := os.Mkdir(nodeDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nodeDir, "pkg.js"), []byte("pkg"), 0644); err != nil {
		t.Fatal(err)
	}

	p := NewLocalProvisioner()
	spec := &domain.ProvisioningSpec{
		Symlinks: []string{"node_modules"},
	}

	res, err := p.Provision(context.Background(), spec, base, work)
	if err != nil {
		t.Fatalf("Provision failed: %v", err)
	}
	if !res.Success {
		t.Errorf("Expected success, got failure: %s", res.Summary)
	}

	// Verify symlink
	dst := filepath.Join(work, "node_modules")
	fi, err := os.Lstat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("Destination %s is not a symlink", dst)
	}
}

func TestProvisioner_PathTraversal(t *testing.T) {
	base := t.TempDir()
	work := t.TempDir()
	
	p := NewLocalProvisioner()
	
	// Test parent traversal
	spec := &domain.ProvisioningSpec{
		Copy: []string{"../outside.txt"},
	}
	res, err := p.Provision(context.Background(), spec, base, work)
	if err == nil {
		t.Fatal("Expected error for parent traversal")
	}
	if !strings.Contains(res.Summary, "parent traversal is not allowed") {
		t.Errorf("Expected traversal error summary, got: %s", res.Summary)
	}

	// Test absolute path
	spec = &domain.ProvisioningSpec{
		Copy: []string{"/etc/passwd"},
	}
	res, err = p.Provision(context.Background(), spec, base, work)
	if err == nil {
		t.Fatal("Expected error for absolute path")
	}
	if !strings.Contains(res.Summary, "absolute paths are not allowed") {
		t.Errorf("Expected absolute path error summary, got: %s", res.Summary)
	}
}

func TestProvisioner_Isolation(t *testing.T) {
	base := t.TempDir()
	work1 := t.TempDir()
	work2 := t.TempDir()
	
	// Setup source
	if err := os.WriteFile(filepath.Join(base, "shared.txt"), []byte("base"), 0644); err != nil {
		t.Fatal(err)
	}

	p := NewLocalProvisioner()
	spec := &domain.ProvisioningSpec{
		Copy: []string{"shared.txt"},
	}

	// Attempt 1
	if _, err := p.Provision(context.Background(), spec, base, work1); err != nil {
		t.Fatal(err)
	}
	// Attempt 2
	if _, err := p.Provision(context.Background(), spec, base, work2); err != nil {
		t.Fatal(err)
	}

	// Modify work1
	if err := os.WriteFile(filepath.Join(work1, "shared.txt"), []byte("modified"), 0644); err != nil {
		t.Fatal(err)
	}

	// Check base hasn't changed
	baseContent, _ := os.ReadFile(filepath.Join(base, "shared.txt"))
	if string(baseContent) != "base" {
		t.Error("Base repository was modified by attempt")
	}

	// Check work2 hasn't changed
	work2Content, _ := os.ReadFile(filepath.Join(work2, "shared.txt"))
	if string(work2Content) != "base" {
		t.Error("Attempt 2 was modified by Attempt 1")
	}
}

func TestProvisioner_Hooks(t *testing.T) {
	base := t.TempDir()
	work := t.TempDir()
	
	p := NewLocalProvisioner()
	spec := &domain.ProvisioningSpec{
		Hooks: domain.ProvisioningHooks{
			PostCreate: "echo 'hello' > hook_out",
		},
	}

	res, err := p.Provision(context.Background(), spec, base, work)
	if err != nil {
		t.Fatalf("Provision failed: %v", err)
	}
	if !res.Success {
		t.Fatal("Expected success")
	}

	// Verify file created by hook
	got, err := os.ReadFile(filepath.Join(work, "hook_out"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(got)) != "hello" {
		t.Errorf("Hook output mismatch: %s", string(got))
	}
}
