# Codencer: The Tactical Orchestration Bridge

Codencer is a tactical orchestration bridge that manages execution, isolation, and high-fidelity audit trails for coding agents. It serves as the **system of record** between a high-level **Planner** (human or LLM) and tactical **Coding Agents** (Codex, Claude-code, Aider). 

Designed for **local-first, self-hosted developer toolchains**, Codencer provides the missing "relay" layer that ensures every task attempt is isolated, provisioned, and validated before it ever reaches your production branch.

> [!IMPORTANT]
> **Project Status: Public Beta (v0.1.0-beta)**.
> Codencer is technically functional for local dev use, with a hardened execution path via the **Codex** adapter. While the core engine is stable, the API and protocols are subject to refinement as we gather community feedback.

---

### Core Guarantees

- **Step-Isolation**: Each step executes in its own git worktree, preventing cross-task interference.
- **Immutable Evidence**: All logs, results, and artifacts are namespaced by Run, Step, and Attempt ID under `.codencer/artifacts/<run-id>/<step-id>/<attempt-id>/`, ensuring full auditability of repeated attempts.
- **Workspace Provisioning**: Automatically prepares attempt worktree environments (copying `.env`, symlinking `node_modules`, running `post_create` hooks). Codencer includes an **optional Grove-compatible subset** for environment preparation; it does not depend on the Grove CLI and is designed to coexist with existing `.groverc.json` or `grove.yaml` files.
  - *Inspiration*: This layer was inspired in part by [Grove](https://github.com/verbaux/grove).
  - *Thanks*: Special thanks to [@verbaux](https://github.com/verbaux) for the conceptual foundation of local workspace preparation.

> **Execution Path Note**: Codencer depends on Git Worktrees for isolating task attempts. Therefore, cloning the repository via `git clone` is the **only supported execution path**. Downloading a ZIP source archive will fail during targeted execution.

---

## 🏛 The Relay Model

Codencer is a **bridge, not a brain**. It does not decide the high-level strategy; it executes tactical instructions and reports high-fidelity evidence.

- **What it is**: A system of record, a workspace isolator, a validator, and a provider of immutable artifacts.
- **What it is not**: A planner, a chat UI, a cloud service, or an AI "agent" that thinks about what to do next.

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

---

## 🚀 The Canonical Run Path (Local-First)

The standard sequence for performing an audited tactical task:

1.  **Clone & Setup**: `git clone` the repo → `make setup build`.
2.  **Start the Bridge**: `make start-sim` (for testing) or `make start` (for real agents).
3.  **Inspect Instance**: `./bin/orchestratorctl instance` (Verify port/repo).
4.  **Start a Run**: `./bin/orchestratorctl run start <RUN_ID> <PROJECT>`.
5.  **Submit & Wait**: `./bin/orchestratorctl submit <RUN_ID> <TASK_FILE>|--goal "<text>" --wait --json`.
6.  **Audit the Result**: `./bin/orchestratorctl step result <UUID>` (The Summary).
7.  **Evidence Drill-down**: `./bin/orchestratorctl step logs/artifacts/validations <UUID>`.

For ordered task lists, the official v1 pattern is an external wrapper loop that calls Codencer one task at a time. Codencer does not include a native workflow engine or planner-like batch runner in v1.

---

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

# 2a. Submit a rich TaskSpec file and wait for completion
./bin/orchestratorctl submit first-run examples/tasks/bug_fix.yaml --wait --json

# 2b. Or use direct convenience input for local automation
./bin/orchestratorctl submit first-run --goal "Fix the failing tests in pkg/foo" --title "Fix pkg/foo tests" --adapter codex --wait --json

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

## 🧾 Submission Inputs

Codencer supports two submit styles:

1. **Canonical TaskSpec**: submit a full YAML or JSON task definition when you need rich structure.
2. **Direct convenience input**: submit a prompt/goal directly and let the CLI deterministically normalize it into `TaskSpec`.

Direct input is intentionally narrow. It does not plan, decompose work, merge multiple sources, or invent strategy.

### Exactly One Primary Source

`submit` requires exactly one of:
- positional task file
- `--task-json <path|->`
- `--prompt-file <path>`
- `--goal <text>`
- `--stdin`

Direct metadata flags are only supported with `--prompt-file`, `--goal`, and `--stdin`:
- `--title`
- `--context`
- `--adapter`
- `--timeout`
- `--policy`
- repeated `--acceptance`
- repeated `--validation`

### Deterministic Defaults

For direct convenience input:
- `version` defaults to `v1`
- `run_id` comes from the CLI `<RUN_ID>`
- `title` comes from `--title`, otherwise the prompt filename basename, otherwise `Direct task`
- `goal` is the exact submitted text from `--goal`, `--prompt-file`, or `--stdin`
- repeated `--validation` flags become deterministic validation commands named `validation-1`, `validation-2`, and so on

`context` and `acceptance` are preserved in the normalized task and provenance, but they are currently retained metadata rather than separate executor-driving runtime fields.

### Provenance and Auditability

Every accepted submission keeps both:
- the original input as `original-input.*`
- the normalized canonical payload as `normalized-task.json`

Those files are written under the attempt artifact root so a later audit can answer what exact content was submitted and what normalized task Codencer actually executed.

## 🔁 Ordered Task Lists

The official v1 sequential-execution story is wrapper-based:
- start or reuse a run
- submit one item at a time with `submit --wait --json`
- inspect the exit code and terminal payload outside Codencer
- decide whether to continue or stop outside Codencer

Official wrapper examples live in [examples/automation](examples/automation):
- [run_tasks.sh](examples/automation/run_tasks.sh)
- [run_tasks.ps1](examples/automation/run_tasks.ps1)
- [run_tasks.py](examples/automation/run_tasks.py)

This keeps Codencer sharp and narrow as a bridge rather than a workflow brain.

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
- **No Native Workflow Engine**: Ordered task lists are handled by wrappers/scripts outside Codencer core in v1.

---

### 📊 Maturity & Capability Matrix

Codencer is in **Beta (v0.1.0-beta)**. Use this to understand what is stable vs. experimental.

| Feature Area | Status | Description |
| :--- | :--- | :--- |
| **Local Bridge Core** | ✅ **Stable Beta** | Persistence, state machine, Git Worktrees. |
| **Provisioning Layer**| ✅ **Stable Beta** | Native copy/symlink layer; optional Grove subset. |
| **Codex Adapter** | ✅ **Stable Beta** | Primary high-fidelity relay for `codex-agent`. |
| **Antigravity Metadata** | ✅ **Ready (Beta)** | Broker-backed context, task IDs, and provenance. |
| **Antigravity Broker** | 🟡 **Operational** | Cross-side (WSL/Windows) bridge for IDE instances. |
| **Simulation Mode** | ✅ **Stable Beta** | Stub-based validation (Bridge-only smoke tests). |
| **IDE Chat Bridge** | 🧪 **Experimental** | Proxy-mediated file access via VS Code (Prototype). |
| **Cloud / Multi-User** | 🚫 **Non-Goal** | Codencer is strictly local-first and self-hosted. |

### 🔍 Direct-Local Antigravity Integration
The `antigravity` adapter uses a **direct-local** model to control active Antigravity instances via RPC (Connect over HTTPS).
- **Primary Model**: Codencer and Antigravity usually run on the **same OS side** (e.g., both in Linux or both in Windows).
- **WSL ↔ Windows (Experimental)**: Cross-side communication is supported via the shared loopback (`127.0.0.1`). Codencer in WSL can discover Windows-side instances if the host's `.gemini` directory is reachable (e.g., via `/mnt/c`).
- **Binding**: Use `orchestratorctl antigravity bind <PID>` to link this repository to an active Antigravity process. Binding establishes repo-scoped target identity and connectivity; execution still depends on the task's explicit `adapter_profile`.

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
- **[CLI Automation Patterns](docs/CLI_AUTOMATION.md)** — Sequential wrapper loops, JSON mode, and shell-capable planner usage.
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
