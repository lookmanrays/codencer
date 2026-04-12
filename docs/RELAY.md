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

Run either relay binary:

- `./bin/relayd`
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
- `enrollment_secret` can still be used directly if configured
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
- relay step/gate/artifact routing is opportunistic and can miss unknown IDs until they were previously observed
- artifact transfer is bounded and is not intended for bulk binary transport
- abort semantics remain best-effort unless the local adapter confirms stop

## Audit Trail

The relay appends audit events for:
- planner enrollment-token creation
- connector enrollment
- connector session establishment
- planner control calls

Each audit event records actor, action, target, outcome, and timestamp.
