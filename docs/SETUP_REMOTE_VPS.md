# Remote VPS Setup

This page replaces the Wave 1 placeholder with the practical `v0.2.0-beta` Remote VPS dev-server walkthrough.

Use this page when all of the following are true:

- the VPS is your development machine
- the repo checkout, worktrees, daemon, and connector live on the VPS
- your laptop hosts the planner-side control plane as a self-host relay or self-host cloud
- your laptop is the planner client location, but it does not execute the coding work

Keep the support claim narrow:

- this is the existing self-host relay/runtime path adapted to a VPS-hosted execution machine
- relay HTTP, relay MCP, connector enrollment, and shared-instance routing are the proven Codencer surfaces
- ChatGPT-style and Claude Code-style operator wiring is packaged around the relay/cloud MCP surfaces; Claude Desktop and vendor UI/auth behavior remain outside repo proof as described in [BETA_TESTING.md](BETA_TESTING.md), [KNOWN_LIMITATIONS.md](KNOWN_LIMITATIONS.md), and [mcp/integrations.md](mcp/integrations.md)

> [!IMPORTANT]
> Use a real `git clone` on the VPS. Do not use a ZIP download. Codencer depends on git worktrees for isolated attempts.

> [!IMPORTANT]
> Do not expose the VPS daemon directly. The public planner surface is the relay `/mcp` or cloud `/api/cloud/v1/mcp`, not the daemon-local `/mcp/call`.

## 1. Topology

Primary topology for this page:

```text
Laptop
  Planner client (ChatGPT-style / Claude Desktop-style / curl / SDK)
    -> relay MCP / planner HTTP on the laptop, or cloud MCP / runtime HTTP on the laptop
    -> operator-controlled HTTPS front door if the VPS or planner product needs a reachable URL

VPS
  codencer-connectord
    -> outbound authenticated websocket to the laptop relay or cloud relay bridge
  orchestratord
    -> local adapters
    -> repo checkout, worktrees, artifacts, validations
```

What stays true in this topology:

- the VPS is the execution side
- the laptop is the planning side
- the laptop does not run the daemon, worktrees, or adapters for this repo
- the connector is an outbound bridge from the VPS to the laptop control plane
- the laptop does not need inbound reachability to the VPS except normal SSH admin access

## 2. Why This Layout

This split is useful for two boring, good reasons:

- isolation: the repo, daemon, worktrees, executor binaries, and artifacts stay on one disposable Linux machine instead of leaking into the laptop environment
- reproducibility: the VPS gives you a stable Ubuntu or Debian runtime for builds, validations, and adapter binaries, while the planner surface on the laptop stays thin and operator-oriented

This layout also keeps the service boundary honest:

- the orchestrator remains the control plane for execution state
- the relay remains the remote planner surface
- the connector remains an outbound transport bridge
- the laptop planner is not turned into an executor

## 3. Prerequisites

### 3.1 VPS

Recommended VPS baseline:

- Ubuntu `22.04+` or Debian `12+`
- `sshd`
- `ufw` or an equivalent host firewall
- Git
- Go `1.25.0+`
- `cc` or `gcc`
- `curl`
- `jq` or Python 3
- a `t3.small`-equivalent or better machine for a comfortable dev-server baseline

One straightforward Ubuntu or Debian package setup is:

```bash
sudo apt update
sudo apt install -y git golang-go build-essential curl jq openssh-server ufw
```

Then verify the toolchain:

```bash
git --version
go version
cc --version
curl --version
jq --version
go env CGO_ENABLED CC
```

Expected shape:

```text
go version go1.25.x linux/amd64
...
1
gcc
```

Notes:

- do not force `CGO_ENABLED=0`; the repo build uses the CGO SQLite driver
- keep the daemon and connector on the same Linux side as the repo checkout
- only SSH should be inbound to the VPS for this walkthrough

### 3.2 Laptop

The laptop needs the planner-side control plane:

- self-host relay on the laptop, or
- self-host cloud on the laptop if you want the cloud control plane instead of raw relay

If you are running the raw relay on the laptop from a repo checkout, the laptop also needs:

- Go `1.25.0+`
- `curl`
- `jq` or Python 3

If the VPS or planner product needs a reachable URL, put an operator-controlled HTTPS front door in front of the laptop relay or cloud. The checked-in relay binary is plain HTTP by itself; HTTPS termination is your operator responsibility when you want outbound `https://...` and `wss://...` from the VPS.

## 4. Walkthrough

This section uses the laptop-hosted relay as the main path because that is the most direct match for the requested topology. If you want laptop-hosted cloud tenancy instead, keep the VPS daemon and connector placement the same and use [CLOUD_SELF_HOST.md](CLOUD_SELF_HOST.md) for the cloud bootstrap.

### 4.1 Clone And Build On The VPS

SSH to the VPS and build the runtime binaries there:

```bash
ssh <user>@<vps-host>

git clone https://github.com/lookmanrays/codencer
cd codencer

make setup
make build
```

Expected build output shape:

```text
==> Building orchestratord...
==> Building orchestratorctl...
==> Building codencer-connectord...
==> Building codencer-relayd...
```

`make build` is enough for the daemon, CLI, connector, and raw relay path.

### 4.2 Start `orchestratord` On The VPS

For a repeatable first proof, start the daemon in simulation mode on the VPS:

```bash
ALL_ADAPTERS_SIMULATION_MODE=1 ./bin/orchestratord --repo-root "$PWD"
```

If you want actual VPS-side execution after the smoke pass, restart it later without `ALL_ADAPTERS_SIMULATION_MODE=1` and make sure the required executor binary is installed on the VPS.

In another VPS shell, confirm instance identity:

```bash
curl -fsS http://127.0.0.1:8085/api/v1/instance | jq '{id, repo_root}'
curl -fsS http://127.0.0.1:8085/api/v1/compatibility | jq '{tier, adapters, environment}'
```

Expected shape:

```json
{
  "id": "<instance-id>",
  "repo_root": "/home/<user>/codencer"
}
```

At this point the VPS is the real execution side, even if the first smoke is simulation-only.

### 4.3 Start The Relay On The Laptop And Mint A Planner Token

On the laptop, use a checkout or copied binaries for the relay side:

```bash
git clone https://github.com/lookmanrays/codencer ~/codencer-relay
cd ~/codencer-relay

make build

mkdir -p .codencer/relay
./bin/codencer-relayd planner-token create \
  --config .codencer/relay/config.json \
  --write-config \
  --name operator \
  --scope '*' \
  --json | tee .codencer/relay/planner-token.json

PLANNER_TOKEN="$(jq -r '.token' .codencer/relay/planner-token.json)"

nohup ./bin/codencer-relayd \
  --config .codencer/relay/config.json \
  > .codencer/relay/relay.log 2>&1 &
echo $! > .codencer/relay/relay.pid

./bin/codencer-relayd status \
  --config .codencer/relay/config.json \
  --json | jq
```

Expected planner-token output snippet:

```json
{
  "name": "operator",
  "token": "<planner-token>",
  "write_config": true,
  "restart_required": true
}
```

Expected relay status snippet:

```json
{
  "planner_auth_mode": "static_bearer_tokens"
}
```

Operator note:

- the relay config defaults to `127.0.0.1:8090`
- if the VPS must reach it over outbound HTTPS only, publish an HTTPS URL for the laptop relay through your own reverse proxy or tunnel
- keep the raw relay private; expose only the HTTPS front door
- if you expect browser-style MCP callers, add the planner origin to `allowed_origins`

For the rest of this guide, assume the VPS will use:

```bash
export RELAY_URL="https://relay.example.com"
```

The laptop can keep using the local config and local admin CLI even if the VPS connector uses the published HTTPS URL.

### 4.4 Create A One-Time Enrollment Token On The Laptop

Mint a short-lived enrollment token from the laptop relay:

```bash
./bin/codencer-relayd enrollment-token create \
  --config .codencer/relay/config.json \
  --label vps-dev \
  --expires-in-seconds 600 \
  --json | tee .codencer/relay/enrollment-token.json

ENROLLMENT_TOKEN="$(jq -r '.secret' .codencer/relay/enrollment-token.json)"
```

Expected output shape:

```json
{
  "token_id": "<token-id>",
  "secret": "<single-use-enrollment-token>",
  "label": "vps-dev",
  "expires_at": "..."
}
```

This token is bootstrap-only and single-use. Prefer it over the legacy static `enrollment_secret` fallback.

### 4.5 Enroll And Run `codencer-connectord` On The VPS

Back on the VPS, enroll the connector against the laptop relay and the local VPS daemon:

```bash
cd ~/codencer

CONNECTOR_CONFIG=".codencer/connector/config.json"

./bin/codencer-connectord enroll \
  --relay-url "$RELAY_URL" \
  --daemon-url http://127.0.0.1:8085 \
  --enrollment-token "$ENROLLMENT_TOKEN" \
  --config "$CONNECTOR_CONFIG" \
  --label vps-dev

./bin/codencer-connectord list --config "$CONNECTOR_CONFIG" --json | jq

nohup ./bin/codencer-connectord run \
  --config "$CONNECTOR_CONFIG" \
  > .codencer/connector/connector.log 2>&1 &
echo $! > .codencer/connector/connector.pid

./bin/codencer-connectord status \
  --config "$CONNECTOR_CONFIG" \
  --json | jq
```

Expected enroll output:

```text
Connector enrolled: <connector-id> machine=<machine-id>
```

Expected `list` snippet:

```json
[
  {
    "daemon_url": "http://127.0.0.1:8085",
    "share": true
  }
]
```

Expected status snippet after the connector settles:

```json
{
  "relay_url": "https://relay.example.com",
  "session_state": "connected"
}
```

What happens here:

- the connector enrolls once with the single-use token
- it opens an outbound authenticated websocket to the relay under `/ws/connectors`
- the laptop still does not need a direct inbound path to the VPS other than SSH for you as the operator

### 4.6 Verify From The Laptop That The VPS Instance Is Shared

First verify the relay HTTP view from the laptop:

```bash
curl -fsS \
  -H "Authorization: Bearer $PLANNER_TOKEN" \
  "$RELAY_URL/api/v2/instances" | jq
```

Expected output snippet:

```json
[
  {
    "instance_id": "<instance-id>",
    "online": true
  }
]
```

Then verify the canonical planner MCP tool from the laptop:

```bash
curl -fsS -D /tmp/codencer-vps-mcp-headers.txt \
  -H "Authorization: Bearer $PLANNER_TOKEN" \
  -H "Content-Type: application/json" \
  -H "MCP-Protocol-Version: 2025-11-25" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}' \
  "$RELAY_URL/mcp" > /tmp/codencer-vps-mcp-init.json

SESSION_ID="$(awk -F': ' 'tolower($1)=="mcp-session-id" {gsub("\r", "", $2); print $2}' /tmp/codencer-vps-mcp-headers.txt)"

curl -fsS \
  -H "Authorization: Bearer $PLANNER_TOKEN" \
  -H "Content-Type: application/json" \
  -H "MCP-Session-Id: ${SESSION_ID}" \
  -H "MCP-Protocol-Version: 2025-11-25" \
  -d '{"jsonrpc":"2.0","id":2,"name":"codencer.list_instances","arguments":{}}' \
  "$RELAY_URL/mcp/call" | jq
```

Expected output snippet:

```json
[
  {
    "instance_id": "<instance-id>",
    "connector_id": "<connector-id>",
    "online": true
  }
]
```

At this point `codencer.list_instances` from the laptop is seeing the VPS-shared instance, not a laptop-local daemon.

### 4.7 Run A Task From The Laptop Planner Through The Relay To The VPS

The Codencer-side contract is:

- planner targets relay `/mcp` or cloud `/api/cloud/v1/mcp`
- planner calls `codencer.list_instances`, `codencer.start_run`, `codencer.submit_task`, `codencer.wait_step`, and `codencer.get_step_result`
- execution still happens on the VPS daemon through the connector

Keep the product claim narrow:

- ChatGPT-style and Claude Code-style operator paths are packaged for the canonical remote MCP surface; Claude Desktop product setup remains `compatibility-only`
- follow [mcp/integrations.md](mcp/integrations.md) for the planner/client contract and the checked-in example files
- do not point those planner products at the VPS daemon directly

For a first planner-side smoke from the laptop, use this sequence:

1. Point the planner at the laptop relay MCP URL.
2. Ask it to call `codencer.list_instances`.
3. Ask it to call `codencer.start_run`.
4. Ask it to call `codencer.submit_task`.
5. Ask it to call `codencer.wait_step` and `codencer.get_step_result`.

Example Codencer MCP payloads:

```json
{
  "instance_id": "<instance-id>",
  "payload": {
    "id": "vps-planner-smoke-001",
    "project_id": "vps-planner-smoke"
  }
}
```

```json
{
  "instance_id": "<instance-id>",
  "run_id": "vps-planner-smoke-001",
  "task": {
    "version": "v1",
    "goal": "Compatibility smoke only. Return the VPS repo root in the final summary. Do not edit files.",
    "is_simulation": true
  }
}
```

Expected `submit_task` shape:

```json
{
  "id": "step-<opaque>",
  "state": "queued"
}
```

Expected `wait_step` shape:

```json
{
  "state": "completed",
  "terminal": true
}
```

When you are ready for real VPS execution instead of a compatibility smoke:

- restart `orchestratord` on the VPS without simulation mode
- install the target executor binary on the VPS
- set `adapter_profile` in the submitted task

The laptop still remains only the planner side.

## 5. Repeatable Verification Smoke

Once the VPS daemon is running and the laptop relay is reachable, use the checked-in self-host smoke helper from the VPS. This is the most repeatable repo-truth verification loop for this topology.

On the VPS:

```bash
cd ~/codencer

PLANNER_TOKEN="<planner-token>" \
RELAY_URL="$RELAY_URL" \
DAEMON_URL="http://127.0.0.1:8085" \
SMOKE_SCENARIOS="status,audit,mcp,mcp-sdk" \
./scripts/self_host_smoke.sh
```

Expected output starts like:

```text
--- Codencer Self-Host Smoke ---
Daemon:    http://127.0.0.1:8085
Relay:     https://relay.example.com
Scenarios: status,audit,mcp,mcp-sdk
Local instance: <instance-id>
```

Expected summary tail looks like:

```text
--- Self-Host Smoke Summary ---
Run:         smoke-run-...
Step:        ...
State:       completed
Terminal:    true
Summary:     ...
MCP SDK:     /tmp/...
```

What this verifies in the VPS-plus-laptop topology:

- the VPS daemon identity is readable
- the laptop relay can mint enrollment tokens and accept the connector session
- the VPS connector can advertise the shared VPS instance
- relay HTTP and relay MCP both route to the VPS daemon
- the official Go SDK smoke helper can talk to relay `/mcp`

This is the repeatable smoke to rerun after VPS rebuilds, relay restarts, or firewall changes.

## 6. Security Notes

For this topology, keep the security posture intentionally small:

- planner auth is a relay bearer token
- connector bootstrap uses a single-use enrollment token
- the VPS should expose no inbound Codencer ports; keep inbound to SSH only
- the connector should reach the laptop relay or cloud over outbound HTTPS and `wss` only
- do not expose the daemon on `8085` to the internet
- if you publish the relay, publish only the relay or cloud front door, not the VPS executor side

Practical firewall shape:

- VPS inbound: `22/tcp` only
- VPS outbound: HTTPS to the laptop relay front door or laptop cloud front door
- laptop inbound: whatever your HTTPS front door requires

## 7. Known Limitations For This Topology

The same repo-wide beta limits still apply here:

- self-host planner auth is static bearer-token auth, not enterprise IAM
- abort is best-effort; Codencer only reports success when the active step actually reaches `cancelled`
- artifact transport is bounded and is not a bulk file tunnel
- connector artifact content transport is capped at `8 MiB`; larger payloads fail as too large for connector transport
- ChatGPT-style and Claude Code-style planner wiring is packaged for operator use; Claude Desktop and vendor product setup remain outside direct repo proof

Use [KNOWN_LIMITATIONS.md](KNOWN_LIMITATIONS.md) for the consolidated boundary list.

## 8. Alternate Topologies

### 8.1 Relay Also On The VPS

If you do not need the laptop to host the raw relay, you can move `codencer-relayd` onto the VPS and keep the same daemon-plus-connector local placement there.

That variant is simpler operationally, but it changes the trust split:

- the planner surface and execution surface now sit on the same machine
- you lose the laptop-local operator separation that this page is optimizing for
- you still should not expose the daemon directly

### 8.2 Laptop Cloud Instead Of Laptop Relay

If you want the laptop to host the cloud control plane instead of the raw relay:

- keep `orchestratord` and `codencer-connectord` on the VPS
- use the cloud-composed runtime path from [CLOUD_SELF_HOST.md](CLOUD_SELF_HOST.md)
- claim the VPS runtime connector into cloud scope before using cloud runtime HTTP or cloud MCP

For the Docker baseline when you want a composed cloud reference instead of the raw relay path, use [CLOUD_SELF_HOST.md](CLOUD_SELF_HOST.md).

## 9. Where To Go Next

- use [BETA_TESTING.md](BETA_TESTING.md) for the frozen public beta track chooser
- use [SELF_HOST_REFERENCE.md](SELF_HOST_REFERENCE.md) for the raw relay/runtime operator flow
- use [mcp/integrations.md](mcp/integrations.md) for the relay-vs-cloud planner/client contract
- use [KNOWN_LIMITATIONS.md](KNOWN_LIMITATIONS.md) when you need the current boundary list in one place
