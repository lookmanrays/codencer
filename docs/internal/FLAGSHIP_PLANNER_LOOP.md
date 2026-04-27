# Flagship Planner Loop Contract

Status: post-beta implementation contract

Last audited: 2026-04-27

This document freezes the narrow flagship loop for external planners that use Codencer as a bridge to local Codex.

Codencer remains the bridge, not the planner. The planner decides the next task. Codencer executes one submitted task, records state, returns evidence, and waits for the next planner decision.

## Canonical Surface

Primary flagship path:

- external planner client
- relay MCP at `POST|GET|DELETE /mcp`
- local connector over the relay websocket
- local Codencer daemon
- local `codex` adapter using `codex exec`

Secondary equivalent path:

- cloud MCP at `POST|GET|DELETE /api/cloud/v1/mcp`
- composed cloud runtime bridge
- claimed runtime connector
- local Codencer daemon
- local `codex` adapter using `codex exec`

HTTP remains supported for generic planner automation:

- relay HTTP under `/api/v2/...`
- cloud runtime HTTP under `/api/cloud/v1/runtime/...`

For ChatGPT-style and Claude-style planner packaging, prefer the MCP surface. Use HTTP for scripts, diagnostics, and simple bearer-token clients.

## Auth Modes

Two auth modes are now part of the flagship path.

### Bearer-Token Mode

Bearer-token mode is the proven private/self-host mode.

- relay MCP uses `Authorization: Bearer <planner-token>`
- cloud MCP uses `Authorization: Bearer <cloud-token>`
- relay tokens come from `planner_token` or `planner_tokens[]`
- cloud tokens are hashed `cct_...` API tokens scoped by org/workspace/project and runtime permissions

Bearer mode is what the repo tests, MCP SDK helper, self-host smoke, cloud smoke, and flagship smoke use for actual tool calls.

### OAuth-Capable Protected Resource Mode

OAuth-capable mode is a product-facing resource-server shape, not a built-in Codencer identity provider.

Codencer exposes OAuth protected-resource metadata and 401 challenges for both canonical MCP endpoints:

- relay metadata: `GET /.well-known/oauth-protected-resource/mcp`
- cloud metadata: `GET /.well-known/oauth-protected-resource/api/cloud/v1/mcp`
- relay challenge: unauthenticated `POST /mcp` returns `WWW-Authenticate: Bearer resource_metadata=".../.well-known/oauth-protected-resource/mcp"`
- cloud challenge: unauthenticated `POST /api/cloud/v1/mcp` returns `WWW-Authenticate: Bearer resource_metadata=".../.well-known/oauth-protected-resource/api/cloud/v1/mcp"`

Operators configure the externally reachable resource URL and OAuth issuer/front-door metadata with:

- relay: `public_base_url`, `oauth_authorization_servers`, `oauth_scopes_supported`, `oauth_resource_documentation`
- cloud: `public_base_url`, `oauth_authorization_servers`, `oauth_scopes_supported`, `oauth_resource_documentation`

Codencer still validates bearer tokens at the MCP resource. If a product client requires a full OAuth authorization-code flow, put Codencer behind an operator-owned OAuth issuer or gateway that mints, translates, or forwards a token Codencer accepts. This keeps Codencer as bridge/resource server, not as an identity provider.

## Loop Boundary

Inside Codencer:

- authenticate the planner request
- list tenant-visible or relay-shared instances
- route to the selected instance
- start runs
- submit TaskSpec-compatible steps
- execute through the configured local adapter
- wait or poll bounded state
- persist run, step, attempt, gate, validation, log, and artifact truth
- return structured evidence

Outside Codencer:

- phase planning
- task decomposition
- next-step selection
- deciding whether a phase is complete
- deciding whether to approve or reject gates
- deciding whether to retry with changed instructions

Codencer does not expose raw shell, arbitrary filesystem browsing, or a generic tunnel.

## Required Tool Sequence

The minimal MCP sequence is:

1. `codencer.list_instances`
2. `codencer.get_instance`
3. `codencer.start_run`
4. `codencer.submit_task`
5. `codencer.wait_step`
6. `codencer.get_step_result`
7. `codencer.get_step_validations`
8. `codencer.get_step_logs`
9. `codencer.list_step_artifacts`
10. `codencer.get_artifact_content`
11. `codencer.list_run_gates`

Gate and retry tools are planner decisions:

- `codencer.approve_gate`
- `codencer.reject_gate`
- `codencer.retry_step`
- `codencer.abort_run`

There is no `codencer.list_runs` MCP tool in this phase. Run listing is HTTP-only.

## Payload Contract

Start a run:

```json
{
  "instance_id": "inst-...",
  "payload": {
    "id": "planner-phase-001",
    "project_id": "codencer",
    "conversation_id": "optional-external-thread-id",
    "planner_id": "chatgpt-or-claude-or-script",
    "executor_id": "codex"
  }
}
```

Submit a Codex task:

```json
{
  "instance_id": "inst-...",
  "run_id": "planner-phase-001",
  "task": {
    "version": "v1",
    "title": "Fix one narrow issue",
    "goal": "Make the requested change and leave structured evidence.",
    "adapter_profile": "codex",
    "validations": [
      {
        "name": "focused-tests",
        "command": "go test ./internal/connector ./internal/relay -count=1"
      }
    ],
    "acceptance": [
      "The focused tests pass.",
      "The result summary names changed files and validation outcome."
    ],
    "timeout_seconds": 900
  }
}
```

Wait for a step:

```json
{
  "step_id": "step-...",
  "timeout_ms": 300000,
  "interval_ms": 1000,
  "include_result": true
}
```

`wait_step` now returns before timeout when a step reaches `needs_approval`. In that case:

```json
{
  "step_id": "step-...",
  "state": "needs_approval",
  "terminal": false,
  "needs_decision": true,
  "decision_kind": "gate",
  "timed_out": false
}
```

The planner must then call `codencer.list_run_gates`, ask the human/operator for a decision, and call either `codencer.approve_gate` or `codencer.reject_gate`.

## Result Interpretation

Terminal success states:

- `completed`
- `completed_with_warnings`

Planner decision states:

- `needs_approval`: inspect gates and ask the operator to approve or reject
- `needs_manual_attention`: inspect logs/result/artifacts and ask the operator for direction

Failure states:

- `failed_validation`: inspect validations and submit a corrected follow-up task
- `failed_adapter`: inspect logs and adapter readiness
- `failed_bridge`: inspect bridge error and local runtime health
- `failed_terminal`: treat as a task failure and plan a new task
- `timeout`: simplify the task or increase `timeout_seconds`
- `cancelled`: operator intervention was recorded

The phase is complete only when the external planner decides it is complete after inspecting the latest result and validations. Codencer does not mark a planner phase complete on its own.

## Gate Handling

Gate flow:

1. Submit a task.
2. Call `codencer.wait_step`.
3. If `needs_decision` is `true`, call `codencer.list_run_gates`.
4. Present gate description and current result to the human/operator.
5. Call `codencer.approve_gate` or `codencer.reject_gate` with explicit `instance_id`.
6. Re-read `codencer.get_run`, `codencer.get_step`, and `codencer.get_step_result`.
7. The planner decides whether to continue, retry, or stop.

Gate approval does not create a new task. It resolves the pending gate and reconciles recorded run/step state.

## Retry Handling

Use `codencer.retry_step` only after reading the prior result, logs, and validations. Retrying re-dispatches the existing step; it does not change the original task instructions. If the planner wants changed instructions, submit a new step instead.

## Recovery And Reconnect

Recovery flow:

1. The planner keeps `run_id`, `step_id`, and `instance_id`.
2. If the relay or connector drops, reconnect the connector with the same config.
3. Call `codencer.list_instances` and confirm the target `instance_id` is visible and online.
4. Re-read `codencer.get_run` and `codencer.get_step`.
5. Continue with `codencer.wait_step`, result inspection, gate handling, or a new task.

State truth lives in the local daemon and relay/cloud stores, not in MCP session memory. An MCP session can be reinitialized after reconnect; the planner should recover by ids.

## Multi-Instance Targeting

The planner must choose an `instance_id` explicitly before mutating state.

Rules:

- Never submit a run or task to "the first instance" unless the operator has confirmed it.
- Store the selected `instance_id` beside the external planner phase.
- Use explicit `instance_id` for mutating tools.
- If a direct step/artifact/gate lookup is ambiguous, call `codencer.get_instance` for the intended target and repeat with the routed context or use the instance-scoped HTTP route.

## Proven In This Phase

Repository proof added or strengthened in this phase:

- local Codex adapter defaults to `codex exec`
- fake-binary Codex adapter test proves non-simulation CLI invocation and result synthesis
- relay wait returns promptly at `needs_approval`
- cloud HTTP wait/retry/gate proxy parity is covered by tests
- relay and cloud MCP expose OAuth protected-resource metadata and `WWW-Authenticate` challenges for product-facing remote MCP setup
- `mcp-sdk-smoke --step-count 2` proves a same-run multi-step phase loop through MCP
- `scripts/self_host_smoke.sh` covers phase-loop, reconnect, strict gate, multi-instance, evidence retrieval, MCP auth metadata, and MCP/SDK flows
- `scripts/cloud_smoke.sh` proves cloud MCP protected-resource metadata/challenge and, when enabled, composed cloud MCP/SDK runtime access
- `make flagship-planner-smoke` runs the flagship relay loop proof with local Codex as the selected adapter profile

Still not claimed:

- universal ChatGPT product compatibility
- universal Claude product compatibility
- live authenticated Codex service proof in CI
- Codencer-native OAuth authorization-code token issuance
- arbitrary MCP client compatibility beyond repo-proven protocol and helper paths
