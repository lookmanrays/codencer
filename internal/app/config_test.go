package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigDefault(t *testing.T) {
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("expected no error for empty config path, got %v", err)
	}

	if cfg.DBPath == "" {
		t.Errorf("expected default DBPath, got empty string")
	}
	if cfg.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Port)
	}
}

func TestLoadConfigCustom(t *testing.T) {
	content := `{
		"db_path": "custom.db",
		"artifact_root": "/tmp/art",
		"port": 9090
	}`
	
	tmpFile := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(tmpFile)
	if err != nil {
		t.Fatalf("expected no error loading file, got %v", err)
	}

	if cfg.DBPath != "custom.db" {
		t.Errorf("expected DBPath custom.db, got %s", cfg.DBPath)
	}
	if cfg.ArtifactRoot != "/tmp/art" {
		t.Errorf("expected ArtifactRoot /tmp/art, got %s", cfg.ArtifactRoot)
	}
	if cfg.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Port)
	}
}

func TestConfigValidation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DBPath = ""
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation to fail for empty DBPath")
	}

	cfg = DefaultConfig()
	cfg.ArtifactRoot = ""
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation to fail for empty ArtifactRoot")
	}
}
