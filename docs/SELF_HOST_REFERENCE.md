# Self-Host Reference

Codencer v2 supports a self-hostable remote planner path without moving execution off the local machine.

## Current Topology

```text
Planner / Chat
  -> Relay planner API or relay MCP
  -> Relay daemon
  -> Connector outbound websocket
  -> Local Codencer daemon
  -> Local adapters
```

Execution still stays local. The relay is transport, auth, and audit. The connector is an outbound bridge. The daemon remains the local system of record.

## Public Surfaces

- Local daemon API: `/api/v1`
- Local daemon compatibility/admin MCP surface: `/mcp/call`
- Relay planner API: `/api/v2`
- Relay MCP: `/mcp`
- Relay MCP compatibility path: `/mcp/call`
- Relay connector websocket: `/ws/connectors`

The local daemon is not the public remote MCP server.

## Operator Flow

### 1. Start the local daemon

Run the daemon near the repo you want to serve:

```bash
./bin/orchestratord --repo-root /path/to/repo
```

Or use the existing convenience flow:

```bash
make start
```

### 2. Inspect local instance identity

Verify the daemon’s stable identity and local manifest-backed metadata:

```bash
./bin/orchestratorctl instance --json
```

Or inspect the daemon directly:

```bash
curl http://127.0.0.1:8085/api/v1/instance
```

The daemon writes a repo-local manifest under `.codencer/instance.json`.

### 3. Start the relay

Run the relay with:
- `db_path`
- `planner_token` or `planner_tokens`

The relay is the public remote control plane. Do not expose the daemon directly.

### 4. Create a one-time enrollment token

```bash
curl -X POST <relay>/api/v2/connectors/enrollment-tokens \
  -H 'Authorization: Bearer <planner-token>' \
  -H 'Content-Type: application/json' \
  -d '{"label":"local-dev","expires_in_seconds":600}'
```

### 5. Enroll the connector

```bash
./bin/codencer-connectord enroll \
  --relay-url <relay> \
  --daemon-url <local-daemon> \
  --enrollment-token <token>
```

The connector persists:
- `relay_url`
- `connector_id`
- `machine_id`
- `private_key`
- `instances[]` allowlist entries

Legacy bootstrap compatibility:
- `enrollment_secret` is still accepted if configured on the relay
- new self-host setups should prefer one-time enrollment tokens

### 6. Verify instance sharing

Enrollment seeds one shared instance from the daemon URL you enrolled against.

Important rules:
- discovery roots do not auto-share repos
- connector config is the allowlist
- only `share: true` instances are advertised

Inspect `.codencer/connector/config.json` and verify only intended instances are shared before running the connector.

### 7. Run the connector

```bash
./bin/codencer-connectord run
```

The connector opens an outbound authenticated websocket session to the relay and advertises only the explicitly shared local instances.

### 8. Connect the planner

Use either:
- relay planner API under `/api/v2`
- relay MCP at `/mcp`

The relay is the remote planner surface. The daemon-local `/mcp/call` endpoint is only a local compatibility/admin bridge.

### 9. Start work and inspect evidence

Typical remote sequence:
1. list instances
2. start run
3. submit task
4. wait for step or poll step/result
5. inspect result
6. inspect validations
7. inspect artifacts

Remote artifact access is ID-based:
- artifact content is fetched by `artifact_id`
- there is no arbitrary path browsing tool
- large binary transport is intentionally bounded

### 10. Operate the run honestly

Supported remote actions include:
- approve gate
- reject gate
- abort run
- retry step

Current limitations remain explicit:
- abort is best-effort unless the adapter actually confirms stop
- relay step/gate/artifact routing is opportunistic and may require prior observation of those IDs

## Allowed Remote Surface

The connector only proxies a narrow allowlist:
- run create/list/read
- run abort
- step submit/read/result/validations/artifacts/logs
- step retry
- step wait
- gate approve/reject
- instance read
- artifact content read

The relay and connector do not expose:
- raw shell
- arbitrary filesystem browsing
- generic network tunneling

## WSL / Windows / Antigravity

The practical default is:
- daemon, connector, repos, worktrees, and artifacts in WSL/Linux
- Antigravity broker and IDE on Windows when needed
- relay wherever the operator wants to host the remote control plane

See [WSL / Windows / Antigravity Topology](WSL_WINDOWS_ANTIGRAVITY.md) for the trust boundaries and placement guidance.

## Default Relay vs Self-Host

- Self-host mode is implemented in this repo and uses your own relay config, sqlite state, and tokens.
- A future default or managed relay can speak the same connector session model, but self-host does not depend on that future service.
