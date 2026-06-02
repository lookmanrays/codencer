package mcpconfig

import (
	"strings"
	"testing"
)

func TestGenerateCodexConfig(t *testing.T) {
	payload, err := Generate(Options{Client: "codex", Endpoint: "https://relay.example.com", TokenEnv: "CODENCER_TOKEN", Name: "codencer"})
	if err != nil {
		t.Fatal(err)
	}
	if payload["endpoint"] != "https://relay.example.com/mcp" {
		t.Fatalf("endpoint = %v", payload["endpoint"])
	}
	if command, _ := payload["command"].(string); !strings.Contains(command, "codex mcp add") || !strings.Contains(command, "bearer-token-env-var CODENCER_TOKEN") {
		t.Fatalf("unexpected command: %s", command)
	}
}

func TestGenerateClaudeCodeConfig(t *testing.T) {
	payload, err := Generate(Options{Client: "claude-code", Endpoint: "https://relay.example.com/mcp", TokenEnv: "CODENCER_MCP_TOKEN", Name: "codencer"})
	if err != nil {
		t.Fatal(err)
	}
	command, _ := payload["command"].(string)
	expected := `claude mcp add --transport http --header "Authorization: Bearer $CODENCER_MCP_TOKEN" codencer https://relay.example.com/mcp`
	if command != expected {
		t.Fatalf("unexpected command: %s", command)
	}
	headerIndex := strings.Index(command, "--header")
	nameIndex := strings.Index(command, " codencer ")
	if headerIndex < 0 || nameIndex < 0 || headerIndex > nameIndex {
		t.Fatalf("expected --header before server name: %s", command)
	}
	payload, err = Generate(Options{Client: "claude-code", Endpoint: "https://relay.example.com/mcp", Token: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if payload["token_included"] != true {
		t.Fatalf("token_included = %v", payload["token_included"])
	}
}

func TestGenerateChatGPTConfig(t *testing.T) {
	payload, err := Generate(Options{Client: "chatgpt", Endpoint: "https://relay.example.com/mcp"})
	if err != nil {
		t.Fatal(err)
	}
	if payload["mode"] != "oauth-dev-or-front-door" {
		t.Fatalf("mode = %v", payload["mode"])
	}
	values := payload["connector_values"].(map[string]any)
	if !strings.Contains(values["authorization_server"].(string), "/.well-known/oauth-authorization-server") {
		t.Fatalf("authorization server metadata missing: %+v", values)
	}
}
