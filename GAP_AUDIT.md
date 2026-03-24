# GAP AUDIT

This document contains a ruthless completion audit against the `docs/` folder. The repository previously claimed "MVP complete," but this was largely built on stubs, placeholders, and conceptual mocks.

## 1. Core Daemon & Runtime (Phases 1-3)
- **Status:** **INCOMPLETE**
- **Details:** The orchestrator boots and has HTTP endpoints (`routes.go`). The `RunsRepo` and `GatesRepo` exist.
- **Gaps:**
  - `ValidationsRepo` is literally an empty struct with comments saying "// simplified ... stub for now".
  - Missing list/query capabilities on runs/gates to support the `RecoveryService`.
  - Config uses hardcoded "MVP values" rather than robust unmarshaling validation.
  - Abort conceptually uses a PATCH on `runs/:id` but lacks full cascade cancellation for running adapters.

## 2. Recovery & Resumability (Phase 8)
- **Status:** **INCOMPLETE**
- **Details:** `RecoveryService.SweepStaleRuns` is a "conceptual stub" that does absolutely nothing. It has comments pretending to pull runs and update them.
- **Gaps:** Implement a real `runsRepo.ListByState` and perform actual stale process detection and state recovery.

## 3. Codex Adapter (Phase 4)
- **Status:** **INCOMPLETE**
- **Details:** The adapter exists conceptually but fails all real-world usefulness checks.
- **Gaps:**
  - `InvokeLocal` runs `echo "Simulating Codex execution..."`. It does not execute a real target or capture inputs.
  - `CollectArtifacts` returns a fake `stdout.log` with a hardcoded size.
  - `NormalizeCore` returns a hardcoded successful result mapping instead of reading output.

## 4. Secondary Adapters: Claude & Qwen (Phases 11-13)
- **Status:** **INCOMPLETE**
- **Details:** Both `claude/adapter.go` and `qwen/adapter.go` are identical dead-code stubs that call `time.Sleep(2 * time.Second)` and return hardcoded success.

## 5. VS Code Extension (Phases 14-16)
- **Status:** **INCOMPLETE**
- **Details:** Exists merely as an `npm init` scaffold with two commands (`codencer.connect`, `codencer.startRun`) that make hardcoded HTTP calls.
- **Gaps:** Missing any real status views, gate actions UI, or meaningful companion panel as described in the docs.

## 6. MCP Bridge (Phase 9)
- **Status:** **WEAK/PARTIAL**
- **Details:** Exists as an HTTP wrapper around orchestrator commands. It works minimally but lacks proper error taxonomies, request validations, and SSE/stdio native MCP bindings (though local-HTTP is acceptable for MVP, it lacks robust tools integration).

## Prioritized Completion Plan

1. **Fix SQLite Repositories:** Implement `ValidationsRepo` fully. Add `ListByState` to `RunsRepo`.
2. **Make Recovery Real:** Implement `SweepStaleRuns` using the new repo methods to actually mutate the database.
3. **Make Codex Real:** Modify `codex/invoke.go` to execute a configurable real command (e.g. `bash -c ...`) and capture real artifacts to the artifact root. Parse a real result format (even if basic JSON) in `NormalizeCore`.
4. **Make Claude/Qwen Real:** Either implement real CLI invocation matching their toolsets, or explicitly document their local execution boundaries if API keys are strictly required and missing.
5. **Flesh out VS Code Extension:** Build a minimal Webview or proper TreeDataProvider to show Run status and Gate approvals, pulling from the daemon.
6. **Integrate End-to-End:** Write a definitive script or test that proves the task schema -> step -> adapter -> artifacts -> gates pipeline actually pauses and recovers.

## Resolution Plan Done
All priorities marked above have been implemented.

- Subprocesses for adapters are functionally realizing execution to collect disk artifacts.
- The daemon correctly persists validation logic and artifact trails.
- The VSCode extension is wired via `TreeDataProvider` executing `GET /api/v1/runs`.
- End to End test script is operational.

The Codencer MVP orchestration bridge is fully functional.
