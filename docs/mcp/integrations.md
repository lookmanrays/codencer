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
- `GET /mcp`
- `DELETE /mcp`

Compatibility path:
- `POST /mcp/call`

Supported MCP methods:
- `initialize`
- `notifications/initialized`
- `tools/list`
- `tools/call`

## Auth And Scope

- Use the same planner bearer token model as the relay HTTP API.
- MCP tool calls do not bypass relay scopes.
- MCP tool calls do not bypass instance sharing or connector allowlists.

## Transport Expectations

The relay MCP surface is intentionally narrow and tool-oriented:
- JSON-RPC over HTTP POST is supported for straightforward planner integrations
- `/mcp` also supports Streamable HTTP-style `GET`, `POST`, and `DELETE`
- `MCP-Protocol-Version` negotiation and `MCP-Session-Id` are supported
- `allowed_origins` can be enforced for browser-style callers

The current implementation remains request/response-first. It is suitable for serious self-host personal use now, but it does not rely on unsolicited long-lived server notifications to expose Codencer functionality.
The daemon-local `/mcp/call` endpoint is only a local compatibility/admin bridge and should not be used as the public remote integration target.

## Compatibility Note

- Self-host mode is implemented in this repo now.
- A future default or managed relay can expose the same narrow Codencer MCP surface without changing the local daemon contract.
