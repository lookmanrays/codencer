package mcpconfig

import (
	"fmt"
	"strings"
)

type Options struct {
	Client   string
	Endpoint string
	TokenEnv string
	Token    string
	Name     string
}

func Generate(opts Options) (map[string]any, error) {
	client := strings.TrimSpace(opts.Client)
	if client == "" {
		client = "codex"
	}
	name := strings.TrimSpace(opts.Name)
	if name == "" {
		name = "codencer"
	}
	tokenEnv := strings.TrimSpace(opts.TokenEnv)
	if tokenEnv == "" {
		tokenEnv = "CODENCER_PLANNER_TOKEN"
	}
	endpointValue := strings.TrimRight(strings.TrimSpace(opts.Endpoint), "/")
	if endpointValue == "" {
		return nil, fmt.Errorf("--endpoint is required")
	}
	if !strings.HasSuffix(endpointValue, "/mcp") {
		endpointValue += "/mcp"
	}

	authValue := "$" + tokenEnv
	if opts.Token != "" {
		authValue = opts.Token
	}
	baseURL := strings.TrimSuffix(endpointValue, "/mcp")
	payload := map[string]any{
		"client":         client,
		"name":           name,
		"endpoint":       endpointValue,
		"token_env":      tokenEnv,
		"metadata_url":   baseURL + "/.well-known/oauth-protected-resource/mcp",
		"token_included": opts.Token != "",
	}

	switch client {
	case "codex":
		payload["command"] = fmt.Sprintf("codex mcp add %s --url %s --bearer-token-env-var %s", shellQuote(name), shellQuote(endpointValue), shellQuote(tokenEnv))
		payload["config_toml"] = fmt.Sprintf("[mcp_servers.%s]\nurl = %q\nbearer_token_env_var = %q\n", name, endpointValue, tokenEnv)
	case "claude-code":
		payload["command"] = fmt.Sprintf("claude mcp add --transport http %s %s --header %q", shellQuote(name), shellQuote(endpointValue), "Authorization: Bearer "+authValue)
		payload["mcp_json"] = map[string]any{
			"mcpServers": map[string]any{
				name: map[string]any{
					"type": "http",
					"url":  endpointValue,
					"headers": map[string]string{
						"Authorization": "Bearer " + authValue,
					},
				},
			},
		}
	case "chatgpt":
		payload["mode"] = "oauth-dev-or-front-door"
		payload["notes"] = []string{
			"ChatGPT custom MCP connectors require a public HTTPS endpoint and eligible workspace developer mode.",
			"Use codencer setup relay --enable-chatgpt-oauth-dev for a single-user self-host test issuer, or place an operator-owned OAuth front door in front of the relay.",
			"Do not claim ChatGPT live proof until the product connector is exercised.",
		}
		payload["connector_values"] = map[string]any{
			"label":                name,
			"endpoint":             endpointValue,
			"oauth_metadata":       baseURL + "/.well-known/oauth-protected-resource/mcp",
			"authorization_server": baseURL + "/.well-known/oauth-authorization-server",
			"upstream_token":       "<relay-planner-token-held-by-front-door>",
		}
	default:
		return nil, fmt.Errorf("unsupported --client %q", client)
	}
	return payload, nil
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	if strings.IndexFunc(value, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\'' || r == '"' || r == '$' || r == '\\'
	}) < 0 {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
