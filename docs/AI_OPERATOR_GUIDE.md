# AI Operator Guide: Rules of Engagement

This guide is the canonical instruction set for **AI Assistants**, **Agentic Planners**, and **Automated Shell Tools** operating the Codencer Bridge. It ensures high-fidelity execution, consistent state handling, and reliable audit trails.

---

## 🏛 The Bridge Doctrine

Codencer is a **Tactical Orchestration Bridge**, not a strategic planner. It handles the **Execution Layer** (isolation, provisioning, monitoring, and evidence) while you handle the **Brain Layer** (planning, task decomposition, and decision-making).

1.  **Bridge, Not Brain**: Do not expect the bridge to plan your next move or recursively fix its own failures. It executes precisely what you submit in a `TaskSpec`.
2.  **Explicit Context**: Always verify the daemon's project and port before taking action.
3.  **Atomic Evidence**: Every task attempt is isolated. Success or failure is reported as a terminal state with immutable artifacts.

---

## 🛠 Phase 1: Instance Discovery

Always verify the daemon's identity to ensure you are targeting the correct repository and execution mode.

```bash
# Verify the current instance
./bin/orchestratorctl instance --json
```

**Expected JSON Response:**
```json
{
  "version": "v0.1.0-beta",
  "repo_root": "/home/user/my-project",
  "execution_mode": "REAL",
  "port": 8085
}
```

---

## ⚡️ Phase 2: Atomic Submission

Use `submit --wait --json` for a synchronous hand-off. This simplifies your control flow by blocking until a terminal state is reached.

### Pattern: The Direct Input Loop
Ideal for human-in-the-loop or iterative assistant tasks.

```bash
cat <<'EOF' | ./bin/orchestratorctl submit my-run-id --stdin --title "Fix Lints" --adapter codex --wait --json
Fix all lint errors in the internal/app package. 
Exclude the test files. 
EOF
```

### Pattern: The Machine-to-Machine JSON Hand-off
Ideal for planners that generate structured `TaskSpec` objects.

```bash
echo '{"version":"v1","goal":"Update README","title":"Update docs"}' | \
  ./bin/orchestratorctl submit my-run-id --task-json - --wait --json
```

---

## 🔍 Phase 3: Auditing Terminal States

Analyze the JSON payload from `submit` to decide your next move.

| State | Action Required by AI Planner |
| :--- | :--- |
| `completed` | **Success**. Move to the next task in your plan. |
| `failed_validation` | **Goal Failure**. Read `step validations` and fix your instructions/logic. |
| `failed_terminal` | **Goal Failure**. Read `step result` summary. The agent finished but did not meet the goal. |
| `failed_adapter` | **Infrastructure**. The agent crashed. Check `step logs` for API errors or OOM. |
| `failed_bridge` | **Infrastructure**. System error. check daemon/disk/git locks. |
| `timeout` | **Infrastructure**. Task took too long. Increase `timeout_seconds` in follow-up. |

### Drill-down Commands
When a task fails, use these to gather evidence before retrying:
```bash
# 1. Why did it fail? (Summary)
./bin/orchestratorctl step result <UUID>

# 2. What were the specific test failures?
./bin/orchestratorctl step validations <UUID>

# 3. What was the agent's raw thought process?
./bin/orchestratorctl step logs <UUID>
```

---

## 🛡 Performance Best Practices for AI

1.  **Use ID Namespacing**: Use clear `RunID` prefixes (e.g., `feature-fix-auth`) to group related steps.
2.  **Narrow Scopes**: Avoid "Fix everything" prompts. Break work into small, verifiable goals.
3.  **Mandatory Validations**: Always include at least one `--validation` command (e.g., `make test` or `go build`) to ensure the agent didn't break the build.
4.  **Audit the Diff**: Use `./bin/orchestratorctl step artifacts <UUID>` to verify the exact changes before finalizing.

---

**Protocol Note**: If any command fails with `error connecting to orchestratord`, the daemon is likely down. Inform the user they need to run `make start` or check their port configuration.
