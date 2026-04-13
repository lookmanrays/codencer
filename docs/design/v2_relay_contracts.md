# Codencer V2 Relay Contracts

> [!IMPORTANT]
> This file is historical design context for the v2 relay build-out.
> It is not the canonical source of current runtime behavior.
>
> For current self-host/runtime truth, use:
> - [README.md](../../README.md)
> - [docs/SELF_HOST_REFERENCE.md](../SELF_HOST_REFERENCE.md)
> - [docs/RELAY.md](../RELAY.md)
> - [docs/CONNECTOR.md](../CONNECTOR.md)
> - [docs/mcp/relay_tools.md](../mcp/relay_tools.md)

Status: verified against the current repository on 2026-04-12.

This file separates:
- `Verified current repo reality`: code already present in this repo.
- `Locked v2 contract`: exact interface to implement next where the current repo is incomplete or inconsistent.

## 1. Reusable Current Surfaces

### 1.1 CLI

Verified current repo reality:

| Surface | Verified current behavior | Reuse |
| --- | --- | --- |
| `cmd/orchestratord` | Local daemon. Starts HTTP API. Supports `--config` and `--repo-root`. | Reuse as the local control plane. |
| `cmd/orchestratorctl` | Local operator CLI for `run`, `step`, `submit`, `gate`, `antigravity`, `doctor`, `instance`, `version`. | Reuse as local admin/debug surface. Do not make it the relay protocol. |
| `cmd/codencer-connectord` | `enroll` and `run`. Persists connector config locally. Opens outbound websocket to relay. | Canonical connector entrypoint. |
| `cmd/codencer-relayd` | Self-hostable relay daemon with sqlite store. | Canonical relay entrypoint. |
| `cmd/broker` | Optional Antigravity host-side broker for discovery/binding. | Reuse only for Antigravity topology; not part of relay protocol. |

### 1.2 Local Daemon REST API

Verified current repo reality:

| Method | Path | Notes |
| --- | --- | --- |
| `GET` | `/api/v1/runs` | Filters supported via query. |
| `POST` | `/api/v1/runs` | Starts a run. Request body is run metadata. |
| `GET` | `/api/v1/runs/{id}` | Returns run. |
| `PATCH` | `/api/v1/runs/{id}` | Supports `{"action":"abort"}`. |
| `GET` | `/api/v1/runs/{id}/steps` | Lists steps for a run. |
| `POST` | `/api/v1/runs/{id}/steps` | Accepts `domain.TaskSpec`. |
| `GET` | `/api/v1/runs/{id}/gates` | Lists gates for a run. |
| `GET` | `/api/v1/steps/{id}` | Returns step. |
| `GET` | `/api/v1/steps/{id}/result` | Returns normalized result envelope. |
| `GET` | `/api/v1/steps/{id}/validations` | Returns validations. |
| `GET` | `/api/v1/steps/{id}/artifacts` | Returns artifacts for latest attempt. |
| `GET` | `/api/v1/steps/{id}/logs` | Returns log artifact content via artifact lookup. |
| `GET` | `/api/v1/artifacts/{id}/content` | Returns artifact bytes/content. |
| `POST` | `/api/v1/gates/{id}` | Supports `{"action":"approve"}` and `{"action":"reject"}`. |
| `GET` | `/api/v1/compatibility` | Runtime-derived adapter/environment truth. |
| `GET` | `/api/v1/routing` | Current routing config surface. |
| `GET` | `/api/v1/benchmarks` | Benchmark surface. |
| `GET` | `/api/v1/instance` | Stable daemon identity and local addressing info. |
| `GET` | `/api/v1/antigravity/instances` | Antigravity discovery. |
| `GET` | `/api/v1/antigravity/status` | Bound Antigravity instance status. |
| `POST`,`DELETE` | `/api/v1/antigravity/bind` | Bind/unbind Antigravity instance. |
| `POST` | `/mcp/call` | Local MCP-like JSON-RPC tool shim. |

### 1.3 Instance Identity

Verified current repo reality:

- `domain.InstanceInfo` includes stable `id`.
- The daemon persists `daemon_instance_id` in sqlite settings.
- `/api/v1/instance` returns:
  - `id`
  - `version`
  - `repo_root`
  - `state_dir`
  - `workspace_root`
  - `host`
  - `port`
  - `base_url`
  - `execution_mode`
  - `pid`
  - `started_at`

Reuse decision:

- Treat daemon `id` as the canonical local instance identity for connector and relay routing.
- Do not invent a second local instance identity source in connector config.

### 1.4 Adapter Registry and Capability Surface

Verified current repo reality:

- Adapters are registered in bootstrap, not discovered from docs.
- Current registered adapter IDs:
  - `codex`
  - `claude`
  - `qwen`
  - `ide-chat`
  - `openclaw-acpx`
  - `antigravity`
  - `antigravity-broker`
- `/api/v1/compatibility` returns `domain.CompatibilityInfo`:
  - `tier`
  - `adapters[]`
  - `environment`
- Each adapter entry includes:
  - `id`
  - `available`
  - `status`
  - `mode`
  - `capabilities[]`

Reuse decision:

- Relay should consume and expose this runtime-derived compatibility surface.
- Do not hardcode support matrices in relay or MCP.

### 1.5 Artifact and Result Retrieval

Verified current repo reality:

- Result retrieval is service-backed:
  - `RunService.GetResultByStep`
  - `ensureResultEnvelope` fills `version`, `run_id`, `step_id`, `state`, `summary`.
- Artifact retrieval is service-backed:
  - `RunService.GetArtifact`
  - `RunService.GetArtifactContent`
  - `RunService.GetLogsByStep`
- `GET /api/v1/steps/{id}/logs` no longer dereferences a raw path in the handler.
- `GET /api/v1/artifacts/{id}/content` exists and is the canonical content read path.

Reuse decision:

- Connector and relay should only use service-backed retrieval surfaces.
- Do not add direct path reads in connector, relay, or MCP.

### 1.6 Broker Integration

Verified current repo reality:

- `AntigravityService` supports:
  - direct discovery mode
  - broker mode when `brokerURL` is configured
- Broker mode uses:
  - `GET /instances`
  - `POST /binding`
  - `DELETE /binding`
  - `GET /binding`
- Bind/unbind is repo-root keyed.
- Broker persists binding outside Codencer state.

Reuse decision:

- Keep broker integration as an executor-side topology detail.
- Do not route planner traffic through the Antigravity broker.

### 1.7 Current MCP Shim Status

Verified current repo reality:

- Local daemon exposes `/mcp/call` through `internal/mcp` as a legacy compatibility/admin bridge.
- Relay exposes `/mcp` and `/mcp/call` through `internal/relay/mcp_server.go`.
- The relay MCP surface supports `initialize`, `tools/list`, and `tools/call`.
- Local daemon tool names:
  - `orchestrator.start_run`
  - `orchestrator.list_runs`
  - `orchestrator.start_step`
  - `orchestrator.retry_step`
  - `orchestrator.get_status`
  - `orchestrator.get_step_result`
  - `orchestrator.get_validations`
  - `orchestrator.list_artifacts`
  - `orchestrator.approve_gate`
  - `orchestrator.reject_gate`
  - `orchestrator.get_benchmarks`
  - `orchestrator.get_routing_config`
- Relay tool names:
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

Verified remaining limitation:

- The daemon-local MCP surface remains legacy/local-only and should not be used as the public remote integration target.

## 2. Missing Pieces For Secure Remote Planner Callability

Verified gaps in the current repo:

| Area | Verified current repo reality | Blocker |
| --- | --- | --- |
| Planner auth | Relay uses static bearer tokens with scopes and optional instance restrictions. | Honest alpha-grade self-host auth; no rotation or enterprise IAM. |
| Connector auth | Enrollment uses one-time tokens or legacy bootstrap secret, plus signed challenge/response for websocket sessions. | Revocation/disable flows are still operator-light. |
| Presence/discovery | Relay persists advertised instances and tracks heartbeat-driven session presence. | Offline routing still depends on current relay state and TTL expiry. |
| Instance descriptor | Relay stores `instance_id`, `connector_id`, `repo_root`, `base_url`, raw compatibility JSON, `last_seen_at`. | Planner-facing normalization is still lightweight. |
| Cancellation | Local daemon supports honest abort and relay exposes abort passthrough. | Abort is still best-effort unless the adapter actually stops; success is only returned on a real `cancelled` outcome. |
| Wait semantics | Relay HTTP and MCP both expose bounded `wait_step`. | No streaming/log-tail transport; wait remains poll-based. |
| Artifact content | Local daemon exposes `/api/v1/artifacts/{id}/content`. Relay proxies it. | Large binary transport remains intentionally bounded. |
| Artifact metadata by ID | Service can load artifact by ID. | No `GET /api/v1/artifacts/{id}` local endpoint; relay cannot build metadata-rich artifact responses from ID alone without cached context. |
| Gate lifecycle | Local gate approval/rejection reconciles step and run state. | No local `GET /api/v1/gates/{id}` read surface; relay gate responses cannot return gate object without extra work. |
| Resource routing | Relay persists observed route hints and probes authorized online shared instances when a `step`, `artifact`, or `gate` route is missing. | Direct lookups still fail closed when no online match exists or multiple instances match. |
| Capability introspection | Daemon compatibility is runtime-derived and truthful. | Relay lists raw stored rows; planner-facing compatibility contract is not normalized. |
| Contract drift | `schemas/result.schema.json` lagged behind `domain.StepState`. | Fixed in this change; keep schema and domain state sets aligned. |

## 3. Locked V2 Contract Package

This section defines the exact contract to implement and preserve across connector, relay, and relay-side MCP.

### 3.1 Connector Enrollment

Current implementation:

- Request and response already exist in `internal/relayproto/types.go`.
- Enrollment now sends connector public key and machine metadata.

Locked v2 contract:

Request:

```json
{
  "enrollment_token": "string",
  "label": "string",
  "public_key": "base64-ed25519-public-key",
  "machine": {
    "hostname": "string",
    "os": "linux",
    "arch": "amd64"
  }
}
```

Response:

```json
{
  "connector_id": "connector-<opaque>",
  "machine_id": "machine-<opaque>",
  "relay": {
    "relay_url": "https://relay.example",
    "websocket_url": "wss://relay.example/ws/connectors",
    "heartbeat_interval_seconds": 15
  }
}
```

Rules:

- Connector private key stays local and is used for session signing.
- Relay stores the connector public key and machine binding.
- `label` is optional but should be persisted by relay for operator visibility.

### 3.2 Connector Challenge

Current implementation:

- Missing.

Locked v2 contract:

Request:

```json
{
  "connector_id": "connector-<opaque>",
  "machine_id": "machine-<opaque>"
}
```

Response:

```json
{
  "challenge_id": "challenge-<opaque>",
  "nonce": "base64url",
  "relay": {
    "relay_url": "https://relay.example",
    "websocket_url": "wss://relay.example/ws/connectors",
    "heartbeat_interval_seconds": 15
  }
}
```

Proof rule:

- `signature = base64(Ed25519Sign(private_key, challenge_id + ":" + nonce + ":" + connector_id + ":" + machine_id))`

### 3.3 Connector Session Hello

Current implementation:

- Current websocket first message is a signed `hello`, followed by `advertise`.

Locked v2 contract:

```json
{
  "type": "hello",
  "connector_id": "connector-<opaque>",
  "machine_id": "machine-<opaque>",
  "challenge_id": "challenge-<opaque>",
  "signature": "base64-ed25519-signature"
}
```

Rules:

- Connector follows `hello` with an `advertise` message carrying one or more shared local instances.
- Relay treats each advertised `instance.id` as the routing key.
- Connector config persists connector identity and an explicit shared-instance allowlist.

### 3.4 Connector Heartbeat

Current implementation:

- Missing explicit heartbeat.

Locked v2 contract:

```json
{
  "type": "heartbeat",
  "connector_id": "connector-<opaque>",
  "instance_id": "daemon-<opaque>",
  "session_id": "session-<opaque>",
  "sent_at": "2026-04-12T10:00:15Z"
}
```

Rules:

- Relay updates `last_seen_at` on every heartbeat.
- Relay marks instance offline when heartbeat TTL expires.
- Heartbeat does not carry planner traffic.

### 3.5 Relay Request Envelope

Current implementation:

- Current internal envelope is `CommandRequest`.

Locked v2 contract:

```json
{
  "type": "request",
  "request_id": "req-<opaque>",
  "method": "GET",
  "path": "/api/v1/steps/step-123/result",
  "query": "",
  "content_type": "application/json",
  "content_encoding": "json",
  "body": null,
  "timeout_ms": 15000
}
```

Rules:

- `path` must be in the connector allowlist.
- `body` is a JSON value when `content_encoding == "json"`.
- `body` is a string when `content_encoding == "utf-8"`.
- `body` is a base64 string when `content_encoding == "base64"`.

### 3.6 Relay Response Envelope

Current implementation:

- Current internal envelope is `CommandResponse`.

Locked v2 contract:

```json
{
  "type": "response",
  "request_id": "req-<opaque>",
  "status_code": 200,
  "content_type": "application/json",
  "content_encoding": "json",
  "body": {},
  "error": ""
}
```

Rules:

- `error` is empty on success.
- Relay must not invent success when connector timeout/cancellation is not confirmed.
- Non-JSON bodies must set `content_encoding` accordingly.

### 3.7 Instance Descriptor

Current implementation:

- Relay currently returns raw `InstanceRecord` rows from storage.

Locked v2 contract:

```json
{
  "instance_id": "daemon-<opaque>",
  "connector_id": "connector-<opaque>",
  "label": "string",
  "version": "string",
  "repo_root": "/abs/path",
  "state_dir": "/abs/path/.codencer/state",
  "workspace_root": "/abs/path/.codencer/workspaces",
  "host": "127.0.0.1",
  "port": 8085,
  "base_url": "http://127.0.0.1:8085",
  "execution_mode": "string",
  "pid": 12345,
  "started_at": "2026-04-12T10:00:00Z",
  "online": true,
  "last_seen_at": "2026-04-12T10:00:15Z",
  "compatibility": {
    "tier": 1,
    "adapters": [],
    "environment": {
      "os": "linux",
      "vscode_detected": false
    }
  }
}
```

Rules:

- Planner-facing instance discovery returns this shape, not raw relay storage rows.
- `online` is derived from heartbeat/session state, not assumed from row presence.

### 3.8 Planner `start_run` Request

Current implementation:

- Local daemon already accepts this shape on `POST /api/v1/runs`.

Locked v2 contract:

```json
{
  "id": "run-optional",
  "project_id": "default-project",
  "conversation_id": "string",
  "planner_id": "string",
  "executor_id": "string"
}
```

Rules:

- `id` is optional.
- Relay passes this through unchanged to the local daemon.

### 3.9 Planner `submit_task` Request

Current implementation:

- Local daemon already accepts `domain.TaskSpec` on `POST /api/v1/runs/{run_id}/steps`.
- `schemas/task.schema.json` is close to code truth and remains the task payload source of truth.

Locked v2 contract:

```json
{
  "version": "v1",
  "project_id": "string",
  "run_id": "run-optional",
  "phase_id": "phase-optional",
  "step_id": "step-optional",
  "title": "string",
  "goal": "string",
  "context": {
    "summary": "string"
  },
  "constraints": ["string"],
  "allowed_paths": ["string"],
  "forbidden_paths": ["string"],
  "validations": [
    {
      "name": "string",
      "command": "string"
    }
  ],
  "acceptance": ["string"],
  "stop_conditions": ["string"],
  "policy_bundle": "string",
  "adapter_profile": "string",
  "timeout_seconds": 300,
  "is_simulation": false
}
```

Rules:

- Relay must not rewrite planner intent fields except to fill omitted `run_id`, `phase_id`, and `step_id` in the daemon-compatible way.

### 3.10 `wait_step` Request/Response

Current implementation:

- Missing as a relay and MCP contract.

Locked v2 contract:

Request:

```json
{
  "interval_ms": 1000,
  "timeout_ms": 300000,
  "include_result": true
}
```

Response:

```json
{
  "step_id": "step-<opaque>",
  "state": "completed",
  "terminal": true,
  "timed_out": false,
  "step": {},
  "result": {}
}
```

Rules:

- Relay implements this by polling existing step/result surfaces.
- No new daemon wait endpoint is required.
- `result` is omitted when `include_result == false` or the step is not terminal.

### 3.11 Artifact Content Response

Current implementation:

- Local daemon and relay currently return raw proxied content for `/artifacts/{id}/content`.

Locked v2 contract:

```json
{
  "artifact_id": "artifact-<opaque>",
  "name": "stdout.log",
  "type": "log",
  "mime_type": "text/plain",
  "encoding": "utf-8",
  "size": 1234,
  "hash": "sha256:<hex>",
  "content": "string-or-base64"
}
```

Rules:

- Planner-facing relay API and relay-side MCP return this JSON shape.
- Local daemon may continue to expose raw bytes on `/api/v1/artifacts/{id}/content`.
- To support this cleanly, daemon should expose artifact metadata by ID or relay must persist enough metadata from list responses.

### 3.12 Gate Action Request/Response

Current implementation:

- Local daemon accepts `POST /api/v1/gates/{id}` with `{"action":"approve"}` or `{"action":"reject"}` and returns empty `200`.

Locked v2 contract:

Request:

```json
{
  "reason": "string"
}
```

Response:

```json
{
  "gate_id": "gate-<opaque>",
  "run_id": "run-<opaque>",
  "step_id": "step-<opaque>",
  "state": "approved",
  "resolved_at": "2026-04-12T10:00:00Z"
}
```

Rules:

- Path encodes the action:
  - `POST /api/v2/gates/{gate_id}/approve`
  - `POST /api/v2/gates/{gate_id}/reject`
- Relay must not return a synthetic success body without confirming the resulting gate state.

## 4. Exact API List

### 4.1 Planner-Facing Relay HTTP API

Lock this list:

| Method | Path | Contract |
| --- | --- | --- |
| `GET` | `/api/v2/instances` | Returns `[]InstanceDescriptor`. |
| `GET` | `/api/v2/instances/{instance_id}` | Returns one `InstanceDescriptor`. |
| `POST` | `/api/v2/instances/{instance_id}/runs` | `start_run` request. |
| `GET` | `/api/v2/instances/{instance_id}/runs` | Returns runs. |
| `GET` | `/api/v2/instances/{instance_id}/runs/{run_id}` | Returns run. |
| `POST` | `/api/v2/instances/{instance_id}/runs/{run_id}/steps` | `submit_task` request. |
| `POST` | `/api/v2/instances/{instance_id}/runs/{run_id}/abort` | `abort_run` request. |
| `GET` | `/api/v2/instances/{instance_id}/runs/{run_id}/gates` | Returns gates for run. |
| `GET` | `/api/v2/steps/{step_id}` | Returns step. |
| `GET` | `/api/v2/steps/{step_id}/result` | Returns result. |
| `GET` | `/api/v2/steps/{step_id}/validations` | Returns validations. |
| `GET` | `/api/v2/steps/{step_id}/artifacts` | Returns artifacts. |
| `POST` | `/api/v2/steps/{step_id}/wait` | `wait_step` request/response. |
| `POST` | `/api/v2/steps/{step_id}/retry` | `retry_step` request. |
| `GET` | `/api/v2/artifacts/{artifact_id}/content` | `artifact content response`. |
| `POST` | `/api/v2/gates/{gate_id}/approve` | `gate action response`. |
| `POST` | `/api/v2/gates/{gate_id}/reject` | `gate action response`. |

### 4.2 Connector-Facing Relay API

Lock this list:

| Method | Path | Contract |
| --- | --- | --- |
| `POST` | `/api/v2/connectors/enroll` | Connector enrollment. |
| `POST` | `/api/v2/connectors/challenge` | Connector challenge. |
| `GET` | `/ws/connectors` | Websocket upgrade. First message is `hello`. |

### 4.3 Relay-Side MCP Tool List

Lock this list:

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

Rules:

- No raw shell.
- No arbitrary filesystem access.
- No direct connector/session management tools for planners.

### 4.4 Local Daemon Additions Required

Required additions to support the locked relay contract cleanly:

| Addition | Why |
| --- | --- |
| Allow remote abort passthrough by permitting `PATCH /api/v1/runs/{id}` in connector allowlist and relay route table. | Remote cancellation is otherwise missing. |
| `GET /api/v1/gates/{id}` or equivalent gate action response body. | Relay cannot return a truthful `gate action response` without reading the updated gate. |
| `GET /api/v1/artifacts/{id}` or equivalent metadata source by artifact ID. | Relay cannot build the locked JSON artifact-content response from content bytes alone. |

Not required:

- No daemon-side `wait_step` endpoint. Relay can poll current step/result surfaces.

## 5. Exact Dependency Graph

### 5.1 Must Land Before Connector

- Stable daemon `InstanceInfo.ID`.
- Runtime-derived `/api/v1/compatibility`.
- Honest abort semantics in `RunService`.
- Repo-root correctness in run/recovery paths.
- Service-backed artifact/log retrieval.
- Result envelope normalization.

Reason:

- Connector identity, instance discovery, and remote truthfulness depend on these existing daemon guarantees.

### 5.2 Must Land Before Relay

- Connector config persistence.
- Connector enrollment.
- Connector hello and heartbeat contracts.
- Connector allowlist for all planner-approved proxy paths.
- Stable instance descriptor shape.

Reason:

- Relay cannot truthfully list or route instances until connector registration and presence are stable.

### 5.3 Must Land Before MCP

- Planner-facing relay HTTP API must be fixed first.
- `wait_step`, `abort_run`, artifact-content, and gate-action relay responses must be stable first.
- Resource routing must not depend on accidental observation of unrelated responses.

Reason:

- Relay-side MCP should be a thin mapping over stable relay APIs, not a second place where semantics are invented.

### 5.4 Overlap and Conflict Areas

| Area | Conflict |
| --- | --- |
| `internal/app/routes.go` vs connector allowlist | Adding a daemon route is insufficient if connector still blocks it. |
| `domain.InstanceInfo` / compatibility types vs relay storage | Relay persistence and planner-facing discovery must stay aligned with daemon identity shape. |
| Gate/action contracts | Relay cannot promise gate response bodies until daemon can supply or relay can reconstruct gate state. |
| Artifact-content contracts | Relay JSON response needs artifact metadata source; current raw passthrough is insufficient. |
| MCP tool naming | Local `orchestrator.*` and relay `relay.*` tool names are different surfaces. Do not merge them implicitly. |

## 6. Acceptance Criteria Per Phase

### Phase 0: Core Hardening

- Daemon repo-root behavior is independent of process cwd.
- `PATCH /api/v1/runs/{id}` is honest about cancellation.
- Run, step, and gate terminal state reconciliation is stable.
- `/api/v1/instance` returns stable `id`.
- `/api/v1/compatibility` reflects runtime truth, not hardcoded claims.
- `/api/v1/artifacts/{id}/content` and `/api/v1/steps/{id}/logs` are service-backed.
- `schemas/result.schema.json` matches actual result state surface.

### Phase 1: Connector

- Connector persists enrollment config locally.
- Connector discovers local daemon identity via `/api/v1/instance`.
- Connector fetches runtime compatibility via `/api/v1/compatibility`.
- Connector authenticates to relay with the locked enrollment/challenge/hello contract.
- Connector heartbeat updates relay presence.
- Connector only proxies allowlisted daemon operations.
- Connector supports remote abort passthrough.

### Phase 2: Relay

- Relay persists connectors, instance descriptors, routes, and audit events in sqlite.
- Relay exposes the locked planner-facing HTTP API.
- Relay lists normalized instance descriptors with truthful online/offline state.
- Relay supports run creation, step submission, wait, artifact fetch, gate action, and abort.
- Relay routing does not require accidental prior observation to resolve core resources.

### Phase 3: Relay-Side MCP

- Relay-side MCP is a thin mapping over the planner-facing relay HTTP API.
- Tool list matches Section 4.3 exactly.
- No raw shell or arbitrary file access is exposed.
- Error handling preserves upstream truth and does not synthesize completion.

### Phase 4: Integration and Docs

- End-to-end tests cover planner -> relay -> connector -> local daemon -> adapter -> result/artifact/gate paths.
- Self-host docs match the locked contracts in this file.
- Windows/WSL and Antigravity topology docs match the actual supported path.
- Docs do not claim challenge, heartbeat, abort, or wait support until those contracts are actually implemented.

## 7. Recommended Next Implementation Order

1. Finish connector/relay auth and presence contracts:
   - challenge endpoint
   - hello proof
   - heartbeat
   - normalized instance descriptor
2. Add remote cancellation and wait:
   - connector allowlist for run abort
   - relay `PATCH /runs/{id}`
   - relay `POST /steps/{id}/wait`
3. Add resource-specific truth surfaces needed by relay:
   - local `GET /api/v1/gates/{id}` or equivalent
   - local `GET /api/v1/artifacts/{id}` or equivalent
4. Normalize planner-facing relay responses:
   - JSON artifact-content response
   - JSON gate-action response
5. Only then expand relay-side MCP to the locked tool list.

## 8. Unresolved Risks

- Relay routing for `step`, `artifact`, and `gate` IDs now probes authorized online shared instances when stored route hints are missing, but still fails closed when no match is online or multiple matches exist.
- Connector presence now uses signed challenge/response plus heartbeat-driven session state, but relay status is still alpha-grade operational metadata rather than enterprise fleet management.
- Gate action responses can be routed directly through the local gate read surface, but richer planner-facing gate summaries are still lightweight.
- Artifact lookup now has a local metadata-by-ID surface, but artifact transfer remains intentionally bounded and not designed for bulk binary delivery.
- Local and relay MCP shims are still JSON-RPC-like compatibility layers, not a standard MCP server transport.
