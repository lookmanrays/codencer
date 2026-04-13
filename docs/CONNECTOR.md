# Connector

Codencer’s connector is the outbound-only bridge between a relay and one or more local Codencer daemons. It is not a planner, not an executor, and not a second orchestration brain.

## Role

The connector is responsible for:
- persistent connector identity
- connector enrollment
- outbound authenticated websocket session to the relay
- explicit local instance sharing
- narrow proxying to the local daemon

The connector is not responsible for:
- planning
- direct local execution
- raw shell exposure
- generic tunneling

## Local Config

Default config path:
- `.codencer/connector/config.json`

The connector persists:
- relay URL and websocket metadata
- connector Ed25519 keypair
- `connector_id`
- `machine_id`
- explicit shared-instance config
- local status snapshot at `.codencer/connector/status.json`

## Operator Status

Use the local status file when you want to check connector state without contacting the relay:

```bash
./bin/codencer-connectord status --config .codencer/connector/config.json --json
```

The status file records:
- `connector_id`
- `machine_id`
- `relay_url`
- `session_state`
- `last_connect_at`
- `last_disconnect_at`
- `last_heartbeat_at`
- `last_error`
- `shared_instances`

Session states are intentionally small and honest:
- `disconnected`
- `connecting`
- `connected`
- `error`

## Enrollment

Preferred flow:

```bash
./bin/codencer-connectord enroll \
  --relay-url http://127.0.0.1:8090 \
  --daemon-url http://127.0.0.1:8085 \
  --enrollment-token <token>
```

Enrollment does two things:
- exchanges the one-time relay enrollment token for connector identity
- seeds one shared instance from the daemon URL used during enrollment

## Shared Instances

The connector does not expose every discovered repo by default.

Sharing rules:
- discovery roots can discover manifests
- discovery alone does not share them
- the connector config is the allowlist
- only entries with `share: true` are advertised

Each shared instance entry should identify the local daemon by one of:
- `instance_id`
- `daemon_url`
- `manifest_path`

## Session Model

At runtime the connector:
1. fetches a relay challenge
2. signs the challenge with its local private key
3. opens an outbound websocket session
4. advertises only shared instances
5. sends heartbeats and re-advertises after reconnect

No inbound listener is required for normal use.

## Allowed Proxy Surface

The connector only proxies the narrow local Codencer API surface:
- instance read
- run create/list/read
- run abort
- step submit/read/result/validations/artifacts/logs
- step wait
- step retry
- gate approve/reject
- artifact content read

It does not expose:
- raw shell
- arbitrary file reads
- generic network tunneling

Abort forwarding stays honest:
- the connector can forward an abort request
- it cannot guarantee a hard process kill on the local adapter side
- a remote abort is only considered successful when the daemon confirms the active step reached `cancelled`

## Placement Guidance

The default recommendation is:
- run the connector on the same side as the daemon
- keep the repo, worktrees, and artifacts on that same side
- let the relay be the remote surface

In mixed WSL/Windows setups:
- daemon and connector usually belong in WSL/Linux
- Antigravity broker and IDE may live on Windows

See [WSL / Windows / Antigravity Topology](WSL_WINDOWS_ANTIGRAVITY.md) for the practical topology.

## Reset / Revocation

To reset a connector locally:
1. stop the connector process
2. remove `.codencer/connector/config.json`
3. enroll again with a fresh enrollment token

Relay-side disable/revocation has storage foundation and can be handled by replacing or disabling the enrolled connector record on the relay.
