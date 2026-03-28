**Defensive, Local-First Relay for Tactical Coding Agents.**

Codencer is a persistent orchestration daemon designed to securely manage, execute, validate, and audit coding tasks performed by external agents. It acts as the **system of record** between a high-level **Planner** (human or LLM) and tactical **Coding Agents** (Codex, Claude-code, Aider). It is designed for **local-first, self-hosted developer toolchains.**

> [!IMPORTANT]
> **Project Status: MVP / Public Beta**. Codencer is technically functional for local dev use, but the API and protocols are subject to change. The primary, most-hardened execution path is via the **Codex** adapter.

---

## 🏛 The Relay Model

Codencer is a **bridge, not a brain**. It does not decide the high-level strategy; it executes tactical instructions and reports high-fidelity evidence.

```text
[ Planner (Brain) ] <---------- (ResultSpec) ---------+
       |                                              |
   (TaskSpec)                                   [ Bridge (Codencer) ]
       |                                              |
       +-------------------> [ Agent (Worker) ] <-----+
                              (File Edits)
```

### Core Roles
- **Planner**: You, a Chat UI, or an agentic planner. Decides **what** to do.
- **Bridge (Codencer)**: Receives the `TaskSpec`, manages workspace isolation (Git Worktrees), enforces policies, and monitors execution.
- **Coding Agent**: The underlying tactical tool performing the actual work (e.g., `codex-agent`, `claude-code`).

For the definitive Day-0 guide, see the **[Canonical Local Runbook](docs/EXAMPLES.md)**.

---

## ⚡️ Quickstart: Local Setup

Get up and running in simulation mode to verify the orchestrator logic.

### 1. Build & Setup
```bash
# Initialize and build binaries
make setup build

# (Optional) Verify your local environment
./bin/orchestratorctl doctor
```

### 2. Start the Daemon
Choose your execution tier in `.env` (Simulation is enabled by default in `.env.example`):
```bash
# Start in Simulation Mode (Background)
make start-sim

# OR Start in Real Mode (Requires agent binaries like codex-agent)
# Edit .env: ALL_ADAPTERS_SIMULATION_MODE=0
make start
```

### 3. Run Your First Task
Submit a task and wait for the bridge to report results:
```bash
# 1. Start a new orchestration run (System of Record)
./bin/orchestratorctl run start first-run my-project

# 2. Submit a tactical task and wait for completion
./bin/orchestratorctl submit first-run examples/tasks/bug_fix.yaml --wait

# 3. Inspect the final outcome
./bin/orchestratorctl step result <stepID>
./bin/orchestratorctl step logs <stepID>
./bin/orchestratorctl step artifacts <stepID>
```

---

## 🔍 Interpreting Outcomes

The Bridge reports high-fidelity evidence for every attempt. Note that **the bridge is a relay**, not a decision-maker; once a terminal state is reached, control returns to the operator/planner to decide the next move.

- **`completed`**: Goal met, all tests passed.
- **`completed_with_warnings`**: Success, but with non-critical issues (lint/tests).
- **`failed_terminal`**: Goal not met (e.g. tests failed). Review validations.
- **`timeout`**: Execution exceeded limits. Review logs for hangs.
- **`cancelled`**: Manually stopped by the operator.
- **`needs_approval`**: Policy gate hit; awaiting operator intervention.
- **`needs_manual_attention`**: System ambiguity or crash. Review daemon/agent logs.

### Auditing the Evidence
Every task execution leaves a permanent audit trail:
1. **Summary**: Run `./bin/orchestratorctl step result <id>` for the high-level spec.
2. **Logs**: Run `./bin/orchestratorctl step logs <id>` for the raw agent stdout.
3. **Artifacts**: Every modified file and diff is stored in `.codencer/artifacts/`. Use `./bin/orchestratorctl step artifacts <id>` to see the exact paths and SHA-256 hashes.
4. **Validations**: Run `./bin/orchestratorctl step validations <id>` to see specific test/lint results.

For a deeper dive into agent installation and advanced configuration, see the **[Environmental Reference Guide](docs/SETUP.md)**.

---

## 🛡 Why Codencer?

Agent-driven coding is non-deterministic. Codencer provides the guardrails:

1. **Workspace Safety**: Agents run in isolated Git Worktrees. Diffs are captured and validated before any commit.
2. **Audit-Proof Ledger**: Every attempt is recorded in a local SQLite database with SHA-256 hashes of all artifacts.
3. **Idempotency**: Interrupted tasks can be resumed or securely analyzed post-crash.
4. **Validation-First**: Tasks only "complete" when your defined validation commands (tests, linters) pass.

---

## ⚠️ Known Limitations (Beta/MVP)

As a local-first Beta/MVP, Codencer has the following constraints:
- **Relay Only**: The bridge does not "think" or plan; it only executes what the Planner instructs.
- **Single-User**: Designed for local development; no multi-user or cloud concurrency.
- **Agent Dependency**: "Real Mode" efficacy is strictly bound to the quality of the underlying agent (Codex, Claude, etc.).
- **Manual Decisions**: The bridge reports terminal states; all recovery or retry decisions remain with the human operator or external planner.

---

## 📊 Maturity & Capability Matrix

Codencer is currently in an **MVP/Beta** state. Use the following matrix to understand current support:

| Feature Area | Status | Description |
| :--- | :--- | :--- |
| **Orchestration Core** | ✅ **Ready (Beta)** | Persistent SQLite ledger, state machine, and Git Worktrees. |
| **CLI & MCP Layer** | ✅ **Ready (Beta)** | Structured JSON outputs, log tailing, and health checks. |
| **Codex Adapter** | ✅ **Ready (Beta)** | High-fidelity relay for the `codex-agent` binary. |
| **Claude/Qwen Adapters** | 🟡 **Functional** | Basic subprocess wrappers; lacks deep artifact extraction. |
| **Simulation Mode** | ✅ **Ready (Beta)** | Robust stubs for orchestrator validation without LLM use. |
| **Diagnostics & Health**| ✅ **Ready (Beta)** | CLI-based `doctor` and `smoke` verification tools. |
| **IDE Chat Bridge** | 🧪 **Prototype** | Experimental proxy-mediated file access via VS Code. |
| **Adaptive Routing** | 📅 **Blueprint** | Heuristic fallback is implemented; dynamic optimization is a blueprint. |
| **Cloud / Multi-User** | 🚫 **Non-Goal** | Codencer is strictly local-first and self-hosted. |

## 🧪 Simulation vs. Real Execution

1. **Simulation Mode** (`make start-sim`): Only validates the **Orchestrator**. It tests if the ledger, state machine, and CLI are working. It does **not** test if the agent can actually code.
2. **Real Mode**: Tests the full end-to-end loop with real agents. **Codex-agent** is the primary supported path; others are in early beta.

---

## 📖 Documentation

Review the following guides to get started with Codencer.

### ⚡️ User Guidance (Start Here)
- **[Canonical Local Runbook](docs/EXAMPLES.md)** — The definitive Day-0 operator flow.
- **[Environmental Reference](docs/SETUP.md)** — Prerequisites, configuration, and agent setup.
- **[Troubleshooting](docs/TROUBLESHOOTING.md)** — How to handle non-success states and recovery.
- **[Architecture Overview](docs/02_architecture.md)** — High-level design and the "Bridge not Brain" model.

### 🛠 Project Governance & Maintenance (Internal)
- **[Gap Audit & Roadmap](docs/internal/GAP_AUDIT.md)** — Current V1 release blockers and debt.
- **[Development Progress](docs/internal/PROGRESS.md)** — Historical and current technical timeline.
- **[Technical Task Backlog](docs/internal/TASKS.md)** — Detailed micro-task status for maintainers.
- **[Contributing Guide](CONTRIBUTING.md)** — How to set up a dev environment and submit PRs.

---

## ⚖ License

Codencer is released under the **MIT License**. See the [LICENSE](LICENSE) file for the full text.
