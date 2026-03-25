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
- [x] Update README and GAP_AUDIT for routing truthfulness.
