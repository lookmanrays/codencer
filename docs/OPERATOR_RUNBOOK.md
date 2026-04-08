# Codencer v1 Operator Runbook

This runbook provides the canonical "Day 0" experience for human operators using Codencer.

> [!TIP]
> If you are an **AI Assistant**, **Agentic Planner**, or **Automated Shell Tool**, please refer to the **[AI Operator Guide](AI_OPERATOR_GUIDE.md)** for canonical rules of engagement and machine-safe calling patterns.

## 0. Core Doctrine
Codencer is a **bridge, not a brain**. It handles execution, isolation, and auditability. It does **not** perform its own planning or high-level decision-making.

---

## Phase 1: Canonical Startup

### 1.1 Project Anchoring
Codencer is **repo-bound**. Every daemon instance is anchored to a specific git repository.
```bash
# Navigate to your target project
cd ~/projects/my-awesome-app
```

### 1.2 Start the Daemon
```bash
# Start in Simulation Mode (Default for initial testing)
make start-sim

# OR Start in Real Mode (Requires agent binaries like codex-agent)
make start
```

### 1.3 Identity Verification
Always verify which repository and port the daemon is serving before proceeding.
```bash
./bin/orchestratorctl instance --json
```

---

## Phase 2: Project Targeting & Isolation

### 2.1 The One-Repo-One-Instance Model
Each Codencer daemon instance is anchored to a single `repo_root`. 
- **DB & Artifacts**: All state is stored in `.codencer/` relative to the `repo_root`.
- **Worktrees**: All task attempts are executed in isolated git worktrees managed within the `repo_root`.

### 2.2 Global Identity Check
The `orchestratorctl instance` command identifies which project and port a daemon is serving. Use it to ensure you are targeting the correct bridge instance.

```bash
ORCHESTRATORD_URL=http://localhost:8085 ./bin/orchestratorctl instance --json
```

---

## Phase 3: Submission Flow

Codencer supports multiple ways to submit work. Choose the one that fits your automation or human flow.

### 3.1 Canonical TaskSpec (File)
Best for rich, structured tasks with constraints.
```bash
./bin/orchestratorctl submit my-run examples/tasks/bug_fix.yaml --wait --json
```

### 3.2 Direct Goal (Convenience)
Best for quick, one-off instructions.
```bash
./bin/orchestratorctl submit my-run --goal "Refactor Auth" --adapter codex --wait --json

# Or for OpenClaw agents (Experimental / Alpha):
./bin/orchestratorctl submit my-run --goal "Refactor Auth" --adapter openclaw-acpx --wait --json
```

### 3.3 Stdin Prompt (Multiline)
Best for large, human-readable prompts without temporary files. Use heredocs to provide the goal:

```bash
cat <<'EOF' | ./bin/orchestratorctl submit my-run --stdin --title "Update Docs" --wait --json
Please update the API documentation to reflect the new v1 endpoints. 
Be sure to include examples for all JSON responses.
EOF
```

### 3.4 JSON Task String (Machine-to-Machine)
Best for integrations where the planner generates a structured JSON task object.

```bash
echo '{"version":"v1","goal":"Fix typos in README"}' | ./bin/orchestratorctl submit my-run --task-json - --wait --json
```

---

## Phase 4: Monitoring & Inspection

### 4.1 Inspecting the Authoritative Truth
If you didn't use `--wait`, or once a task is complete, you can check the authoritative evidence at any time.

```bash
# Get the high-level summary and result spec (The Truth)
./bin/orchestratorctl step result <HANDLE>

# Drill down into the agent's brain (stdout/stderr)
./bin/orchestratorctl step logs <HANDLE>

# Inspect the artifacts and proof
./bin/orchestratorctl step artifacts <HANDLE>
./bin/orchestratorctl step validations <HANDLE>
```

### 4.2 Interpreting States
Codencer distinguishes between categories of failure to help you recover:

- **`completed`**: SUCCESS. All goals and validations met.
- **`failed_validation`**: GOAL FAILURE. Agent finished fine, but your defined tests/lint failed.
- **`failed_terminal`**: GOAL FAILURE. Agent finished but explicitly reported it could NOT meet the goal.
- **`failed_adapter`**: AGENT CRASH. The agent binary itself failed (check `logs`).
- **`failed_bridge`**: SYSTEM FAILURE. System error (Disk Full, Git Error, Provisioning).
- **`timeout`**: KILLED. Execution exceeded `timeout_seconds`.
- **`cancelled`**: STOPPED. Manually aborted by player/operator.

---

## Phase 5: First Flight (Validation Scenario)

To verify your setup is fully functional, run the internal version bump scenario:

1. **Start the daemon**: `make start` (or `make start-sim`).
2. **Run validation**:
   ```bash
   make validate
   ```
This will automatically submit a task to update `internal/app/version.go` and report the outcome.

---

## Phase 6: Lifecycle Management

### 6.1 Managing Multiple Projects
To serve Project A and Project B simultaneously:
```bash
# Terminal 1
./scripts/start_instance.sh ~/projects/project-a 8085

# Terminal 2
./scripts/start_instance.sh ~/projects/project-b 8086
```

### 6.2 Stopping the Daemon
```bash
make stop
```

### 6.3 Cleanup (Nuke)
**CAUTION**: This deletes all local history and worktrees.
```bash
make nuke
```
