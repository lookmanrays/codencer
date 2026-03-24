# RUTHLESS GAP AUDIT (V2)

## 1. Core Orchestration Engine (Missing)
The existing `RunService` manages basic Run CRUD (Start, List, Get, Abort). However, a true orchestrator pipeline is entirely absent:
- No `Step` lifecycle management.
- No `Attempt` tracking or creation.
- No central runner/dispatcher that coordinates Adapter Start -> Poll -> Collect Artifacts -> Normalize -> Validation Run -> Policy Eval -> Gate -> Complete/Retry limits.

## 2. Operator CLI (Partial/Missing)
`orchestratorctl` currently mimics phase 3 superficially:
- Has `run start` and `run status`.
- Missing `step start` and `step result`.
- Missing robust lifecycle commands `gate approve/reject`.

## 3. MCP Control Plane (Missing)
`internal/mcp/server.go` and `tools.go` export a few tools but entirely lack the actual comprehensive tool surface:
- Missing `orchestrator.start_step`, `get_result`, `list_artifacts`, `reject_gate`, `retry_step`, `run_validations`.
- Missing firm machine-readable error handling according to MCP schema specs.

## 4. Codex Adapter Integration (Fake / Simulated)
`InvokeLocal` executes a bash script `echo`-ing a fake `result.json`. It does not execute a real Codex CLI, nor does it define the strict configurable execution command path required for local-first deployment.
- `CollectArtifacts` uses hardcoded struct responses instead of inspecting real disk footprints.

## 5. Secondary Adapters (Simulators)
Claude and Qwen adapters are placeholder simulators (`time.Sleep` or bash echo analogs).
- Required: Genuine shared adapter infrastructure to execute external binaries or explicitly handle auth/binary absence with honest error logging and degradation.

## 6. VS Code Extension (Minimal)
Currently just a TreeDataProvider showing a hardcoded string or basic run lists.
- Required: Per-run detail UI, gate approval mechanisms, phase status inspection, and actual artifact URI hooks.

## 7. Recovery, Tests, and Validation (Shallow)
- `e2e_test.sh` proves string insertion, not actual orchestration.
- `RecoveryService` just fails everything on restart. Resumability of paused runs is missing.
- Verification tests do not validate policy/gate logic.

## Summary
The codebase is a **scaffold**. The control plane is not fully wired to a lifecycle engine.
