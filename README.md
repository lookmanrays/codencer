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

## Current State & Maturity (Phase 2)

**Phase 1 MVP (Complete):**
The foundational orchestration shell is fully operational. A persistent SQLite ledger, an initial state-machine loop, workspace isolation via Git Worktrees, basic MCP tool mapping, and a scaffolding IDE extension have all been implemented.

**Phase 2 Production Hardening (Active):**
While structurally sound, the bridge operates primarily via simulated or thin interactions. Phase 2 aims to break out of "simulated correctness" by:
1. **Dismantling Monoliths**: Transitioning `DispatchStep` into discrete, fault-tolerant lifecycle coordinators.
2. **Honest Adapter Contracts**: Executing genuine Codex/Claude/Qwen paths capable of catching process regressions, missing binaries, and enforcing rigid JSON Result contracts.
3. **Retrieval Completeness**: Exposing standard API, CLI, and MCP retrieval functions to list artifacts and inspect step validation outputs natively, moving beyond simple state checking.
4. **Strong Policies**: Removing hardcoded env mocks and forcing the Policy Engine to read explicit execution boundaries mapping directly to realistic disk changes (`diff` sizing, missing dependencies).
5. **Interactive Integrations**: Converting the passive VS Code UI into an active Control Plane capable of resolving Gates and retracing failed workflows reliably.

> **Limitations:** For Phase 2, certain complex agent topologies (like interactive persistent CLI shells or cloud orchestration) are still explicitly unsupported to guarantee local-first safety.