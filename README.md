# Codencer Orchestration Bridge

Codencer is a local-first orchestration daemon designed to securely manage, execute, validate, and audit coding tasks performed by autonomous agents. It acts as the system of record between the Planner (MCP clients or LLMs) and the underlying Adapters (Codex, Claude, Qwen).

## Core Architecture
- **Orchestratord (Daemon)**: The persistent state engine using a local SQLite ledger to safely track Runs, Phases, Steps, and Attempts.
- **Adapters**: Abstractions over vendor agents (e.g. Codex) standardizing initialization, polling, capability discovery, and artifact collection.
- **Policy Engine**: Ensures execution safety by intercepting workflows based on heuristic thresholds (e.g., changed files, validation failures) and pausing execution until a human operator responds via a **Gate**.
- **CLI & MCP**: Primary control surfaces. `orchestratorctl` enables terminal-centric operations, while the MCP server provides integration hooks for planning agents.

## Why Codencer?
Agents are chaotic and non-deterministic. Codencer wraps them in a deterministic framework:
1. **Safety**: Agents run in configurable bounds (optional git worktrees, strict diff capturing).
2. **Idempotency**: Runs and attempts are carefully ledgered; interrupted tasks can be resumed or securely analyzed post-crash.
3. **Traceability**: All outputs (stdout, result.json, diffs) are meticulously persisted per-attempt in the artifact store.

## Current State
Codencer MVP is currently under active development. The primary run state management structure exists, and we are heavily focusing on finalizing real step dispatching, artifact collection, and Human-in-the-Loop policy gating.