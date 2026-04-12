# Relay MCP Tools

Codencer exposes the remote MCP surface from the relay, not from the local daemon.

## Endpoint

Use the relay MCP endpoint:
- `POST /mcp`

Compatibility path:
- `POST /mcp/call`

The relay MCP server currently supports:
- `initialize`
- `tools/list`
- `tools/call`

## Tool List

- `codencer.list_instances`
- `codencer.get_instance`
- `codencer.start_run`
- `codencer.get_run`
- `codencer.submit_task`
- `codencer.get_step`
- `codencer.wait_step`
- `codencer.get_step_result`
- `codencer.list_step_artifacts`
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
- `submit_task` accepts the real Codencer `TaskSpec` shape.
- `wait_step` is bounded and takes explicit timeout input.
- `get_artifact_content` reads by `artifact_id` and returns text or base64-safe content metadata.
- `abort_run` returns a successful tool result only when the daemon confirms the active step reached `cancelled`.
- There is no raw shell tool.
- There is no arbitrary filesystem browsing tool.

## Local MCP Distinction

The daemon-local `/mcp/call` endpoint is separate.

It is useful as a local compatibility/admin bridge, but it is not the public remote MCP surface for planner integrations.
