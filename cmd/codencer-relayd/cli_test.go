package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestRunAuditUsesLimitQuery(t *testing.T) {
	var gotAuth string
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`[{"action":"abort_run"},{"action":"disable_connector"}]`))
	}))
	defer server.Close()

	output := captureStdout(t, func() {
		if err := run([]string{"audit", "--relay-url", server.URL, "--token", "planner-token", "--limit", "2"}); err != nil {
			t.Fatalf("run audit: %v", err)
		}
	})

	if gotAuth != "Bearer planner-token" {
		t.Fatalf("expected bearer auth header, got %q", gotAuth)
	}
	if gotQuery != "limit=2" {
		t.Fatalf("expected limit query, got %q", gotQuery)
	}
	if !strings.Contains(output, `"action": "abort_run"`) {
		t.Fatalf("expected pretty audit output, got %s", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	defer func() { os.Stdout = original }()

	fn()

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
