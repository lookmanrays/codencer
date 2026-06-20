# Legacy v0.2 Beta Document

This document describes the older v0.2 beta track. For current v0.3 local/self-host RC guidance, use [README](../../../README.md), [Local Quickstart](../../quickstart-local.md), and [Self-Host Relay Quickstart](../../quickstart-self-host-relay.md).

# Environmental Reference & Setup

This guide is the setup hub for the Codencer beta tracks. Use it for prerequisites, a platform chooser, the native Linux inline path, and links back to the frozen beta-track docs.

## 1. Prerequisites

### Software Requirements
- **Git**: Required for worktree isolation.
- **Go 1.25.0+**: Required to build the daemon, CLI, connector, relay, and MCP SDK proof helper.
- **C Compiler (gcc/cc)**: Required for the CGO-based SQLite driver.
- **curl**: Required for health checking and polling.
- **jq or Python 3**: Recommended for bash/zsh automation wrappers that parse Codencer JSON output.

### Operating System
- **Linux** (Native): Primary supported platform.
- **WSL2** (Ubuntu/Debian): Fully supported.
- **Windows**: Not natively supported for the daemon; use the **Antigravity Broker** for cross-side communication.

---

## 2. Common Prerequisites And Build

### 2.1 Clone & Build
```bash
git clone https://github.com/lookmanrays/codencer
cd codencer

# 1. Initialize environment and check requirements
make setup

# 2. Build the supported beta-track binaries
make build-supported

# 3. Build the Windows-side agent-broker separately only if you need it
make build-broker
```

If you only need the local and relay tracks, `make build` remains sufficient.

### 2.2 Verify Environment
The `doctor` tool verifies whether your environment is ready for local execution and operator flows.

```bash
./bin/orchestratorctl doctor
```

---

## 3. Choose Your Platform

Use the platform guide that matches where the repo, daemon, and connector will actually live.

| Platform | Status in this wave | Start here | Notes |
| --- | --- | --- | --- |
| Native Linux | inline on this page | [Native Linux Inline Guide](#4-native-linux-inline-guide) | Primary supported platform for the local daemon and local artifacts. |
| macOS | Wave 2 placeholder | [SETUP_MACOS.md](SETUP_MACOS.md) | Placeholder only in this pass; use the beta-track and planner/client docs for current truth. |
| Windows Native | Wave 2 placeholder | [SETUP_WINDOWS.md](SETUP_WINDOWS.md) | The daemon is not a native Windows-supported path today. |
| WSL | Wave 2 placeholder | [SETUP_WSL.md](SETUP_WSL.md) | Recommended cross-side layout keeps repo, daemon, connector, and artifacts in WSL/Linux. |
| Remote VPS | Wave 2 placeholder | [SETUP_REMOTE_VPS.md](SETUP_REMOTE_VPS.md) | Use when hosting relay or cloud remotely while keeping the runtime boundaries explicit. |

If you want the planner/client surface chooser right away, use [mcp/integrations.md](mcp/integrations.md). If you want the repo-wide supported test matrix, use [BETA_TESTING.md](BETA_TESTING.md).

---

## 4. Native Linux Inline Guide

Native Linux remains the primary inline setup path in this doc.

### 4.1 Start The Daemon

The `orchestratord` daemon is the persistent system of record. It must be running to receive tasks.

Simulation mode validates the local daemon, CLI, and evidence path without requiring live executor binaries:

```bash
make start-sim
```

Real mode is for actual local execution:

```bash
# Edit .env to set ALL_ADAPTERS_SIMULATION_MODE=0
make start
```

Claude is executed in headless print mode as `claude -p --output-format json`. Codencer writes the built prompt to `prompt.txt`, sends it on `stdin`, and runs the process from the attempt workspace root.

The `/api/v1/compatibility` endpoint is a runtime diagnostic only. It is useful for checking binary availability, simulation mode, and local bindings, but it is not a support-certification surface by itself.

### 4.2 Current Local Adapter Proof Levels

| Surface | Current repo proof | Local beta truth |
| --- | --- | --- |
| Local daemon + CLI + simulation lifecycle | direct smoke + repo tests | canonical |
| `codex` adapter | simulation smoke + conformance + fake-binary `codex exec` proof | primary intended local beta adapter; live authenticated Codex service calls remain operator-environment proof |
| `claude` adapter | wrapper, prompt, normalize, and fake-binary tests | supported-beta target with narrow wrapper claims only |
| `qwen` adapter | conformance/simulation only | secondary |
| `antigravity` / `antigravity-broker` | mocked integration and environment-specific proof | secondary |
| `openclaw-acpx` | unit and simulation-only proof | experimental / deferred |
| `ide-chat` | code/manual handoff only | experimental / deferred |
| daemon-local `/mcp/call` | compatibility/admin bridge only | compatibility only; not the public planner MCP contract |

### 4.3 Local Smoke And Parity Proof

The canonical local proof paths are:

```bash
./scripts/smoke_test_v1.sh
./scripts/smoke_test_v1.sh
make smoke
```

`scripts/smoke_test_v1.sh` verifies the legacy six-input same-run path. If no daemon is already reachable, it auto-starts a temporary simulation daemon in the same shell so the local `submit --wait` barrier stays trustworthy for back-to-back submissions.

> [!IMPORTANT]
> The daemon-local `/mcp/call` endpoint is only a local compatibility/admin surface. The canonical remote MCP surface for planners lives on the relay at `/mcp`.

> [!IMPORTANT]
> For the practical self-host relay path, the canonical public binaries are `codencer-connectord` and `codencer-relayd`. The Windows-side `agent-broker` binary is built separately with `make build-broker` because `cmd/broker` is a nested module.

### 4.4 Daemon Management And Targeting

Codencer follows a **One-Repo-One-Instance** model. Each repo clone manages its own database and worktrees.

Anchor the daemon to a specific repository regardless of your current directory:

```bash
./bin/orchestratord --repo-root /path/to/my-project
```

To run multiple instances on the same machine, use a different port:

```bash
PORT=8086 ./bin/orchestratord --repo-root /path/to/project-b
```

Or use the helper:

```bash
./scripts/start_instance.sh ~/projects/my-api 8085
```

### 4.5 Environment Variables

Codencer uses these variables to locate executor binaries and target the daemon:
- `CODEX_BINARY`: Path to the `codex` CLI. Defaults to `codex`.
- `CODEX_ADAPTER_MODE`: Optional. Use `legacy-agent` only for the older `codex-agent` wrapper.
- `CLAUDE_BINARY`: Path to the `claude` binary. Defaults to `claude`.
- `OPENCLAW_ACPX_BINARY`: Path to the `acpx` CLI (for OpenClaw support).
- `ORCHESTRATORD_URL`: URL of the daemon (default: `http://localhost:8085`).

Claude notes:
- install the Claude CLI so the `claude` binary is available on your `$PATH`, or point `CLAUDE_BINARY` at the full path
- Codencer does not pass a workspace flag to Claude; the attempt workspace is supplied via process `cwd`
- raw Claude output is preserved in `stdout.log`; Codencer parses that JSON and synthesizes the normalized `result.json`

### 4.6 Workspace Provisioning

Codencer isolates every task attempt in a dedicated Git worktree. You can configure how those worktrees are prepared using `.codencer/workspace.json`.

Example:

```json
{
  "provisioning": {
    "copy": [".env"],
    "symlinks": ["node_modules"],
    "hooks": {
      "post_create": "go mod download"
    }
  }
}
```

Codencer can also read a Grove-compatible subset from `grove.yaml` or `.groverc.json` if a native config is missing, but it does not depend on the Grove CLI.

### 4.7 Submission Inputs

`orchestratorctl submit` supports both rich task files and narrow direct input.

Use a full YAML or JSON `TaskSpec` when you need path controls, constraints, or custom validation layout.
Use direct convenience input when a shell wrapper, planner, or local script needs a deterministic single-task submission:

- `--task-json <path|->`
- `--prompt-file <path>`
- `--goal <text>`
- `--stdin`

Exactly one primary source is required. Direct convenience input stays intentionally narrow and preserves both `original-input.*` and `normalized-task.json` in the attempt artifacts.

For concrete submit examples, see [EXAMPLES.md](EXAMPLES.md). For planner- and wrapper-oriented examples, see [CLI_AUTOMATION.md](CLI_AUTOMATION.md).

### 4.8 Cross-Side And Self-Host Notes

If you need the Windows-side bridge, build and run `agent-broker` separately and keep it distinct from the relay. For the practical WSL-first topology, keep the daemon and connector in WSL/Linux next to the repo and worktrees, keep the agent-broker on Windows when Antigravity is in play, and expose the relay instead of the daemon. See [WSL_WINDOWS_ANTIGRAVITY.md](WSL_WINDOWS_ANTIGRAVITY.md).

After the daemon and relay are running, the current happy-path self-host proof is:

```bash
PLANNER_TOKEN=<planner-token> make self-host-smoke
PLANNER_TOKEN=<planner-token> make self-host-smoke-mcp
PLANNER_TOKEN=<planner-token> make self-host-smoke-all
```

Use [SELF_HOST_REFERENCE.md](SELF_HOST_REFERENCE.md) for the full relay/runtime order of operations. Use [CLOUD_SELF_HOST.md](CLOUD_SELF_HOST.md) and [CLOUD.md](CLOUD.md) when the cloud control plane is part of the deployment. Use [mcp/integrations.md](mcp/integrations.md) for the relay-vs-cloud planner/client chooser.

### 4.9 OpenClaw Setup (Experimental / Alpha)

Codencer provides experimental ACP bridge support through the `openclaw-acpx` adapter.

- **Adapter ID**: `openclaw-acpx`
- **Binary**: `acpx`
- **Status**: experimental / deferred for the current beta promise

To configure a custom ACPX binary path:

```bash
export OPENCLAW_ACPX_BINARY=/path/to/custom/acpx
```

Codencer manages the `acpx` process lifecycle and workspace isolation, but it does not manage model routing, API keys, or backend selection for OpenClaw.

---

## 5. Choose Your Track

Use [BETA_TESTING.md](BETA_TESTING.md) as the repo-level tester guide. Pick the track that matches the surface you want to prove.

| Track | Start here | Build | Proof command | Current boundary |
| --- | --- | --- | --- | --- |
| Local-only daemon + CLI | [SETUP.md](SETUP.md) | `make build` | `./scripts/smoke_test_v1.sh` then `make smoke` | Canonical local proof is simulation-first; live adapter proof stays narrow. |
| Self-host relay + runtime connector | [SELF_HOST_REFERENCE.md](SELF_HOST_REFERENCE.md) | `make build` | `PLANNER_TOKEN=<planner-token> make self-host-smoke-mcp` | Canonical remote self-host path; relay `/mcp` is the public MCP surface. |
| Self-host cloud control plane | [CLOUD_SELF_HOST.md](CLOUD_SELF_HOST.md) | `make build-cloud` | `make cloud-smoke` | Binary-native proof covers bootstrap, tenancy, provider installs, and optional composed runtime/MCP/SDK proof. |
| Planner / client integrations | [mcp/integrations.md](mcp/integrations.md) | `make build build-cloud build-mcp-sdk-smoke` | `make flagship-planner-smoke` or self-host/cloud smoke with MCP/SDK enabled | ChatGPT-style and Claude Code-style operator lanes are packaged around canonical remote MCP; universal product UI/auth support remains unclaimed. |
| Provider connectors | [CLOUD_CONNECTORS.md](CLOUD_CONNECTORS.md) | `make build-cloud` | `make cloud-smoke` plus provider-focused tests | Slack is strongest today; Jira is polling-first; the rest stay narrow operator/package surfaces. |

For the repo-level supported verification pass:

```bash
make build-supported
make verify-beta
```

Run `make verify-beta-docker` only on a Docker-capable host when you also want the Docker self-host cloud baseline.
