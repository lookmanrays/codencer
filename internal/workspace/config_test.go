package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadWorkspaceConfig_Native(t *testing.T) {
	root := t.TempDir()
	
	// Native config
	nativePath := filepath.Join(root, ".codencer")
	if err := os.Mkdir(nativePath, 0755); err != nil {
		t.Fatal(err)
	}
	content := `{"provisioning": {"copy": [".env"], "symlinks": ["node_modules"]}}`
	if err := os.WriteFile(filepath.Join(nativePath, "workspace.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	spec, err := LoadWorkspaceConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if spec == nil {
		t.Fatal("Expected spec")
	}

	if len(spec.Copy) != 1 || spec.Copy[0] != ".env" {
		t.Errorf("Expected .env in copy, got %v", spec.Copy)
	}
	if len(spec.Symlinks) != 1 || spec.Symlinks[0] != "node_modules" {
		t.Errorf("Expected node_modules in symlinks, got %v", spec.Symlinks)
	}
}

func TestLoadWorkspaceConfig_SpecGrove(t *testing.T) {
	root := t.TempDir()
	
	// grove.yaml
	content := `
workspace:
  setup:
    copy:
      - ".env.shared"
    symlinks:
      - "vendor"
  hooks:
    post_create: "make setup"
`
	if err := os.WriteFile(filepath.Join(root, "grove.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	spec, err := LoadWorkspaceConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if spec == nil {
		t.Fatal("Expected spec")
	}

	if len(spec.Copy) != 1 || spec.Copy[0] != ".env.shared" {
		t.Errorf("Expected .env.shared in copy, got %v", spec.Copy)
	}
	if spec.Hooks.PostCreate != "make setup" {
		t.Errorf("Expected hook, got %s", spec.Hooks.PostCreate)
	}
}

func TestLoadWorkspaceConfig_LegacyGroverc(t *testing.T) {
	root := t.TempDir()
	
	// .groverc.json (Legacy)
	content := `{"symlink": ["node_modules"], "afterCreate": "npm install"}`
	if err := os.WriteFile(filepath.Join(root, ".groverc.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	spec, err := LoadWorkspaceConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if spec == nil {
		t.Fatal("Expected spec")
	}

	if len(spec.Symlinks) != 1 || spec.Symlinks[0] != "node_modules" {
		t.Errorf("Expected node_modules, got %v", spec.Symlinks)
	}
	if spec.Hooks.PostCreate != "npm install" {
		t.Errorf("Expected hook, got %s", spec.Hooks.PostCreate)
	}
}

func TestLoadWorkspaceConfig_PrecedenceChain(t *testing.T) {
	root := t.TempDir()
	
	// Native (Wins on copy)
	nativePath := filepath.Join(root, ".codencer")
	_ = os.Mkdir(nativePath, 0755)
	native := `{"provisioning": {"copy": [".env.native"]}}`
	_ = os.WriteFile(filepath.Join(nativePath, "workspace.json"), []byte(native), 0644)

	// Spec-Grove (Wins on symlinks because Native didn't specify them)
	specGrove := `
workspace:
  setup:
    copy: [".env.grove"]
    symlinks: ["vendor"]
`
	_ = os.WriteFile(filepath.Join(root, "grove.yaml"), []byte(specGrove), 0644)

	// Legacy-Grove (Wins on hooks because Native/Spec didn't specify them)
	legacy := `{"afterCreate": "make all"}`
	_ = os.WriteFile(filepath.Join(root, ".groverc.json"), []byte(legacy), 0644)

	spec, err := LoadWorkspaceConfig(root)
	if err != nil {
		t.Fatal(err)
	}

	// Native overrides Grove's copy
	if len(spec.Copy) != 1 || spec.Copy[0] != ".env.native" {
		t.Errorf("Expected .env.native, got %v", spec.Copy)
	}
	// Spec-Grove's symlinks are merged
	if len(spec.Symlinks) != 1 || spec.Symlinks[0] != "vendor" {
		t.Errorf("Expected vendor, got %v", spec.Symlinks)
	}
	// Legacy-Grove's hook is merged
	if spec.Hooks.PostCreate != "make all" {
		t.Errorf("Expected make all, got %s", spec.Hooks.PostCreate)
	}
}
