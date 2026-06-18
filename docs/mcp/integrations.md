# MCP Integrations

Codencer exposes one current open-source remote MCP surface:

```text
self-host Relay: https://<relay-host>/mcp
```

The local daemon is not the public remote MCP endpoint. Future Codencer
Gateway/Cloud is a separate managed layer and must not be confused with the
self-host Relay shipped by this repository.

## Current Surface

Use self-host Relay for remote planners and MCP clients:

- HTTP planner API under `/api/v2/...`
- MCP endpoint at `/mcp`
- OAuth protected-resource metadata at
  `/.well-known/oauth-protected-resource/mcp`
- optional single-user OAuth dev issuer for ChatGPT testing

Remote clients must not target:

- daemon-local URLs;
- `http://127.0.0.1` or WSL-only loopback URLs for product-hosted clients;
- archived cloud-control-plane examples;
- hosted Codencer Gateway/Cloud endpoints unless an official managed service is
  explicitly documented.

## Auth Modes

Bearer token mode is the repo-proven execution mode:

```text
Authorization: Bearer <planner-token>
```

OAuth protected-resource metadata and bearer challenges exist for product-facing
MCP setup. For self-host ChatGPT testing, `codencer setup relay
--enable-chatgpt-oauth-dev` enables a minimal single-user OAuth dev issuer.
Production IAM should use an operator-owned OAuth issuer/front door that
translates to a bearer token Codencer accepts.

## Client Status Matrix

| Client path | Status | Repo proof | Notes |
| --- | --- | --- | --- |
| Relay HTTP | proven | relay tests and smoke | Planner API under `/api/v2/...`. |
| Relay MCP | proven | relay MCP tests, SDK helper, `make verify-local-relay-mcp` | Project-aware `codencer.*` tools. |
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

See [Relay MCP tools](relay_tools.md) for the full list and schemas.

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
Relay, calls a tool, and evidence is saved.

## Client Guides

- [Codex MCP activation](codex-mcp-live.md)
- [Claude Code MCP activation](claude-code-mcp-live.md)
- [ChatGPT custom MCP app setup](chatgpt-app-setup.md)
- [ChatGPT OAuth dev mode](chatgpt-oauth-dev.md)
- [Relay MCP tools](relay_tools.md)
- [ChatGPT integration notes](integrations/chatgpt.md)
- [Claude Code integration notes](integrations/claude.md)
- [Gemini CLI integration notes](integrations/gemini-cli.md)
