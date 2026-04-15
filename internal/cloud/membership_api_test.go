package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestMembershipLifecycleAndRoleScopedTokenIssuance(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "cloud.db"), "cloud-master-key")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	org, workspace, project := seedCloudScope(t, store, ctx)

	adminRaw, err := GenerateAPIToken()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateAPIToken(ctx, APIToken{
		OrgID:       org.ID,
		WorkspaceID: workspace.ID,
		ProjectID:   project.ID,
		Name:        "bootstrap",
		Scopes: []string{
			"cloud:admin",
			"memberships:read", "memberships:write",
			"tokens:read", "tokens:write",
			"cloud:read",
		},
	}, adminRaw); err != nil {
		t.Fatal(err)
	}

	server := NewServer(DefaultConfig(), store, nil, nil)
	handler := server.Handler()

	createMembership := func(role string) Membership {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/cloud/v1/memberships", bytes.NewReader(mustJSON(map[string]any{
			"org_id":       org.ID,
			"workspace_id": workspace.ID,
			"project_id":   project.ID,
			"name":         role,
			"role":         role,
		})))
		req.Header.Set("Authorization", "Bearer "+adminRaw)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("expected membership create ok, got %d body=%s", rr.Code, rr.Body.String())
		}
		var membership Membership
		if err := json.NewDecoder(rr.Body).Decode(&membership); err != nil {
			t.Fatal(err)
		}
		return membership
	}

	viewer := createMembership(RoleProjectViewer)
	operator := createMembership(RoleProjectOperator)

	req := httptest.NewRequest(http.MethodPost, "/api/cloud/v1/tokens", bytes.NewReader(mustJSON(map[string]any{
		"org_id":        org.ID,
		"workspace_id":  workspace.ID,
		"project_id":    project.ID,
		"membership_id": viewer.ID,
		"name":          "viewer-write",
		"scopes":        []string{"installations:write"},
	})))
	req.Header.Set("Authorization", "Bearer "+adminRaw)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid scopes for viewer role, got %d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/cloud/v1/tokens", bytes.NewReader(mustJSON(map[string]any{
		"org_id":        org.ID,
		"workspace_id":  workspace.ID,
		"project_id":    project.ID,
		"membership_id": operator.ID,
		"name":          "operator",
		"scopes":        []string{"installations:write", "runtime_connectors:write"},
	})))
	req.Header.Set("Authorization", "Bearer "+adminRaw)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected operator token create ok, got %d body=%s", rr.Code, rr.Body.String())
	}
	var tokenPayload struct {
		Token  string   `json:"token"`
		Record APIToken `json:"record"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&tokenPayload); err != nil {
		t.Fatal(err)
	}
	if tokenPayload.Record.MembershipID != operator.ID {
		t.Fatalf("expected membership-linked token, got %+v", tokenPayload.Record)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/cloud/v1/memberships/"+operator.ID+"/disable", nil)
	req.Header.Set("Authorization", "Bearer "+adminRaw)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected disable membership ok, got %d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/cloud/v1/status", nil)
	req.Header.Set("Authorization", "Bearer "+tokenPayload.Token)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected disabled membership token to fail auth, got %d body=%s", rr.Code, rr.Body.String())
	}
}
