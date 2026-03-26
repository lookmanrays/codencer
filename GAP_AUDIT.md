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
- [x] Implement Codex-specific result normalization and outcome mapping (V1.3.1 Complete) <!-- id: 64 -->
- [x] Align Codex adapter reporting with relay contracts (V1.3.1 Complete) <!-- id: 65 -->
- [x] Harden Codex result file harvesting and artifact linking (V1.3.1 Complete) <!-- id: 68 -->
- [x] Harden Codex artifact discovery- [x] Final alignment for Batch V1.3.2 (Complete) <!-- id: 71 -->
- [x] Define and document local validation scenario (V1.3.2 Complete) <!-- id: 72 -->
- [x] Add/align practical local validation path (V1.3.2 Complete) <!-- id: 73 -->
- [x] Improve observable success/failure evidence (V1.3.2 Complete) <!-- id: 74 -->
- [x] Add/strengthen validation coverage (V1.3.2 Complete) <!-- id: 75 -->
- [ ] Implement state discovery (run/step listing) <!-- id: 52 -->
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

## Codex Adapter Hardening Audit (V1.3.1)

### Current Capabilities
- **Binary Execution**: Standard `os/exec` wrapper with environment variable overrides (`CODEX_BINARY`).
- **Simulation**: Independent stubbing path that verifies orchestrator transitions without real LLM use.
- **Artifacts**: Automatic collection of `stdout.log`, `result.json`, and diffs.

### Identified Weaknesses
- **Task Propagation**: [RESOLVED] The bridge now passes `Goal` and `Title` to the agent CLI.
- **Delayed Validation**: [RESOLVED] `Start` now fails fast if the agent binary is missing.
- **Weak Normalization**: [RESOLVED] Implemented Codex-specific normalization with robust error handling.
- **Opaque Streams**: `stdout.log` is captured but not yet streamed to the planner for real-time progress.

### Next Hardening Steps
1. **Early Binary Validation**: [DONE] Fail `Start()` fast if the adapter binary is missing.
2. **TaskSpec Delivery**: [DONE] Pass `Goal` and `Title` to the adapter CLI as arguments.
3. **Outcome Normalization**: [DONE] Refine `NormalizeResult` to handle edge cases and provide "Bridge Interface Error" context.
4. **Reporting Alignment**: [DONE] Ensure `RequestedAdapter` and `Adapter` are clearly distinguished in results.
5. **Harvesting Hardening**: [DONE] Linked `stdout.log` to `RawOutputRef` and added explicit file verification.
6. **Artifact Discovery Hardening**: [DONE] Implemented SHA-256 hashing and content-based MIME detection.
7. **Canonical Alignment**: [DONE] aligned `ResultSpec` with `v1` schema, including explicit artifact mapping.
8. **Final Alignment (V1.3.2)**: [DONE] Verified metadata integrity, fixed string literal inconsistencies, and updated docs to reflect high-fidelity harvesting.
9. **Validation Scenario**: [DONE] Documented safe, realistic v0.1.0 version-bump smoke test.
10. **Validation Path**: [DONE] Added `make validate` and `docs/validation_task.yaml` for repeatable execution.
11. **Evidence Visibility**: [DONE] Implemented JSON pretty-printing in `orchestratorctl` and enhanced terminal status reporting.
12. **Validation Coverage**: [DONE] Implemented `internal/service/validation_scenario_test.go` to automate "version bump" evidence flow verification.
13. **Persistence Fix**: [DONE] Updated `AttemptsRepo` to correctly store/retrieve `Version` and `Artifacts` metadata.
### 1. Codex Harvesting Audit (V1.3.1)

#### Current Flow
1. **Discovery**: `CollectStandardArtifacts` uses `os.ReadDir` on the attempt's unique artifact directory.
2. **Classification**: Filenames like `stdout.log` and `result.json` are mapped to standard `domain.ArtifactType` values.
3. **Capture**: Metadata (size, mod-time) is captured and persisted to SQLite via `ArtifactsRepo.Create`.
4. **Resilience**: Missing or malformed `result.json` files trigger a "Bridge Interface Error" reported as a terminal failure.

#### Key Hardening Outcomes
- **Artifact Hashing**: [RESOLVED] Implemented SHA-256 hashing during discovery for data integrity.
- **Raw Output Linking**: [RESOLVED] Systematically populate `RawOutputRef` in the `ResultSpec` from `stdout.log`.
- **MIME/Type Refinement**: [RESOLVED] Use `http.DetectContentType` for robust artifact typing.
- **Persistence Hardening**: [RESOLVED] Updated SQLite schema and repository to persist `Version` and `Artifacts` metadata.
- **Validation**: [RESOLVED] Added non-simulated integration tests for the "version bump" smoke test.

### 2. Waiting & Polling Audit (V1.4.1)

#### Current Capabilities
- **Server Polling**: `RunService` polls adapters every 2s using `adapter.Poll`.
- **Termination Detection**: `StepState.IsTerminal()` correctly identifies final states (`completed`, `failed_terminal`, `timeout`, `cancelled`).
- **CLI Utility**: `orchestratorctl step wait` implements a basic polling loop for terminal or intervention (`needs_approval`) states.
- **Relay Contract**: `TaskSpec` includes `timeout_seconds` for planner-defined limits.

#### Identified Gaps
- **Server-Side Timeout Enforcement**: `RunService` ignores `TaskSpec.TimeoutSeconds`; it does not yet enforce execution limits natively.
- **Client-Side Robustness**: `orchestratorctl step wait` has no `--timeout` flag and provides no progress feedback (e.g., "Still running...").
- **State Transition Mismatch**: If an adapter hangs, the bridge remains in `running` forever.
- **Wait loop identity**: No way to `wait` for an entire Run or Phase, only individual Steps.
- **Machine-Use Consistency**: Polling intervals are hardcoded (2s) and not configurable for higher-frequency local testing.

#### Key Hardening Outcomes
- **Timeout Enforcement**: [RESOLVED] Updated `RunService` to enforce `TaskSpec.TimeoutSeconds` natively and transition to `StepStateTimeout`.
- **CLI Progress Feedback**: [RESOLVED] Added periodic progress indicators (.) to wait loops, redirected to `stderr`.
- **CLI Timeout Flag**: [RESOLVED] Implemented `--timeout` in `orchestratorctl` for client-side safety.
- **Interval Exposure**: [RESOLVED] Exposed configurable polling frequency via `--interval` flag.
- **Relay Alignment**: [RESOLVED] Ensured `stdout` remains a clean JSON stream for machine-usable terminal results.

## Terminal Outcome Semantics Audit (V1.4.5)

### Current Terminal Outcomes
- **Completed**: Successful execution as reported by the adapter.
- **Completed with Warnings**: Success, but with non-terminal issues (lint, non-breaking test failures).
- **Failed Terminal**: Hard failure requiring planner intervention or a new approach.
- **Failed Retryable**: Process failure that the bridge suggests can be retried (e.g. transient error).
- **Timeout**: Supervisor killed the process after exceeding `timeout_seconds`.
- **Cancelled**: Explicit terminal state triggered by user/planner abort.
- **Needs Manual Attention**: Generic bridge-side block requiring human eyes (relay stalled).
- **Needs Approval**: Specific policy gate block (Run is `paused_for_gate`).

### Key Hardening Outcomes
- **Aligned CLI Output**: [RESOLVED] Refined `orchestratorctl` run/step wait loops to use canonical terminal states in `stderr` and maintain clean JSON on `stdout`.
- **Relay Model Enforcement**: [RESOLVED] Removed language implying autonomous bridge-side decisions; outcomes are now strictly reported properties.
- **State Machine Coherence**: [RESOLVED] Hardened `internal/state/machine.go` to support transitions to `timeout` and `needs_manual_attention`.

## Local Dev Usability Audit (V1.5.5)

### Current Flow
- **Config**: JSON-driven via `LoadConfig`; no environment variable overrides for the daemon envelope.
- **Startup**: Manual `make setup` then `make run`. Simulation toggled via `ALL_ADAPTERS_SIMULATION_MODE=1`.
- **CLI**: Canonical surface for `run`/`step` control; JSON-first outputs.

### Friction Points
- **Env Divergence**: `DefaultConfig()` paths (`.artifacts`) mismatch `Makefile` setup (`.codencer/artifacts`).
- **Hidden Config**: Adapter binary paths (`CODEX_BINARY`) and simulate flags are ad-hoc env vars, not unified in `Config`.
- **Destructive Clean**: `make clean` nukes the database, preventing history persistence during dev.
- **No Init**: No `orchestratord init/doctor` to verify local environment readiness.

### Recommended Cleanup
- **Unified Config**: Support environment overrides (PORT, LOG_LEVEL, BINARY_PATHS) natively in `Config`.
- **Initialization Tooling**: [RESOLVED] Added `make dev` and `make doctor` to verify local environment readiness.
- **Path Alignment**: [RESOLVED] Unified all default paths to `.codencer/` root in `internal/app/config.go`.

## Readiness for Phase V1.6 (Local Usability)
- Default paths are aligned (`.codencer/`).
- Convenience targets (`make dev`, `make doctor`, `make nuke`) implemented.
- README Quickstart provides clear daily-use instructions.

## Readiness for Phase V1.5 (State Discovery)
- Canonical terminal outcomes are explicit and documented.
- `orchestratorctl` provides consistent, machine-usable terminal evidence.
- Bridge/Planner boundary is strictly factual (Bridge reports, Planner decides).
