# Codencer: The Tactical Orchestration Bridge

Codencer is a persistent orchestration daemon designed to securely manage, execute, validate, and audit coding tasks performed by external agents. It acts as the **system of record** between a high-level **Planner** (human or LLM) and tactical **Coding Agents** (Codex, Claude-code, Aider). It is designed for **local-first, self-hosted developer toolchains.**

> [!IMPORTANT]
> **Project Status: Public Beta (v0.1.0-beta)**.
> Codencer is technically functional for local dev use, with a hardened execution path via the **Codex** adapter. While the core engine is stable, the API and protocols are subject to refinement as we gather community feedback.

---

### Core Guarantees

- **Step-Isolation**: Each step executes in its own git worktree, preventing cross-task interference.
- **Immutable Evidence**: All logs, results, and artifacts are namespaced by Run, Step, and Attempt ID under `.codencer/artifacts/<run-id>/<step-id>/<attempt-id>/`, ensuring full auditability of repeated attempts.

> **Execution Path Note**: Codencer depends on Git Worktrees for isolating task attempts. Therefore, cloning the repository via `git clone` is the **only supported execution path**. Downloading a ZIP source archive will fail during targeted execution.

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

### 3. Run Your First Tactical Task
Submit a task and wait for the bridge to report results. For the full auditing sequence, see the **[Canonical Local Runbook](docs/EXAMPLES.md)**.

```bash
# 1. Start a new mission (System of Record)
./bin/orchestratorctl run start first-run my-project

# 2. Submit a tactical task and wait for completion
./bin/orchestratorctl submit first-run examples/tasks/bug_fix.yaml --wait

# 3. View the Authoritative Truth (The Summary)
# Note: Use the Step UUID Handle printed after submission
./bin/orchestratorctl step result <UUID>
```

---

## 🔍 The Audit Trail (Authoritative Evidence)

Codencer ensures that every tactical execution is backed by high-fidelity evidence. Follow the **Canonical Sequence** in `EXAMPLES.md` to audit your task:

1.  **Authoritative Summary**: `step result <UUID>` (Start here).
2.  **Raw Execution Trail**: `step logs <UUID>` (The agent's brain).
3.  **Audit Evidence**: `step artifacts <UUID>` and `step validations <UUID>` (The proof).

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
2. **Audit-Proof Ledger**: Every attempt is recorded in a local SQLite database (embedded via CGO) with SHA-256 hashes of all artifacts.
3. **Idempotency**: Interrupted tasks can be resumed or securely analyzed post-crash.
4. **Validation-First**: Tasks only "complete" when your defined validation commands (tests, linters) pass.

---

## ⚠️ Known Limitations (Beta/MVP)

As a local-first Beta/MVP, Codencer has the following constraints:
- **Relay Only**: The bridge does not "think" or plan; it only executes what the Planner instructs.
- **Single-User**: Designed for local development; no multi-user or cloud concurrency.
- **Static Extension Routing**: The experimental VS Code extension assumes the daemon binds at `127.0.0.1:8085`. Dynamic connection configuration for running instances on multiple ports is not yet natively surfaced in the IDE client.
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
| **Instance Identity** | ✅ **Ready (Beta)** | One-repo-one-daemon model with explicit `instance` inspection. |
| **Run Metadata**      | ✅ **Ready (Beta)** | Label runs by `project`, `conversation`, `planner`, and `executor`. |
| **Claude/Qwen Adapters** | 🟡 **Functional** | Basic subprocess wrappers; lacks deep artifact extraction. |
| **Simulation Mode** | ✅ **Ready (Beta)** | Robust stubs for orchestrator validation without LLM use. |
| **Diagnostics & Health**| ✅ **Ready (Beta)** | CLI-based `doctor` and `smoke` verification tools. |
| **Antigravity Adapter**| ✅ **Ready (Beta)** | Direct-local execution via the Antigravity LS protocol. |
| **IDE Chat Bridge** | 🧪 **Prototype** | Experimental proxy-mediated file access via VS Code. |
| **Cloud / Multi-User** | 🚫 **Non-Goal** | Codencer is strictly local-first and self-hosted. |

### 🔍 Direct-Local Antigravity Integration
The `antigravity` adapter uses a **direct-local** model to control active Antigravity instances via RPC (Connect over HTTPS).
- **Same-Side Requirement**: Codencer and Antigravity must run on the **same OS side** (e.g., both in Linux/WSL or both in Windows).
- **Binding**: Use `orchestratorctl antigravity bind <PID>` to link a repository to an active Antigravity process discovered in `~/.gemini/antigravity/daemon`.
- **WSL Note**: WSL-to-Windows cross-side communication is not yet supported in direct-local mode.

### 🔍 Terminal Step States
Codencer distinguishes between different failure modes to help you recover faster:

| State | Meaning | Typical Recovery |
| :--- | :--- | :--- |
| `completed` | Success: All goals and validations met. | Next step. |
| `failed_validation` | Validations failed: Agent finished but tests/lint failed. | Fix code/prompt. |
| `failed_adapter` | Agent crashed: The binary or process failed. | Check config/keys. |
| `failed_bridge` | Bridge error: Orchestrator infrastructure failure. | Check disk/git/locks. |
| `timeout` | Time limit exceeded: Process was killed. | Increase timeout. |
| `cancelled` | Explicit stop: Operator aborted the run. | Resubmit if needed. |

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
## 🏗 One-Repo-One-Instance Model
Codencer is designed around a strictly local, repo-bound execution model:
- **1 Git Clone = 1 Daemon Instance**: Each repository checkout manages its own ledger and workspaces.
- **Multi-Instance Support**: To run multiple instances on the same machine, simply use different ports (e.g., `PORT=8086 make start`).
- **Identity Verification**: Use `./bin/orchestratorctl instance` to verify which repository and port a daemon is serving.

For more details, see **[Setup & Multi-Instance Workflows](docs/SETUP.md)**.

Codencer is released under the **MIT License**. See the [LICENSE](LICENSE) file for the full text.
