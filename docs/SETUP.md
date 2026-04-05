# Setup & Environmental Reference

This guide describes the technical prerequisites and environmental configuration required to run the Codencer bridge. For the operational guide, see the **[Canonical Local Runbook](EXAMPLES.md)**.

---

## 📋 Prerequisites

### 1. Core Runtime (Required)
- **Go**: Version 1.21 or higher.
- **C Compiler**: `gcc`, `clang`, or `cc` (Required to build the CGO embedded SQLite driver).
- **Git**: Required for workspace-isolated runs (Git Worktrees).

### ⚡️ The 30-Second Mission (Simulation)
Use this flow to verify the bridge logic (ledger, state machine, CLI) without requiring external LLMs or agent binaries.

### 1. Automated Verification (Recommended)
From a clean clone, run the automated verification suite:
```bash
make setup build smoke
```
This single command initializes the environment, builds the binaries, and runs a full simulation loop.

### 2. Tactical Agents (Real Mode Only)
To perform real file edits, you need at least one tactical agent binary in your `$PATH`.

#### **Claude (Recommended)**
```bash
npm install -g @anthropic-ai/claude-code
```

#### **Codex**
```bash
npm install -g @lookman/codex-agent
```

---

## 🎭 Execution Modes

Codencer allows you to decouple **Orchestrator** verification from **Agent** reasoning.

### 1. Simulation Mode
- **Goal**: Verify state machine, ledger, and CLI.
- **Config**: `ALL_ADAPTERS_SIMULATION_MODE=1` in `.env`.
- **Start**: `make start-sim` (background) or `make simulate` (foreground).

### 2. Real Mode
- **Goal**: Perform actual file edits using LLM-based agents.
- **Config**: `ALL_ADAPTERS_SIMULATION_MODE=0` and agent binary paths.
- **Start**: `make start` (background) or `./bin/orchestratord` (foreground).

---

## ⚙️ Detailed Configuration

Codencer honors environment variable overrides and a local `.env` file.

| Variable | Description | Default |
| :--- | :--- | :--- |
| `PORT` | Listening port for the daemon API. | `8085` |
| `DB_PATH` | Path to the SQLite ledger. | `.codencer/codencer.db` |
| `ARTIFACT_ROOT`| Storage vault for diffs and logs. | `.codencer/artifacts` |
| `CODEX_BINARY` | Path to the Codex agent binary. | `codex-agent` |
| `LOG_LEVEL` | `debug`, `info`, `warn`, `error`. | `info` |

---

## 🏗 Storage Model

| Path | Responsibility |
| :--- | :--- |
| `.codencer/codencer.db` | The system-of-record (SQLite). |
| `.codencer/artifacts/` | Per-attempt diffs, logs, and artifacts. |
| `.codencer/workspace/` | Temporary Git Worktrees for isolated execution. |

---

## 🔍 Self-Review & Health

Before running your first mission, use the built-in diagnostic tool to verify your local environment:

```bash
./bin/orchestratorctl doctor
```

The doctor checks:
- **Environment**: Presence of `.env` and `.codencer/` directory.
- **Permissions**: Write access to the local ledger storage.
- **Binaries**: Presence and versions of `git`, `go`, and `cc` (for embedded DB).
- **Adapters**: Detects whether `codex-agent` or other adapters are reachable in your PATH (Informational/Optional).
- **Mode**: Confirms whether you are running in **Simulation** or **Real** execution mode.

If any check fails, the doctor will provide targeted instructions (e.g., "Run 'make setup'" or "Install git").

### 🧪 Relay Validation
Once the doctor reports `[OK]`, execute the automated smoke test to verify the full end-to-end relay loop in simulation mode:

```bash
make smoke
```

The smoke test validates:
1. Daemon startup and health connectivity.
2. Mission run initialization.
3. Task submission and synchronous completion (`submit --wait`).
4. Authoritative result reporting (`step result`).

---

## 🏗 Single vs. Multi-Instance Workflows
By default, Codencer is designed as a single-instance bridge for a single repository checkout.
- **1 Repo Checkout = 1 Daemon Instance**

If you need to work with multiple repositories simultaneously on the same machine, you must run multiple daemon instances on different ports:

```bash
# Repo A
cd repo_a
PORT=8085 make start

# Repo B
cd repo_b
PORT=8086 make start
```

## 💻 Environment Workflows (macOS / WSL)
Codencer provides an identical local-first technical surface on macOS and Windows Subsystem for Linux (WSL).
- **WSL Users**: Run the daemon inside your WSL instance. Windows-side IDEs (like VS Code) can connect to the daemon over the local loopback `http://localhost:8085` transparently.
- **Embedded DB Architecture**: The bridge embeds SQLite via Go's CGO. This means no external DB service (like Postgres or SQLite daemon) needs to be installed, though a standard local C compiler (`cc`/`gcc`) is briefly hit during the `go build` step.

## 📖 Further Reading
- [Canonical Local Runbook](EXAMPLES.md)
- [Architecture Overview](02_architecture.md)
