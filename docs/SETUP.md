# Setup & Self-Hosting Guide

This guide describes how to deploy and run the Codencer Orchestration Bridge in your local environment.

## ⚡️ 1-2-3 Quickstart

If you have **Go** and **Git** installed, you can start a simulated run in 30 seconds:

```bash
# 1. Initialize and build
make setup build

# 2. Start the daemon in simulation mode
make simulate

# 3. (New Tab) Verify the loop
make smoke
```

---

## 🏛 Self-Host Inventory

When you run Codencer locally, you are hosting a **three-tier orchestration stack**:

| Component | Responsibility | Hosting Mode |
| :--- | :--- | :--- |
| **Orchestratord** | The persistent state daemon & SQLite ledger. | **Local Process** |
| **Orchestratorctl** | The terminal control surface (CLI). | **Local Binary** |
| **Adapters** | Subprocess wrappers for coding agents. | **Local Process** |
| **Coding Agents** | Tactical workers (Codex, Claude-code, Aider). | **Local Binary** |
| **Artifact Store** | Secure vault for logs, diffs, and results. | **Local Filesystem** (`.codencer/`) |

---

## 📋 Prerequisites

### 1. Core Runtime (Required)
- **Go**: Version 1.21 or higher.
- **SQLite3**: For the local persistent ledger.
- **Git**: Required for workspace-isolated runs (Git Worktrees).

### 2. Tactical Agents (Required for Real Mode)
To perform real file edits, you need at least one tactical agent installed in your `$PATH`.

#### **Claude (Recommended)**
```bash
npm install -g @anthropic-ai/claude-code
```

#### **Codex**
```bash
npm install -g @lookman/codex-agent
```

*Note: If these are missing, you can still test the orchestrator logic using **Simulation Mode** (see below).*

---

## 🛠 Installation

### 1. Initialize Environment
```bash
# Creates the .codencer/ directory and validates prerequisites
make setup
```

### 2. Build Binaries
```bash
# Compiles bin/orchestratord and bin/orchestratorctl
make build
```

---

## ⚙️ Configuration

Codencer is **configuration-first**. It looks for `config/default.json` but honors environment variable overrides.

### 1. Key Variables
| Variable | Description | Default |
| :--- | :--- | :--- |
| `PORT` | Listening port for the daemon API/MCP. | `8080` |
| `DB_PATH` | Path to the SQLite database. | `.codencer/codencer.db` |
| `ARTIFACT_ROOT` | Where per-attempt artifacts are stored. | `.codencer/artifacts` |
| `CODEX_BINARY` | Custom path to the Codex agent. | `codex-agent` (in $PATH) |

### 2. Simulation Overrides
To verify orchestration without running real agents:
- `ALL_ADAPTERS_SIMULATION_MODE=1`: Stubs all agent execution.
- `CODEX_SIMULATION_MODE=1`: Stubs only the Codex adapter.

---

## 🌐 External Connectivity

Codencer is **Local-First but Agent-Aware**:
1. **The Bridge is Local**: No data leaves your machine via the Codencer daemon itself. All state is in your local SQLite and filesystem.
2. **Agents may be Remote**: While the *Bridge* is local, the **Coding Agents** (like Claude-code) may connect to their respective vendor APIs (Anthropic, OpenAI) to perform reasoning.
3. **Auditability**: Because Codencer acts as a relay, every request sent to and received from these agents is captured and hashed locally in your `Artifact Store`.

---

## 🏃 Running the Stack

### 1. Start the Daemon
```bash
# Normal Mode
./bin/orchestratord

# Or using the simulation helper
make simulate
```

### 2. Verify with the CLI
```bash
# Check if the daemon is responsive
./bin/orchestratorctl doctor

# Start a simple run
./bin/orchestratorctl run start my-run my-project
```

### 3. Submit a Task
Most production-style interactions use a YAML `TaskSpec`:
```bash
./bin/orchestratorctl submit my-run examples/tasks/bug_fix.yaml
```

---

## 🔍 Verification & Health
- **Logs**: View live agent output with `orchestratorctl step logs <id>`.
- **Doctor**: Run `make doctor` to verify local binary availability and environment paths.
- **Nuke**: Run `make nuke` to completely reset all local state and history.
