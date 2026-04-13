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

Status path:
- `.codencer/connector/status.json`

The connector persists:
- relay URL and websocket metadata
- connector Ed25519 keypair
- `connector_id`
- `machine_id`
- explicit shared-instance config
- the last local session snapshot in `status.json`

## Commands

The connector CLI now exposes the full local operator surface:

```bash
./bin/codencer-connectord enroll ...
./bin/codencer-connectord run ...
./bin/codencer-connectord status [--json]
./bin/codencer-connectord list [--json]
./bin/codencer-connectord share --instance-id <id>
./bin/codencer-connectord share --daemon-url http://127.0.0.1:8085
./bin/codencer-connectord unshare --instance-id <id>
./bin/codencer-connectord config [--json] [--show-secrets]
```

Command semantics:
- `status` reads the local status snapshot. Plain text is richer and includes configured shared/unshared instances. `--json` still prints the raw status file for machine consumers.
- `list` shows every configured connector instance, including `share: false` entries.
- `share` upserts an allowlist entry and sets `share=true`. When `--daemon-url` is provided, the connector will try to enrich the entry from the daemon’s `/api/v1/instance` response.
- `unshare` keeps the entry but flips `share=false`. It does not delete history from the config.
- `config` prints the persisted config safely by default. `private_key` is redacted unless `--show-secrets` is explicitly passed. `--json` is available for machine-readable output.

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

The plain-text `status` view additionally shows:
- currently shared instance IDs from the latest live session snapshot
- configured shared and unshared counts
- each configured instance selector line from the local allowlist

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

That enrollment seed is only a starting point. Use `share`, `unshare`, and `list` for day-to-day instance management after the connector is enrolled.

## Shared Instances

The connector does not expose every discovered repo by default.

Sharing rules:
- discovery roots can discover manifests
- discovery alone does not share them
- the connector config is the allowlist
- only entries with `share: true` are advertised to the relay
- `share: false` entries remain in the config so operators can see what has been intentionally withheld

Each shared instance entry can identify the local daemon by one or more of:
- `instance_id`
- `daemon_url`
- `manifest_path`

Examples:

```bash
./bin/codencer-connectord share \
  --config .codencer/connector/config.json \
  --daemon-url http://127.0.0.1:8086

./bin/codencer-connectord unshare \
  --config .codencer/connector/config.json \
  --instance-id inst_repo_b

./bin/codencer-connectord list \
  --config .codencer/connector/config.json
```

## Session Model

At runtime the connector:
1. fetches a relay challenge
2. signs the challenge with its local private key
3. opens an outbound websocket session
4. advertises only shared instances
5. sends heartbeats and re-advertises after reconnect

No inbound listener is required for normal use.

Reconnect behavior is deliberately simple:
- failures back off exponentially from a short base delay up to a capped delay
- a successful connection resets the backoff window
- status snapshots keep the last error and latest heartbeat timestamps

## Allowed Proxy Surface

The connector only proxies the narrow local Codencer API surface:
- instance read
- run create/list/read
- run patch operations such as abort
- run gate listing
- step submit
- step read
- step result
- step validations
- step artifact listing
- step logs
- step wait
- step retry
- gate read
- gate approve/reject
- artifact read
- artifact content read

The relay evidence path now works end to end for:
- `GET /api/v1/steps/{id}/validations`
- `GET /api/v1/steps/{id}/artifacts`
- `GET /api/v1/steps/{id}/logs`

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

Relay-side disable and revocation are controlled by the relay control plane. The connector will report disconnect and auth failures honestly in `status.json` if the relay stops accepting the connector.
