# ChatGPT-Style Planner — Codencer Walkthrough

This walkthrough is for the post-beta flagship planner loop and the OpenAI docs linked here as checked on 2026-04-28.

Use this page together with [Beta Testing](../../BETA_TESTING.md) and [Planner / Client Integration Notes](../integrations.md).

## Status

Codencer status for the ChatGPT-style operator path is `operator-packaged`.

What that means in practice:

- Codencer proves the relay/cloud MCP protocol surfaces directly.
- Codencer ships a flagship loop smoke for the Codencer side of the path.
- Codencer exposes OAuth protected-resource metadata and bearer challenges for product-facing remote MCP setup.
- Codencer does not claim universal proof of the ChatGPT product UI, plan eligibility, approval UX, or deployment flow.
- ChatGPT must target the remote relay `/mcp` surface or the remote cloud `/api/cloud/v1/mcp` surface.
- Do not point ChatGPT at the daemon-local `/mcp/call` endpoint.

Publishable write-capable target: ChatGPT custom MCP connectors/apps on Business, Enterprise, and Edu workspaces with OAuth dev auth for self-host testing or an operator-owned OAuth front door for production IAM.

Narrower adjacent paths: OpenAI Responses API remote MCP and read/fetch-only product paths may be useful for testing, but they are not the baseline for write-capable Codencer operation.

## Prerequisites

Before you open ChatGPT, have all of the following ready:

- A running self-host relay or a running self-host cloud control plane in composed runtime mode.
- A remote MCP URL:
  - relay: `https://<your-relay-host>/mcp`
  - cloud: `https://<your-cloud-host>/api/cloud/v1/mcp`
- Auth for that surface:
  - private/API testing: relay planner token or cloud token where the client supports bearer credentials
  - ChatGPT product setup: Sprint 7 OAuth dev mode for self-host testing, or an operator-owned issuer/gateway that produces or forwards a bearer token Codencer accepts
- At least one reachable shared runtime instance:
  - relay path: shared through the connector and visible from relay `/mcp`
  - cloud path: claimed into org/workspace/project scope and visible from cloud `/api/cloud/v1/mcp`
- For cloud, token scopes that cover runtime discovery and run execution. `codencer.list_instances` requires `runtime_instances:read`.
- A ChatGPT Business, Enterprise, or Edu workspace when you need write/modify-capable custom MCP connector use.

Treat Business/Enterprise/Edu workspace setup as the reliable lane for write-capable operator use unless your current OpenAI account explicitly exposes the needed remote MCP app flow. Verify current eligibility before rollout in the live OpenAI docs:

- [Building MCP servers for ChatGPT and API integrations](https://platform.openai.com/docs/mcp/)
- [ChatGPT Developer mode](https://platform.openai.com/docs/guides/developer-mode)

## Step 1 Choose the Codencer surface

Pick one public MCP surface and stay on it for the whole setup:

- Relay self-host path: [Self-Host Relay / Runtime Reference](../../SELF_HOST_REFERENCE.md) and [Relay MCP Tools](../relay_tools.md)
- Cloud self-host path: [Self-Host Cloud Control Plane Guide](../../CLOUD_SELF_HOST.md) and [Cloud MCP Tools](../cloud_tools.md)

Do not use:

- daemon-local `/mcp/call`
- `http://127.0.0.1:8085/...`
- a localhost-only relay or cloud URL that ChatGPT cannot reach remotely

## Step 2 Enable developer mode in ChatGPT

Use the current OpenAI UI path for your plan and verify it against the live docs before rollout.

Current OpenAI guidance checked on 2026-04-28:

- Admin workspace enablement path in the Help Center:
  - `Workspace Settings -> Permissions & Roles -> Connected Data Developer mode / Create custom MCP connectors`
- Developer guide path:
  - `Settings -> Apps & Connectors -> Advanced settings -> Developer mode`
- Enterprise/Edu user path in the Help Center:
  - `Settings -> Apps -> Advanced Settings -> Developer mode`
- Create-app entry point after developer mode is enabled:
  - `Settings -> Connectors -> Create`
  - or workspace-admin path `Workspace Settings -> Apps -> Create`

OpenAI’s current references:

- [Building MCP servers for ChatGPT and API integrations](https://platform.openai.com/docs/mcp/)
- [ChatGPT Developer mode](https://platform.openai.com/docs/guides/developer-mode)

Keep the claim narrow: OpenAI currently documents ChatGPT as a remote MCP client surface. This Codencer walkthrough does not imply local daemon support in ChatGPT.

## Step 3 Create the ChatGPT app entry

In the ChatGPT create flow, use the Codencer remote MCP endpoint for the surface you chose in Step 1.

Operator inputs:

- relay endpoint: `https://<your-relay-host>/mcp`
- cloud endpoint: `https://<your-cloud-host>/api/cloud/v1/mcp`
- relay token: `<your-planner-token>`
- cloud token: `<your-cloud-token>`
- relay OAuth metadata: `https://<your-relay-host>/.well-known/oauth-protected-resource/mcp`
- cloud OAuth metadata: `https://<your-cloud-host>/.well-known/oauth-protected-resource/api/cloud/v1/mcp`

Auth choices:

- ChatGPT product connector setup: use OAuth mode. For self-host testing, `codencer setup relay --enable-chatgpt-oauth-dev` exposes authorization metadata and PKCE token exchange. For production IAM, use an operator-owned issuer/gateway that forwards a Codencer bearer token upstream.
- Private API-style clients: bearer-token mode is repo-proven where the client supports bearer credentials.
- No-auth product setup is not appropriate for a write-capable Codencer connector.

The checked-in example files are operator value references only:

- [chatgpt-relay.mcp.json](../examples/chatgpt-relay.mcp.json)
- [chatgpt-cloud.mcp.json](../examples/chatgpt-cloud.mcp.json)

They are not direct ChatGPT imports. ChatGPT developer mode currently asks you to enter the endpoint and auth/metadata through its own UI. Use the files as copy-pasteable reference values, then verify the exact current ChatGPT auth fields in the OpenAI docs above before rollout.

## Step 4 Run the first tool call from ChatGPT

Start with discovery, not execution.

Ask ChatGPT to call this exact tool first:

- `codencer.list_instances`

Tool arguments:

```json
{}
```

Recommended prompt:

```text
Use only the Codencer app for this turn. Call codencer.list_instances with {} and show me the JSON response.
```

Expected JSON shape:

```json
[
  {
    "instance_id": "inst-<opaque>"
  }
]
```

Treat that as the minimum contract you should rely on from ChatGPT.

Additional fields differ by surface:

- relay commonly returns fields such as `connector_id`, `repo_root`, `base_url`, `online`, `status`, `last_seen_at`, and nested `instance`
- cloud commonly returns tenant-scoped runtime instance fields such as `org_id`, `workspace_id`, `project_id`, `runtime_connector_installation_id`, `repo_root`, `status`, `enabled`, `health`, and `shared`

For the walkthrough, you only need one returned object with a non-empty `instance_id`.

## Step 5 Start a run

After you have a target `instance_id`, ask ChatGPT to call `codencer.start_run`.

Tool arguments:

```json
{
  "instance_id": "<instance-id>",
  "payload": {
    "id": "chatgpt-smoke-001",
    "project_id": "chatgpt-smoke"
  }
}
```

Recommended prompt:

```text
Use only the Codencer app. Call codencer.start_run with the selected instance_id and payload {"id":"chatgpt-smoke-001","project_id":"chatgpt-smoke"}.
```

`codencer.start_run` is the correct Codencer MCP tool name. There is no ChatGPT-specific alias in this repo.

## Step 6 Submit the task

Use the real Codencer `TaskSpec` shape through `codencer.submit_task`.

For a repeatable compatibility smoke, prefer a simulation task so you can validate the ChatGPT-to-Codencer wiring without depending on a live adapter binary:

```json
{
  "instance_id": "<instance-id>",
  "run_id": "chatgpt-smoke-001",
  "task": {
    "version": "v1",
    "goal": "Compatibility smoke only. Return the repository root in the final summary. Do not edit files.",
    "is_simulation": true
  }
}
```

Recommended prompt:

```text
Use only the Codencer app. Call codencer.submit_task with the selected instance_id, run_id "chatgpt-smoke-001", and the simulation task payload. Then show me the returned step JSON.
```

Expected response shape is a step object. In current repo proof, the minimum fields you should rely on are:

```json
{
  "id": "step-<opaque>",
  "state": "queued"
}
```

Record the returned `id` as `step_id`.

If you want a live execution attempt instead of a compatibility smoke, remove `is_simulation` and add the appropriate `adapter_profile`. That is a separate operator decision and is not required for this walkthrough.

For the flagship local Codex loop, use:

```json
{
  "instance_id": "<instance-id>",
  "run_id": "chatgpt-smoke-001",
  "task": {
    "version": "v1",
    "goal": "Make one narrow repository-safe change or inspect the repo and report the result. Do not choose a follow-up task.",
    "adapter_profile": "codex",
    "validations": [
      {
        "name": "focused-check",
        "command": "go test ./... -count=1"
      }
    ]
  }
}
```

## Step 7 Wait for completion and inspect the result

Use the real Codencer polling/result tools:

1. `codencer.wait_step`
2. `codencer.get_step_result`

There is no separate `get_result` tool in this repo. The actual result tool name is `codencer.get_step_result`.

Suggested `codencer.wait_step` arguments:

```json
{
  "step_id": "<step-id>",
  "timeout_ms": 5000,
  "interval_ms": 100
}
```

Suggested `codencer.get_step_result` arguments:

```json
{
  "step_id": "<step-id>"
}
```

Recommended prompt:

```text
Use only the Codencer app. Call codencer.wait_step for the returned step_id with timeout_ms 5000 and interval_ms 100. When it becomes terminal, call codencer.get_step_result for the same step_id and show me the JSON result.
```

Expected `wait_step` shape:

```json
{
  "step_id": "step-<opaque>",
  "state": "completed",
  "terminal": true,
  "timed_out": false
}
```

Expected `get_step_result` shape:

```json
{
  "version": "v1",
  "run_id": "chatgpt-smoke-001",
  "step_id": "step-<opaque>",
  "state": "completed",
  "summary": "..."
}
```

If you need deeper evidence after that, keep using the Codencer MCP tools already frozen in this repo:

- `codencer.get_step_validations`
- `codencer.get_step_logs`
- `codencer.list_step_artifacts`
- `codencer.list_run_gates`

## Verification smoke

Run the Codencer-side flagship proof before doing any ChatGPT product setup:

```bash
make flagship-planner-smoke
```

Use this as the repeatable operator smoke for the ChatGPT surface:

1. In ChatGPT, select only the Codencer app for the conversation.
2. Call `codencer.list_instances` with `{}`.
3. Confirm the response includes at least one object with a non-empty `instance_id`.
4. Call `codencer.start_run` with a unique run id such as `chatgpt-smoke-001`.
5. Call `codencer.submit_task` with the simulation task shown above.
6. Confirm the submit response returns a non-empty `step_id` in the step object’s `id` field.
7. Call `codencer.wait_step` until `terminal` is `true`.
8. Call `codencer.get_step_result`.
9. Confirm the final result includes a terminal state and a non-empty `summary`.

This smoke proves the ChatGPT remote MCP wiring narrowly for the operator. It does not claim universal ChatGPT product compatibility.

For the repo-level beta proof boundaries, keep using [Beta Testing](../../BETA_TESTING.md) and [Planner / Client Integration Notes](../integrations.md).

## Known Limitations on this surface

- This is an operator-packaged Codencer path for the canonical remote MCP surface, not a universal ChatGPT product-support claim.
- ChatGPT is documented here only as a remote MCP client. Do not infer local daemon support.
- Product plan availability and UI wording can change. Verify current eligibility before rollout.
- ChatGPT Agent Mode is not the write-capable custom connector target for this path; use the custom MCP connector/app setup flow documented by OpenAI.
- ChatGPT custom connectors are remote MCP only; local MCP servers are not the product target.
- Codencer does not issue OAuth authorization-code tokens; product OAuth flows need an operator-owned OAuth issuer or gateway in front of Codencer.
- OpenAI owns the ChatGPT UI, tool approval UX, draft/publish flow, and any product-side restrictions.
- The example `.mcp.json` files in this repo are value-reference templates only, not direct ChatGPT imports.
- Cloud mode requires composed runtime mode plus a claimed runtime instance. A relay planner token does not replace a cloud token, and a cloud token does not replace relay planner auth.
- Write-style tools in ChatGPT may require explicit confirmation. Plan for operator review when using mutating Codencer tools.

## Troubleshooting

If the app does not appear in ChatGPT:

- Re-check the exact developer mode path in the OpenAI docs.
- Confirm developer mode is enabled for the current user, not only at the workspace level.
- Refresh the app entry from ChatGPT app settings after tool metadata changes.

If `codencer.list_instances` returns an empty array:

- relay path: confirm the connector is enrolled, running, and the instance is explicitly shared
- cloud path: confirm the runtime connector is claimed and the instance is tenant-visible
- confirm you targeted the relay `/mcp` or cloud `/api/cloud/v1/mcp` endpoint, not the daemon-local surface

If ChatGPT can connect but tool calls fail with auth or scope errors:

- relay path: verify the planner token and the relay scopes required for the tool
- cloud path: verify the cloud token and runtime scopes, including `runtime_instances:read` for discovery

If `codencer.wait_step` times out:

- call `codencer.get_step_result` and `codencer.get_step_logs` directly for more evidence
- increase `timeout_ms` for slow environments
- if you removed `is_simulation`, confirm the target adapter is actually installed and usable on the daemon host

If you accidentally pointed ChatGPT at a local-only URL:

- stop and move the configuration to a remotely reachable relay or cloud host
- do not expose the daemon-local `/mcp/call` bridge as the public ChatGPT target
