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
	payload, err := Generate(Options{Client: "claude-code", Endpoint: "https://relay.example.com/mcp", Token: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if command, _ := payload["command"].(string); !strings.Contains(command, "claude mcp add") || !strings.Contains(command, "Authorization: Bearer secret") {
		t.Fatalf("unexpected command: %s", command)
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
	if payload["mode"] != "oauth-front-door" {
		t.Fatalf("mode = %v", payload["mode"])
	}
}
