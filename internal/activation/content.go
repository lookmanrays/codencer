package activation

import (
	"fmt"
	"strings"

	"agent-bridge/internal/local"
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

1. On the VPS, create a connector enrollment token with codencer-relayd enrollment-token create.
2. On the local machine, run ./connector-enrollment.sh or the equivalent codencer connector enroll command.
3. Share the intended project with codencer project share.
4. Run ./curl-smoke.sh with your token env set.
5. Use codencer.list_projects first.
6. Use codencer.run_project_manifest for approved multi-step work.
7. Use codencer.submit_project_task_and_wait for one approved task.
8. Stop and return blocker details when planner_decision_required is true.

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
PROTOCOL_VERSION="2025-11-25"
SESSION_ID=""

tmp_headers="$(mktemp)"
tmp_body="$(mktemp)"
trap 'rm -f "${tmp_headers}" "${tmp_body}"' EXIT

print_json() {
  if command -v jq >/dev/null 2>&1; then
    jq .
  else
    cat
  fi
}

mcp_post() {
  local payload="$1"
  local session_args=()
  if [ -n "${SESSION_ID}" ]; then
    session_args=(-H "MCP-Session-Id: ${SESSION_ID}")
  fi
  curl -fsS \
    -D "${tmp_headers}" \
    -o "${tmp_body}" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Accept: application/json, text/event-stream" \
    -H "Content-Type: application/json" \
    -H "MCP-Protocol-Version: ${PROTOCOL_VERSION}" \
    "${session_args[@]}" \
    --data "${payload}" \
    "${MCP_URL}"
  local returned_session
  returned_session="$(awk -F': ' 'tolower($1)=="mcp-session-id" {gsub("\r","",$2); print $2}' "${tmp_headers}" | tail -n 1)"
  if [ -n "${returned_session}" ]; then
    SESSION_ID="${returned_session}"
  fi
  cat "${tmp_body}" | print_json
}

curl -fsS -H "Authorization: Bearer ${TOKEN}" -H "Accept: application/json" "${RELAY_URL}/api/v2/status" | print_json
curl -fsS -H "Accept: application/json" "${RELAY_URL}/.well-known/oauth-protected-resource/mcp" | print_json

mcp_post '{"jsonrpc":"2.0","id":"init","method":"initialize","params":{"protocolVersion":"2025-11-25","clientInfo":{"name":"codencer-activation-smoke","version":"v0"}}}'
mcp_post '{"jsonrpc":"2.0","id":"tools","method":"tools/list","params":{}}'
mcp_post '{"jsonrpc":"2.0","id":"list-projects","method":"tools/call","params":{"name":"codencer.list_projects","arguments":{}}}'

if [ "${RUN_FAKE_MANIFEST:-0}" = "1" ]; then
  mcp_post "$(cat <<JSON
{"jsonrpc":"2.0","id":"fake-manifest","method":"tools/call","params":{"name":"codencer.run_project_manifest","arguments":{"project_id":"${PROJECT_ID}","manifest":{"version":"v0.3","kind":"codencer.run_plan","metadata":{"name":"activation-fake-success"},"project":{"id":"${PROJECT_ID}"},"execution":{"adapter":"fake-success","profile":"fake-success"},"policy":{"stop_on_blocker":true,"stop_on_failure":true},"tasks":[{"id":"fake-success","title":"activation fake success","goal":"Return fake-success evidence for activation preflight."}]}}}}
JSON
)"
fi

echo "Project prompt: use codencer.list_projects first, then inspect ${PROJECT_ID}."
`, env, env, relayURL, mcpURL, project)) + "\n"
}

func connectorEnrollmentContent(opts Options, relayURL string) string {
	project := firstNonEmpty(opts.ProjectID, "<project-id>")
	return strings.TrimSpace(fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail

RELAY_URL=%q
DAEMON_URL="${DAEMON_URL:-http://127.0.0.1:8085}"
CONFIG_PATH="${CONNECTOR_CONFIG_PATH:-${CODENCER_HOME:-$HOME/.codencer}/runtime/connector/config.json}"
LABEL="${CONNECTOR_LABEL:-$(hostname 2>/dev/null || echo codencer-connector)}"
ENROLLMENT_TOKEN="${CODENCER_CONNECTOR_ENROLLMENT_TOKEN:?set CODENCER_CONNECTOR_ENROLLMENT_TOKEN from codencer-relayd enrollment-token create}"

# VPS side, run this first and copy only the returned enrollment token to the local machine:
# codencer-relayd enrollment-token create --relay-url "${RELAY_URL}" --token "${CODENCER_MCP_TOKEN:?set relay planner token}" --label "${LABEL}" --json

# Preferred local facade:
codencer connector enroll \
  --relay-url "${RELAY_URL}" \
  --daemon-url "${DAEMON_URL}" \
  --enrollment-token "${ENROLLMENT_TOKEN}" \
  --config "${CONFIG_PATH}" \
  --label "${LABEL}" \
  --json

# Low-level fallback if the facade is unavailable:
# codencer-connectord enroll --relay-url "${RELAY_URL}" --daemon-url "${DAEMON_URL}" --enrollment-token "${ENROLLMENT_TOKEN}" --config "${CONFIG_PATH}" --label "${LABEL}" --json

codencer connector status --config "${CONFIG_PATH}" --json
codencer project share %s --daemon-url "${DAEMON_URL}" --json
codencer connector run --config "${CONFIG_PATH}"
`, relayURL, project)) + "\n"
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
	steps, _ := payload["chatgpt_ui_steps"].([]string)
	evidence, _ := payload["evidence_checklist"].([]string)
	lines := []string{
		"# ChatGPT Custom MCP App Setup",
		"",
		"Codencer is a bridge, not a planner. ChatGPT product proof remains pending until an operator completes these steps in an eligible workspace.",
		"",
		"Values:",
		"",
		"- MCP endpoint: " + mcpURL,
		"- Auth mode: " + stringValue(payload["auth_mode"]),
		"- Client id: " + stringValue(payload["client_id"]),
		"- OAuth metadata: " + stringValue(payload["authorization_server_metadata"]),
		"- OpenID configuration: " + stringValue(payload["openid_configuration"]),
		"- Protected resource metadata: " + stringValue(payload["protected_resource_metadata"]),
		"",
		"OAuth dev redirect behavior: valid redirect URIs are accepted for self-host dev testing. Production must use redirect allowlisting or an external IdP. The OAuth dev front-door is not public multi-user production.",
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
	)
	for _, item := range evidence {
		lines = append(lines, "- "+item)
	}
	return strings.Join(lines, "\n") + "\n"
}

func chatGPTPayload(opts Options, relayURL, mcpURL string) map[string]any {
	auth := firstNonEmpty(opts.AuthMode, "oauth")
	project := firstNonEmpty(opts.ProjectID, "<project-id>")
	paths, _ := local.ResolvePathsForHome("", "", opts.CodencerHome)
	clientSecretFile := ""
	operatorCodeFile := ""
	if paths.TokensDir != "" {
		clientSecretFile = paths.TokensDir + "/chatgpt-oauth-client-secret"
		operatorCodeFile = paths.TokensDir + "/chatgpt-oauth-operator-code"
	}
	return map[string]any{
		"mcp_endpoint":                  mcpURL,
		"auth_mode":                     auth,
		"client_id":                     "codencer-chatgpt-dev",
		"client_secret_file":            clientSecretFile,
		"operator_code_file":            operatorCodeFile,
		"authorization_server_metadata": relayURL + "/.well-known/oauth-authorization-server",
		"openid_configuration":          relayURL + "/.well-known/openid-configuration",
		"authorization_endpoint":        relayURL + "/oauth/authorize",
		"token_endpoint":                relayURL + "/oauth/token",
		"protected_resource_metadata":   relayURL + "/.well-known/oauth-protected-resource/mcp",
		"scopes": []string{
			"projects:read",
			"projects:write",
			"runs:read",
			"runs:write",
			"steps:read",
			"artifacts:read",
			"reports:read",
		},
		"expected_tools": []string{
			"codencer.list_projects",
			"codencer.get_project",
			"codencer.start_project_run",
			"codencer.list_project_runs",
			"codencer.get_project_run",
			"codencer.submit_project_task",
			"codencer.submit_project_task_and_wait",
			"codencer.run_project_manifest",
			"codencer.get_execution_report",
			"codencer.get_run_report",
			"codencer.get_project_blocker",
			"codencer.get_blocker",
		},
		"chatgpt_ui_steps": []string{
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
		"evidence_checklist": []string{
			"App setup screenshot or exported settings.",
			"MCP initialize and tools list transcript.",
			"codencer.list_projects result.",
			"One approved tool result or blocker report.",
			"server_ready",
			"client_config_generated",
			"client_connected",
			"client_used_tool",
			"full_e2e_execution",
		},
		"proof_states": map[string]any{
			"server_ready":            true,
			"client_config_generated": true,
			"client_connected":        "pending_manual_product_proof",
			"client_used_tool":        "pending_manual_product_proof",
			"full_e2e_execution":      "pending_manual_product_proof",
		},
		"oauth_dev_redirect_behavior": "valid redirect URIs are accepted for dev; production must use redirect allowlisting or an external IdP; OAuth dev is not public multi-user production",
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
