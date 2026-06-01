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

For the frozen post-beta product path contract, see [Flagship Planner Product Path](../internal/FLAGSHIP_PLANNER_PRODUCT_PATH.md). For the deployable OAuth resource-server/front-door pattern, see [OAuth Front Door](OAUTH_FRONT_DOOR.md).

## Auth Modes

`bearer-token` mode is the repo-proven execution mode:

- relay: `Authorization: Bearer <planner-token>`
- cloud: `Authorization: Bearer <cloud-token>`
- covered by relay/cloud tests, MCP SDK proof, self-host smoke, cloud smoke, and `make flagship-planner-smoke`

`oauth-protected-resource` mode is product-facing discovery/challenge support:

- relay metadata: `GET /.well-known/oauth-protected-resource/mcp`
- cloud metadata: `GET /.well-known/oauth-protected-resource/api/cloud/v1/mcp`
- unauthenticated relay/cloud MCP calls include a `WWW-Authenticate` bearer challenge with `resource_metadata`
- configure `public_base_url`, `oauth_authorization_servers`, `oauth_scopes_supported`, and `oauth_resource_documentation`

This is OAuth-capable resource-server behavior, not Codencer-native OAuth token issuance. If ChatGPT, Claude, or an API connector requires a full OAuth authorization flow, run an operator-owned OAuth issuer or gateway in front of Codencer and have it issue or translate to a bearer token Codencer accepts. The concrete deployment contract is [OAuth Front Door](OAUTH_FRONT_DOOR.md).

## Compatibility Matrix

`operator-packaged` means the Codencer-side loop, examples, and smoke proof are now concrete for operator use. It is not a universal claim about every vendor UI, plan tier, approval prompt, or publication workflow.

| Path | Canonical surface | Status | Direct repo proof | Notes |
| --- | --- | --- | --- | --- |
| Relay HTTP | relay `/api/v2/...` | `proven` | relay integration tests + self-host smoke | Narrow instance-scoped planner API. |
| Relay project HTTP | relay `/api/v2/projects/...` | `proven` | `make verify-local-relay-mcp` | Project-aware local execution via connector and Sprint 2 `localexec`. |
| Relay MCP | relay `/mcp` | `proven` | relay MCP tests + self-host MCP smoke + `make verify-local-relay-mcp` | Session-bound Streamable HTTP plus JSON-RPC POST, with project-aware tools preferred. |
| Cloud HTTP | cloud `/api/cloud/v1/runtime/...` | `proven` | cloud runtime tests + composed cloud smoke | Tenant-scoped only in composed mode. |
| Cloud MCP | cloud `/api/cloud/v1/mcp` | `proven` | cloud MCP tests + composed cloud smoke | Tenant-scoped only in composed mode. |
| Official Go SDK | relay `/mcp` and cloud `/api/cloud/v1/mcp` | `proven` | MCP server tests + `cmd/mcp-sdk-smoke` + smoke | Proven for MCP only, not for the REST HTTP APIs. |
| Generic HTTP clients | relay/cloud HTTP surfaces | `proven` | direct `net/http` tests + `curl` smoke | Plain bearer-token JSON callers are the intended HTTP baseline. |
| Generic MCP clients | relay `/mcp` and cloud `/api/cloud/v1/mcp` | `expected-only` | protocol behavior is repo-proven, but specific client products are not | Do not turn this into a universal desktop/client compatibility claim. |
| ChatGPT custom MCP connector path | relay `/mcp` or cloud `/api/cloud/v1/mcp` | `operator-packaged` | Codencer-side flagship smoke + bearer/OAuth protected-resource metadata tests | Publishable target is ChatGPT Business/Enterprise/Edu for write-capable custom MCP connector use with OAuth front door. Product UI publication is operator-exercised, not repo-clicked. See [integrations/chatgpt.md](integrations/chatgpt.md). |
| OpenAI Responses API remote MCP path | relay `/mcp` or cloud `/api/cloud/v1/mcp` | `operator-packaged expected` | Codencer-side MCP proof + examples | API callers provide `server_url` and authorization as supported by OpenAI's API. This is not the ChatGPT workspace app publish lane. |
| Claude Code remote HTTP MCP path | relay `/mcp` or cloud `/api/cloud/v1/mcp` | `operator-packaged` | Codencer-side flagship smoke + checked-in HTTP MCP config + bearer auth proof | Stronger for Claude Code operator use with bearer headers. Separate from the local `claude` executor-side adapter. See [integrations/claude.md](integrations/claude.md). |
| Anthropic Messages API MCP connector path | relay `/mcp` or cloud `/api/cloud/v1/mcp` | `operator-packaged expected` | Current request-shape examples + Codencer-side MCP proof | Uses `mcp-client-2025-11-20`, `mcp_servers`, and `mcp_toolset`; not executed against Anthropic's API by repo tests. |
| Claude Desktop / `claude.ai` remote connector path | relay `/mcp` or cloud `/api/cloud/v1/mcp` | `compatibility-only until operator-exercised` | OAuth protected-resource metadata is repo-proven; product UI setup is not | Product-side auth/setup remains outside repo proof. Separate from the local `claude` executor-side adapter and from local `claude_desktop_config.json`. |
| Gemini CLI remote MCP path | relay `/mcp` or cloud `/api/cloud/v1/mcp` | `expected-only` | docs only | Use Gemini CLI `httpUrl` plus bearer-token headers against the remote MCP surface, not the local daemon. This repo does not directly exercise Gemini CLI product setup. See [integrations/gemini-cli.md](integrations/gemini-cli.md). |
| Daemon-local MCP | daemon `/mcp/call` | `compatibility-only` | local package tests only | Local compatibility/admin bridge, not the public planner contract. |
| Local daemon as a public remote MCP target | none | `unsupported` | none | Keep remote planners on relay or cloud, not on the daemon directly. |

## Repo-Proven Entry Points

Flagship external-planner loop proof:

```bash
make flagship-planner-smoke
```

That proof uses the canonical relay MCP/HTTP surfaces, local connector, local daemon, and `codex` adapter profile. It covers single-step, same-run phase loop, gate, multi-instance, reconnect, MCP auth metadata/challenge, MCP calls, and official Go SDK MCP with two sequential steps.

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

Project-aware local Relay/MCP proof:

```bash
make verify-local-relay-mcp
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
curl -fsS http://127.0.0.1:8090/.well-known/oauth-protected-resource/mcp

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

Project-aware MCP call:

```bash
curl -fsS \
  -H "Authorization: Bearer <planner-token>" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":2,"name":"codencer.list_projects","arguments":{}}' \
  http://127.0.0.1:8090/mcp/call
```

Sprint 5 live proof commands:

```bash
./bin/codencer live codex-mcp --json --endpoint https://relay.example.com/mcp
./bin/codencer live claude-mcp --json --endpoint https://relay.example.com/mcp
CODENCER_LIVE_RELAY_MCP=1 ./bin/codencer live relay-mcp --json --bin-dir ./bin --repo .
```

The Codex and Claude Code MCP proof commands generate config snippets and distinguish `config_generated`, `endpoint_verified`, and product-client proof. They do not write user-level client config unless explicit config-write environment variables are set. ChatGPT product UI proof remains manual; use [ChatGPT Custom MCP Live Checklist](chatgpt-live-checklist.md).

Minimal cloud initialize plus compatibility-call flow:

```bash
curl -fsS http://127.0.0.1:8190/.well-known/oauth-protected-resource/api/cloud/v1/mcp

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

For Anthropic Messages API request-shape packaging, use [examples/anthropic-messages-relay.mcp.json](examples/anthropic-messages-relay.mcp.json) or [examples/anthropic-messages-cloud.mcp.json](examples/anthropic-messages-cloud.mcp.json). These examples use the current `mcp_servers` plus `mcp_toolset` shape and the `mcp-client-2025-11-20` beta label; the repo does not call Anthropic's API during verification.

For Gemini CLI style remote HTTP MCP configuration, see [integrations/gemini-cli.md](integrations/gemini-cli.md) and [examples/gemini-cli-relay.mcp.json](examples/gemini-cli-relay.mcp.json). This remains an `expected-only` packaging path aligned to the current official Gemini CLI docs, not a repo-executed product proof. The local environment for this documentation pass did not have `gemini` installed, so this repo does not claim local Gemini CLI validation here.

## ChatGPT-Style And Anthropic API Paths

The Codencer-side ChatGPT custom MCP connector, Claude Code remote MCP, and Anthropic Messages API packaging lanes are now concrete enough for operator use: they use the same canonical relay/cloud MCP surfaces as the repo smokes, and `make flagship-planner-smoke` proves the Codencer side of the loop.
Vendor product UI/auth behavior, marketplace publication, Anthropic remote connector setup, and every other product-specific client flow remain outside repo proof unless explicitly exercised.

Current external platform references:

- OpenAI currently documents ChatGPT custom MCP connector/app setup through developer mode, remote MCP only, OAuth-style app authentication, and Business/Enterprise/Edu as the full write-capable workspace target. Follow the current official OpenAI docs when wiring ChatGPT-style or OpenAI API clients to relay `/mcp` or cloud `/api/cloud/v1/mcp`.
- Anthropic currently documents remote custom connectors for Claude, Cowork, and Claude Desktop; Claude Code remote HTTP MCP; and the Messages API MCP connector with the `mcp-client-2025-11-20` beta header. Follow the current official Anthropic docs when wiring Claude-style clients to relay `/mcp` or cloud `/api/cloud/v1/mcp`.

Keep these claims narrow:

- this repo proves the Codencer relay/cloud MCP protocol surfaces directly
- this repo does not prove every vendor client UI, approval flow, auth setup, or publication workflow
- this repo does not turn the local daemon into a public remote MCP target

## Sprint 6 Setup Commands

Use the unified CLI for current MCP snippets:

```bash
./bin/codencer setup mcp --client codex --endpoint https://relay.example.com/mcp --token-env CODENCER_PLANNER_TOKEN --json
./bin/codencer setup mcp --client claude-code --endpoint https://relay.example.com/mcp --token-env CODENCER_PLANNER_TOKEN --json
./bin/codencer setup mcp --client chatgpt --endpoint https://relay.example.com/mcp --json
```

These commands reuse the same generator as `codencer-relayd mcp-config` and do not write user-level client files. Literal tokens, if supplied for an operator-run proof, are redacted from setup reports. ChatGPT remains an OAuth-front-door/product-UI proof path; keep it pending until a real workspace connector has been exercised.
