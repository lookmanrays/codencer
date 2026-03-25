# Codencer Gap Audit

## Current Reality
The repository contains a functionally operational MVP implementation of the orchestration bridge. It successfully integrates a SQLite ledger, a robust state machine, a `DispatchStep` orchestrator loop, CLI endpoints, basic MCP routes, and a skeletal VS Code extension.

However, a rigorous audit reveals the following significant gaps separating the MVP from a "production-ready" local tool:

1. **Orchestration Workflow is Decomposed**: [RESOLVED] `RunService.DispatchStep` has been refactored into modular `initialize`, `runAttemptLoop`, and `finalize` stages.
2. **Adapter Paths are Hardened**: [IMPROVED] Adapters now handle environment setup errors (like worktree collisions) gracefully, and the dispatch loop handles system failures distinctly from adapter failures.
3. **Retrieval Flows are Hardened**: [RESOLVED] Detailed retrieval of `artifacts`, `structured results`, and `validations` is now available across Service, API, MCP, and CLI layers with stable JSON schemas.
4. **Policy Engine is Defaulted**: Execution policies are instantiated with hardcoded mock thresholds inside the dispatcher loop. There is no true persisted policy binding per step or run from configuration.
5. **Recovery is Simplistic**: [RESOLVED] Integrated exclusive workspace locking and a deep reconciliation engine that salvages results and cleans up orphans.
6. **MCP Control Plane is Corrected**: [IMPROVED] Fixed critical identity resolution bugs in `ToolRetryStep` and updated all MCP tools to return machine-usable structured JSON payloads.
7. **Test Suite is Simulation-Bound**: [IMPROVED] Added recovery-specific integration tests validating salvage and cleanup boundaries.

## Objective
The goal is to deepen the orchestrator runtime, transition away from mock representations to deterministic execution contracts, and complete the retrieval and recovery flows to establish genuine local-first reliability.

### Extension Audit [BATCH 4 COMPLETE]
- [x] **Passive Viewer**: The extension is now a functional operator surface.
- [x] **Noisy Polling**: Polling is replaced with cleaner manual refresh and stable client logic.
- [x] **Missing Actions**: Added Approve, Reject, and Retry actions.
- [x] **Structured Inspection**: Integrated JSON buffers for results and validations.
