# Relay MCP Tools

Codencer exposes a direct MCP surface from the Relay for advanced/direct/debug
mode. Official client setup should use Codencer Gateway instead:

```text
AI client -> Codencer Gateway -> selected Relay -> local connector -> daemon -> project
```

This page is about direct relay mode.

For official Gateway setup, see [Official Gateway activation](../activation-official-gateway.md) and [MCP Gateway model](../architecture/mcp-gateway-model.md). Experimental cloud-control-plane MCP behavior is documented separately in [Cloud MCP Tools](cloud_tools.md) and is not the hosted Codencer Gateway/Cloud service.

For the current planner/client matrix and client-specific packaging notes, see [MCP Integrations](integrations.md).

## Endpoint

Use the relay MCP endpoint:
- `POST /mcp`
- `GET /mcp`
- `DELETE /mcp`

OAuth protected-resource metadata:
- `GET /.well-known/oauth-protected-resource/mcp`

Compatibility path:
- `POST /mcp/call`

The relay MCP server currently supports:
- `initialize`
- `notifications/initialized`
- `tools/list`
- `tools/call`

## Tool List

Project-aware tools:

- `codencer.list_projects`
- `codencer.get_project`
- `codencer.start_project_run`
- `codencer.list_project_runs`
- `codencer.get_project_run`
- `codencer.submit_project_task`
- `codencer.submit_project_task_and_wait`
- `codencer.run_project_manifest`
- `codencer.get_execution_report`
- `codencer.get_run_report` (read-only alias)
- `codencer.get_project_blocker`
- `codencer.get_blocker` (read-only alias)
- `codencer.get_project_step_result`
- `codencer.get_project_step_artifacts`
- `codencer.get_project_step_logs`
- `codencer.get_project_step_validations`

Compatibility instance tools:

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

- Project-aware tools are preferred for remote planners.
- Project tools require projects shared from the user-level registry with `shared_to_relay:true`.
- `codencer.list_projects` returns projects with `locations[]`; each location includes `machine_id`, `host_label`, connector/instance ids, status, and safe repo labels/hashes. Absolute local paths are not exposed.
- Project execution tools accept optional `machine_id` or `host_label`. If exactly one online location exists, no selector is required. If multiple online locations exist for the same `project_id`, the tool returns structured blocker `ambiguous_project_location` with `planner_decision_required:true`.
- Planner tokens can restrict project tools with `project_ids`, `instance_ids`, and scopes such as `projects:read`, `runs:write`, `steps:write`, `reports:read`, and `artifacts:read`.
- Project task and manifest tools call the same Sprint 2 local execution contract as `codencer submit` and `codencer run-plan`.
- Project blockers are returned as structured report data with `exit_code`; they are not converted into transport errors.
- Remote manifest payloads accept `manifest` or `manifest_text`; `prompt_file` is rejected for remote manifests because planner-side file paths are not local execution paths.
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
- unauthenticated MCP calls return a bearer `WWW-Authenticate` challenge with `resource_metadata` pointing at the metadata URL above
- `public_base_url` controls the public resource URL used in metadata and auth challenges
- `oauth_authorization_servers`, `oauth_scopes_supported`, and `oauth_resource_documentation` populate the metadata for OAuth-capable product front doors
- Sprint 7 self-host OAuth dev mode also exposes `/.well-known/oauth-authorization-server`, `/.well-known/openid-configuration`, `/oauth/authorize`, and `/oauth/token` for single-user ChatGPT testing
- `GET /mcp` keeps an SSE stream open for the negotiated session and emits keepalive comments
- `POST /mcp/call` remains as a compatibility alias for simple POST callers; `/mcp` is still the canonical session path
- the Codencer tool model remains intentionally request/response-oriented even though the transport now supports a real SSE session

## Client Snippets

Generate direct Relay debug snippets:

```bash
./bin/codencer-relayd mcp-config --client codex --endpoint https://relay.example.com/mcp --token-env CODENCER_PLANNER_TOKEN
./bin/codencer-relayd mcp-config --client claude-code --endpoint https://relay.example.com/mcp --token-env CODENCER_PLANNER_TOKEN
./bin/codencer-relayd mcp-config --client chatgpt --endpoint https://relay.example.com/mcp
```

ChatGPT custom MCP connector setup requires public HTTPS, OAuth protected-resource metadata, and an eligible workspace with developer mode. For self-host testing, `codencer setup relay --enable-chatgpt-oauth-dev` enables a minimal OAuth dev issuer. For production IAM, use an operator-owned OAuth front door.

Activation artifacts:

```bash
./bin/codencer activation package --relay https://relay.example.com --project codencer --token-env CODENCER_MCP_TOKEN --json
./bin/codencer activation chatgpt --relay https://relay.example.com --project codencer --auth oauth --json
./bin/codencer activation codex --relay https://relay.example.com --token-env CODENCER_MCP_TOKEN --json
./bin/codencer activation claude-code --relay https://relay.example.com --token-env CODENCER_MCP_TOKEN --json
```

## Proven Compatibility

- verified in repo tests against the official Go SDK `StreamableClientTransport`
- verified for manual JSON-RPC callers using `POST /mcp` and `POST /mcp/call`
- verified for bearer-token auth, OAuth protected-resource metadata, and 401 bearer challenges
- not overclaimed as universal client compatibility beyond the integrations directly exercised here

## Local MCP Distinction

The daemon-local `/mcp/call` endpoint is separate.

It is useful as a local compatibility/admin bridge, but it is not the public remote MCP surface for planner integrations.
