# macOS Setup

This page is the macOS zero-to-smoke walkthrough for the `v0.2.0-beta` local and self-host relay paths. Keep claims narrow: the canonical repo-wide beta contract still lives in [BETA_TESTING.md](BETA_TESTING.md), and the planner/client contract still lives in [mcp/integrations.md](mcp/integrations.md).

Use this page when all of the following are true:

- you are on macOS
- you are running the repo, daemon, and connector locally on that Mac
- you want the supported non-Docker verification path first

> [!IMPORTANT]
> Use a real `git clone`. Do not use a ZIP download. Codencer depends on git worktrees for isolated attempts.

## 1. macOS Prerequisites

Codencer's supported beta verification path requires:

- Git
- Go `1.25.0+`
- `cc` for the CGO SQLite build
- `curl`
- `jq` or Python 3 for JSON-parsing shell helpers

On macOS, the shortest reliable setup is:

```bash
xcode-select --install
brew install go jq
```

Then confirm the toolchain:

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
go version go1.25.x darwin/arm64
Apple clang version ...
1
clang
```

Notes:

- Xcode Command Line Tools provide `cc` on macOS.
- Homebrew Go `1.25+` is the required floor for this beta track.
- Do not force `CGO_ENABLED=0`; the repo build uses the CGO SQLite driver.
- If `go env CGO_ENABLED` reports `0`, unset that override before building.

## 2. Clean Clone To Supported Verification

This is the repo-truth clean-checkout path for macOS:

```bash
git clone https://github.com/lookmanrays/codencer
cd codencer
make setup
make build-supported
make verify-beta
```

### 2.1 `make setup`

Expected output from a fresh checkout:

```text
==> Initializing local environment (.codencer/)...
==> Creating .env from .env.example...
```

What it does:

- creates `.env` from `.env.example` if needed
- creates `bin/`
- creates `.codencer/artifacts`
- creates `.codencer/workspace`

### 2.2 `make build-supported`

Expected output shape:

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

### 2.3 `make verify-beta`

`make verify-beta` is the supported non-Docker repo pass from [BETA_TESTING.md](BETA_TESTING.md). Expected output shape:

```text
==> Running main-module tests...
==> Running local smoke...
==> smoke test complete: SUCCESS
==> Creating temporary relay planner token config...
==> Starting temporary simulation daemon on http://127.0.0.1:18085...
==> Starting temporary relay on http://127.0.0.1:18090...
==> Running self-host relay/runtime smoke with MCP + SDK...
==> Running cloud binary smoke...
==> Validating docker compose config...
==> Supported beta-track verification complete.
```

If you only want the Docker-backed cloud baseline on a Docker-capable host, that is a separate command:

```bash
make verify-beta-docker
```

## 3. Repeatable Local Smoke On macOS

After the clean-checkout pass, this is the repeatable local smoke loop:

```bash
make start-sim
./bin/orchestratorctl doctor
./scripts/smoke_test_v1.sh
make smoke
```

Expected output shape:

```text
==> Starting orchestratord in SIMULATION MODE (background)...
Simulated daemon successfully started on http://127.0.0.1:8085 ...
```

```text
[OK]    .codencer directory found
[OK]    Git detected: ...
[OK]    Go detected: ...
[OK]    C Compiler (for SQLite CGO) detected: ...
[OK]    Daemon reachable at http://127.0.0.1:8085
```

```text
==> starting smoke test: smoke-test-...
==> Auditing last step: ...
==> smoke test complete: SUCCESS
```

Use `/api/v1/compatibility`, `orchestratorctl doctor`, and the smoke scripts as runtime truth for a Mac environment. Do not use the compatibility endpoint as a support certificate by itself.

## 4. Local Adapter Verification Matrix For macOS

Keep the support claims aligned with [internal/BETA_SUPPORT_CLASSIFICATION.md](internal/BETA_SUPPORT_CLASSIFICATION.md), [BETA_TESTING.md](BETA_TESTING.md), and [KNOWN_LIMITATIONS.md](KNOWN_LIMITATIONS.md).

| Surface | macOS path | Current repo truth | What to verify locally on Mac |
| --- | --- | --- | --- |
| `codex` adapter | local daemon path on the Mac | `supported-beta target`; checked-in proof covers simulation plus fake-binary `codex exec` | Canonical Mac proof is still the local smoke path. For a live Codex binary test, set `CODEX_BINARY` if needed, run real mode, and confirm availability through `/api/v1/compatibility`. |
| `claude` adapter | local daemon path on the Mac | `supported-beta target`, wrapper path proven, no live authenticated repo proof | Install the `claude` CLI, ensure `CLAUDE_BINARY` or `$PATH` resolves it, then verify with `/api/v1/compatibility` and a real-mode local run. Claude is invoked as `claude -p --output-format json` with `stdin` prompt input and attempt workspace `cwd`. |
| `antigravity` / `agent-broker` | not applicable on native macOS | `secondary`, Windows/WSL-oriented topology only | Do not treat this as a native Mac path. The documented broker topology is for Windows-side Antigravity with Linux/WSL-side execution boundaries, not a primary macOS local adapter path. |
| `openclaw-acpx` | local daemon path on the Mac if `acpx` is installed | `experimental` / deferred | Only use this if you are intentionally testing the experimental ACP bridge. Set `OPENCLAW_ACPX_BINARY` if needed and keep claims at experimental status. |

## 5. Single-Machine Relay Loopback On macOS

This section keeps the daemon, relay, and connector on one Mac and exposes only the relay as the planner surface.

### 5.1 Start the local daemon

Use simulation mode for the repeatable smoke path:

```bash
make start-sim
./bin/orchestratorctl instance --json
```

### 5.2 Create a relay config and planner token

```bash
mkdir -p .codencer/relay
./bin/codencer-relayd planner-token create \
  --config .codencer/relay/config.json \
  --write-config \
  --name operator \
  --scope '*'
```

### 5.3 Start the relay

```bash
./bin/codencer-relayd --config .codencer/relay/config.json
```

In another terminal:

```bash
./bin/codencer-relayd status --config .codencer/relay/config.json --json
```

### 5.4 Enroll the local connector

Create a one-time enrollment token:

```bash
./bin/codencer-relayd enrollment-token create \
  --config .codencer/relay/config.json \
  --label mac-local \
  --expires-in-seconds 600 \
  --json
```

Then enroll the connector against the same Mac's daemon:

```bash
./bin/codencer-connectord enroll \
  --relay-url http://127.0.0.1:8090 \
  --daemon-url http://127.0.0.1:8085 \
  --enrollment-token <token>
```

### 5.5 Run and verify the connector

```bash
./bin/codencer-connectord run
```

In another terminal:

```bash
./bin/codencer-connectord status --json
./bin/codencer-connectord list
./bin/codencer-relayd instances --config .codencer/relay/config.json --json
./bin/codencer-relayd connectors --config .codencer/relay/config.json --json
```

Expected outcome:

- the connector is online
- the repo-local daemon instance appears on the relay
- the relay is the only planner-facing surface you expose

### 5.6 Run the relay smoke

Once the daemon and relay are already running:

```bash
PLANNER_TOKEN=<planner-token> make self-host-smoke-mcp
```

Expected output shape:

```text
--- Codencer Self-Host Smoke ---
Daemon:    http://127.0.0.1:8085
Relay:     http://127.0.0.1:8090
Scenarios: status,audit,mcp,mcp-sdk
Local instance: ...
--- Self-Host Smoke Summary ---
Run:         ...
Step:        ...
State:       completed
Terminal:    true
Summary:     ...
```

This is the canonical single-machine loopback proof for the self-host relay/runtime path on macOS. For the broader operator flow, see [SELF_HOST_REFERENCE.md](SELF_HOST_REFERENCE.md).

## 6. Wiring ChatGPT And Claude Desktop On macOS

Keep the planner/client claim narrow:

- use the relay MCP surface at `/mcp`
- do not point ChatGPT, Claude Desktop, or another planner runtime at the local daemon
- treat ChatGPT-style and Claude Code-style operator lanes as packaged around the remote MCP surface; keep vendor product UI/auth claims narrow

Codencer-side loopback target for curl, MCP Inspector, the SDK smoke, or a local Claude Code process running on the same Mac:

```text
http://127.0.0.1:8090/mcp
Authorization: Bearer <planner-token>
```

For ChatGPT product custom connectors on macOS:

- use a public HTTPS relay/cloud URL, not `127.0.0.1` and not daemon `/mcp/call`
- use OAuth front-door auth for write-capable product setup
- keep the claim at the operator-packaged remote MCP lane, not universal ChatGPT product support
- follow [mcp/integrations/chatgpt.md](mcp/integrations/chatgpt.md) for the operator walkthrough

For Claude Desktop or `claude.ai` on macOS:

- use a public HTTPS relay/cloud URL because remote connectors originate from Anthropic's cloud
- keep the planner-side remote connector path separate from the local `claude` executor adapter
- follow [mcp/integrations/claude.md](mcp/integrations/claude.md) for the current remote connector walkthrough
- use [mcp/examples/claude-desktop-relay.mcp.json](mcp/examples/claude-desktop-relay.mcp.json) as a value-reference example, not as a direct product import

For current product-specific MCP behavior outside Codencer itself, follow the current vendor docs plus the Codencer-side contract in [mcp/integrations.md](mcp/integrations.md).

## 7. macOS-Specific Practical Limits

These are practical operator-side notes for Mac environments. Keep them as local-environment caveats, not beta-promotion claims.

- Keychain permissions: vendor CLIs such as Claude or Codex may prompt for macOS Keychain access the first time they need credentials. If a real-mode run fails immediately, re-check the CLI directly in the same terminal session.
- Firewall prompts: the first local bind for relay or daemon may trigger a macOS firewall prompt depending on host policy. Allow local loopback access if you intend to use `127.0.0.1:8085` and `127.0.0.1:8090`.
- Codesign noise: locally built Go binaries may produce unsigned-binary or first-run trust noise on tightly managed Macs. Treat this as host policy friction, not as a Codencer planner/runtime feature.
- Adapter proof remains narrow: on macOS, as on other platforms, the canonical proof is still simulation-first for local smoke; live adapter validation depends on your local binary, auth, and shell environment.

## 8. Where To Go Next

- Use [BETA_TESTING.md](BETA_TESTING.md) for the frozen public beta track chooser.
- Use [mcp/integrations.md](mcp/integrations.md) for the relay-vs-cloud planner/client contract.
- Use [SELF_HOST_REFERENCE.md](SELF_HOST_REFERENCE.md) for the full relay/runtime operator flow.
- Use [KNOWN_LIMITATIONS.md](KNOWN_LIMITATIONS.md) when you need the current boundary list in one place.
