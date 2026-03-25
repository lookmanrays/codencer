# Codencer Gap Audit

## Current Reality
The repository contains a functionally operational MVP implementation of the orchestration bridge. It successfully integrates a SQLite ledger, a robust state machine, a `DispatchStep` orchestrator loop, CLI endpoints, basic MCP routes, and a skeletal VS Code extension.

However, a rigorous audit reveals the following significant gaps separating the MVP from a "production-ready" local tool:

1. **Orchestration Workflow is Monolithic**: The `RunService.DispatchStep` handles attempt creation, adapter dispatch, polling, validation, policy evaluation, and gating inline. It lacks clean decoupling and service boundaries for lifecycle stages.
2. **Adapter Paths are Simulated/Fragile**: The Codex, Claude, and Qwen adapters operate mostly as thin subprocess wrappers. They lack robust error classification, structured result contract validation, or clear degraded-mode behaviors when binaries are misconfigured.
3. **Retrieval Flows are Incomplete**: While we can `start` runs/steps and view summary statuses, detailed retrieval of `artifacts`, structured `results`, and `validations` is missing across the API, MCP, and CLI layers.
4. **Policy Engine is Defaulted**: Execution policies are instantiated with hardcoded mock thresholds inside the dispatcher loop. There is no true persisted policy binding per step or run from configuration.
5. **Recovery is Simplistic**: The `RecoveryService` sweeps and marks stale runs as failed but fails to reconstitute incomplete attempts, paused run states, or cleanup locked Git worktrees intelligently.
6. **IDE/MCP Control Plane is Thin**: The VS Code extension accurately reads tree states but lacks interactive controls for Gate Management, run refreshes, or rich artifact inspection. The MCP server is missing critical read/write endpoints for results, validations, and step retries.
7. **Test Suite is Simulation-Bound**: Tests overly rely on `ENV` flags to force mock policies and successful adapter simulation, leaving genuine edge cases and robust integration undocumented.

## Objective
The goal is to deepen the orchestrator runtime, transition away from mock representations to deterministic execution contracts, and complete the retrieval and recovery flows to establish genuine local-first reliability.
