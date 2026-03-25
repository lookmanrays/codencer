# Codencer Gap Audit

## Current Reality
The repository contains a functionally operational MVP implementation of the orchestration bridge. It successfully integrates a SQLite ledger, a robust state machine, a `DispatchStep` orchestrator loop, CLI endpoints, basic MCP routes, and a skeletal VS Code extension.

However, a rigorous audit reveals the following significant gaps separating the MVP from a "production-ready" local tool:

1. **Orchestration Workflow is Decomposed**: [RESOLVED] `RunService.DispatchStep` has been refactored into modular `initialize`, `runAttemptLoop`, and `finalize` stages.
2. **Adapter Paths are Hardened**: [IMPROVED] Adapters now handle environment setup errors (like worktree collisions) gracefully, and the dispatch loop handles system failures distinctly from adapter failures.
3. **Retrieval Flows are Hardened**: [RESOLVED] Detailed retrieval of `artifacts`, `structured results`, and `validations` is now available across Service, API, MCP, and CLI layers with stable JSON schemas.
4. **Policy Engine is Defaulted**: Execution policies are instantiated with hardcoded mock thresholds inside the dispatcher loop. There is no true persisted policy binding per step or run from configuration.
5. **Recovery is Simplistic**: The `RecoveryService` sweeps and marks stale runs as failed but fails to reconstitute incomplete attempts, paused run states, or cleanup locked Git worktrees intelligently.
6. **MCP Control Plane is Corrected**: [IMPROVED] Fixed critical identity resolution bugs in `ToolRetryStep` and updated all MCP tools to return machine-usable structured JSON payloads.
7. **Test Suite is Simulation-Bound**: Tests overly rely on `ENV` flags to force mock policies and successful adapter simulation, leaving genuine edge cases and robust integration undocumented.

## Objective
The goal is to deepen the orchestrator runtime, transition away from mock representations to deterministic execution contracts, and complete the retrieval and recovery flows to establish genuine local-first reliability.
