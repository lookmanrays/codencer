package relay_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"agent-bridge/internal/relay"
)

func TestPlannerScopeDeniesWriteWithReadOnlyToken(t *testing.T) {
	t.Parallel()

	store, err := relay.OpenStore(filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	server := relay.NewServer(&relay.Config{
		DBPath: filepath.Join(t.TempDir(), "unused.db"),
		PlannerTokens: []relay.PlannerTokenConfig{{
			Name:   "read-only",
			Token:  "read-token",
			Scopes: []string{"instances:read"},
		}},
	}, store)

	req := httptest.NewRequest(http.MethodPost, "/api/v2/instances/inst-1/runs", nil)
	req.Header.Set("Authorization", "Bearer read-token")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPlannerInstanceRestrictionDeniesOtherInstances(t *testing.T) {
	t.Parallel()

	store, err := relay.OpenStore(filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	server := relay.NewServer(&relay.Config{
		DBPath: filepath.Join(t.TempDir(), "unused.db"),
		PlannerTokens: []relay.PlannerTokenConfig{{
			Name:        "scoped",
			Token:       "scoped-token",
			Scopes:      []string{"runs:*"},
			InstanceIDs: []string{"inst-allowed"},
		}},
	}, store)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/instances/inst-denied/runs", nil)
	req.Header.Set("Authorization", "Bearer scoped-token")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPlannerProjectRestrictionDeniesOtherProjects(t *testing.T) {
	t.Parallel()

	store, err := relay.OpenStore(filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.SaveConnector(context.Background(), "connector-1", "machine-1", "pub", "label"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReplaceConnectorProjects(context.Background(), "connector-1", []relay.ProjectRecord{{
		ProjectID:   "proj-denied",
		ConnectorID: "connector-1",
		InstanceID:  "inst-1",
		RepoRoot:    "/repo",
		ProjectJSON: `{"id":"proj-denied"}`,
	}}); err != nil {
		t.Fatal(err)
	}

	server := relay.NewServer(&relay.Config{
		DBPath: filepath.Join(t.TempDir(), "unused.db"),
		PlannerTokens: []relay.PlannerTokenConfig{{
			Name:       "project-scoped",
			Token:      "project-token",
			Scopes:     []string{"projects:read"},
			ProjectIDs: []string{"proj-allowed"},
		}},
	}, store)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/projects/proj-denied", nil)
	req.Header.Set("Authorization", "Bearer project-token")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPlannerAdminScopeRequiredForRelayStatus(t *testing.T) {
	t.Parallel()

	store, err := relay.OpenStore(filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	server := relay.NewServer(&relay.Config{
		DBPath: filepath.Join(t.TempDir(), "unused.db"),
		PlannerTokens: []relay.PlannerTokenConfig{{
			Name:   "instances-only",
			Token:  "instances-token",
			Scopes: []string{"instances:read"},
		}},
	}, store)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/status", nil)
	req.Header.Set("Authorization", "Bearer instances-token")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPlannerBearerSchemeIsCaseInsensitive(t *testing.T) {
	t.Parallel()

	store, err := relay.OpenStore(filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	server := relay.NewServer(&relay.Config{
		DBPath:       filepath.Join(t.TempDir(), "unused.db"),
		PlannerToken: "planner-token",
	}, store)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/status", nil)
	req.Header.Set("Authorization", "bearer planner-token")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestServeAsPlannerAllowsTrustedInProcessPrincipal(t *testing.T) {
	t.Parallel()

	store, err := relay.OpenStore(filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	server := relay.NewServer(&relay.Config{
		DBPath: filepath.Join(t.TempDir(), "unused.db"),
	}, store)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/status", nil)
	rec := httptest.NewRecorder()
	server.ServeAsPlanner(rec, req, "cloud", []string{"admin:read"}, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestServeAsPlannerStillEnforcesScopeAndInstanceRestrictions(t *testing.T) {
	t.Parallel()

	store, err := relay.OpenStore(filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	server := relay.NewServer(&relay.Config{
		DBPath: filepath.Join(t.TempDir(), "unused.db"),
	}, store)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/instances/inst-denied/runs", nil)
	rec := httptest.NewRecorder()
	server.ServeAsPlanner(rec, req, "cloud", []string{"runs:*"}, []string{"inst-allowed"})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}
