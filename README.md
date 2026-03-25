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
While structurally sound, the bridge is transitioning from "simulated correctness" to "native reliability" by:
1. **Dismantling Monoliths**: [RESOLVED] Transitioned `DispatchStep` into discrete, fault-tolerant lifecycle coordinators.
2. **Honest Adapter Contracts**: [RESOLVED] Standardized Codex, Claude, and Qwen adapters via a unified execution core (`internal/adapters/common`). Adapters now explicitly detect missing binaries and separate simulation and real execution.
3. **Retrieval Completeness**: [RESOLVED] Exposing standard API, CLI, and MCP retrieval functions to list artifacts and inspect step validation outputs natively.
4. **Strong Policies**: Removing hardcoded env mocks and forcing the Policy Engine to read explicit execution boundaries.
5. **Interactive Integrations**: [RESOLVED] Converted the VS Code UI into an active Control Plane for Gate resolution and workflow retracing.

**Phase 5 Orchestration & MCP Correctness (Complete):**
The core execution engine has been hardened for production-grade reliability. Key improvements include:
1. **Lifecycle Decomposition**: `RunService.DispatchStep` is now a modular coordinator with clear attempt-loop and environment-setup boundaries.
2. **MCP Identity Correctness**: Resolved critical bugs in Step Retry logic to ensure correct RunID propagation.
3. **Structured MCP Payloads**: All tool outputs now return machine-usable JSON, enabling better automated planning.
4. **Environment Robustness**: Worktree management now handles branch collisions and setup failures with explicit recovery paths.

> **Limitations:** For Phase 2, certain complex agent topologies (like interactive persistent CLI shells or cloud orchestration) are still explicitly unsupported to guarantee local-first safety.
> **Note on Adapters:** Codex, Claude, and Qwen are currently integrated as CLI wrappers. They require local binary installation (e.g. `claude-code`) unless the corresponding `*_SIMULATION_MODE=1` environment variable is set for testing/evaluation.
