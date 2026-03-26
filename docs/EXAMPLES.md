# Practical Usage Examples

This guide provides copy-pasteable command sequences and role-based workflows for daily local use.

## Core Roles
- **Planner (Brain)**: You (or an LLM) decide *what* to do next.
- **Bridge (Codencer)**: Executes the instruction, polls for completion, and reports evidence.
- **Coding Agent (Worker)**: The underlying tool (Codex, Claude, etc.) that performs the file edits.

---

## 0. Daily Local Workflow

This is the standard inner-loop for using the bridge with a planner.

### Step 1: Start the Bridge (Daemon)
The bridge must be running to receive and execute tasks.

```bash
# Real Mode (Executes actual code changes via Codex/Claude)
./bin/orchestratord

# OR Simulation Mode (Verifies orchestrator logic without edits)
# ALL_ADAPTERS_SIMULATION_MODE=1 ./bin/orchestratord
```

### Step 2: Submit a Task (Planner Decision)
The **Planner** issues a declarative `TaskSpec` (YAML) to the bridge.

```bash
# Start a session (Run) if not already active
./bin/orchestratorctl run start daily-fix my-project

# Submit the instruction
./bin/orchestratorctl submit daily-fix examples/tasks/bug_fix.yaml
```

### Step 3: Monitor & Report (Bridge Execution)
The **Bridge** handles the tactical execution and provides real-time progress.

```bash
# Tail the agent's output as it works
./bin/orchestratorctl step logs step-fix-nil-err

# Wait for the bridge to reach a terminal state
./bin/orchestratorctl step wait step-fix-nil-err
```

### Step 4: Final Insight (Planner Review)
The **Planner** inspects the bridge's report to decide the next action.

```bash
# Inspect the structured result and validation outcome
./bin/orchestratorctl step result step-fix-nil-err | jq .
```

- **If `completed`**: The Planner proceeds to the next high-level goal.
- **If `failed` or `needs_manual_attention`**: The Planner (or human) reviews logs and submits a corrective follow-up step.

---

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

## 3. Realistic Task Library

A set of realistic, planner-ready `TaskSpec` templates is available in the `examples/tasks/` directory:

- **[bug_fix.yaml](../examples/tasks/bug_fix.yaml)**: Small code fix with build validation.
- **[docs_only.yaml](../examples/tasks/docs_only.yaml)**: Documentation-only update with strict path constraints.
- **[config_update.yaml](../examples/tasks/config_update.yaml)**: Internal configuration change.
- **[simulation_task.yaml](../examples/tasks/simulation_task.yaml)**: Template for verifying orchestrator logic.

Submit these using:
```bash
./bin/orchestratorctl submit <runID> examples/tasks/bug_fix.yaml
```

---

## 4. Real Work: Codex Execution

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

