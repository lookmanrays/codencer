# MCP Integration Notes

Codencer v2’s public MCP surface lives on the relay.

Do not point ChatGPT, Claude, or another planner runtime at the local Codencer daemon directly.

## Recommended Self-Host Pattern

1. Run the local Codencer daemon near the repo and adapters.
2. Run the connector on the same side as that daemon.
3. Run the relay as the authenticated remote control plane.
4. Point the MCP client at the relay MCP endpoint.

## Endpoint

- `POST /mcp`

Compatibility path:
- `POST /mcp/call`

Supported MCP methods:
- `initialize`
- `tools/list`
- `tools/call`

## Auth And Scope

- Use the same planner bearer token model as the relay HTTP API.
- MCP tool calls do not bypass relay scopes.
- MCP tool calls do not bypass instance sharing or connector allowlists.

## Transport Expectations

The relay MCP surface is JSON-RPC over HTTP.

It is intentionally narrow and maps to relay behavior rather than exposing a second planner or execution protocol.

## Compatibility Note

- Self-host mode is implemented in this repo now.
- A future default or managed relay can expose the same narrow Codencer MCP surface without changing the local daemon contract.
