package activation

import (
	"fmt"
	"strings"

	"agent-bridge/internal/mcpconfig"
	"agent-bridge/internal/security"
)

func readmeContent(opts Options, relayURL, mcpURL string) string {
	return strings.TrimSpace(fmt.Sprintf(`# Codencer Activation Package

Codencer is a bridge, not a planner. Use this package to connect an approved planner client to a self-hosted Codencer Relay/MCP endpoint.

Relay endpoint: %s
MCP endpoint: %s
Project: %s

Proof states are separate:

- server_ready: relay and MCP endpoints answer checks.
- client_config_generated: a client setup artifact was generated.
- client_connected: the real client connected to the MCP endpoint.
- client_used_tool: the real client listed or called a Codencer tool.
- full_e2e_execution: a real end-to-end task or manifest ran and produced evidence.

Recommended preflight:

1. Run ./curl-smoke.sh with your token env set.
2. Use codencer.list_projects first.
3. Use codencer.run_project_manifest for approved multi-step work.
4. Use codencer.submit_project_task_and_wait for one approved task.
5. Stop and return blocker details when planner_decision_required is true.

Fake manifest preflights prove server routing only. They are not live Codex, Claude, or ChatGPT product proof.
`, relayURL, mcpURL, firstNonEmpty(opts.ProjectID, "<project-id>"))) + "\n"
}

func curlSmokeContent(opts Options, relayURL, mcpURL string) string {
	env := tokenEnv(opts)
	project := firstNonEmpty(opts.ProjectID, "codencer")
	return strings.TrimSpace(fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail

TOKEN="${%s:?set %s to a relay planner or OAuth access token}"
RELAY_URL=%q
MCP_URL=%q
PROJECT_ID=%q

curl -fsS -H "Authorization: Bearer ${TOKEN}" "${RELAY_URL}/api/v2/status" | jq .
curl -fsS "${RELAY_URL}/.well-known/oauth-protected-resource/mcp" | jq .
curl -fsS \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -H "MCP-Protocol-Version: 2025-11-25" \
  --data '{"jsonrpc":"2.0","id":"tools","method":"tools/list","params":{}}' \
  "${MCP_URL}" | jq .
echo "Project prompt: use codencer.list_projects first, then inspect ${PROJECT_ID}."
`, env, env, relayURL, mcpURL, project)) + "\n"
}

func codexConfigContent(opts Options, mcpURL string) string {
	payload, err := mcpconfig.Generate(mcpconfig.Options{
		Client:   "codex",
		Endpoint: mcpURL,
		TokenEnv: tokenEnv(opts),
		Name:     "codencer",
	})
	if err != nil {
		return "# " + err.Error() + "\n"
	}
	config, _ := payload["config_toml"].(string)
	return strings.TrimSpace(config) + "\n"
}

func claudeCommandContent(opts Options, mcpURL string) string {
	payload, err := mcpconfig.Generate(mcpconfig.Options{
		Client:   "claude-code",
		Endpoint: mcpURL,
		TokenEnv: tokenEnv(opts),
		Name:     "codencer",
	})
	if err != nil {
		return "#!/usr/bin/env bash\nset -euo pipefail\n# " + err.Error() + "\n"
	}
	command, _ := payload["command"].(string)
	return strings.TrimSpace(fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail

%s
`, security.Redact(command))) + "\n"
}

func chatGPTContent(opts Options, relayURL, mcpURL string) string {
	payload := chatGPTPayload(opts, relayURL, mcpURL)
	steps, _ := payload["steps"].([]string)
	lines := []string{
		"# ChatGPT Custom MCP App Setup",
		"",
		"Codencer is a bridge, not a planner. ChatGPT product proof remains pending until an operator completes these steps in an eligible workspace.",
		"",
		"Values:",
		"",
		"- MCP endpoint: " + mcpURL,
		"- OAuth metadata: " + relayURL + "/.well-known/oauth-authorization-server",
		"- Protected resource metadata: " + relayURL + "/.well-known/oauth-protected-resource/mcp",
		"- Auth mode: " + firstNonEmpty(opts.AuthMode, "oauth"),
		"",
		"Steps:",
		"",
	}
	for i, step := range steps {
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, step))
	}
	lines = append(lines,
		"",
		"Evidence to save:",
		"",
		"- App setup screenshot or exported settings.",
		"- MCP initialize and tools list transcript.",
		"- codencer.list_projects result.",
		"- One approved tool result or blocker report.",
	)
	return strings.Join(lines, "\n") + "\n"
}

func chatGPTPayload(opts Options, relayURL, mcpURL string) map[string]any {
	auth := firstNonEmpty(opts.AuthMode, "oauth")
	project := firstNonEmpty(opts.ProjectID, "<project-id>")
	return map[string]any{
		"client":                              "chatgpt",
		"server_ready":                        true,
		"client_config_generated":             true,
		"client_connected":                    "pending_manual_product_proof",
		"client_used_tool":                    "pending_manual_product_proof",
		"full_e2e_execution":                  "pending_manual_product_proof",
		"mcp_endpoint":                        mcpURL,
		"auth_mode":                           auth,
		"oauth_authorization_server_metadata": relayURL + "/.well-known/oauth-authorization-server",
		"oauth_openid_configuration":          relayURL + "/.well-known/openid-configuration",
		"protected_resource_metadata":         relayURL + "/.well-known/oauth-protected-resource/mcp",
		"expected_tools": []string{
			"codencer.list_projects",
			"codencer.get_project",
			"codencer.submit_project_task_and_wait",
			"codencer.run_project_manifest",
			"codencer.get_blocker",
		},
		"steps": []string{
			"Open ChatGPT workspace settings and confirm Developer Mode/custom MCP app access is available.",
			"Create a draft custom MCP app named Codencer pointing to the MCP endpoint.",
			"Use OAuth mode for the dev front-door, or explicit dev-noauth only on a private test relay.",
			"Complete OAuth authorization with the operator approval code when prompted.",
			"Scan tools and confirm codencer.list_projects and codencer.get_blocker are present.",
			"Ask ChatGPT to call codencer.list_projects first.",
			"For one approved task, ask it to call codencer.submit_project_task_and_wait for project " + project + ".",
			"For approved multi-step work, ask it to call codencer.run_project_manifest.",
			"Save returned blocker details instead of inferring next actions from logs.",
			"Keep the app in draft until product proof evidence is saved.",
		},
		"test_prompts": []string{
			"List shared Codencer projects using codencer.list_projects.",
			"For project " + project + ", submit one approved fake-success task and wait for evidence.",
			"If planner_decision_required is true, stop and return the blocker JSON.",
		},
		"evidence_required": []string{
			"server_ready",
			"client_config_generated",
			"client_connected",
			"client_used_tool",
			"full_e2e_execution",
		},
	}
}

func clientPayload(clientName string, opts Options, payload map[string]any) map[string]any {
	project := firstNonEmpty(opts.ProjectID, "<project-id>")
	out := map[string]any{
		"client":                  clientName,
		"client_config_generated": true,
		"client_connected":        "pending_manual_client_proof",
		"client_used_tool":        "pending_manual_client_proof",
		"full_e2e_execution":      "pending_manual_client_proof",
		"setup":                   payload,
		"endpoint_check":          "curl -fsS " + firstNonEmpty(stringValue(payload["metadata_url"]), strings.TrimSuffix(stringValue(payload["endpoint"]), "/mcp")+"/.well-known/oauth-protected-resource/mcp"),
		"prompts": []string{
			"Use codencer.list_projects first.",
			"Inspect project " + project + " with codencer.get_project.",
			"Run one approved fake-success manifest with codencer.run_project_manifest.",
			"Stop and return blocker details when planner_decision_required is true.",
		},
		"evidence_required": []string{
			"client config command/snippet used",
			"MCP tools list includes project tools",
			"codencer.list_projects result",
			"approved tool result or blocker report",
		},
	}
	if clientName == "codex" {
		out["codex_mcp_command"] = payload["command"]
		out["project_config_toml"] = payload["config_toml"]
		out["user_config_toml"] = payload["config_toml"]
	}
	if clientName == "claude-code" {
		out["claude_mcp_command"] = payload["command"]
		out["mcp_json"] = payload["mcp_json"]
	}
	return security.RedactJSON(out).(map[string]any)
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}
