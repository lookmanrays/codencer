# Setup Guide

This guide describes how to set up and run the Codencer MVP Orchestration Bridge locally.

## Prerequisites

- **Go**: Version 1.21 or higher.
- **SQLite3**: For the local persistent ledger.
- **Git**: Required for workspace-isolated runs (Git Worktrees).
- **Adapters (Optional for Simulation)**:
  - `claude-code`
  - `codex` (if using local binaries)
  - `qwen`

## 1. Initial Setup

Clone the repository and run the setup target to initialize the directory structure:

```bash
make setup
```

This creates the `.codencer/` directory for artifacts and workspace isolation.

## 2. Building the Binaries

Build both the daemon (`orchestratord`) and the controller (`orchestratorctl`):

```bash
make build
```

Binaries will be placed in the `bin/` directory.

## 3. Configuration

Configuring Codencer is primarily done via environment variables:

- `PORT`: HTTP port for the daemon (default: `8080`).
- `DB_PATH`: Path to the SQLite database (default: `codencer.db`).
- `ARTIFACT_ROOT`: Directory for storing run artifacts.
- `WORKSPACE_ROOT`: Directory for isolated worktrees.
- `*_SIMULATION_MODE=1`: Enables simulation for a specific adapter (e.g., `CODEX_SIMULATION_MODE=1`).

## 4. Running the Daemon

You can run the daemon directly or via simulation:

**Normal Mode:**
```bash
./bin/orchestratord
```

**Simulation Mode (Recommended for MVP verification):**
```bash
make simulate
```

## 5. Using the CLI

Once the daemon is running, use `orchestratorctl` to manage runs:

**Start a Run:**
```bash
# orchestratorctl run start <id> <project_id>
./bin/orchestratorctl run start my-run my-proj
```

**Dispatch a Step:**
```bash
# orchestratorctl step start <runID> <taskFile.yaml>
./bin/orchestratorctl step start my-run task.yaml
```

**Resolve a Gate:**
```bash
./bin/orchestratorctl gate approve --gate-id <gate-id>
```

## 6. Verification

To verify the installation is working correctly:
1. Run `make simulate`.
2. Start a run via CLI.
3. Observe the logs for state transitions (`PENDING` -> `RUNNING` -> `COMPLETED`).
4. Check the `codencer.db` or use the CLI to list runs.
