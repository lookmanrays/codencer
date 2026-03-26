# Codencer Gap Audit

## Current Reality
The repository contains a functionally operational MVP implementation of the orchestration bridge. It successfully integrates a SQLite ledger, a robust state machine, a `DispatchStep` orchestrator loop, CLI endpoints, basic MCP routes, and a skeletal VS Code extension.

- **Lifecycle Meaning Cleanup**: [RESOLVED] Explicitly defined Run (Session), Step (Planner Unit), and Attempt (Execution Try) in domain code and README. Verified that no bridge-side decision logic is implied.

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
| **E2E Flow** | PASS | Simulated (Orchestrator-only verification) |
| **API Endpoints** | PASS | Verified in `api_test.go` |
| **Extension** | PASS | Verified via manual VS Code sideload (Beta) |

## Known Technical Debt & Limitations
- **Adaptive Routing**: Routing is currently based on a static heuristic chain; benchmark-driven optimization is documented but not dynamic.
- **Process Introspection**: CLI-wrapped adapters provide limited visibility beyond standard streams.
- **Simulation Limits**: Simulation Mode stubs all actions; it validates the orchestrator's state-machine but does not test real agent logic.
- **IDE-Chat**: Proxy-mediated support means the daemon does not have deep native control over IDE-specific chat internals.

## Relay Contract Audit (Micro-task)

### Current contract-related files
- **Domain (Go)**: `internal/domain/{task,result_spec,run,step,attempt,policy,benchmark}.go`
- **Schemas (JSON)**: `schemas/{task,result,policy}.schema.json`
- **Docs**: `docs/05_dsl_and_mcp.md`

### Canonical targets
- **TaskPayload**: `internal/domain.TaskSpec` (The source of truth for adapter instructions) [HARDENED]
- **ResultPayload**: `internal/domain.ResultSpec` (The source of truth for agent outcomes) [HARDENED]
- **State Enum**: `internal/domain.StepState` (Standardized across all layers)

### Conflicts & Gaps (Resolved for Input)
- **Model Inconsistency**: [RESOLVED] `Result` in `attempt.go` has been deprecated in favor of the comprehensive `ResultSpec`.
- **Schema Lag**: [RESOLVED] `schemas/task.schema.json` now includes all fields from Go `TaskSpec` including `timeout_seconds` and `is_simulation`.
- **Property Mismatch**: [RESOLVED] `schemas/result.schema.json` and `ResultSpec` now use standardized `state` and include raw outputs.
- **Simulation Leakage**: [RESOLVED] `is_simulation` explicitly added to the canonical input `TaskSpec`.

### State & Simulation Hardening (Micro-task)

- **State Semantics**: [RESOLVED] Standardized on 11 discrete states in `internal/domain/step.go`. Added `timeout` and `needs_manual_attention` to the core vocabulary.
- **Simulation Semantics**: [RESOLVED] Explicitly separated simulation data in benchmarks. Added machine-readable `is_simulation` flag to all relay results. Documentation now clearly distinguishes simulation from real execution.
- **Manual-Attention Semantics**: [RESOLVED] Clarified that the bridge *reports* attention needed while the planner *decides* the outcome.

## Execution State Audit (Micro-task)

### Current State Vocabularies
- **RunState**: `created`, `running`, `paused_for_gate`, `completed`, `failed`, `cancelled`.
- **StepState**: `pending`, `dispatching`, `running`, `collecting_artifacts`, `validating`, `completed`, `completed_with_warnings`, `needs_approval`, `needs_manual_attention`, `failed_retryable`, `failed_terminal`, `timeout`, `cancelled`.
- **ValidationState**: `not_run`, `running`, `passed`, `failed`, `errored`.
- **GateState**: `pending`, `approved`, `rejected`.

### Important Inconsistencies
- **Attempt State Mismatch**: Attempts currently reuse the 13-state `StepState` enum. This is semantically incorrect as attempts have a narrower lifecycle (start -> outcome) and do not "own" the orchestrator's collection/validation phases.
- **Human Attention Overlap**: `needs_approval` (gate-specific) and `needs_manual_attention` (general signal) are redundant. A unified "intervention required" model is needed for the relay.
- **Terminology Drift**: Validations and Gates still use the `Status` suffix, while Runs and Steps have standardized on `State`.
- **Process Transparency**: States like `dispatching` and `collecting_artifacts` reflect bridge internal mechanics rather than the planner's high-level intent.

### Next Steps (V1.1.3 / V1.2.1)
- **Refactor Attempt State**: Decouple Attempts from `StepState` and create a dedicated, narrower enum.
- **Unify Intervention States**: Consolidate `needs_approval` and `needs_manual_attention`.
- **Harden Timeout**: Fully integrate `StepStateTimeout` into the supervisor process management.
- **CLI Control Surface (V1.2.1)**: [RESOLVED] Task submission, status inspection, and action commands aligned for 100% reliable machine-usability (pure JSON across all planner-facing flows).

## CLI Surface Audit (V1.2.1)

### Current Commands
- `run start/status/abort`
- `step start/status/result/artifacts/validations`
- `gate approve/reject`
- `submit` / `step result` / `step wait` (Planner-Facing Canonical Commands)

### Identified Gaps
- **Output Control**: [RESOLVED] All planner-facing commands now return pure JSON for machine readability.
- **Discovery**: Missing `run list` and `step list <run_id>` for state inspection.
- **Planner Bridging**: [RESOLVED] Task submission aligned with canonical TaskSpec contract.
- **Telemetry**: Benchmarks and Routing config are unexposed via CLI (API/MCP only).
- **Control Flow**: [RESOLVED] Implemented `orchestratorctl step wait` for terminal state polling.
- **Gap - Terminal Waiting**: [RESOLVED] Implemented `orchestratorctl step wait` with domain-aligned terminal state detection.
- **Gap - Exit Semantics**: [RESOLVED] All CLI commands now return structured JSON on both success and error for reliable automated parsing.

### Next Alignment Steps
1. Implement `run list` and `step list` for discovery.
2. Expose `benchmarks` and `routing` groups.
