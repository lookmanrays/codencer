# Codencer Gap Audit

## Current Reality
The repository contains a functionally operational MVP implementation of the orchestration bridge. It successfully integrates a SQLite ledger, a robust state machine, a `DispatchStep` orchestrator loop, CLI endpoints, basic MCP routes, and a skeletal VS Code extension.

However, a rigorous audit reveals the following gaps to address for a more feature-complete MVP:

1. **Orchestration Workflow is Decomposed**: [RESOLVED] `RunService.DispatchStep` has been refactored into modular `initialize`, `runAttemptLoop`, and `finalize` stages.
2. **Adapter Paths are Hardened**: [RESOLVED] Unified adapter core implemented in `internal/adapters/common`. Mandatory binary checks enforced; simulation is explicitly separated and auditable. Hardcoded artifacts replaced with real filesystem collection for all adapters including Qwen.
3. **Retrieval Flows are Hardened**: [RESOLVED] Detailed retrieval of `artifacts`, `structured results`, and `validations` is now available across Service, API, MCP, and CLI layers with stable JSON schemas.
4. **Policy Engine is Defaulted**: [RESOLVED] Execution policies are now configuration-driven via `PolicyRegistry`. Logic supports YAML loading and provides baseline safe defaults.
5. **Recovery is Simplistic**: [RESOLVED] Integrated exclusive workspace locking and a deep reconciliation engine that salvages results and cleans up orphans.
6. **MCP Control Plane is Corrected**: [RESOLVED] Fixed critical identity resolution bugs in `ToolRetryStep` and updated all MCP tools to return machine-usable structured JSON payloads.
7. **Test Suite is Simulation-Bound**: [RESOLVED] Added recovery-specific, routing-specific, and robust API integration tests in `internal/app/api_test.go`.
8. **Routing logic is Opaque**: [RESOLVED] Transitioned to explicit heuristic chains with simulation-aware benchmark persistence and REST/MCP exposure for technical transparency.
9. **Terminology Inconsistency**: [RESOLVED] Renamed all outcome indicators to `State` (RunState, StepState, Result.State) for uniform operator experience. Building is verified 100% compliant.
10. **Documentation Stale**: [RESOLVED] README, Progress, and Tasks updated to reflect final project hardening status and explicit limitations.

## Final Feature Status Matrix

| Component | Status | Implementation Type | Notes |
| :--- | :--- | :--- | :--- |
| **Orchestration Core** | Complete | Native (SQLite) | Persistent ledger for runs, steps, and attempts. |
| **Workspace Isolation** | Complete | Native (Git) | Exclusive locking and worktree management. |
| **Policy Engine** | Complete | Native (Heuristic) | Gating based on migrations, file counts, and failures. |
| **CLI & MCP** | Complete | Native | Full surface for inspection and control. |
| **VS Code Extension** | Complete | Native | Functional tree-view and action surface. |
| **Recovery Engine** | Complete | Native | Decision-based reconciliation for stale runs. |
| **Benchmarking** | Complete | Native | Simulation-aware performance telemetry. |
| **Routing** | Functional | Heuristic | Deterministic static fallback chain; not yet benchmark-driven. |
| **Codex Adapter** | Functional | CLI Wrapper | Requires local `codex` binary. |
| **Claude Adapter** | Functional | CLI Wrapper | Requires local `claude-code` binary. |
| **Qwen Adapter** | Functional | CLI Wrapper | Requires local `qwen` / `aider` binary. |
| **IDE Chat Adapter** | Partial | Proxy-Mediated | Experimental extension-bound file proxy. |

## Verification Status

| Check | Result | Environment |
| :--- | :--- | :--- |
| **Build** | PASS | Linux / Go 1.21+ |
| **Unit Tests** | PASS | Isolated via `t.TempDir` |
| **E2E Flow** | PASS | Verified via Simulation Mode (Mocked behavior) |
| **API Endpoints** | PASS | Verified in `api_test.go` |
| **Extension** | PASS | Verified via manual VS Code sideload (Beta) |

## Known Technical Debt & Limitations
- **Adaptive Routing**: Routing is currently based on a static heuristic chain; benchmark-driven optimization is documented but not dynamic.
- **Process Introspection**: CLI-wrapped adapters provide limited visibility beyond standard streams.
- **Simulation Limits**: Simulation Mode stubs all actions; it validates the orchestrator's state-machine but does not test real agent logic.
- **IDE-Chat**: Proxy-mediated support means the daemon does not have deep native control over IDE-specific chat internals.
