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

### 0. Create a relay config and planner token

The practical cold-start flow is:

```bash
mkdir -p .codencer/relay
./bin/codencer-relayd planner-token create \
  --config .codencer/relay/config.json \
  --write-config \
  --name operator \
  --scope '*'
```

That command creates or updates a local relay config file with a high-entropy static planner bearer token.

Minimal relay config example:

```json
{
  "host": "127.0.0.1",
  "port": 8090,
  "db_path": ".codencer/relay/relay.db",
  "planner_tokens": [
    {
      "name": "operator",
      "token": "<generated-by-planner-token-create>",
      "scopes": ["*"]
    }
  ],
  "proxy_timeout_seconds": 300,
  "allowed_origins": ["http://127.0.0.1:8090"]
}
```

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

Run the relay with the config you created:

```bash
./bin/codencer-relayd --config .codencer/relay/config.json
```

The relay is the public remote control plane. Do not expose the daemon directly.

Operator status/admin endpoints live on the relay too:
- `GET /api/v2/status`
- `GET /api/v2/connectors`
- `GET /api/v2/audit?limit=N`

Local helper commands are available too:

```bash
./bin/codencer-relayd status --config .codencer/relay/config.json
./bin/codencer-relayd connectors --config .codencer/relay/config.json
./bin/codencer-relayd instances --config .codencer/relay/config.json
```

### 4. Create a one-time enrollment token

```bash
./bin/codencer-relayd enrollment-token create \
  --config .codencer/relay/config.json \
  --label local-dev \
  --expires-in-seconds 600 \
  --json
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
- `status.json` session snapshot

Legacy bootstrap compatibility:
- `enrollment_secret` is still accepted if configured on the relay as a bootstrap-only fallback
- new self-host setups should prefer one-time enrollment tokens

### 6. Verify instance sharing

Enrollment seeds one shared instance from the daemon URL you enrolled against.

Important rules:
- discovery roots do not auto-share repos
- connector config is the allowlist
- only `share: true` instances are advertised

Inspect and manage the allowlist explicitly before running the connector:

```bash
./bin/codencer-connectord discover --config .codencer/connector/config.json
./bin/codencer-connectord list
./bin/codencer-connectord share --daemon-url http://127.0.0.1:8085
./bin/codencer-connectord unshare --instance-id <instance-id>
./bin/codencer-connectord config
```

`unshare` marks an instance as `share=false` and keeps the record in local config, so operators can see both known-shared and known-unshared repos.

You can also inspect the relay-side view of shared instances with:

```bash
./bin/codencer-relayd connectors --config .codencer/relay/config.json
./bin/codencer-relayd audit --config .codencer/relay/config.json --limit 20
```

### 7. Run the connector

```bash
./bin/codencer-connectord run
```

The connector opens an outbound authenticated websocket session to the relay and advertises only the explicitly shared local instances.

Check connector state locally at any time:

```bash
./bin/codencer-connectord status --json
```

### 8. Connect the planner

Use either:
- relay planner API under `/api/v2`
- relay MCP at `/mcp`

The relay is the remote planner surface. The daemon-local `/mcp/call` endpoint is only a local compatibility/admin bridge.

Current MCP transport posture:
- canonical endpoint: `/mcp`
- compatibility alias: `/mcp/call`
- POST JSON-RPC is supported for straightforward planner integrations
- Streamable HTTP compatibility is implemented on `/mcp` with `GET`, `POST`, and `DELETE`, `MCP-Protocol-Version`, and `MCP-Session-Id`
- the current relay is still request/response-first and does not emit long-lived unsolicited server notifications

### 9. Start work and inspect evidence

Typical remote sequence:
1. list instances
2. start run
3. submit task
4. wait for step or poll step/result
5. inspect result
6. inspect validations
7. inspect logs
8. inspect artifacts

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
- disable or enable a connector from the relay admin surface

Current limitations remain explicit:
- abort is best-effort unless the adapter actually confirms stop, and the caller only gets a successful abort when the active step reaches `cancelled`
- large binary artifact transfer is intentionally bounded

Current routing behavior:
- relay step/gate/artifact lookups first use stored route hints
- if a hint is missing, the relay probes only authorized online shared instances
- successful probes are persisted as route hints for later lookups
- ambiguous matches still fail closed

## Allowed Remote Surface

The connector only proxies a narrow allowlist:
- run create/list/read
- run abort
- run gate listing
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

## Practical Smoke Path

Once the daemon and relay are already running, use the repo smoke helper for the happy path:

```bash
PLANNER_TOKEN=<planner-token> make self-host-smoke
```

The smoke flow:
1. reads the local daemon instance identity
2. creates a one-time relay enrollment token through `codencer-relayd enrollment-token create`
3. enrolls and runs a temporary connector
4. waits for instance advertisement
5. starts a run through the relay
6. submits a real `TaskSpec`-compatible task
7. waits for the step
8. fetches result, validations, logs, gates, and artifacts

Optional smoke scenario coverage:

```bash
PLANNER_TOKEN=<planner-token> SMOKE_SCENARIOS=status,audit,mcp,mcp-sdk make self-host-smoke
PLANNER_TOKEN=<planner-token> make self-host-smoke-all
```

`make self-host-smoke-mcp` includes the official Go SDK proof helper, while `make self-host-smoke-all` adds the share-control and multi-instance scenarios.

If you want the standalone SDK proof path, build and run the helper directly:

```bash
make build-mcp-sdk-smoke
./bin/mcp-sdk-smoke --endpoint http://127.0.0.1:8090/mcp --token <planner-token> --instance-id <instance-id>
```

If you need the Windows-side agent-broker binary too, build it separately with:

```bash
make build-broker
```

## WSL / Windows / Antigravity

The practical default is:
- daemon, connector, repos, worktrees, and artifacts in WSL/Linux
- agent-broker and IDE on Windows when needed
- relay wherever the operator wants to host the remote control plane

See [WSL / Windows / Antigravity Topology](WSL_WINDOWS_ANTIGRAVITY.md) for the trust boundaries and placement guidance.

## Default Relay vs Self-Host

- Self-host mode is implemented in this repo and uses your own relay config, sqlite state, and tokens.
- A future default or managed relay can speak the same connector session model, but self-host does not depend on that future service.
