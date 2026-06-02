# Flagship Planner Quickstart

Use this when you want an external planner, such as a ChatGPT-style or Claude-style client, to drive local Codex through Codencer.

The flagship loop is:

```text
external planner -> relay/cloud MCP -> connector -> local daemon -> local codex -> result/evidence -> planner decides next task
```

## Fast Repo Proof

From a built checkout:

```bash
make flagship-planner-smoke
```

This starts temporary local daemons and a relay, then proves:

- single-step task flow
- same-run phase loop
- strict gate approval flow
- multi-instance targeting
- connector reconnect/recovery
- relay MCP initialize/tool calls
- official Go SDK MCP helper with a two-step loop

By default this proof uses simulation mode while targeting the `codex` adapter profile. To attempt a live local Codex executor run on your machine:

```bash
FLAGSHIP_LIVE_CODEX=1 CODEX_BINARY=codex make flagship-planner-smoke
```

Live Codex requires your local `codex` CLI to be installed and authenticated.
The live option forces `ALL_ADAPTERS_SIMULATION_MODE=0` and `CODEX_SIMULATION_MODE=0`, checks `/api/v1/compatibility` for a live available `codex` adapter, and raises the relay proxy and wait budgets because real executor calls can take materially longer than the simulation proof.

## Operator Setup

1. Build the supported local binaries:

```bash
make build build-mcp-sdk-smoke
```

2. Start the local daemon near the repository you want Codex to edit:

```bash
CODEX_BINARY=codex ALL_ADAPTERS_SIMULATION_MODE=0 ./bin/orchestratord --repo-root /path/to/repo
```

3. Create a relay config and planner token:

```bash
mkdir -p .codencer/relay
./bin/codencer-relayd planner-token create \
  --config .codencer/relay/config.json \
  --write-config \
  --name operator \
  --scope '*'
```

For a public product-facing MCP endpoint, add the public URL and OAuth protected-resource metadata to `.codencer/relay/config.json`:

```json
{
  "public_base_url": "https://<your-relay-host>",
  "oauth_authorization_servers": ["https://<your-oauth-issuer-or-gateway>"],
  "oauth_scopes_supported": [
    "instances:read",
    "runs:read",
    "runs:write",
    "steps:read",
    "steps:write",
    "artifacts:read",
    "gates:read",
    "gates:write"
  ],
  "oauth_resource_documentation": "https://<your-docs>/codencer-relay-mcp"
}
```

Codencer remains the MCP resource server. Bearer tokens are the proven execution credential; a full product OAuth authorization flow needs your OAuth issuer/front door to issue or translate to a token Codencer accepts. See [OAuth Front Door](mcp/OAUTH_FRONT_DOOR.md).

4. Start the relay:

```bash
./bin/codencer-relayd --config .codencer/relay/config.json
```

5. Create an enrollment token and enroll the connector:

```bash
./bin/codencer-relayd enrollment-token create \
  --config .codencer/relay/config.json \
  --label local-codex \
  --expires-in-seconds 600 \
  --json

./bin/codencer-connectord enroll \
  --relay-url http://127.0.0.1:8090 \
  --daemon-url http://127.0.0.1:8085 \
  --enrollment-token <enrollment-token>
```

6. Run the connector:

```bash
./bin/codencer-connectord run
```

7. Verify the MCP loop before connecting a product UI:

```bash
./bin/mcp-sdk-smoke \
  --endpoint http://127.0.0.1:8090/mcp \
  --token <planner-token> \
  --adapter-profile codex \
  --step-count 2 \
  --json
```

## ChatGPT-Style Path

Canonical endpoint:

- relay: `https://<your-relay-host>/mcp`
- cloud: `https://<your-cloud-host>/api/cloud/v1/mcp`

Use the ChatGPT custom MCP connector/app setup flow for a remote MCP server. The write-capable publishable target is Business, Enterprise, and Edu workspace custom connectors with OAuth front-door auth. For private API-style testing, use bearer-token auth where the client supports it.

Then ask ChatGPT to use only the Codencer tool/app and run this sequence:

1. `codencer.list_instances`
2. `codencer.get_instance`
3. `codencer.start_run`
4. `codencer.submit_task` with `adapter_profile: "codex"`
5. `codencer.wait_step`
6. `codencer.get_step_result`
7. `codencer.get_step_validations`
8. `codencer.get_step_logs`
9. `codencer.list_step_artifacts`

Status: materially stronger than compatibility-only for the operator's Codencer-side loop and publishability packaging. Direct product proof remains limited to the operator's own ChatGPT workspace setup; this repo does not claim universal ChatGPT product support, consumer-plan parity, or Agent Mode write-capable connector use.

## Claude-Style Path

Best current operator lanes:

- Claude Code remote HTTP MCP configuration using an `Authorization` header
- Anthropic Messages API MCP connector using `mcp-client-2025-11-20`, `mcp_servers`, and `mcp_toolset`
- relay example: [docs/mcp/examples/claude-code-relay.mcp.json](mcp/examples/claude-code-relay.mcp.json)
- cloud example: [docs/mcp/examples/claude-code-cloud.mcp.json](mcp/examples/claude-code-cloud.mcp.json)
- API relay example: [docs/mcp/examples/anthropic-messages-relay.mcp.json](mcp/examples/anthropic-messages-relay.mcp.json)
- API cloud example: [docs/mcp/examples/anthropic-messages-cloud.mcp.json](mcp/examples/anthropic-messages-cloud.mcp.json)

Claude Code setup:

```bash
claude mcp add --transport http --header "Authorization: Bearer <planner-token>" \
  codencer-relay https://<your-relay-host>/mcp
```

For Claude Desktop or `claude.ai` remote custom connectors, keep the existing auth caveat: Codencer's private planner auth is bearer-token based, while Anthropic's remote connector UI may require OAuth-oriented setup. Use the OAuth front-door pattern for that product setup.

For Claude Desktop, `claude.ai`, or Anthropic Messages API remote MCP connector setup, use the public relay/cloud MCP URL. Bearer-token mode is proven for direct clients and Claude Code-style configs; OAuth protected-resource metadata is implemented for product-facing flows that expect OAuth bearer auth, but an OAuth issuer/front door remains operator-owned.

Status: materially stronger than compatibility-only for Claude Code remote HTTP MCP operator use, Anthropic Messages API packaging, and Codencer-side OAuth-capable MCP discovery; Claude Desktop/`claude.ai` product UI setup remains compatibility-only until exercised in that product.

## Troubleshooting

- No instances: confirm the connector is running and the instance is shared or cloud-claimed.
- Auth failures: verify the relay planner token or cloud token and scopes.
- OAuth product setup cannot complete: verify `/.well-known/oauth-protected-resource/...` is public and points at your OAuth issuer/front door; Codencer does not issue OAuth authorization-code tokens itself.
- `codex` missing: set `CODEX_BINARY=/full/path/to/codex` and run `./bin/orchestratorctl doctor`.
- Gate returned: inspect `codencer.list_run_gates`, ask the human/operator, then call approve or reject.
- Reconnect happened: reinitialize MCP, call `codencer.list_instances`, then continue from saved `run_id` and `step_id`.
