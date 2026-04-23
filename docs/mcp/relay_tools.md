# Relay MCP Tools

Codencer exposes the remote MCP surface from the relay, not from the local daemon.

This page is about direct relay mode.

If you are operating through Codencer Cloud tenancy and composed runtime mode, use `/api/cloud/v1/mcp` instead and treat [Cloud MCP Tools](cloud_tools.md) as the source of truth for that boundary.

For the frozen planner/client compatibility matrix, generic client examples, and client-specific packaging notes, see [Planner / Client Integration Notes](integrations.md).

## Endpoint

Use the relay MCP endpoint:
- `POST /mcp`
- `GET /mcp`
- `DELETE /mcp`

Compatibility path:
- `POST /mcp/call`

The relay MCP server currently supports:
- `initialize`
- `notifications/initialized`
- `tools/list`
- `tools/call`

## Tool List

- `codencer.list_instances`
- `codencer.get_instance`
- `codencer.start_run`
- `codencer.get_run`
- `codencer.list_run_gates`
- `codencer.submit_task`
- `codencer.get_step`
- `codencer.wait_step`
- `codencer.get_step_result`
- `codencer.list_step_artifacts`
- `codencer.get_step_logs`
- `codencer.get_artifact_content`
- `codencer.get_step_validations`
- `codencer.approve_gate`
- `codencer.reject_gate`
- `codencer.abort_run`
- `codencer.retry_step`

## Tool Rules

- Mutating tools require explicit `instance_id`.
- Tool calls respect the same planner auth scopes as the relay HTTP API.
- Tool calls do not bypass connector sharing or instance routing.
- Direct `step`, `artifact`, and `gate` lookups do not require prior observation of those ids; the relay probes only authorized online shared instances and persists successful route hints.
- `approve_gate`, `reject_gate`, and `retry_step` require explicit `instance_id` even though the corresponding relay HTTP routes can resolve routed ids implicitly.
- `submit_task` accepts the real Codencer `TaskSpec` shape.
- `wait_step` is bounded and takes explicit timeout input.
- `list_run_gates` is the canonical gate-discovery tool for a known run and instance.
- run listing remains HTTP-only in this phase; there is no `codencer.list_runs` tool yet.
- `get_step_logs` returns the collected step logs as explicit text or base64-safe content metadata.
- `get_artifact_content` reads by `artifact_id` and returns text or base64-safe content metadata.
- `abort_run` returns a successful tool result only when the daemon confirms the active step reached `cancelled`.
- There is no raw shell tool.
- There is no arbitrary filesystem browsing tool.

## Transport Notes

- `/mcp` supports session-bound Streamable HTTP `GET`, `POST`, and `DELETE`
- the relay returns `MCP-Protocol-Version`
- the relay can return `MCP-Session-Id` on `initialize`
- `GET /mcp` keeps an SSE stream open for the negotiated session and emits keepalive comments
- `POST /mcp/call` remains as a compatibility alias for simple POST callers; `/mcp` is still the canonical session path
- the Codencer tool model remains intentionally request/response-oriented even though the transport now supports a real SSE session

## Proven Compatibility

- verified in repo tests against the official Go SDK `StreamableClientTransport`
- verified for manual JSON-RPC callers using `POST /mcp` and `POST /mcp/call`
- not overclaimed as universal client compatibility beyond the integrations directly exercised here

## Local MCP Distinction

The daemon-local `/mcp/call` endpoint is separate.

It is useful as a local compatibility/admin bridge, but it is not the public remote MCP surface for planner integrations.
