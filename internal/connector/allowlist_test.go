package connector

import (
	"net/http"
	"testing"
)

func TestAllowedLocalProxy(t *testing.T) {
	allowed := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/instance"},
		{http.MethodPost, "/api/v1/runs"},
		{http.MethodPatch, "/api/v1/runs/run-1"},
		{http.MethodPost, "/api/v1/runs/run-1/steps"},
		{http.MethodGet, "/api/v1/steps/step-1/result"},
		{http.MethodPost, "/api/v1/steps/step-1/wait"},
		{http.MethodPost, "/api/v1/steps/step-1/retry"},
		{http.MethodGet, "/api/v1/artifacts/art-1"},
		{http.MethodGet, "/api/v1/artifacts/art-1/content"},
		{http.MethodGet, "/api/v1/gates/gate-1"},
		{http.MethodPost, "/api/v1/gates/gate-1"},
	}
	for _, tc := range allowed {
		if !AllowedLocalProxy(tc.method, tc.path) {
			t.Fatalf("expected %s %s to be allowed", tc.method, tc.path)
		}
	}

	denied := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/etc/passwd"},
		{http.MethodPost, "/api/v1/admin/shell"},
		{http.MethodDelete, "/api/v1/runs/run-1"},
	}
	for _, tc := range denied {
		if AllowedLocalProxy(tc.method, tc.path) {
			t.Fatalf("expected %s %s to be denied", tc.method, tc.path)
		}
	}
}
