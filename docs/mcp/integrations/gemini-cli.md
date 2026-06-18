# Gemini CLI MCP Integration Notes

Gemini CLI integration remains expected/operator-packaged. Codencer proves the
self-host Relay MCP surface directly, but this repository does not claim Gemini
CLI product proof unless an operator runs Gemini CLI and saves evidence.

Use this page with:

- [MCP integrations](../integrations.md)
- [Relay MCP tools](../relay_tools.md)
- [Self-host Relay quickstart](../../quickstart-self-host-relay.md)

## Current Surface

Use the self-host Relay MCP endpoint:

```text
https://<relay-host>/mcp
```

Do not point Gemini CLI at the local daemon or a loopback URL unless you are
running a private local experiment and labeling the result accordingly.

## Operator Setup Shape

Configure Gemini CLI with:

- HTTP MCP server URL: `https://<relay-host>/mcp`
- Authorization header: `Bearer <planner-token>`
- Server name: `codencer`

Then ask Gemini CLI to make a non-destructive first call:

```text
Use the Codencer relay MCP server only. Call codencer.list_projects and report
the project ids and locations.
```

## Proof Boundary

Only mark Gemini CLI proof passed when Gemini CLI actually connects, calls a
tool, and evidence is saved. Until then, this page is setup guidance and the
proof gate remains pending/manual.

If multiple online machines advertise the same `project_id`, execution tools
must pass `machine_id` or `host_label`; otherwise Codencer returns
`ambiguous_project_location`.
