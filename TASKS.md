# Tasks

## Priority 1 — Refactor and strengthen orchestration runtime [COMPLETE]
- [x] Extract orchestration workflow out of monolithic `DispatchStep()`.
- [x] Make run/step/attempt lifecycle transitions explicit.
- [x] Ensure attempt start/poll/collect/normalize/validate/policy/gate/finalize is modeled cleanly.

## Priority 2 — Make the Codex/Claude/Qwen paths honest and robust [BATCH 5 COMPLETE]
- [x] Implement explicit configurable binary/args contract.
- [x] Implement strong error classification and deterministic failure semantics.
- [x] Add structured result expectations and artifact contract checks.
- [x] Handle missing binaries explicitly.

## Priority 3 — Artifact, result, and validation retrieval [COMPLETE]
- [x] Implement list artifacts flow for steps/attempts/runs.
- [x] Retrieve step result in structured form via API.
- [x] Expose validation results via API.
- [x] Support retrieval through CLI and MCP.

## Priority 4 — Stronger policy model [COMPLETE]
- [x] Implement policy loading/config usage aligned with docs (via `PolicyRegistry`).
- [x] Terminology: Rename `Result.Status` to `Result.State` for cross-model consistency.
- [x] Service: Repair `finalizeStep` to correctly integrate policy evaluation and gating.
- [x] Testing: Fix test regressions and ensure reliable simulation behavior.
- [x] Docs: Finalize README and SETUP guides for MVP hardening review.
- [x] Persist or explicitly bind execution policy per step/run.
- [x] Real changed-file detection feeding policy evaluation.
- [x] Cleaner gate reason generation natively tied to policy rules.

## Priority 5 — Recovery and resumability [COMPLETE]
- [x] Reconstitute incomplete attempts and paused runs on startup.
- [x] Reconcile artifact directory presence vs DB state mismatch.
- [x] Implement safe worktree and lock cleanup for interrupted processes.

## Priority 6 — MCP/control plane completion [COMPLETE]
- [x] Add structured result retrieval tool to MCP.
- [x] Add artifact listing tool to MCP.
- [x] Add `retry_step` tool to MCP. [FIXED RunID bug]
- [x] Ensure machine-readable error payloads natively across MCP. [STRUCTURED JSON]

## Priority 7 — VS Code extension completion [BATCH 4 COMPLETE]
- [x] Show steps and gates with richer status information in the TreeView.
- [x] Allow gate approve/reject operations natively from UI commands.
- [x] Add explicit refresh controls.
- [x] Expose artifacts/results via extension links and JSON buffers.

## Priority 8 — Better tests [COMPLETE]
- [x] Implement robust lifecycle state-transition tests (Added routing/recovery suites).
- [x] Write integration tests for API endpoints covering retrieval flows (`api_test.go`).
- [x] Formalize non-simulated path assertions ensuring real failure boundaries are caught.

## Priority 9 — Routing & Benchmark Hardening [BATCH 6 COMPLETE]
- [x] Make routing behavior explicit (rename and document static logic).
- [x] Implement `is_simulation` tracking in benchmarks.
- [x] Persist selection reasons and fallback paths in attempts.
- [x] Expose benchmark and routing config via API/CLI.
## Priority 10 — Final Consistency & Polish [COMPLETE]
- [x] Standardize internal state terminology.
- [x] Audit vocabulary alignment (Completed/Failed).
- [x] Remove character artifacts and stale comments.
- [x] Update documentation for retroactive truthfulness.

- [x] Ensure MCP tools use canonical schema validation logic.

## Phase 14 — State & Simulation Clarification [COMPLETE]
- [x] Standardize `StepState` vocabulary (timeout, needs_manual_attention).
- [x] Document explicit simulation semantics and contract representation.
- [x] Align schemas and examples with clarified state/simulation fields.

## Batch V1.1.2 — State & Terminology Hardening [/]
- [x] Audit execution state model (Micro-task complete) <!-- id: 40 -->
- [x] Define and document canonical execution/result state semantics <!-- id: 45 -->
- [x] Clarify lifecycle meaning of Runs, Steps, and Attempts <!-- id: 46 -->
- [x] Align manual-attention and simulation semantics with relay model <!-- id: 47 -->
- [ ] Decouple Attempt states from StepState <!-- id: 41 -->
- [ ] Standardize `ValidationState` and `GateState` (remove "Status" suffix) <!-- id: 43 -->
- [ ] Update supervisor to natively trigger `StepStateTimeout` <!-- id: 44 -->

## Phase 16: Planner-Facing CLI Surface (V1.2) [x]
- [x] Audit existing CLI and identify gaps (V1.2.1 Complete) <!-- id: 50 -->
- [x] Align task submission with canonical contract (V1.2.1 Complete) <!-- id: 55 -->
- [x] Implement machine-readable JSON output (V1.2.1 Complete) <!-- id: 51 -->
- [x] Refine CLI for reliable machine-readability (Batch V1.2.1 Complete) <!-- id: 56 -->
- [x] Audit wait/result retrieval paths (Batch V1.2.2 Micro-task complete) <!-- id: 57 -->
- [x] Align structured result retrieval CLI (Batch V1.2.2 Micro-task complete) <!-- id: 58 -->
- [x] Add `wait` support for terminal state monitoring (Batch V1.2.2 Complete) <!-- id: 54 -->
- [x] Refine CLI wait/result consistency (Batch V1.2.2 Complete) <!-- id: 59 -->
- [ ] Implement state discovery (run/step listing)
- [ ] Expose Telemetry and Routing CLI groups <!-- id: 53 -->

- [x] Audit Codex adapter and identify hardening requirements (V1.3.1 Complete) <!-- id: 61 -->
- [x] Validate adapter binary availability in `Start()` (V1.3.1 Complete) <!-- id: 62 -->
- [x] Pass canonical task metadata to adapter command invocation (V1.3.1 Complete) <!-- id: 63 -->
- [x] Implement Codex-specific result normalization and outcome mapping (V1.3.1 Complete) <!-- id: 64 -->
- [x] Align Codex adapter reporting with relay contracts (V1.3.1 Complete) <!-- id: 65 -->
- [x] Audit Codex artifact harvesting flow (V1.3.1 Complete) <!-- id: 67 -->
- [x] Harden Codex artifact discovery and metadata capture (V1.3.1 Complete) <!-- id: 68 -->
- [x] Align Codex harvested outputs with canonical contracts (Batch V1.3.2 Complete) <!-- id: 70 -->
- [x] Finalize Batch V1.3.2 alignment (Complete) <!-- id: 71 -->
- [x] Define and document local validation scenario (V1.3.2 Complete) <!-- id: 72 -->
- [x] Add/align practical local validation path (V1.3.2 Complete) <!-- id: 73 -->
- [x] Improve observable success/failure evidence (V1.3.2 Complete) <!-- id: 74 -->
- [x] Add/strengthen validation coverage (V1.3.2 Complete) <!-- id: 75 -->
- [x] Final alignment for validation scenario (Batch V1.3.3 Complete) <!-- id: 76 -->
- [x] Audit execute-and-wait loop behavior (Phase V1.4.1 Complete) <!-- id: 80 -->
- [x] Audit execute-and-wait loop behavior (Phase V1.4.1 Complete) <!-- id: 80 -->
- [x] Implement native timeout enforcement in RunService (V1.4.2 Complete) <!-- id: 81 -->
- [x] Add/align practical timeout and polling controls (V1.4.3 Complete) <!-- id: 82 -->
- [x] Align wait flow output with relay contract (V1.4.4 Complete) <!-- id: 110 -->
- [x] Final alignment for Batch V1.4.1 (V1.4 Complete) <!-- id: 111 -->
- [x] Audit terminal outcome semantics (V1.4.5 Complete) <!-- id: 115 -->
- [x] Clarify canonical terminal outcome meanings (V1.4.6 Complete) <!-- id: 118 -->
- [x] Clarify manual-attention and retry reporting (V1.4.7 Complete) <!-- id: 119 -->
- [x] Align terminal outcome semantics in CLI/Result output (V1.4.8 Complete) <!-- id: 120 -->
- [x] Final alignment for Batch V1.4.2 (V1.4.9 Complete) <!-- id: 121 -->
- [x] Audit local dev/config workflow (V1.5.5 Complete) <!-- id: 125 -->
- [ ] Implement unified config with env overrides <!-- id: 126 -->
- [x] Align default paths and improve local startup (V1.6.1 Complete) <!-- id: 127 -->
- [x] Clarify execution modes and config expectations (V1.6.2 Complete) <!-- id: 130 -->
- [ ] Decouple attempt outcomes from orchestrator states <!-- id: 116 -->
- [ ] Consolidate intervention states (NeedsApproval/NeedsManualAttention) <!-- id: 117 -->
- [ ] Expose Telemetry and Routing CLI groups
