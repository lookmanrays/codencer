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

## Phase 7: Repo Safety + Worktrees
- [x] Pending dirty checks, lock management, worktrees, and cleanup.mentation.

## Phase 8: Hardening + Recovery
- [x] Pending restart recovery, cancellation, retry backoff.

## Phase 9: MCP Bridge
- [x] Pending MCP server layer and tool exposure.

## Phase 10: DSL/Schema Hardening
- [x] Pending JSON schemas for task/result/policy.

## Phase 11: IDE Extension MVP
- [x] Pending mock extension scaffolding and daemon connectivity.

## Phase 12-13: Secondary Adapters
- [x] Pending Claude and Qwen adapter structure.

## Phase 5: Validation + Artifacts
- [x] Pending artifact persistence and validation commands.

## Phase 6: Policy Engine + Gates
- [x] Pending evaluation and threshold checks.
