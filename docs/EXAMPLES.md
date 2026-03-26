# Codencer Self-Host Runbook

This guide provides concrete, command-based sequences for operating the bridge locally.

---

## 🏛 The Self-Host Model

Codencer is the **Defensive Relay** in your local toolchain.

1. **Planner Decides**: Issues a `TaskSpec` (YAML).
2. **Bridge Executes**: Manages workspace, polls agent, captures artifacts.
3. **Agent Performs**: Does the tactical file edits (Codex, Claude, etc.).

---

## ⚡️ Flow A: The 30-Second Simulation (Verification)
Use this flow to verify the orchestrator's state machine without requiring LLMs or agent binaries.

### 1. Start the Simulated Bridge
```bash
# ALL_ADAPTERS_SIMULATION_MODE=1 stubs all agent execution
make simulate
```

### 2. Submit a Test Task
In a new terminal:
```bash
# Submit the built-in smoke task
./bin/orchestratorctl submit smoke-run .codencer/smoke_task.yaml
```

### 3. Monitor & Wait
```bash
# Wait for terminal state (should complete in ~5 seconds)
./bin/orchestratorctl step wait smoke-step-1
```

### 4. Inspect the Result
```bash
# Verify the 'completed' state and captured metadata
./bin/orchestratorctl step result smoke-step-1 | jq .
```

---

## 🛠 Flow B: The Real Codex Loop (Tactical Fix)
Use this flow for actual daily coding tasks. Requires `codex-agent` installed.

### 1. Configuration
```bash
# Export the path if not in your $PATH
export CODEX_BINARY=codex-agent
```

### 2. Start the Real Bridge
```bash
./bin/orchestratord
```

### 3. Submit a Realistic Fix
```bash
./bin/orchestratorctl run start fixer-01 my-project
./bin/orchestratorctl submit fixer-01 examples/tasks/bug_fix.yaml
```

### 4. Tail the Agent (Live)
```bash
# Watch the agent's stdout as it works
./bin/orchestratorctl step logs step-fix-nil-err
```

### 5. Review & Apply
Once `wait` returns:
```bash
# Inspect the diff harvested by the bridge
ls -R .codencer/artifacts/fixer-01/
cat .codencer/artifacts/fixer-01/*/unified.diff
```

---

## 🔍 Flow C: Artifact & Ledger Audit
How to inspect the "System of Record" for an attempt.

### 1. List All Runs
```bash
./bin/orchestratorctl run list
```

### 2. Inspect Step Validations
```bash
# See which tests passed or failed as seen by the bridge
./bin/orchestratorctl step validations <stepID> | jq .
```

### 3. Inspect Captured Artifacts
Every file modified by an agent is hashed and archived locally.
```bash
./bin/orchestratorctl step artifacts <stepID> | jq .
```

---

## 🧹 Flow D: Resetting the Lab
To completely clear your local environment and start fresh:

```bash
# WARNING: This deletes the SQLite database and all artifacts in .codencer/
make nuke
```

---

## 📖 Further Reading
- [Setup & Self-Hosting Guide](SETUP.md)
- [Troubleshooting Guide](TROUBLESHOOTING.md)
- [Architecture Overview](02_architecture.md)
