# Planner / Client Integration Notes

Codencer has two remote planner surfaces:

- direct self-host relay: HTTP under `/api/v2/...` and MCP at `/mcp`
- cloud tenancy in composed mode: HTTP under `/api/cloud/v1/runtime/...` and MCP at `/api/cloud/v1/mcp`

Do not point ChatGPT, Claude, or another planner runtime at the local Codencer daemon directly.
The daemon-local `/mcp/call` endpoint is only a local compatibility/admin bridge.

Provider connectors such as GitHub, GitLab, Jira, Linear, and Slack are separate cloud integrations.
They are not planner/client entrypoints.

Local execution adapters such as `codex`, local `claude`, `qwen`, and `antigravity*` are also separate.
They are executor-side adapters, not remote planner surfaces.

## Surface Selection

- Use relay `/api/v2/...` and relay `/mcp` when you are operating the self-host relay directly.
- Use cloud `/api/cloud/v1/runtime/...` and cloud `/api/cloud/v1/mcp` when cloud tenancy is the public control plane.
- Treat relay `/mcp/call`, cloud `/api/cloud/v1/mcp/call`, and daemon `/mcp/call` as compatibility POST aliases rather than the primary session contract.

## Compatibility Matrix

| Path | Canonical surface | Status | Direct repo proof | Notes |
| --- | --- | --- | --- | --- |
| Relay HTTP | relay `/api/v2/...` | `proven` | relay integration tests + self-host smoke | Narrow instance-scoped planner API. |
| Relay MCP | relay `/mcp` | `proven` | relay MCP tests + self-host MCP smoke | Session-bound Streamable HTTP plus JSON-RPC POST. |
| Cloud HTTP | cloud `/api/cloud/v1/runtime/...` | `proven` | cloud runtime tests + composed cloud smoke | Tenant-scoped only in composed mode. |
| Cloud MCP | cloud `/api/cloud/v1/mcp` | `proven` | cloud MCP tests + composed cloud smoke | Tenant-scoped only in composed mode. |
| Official Go SDK | relay `/mcp` and cloud `/api/cloud/v1/mcp` | `proven` | MCP server tests + `cmd/mcp-sdk-smoke` + smoke | Proven for MCP only, not for the REST HTTP APIs. |
| Generic HTTP clients | relay/cloud HTTP surfaces | `proven` | direct `net/http` tests + `curl` smoke | Plain bearer-token JSON callers are the intended HTTP baseline. |
| Generic MCP clients | relay `/mcp` and cloud `/api/cloud/v1/mcp` | `expected-only` | protocol behavior is repo-proven, but specific client products are not | Do not turn this into a universal desktop/client compatibility claim. |
| ChatGPT-style remote MCP path | relay `/mcp` or cloud `/api/cloud/v1/mcp` | `compatibility-only` | docs only | Remote MCP only. Use the relay/cloud surface, not the local daemon, and see [integrations/chatgpt.md](integrations/chatgpt.md) for the operator walkthrough. |
| Claude-style remote MCP path | relay `/mcp` or cloud `/api/cloud/v1/mcp` | `compatibility-only` | docs only | Planner-side remote connector flow only. Separate from the local `claude` executor-side adapter and from local `claude_desktop_config.json`. See [integrations/claude.md](integrations/claude.md). |
| Gemini CLI remote MCP path | relay `/mcp` or cloud `/api/cloud/v1/mcp` | `expected-only` | docs only | Use Gemini CLI `httpUrl` plus bearer-token headers against the remote MCP surface, not the local daemon. This repo does not directly exercise Gemini CLI product setup. See [integrations/gemini-cli.md](integrations/gemini-cli.md). |
| Daemon-local MCP | daemon `/mcp/call` | `compatibility-only` | local package tests only | Local compatibility/admin bridge, not the public planner contract. |
| Local daemon as a public remote MCP target | none | `unsupported` | none | Keep remote planners on relay or cloud, not on the daemon directly. |

## Repo-Proven Entry Points

Relay-side proof:

```bash
PLANNER_TOKEN=<planner-token> make self-host-smoke
PLANNER_TOKEN=<planner-token> make self-host-smoke-mcp
PLANNER_TOKEN=<planner-token> SMOKE_SCENARIOS=status,audit,share-control,mcp,mcp-sdk make self-host-smoke
```

Cloud-side proof:

```bash
make cloud-smoke
make build-cloud build-mcp-sdk-smoke
CLOUD_RELAY_CONFIG=.codencer/relay/config.json \
CLOUD_RUNTIME_DAEMON_URL=http://127.0.0.1:8080 \
CLOUD_SMOKE_MCP=1 \
CLOUD_SMOKE_SDK=1 \
make cloud-smoke
```

Standalone official Go SDK proof:

```bash
make build-mcp-sdk-smoke
./bin/mcp-sdk-smoke --endpoint http://127.0.0.1:8090/mcp --token <planner-token> --instance-id <instance-id>
./bin/mcp-sdk-smoke --endpoint http://127.0.0.1:8190/api/cloud/v1/mcp --token <cloud-token> --instance-id <instance-id>
```

## Generic HTTP Examples

Relay HTTP:

```bash
curl -fsS \
  -H "Authorization: Bearer <planner-token>" \
  -H "Content-Type: application/json" \
  -d '{"id":"relay-http-demo","project_id":"demo-project"}' \
  http://127.0.0.1:8090/api/v2/instances/<instance-id>/runs

curl -fsS \
  -H "Authorization: Bearer <planner-token>" \
  -H "Content-Type: application/json" \
  -d '{"version":"v1","goal":"Verify relay HTTP planner path","adapter_profile":"codex"}' \
  http://127.0.0.1:8090/api/v2/instances/<instance-id>/runs/relay-http-demo/steps
```

Cloud HTTP:

```bash
curl -fsS \
  -H "Authorization: Bearer <cloud-token>" \
  -H "Content-Type: application/json" \
  -d '{"id":"cloud-http-demo","project_id":"demo-project"}' \
  http://127.0.0.1:8190/api/cloud/v1/runtime/instances/<instance-id>/runs

curl -fsS \
  -H "Authorization: Bearer <cloud-token>" \
  -H "Content-Type: application/json" \
  -d '{"version":"v1","goal":"Verify cloud HTTP planner path","adapter_profile":"codex"}' \
  http://127.0.0.1:8190/api/cloud/v1/runtime/instances/<instance-id>/runs/cloud-http-demo/steps
```

These bearer-token HTTP examples are repo-proven through direct tests and smoke.

## Generic MCP Examples

For the full tool lists and input rules, see [Relay MCP Tools](relay_tools.md) and [Cloud MCP Tools](cloud_tools.md).

Minimal relay initialize plus compatibility-call flow:

```bash
curl -fsS -D /tmp/codencer-mcp-headers.txt \
  -H "Authorization: Bearer <planner-token>" \
  -H "Content-Type: application/json" \
  -H "MCP-Protocol-Version: 2025-11-25" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}' \
  http://127.0.0.1:8090/mcp

SESSION_ID="$(awk -F': ' 'tolower($1)==\"mcp-session-id\" {gsub(\"\\r\", \"\", $2); print $2}' /tmp/codencer-mcp-headers.txt)"

curl -fsS \
  -H "Authorization: Bearer <planner-token>" \
  -H "Content-Type: application/json" \
  -H "MCP-Session-Id: ${SESSION_ID}" \
  -H "MCP-Protocol-Version: 2025-11-25" \
  -d '{"jsonrpc":"2.0","id":2,"name":"codencer.list_instances","arguments":{}}' \
  http://127.0.0.1:8090/mcp/call
```

Minimal cloud initialize plus compatibility-call flow:

```bash
curl -fsS -D /tmp/codencer-cloud-mcp-headers.txt \
  -H "Authorization: Bearer <cloud-token>" \
  -H "Content-Type: application/json" \
  -H "MCP-Protocol-Version: 2025-11-25" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}' \
  http://127.0.0.1:8190/api/cloud/v1/mcp

SESSION_ID="$(awk -F': ' 'tolower($1)==\"mcp-session-id\" {gsub(\"\\r\", \"\", $2); print $2}' /tmp/codencer-cloud-mcp-headers.txt)"

curl -fsS \
  -H "Authorization: Bearer <cloud-token>" \
  -H "Content-Type: application/json" \
  -H "MCP-Session-Id: ${SESSION_ID}" \
  -H "MCP-Protocol-Version: 2025-11-25" \
  -d '{"jsonrpc":"2.0","id":2,"name":"codencer.list_instances","arguments":{}}' \
  http://127.0.0.1:8190/api/cloud/v1/mcp/call
```

Notes:

- `/mcp` and `/api/cloud/v1/mcp` are the canonical session paths.
- `POST /mcp/call` and `POST /api/cloud/v1/mcp/call` are compatibility aliases for simple POST callers.
- The alias accepts both full JSON-RPC `tools/call` bodies and the shorthand top-level `name` / `arguments` form shown above.
- `GET` streaming requires `Accept: text/event-stream`, `MCP-Session-Id`, and the negotiated `MCP-Protocol-Version`.
- `DELETE` session close also requires `MCP-Session-Id`.
- `notifications/initialized` is accepted after `initialize`, but the current repo proof helpers do not depend on it.

## Checked-In MCP Config Examples

For project-scoped Claude Code style HTTP MCP configuration, use the checked-in examples:

- [examples/claude-code-relay.mcp.json](examples/claude-code-relay.mcp.json)
- [examples/claude-code-cloud.mcp.json](examples/claude-code-cloud.mcp.json)

Those examples use the relay and cloud canonical MCP URLs, plus environment-variable-driven bearer headers.
They are packaging examples, not repo-executed Claude product proof.

For the narrow operator flow that uses the repo's actual `codencer.*` MCP tool names in ChatGPT, see [integrations/chatgpt.md](integrations/chatgpt.md). The checked-in [examples/chatgpt-relay.mcp.json](examples/chatgpt-relay.mcp.json) and [examples/chatgpt-cloud.mcp.json](examples/chatgpt-cloud.mcp.json) are value-reference templates for ChatGPT app setup, not direct ChatGPT imports.

For the current Claude Desktop and `claude.ai` operator walkthrough, see [integrations/claude.md](integrations/claude.md). It keeps the planner-side remote connector flow separate from the executor-side adapter story, points operators to Anthropic's current `Customize > Connectors` or organization `Settings > Connectors` flow, and calls out that `claude_desktop_config.json` is the separate local-MCP mechanism rather than the remote connector path.

For Gemini CLI style remote HTTP MCP configuration, see [integrations/gemini-cli.md](integrations/gemini-cli.md) and [examples/gemini-cli-relay.mcp.json](examples/gemini-cli-relay.mcp.json). This remains an `expected-only` packaging path aligned to the current official Gemini CLI docs, not a repo-executed product proof. The local environment for this documentation pass did not have `gemini` installed, so this repo does not claim local Gemini CLI validation here.

## ChatGPT-Style And Anthropic API Paths

These remain `compatibility-only` in Codencer's beta contract.
They are documented patterns, not directly exercised repo integrations.

Current external platform references:

- OpenAI ChatGPT developer mode currently documents remote MCP support and app setup flows through developer mode. Follow the current official OpenAI docs when wiring ChatGPT-style or OpenAI API clients to relay `/mcp` or cloud `/api/cloud/v1/mcp`.
- Anthropic currently documents remote custom connectors for Claude Desktop and `claude.ai`, plus separate local/project MCP configuration for Claude Code. Follow the current official Anthropic docs when wiring Claude-style clients to relay `/mcp` or cloud `/api/cloud/v1/mcp`.

Keep these claims narrow:

- this repo proves the Codencer relay/cloud MCP protocol surfaces directly
- this repo does not prove every vendor client UI, approval flow, or publication workflow
- this repo does not turn the local daemon into a public remote MCP target
