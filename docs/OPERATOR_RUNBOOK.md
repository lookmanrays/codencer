# Codencer v1 Operator Runbook

This runbook provides the canonical "Day 0" experience for operating Codencer v1. It is designed for both human operators and shell-capable AI assistants.

## 0. Core Doctrine
Codencer is a **bridge, not a brain**. It handles execution, isolation, and auditability. It does **not** perform its own planning or high-level decision-making.

---

## Phase 1: Canonical Startup

### 1.1 Choose Your Repository
Codencer is **repo-bound**. You must target a specific git repository for each daemon instance.

```bash
# Navigate to your target project
cd ~/projects/my-awesome-app
```

### 1.2 Start the Daemon
You can start the daemon in **Simulation Mode** (to test the bridge) or **Real Mode** (to run actual agents).

```bash
# Start in Simulation Mode (Default for initial testing)
make start-sim

# OR Start in Real Mode (Requires agent binaries like codex-agent)
make start
```

### 1.3 Verify the Instance
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

# Or for OpenClaw agents:
./bin/orchestratorctl submit my-run --goal "Refactor Auth" --adapter openclaw-acpx --wait --json
```

### 3.3 Stdin Prompt (Multiline)
Best for large, human-readable prompts without temporary files.
```bash
cat <<EOF | ./bin/orchestratorctl submit my-run --stdin --title "Update Docs" --wait --json
Please update the API documentation to reflect the new v1 endpoints. 
Be sure to include examples for all JSON responses.
EOF
```
```

### 3.4 JSON Task String (Machine-to-Machine)
Best for integrations where the planner generates a JSON task object.
```bash
echo '{"version":"v1","goal":"Fix typos in README"}' | ./bin/orchestratorctl submit my-run --task-json - --wait --json
```
```

---

## Phase 4: Monitoring & Inspection

### 4.1 Polling for Results
If you didn't use `--wait`, you can check the authoritative evidence at any time.

```bash
# Get the high-level summary and state
./bin/orchestratorctl step result <UUID>

# Drill down into the agent's brain (stdout)
./bin/orchestratorctl step logs <UUID>

# Inspect the proof (test/lint results)
./bin/orchestratorctl step validations <UUID>
```

### 4.2 Interpreting States
- **`completed`**: SUCCESS. All goals and validations met.
- **`failed_validation`**: GOAL FAILURE. Agent finished, but tests/lint failed.
- **`failed_adapter`**: CRASH. The agent binary failed (check `logs`).
- **`failed_bridge`**: SYSTEM FAILURE. Check disk space or git locks.
- **`timeout`**: KILLED. Execution exceeded time limits.

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
