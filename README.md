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

### 1. 30-Second Verification (Simulation)
Test the full orchestration loop without requiring real LLM agents or binary setup:
```bash
# Initialize, build, and start the daemon in simulation mode
make setup build simulate

# (New Tab) Run the automated smoke test
make smoke
```

### 2. Real-World Execution
Submit a realistic task to a real agent (requires `claude-code` or `codex-agent` in `$PATH`):
```bash
# Install an agent
npm install -g @anthropic-ai/claude-code

# Start a run and submit a task
./bin/orchestratorctl run start my-run my-project
./bin/orchestratorctl submit my-run examples/tasks/bug_fix.yaml
```

For detailed agent installation and configuration, see the **[Setup & Self-Hosting Guide](docs/SETUP.md)**.

### 3. Real-World Execution
Submit a realistic task to a real agent (requires `codex-agent` or similar in `$PATH`):
```bash
# Start a new run
./bin/orchestratorctl run start my-fix-run my-project

# Submit an instruction (YAML TaskSpec)
./bin/orchestratorctl submit my-fix-run examples/tasks/bug_fix.yaml

# Monitor progress (live tail)
./bin/orchestratorctl step logs <stepID>

# Wait for terminal results
./bin/orchestratorctl step wait <stepID>
```

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

1. **Simulation Mode** (`make simulate`): Only validates the **Orchestrator**. It tests if the ledger, state machine, and CLI are working. It does **not** test if the agent can actually code.
2. **Real Mode**: Tests the full end-to-end loop. Requires real agent binaries (`claude-code`, etc.) and incurs real LLM costs.

---

## 📖 Documentation
- **[Self-Host Runbook (Flows)](docs/EXAMPLES.md)** (Start here for daily use)
- [Setup & Self-Hosting Guide](docs/SETUP.md)
- [Architecture Overview](docs/02_architecture.md)
- [Troubleshooting Guide](docs/TROUBLESHOOTING.md)
- [Gap Audit & Roadmap](docs/internal/GAP_AUDIT.md)

---

## ⚖ License
*Licence pending (intended MIT/Apache 2.0). See [GAP_AUDIT.md](GAP_AUDIT.md) for publication status.*
