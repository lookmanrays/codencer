# Relay

The Codencer relay is a narrow self-hostable control plane. It is not a planner, not an executor, and not a remote shell.

## Role

The relay does three things:
- authenticates planners and connectors
- routes planner calls to explicitly shared local instances
- records audit events for remote control actions

The relay does not:
- plan work
- execute local code
- expose a generic tunnel
- expose raw shell or arbitrary filesystem tools

## Public Surfaces

- planner HTTP API: `/api/v2/...`
- relay MCP: `/mcp`
- relay MCP compatibility path: `/mcp/call`
- connector websocket: `/ws/connectors`

The relay is the public remote control surface.

The local daemon is not intended to be exposed directly.

## Startup

Run the canonical relay binary:

- `./bin/codencer-relayd`
- `./bin/codencer-relayd serve --config .codencer/relay/config.json`

The relay config must include at least:
- `db_path`
- `planner_token` or `planner_tokens`

Useful config keys for practical self-host use:
- `proxy_timeout_seconds`
- `allowed_origins`
- `heartbeat_interval_seconds`
- `session_ttl_seconds`
- `challenge_ttl_seconds`

Optional bootstrap compatibility:
- `enrollment_secret`

## Planner Auth

Planner callers authenticate with bearer tokens.

Supported config shapes:
- `planner_token`: one full-scope token for small self-host setups
- `planner_tokens[]`: named tokens with `token`, `scopes`, and optional `instance_ids`

Current auth model is intentionally small:
- static token config
- explicit scopes
- optional instance scoping
- suitable for self-host alpha use

It is not enterprise IAM.

Operator helper:

```bash
./bin/codencer-relayd planner-token create \
  --config .codencer/relay/config.json \
  --write-config \
  --name operator \
  --scope '*'
```

## Connector Auth

Connector auth uses:
- one-time enrollment token exchange
- connector Ed25519 keypair
- signed challenge/response
- outbound websocket session
- heartbeat-driven presence

Legacy bootstrap compatibility:
- `enrollment_secret` can still be used directly if configured, but it should be treated as bootstrap-only fallback
- new deployments should prefer one-time enrollment tokens

## Persisted State

The relay persists in SQLite:
- connector records
- one-time enrollment tokens
- connector challenge state
- advertised instance descriptors for the connector's current shared set
- resource routing hints
- audit events

Advertise truth model:
- each connector `advertise` payload is treated as the authoritative current shared-instance set for that connector
- newly advertised instances are upserted
- previously connector-owned instances that are absent from the new advertise payload are pruned
- pruning also deletes cached resource-route hints for that instance so unshared instances stop appearing in `/api/v2/instances` and stop being routable

## Planner API

Planner-facing HTTP routes live under `/api/v2`.

Current routes include:
- `GET /api/v2/status`
- `GET /api/v2/connectors`
- `GET /api/v2/connectors/{connector_id}`
- `POST /api/v2/connectors/{connector_id}/disable`
- `POST /api/v2/connectors/{connector_id}/enable`
- `GET /api/v2/audit?limit=N`
- `GET /api/v2/instances`
- `GET /api/v2/instances/{instance_id}`
- `GET|POST /api/v2/instances/{instance_id}/runs`
- `GET /api/v2/instances/{instance_id}/runs/{run_id}`
- `GET /api/v2/instances/{instance_id}/runs/{run_id}/gates`
- `POST /api/v2/instances/{instance_id}/runs/{run_id}/steps`
- `POST /api/v2/instances/{instance_id}/runs/{run_id}/abort`
- `GET /api/v2/steps/{step_id}`
- `POST /api/v2/steps/{step_id}/wait`
- `POST /api/v2/steps/{step_id}/retry`
- `GET /api/v2/steps/{step_id}/result`
- `GET /api/v2/steps/{step_id}/logs`
- `GET /api/v2/steps/{step_id}/artifacts`
- `GET /api/v2/steps/{step_id}/validations`
- `GET /api/v2/artifacts/{artifact_id}/content`
- `POST /api/v2/gates/{gate_id}/approve`
- `POST /api/v2/gates/{gate_id}/reject`

These routes stay narrow and instance-oriented.

Operational notes:
- `/api/v2/status` returns relay version, start time, connector and instance counts, auth mode, and whether bootstrap `enrollment_secret` mode is enabled
- `/api/v2/connectors` returns connector identity, online/offline state, last seen, disabled state, and shared instance ids
- connector enable/disable mutations are explicit planner-admin actions and are audited
- `/api/v2/audit` returns recent persisted audit events newest first, default limit `100`, max `1000`
- offline connectors remain visible through `/api/v2/connectors`, but stale offline sessions are never used for routing
- `/api/v2/steps/{step_id}/wait` uses planner-provided `timeout_ms` when present, capped by `proxy_timeout_seconds`

## MCP Surface

The relay is also the public MCP surface.

Supported MCP transport behavior:
- `POST /mcp`
- `GET /mcp`
- `DELETE /mcp`
- `/mcp/call` remains as a compatibility alias for POST callers

Supported MCP methods:
- `initialize`
- `notifications/initialized`
- `tools/list`
- `tools/call`

Supported tools are the `codencer.*` relay tools documented in [mcp/relay_tools.md](mcp/relay_tools.md).

Protocol notes:
- the relay negotiates and returns `MCP-Protocol-Version`
- the relay can return `MCP-Session-Id` on `initialize`
- the relay enforces `allowed_origins` for browser-style MCP callers when configured
- the current relay remains request/response-first; it does not rely on unsolicited long-lived server notifications for planner functionality

The local daemon’s `/mcp/call` surface is separate and should be treated as local compatibility/admin tooling, not as the remote public MCP endpoint.

## Security Boundaries

The remote path is intentionally narrow:
- planner talks to relay
- relay talks only to authenticated connectors
- connector proxies only an allowlisted daemon API
- daemon executes locally through adapters

The relay does not widen privileges beyond planner token scopes or connector sharing decisions.

## Known Limitations

Current honest limitations:
- planner auth is static-token based
- relay resolves unknown `step`, `artifact`, and `gate` ids by probing only authorized online shared instances, then persists route hints; lookups still fail closed when no online match exists or multiple instances match
- artifact transfer is bounded and is not intended for bulk binary transport
- abort semantics remain best-effort unless the local adapter confirms stop; planner callers only get a successful abort when the daemon actually reaches `cancelled`
- MCP compatibility is intentionally tool-focused; the public planner contract is the explicit `codencer.*` tool set rather than a broader autonomous control surface

## Audit Trail

The relay appends audit events for:
- planner enrollment-token creation
- connector enrollment
- connector session establishment
- planner control calls

Each audit event records actor, action, target, outcome, and timestamp.

## Admin Helpers

```bash
./bin/codencer-relayd status --config .codencer/relay/config.json
./bin/codencer-relayd connectors --config .codencer/relay/config.json
./bin/codencer-relayd instances --config .codencer/relay/config.json
./bin/codencer-relayd audit --config .codencer/relay/config.json --limit 50
./bin/codencer-relayd enrollment-token create --config .codencer/relay/config.json --label local-dev --json
./bin/codencer-relayd connector disable <connector-id> --config .codencer/relay/config.json
./bin/codencer-relayd connector enable <connector-id> --config .codencer/relay/config.json
```
