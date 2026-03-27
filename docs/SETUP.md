# Setup & Environmental Reference

This guide describes the technical prerequisites and environmental configuration required to run the Codencer bridge. For the operational guide, see the **[Canonical Local Runbook](EXAMPLES.md)**.

---

## 📋 Prerequisites

### 1. Core Runtime (Required)
- **Go**: Version 1.21 or higher.
- **SQLite3**: For the local persistent ledger.
- **Git**: Required for workspace-isolated runs (Git Worktrees).

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
| `PORT` | Listening port for the daemon API. | `8080` |
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
- Run `./bin/orchestratorctl doctor` to verify local binary availability.
- Run `make smoke` to execute the automated state-machine validation suite.

## 📖 Further Reading
- [Canonical Local Runbook](EXAMPLES.md)
- [Architecture Overview](02_architecture.md)
