package cloud

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestAPITokenHashingAndLookup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cloud.db")
	store, err := OpenStore(path, "cloud-master-key")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	org, err := store.CreateOrg(ctx, Org{Slug: "acme", Name: "Acme"})
	if err != nil {
		t.Fatal(err)
	}

	rawToken, err := GenerateAPIToken()
	if err != nil {
		t.Fatal(err)
	}
	record, err := store.CreateAPIToken(ctx, APIToken{
		OrgID:  org.ID,
		Name:   "planner",
		Kind:   "planner",
		Scopes: []string{"runs:read", "runs:write"},
	}, rawToken)
	if err != nil {
		t.Fatal(err)
	}

	if record.TokenHash == "" {
		t.Fatal("expected token hash")
	}
	if record.TokenHash == rawToken {
		t.Fatal("expected hash to differ from raw token")
	}

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var storedHash string
	if err := db.QueryRow(`SELECT token_hash FROM api_tokens WHERE id = ?`, record.ID).Scan(&storedHash); err != nil {
		t.Fatal(err)
	}
	if storedHash != HashAPIToken(rawToken) {
		t.Fatalf("unexpected stored hash: %s", storedHash)
	}

	found, err := store.LookupAPIToken(ctx, rawToken)
	if err != nil {
		t.Fatal(err)
	}
	if found.ID != record.ID {
		t.Fatalf("expected lookup to return created token, got %s", found.ID)
	}
	if len(found.Scopes) != 2 || found.Scopes[0] != "runs:read" || found.Scopes[1] != "runs:write" {
		t.Fatalf("unexpected scopes: %+v", found.Scopes)
	}

	if _, err := store.LookupAPIToken(ctx, "cct_invalid-token"); err == nil {
		t.Fatal("expected invalid token lookup to fail")
	}
}
