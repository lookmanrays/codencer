# MCP Integrations

Codencer exposes one official remote MCP surface:

```text
Codencer Gateway: https://mcp.codencer.dev/mcp
```

Gateway routes to one or more backend Relays. The local daemon is not the public
remote MCP endpoint. Direct self-host Relay `/mcp` remains supported for
advanced/direct/debug mode.

## Current Surface

Use Gateway for official remote planners and MCP clients:

- MCP endpoint at `/mcp`
- OAuth protected-resource metadata at
  `/.well-known/oauth-protected-resource/mcp`
- optional OAuth dev issuer for ChatGPT Developer Mode
- Relay profiles that route to self-host or managed Relays

Gateway forwards to self-host Relay:

- HTTP planner API under `/api/v2/...`
- MCP endpoint at `/mcp`
- OAuth protected-resource metadata at
  `/.well-known/oauth-protected-resource/mcp`
- optional single-user OAuth dev issuer for ChatGPT testing

Remote clients must not target:

- daemon-local URLs;
- `http://127.0.0.1` or WSL-only loopback URLs for product-hosted clients;
- archived cloud-control-plane examples;
- backend Relay `/mcp` unless they are intentionally using direct/debug mode.

## Auth Modes

Gateway bearer-dev mode is the repo-proven official pre-prod auth mode:

```text
Authorization: Bearer <gateway-token>
```

OAuth protected-resource metadata and bearer challenges exist for product-facing
MCP setup. For ChatGPT testing, `codencer setup gateway --enable-oauth-dev`
enables a minimal single-user OAuth dev issuer.
Production IAM should use an operator-owned OAuth issuer/front door that
translates to a bearer token Codencer accepts.

## Client Status Matrix

| Client path | Status | Repo proof | Notes |
| --- | --- | --- | --- |
| Gateway MCP | proven | Gateway tests and `make verify-gateway` | Official connector endpoint and project-aware tools. |
| Relay HTTP | proven | relay tests and smoke | Planner API under `/api/v2/...`. |
| Relay MCP | proven direct/debug | relay MCP tests, SDK helper, `make verify-local-relay-mcp` | Advanced direct Relay endpoint. |
| Generic HTTP/MCP callers | proven for protocol | curl/tests/SDK helper | Specific product UIs are not universally proven. |
| Codex MCP setup | setup/preflight | activation snippets and preflight | Live Codex client proof pending unless Codex connects and calls a tool. |
| Claude Code MCP setup | setup/preflight | activation snippets and preflight | Live Claude Code proof pending unless Claude Code connects and calls a tool. |
| ChatGPT custom MCP app | setup/preflight | activation package, OAuth dev metadata, setup sheet | ChatGPT product UI proof pending unless an eligible workspace connects and calls a tool. |
| Gemini CLI | expected/operator-packaged | docs only | No repo-executed Gemini CLI product proof. |

## Project-Aware Tools

Prefer project-aware tools such as:

- `codencer.list_projects`
- `codencer.submit_project_task`
- `codencer.submit_project_task_and_wait`
- `codencer.run_project_manifest`
- `codencer.get_project_blocker`
- `codencer.get_execution_report`

See [MCP Gateway model](../architecture/mcp-gateway-model.md) for Gateway
routing and [Relay MCP tools](relay_tools.md) for direct Relay mode.

Project listings include `locations[]` with `machine_id`, `host_label`,
connector/instance ids, status, and safe repo labels/hashes. Absolute local
paths are not exposed.

Execution tools accept optional `machine_id` or `host_label`. If exactly one
online location exists, no selector is required. If multiple online locations
exist for the same `project_id`, Codencer returns structured blocker
`ambiguous_project_location` instead of choosing randomly.

## Activation Commands

```bash
make activation-preflight
./bin/codencer activation gateway --gateway https://mcp.codencer.dev --relay https://relay.example.com --project codencer --token-env CODENCER_GATEWAY_MCP_TOKEN --json
./bin/codencer activation package --relay https://relay.example.com --project codencer --token-env CODENCER_MCP_TOKEN --json
./bin/codencer activation chatgpt --relay https://relay.example.com --project codencer --auth oauth --json
./bin/codencer activation codex --relay https://relay.example.com --token-env CODENCER_MCP_TOKEN --json
./bin/codencer activation claude-code --relay https://relay.example.com --token-env CODENCER_MCP_TOKEN --json
```

These commands generate setup artifacts and preflight materials. They do not
write user-level client config and do not prove product UI behavior by
themselves.

## Product Proof Rule

Do not mark live ChatGPT, Codex, Claude Code, Gemini CLI, Claude Desktop, or
other product-client proof as passed until that product actually connects to the
Gateway, calls a tool, and evidence is saved.

## Client Guides

- [Codex MCP activation](codex-mcp-live.md)
- [Claude Code MCP activation](claude-code-mcp-live.md)
- [ChatGPT custom MCP app setup](chatgpt-app-setup.md)
- [ChatGPT OAuth dev mode](chatgpt-oauth-dev.md)
- [Official Gateway activation](../activation-official-gateway.md)
- [Relay MCP tools](relay_tools.md)
- [ChatGPT integration notes](integrations/chatgpt.md)
- [Claude Code integration notes](integrations/claude.md)
- [Gemini CLI integration notes](integrations/gemini-cli.md)
