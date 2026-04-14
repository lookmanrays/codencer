# Environmental Reference & Setup

This guide provides the technical baseline for running the Codencer Orchestration Bridge.

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

## 2. Getting Started (Canonical Path)

### 2.1 Clone & Build
```bash
git clone https://github.com/lookmanrays/codencer
cd codencer

# 1. Initialize environment and check requirements
make setup

# 2. Build the canonical daemon, CLI, connector, and relay binaries
make build

# 3. Build the Windows-side agent-broker separately if you need it
make build-broker
```

### 2.2 Verify Environment
The `doctor` tool verifies if your environment is ready for tactical execution.
```bash
./bin/orchestratorctl doctor
```

---

## 3. Daemon Management

The `orchestratord` is the persistent system of record. It must be running to receive tasks.

### 3.1 Simulation Mode (Orchestrator Validation)
Use this mode to test your local setup, CLI, and local daemon surfaces without consuming LLM credits or requiring agent binaries.
```bash
make start-sim
```

### 3.2 Real Mode (Tactical Execution)
Use this mode for real-world tasks. It requires agents like `codex-agent` or `claude` to be installed.
```bash
# Edit .env to set ALL_ADAPTERS_SIMULATION_MODE=0
make start
```

Claude is executed in headless print mode as `claude -p --output-format json`. Codencer builds the task prompt, writes it to `prompt.txt`, delivers it on `stdin`, and runs the process from the attempt workspace root.

> [!IMPORTANT]
> The daemon-local `/mcp/call` endpoint is only a local compatibility/admin surface. The canonical remote MCP surface for planners lives on the relay at `/mcp`.

> [!IMPORTANT]
> For the practical self-host relay path, the canonical public binaries are `codencer-connectord` and `codencer-relayd`. The Windows-side `agent-broker` binary is built separately with `make build-broker` because `cmd/broker` is a nested module.

---

## 4. Daemon Management & Targeting
Codencer follows a **One-Repo-One-Instance** model. Each repo clone manages its own database and worktrees.

### 4.1 Explicit Repo Targeting
To anchor a daemon to a specific repository regardless of your current directory, use the `--repo-root` flag.

```bash
# Anchor the daemon to a specific repo root
./bin/orchestratord --repo-root /path/to/my-project
```

### 4.2 Port Management
The daemon listens on port `8085` by default. To run multiple instances on the same machine, use the `PORT` environment variable:

```bash
# Start an instance on a custom port
PORT=8086 ./bin/orchestratord --repo-root /path/to/project-b
```

### 4.3 Startup Helper
Use the provided script to start and build a daemon instance for a specific project:

```bash
# Usage: ./scripts/start_instance.sh <repo_root> [port] [extra_flags]
./scripts/start_instance.sh ~/projects/my-api 8085
```

### 4.4 Environment Variables
Codencer uses these variables to locate agent binaries and target the daemon:
- `CODEX_BINARY`: Path to the `codex-agent` binary.
- `CLAUDE_BINARY`: Path to the `claude` binary. Defaults to `claude`.
- `OPENCLAW_ACPX_BINARY`: Path to the `acpx` CLI (for OpenClaw support).
- `ORCHESTRATORD_URL`: URL of the daemon (default: `http://localhost:8085`).

### 4.5 Claude Adapter Notes
- Install the Claude CLI so the `claude` binary is available on your `$PATH`, or point `CLAUDE_BINARY` at the full path.
- Codencer does not pass a workspace flag to Claude. The attempt workspace is supplied via process `cwd`.
- Claude raw output is preserved in `stdout.log`; Codencer parses that JSON and synthesizes the normalized `result.json`.

---

## 5. OpenClaw Setup (Experimental / Alpha)

Codencer provides experimental support for the **Agent Client Protocol (ACP)** via the OpenClaw adapter. This integration is currently in **Alpha** and is intended for early-access testing of OpenClaw-compatible executors.

- **Adapter ID**: `openclaw-acpx`
- **Binary**: `acpx` (Agent Client Protocol CLI).
- **Local Runtime**: A running OpenClaw-compatible backend or agent stack must be discoverable by `acpx`.

To configure a custom path for the ACPX binary:
```bash
# Add to your .env or export directly
export OPENCLAW_ACPX_BINARY=/path/to/custom/acpx
```

> [!WARNING]
> **OpenClaw support is Experimental (Alpha)**.
> Codencer acts strictly as a **bridge**. It manages the `acpx` process lifecycle and workspace isolation, but it does **not** manage model routing, API keys, or backend selection for OpenClaw. Configure those directly via the OpenClaw/acpx configuration on your host machine.

---

## 6. Workspace Provisioning
Codencer isolates every task attempt in a dedicated Git worktree. You can configure how these worktrees are prepared using `.codencer/workspace.json`.

### Example `.codencer/workspace.json`
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

### Grove Compatibility
Codencer optionally reads an environment-prep subset of Grove config (`grove.yaml` or `.groverc.json`) if a native config is missing. 

> [!IMPORTANT]
> Codencer does **not** depend on the Grove CLI and is designed to coexist with existing Grove setups.

For advanced provisioning examples, see **[EXAMPLES.md](EXAMPLES.md)**.

---

## 6. Automation-Friendly Submission Inputs

`orchestratorctl submit` supports both rich canonical task definitions and narrow direct convenience input.

Use a full YAML or JSON `TaskSpec` when you need rich structure such as constraints, path controls, or custom validation layouts.

Use direct convenience input when a shell wrapper, planner, or local script needs a deterministic way to submit one task without authoring YAML first:
- `--task-json <path|->`
- `--prompt-file <path>`
- `--goal <text>`
- `--stdin`

Exactly one primary source is required.

Direct convenience input stays intentionally narrow. It deterministically normalizes into the canonical `TaskSpec` used by the daemon and preserves both:
- `original-input.*`
- `normalized-task.json`

For concrete submit examples, see **[EXAMPLES.md](EXAMPLES.md)**. For planner- and wrapper-oriented examples, see **[CLI_AUTOMATION.md](CLI_AUTOMATION.md)**.

The official v1 ordered-task model is wrapper-based. Use the scripts in `examples/automation/` when you need to execute an explicit ordered list one item at a time.

---

## 7. Agent Broker (Cross-Side Execution)

Use the `agent-broker` for **cross-side execution** (e.g., Codencer in WSL controlling Antigravity in Windows).

### 7.1 Broker Execution Model
The broker uses a **dual-path model**:
- **Repo Root (Identity)**: The stable path used to bind this repository to an active IDE instance.
- **Workspace Root (Execution)**: The isolated worktree path where the task is actually executed.

### 7.2 Setup & Binding
1.  **Build and Start the Broker**: Run `make build-broker`, then start the resulting `agent-broker` binary on the host machine.
2.  **Bind**: Link your local repository to a running IDE instance:
    ```bash
    ./bin/orchestratorctl antigravity bind <PID>
    ```
3.  **Execute**: Submit tasks using the current `antigravity-broker` adapter name against the `agent-broker` bridge:
    ```bash
    ./bin/orchestratorctl submit <runID> --goal "Check UI" --adapter antigravity-broker --wait
    ```

For detailed examples, see **[EXAMPLES.md](EXAMPLES.md)**.

## 8. Self-Host Smoke Path

After the daemon and relay are running, you can exercise the current happy path with:

```bash
PLANNER_TOKEN=<planner-token> make self-host-smoke
```

This helper enrolls a temporary connector, waits for the shared instance to appear on the relay, starts a run, submits a task, waits for the step, and fetches the result, validations, logs, gates, and artifacts through the relay.

Optional scenario coverage:

```bash
PLANNER_TOKEN=<planner-token> make self-host-smoke-mcp
PLANNER_TOKEN=<planner-token> make self-host-smoke-all
```

`make self-host-smoke-mcp` includes the official Go SDK proof path via `cmd/mcp-sdk-smoke`. `make self-host-smoke-all` adds share-control and multi-instance coverage.

If you want the standalone proof helper, build and run it directly:

```bash
make build-mcp-sdk-smoke
./bin/mcp-sdk-smoke --endpoint http://127.0.0.1:8090/mcp --token <planner-token> --instance-id <instance-id>
```

## 9. Practical Self-Host Order Of Operations

For a fresh self-host setup:

```bash
make build
mkdir -p .codencer/relay
./bin/codencer-relayd planner-token create --config .codencer/relay/config.json --write-config --name operator --scope '*'
./bin/codencer-relayd --config .codencer/relay/config.json
make start
./bin/codencer-relayd enrollment-token create --config .codencer/relay/config.json --label local-dev --json
./bin/codencer-connectord enroll --relay-url http://127.0.0.1:8090 --daemon-url http://127.0.0.1:8085 --enrollment-token <token>
./bin/codencer-connectord run
./bin/codencer-connectord discover --config .codencer/connector/config.json
./bin/codencer-connectord list --config .codencer/connector/config.json
./bin/codencer-connectord share --config .codencer/connector/config.json --daemon-url http://127.0.0.1:8085
./bin/codencer-connectord unshare --config .codencer/connector/config.json --instance-id <instance-id>
./bin/codencer-relayd instances --config .codencer/relay/config.json
./bin/codencer-relayd audit --config .codencer/relay/config.json --limit 20
```

For the practical WSL-first topology, keep the daemon and connector in WSL/Linux next to the repo and worktrees, keep the agent-broker on Windows when Antigravity is in play, and expose the relay instead of the daemon.
