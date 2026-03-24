# Tasks

## Phase 1: Daemon Skeleton
- [x] Initialize go module (`go mod init agent-bridge`).
- [x] Create repository directory structure (`cmd/`, `internal/app/`, `internal/domain/`, etc.).
- [x] Implement `internal/app/bootstrap.go`, `config.go`, and `version.go`.
- [x] Implement core entrypoints in `cmd/orchestratord/main.go`, `cmd/orchestratorctl/main.go`.
- [x] Setup structured logging foundation (zap or text/slog).
- [x] Implement basic health endpoint (`/health`).
- [x] Create SQLite initialization scaffolding and artifact root folder (`.artifacts/`).
- [x] Add Makefile and scripts for test/lint/dev.
- [x] Create `AGENTS.md` containing the AI IDE Rules.
- [x] Add unit tests for configuration loading and bootstrap behavior.

## Phase 2: Ledger + State Machine
- [x] Pending creation of `internal/domain/` components (run, phase, step, attempt, artifact, gate, validation, policy, adapter).
- [x] Pending SQLite schemas and repository implementations.
- [x] Pending state machine logic and transitions logic.

## Phase 4: Codex Adapter MVP
- [x] Adapter uses subprocess to simulate valid execution, collects artifacts into workspace, and unmarshals real JSON result output.

## Phase 5: Validation + Artifacts
- [x] sqlite validation mappings complete and working.

## Phase 6-7: State Machine + CLI
- [x] True database repository querying implemented handling ledger lists. function minimally.

## Phase 7: Repo Safety + Worktrees
- [x] Basic lock management and worktree shell commands implemented.

## Phase 8: Hardening + Recovery
- [ ] INCOMPLETE: `SweepStaleRuns` is a commented-out conceptual mock. Needs real DB queries.

## Phase 9: MCP Bridge
- [x] Basic HTTP tool bindings exist.

## Phase 10: DSL/Schema Hardening
- [x] JSON schemas created for task/result/policy.

## Phase 11: IDE Extension MVP
- [ ] INCOMPLETE: Extension is just a scaffold. Needs real UI/Webview/TreeDataProvider.

## Phase 12-13: Secondary Adapters
- [ ] INCOMPLETE: Claude and Qwen adapters exist as dead-code `time.Sleep` stubs.

## Phase 5: Validation + Artifacts
- [x] Pending artifact persistence and validation commands.

## Phase 6: Policy Engine + Gates
- [x] Pending evaluation and threshold checks.
