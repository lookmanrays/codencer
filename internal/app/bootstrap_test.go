package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"agent-bridge/internal/domain"
)

func TestBootstrap(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	artifactRoot := filepath.Join(tmpDir, "artifacts")

	content := `{
		"db_path": "` + dbPath + `",
		"artifact_root": "` + artifactRoot + `",
		"port": 0
	}`

	configFile := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	appCtx, err := Bootstrap(context.Background(), configFile, "")
	if err != nil {
		t.Fatalf("failed to bootstrap: %v", err)
	}
	defer appCtx.Close()

	// Check if artifact root was created
	stat, err := os.Stat(artifactRoot)
	if err != nil {
		t.Fatalf("artifact root not created: %v", err)
	}
	if !stat.IsDir() {
		t.Errorf("artifact root is not a directory")
	}

	// Check if DB exists
	_, err = os.Stat(dbPath)
	if err != nil {
		t.Fatalf("database file not created: %v", err)
	}

	// Test health endpoint using httptest
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	appCtx.Server.Handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	expected := `{"status":"ok"}`
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
	}
}

func TestBootstrap_RepoRootOverride(t *testing.T) {
	repoRoot := t.TempDir()
	dbRelative := "subdir/codencer.db"

	content := `{
		"db_path": "` + dbRelative + `",
		"artifact_root": "arts",
		"workspace_root": "ws",
		"port": 0
	}`

	configFile := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	appCtx, err := Bootstrap(context.Background(), configFile, repoRoot)
	if err != nil {
		t.Fatalf("failed to bootstrap: %v", err)
	}
	defer appCtx.Close()

	expectedDB := filepath.Join(repoRoot, dbRelative)
	if appCtx.Config.DBPath != expectedDB {
		t.Errorf("expected DBPath %s, got %s", expectedDB, appCtx.Config.DBPath)
	}

	if appCtx.RepoRoot != repoRoot {
		t.Errorf("expected RepoRoot %s, got %s", repoRoot, appCtx.RepoRoot)
	}

	// Verify directories were created relative to repoRoot
	if _, err := os.Stat(filepath.Join(repoRoot, "arts")); err != nil {
		t.Errorf("artifact root not created in repo root: %v", err)
	}
}

func TestBootstrap_PersistsStableInstanceID(t *testing.T) {
	repoRoot := t.TempDir()
	configFile := filepath.Join(t.TempDir(), "config.json")
	content := `{
		"db_path": ".codencer/test.db",
		"artifact_root": ".codencer/artifacts",
		"workspace_root": ".codencer/workspace",
		"port": 0
	}`
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	appCtx1, err := Bootstrap(context.Background(), configFile, repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	instanceID1 := appCtx1.InstanceID
	_ = appCtx1.Close()

	appCtx2, err := Bootstrap(context.Background(), configFile, repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer appCtx2.Close()

	if instanceID1 == "" {
		t.Fatal("expected first bootstrap to persist an instance ID")
	}
	if appCtx2.InstanceID != instanceID1 {
		t.Fatalf("expected stable instance ID %s, got %s", instanceID1, appCtx2.InstanceID)
	}

	stateDir := filepath.Join(repoRoot, ".codencer")
	manifestPath := filepath.Join(stateDir, "instance.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("expected manifest to be written: %v", err)
	}
	var info domain.InstanceInfo
	if err := json.Unmarshal(data, &info); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if info.ID != instanceID1 {
		t.Fatalf("expected manifest instance id %s, got %s", instanceID1, info.ID)
	}
	if info.RepoRoot != repoRoot {
		t.Fatalf("expected manifest repo root %s, got %s", repoRoot, info.RepoRoot)
	}
	if info.WorkspaceRoot != filepath.Join(repoRoot, ".codencer/workspace") {
		t.Fatalf("expected manifest workspace root to be absolute, got %s", info.WorkspaceRoot)
	}
	if info.BaseURL != "http://127.0.0.1:0" {
		t.Fatalf("expected manifest base url http://127.0.0.1:0, got %s", info.BaseURL)
	}
	if info.ManifestPath != manifestPath {
		t.Fatalf("expected manifest path %s, got %s", manifestPath, info.ManifestPath)
	}
}
