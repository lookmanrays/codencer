# Codencer Orchestration Bridge (MVP/Beta)

**Defensive, Local-First Relay for Autonomous Coding Agents.**

Codencer is a persistent orchestration daemon designed to securely manage, execute, validate, and audit coding tasks performed by external agents. It acts as the **system of record** between a high-level **Planner** (you or an LLM) and tactical **Coding Agents** (Codex, Claude-code, Aider). It is **100% self-hostable** and designed for local-first developer toolchains.

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
- **Planner**: You, a Chat UI, or an autonomous agent. Decides **what** to do.
- **Bridge (Codencer)**: Receives the `TaskSpec`, manages workspace isolation (Git Worktrees), enforces policies, and monitors execution.
- **Coding Agent**: The underlying tool performing the actual work (e.g., `codex-agent`, `claude-code`).

For detailed local setup instructions, see the **[Setup & Self-Hosting Guide](docs/SETUP.md)**.

---

---

## ⚡️ Quickstart: 1-2-3 Local Setup

Get up and running locally in less than a minute.

### 1. Build & Configure
```bash
# Initialize and build binaries
make setup build

# Copy example environment configuration
cp .env.example .env
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
# 2. Submit a tactical task (Automatic wait)
./bin/orchestratorctl submit my-first-run examples/tasks/bug_fix.yaml --wait

# 3. Inspect the outcome
./bin/orchestratorctl step result <stepID>
./bin/orchestratorctl step artifacts <stepID>
./bin/orchestratorctl step validations <stepID>

---

## 🔍 Interpreting Outcomes

The Bridge reports high-fidelity evidence for every attempt:
- **`completed`**: Goal met, all tests passed.
- **`completed_with_warnings`**: Success, but with lint/test warnings.
- **`failed_terminal`**: Execution halted due to an unrecoverable error.
- **`needs_approval`**: Policy gate hit; run `orchestratorctl gate approve <id>`.

For a deeper dive into agent installation and advanced flows, see the **[Setup & Self-Hosting Guide](docs/SETUP.md)**.

---

## 🛡 Why Codencer?

Autonomous agents are non-deterministic. Codencer provides the guardrails:

1. **Workspace Safety**: Agents run in isolated Git Worktrees. Diffs are captured and validated before any commit.
2. **Audit-Proof Ledger**: Every attempt is recorded in a local SQLite database with SHA-256 hashes of all artifacts.
3. **Idempotency**: Interrupted tasks can be resumed or securely analyzed post-crash.
4. **Validation-First**: Tasks only "complete" when your defined validation commands (tests, linters) pass.

---

## 📊 Maturity & Capability Matrix

Codencer is currently in an **MVP/Beta** state. Use the following matrix to understand current support:

| Feature Area | Status | Description |
| :--- | :--- | :--- |
| **Orchestration Core** | ✅ **Ready** | Persistent SQLite ledger, state machine, and Git Worktrees. |
| **CLI & MCP Layer** | ✅ **Ready** | Structured JSON outputs, log tailing, and health checks. |
| **Codex Adapter** | ✅ **Ready** | High-fidelity relay for the `codex-agent` binary. |
| **Claude/Qwen Adapters** | 🟡 **Functional** | Basic subprocess wrappers; lacks deep artifact extraction. |
| **Simulation Mode** | ✅ **Ready** | Robust stubs for orchestrator validation without LLM use. |
| **IDE Chat Bridge** | 🧪 **Experimental**| Proxy-mediated file access via VS Code extension. |
| **Adaptive Routing** | 🧪 **Experimental**| Static fallback chain; benchmark-aware logic is a blueprint. |
| **Cloud / Multi-User** | 📅 **Future** | Not implemented. Codencer is strictly local-first today. |

## 🧪 Simulation vs. Real Execution

1. **Simulation Mode** (`make start-sim` or `make simulate`): Only validates the **Orchestrator**. It tests if the ledger, state machine, and CLI are working. It does **not** test if the agent can actually code.
2. **Real Mode**: Tests the full end-to-end loop. Requires real agent binaries (`claude-code`, etc.) and incurs real LLM costs.

---

## 📖 Documentation
- **[Self-Host Runbook (Flows)](docs/EXAMPLES.md)** (Start here for daily use)
- [Setup & Self-Hosting Guide](docs/SETUP.md)
- [Architecture Overview](docs/02_architecture.md)
- **[Troubleshooting & Recovery](docs/TROUBLESHOOTING.md)** (What to do when things fail)
- [Gap Audit & Roadmap](docs/internal/GAP_AUDIT.md)

---

## ⚖ License
*Licence pending (intended MIT/Apache 2.0). See [GAP_AUDIT.md](GAP_AUDIT.md) for publication status.*
