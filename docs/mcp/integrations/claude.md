# Claude Code MCP Integration Notes

Claude Code MCP setup is configuration/preflight proof until Claude Code
actually connects to the self-host Relay, calls a tool, and evidence is saved.
Do not mark live Claude Code proof passed from repository-only tests.

Use this page with:

- [Claude Code MCP activation](../claude-code-mcp-live.md)
- [Relay MCP tools](../relay_tools.md)
- [MCP integrations](../integrations.md)

## Current Supported Surface

Claude Code should target the self-host Relay MCP endpoint:

```text
https://<relay-host>/mcp
```

Do not point Claude Code at:

- the local daemon;
- `http://127.0.0.1` unless running a purely local private experiment;
- WSL-only loopback URLs for remote product flows;
- old cloud-control-plane MCP examples;
- hosted Codencer Gateway/Cloud endpoints unless an official managed service is
  explicitly documented.

## Generate Setup

```bash
codencer setup mcp --client claude-code --endpoint https://relay.example.com/mcp --json
codencer activation package --relay https://relay.example.com --project codencer --token-env CODENCER_MCP_TOKEN --json
```

Canonical command shape:

```bash
claude mcp add \
  --transport http \
  --header "Authorization: Bearer $CODENCER_MCP_TOKEN" \
  codencer \
  https://relay.example.com/mcp
```

## Narrow Product Smoke

Only mark Claude Code proof passed when all of this evidence exists:

1. Claude Code is configured with the Relay MCP endpoint.
2. Claude Code connects successfully.
3. Claude Code calls `codencer.list_projects`.
4. If execution proof is claimed, Claude Code calls an execution tool against a
   shared fake/local project and Codencer returns a structured result or blocker.
5. Evidence is saved with timestamps and the exact command/config used.

Until then, Claude Code proof is pending/manual, not passed.

## Notes

Claude Code as an MCP client is separate from the local `claude` executor
adapter. The MCP client talks to Relay. The executor adapter, when configured,
runs locally through the daemon.

Project listings include `locations[]`. If multiple online machines advertise
the same `project_id`, pass `machine_id` or `host_label`; otherwise Codencer
returns `ambiguous_project_location`.
