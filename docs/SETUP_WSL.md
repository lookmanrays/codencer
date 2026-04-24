# WSL Setup

This guide is the WSL2 zero-to-smoke path for Codencer `v0.2.0-beta`.

Use it when all of the following are true:

- you are on **WSL2**
- your distro is **Ubuntu or Debian**
- you want the **repo, daemon, connector, worktrees, and artifacts** to stay inside WSL/Linux
- you want the relay reachable from WSL first, and optionally from Windows-side planner clients later

For the frozen repo-wide beta tracks, see [BETA_TESTING.md](BETA_TESTING.md). For the remote planner/client contract, see [mcp/integrations.md](mcp/integrations.md). For the WSL plus Windows broker layout, see [WSL_WINDOWS_ANTIGRAVITY.md](WSL_WINDOWS_ANTIGRAVITY.md).

## 1. WSL Support Boundary

Current repo truth for this guide:

- **WSL2 (Ubuntu/Debian)** is a supported setup path for the local daemon and repo checkout.
- Keep the **daemon and connector on the same side as the repo**: inside WSL.
- The **relay** is the remote planner surface.
- The daemon-local `/mcp/call` endpoint is **compatibility-only** and is **not** the public remote MCP contract.
- ChatGPT-style and Claude-style desktop wiring remains **compatibility-only** in this beta. Codencer proves the relay `/mcp` surface, not each product UI flow.

Recommended topology:

```text
Windows desktop client (optional)
  -> relay /mcp on localhost:8090
  -> connector websocket from WSL
  -> daemon on 127.0.0.1:8085 inside WSL
  -> WSL-local executor adapter
```

## 2. WSL2 Prerequisites

Install the Ubuntu/Debian packages that the beta docs require:

```bash
sudo apt update
sudo apt install -y build-essential ca-certificates curl git jq
```

Install Go `1.25.0+`.

The repo requires Go `1.25.0+`. The example below uses `go1.25.9`, which was listed on `go.dev/dl` on 2026-04-24. If a newer `1.25.x` patch exists when you read this, use that instead.

```bash
ARCH="$(dpkg --print-architecture)"
case "$ARCH" in
  amd64) GO_ARCH=amd64 ;;
  arm64) GO_ARCH=arm64 ;;
  *)
    echo "Unsupported WSL architecture: $ARCH"
    exit 1
    ;;
esac

GO_VERSION=1.25.9
curl -fsSLO "https://go.dev/dl/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz"
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf "go${GO_VERSION}.linux-${GO_ARCH}.tar.gz"
echo 'export PATH=/usr/local/go/bin:$PATH' >> ~/.profile
export PATH=/usr/local/go/bin:$PATH
```

Verify the toolchain:

```bash
go version
gcc --version | head -n 1
jq --version
git --version
```

Expected output looks like:

```text
go version go1.25.9 linux/amd64
gcc (Ubuntu ...) ...
jq-1.6
git version ...
```

## 3. Clean Clone And Supported Verification

Start with a clean WSL clone and run the repo-level supported verifier exactly as the beta docs describe:

```bash
git clone https://github.com/lookmanrays/codencer
cd codencer

make build-supported
make verify-beta
```

What to expect from `make build-supported`:

```text
==> Building orchestratord...
==> Building orchestratorctl...
==> Building codencer-connectord...
==> Building codencer-relayd...
==> Building codencer-cloudctl...
==> Building codencer-cloudd...
==> Building codencer-cloudworkerd...
==> Building mcp-sdk-smoke (official MCP SDK proof helper)...
```

What to expect from `make verify-beta`:

```text
==> Running main-module tests...
==> Running local smoke...
==> Running self-host relay/runtime smoke with MCP + SDK...
==> Running cloud binary smoke...
==> Validating docker compose config...
==> Supported beta-track verification complete.
```

This is the canonical fresh-checkout proof path from [BETA_TESTING.md](BETA_TESTING.md). Run `make verify-beta-docker` only if you also want the Docker-backed cloud baseline on a Docker-capable host.

## 4. One-Host Loopback Flow Inside WSL

This section keeps the **daemon, relay, and connector all inside WSL** on one host. It matches the self-host boundaries from [SELF_HOST_REFERENCE.md](SELF_HOST_REFERENCE.md) and the WSL topology guidance from [WSL_WINDOWS_ANTIGRAVITY.md](WSL_WINDOWS_ANTIGRAVITY.md).

### 4.1 Start A Simulation Daemon In WSL

Simulation mode is the most repeatable local proof path.

```bash
mkdir -p .codencer/relay .codencer/connector

REPO_ROOT="$(pwd)"
DAEMON_URL="http://127.0.0.1:8085"
RELAY_URL="http://127.0.0.1:8090"
CONNECTOR_CONFIG=".codencer/connector/config.json"

nohup env ALL_ADAPTERS_SIMULATION_MODE=1 ./bin/orchestratord \
  --repo-root "$REPO_ROOT" \
  > .codencer/daemon.log 2>&1 &
echo $! > .codencer/daemon.pid

for _ in $(seq 1 20); do
  if curl -fsS "$DAEMON_URL/api/v1/instance" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

curl -fsS "$DAEMON_URL/api/v1/instance" | tee .codencer/instance.json | jq
```

Expected output snippet:

```json
{
  "id": "<instance-id>",
  "repo_root": "/home/<user>/.../codencer"
}
```

### 4.2 Create A Relay Config And Planner Token

The relay is the public remote planner surface. Do not expose the daemon directly.

```bash
./bin/codencer-relayd planner-token create \
  --config .codencer/relay/config.json \
  --write-config \
  --name wsl-operator \
  --scope '*' \
  --json | tee .codencer/relay/planner-token.json

PLANNER_TOKEN="$(jq -r '.token' .codencer/relay/planner-token.json)"
```

Expected output snippet:

```json
{
  "name": "wsl-operator",
  "token": "<planner-token>"
}
```

### 4.3 Start The Relay In WSL

```bash
nohup ./bin/codencer-relayd \
  --config .codencer/relay/config.json \
  > .codencer/relay/relay.log 2>&1 &
echo $! > .codencer/relay/relay.pid

for _ in $(seq 1 20); do
  if ./bin/codencer-relayd status --config .codencer/relay/config.json --json >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

./bin/codencer-relayd status \
  --config .codencer/relay/config.json \
  --json | jq
```

Expected output snippet:

```json
{
  "planner_auth_mode": "static_bearer_tokens"
}
```

### 4.4 Enroll And Run The Connector In WSL

Create a short-lived enrollment token, enroll the connector against the WSL daemon, then run it.

```bash
./bin/codencer-relayd enrollment-token create \
  --config .codencer/relay/config.json \
  --label wsl-loopback \
  --expires-in-seconds 600 \
  --json | tee .codencer/relay/enrollment-token.json

ENROLLMENT_TOKEN="$(jq -r '.secret' .codencer/relay/enrollment-token.json)"

./bin/codencer-connectord enroll \
  --relay-url "$RELAY_URL" \
  --daemon-url "$DAEMON_URL" \
  --enrollment-token "$ENROLLMENT_TOKEN" \
  --config "$CONNECTOR_CONFIG" \
  --label wsl-loopback

./bin/codencer-connectord list --config "$CONNECTOR_CONFIG" --json | jq

nohup ./bin/codencer-connectord run \
  --config "$CONNECTOR_CONFIG" \
  > .codencer/connector/connector.log 2>&1 &
echo $! > .codencer/connector/connector.pid

for _ in $(seq 1 20); do
  if curl -fsS -H "Authorization: Bearer $PLANNER_TOKEN" "$RELAY_URL/api/v2/instances" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

./bin/codencer-connectord status --config "$CONNECTOR_CONFIG" --json | jq
```

Expected output snippet after `list`:

```json
[
  {
    "daemon_url": "http://127.0.0.1:8085",
    "share": true
  }
]
```

### 4.5 Confirm The Relay Sees The WSL Instance

```bash
curl -fsS \
  -H "Authorization: Bearer $PLANNER_TOKEN" \
  "$RELAY_URL/api/v2/instances" | jq
```

Expected output snippet:

```json
[
  {
    "instance_id": "<instance-id>"
  }
]
```

At this point the one-host loopback chain is live:

- daemon in WSL on `127.0.0.1:8085`
- relay in WSL on `127.0.0.1:8090`
- connector in WSL with an outbound websocket to the relay
- planner clients targeting the relay, not the daemon

## 5. Repeatable Verification Smoke

For a repeatable relay-plus-MCP proof against the WSL loopback stack you just started, run:

```bash
PLANNER_TOKEN="$PLANNER_TOKEN" \
RELAY_CONFIG=".codencer/relay/config.json" \
RELAY_URL="$RELAY_URL" \
DAEMON_URL="$DAEMON_URL" \
SMOKE_SCENARIOS="status,audit,mcp,mcp-sdk" \
./scripts/self_host_smoke.sh
```

Expected output starts like:

```text
--- Codencer Self-Host Smoke ---
Daemon:    http://127.0.0.1:8085
Relay:     http://127.0.0.1:8090
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

This lines up with the self-host proof boundary in [BETA_TESTING.md](BETA_TESTING.md) and the planner/client contract in [mcp/integrations.md](mcp/integrations.md).

## 6. Executor Adapters On WSL

Current WSL-facing adapter posture:

| Adapter | WSL posture | Beta label | Notes |
| --- | --- | --- | --- |
| `codex` | runs inside WSL if the binary is available there | `supported-beta target` | Primary intended local beta adapter, but checked-in proof is still simulation-heavy rather than live-binary proven. |
| `claude` / Claude Code | runs inside WSL if the `claude` binary is available there | `supported-beta target` | Wrapper proof is real, but live authenticated proof remains narrow. This is separate from Claude Desktop remote MCP wiring. |
| `qwen` | runs inside WSL if the binary is available there | `secondary` | Conformance and simulation coverage only; not part of the primary beta promise. |
| `antigravity` | do not treat as a WSL-native adapter path | `secondary` | Keep Antigravity on Windows. WSL reaches it through `agent-broker`; see [SETUP_WINDOWS.md](SETUP_WINDOWS.md) and [WSL_WINDOWS_ANTIGRAVITY.md](WSL_WINDOWS_ANTIGRAVITY.md). |

Antigravity remains a Windows-side concern in this topology:

- keep the repo, worktrees, daemon, connector, and artifacts in WSL
- keep `agent-broker` on Windows only when you need the IDE-side bridge
- do not move the connector to Windows just because Antigravity exists

## 7. Reaching The WSL Relay From Windows-Side ChatGPT Desktop Or Claude Desktop

Codencer-side truth stays narrow:

- point Windows-side planner clients at the **relay** on `/mcp`
- do **not** point them at the WSL daemon
- product-specific ChatGPT-style and Claude-style setup remains **compatibility-only**; use [mcp/integrations.md](mcp/integrations.md) as the Codencer contract

### 7.1 First Try `localhost`

Microsoft's current WSL networking docs say Windows can usually reach Linux-side services through `localhost`, and WSL's `localhostForwarding` setting is on by default for WSL2. If that holds on your machine, the Windows-side MCP endpoint is simply:

```text
http://localhost:8090/mcp
```

If your desktop client needs a config file on the Windows side, keep the URL as `http://localhost:8090/mcp`. You can read example files from the repo through `\\wsl$`, but `\\wsl$` is a filesystem path, not an HTTP host.

### 7.2 If `localhost` Does Not Work, Use An Explicit Windows Port Proxy

This is the explicit Windows-side port forward fallback for the WSL relay.

WSL networking mode, VPNs, firewall policy, and mirrored-mode behavior can change reachability. If a Windows-side client cannot hit `http://localhost:8090/mcp`, create an explicit Windows port proxy to the current WSL IP.

In **PowerShell as Administrator**:

```powershell
$WslIp = (wsl hostname -I).Trim().Split(' ')[0]
netsh interface portproxy add v4tov4 `
  listenaddress=127.0.0.1 `
  listenport=8090 `
  connectaddress=$WslIp `
  connectport=8090
```

Then use:

```text
http://localhost:8090/mcp
```

Important caveats:

- the WSL IP can change after `wsl --shutdown`, reboot, or distro restart
- mirrored networking and NAT networking behave differently
- Windows firewall or Hyper-V firewall policy can still block access
- some VPN setups change or break expected reachability

### 7.3 What `\\wsl$` Is Good For

`\\wsl$` is useful for:

- reading logs from Windows
- copying a checked-in example config out of the repo
- editing files from Windows tools while the repo still lives in WSL

`\\wsl$` is **not** the transport for relay HTTP or MCP traffic. Use `localhost` or an explicit forwarded port for that.

## 8. Known WSL Callouts

Keep these WSL-specific cautions in mind while testing:

- **Clock drift after sleep or resume can happen on WSL2.** If tokens suddenly look expired or TLS timestamps look wrong, compare `date` in WSL with Windows time. A WSL restart such as `wsl --shutdown` usually resets the clock; on affected machines you may also need an explicit time sync inside the distro.
- **Networking mode differences are real.** WSL NAT, mirrored mode, VPN policy, and firewall policy can change whether Windows reaches WSL services through `localhost` without extra setup.
- **Cross-side paths are not execution contracts.** Keep repo roots, worktrees, artifacts, daemon state, and connector state on the WSL side. Use APIs and CLI output for results and artifacts instead of assuming raw WSL paths are meaningful to Windows-side clients.
- **Do not expose the daemon directly.** Expose the relay if a Windows-side planner client needs remote MCP access.

These cautions are consistent with [KNOWN_LIMITATIONS.md](KNOWN_LIMITATIONS.md) and [WSL_WINDOWS_ANTIGRAVITY.md](WSL_WINDOWS_ANTIGRAVITY.md).

## 9. Stop The Local WSL Processes

When you are done with the manual loopback stack:

```bash
kill "$(cat .codencer/connector/connector.pid)"
kill "$(cat .codencer/relay/relay.pid)"
kill "$(cat .codencer/daemon.pid)"
```

If you used `make start-sim` instead of the explicit daemon command, you can also use:

```bash
make stop
```

## 10. Where To Go Next

- For the frozen beta verification tracks, go back to [BETA_TESTING.md](BETA_TESTING.md).
- For planner/client URL and MCP surface choices, use [mcp/integrations.md](mcp/integrations.md).
- For the mixed WSL plus Windows Antigravity topology, use [WSL_WINDOWS_ANTIGRAVITY.md](WSL_WINDOWS_ANTIGRAVITY.md) and [SETUP_WINDOWS.md](SETUP_WINDOWS.md).
- For the fuller self-host relay/runtime operator flow, use [SELF_HOST_REFERENCE.md](SELF_HOST_REFERENCE.md).
