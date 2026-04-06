# Environmental Reference & Setup

This guide provides the technical baseline for running the Codencer Orchestration Bridge.

## 1. Prerequisites

### Software Requirements
- **Git**: Required for worktree isolation.
- **Go 1.21+**: Required to build the daemon and CLI.
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
git clone https://github.com/verbaux/codencer
cd codencer

# 1. Initialize environment and check requirements
make setup

# 2. Build orchestratord and orchestratorctl binaries
make build
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
Use this mode to test your local setup, CLI, and MCP layers without consuming LLM credits or requiring agent binaries.
```bash
make start-sim
```

### 3.2 Real Mode (Tactical Execution)
Use this mode for real-world tasks. It requires agents like `codex-agent` or `aider` to be installed.
```bash
# Edit .env to set ALL_ADAPTERS_SIMULATION_MODE=0
make start
```

---

## 4. Multi-Instance Workflows
Codencer follows a **One-Repo-One-Instance** model. Each repo clone manages its own database and worktrees.

To run multiple instances on one machine:
1. Ensure each instance has a unique port.
2. Set the `PORT` environment variable.

```bash
# Start an instance on port 8086
PORT=8086 ./bin/orchestratord
```

Check instance identity:
```bash
./bin/orchestratorctl instance
```

---

## 5. Workspace Provisioning
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
