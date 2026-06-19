# Self-Host Relay / Runtime Reference

Codencer v2 supports a self-hostable remote planner path without moving execution off the local machine.

If you want cloud tenancy, cloud-scoped runtime control, or provider installations instead of the raw relay/runtime path, start with [CLOUD_SELF_HOST.md](CLOUD_SELF_HOST.md).

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

Project-aware Relay/MCP is the preferred Sprint 3 remote planner surface. Instance routes remain for compatibility, but new planner integrations should list and target shared projects.

Sprint 4 adds a local runtime supervisor for the daemon, relay, and connector. It can render and manage macOS LaunchAgents or Linux systemd user units, run watchdog checks, and perform conservative recovery. See [runtime-supervisor.md](runtime-supervisor.md) for service install/start/status/logs, WSL fallback behavior, and recovery boundaries.

## Public Surfaces

- Local daemon API: `/api/v1`
- Local daemon compatibility/admin MCP surface: `/mcp/call`
- Relay planner API: `/api/v2`
- Relay MCP: `/mcp`
- Relay MCP compatibility path: `/mcp/call`
- Relay connector websocket: `/ws/connectors`
- Relay project API: `/api/v2/projects`

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
  "allowed_origins": ["http://127.0.0.1:8090"],
  "public_base_url": "https://relay.example.com",
  "oauth_authorization_servers": ["https://auth.example.com"],
  "oauth_scopes_supported": ["projects:read", "projects:write", "reports:read", "instances:read", "runs:read", "runs:write", "steps:read", "steps:write", "artifacts:read", "gates:read", "gates:write"],
  "oauth_resource_documentation": "https://docs.example.com/codencer-relay-mcp"
}
```

The OAuth fields are only needed for product-facing remote MCP deployments. Codencer remains the resource server and continues to validate bearer tokens. Sprint 7 also includes a minimal single-user OAuth dev issuer for self-host ChatGPT testing; production IAM can still use an operator-owned OAuth front door.

For project-first planners, include `projects:read`, `projects:write`, and `reports:read` in scoped token policies. Planner token entries can also include `project_ids` to restrict access to specific shared projects.

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

With Sprint 4 service supervision, use the same config path through the local config and inspect the generated unit before installing:

```bash
./bin/codencer service render relay --format launchd
./bin/codencer service render relay --format systemd
./bin/codencer service install relay --dry-run --json
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
./bin/codencer connector enroll \
  --relay-url <relay> \
  --daemon-url <local-daemon> \
  --enrollment-token <token> \
  --config "$CODENCER_HOME/runtime/connector/config.json" \
  --json
```

Use `codencer-connectord enroll` with the same values only as the low-level fallback.

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
./bin/codencer-connectord share --instance-id <instance-id>
./bin/codencer-connectord unshare --instance-id <instance-id>
./bin/codencer-connectord config
```

`unshare` marks an instance as `share=false` and keeps the record in local config, so operators can see both known-shared and known-unshared repos.

`share --instance-id` is only valid when discovery or existing connector metadata can resolve that id back to a healthy local daemon. `share --daemon-url` is the self-sufficient operator path.

You can also inspect the relay-side view of shared instances with:

```bash
./bin/codencer-relayd connectors --config .codencer/relay/config.json
./bin/codencer-relayd audit --config .codencer/relay/config.json --limit 20
```

### 6.1 Share projects

Project sharing uses the user-level registry created by `codencer init`.

```bash
export CODENCER_HOME="$HOME/.codencer"
./bin/codencer project init \
  --id codencer \
  --repo /path/to/repo \
  --adapter codex \
  --profile codex-workspace \
  --daemon-url http://127.0.0.1:8085 \
  --share-to-relay

./bin/codencer project share codencer --daemon-url http://127.0.0.1:8085
./bin/codencer project unshare codencer
```

Set `codencer_home` in `.codencer/connector/config.json`, or enroll with `--codencer-home`, when the connector should read a non-default registry. The connector advertises only `shared_to_relay:true` projects whose daemon is reachable. If a project pins `relay_instance_id`, the live daemon must report that same instance id.

### 7. Run the connector

```bash
./bin/codencer-connectord run
```

The runtime supervisor can manage the connector only after `connector_config_path` is set in `$CODENCER_HOME/config.json`. Missing connector config is reported as `not_configured`, not silently generated.

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

For the frozen planner/client compatibility matrix, generic HTTP/MCP examples, and client-specific packaging notes, see [mcp/integrations.md](mcp/integrations.md) and [mcp/relay_tools.md](mcp/relay_tools.md).

Current MCP transport posture:
- canonical endpoint: `/mcp`
- compatibility alias: `/mcp/call`
- OAuth metadata: `/.well-known/oauth-protected-resource/mcp`
- POST JSON-RPC is supported for straightforward planner integrations
- Streamable HTTP compatibility is implemented on `/mcp` with `GET`, `POST`, and `DELETE`, `MCP-Protocol-Version`, and `MCP-Session-Id`
- unauthenticated MCP calls return `WWW-Authenticate` with `resource_metadata`
- the current relay is still request/response-first and does not emit long-lived unsolicited server notifications

### 9. Start work and inspect evidence

Typical remote sequence:
1. list projects
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

Project HTTP examples:

```bash
curl -fsS -H "Authorization: Bearer <planner-token>" \
  https://relay.example.com/api/v2/projects

curl -fsS \
  -H "Authorization: Bearer <planner-token>" \
  -H "Content-Type: application/json" \
  -d '{"goal":"Verify the project relay path","profile":"fake-success","wait":true}' \
  https://relay.example.com/api/v2/projects/codencer/submit
```

Project MCP tools include `codencer.list_projects`, `codencer.get_project`, `codencer.start_project_run`, `codencer.submit_project_task`, `codencer.submit_project_task_and_wait`, `codencer.run_project_manifest`, `codencer.get_execution_report`, `codencer.get_project_blocker`, `codencer.get_blocker`, and project step evidence tools.

Sprint 5 adds an opt-in live Relay/MCP proof command:

```bash
CODENCER_LIVE_RELAY_MCP=1 ./bin/codencer live relay-mcp --json --bin-dir ./bin --repo .
```

It starts temporary daemon, relay, and connector processes, shares a disposable fake project, calls project-aware MCP tools, verifies reports/evidence, and records a live matrix report. Use `codencer.get_run_report` as a read-only alias for `codencer.get_execution_report` when a client prefers shorter report tool names.

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
- project list/read/run/submit/run-plan/report/evidence through `/codencer/v1/projects/...` connector commands

The relay and connector do not expose:
- raw shell
- arbitrary filesystem browsing
- generic network tunneling

## Practical Smoke Path

Once the daemon and relay are already running, use the repo smoke helper for the happy path:

```bash
PLANNER_TOKEN=<planner-token> make self-host-smoke
make verify-local-relay-mcp
```

The smoke flow:
1. reads the local daemon instance identity
2. creates a one-time relay enrollment token through `codencer-relayd enrollment-token create`
3. enrolls and runs a temporary connector
4. waits for instance and project advertisement
5. starts a run through the relay
6. submits a real `TaskSpec`-compatible task
7. waits for the step
8. fetches result, validations, logs, gates, and artifacts

Optional smoke scenario coverage:

Default proof from `make self-host-smoke`:
- connector enrollment and websocket session establishment
- relay instance visibility for the enrolled daemon
- run create, task submit, wait, result, validations, logs, gates, and artifact fetch over relay HTTP
- relay audit visibility when `audit` is enabled

Optional proof paths:

```bash
PLANNER_TOKEN=<planner-token> SMOKE_SCENARIOS=status,audit,share-control,mcp,mcp-sdk make self-host-smoke
PLANNER_TOKEN=<planner-token> make self-host-smoke-all
make flagship-planner-smoke
```

- `share-control` now proves `unshare` removes relay visibility and blocks routing, then `share --instance-id` restores visibility before the main relay flow runs again.
- `mcp` proves manual relay MCP initialize, SSE stream bootstrap, compatibility POST alias use, tool calls, and session delete.
- `mcp-sdk` proves official Go SDK interoperability against relay `/mcp`; set `MCP_SDK_STEP_COUNT=2` to prove a same-run MCP phase loop.
- `multi-instance` proves one connector can advertise two local daemons and that explicit instance targeting reaches only the selected daemon.
- `phase-loop` submits a second step into the same run after reading the first result.
- `reconnect` stops and restarts the connector with the same config, then proves routing recovers by id.
- `gate-strict` expects a real gate and fails if none is produced. Use it with a daemon started with `FORCE_GATE_FOR_TESTING=1`, or run `make flagship-planner-smoke`.
- For slower real-executor proof, tune `WAIT_TIMEOUT_MS`, `MCP_SDK_WAIT_TIMEOUT_MS`, or `FLAGSHIP_PROXY_TIMEOUT_SECONDS`; `FLAGSHIP_LIVE_CODEX=1 make flagship-planner-smoke` defaults those budgets higher than the simulation smoke.

Still outside smoke proof:
- cold bootstrap of the daemon and relay themselves
- live non-simulation adapter execution unless explicitly enabled with the live Codex flagship option
- WSL/Windows/Antigravity topology behavior
- hard guarantees for gate, retry, or abort semantics beyond the statuses captured by the script

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

This is recommended operator topology, not an automated smoke proof. See [WSL / Windows / Antigravity Topology](WSL_WINDOWS_ANTIGRAVITY.md) for the trust boundaries and placement guidance.

## Default Relay vs Self-Host

- Self-host mode is implemented in this repo and uses your own relay config, sqlite state, and tokens.
- A future default or managed relay can speak the same connector session model, but self-host does not depend on that future service.

## Sprint 8 Gateway Short Path

Official client setup is Gateway-first. A self-host Relay is still the backend
transport, but ChatGPT, Claude Code, and Codex should point at Gateway:

```bash
make build
./bin/codencer setup gateway \
  --base-url https://mcp.codencer.dev \
  --mcp-url https://mcp.codencer.dev/mcp \
  --token-env CODENCER_GATEWAY_MCP_TOKEN \
  --enable-oauth-dev \
  --json
./bin/codencer gateway relay add --id personal --url https://relay.example.com --token-env CODENCER_RELAY_PERSONAL_TOKEN --json
./bin/codencer activation gateway --gateway https://mcp.codencer.dev --relay https://relay.example.com --project codencer --token-env CODENCER_GATEWAY_MCP_TOKEN --json
```

## Sprint 7 Direct Relay Short Path

The direct Relay path below is retained for advanced/direct/debug mode and
historical self-host testing. It is not the official connector path.

```bash
make build
./bin/codencer setup relay \
  --base-url https://relay.example.com \
  --mcp-url https://relay.example.com/mcp \
  --generate-planner-token \
  --json
./bin/codencer setup relay \
  --base-url https://relay.example.com \
  --mcp-url https://relay.example.com/mcp \
  --generate-planner-token \
  --enable-chatgpt-oauth-dev \
  --json
./bin/codencer setup mcp --client codex --endpoint https://relay.example.com/mcp --json
./bin/codencer setup mcp --client claude-code --endpoint https://relay.example.com/mcp --json
./bin/codencer setup mcp --client chatgpt --endpoint https://relay.example.com/mcp --json
./bin/codencer activation package --relay https://relay.example.com --project codencer --token-env CODENCER_MCP_TOKEN --json
./bin/codencer activation check --relay https://relay.example.com --project codencer --token-env CODENCER_MCP_TOKEN --check-oauth --json
```

`setup relay` writes runnable relay and connector config only when a planner token is already present or explicitly supplied/generated. Generated tokens are stored under `$CODENCER_HOME/tokens` and redacted from output. Service install/start remains explicit.

Remote project descriptors do not expose absolute local repository paths by default. The local registry and relay storage keep routing data; planner-facing project payloads use safe labels and hashes.

Use `docs/quickstart-self-host-relay.md` for the shortest operator flow, `docs/activation-vps-relay.md` and `docs/activation-local-connector.md` for Sprint 7 activation, and `docs/release-checklist.md` for release acceptance evidence.
