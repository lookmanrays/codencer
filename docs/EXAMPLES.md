# Practical Usage Examples

This guide provides copy-pasteable command sequences for daily local workflows with the Codencer Orchestration Bridge.

## 1. Setup & Environment Health

Before starting, ensure the binaries are built and your environment is sane.

```bash
# Build daemon and CLI
make build

# Verify environment health (checks for .codencer setup and agent binaries)
./bin/orchestratorctl doctor
```

---

## 2. Fast Path: Orchestration Simulation

Use this mode to verify the bridge's state machine and your integration logic without requiring real LLM agents.

### Start the Daemon
```bash
# Start in simulation mode (stubs all adapters)
ALL_ADAPTERS_SIMULATION_MODE=1 ./bin/orchestratord
```

### Submit and Inspect a Task
In a new terminal:
```bash
# 1. Start a new run
./bin/orchestratorctl run start my-run-01 my-project

# 2. Submit a simulated task
# (.codencer/smoke_task.yaml is a good default template)
./bin/orchestratorctl submit my-run-01 .codencer/smoke_task.yaml

# 3. Wait for terminal state (returns JSON on completion)
./bin/orchestratorctl step wait my-step-1

# 4. View simulated logs (will be empty in simulation, but verifies the command)
./bin/orchestratorctl step logs my-step-1
```

---

## 3. Real Work: Codex Execution

Use this mode for real daily coding tasks. Requires the `codex-agent` binary.

### Configuration
```bash
# Set path to your real codex binary if not in $PATH
export CODEX_BINARY=/path/to/codex-agent
```

### Execution Flow
```bash
# 1. Start the real daemon
./bin/orchestratord

# 2. Submit a real task
./bin/orchestratorctl submit run-888 task_fix_bug.yaml

# 3. Tail the agent's output in real-time
./bin/orchestratorctl step logs step-999

# 4. Wait for completion and inspect evidence
./bin/orchestratorctl step wait step-999
```

---

## 4. Inspecting Evidence & Artifacts

The bridge centralizes all evidence in the `.codencer/` directory.

### CLI Inspection
```bash
# View structured outcome evidence
./bin/orchestratorctl step result <stepID> | jq .

# View validation results (tests, linters)
./bin/orchestratorctl step validations <stepID> | jq .

# View all artifact metadata
./bin/orchestratorctl step artifacts <stepID> | jq .
```

### Filesystem Inspection
```bash
# Browse the raw workspace (if worktree was used)
ls -R .codencer/workspace/

# Browse archived artifacts for a specific attempt
# (Path is printed by 'orchestratorctl wait' upon completion)
ls -R .codencer/artifacts/<runID>/<attemptID>/
```

---

## 5. Troubleshooting Cheat Sheet

| Question | Command |
| :--- | :--- |
| **What is the bridge doing?** | `tail -f .codencer/smoke_daemon.log` (if redirected) or check daemon console. |
| **What did the agent say?** | `bin/orchestratorctl step logs <stepID>` |
| **Where are the diffs?** | Look for `unified.diff` in `.codencer/artifacts/<runID>/<attemptID>/`. |
| **Clean state?** | `make nuke` (Deletes DB and all artifacts - USE WITH CAUTION). |

Looking for more? See the full [Troubleshooting Guide](TROUBLESHOOTING.md).

