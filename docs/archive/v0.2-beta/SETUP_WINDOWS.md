# Legacy v0.2 Beta Document

This document describes the older v0.2 beta track. For current v0.3 local/self-host RC guidance, use [README](../../../README.md), [Local Quickstart](../../quickstart-local.md), and [Self-Host Relay Quickstart](../../quickstart-self-host-relay.md).

# Windows Setup (WSL Daemon + Windows IDE / Broker)

This page replaces the Wave 1 placeholder with the current repo-truth Windows walkthrough for `v0.2.0-beta`.

> [!IMPORTANT]
> Repo truth for this release:
> - `orchestratord` is **not** a natively supported Windows daemon path.
> - The recommended Windows topology is:
>   - **WSL/Linux**: repo checkout, daemon, worktrees, artifacts, connector
>   - **Windows**: IDE, Antigravity, optional `agent-broker`
> - If you want a pure Windows-only runtime, use **WSL2** for the daemon side instead of trying to run the daemon natively on Windows.
> - `agent-broker` remains a **secondary / experimental** bridge in the repo support matrix, not part of the primary beta promise.

Use [SETUP.md](SETUP.md) for the common build and platform hub, [BETA_TESTING.md](BETA_TESTING.md) for the frozen beta tracks, and [mcp/integrations.md](mcp/integrations.md) for the exact relay/cloud planner-client contract.

## 1. Recommended Topology

This is the practical mixed Windows layout described elsewhere in the repo:

```text
Windows IDE / planner client
  -> Windows localhost relay URL (native or forwarded)
  -> Codencer relay
  -> Codencer connector (WSL)
  -> Codencer daemon (WSL)
  -> local adapters / worktrees / artifacts (WSL)

Windows Antigravity
  -> agent-broker (Windows localhost)
  -> daemon Antigravity binding APIs (WSL)
```

Keep these boundaries clear:

- the **daemon** is the local system of record
- the **connector** is the outbound bridge from daemon to relay
- the **relay** is the remote planner surface
- the **agent-broker** is a local IDE bridge, not the relay

Do **not** expose the daemon directly to ChatGPT, Claude Desktop, or any other remote planner client. The daemon-local `/mcp/call` path is compatibility-only and not the public planner contract.

## 2. What Runs Where

### WSL/Linux side

- repo checkout
- `orchestratord`
- `codencer-connectord`
- worktrees
- artifacts
- optional relay if you keep the relay next to the daemon

### Windows side

- IDE with **Antigravity** active
- `agent-broker`
- optional relay if you intentionally want the relay to terminate on Windows localhost
- ChatGPT / Claude Desktop style clients pointed at the **relay**, not the daemon

## 3. Build The Required Binaries

### 3.1 WSL: build the daemon-side binaries

From the repo root in WSL:

```bash
make build
```

Expected build lines include:

```text
==> Building orchestratord...
==> Building orchestratorctl...
==> Building codencer-connectord...
==> Building codencer-relayd...
```

### 3.2 Windows: build `agent-broker`

The broker is built separately because `cmd/broker` is a nested module.

From the repo root in a **Windows shell**:

```powershell
make build-broker
```

Expected build line:

```text
==> Building agent-broker (nested module)...
```

The resulting Windows binary is:

```text
.\bin\agent-broker.exe
```

If you intentionally want the relay on Windows too, build the normal relay binary with the standard `make build` flow from [SETUP.md](SETUP.md). The mixed-layout recommendation in this page still keeps the daemon and connector in WSL.

## 4. WSL Walkthrough: Daemon + Relay + Connector

This is the recommended side for repo state, worktrees, artifacts, and execution.

### 4.1 Start the daemon in WSL

```bash
./bin/orchestratord --repo-root /home/<user>/Projects/codencer
```

In a second WSL shell, verify runtime truth:

```bash
curl -fsS http://127.0.0.1:8085/api/v1/compatibility
./bin/orchestratorctl instance --json
```

Expected output snippets:

```json
{"tier":...}
```

```json
{
  "id": "<instance-id>",
  "repo_root": "/home/<user>/Projects/codencer",
  "base_url": "http://127.0.0.1:8085"
}
```

### 4.2 Start a relay in WSL (recommended default)

```bash
mkdir -p .codencer/relay
./bin/codencer-relayd planner-token create \
  --config .codencer/relay/config.json \
  --write-config \
  --name operator \
  --scope '*'

./bin/codencer-relayd --config .codencer/relay/config.json
```

From another WSL shell:

```bash
./bin/codencer-relayd status --config .codencer/relay/config.json
```

Expected output snippet:

```json
{
  "planner_auth_mode": "static_bearer_tokens",
  "connector_count": 0,
  "instance_count": 0
}
```

### 4.3 Enroll and run the connector in WSL

Create a one-time enrollment token:

```bash
./bin/codencer-relayd enrollment-token create \
  --config .codencer/relay/config.json \
  --label wsl-dev \
  --expires-in-seconds 600 \
  --json
```

Enroll the connector against the WSL daemon:

```bash
./bin/codencer-connectord enroll \
  --relay-url http://127.0.0.1:8090 \
  --daemon-url http://127.0.0.1:8085 \
  --enrollment-token <token>

./bin/codencer-connectord share --daemon-url http://127.0.0.1:8085
./bin/codencer-connectord run
```

Verify connector state:

```bash
./bin/codencer-connectord status
./bin/codencer-connectord list
```

Expected output snippets:

```text
connector=<connector-id> machine=<machine-id> relay=http://127.0.0.1:8090 state=connected
configured_instances=1 shared_config=1 unshared_config=0
```

```text
state=shared instance_id=<instance-id> daemon_url=http://127.0.0.1:8085 manifest_path=<...>
```

## 5. Windows Walkthrough: Antigravity + `agent-broker`

This page assumes **Antigravity** is the Windows IDE path.

### 5.1 Antigravity discovery path on Windows

The broker discovers Antigravity instances from:

```text
%USERPROFILE%\.gemini\antigravity\daemon\ls_*.json
```

The broker persists repo bindings at:

```text
%USERPROFILE%\.gemini\antigravity\broker_binding.json
```

If `orchestratorctl antigravity list` shows no instances, first confirm that Antigravity is active in the Windows IDE and that the `ls_*.json` files exist under the directory above.

### 5.2 Start the broker on Windows

In a Windows shell:

```powershell
.\bin\agent-broker.exe
```

Expected startup line:

```text
Antigravity Broker v0.1.0-alpha starting on 127.0.0.1:8088
```

Verify the local broker endpoints:

```powershell
curl.exe -fsS http://127.0.0.1:8088/health
curl.exe -fsS http://127.0.0.1:8088/version
```

Expected output:

```json
{"status":"ok"}
```

```json
{"version":"0.1.0-alpha"}
```

### 5.3 Bind WSL Codencer to the Windows broker

Back in WSL:

```bash
export CODENCER_ANTIGRAVITY_BROKER_URL=http://127.0.0.1:8088
./bin/orchestratorctl antigravity list
./bin/orchestratorctl antigravity bind <pid>
./bin/orchestratorctl antigravity status
```

Expected output snippets:

```text
PID      PORT     REACHABLE    WORKSPACE
------------------------------------------------------------
```

```text
Successfully bound repo to Antigravity PID <pid>
```

```text
Status:      BOUND (Active)
PID:         <pid>
Port:        <https-port>
Workspace:   <workspace-uri>
```

When you want the cross-side adapter path explicitly, use the `antigravity-broker` adapter:

```bash
./bin/orchestratorctl submit windows-broker-smoke \
  --goal "Verify Windows Antigravity broker path" \
  --adapter antigravity-broker \
  --wait --json
```

That adapter choice is required for the broker-backed execution path. A repo-scoped Antigravity bind alone does not switch adapters automatically.

## 6. Wire ChatGPT / Claude Desktop On Windows To The Relay

ChatGPT-style and Claude Code-style operator lanes are packaged around the relay/cloud MCP surfaces. Keep the product claim narrow:

- Codencer proves the relay/cloud MCP surfaces
- Codencer does **not** claim direct repo-executed ChatGPT, Claude Desktop, or `claude.ai` product setup
- remote clients should target the **relay** `/mcp` endpoint, never the local daemon

### 6.1 Canonical Codencer-side values

For curl, MCP Inspector, the SDK smoke, or a local Claude Code process on Windows, the Codencer-side values are:

- URL: `http://127.0.0.1:8090/mcp`
- auth header: `Authorization: Bearer <planner-token>`
- canonical MCP path: `/mcp`
- compatibility alias for simple POST callers: `/mcp/call`

If the client asks for a protocol version, use the current repo examples from [mcp/integrations.md](mcp/integrations.md), which show `MCP-Protocol-Version: 2025-11-25`.

For ChatGPT product custom connectors or Claude remote connector UI setup, use a public HTTPS relay/cloud URL and OAuth front-door auth where the product requires OAuth. Do not use `127.0.0.1`/`localhost` as the product connector URL.

### 6.2 If the relay itself runs on Windows

Point the Windows client directly at:

```text
http://127.0.0.1:8090/mcp
```

This is the simplest Windows client wiring because the client and relay are on the same loopback interface.

### 6.3 If the relay runs in WSL

Preferred outcome: the Windows client still reaches the relay as:

```text
http://127.0.0.1:8090/mcp
```

If Windows localhost already reaches the WSL listener, no extra step is needed.

If it does **not**, add an OS-level localhost port forward from Windows to the WSL relay port and keep the client URL unchanged. Example:

```powershell
wsl hostname -I
netsh interface portproxy add v4tov4 listenaddress=127.0.0.1 listenport=8090 connectaddress=<wsl-ip> connectport=8090
```

After that, a Windows-local client can still use:

```text
http://127.0.0.1:8090/mcp
```

This keeps ChatGPT / Claude Desktop style Windows configuration stable even when the actual relay process lives in WSL.

### 6.4 Windows-side relay verification before opening the client

Before you configure a Windows MCP client, verify the relay from Windows:

```powershell
curl.exe -fsS ^
  -H "Authorization: Bearer <planner-token>" ^
  http://127.0.0.1:8090/api/v2/status
```

Expected output snippet:

```json
{
  "planner_auth_mode": "static_bearer_tokens",
  "connector_count": 1,
  "instance_count": 1
}
```

Then verify MCP initialize on the same Windows URL:

```powershell
curl.exe -fsS -D mcp-headers.txt ^
  -H "Authorization: Bearer <planner-token>" ^
  -H "Content-Type: application/json" ^
  -H "MCP-Protocol-Version: 2025-11-25" ^
  -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\",\"params\":{\"protocolVersion\":\"2025-11-25\"}}" ^
  http://127.0.0.1:8090/mcp
```

Expected verification points:

- HTTP `200`
- response body contains `"jsonrpc":"2.0"`
- response headers include `MCP-Session-Id`

If this Windows `curl.exe` check fails, the product client will fail too. Fix the relay URL, token, or localhost forwarding first.

## 7. Known Limitations For This Topology

Keep these limits explicit:

- the Windows daemon path is **unsupported**; use WSL for daemon-side runtime
- the mixed WSL/Windows layout is **operator guidance**, not an automated smoke-proof matrix
- `agent-broker` is a **secondary / experimental** bridge in the repo support classification
- `agent-broker` task sessions are **in-memory only**; restarting the broker can orphan active tasks
- ChatGPT-style and Claude Code-style MCP wiring is packaged for operator use; vendor product UI/auth remains outside repo proof
- Windows Firewall / Defender may prompt the first time `agent-broker.exe`, `codencer-relayd.exe`, or localhost port forwarding is used; approve only the local/private binding you intend to use

Also remember:

- keep the relay separate from the broker
- keep the connector on the same side as the daemon
- keep artifacts and worktrees on the WSL side
- inspect results via Codencer APIs and CLI rather than raw cross-side paths

## 8. Repeatable Verification Smoke

Use this as the repeatable mixed-layout smoke for this page.

### 8.1 WSL daemon truth

```bash
curl -fsS http://127.0.0.1:8085/api/v1/compatibility
./bin/orchestratorctl instance --json
```

Pass condition:

- `/api/v1/compatibility` includes `"tier"`
- `instance --json` shows the expected WSL `repo_root`

### 8.2 Windows broker truth

```powershell
curl.exe -fsS http://127.0.0.1:8088/health
curl.exe -fsS http://127.0.0.1:8088/version
```

Pass condition:

- `{"status":"ok"}`
- `{"version":"0.1.0-alpha"}`

### 8.3 WSL Antigravity bind truth

```bash
export CODENCER_ANTIGRAVITY_BROKER_URL=http://127.0.0.1:8088
./bin/orchestratorctl antigravity list
./bin/orchestratorctl antigravity bind <pid>
./bin/orchestratorctl antigravity status
```

Pass condition:

- at least one reachable Antigravity instance is listed
- bind succeeds
- status is `BOUND (Active)`

### 8.4 Relay truth from Windows

```powershell
curl.exe -fsS ^
  -H "Authorization: Bearer <planner-token>" ^
  http://127.0.0.1:8090/api/v2/status
```

Pass condition:

- JSON contains `"planner_auth_mode":"static_bearer_tokens"`
- JSON shows at least one connected connector / instance after the WSL connector is running

### 8.5 Relay MCP truth from Windows

```powershell
curl.exe -fsS -D mcp-headers.txt ^
  -H "Authorization: Bearer <planner-token>" ^
  -H "Content-Type: application/json" ^
  -H "MCP-Protocol-Version: 2025-11-25" ^
  -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\",\"params\":{\"protocolVersion\":\"2025-11-25\"}}" ^
  http://127.0.0.1:8090/mcp
```

Pass condition:

- HTTP `200`
- body contains `"jsonrpc":"2.0"`
- `mcp-headers.txt` contains `MCP-Session-Id`

### 8.6 Optional broker-backed adapter smoke

```bash
./bin/orchestratorctl submit windows-broker-smoke \
  --goal "Verify Windows Antigravity broker path" \
  --adapter antigravity-broker \
  --wait --json
```

Pass condition:

- the run completes without broker connection failure
- follow-up result inspection shows the broker execution context rather than a local non-broker adapter path

For the frozen repo-level beta proof boundaries, return to [BETA_TESTING.md](BETA_TESTING.md). For the canonical planner/client surfaces and MCP examples, return to [mcp/integrations.md](mcp/integrations.md).
