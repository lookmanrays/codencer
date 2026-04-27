# Cloud MCP Tools

Codencer exposes a tenant-scoped remote MCP surface from the cloud control plane in composed runtime mode.

This page is about cloud tenancy mode.

If you are operating the self-host relay directly without cloud tenancy, use relay `/mcp` instead and treat [Relay MCP Tools](relay_tools.md) as the source of truth for that boundary.

## Endpoint

Use the cloud MCP endpoint:

- `POST /api/cloud/v1/mcp`
- `GET /api/cloud/v1/mcp`
- `DELETE /api/cloud/v1/mcp`

OAuth protected-resource metadata:

- `GET /.well-known/oauth-protected-resource/api/cloud/v1/mcp`

Compatibility path:

- `POST /api/cloud/v1/mcp/call`

The cloud MCP server currently supports:

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

- `codencer.list_instances` and `codencer.get_instance` require `runtime_instances:read`.
- Mutating tools require explicit `instance_id`.
- Tool calls respect the same tenant scope and runtime scopes as the cloud HTTP API.
- Tool calls do not bypass claimed-runtime ownership or org/workspace/project visibility.
- Routed `step`, `artifact`, and `gate` lookups stay inside the caller's authorized tenant-visible runtime set.
- `approve_gate`, `reject_gate`, and `retry_step` require explicit `instance_id`.
- `submit_task` accepts the real Codencer `TaskSpec` shape.
- `wait_step` is bounded and takes explicit timeout input.
- `list_run_gates` is the canonical gate-discovery tool for a known run and instance.
- run listing remains HTTP-only in this phase; there is no `codencer.list_runs` tool.
- `get_step_logs` returns collected step logs as explicit text or base64-safe content metadata.
- `get_artifact_content` reads by `artifact_id` and returns text or base64-safe content metadata.
- `abort_run` returns a successful tool result only when the daemon confirms the active step reached `cancelled`.
- There is no raw shell tool.
- There is no arbitrary filesystem browsing tool.

## Transport Notes

- `/api/cloud/v1/mcp` supports session-bound Streamable HTTP `GET`, `POST`, and `DELETE`
- the cloud daemon returns `MCP-Protocol-Version`
- the cloud daemon can return `MCP-Session-Id` on `initialize`
- unauthenticated MCP calls return a bearer `WWW-Authenticate` challenge with `resource_metadata` pointing at the metadata URL above
- `public_base_url` controls the public resource URL used in metadata and auth challenges
- `allowed_origins` can explicitly permit browser-style remote MCP origins
- `oauth_authorization_servers`, `oauth_scopes_supported`, and `oauth_resource_documentation` populate the metadata for OAuth-capable product front doors
- `GET /api/cloud/v1/mcp` keeps an SSE stream open for the negotiated session and emits keepalive comments
- `POST /api/cloud/v1/mcp/call` remains as a compatibility alias for simple POST callers; `/api/cloud/v1/mcp` is still the canonical session path
- the compatibility alias accepts both full JSON-RPC `tools/call` requests and the shorthand top-level `name` / `arguments` form
- the Codencer tool model remains intentionally request/response-oriented even though the transport now supports a real SSE session
- cloud MCP sessions are token-bound; a session cannot be reused by a different token, and revoked tokens are rejected across the cloud MCP surface

## Proven Compatibility

- verified in repo tests for initialize, tools/list, tools/call, stream bootstrap, session delete, browser-origin handling, compatibility aliasing, token-bound sessions, and revoked-token denial
- verified in repo tests for OAuth protected-resource metadata, 401 bearer challenges, and configured cloud MCP CORS origins
- verified in composed cloud smoke for initialize, list, call, and official Go SDK access
- not overclaimed as universal client compatibility beyond the integrations directly exercised here

## Payload Notes

`codencer.get_step_logs` and `codencer.get_artifact_content` return structured metadata rather than raw path access.

The payload includes:

- `content_type`
- `encoding`
- `text` for textual content
- `base64` for non-text content

This matches the relay MCP shape so remote MCP clients do not need a different content model for cloud.
