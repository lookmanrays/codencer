package projectconfig

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDefaultProjectConfigValidates(t *testing.T) {
	cfg := Default(DefaultOptions{ProjectID: "codencer", ProjectName: "Codencer"})
	if _, err := Validate(cfg); err != nil {
		t.Fatalf("default config should validate: %v", err)
	}
	if cfg.Workspace.Root != "." {
		t.Fatalf("workspace root = %q", cfg.Workspace.Root)
	}
}

func TestProjectConfigRejectsAbsolutePathsAndTokenLikeFields(t *testing.T) {
	cfg := Default(DefaultOptions{ProjectID: "codencer", ProjectName: "Codencer"})
	cfg.Workspace.Root = "/Users/example/repo"
	if _, err := Validate(cfg); err == nil {
		t.Fatal("expected absolute workspace root to be rejected")
	}

	raw := map[string]any{
		"version": APIVersion,
		"kind":    Kind,
		"project": map[string]any{
			"id":    "codencer",
			"name":  "Codencer",
			"token": "secret-token",
		},
		"execution": map[string]any{
			"default_adapter": "codex",
			"default_profile": "codex-workspace",
		},
		"workspace": map[string]any{
			"root":            ".",
			"forbidden_paths": []any{".env"},
		},
	}
	cfg = Default(DefaultOptions{ProjectID: "codencer", ProjectName: "Codencer"})
	if _, err := ValidateWithRaw(cfg, raw); err == nil {
		t.Fatal("expected token-like field to be rejected")
	}
}

func TestProjectScanIsReadOnly(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module example.test/codencer\n"), 0644); err != nil {
		t.Fatal(err)
	}
	before := listTree(t, repo)
	proposal, err := Scan(repo)
	if err != nil {
		t.Fatal(err)
	}
	after := listTree(t, repo)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("scan modified tree: before=%v after=%v", before, after)
	}
	if !proposal.ReadOnly || proposal.SuggestedProjectID == "" {
		t.Fatalf("unexpected scan proposal: %+v", proposal)
	}
}

func listTree(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out = append(out, rel)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return out
}
