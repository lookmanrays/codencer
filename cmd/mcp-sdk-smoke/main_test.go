package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestRunRequiresTokenBeforeConnecting(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run(context.Background(), []string{"--endpoint", "http://127.0.0.1:1/mcp"}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "--token is required") {
		t.Fatalf("expected missing token error, got %v", err)
	}
}

func TestSmokeOutputUsesOperatorFriendlyToolNamesJSON(t *testing.T) {
	data, err := json.Marshal(smokeOutput{ToolNames: []string{"codencer.list_instances"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"tool_names"`) {
		t.Fatalf("expected tool_names JSON field, got %s", string(data))
	}
}
