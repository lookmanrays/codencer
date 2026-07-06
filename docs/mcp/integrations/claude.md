# Claude Code MCP Integration Notes

Claude Code MCP setup is configuration/preflight proof until Claude Code
actually connects to the operator's self-host Gateway, calls a tool, and
evidence is saved. Do not mark live Claude Code proof passed from
repository-only tests.

Use this page with:

- [Claude Code MCP activation](../claude-code-mcp-live.md)
- [Self-host MCP proof](../self-host-mcp-proof.md)
- [MCP integrations](../integrations.md)

## Current Supported Surface

Claude Code should target the self-host Gateway MCP endpoint. For a local
operator proof:

```text
http://127.0.0.1:19090/mcp
```

Do not point Claude Code at:

- the local daemon;
- `http://127.0.0.1` unless running a purely local private experiment;
- WSL-only loopback URLs for remote product flows;
- old cloud-control-plane MCP examples;
- a self-host Relay `/mcp` endpoint unless you are deliberately running
  advanced/direct/debug mode.

## Generate Setup

```bash
codencer activation self-host --gateway http://127.0.0.1:19090 --relay https://relay.example.com --project codencer --token-env CODENCER_GATEWAY_MCP_TOKEN --json
codencer setup mcp --client claude-code --endpoint http://127.0.0.1:19090/mcp --token-env CODENCER_GATEWAY_MCP_TOKEN --json
```

Canonical command shape:

```bash
claude mcp add \
  --transport http \
  --header "Authorization: Bearer $CODENCER_GATEWAY_MCP_TOKEN" \
  codencer \
  http://127.0.0.1:19090/mcp
```

Direct self-host Relay snippets remain available for advanced/direct/debug
testing, but they are not the public Gateway-first connector path.

## Narrow Product Smoke

Only mark Claude Code proof passed when all of this evidence exists:

1. Claude Code is configured with the Gateway MCP endpoint.
2. Claude Code connects successfully.
3. Claude Code calls `codencer.list_projects`.
4. If execution proof is claimed, Claude Code calls an execution tool against a
   shared fake/local project and Codencer returns a structured result or blocker.
5. Evidence is saved with timestamps and the exact command/config used.

Until then, Claude Code proof is pending/manual, not passed.

## Notes

Claude Code as an MCP client is separate from the local `claude` executor
adapter. The MCP client talks to Gateway; Gateway routes to the selected Relay,
then to the local connector and daemon. The executor adapter, when configured,
runs locally through the daemon.

Project listings include `locations[]`. If multiple online machines advertise
the same `project_id`, pass `machine_id` or `host_label`; otherwise Codencer
returns `ambiguous_project_location`.
