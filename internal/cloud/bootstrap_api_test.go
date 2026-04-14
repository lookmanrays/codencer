package cloud

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	cloudconnectors "agent-bridge/internal/cloud/connectors"
)

func TestServerBootstrapFlowsAndInstallationRollback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cloud.db")
	store, err := OpenStore(path, "")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	org, err := store.CreateOrg(ctx, Org{Slug: "acme", Name: "Acme"})
	if err != nil {
		t.Fatal(err)
	}
	adminRaw, err := GenerateAPIToken()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateAPIToken(ctx, APIToken{
		OrgID: org.ID,
		Name:  "admin",
		Kind:  "bootstrap",
		Scopes: []string{
			"cloud:read",
			"orgs:read",
			"workspaces:read", "workspaces:write",
			"projects:read", "projects:write",
			"tokens:read", "tokens:write",
			"installations:read", "installations:write",
			"events:read", "audit:read",
			"cloud:admin",
		},
	}, adminRaw); err != nil {
		t.Fatal(err)
	}

	server := NewServer(DefaultConfig(), store, cloudconnectors.NewRegistry(), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"relay": "ok"})
	}))
	handler := server.Handler()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/cloud/v1/orgs", nil)
	req.Header.Set("Authorization", "Bearer "+adminRaw)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected org list ok, got %d body=%s", rr.Code, rr.Body.String())
	}
	var orgs []Org
	if err := json.NewDecoder(rr.Body).Decode(&orgs); err != nil {
		t.Fatal(err)
	}
	if len(orgs) != 1 || orgs[0].ID != org.ID {
		t.Fatalf("expected seeded org in list, got %+v", orgs)
	}

	workspace := createWorkspaceViaHTTP(t, handler, adminRaw, Workspace{OrgID: org.ID, Slug: "platform", Name: "Platform"})
	project := createProjectViaHTTP(t, handler, adminRaw, Project{OrgID: org.ID, WorkspaceID: workspace.ID, Slug: "core", Name: "Core"})

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/cloud/v1/status", nil)
	req.Header.Set("Authorization", "Bearer "+adminRaw)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status ok, got %d body=%s", rr.Code, rr.Body.String())
	}
	var status cloudStatusResponse
	if err := json.NewDecoder(rr.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	if !status.RelayComposed {
		t.Fatalf("expected composed relay handler in status: %+v", status)
	}

	tokenBody := map[string]any{
		"org_id":       org.ID,
		"workspace_id": workspace.ID,
		"project_id":   project.ID,
		"name":         "project-operator",
		"kind":         "project",
		"scopes":       []string{"installations:read"},
	}
	tokenRaw := marshalJSON(t, tokenBody)
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/cloud/v1/tokens", strings.NewReader(string(tokenRaw)))
	req.Header.Set("Authorization", "Bearer "+adminRaw)
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected token create ok, got %d body=%s", rr.Code, rr.Body.String())
	}

	unsupportedInstall := map[string]any{
		"org_id":        org.ID,
		"workspace_id":  workspace.ID,
		"project_id":    project.ID,
		"connector_key": "unknown-provider",
		"name":          "bad",
	}
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/cloud/v1/installations", strings.NewReader(string(marshalJSON(t, unsupportedInstall))))
	req.Header.Set("Authorization", "Bearer "+adminRaw)
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected unsupported connector rejection, got %d body=%s", rr.Code, rr.Body.String())
	}

	secretInstall := map[string]any{
		"org_id":        org.ID,
		"workspace_id":  workspace.ID,
		"project_id":    project.ID,
		"connector_key": string(cloudconnectors.ProviderGitHub),
		"name":          "needs-secret",
		"secrets": map[string]string{
			"token": "ghp_example",
		},
	}
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/cloud/v1/installations", strings.NewReader(string(marshalJSON(t, secretInstall))))
	req.Header.Set("Authorization", "Bearer "+adminRaw)
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected secret store failure without master key, got %d body=%s", rr.Code, rr.Body.String())
	}

	installations, err := store.ListConnectorInstallations(ctx, org.ID, workspace.ID, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(installations) != 0 {
		t.Fatalf("expected failed installation to roll back cleanly, got %+v", installations)
	}
}

func createWorkspaceViaHTTP(t *testing.T, handler http.Handler, bearer string, payload Workspace) Workspace {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/cloud/v1/workspaces", strings.NewReader(string(marshalJSON(t, payload))))
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected create workspace ok, got %d body=%s", rr.Code, rr.Body.String())
	}
	var created Workspace
	if err := json.NewDecoder(rr.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	return created
}

func createProjectViaHTTP(t *testing.T, handler http.Handler, bearer string, payload Project) Project {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/cloud/v1/projects", strings.NewReader(string(marshalJSON(t, payload))))
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected create project ok, got %d body=%s", rr.Code, rr.Body.String())
	}
	var created Project
	if err := json.NewDecoder(rr.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	return created
}

func marshalJSON(t *testing.T, payload any) []byte {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
