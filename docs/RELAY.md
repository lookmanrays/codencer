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

The relay config must include at least:
- `db_path`
- `planner_token` or `planner_tokens`

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
- advertised instance descriptors
- resource routing hints
- audit events

## Planner API

Planner-facing HTTP routes live under `/api/v2`.

Current routes include:
- `GET /api/v2/status`
- `GET /api/v2/connectors`
- `GET /api/v2/audit?limit=N`
- `GET /api/v2/instances`
- `GET /api/v2/instances/{instance_id}`
- `GET|POST /api/v2/instances/{instance_id}/runs`
- `GET /api/v2/instances/{instance_id}/runs/{run_id}`
- `POST /api/v2/instances/{instance_id}/runs/{run_id}/steps`
- `POST /api/v2/instances/{instance_id}/runs/{run_id}/abort`
- `GET /api/v2/steps/{step_id}`
- `POST /api/v2/steps/{step_id}/wait`
- `POST /api/v2/steps/{step_id}/retry`
- `GET /api/v2/steps/{step_id}/result`
- `GET /api/v2/steps/{step_id}/artifacts`
- `GET /api/v2/steps/{step_id}/validations`
- `GET /api/v2/artifacts/{artifact_id}/content`
- `POST /api/v2/gates/{gate_id}/approve`
- `POST /api/v2/gates/{gate_id}/reject`

These routes stay narrow and instance-oriented.

Operational notes:
- `/api/v2/status` returns relay version, start time, connector and instance counts, auth mode, and whether bootstrap `enrollment_secret` mode is enabled
- `/api/v2/connectors` returns connector identity, online/offline state, last seen, disabled state, and shared instance ids
- `/api/v2/audit` returns recent persisted audit events newest first, default limit `100`, max `1000`

## MCP Surface

The relay is also the public MCP surface.

Supported MCP methods:
- `initialize`
- `tools/list`
- `tools/call`

Supported tools are the `codencer.*` relay tools documented in [mcp/relay_tools.md](mcp/relay_tools.md).

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

## Audit Trail

The relay appends audit events for:
- planner enrollment-token creation
- connector enrollment
- connector session establishment
- planner control calls

Each audit event records actor, action, target, outcome, and timestamp.
