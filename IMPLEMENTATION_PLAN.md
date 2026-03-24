# Codencer Implementation Plan

## Overview
Codencer is a local orchestration bridge for coding agents. It separates architectural planning from implementation execution, acting as a deterministic control plane that manages runs, state, policy gates, and artifacts.

## Execution Order

1. **Phase 1: Daemon skeleton**
   - Bootstrap the Go project and repository structure.
   - Implement configuration, logging, health endpoints, SQLite initialization, and artifact root initialization.
2. **Phase 2: Ledger + state machine**
   - Implement the domain models, SQLite repository schemas for runs/phases/steps/artifacts/gates, and state transition logic.
3. **Phase 3: Operator CLI**
   - Build `orchestratorctl` for interacting with the service layer to control lifecycle (start run, approve gate, etc.).
4. **Phase 4: Codex adapter MVP**
   - Implement the common adapter interface and the specific Codex adapter including subprocess invocation, timeout handling, and result normalization.
5. **Phase 5: Validation + artifacts**
   - Implement diff collection, validation runner (tests/lint), and robust artifact persistence.
6. **Phase 6: Policy engine + gates**
   - Build the rules evaluator to pause execution ("gate") for risk factors like broken tests or schema changes.
7. **Phase 7: Repo safety + worktrees**
   - Implement git dirty checks, file locks, and cleanup logic for runs.
8. **Phase 8: Hardening + recovery**
   - Make execution resumable and robust against restart, timeouts, and duplicates.
9. **Phase 9: MCP bridge**
   - Expose control primitives via the MCP protocol for external AI planners.
10. **Phase 10: DSL/schema hardening**
    - JSON schemas for defining tasks, policies, and results.
11. **Phase 11-16: Extensibility Pipeline**
    - Adapters for Claude Code and Qwen Code.
    - VS Code companion extension.
    - Generic IDE integration layers.

## Architecture Guidelines
- Strict separation of Domain, Services, State, Adapters, Storage, CLI.
- No cloud control plane; local-first only.
- State ledger (SQLite) is the source of truth, not in-memory logic or the IDE.
- Deterministic behavior: artifacts, policy gates, and typed interfaces.

## Dependencies
- Go 1.22+
- `github.com/mattn/go-sqlite3` for local persistence.
- Standard libraries heavily favored for CLI and sub-process supervision.
