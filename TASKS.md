# Tasks

## Priority 1 — Refactor and strengthen orchestration runtime
- [ ] Extract orchestration workflow out of monolithic `DispatchStep()`.
- [ ] Make run/step/attempt lifecycle transitions explicit.
- [ ] Ensure attempt start/poll/collect/normalize/validate/policy/gate/finalize is modeled cleanly.

## Priority 2 — Make the Codex path honest and robust
- [ ] Implement explicit configurable binary/args contract.
- [ ] Implement strong error classification and deterministic failure semantics.
- [ ] Add structured result expectations and artifact contract checks.
- [ ] Handle missing binaries explicitly.

## Priority 3 — Artifact, result, and validation retrieval
- [ ] Implement list artifacts flow for steps/attempts/runs.
- [ ] Retrieve step result in structured form via API.
- [ ] Expose validation results via API.
- [ ] Support retrieval through CLI and MCP.

## Priority 4 — Stronger policy model
- [ ] Implement policy loading/config usage aligned with docs rather than inline mocks.
- [ ] Persist or explicitly bind execution policy per step/run.
- [ ] Real changed-file detection feeding policy evaluation.
- [ ] Cleaner gate reason generation natively tied to policy rules.

## Priority 5 — Recovery and resumability
- [ ] Reconstitute incomplete attempts and paused runs on startup.
- [ ] Reconcile artifact directory presence vs DB state mismatch.
- [ ] Implement safe worktree and lock cleanup for interrupted processes.

## Priority 6 — MCP/control plane completion
- [ ] Add structured result retrieval tool to MCP.
- [ ] Add artifact listing tool to MCP.
- [ ] Add `retry_step` tool to MCP.
- [ ] Ensure machine-readable error payloads natively across MCP.

## Priority 7 — VS Code extension completion
- [ ] Show steps and gates with richer status information in the TreeView.
- [ ] Allow gate approve/reject operations natively from UI commands.
- [ ] Add explicit refresh controls.
- [ ] Expose artifacts/results via extension links if feasible.

## Priority 8 — Better tests
- [ ] Implement robust lifecycle state-transition tests without mock overrides.
- [ ] Write integration tests for API endpoints covering retrieval flows.
- [ ] Formalize non-simulated path assertions ensuring real failure boundaries are caught.
