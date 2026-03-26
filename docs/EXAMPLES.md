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
# Start in background (recommended)
make start-sim

# OR start in foreground
make simulate
```

### 2. Submit a Test Task
In a new terminal:
```bash
# Submit a realistic task found in the examples directory
./bin/orchestratorctl run start verify-run verify-proj
./bin/orchestratorctl submit verify-run examples/tasks/bug_fix.yaml
```

### 3. Monitor & Wait
```bash
# Wait for terminal state (should complete in ~5 seconds in simulation)
./bin/orchestratorctl step wait <stepID>
```
Once finished, the bridge will echo the **Summary**, **Logs path**, and **Artifacts directory**.

### 4. Inspect the Result
```bash
# Verify the 'completed' state and captured metadata
./bin/orchestratorctl step result <stepID>
```

---

## 🔍 Audit & Troubleshooting Flow

If a task fails or yields unexpected results, follow this audit trail:

### 1. View Execution Logs
See exactly what the agent saw and did in its terminal session.
```bash
./bin/orchestratorctl step logs <stepID>
```

### 2. Verify Success (The "Proof")
A successful task should result in:
1. **`completed` State**: Seen in `./bin/orchestratorctl step result <id>`.
2. **`passed` Validations**: Seen in `./bin/orchestratorctl step validations <id>`.
3. **Modified Files**: Seen in the `./bin/orchestratorctl step artifacts <id>` list.

### 3. Inspect Evidence (Artifacts)
Codencer captures all generated files, diffs, and metadata in the local vault.
```bash
# List all artifacts for a step
./bin/orchestratorctl step artifacts <stepID>
```
Every artifact includes a **SHA-256 hash** for cryptographic integrity.
Artifacts are stored on disk at: `.codencer/artifacts/<runID>/<stepID>/...`

### 4. Verify Correctness (Validations)
Check which specific verification commands (tests, linters) passed or failed.
```bash
./bin/orchestratorctl step validations <stepID>
```

### 4. Policy Gates (Intervention)
If the bridge pauses for a gate, identify the reason and approve:
```bash
# Check gate state and reason
./bin/orchestratorctl run state <runID>

# Approve the sensitive action
./bin/orchestratorctl gate approve gate-<stepID>
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
# Inspect the agent's current progress (snapshot)
./bin/orchestratorctl step logs <stepID>
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
## 🛠 Recovery from Non-Success

The bridge reports what happened; you decide what to do next.

### Example: Recovering from `failed_terminal`
If a task finishes with `failed_terminal` (e.g. tests failed):
1. **Audit**: Run `./bin/orchestratorctl step validations <stepID>` to see which specific test failed.
2. **Diagnose**: Run `./bin/orchestratorctl step logs <stepID>` to see the agent's error messages.
3. **Decide**: 
   - If the instructions were unclear: Update your `task.yaml`.
   - If the agent made a silly mistake: Add a hint to the `goal` or `instructions`.
4. **Resubmit**: Just run the `submit` command again in the same run.
   ```bash
   ./bin/orchestratorctl submit <runID> examples/tasks/bug_fix.yaml --wait
   ```

### Example: Responding to `timeout`
If a task times out:
1. **Check Logs**: See if the agent was actually making progress or just hanging.
2. **Adjust**: If it was making progress but slow, increase `timeout_seconds` in the task YAML.
3. **Resubmit**: Run the task again; the bridge will use a fresh worktree.

## 📖 Further Reading
- [Setup & Self-Hosting Guide](SETUP.md)
- [Troubleshooting Guide](TROUBLESHOOTING.md)
- [Architecture Overview](02_architecture.md)
