# Detailed Roadmap

## Phase 0 — Spec lock
Deliver:
- product definition
- architecture
- state machine
- repo structure
- schemas
- policy taxonomy
- implementation sequence

Exit:
- MVP scope frozen
- adapter contract frozen
- no open contradictions in docs

## Phase 1 — Daemon skeleton
Implement:
- bootstrap
- config loading
- logging
- health/version endpoint
- SQLite bootstrap
- artifact root init

Exit:
- daemon starts
- config validates
- migrations run
- health endpoint works

## Phase 2 — Ledger + state machine
Implement:
- domain entities
- SQLite schema
- repos/services
- state machine
- transition validation
- audit trail

Exit:
- transitions are explicit
- invalid transitions rejected
- audit events persisted

## Phase 3 — Operator CLI
Implement:
- run start
- run status
- step start
- step result
- approve/reject gate
- abort run

Exit:
- operator can manage lifecycle without DB access

## Phase 4 — Codex adapter MVP
Implement:
- common adapter interface
- Codex adapter
- invocation
- process capture
- timeout handling
- cancellation handling
- normalized result shell

Exit:
- one real step can run end-to-end via Codex

## Phase 5 — Validation + artifacts
Implement:
- validation runner
- diff collection
- changed files collection
- artifact persistence
- validation result persistence

Exit:
- step ends with artifacts and validations persisted

## Phase 6 — Policy engine + gates
Implement:
- policy evaluator
- gate creation
- approve/reject flow
- retry semantics
- threshold rules

Initial gates:
- validation fails
- dependency files changed
- migration detected
- forbidden path touched
- too many files changed
- unresolved questions

Exit:
- risky steps pause instead of silently continuing

## Phase 7 — Repo safety + worktrees
Implement:
- dirty repo checks
- lock management
- optional worktree allocation
- cleanup path
- run-specific workspace management

Exit:
- repo safety story is real

## Phase 8 — Hardening + recovery
Implement:
- daemon restart recovery
- stale process detection
- resumable runs
- cancellation
- retry backoff
- idempotency protections

Exit:
- interrupted runs are recoverable and inspectable

## Phase 9 — MCP bridge
Implement:
- MCP server
- tool mapping
- request validation
- error taxonomy
- compact result payloads

Exit:
- planner can control orchestrator through MCP

## Phase 10 — DSL/schema hardening
Implement:
- task schema
- result schema
- policy schema
- semantic validation
- examples

Exit:
- ad hoc prompts replaced by execution contracts

## Phase 11 — Claude Code adapter
Implement:
- Claude adapter
- capability metadata
- normalization
- conformance tests

Exit:
- same TaskSpec runs on Codex and Claude

## Phase 12 — Qwen Code adapter
Implement:
- Qwen adapter
- capability metadata
- normalization
- conformance tests

Exit:
- same TaskSpec runs on Qwen

## Phase 13 — Benchmark harness + routing
Implement:
- benchmark corpus
- per-adapter scoring
- routing profiles
- fallback strategy

Exit:
- orchestrator can recommend/select adapter

## Phase 14 — VS Code companion extension
Implement:
- extension skeleton
- daemon communication
- status views
- gate actions
- artifact links

Exit:
- local control and visibility from VS Code

## Phase 15 — One targeted IDE chat adapter
Implement:
- one supported IDE/agent chat bridge only
- prompt submission
- completion detection
- transcript/result capture
- fallback to CLI path

Exit:
- one supported pair is stable and documented

## Phase 16 — VS Code-like IDEs + Antigravity
Implement:
- compatibility matrix
- extension support adjustments
- Antigravity validation path
- daemon-only fallback mode

Exit:
- support is tested, not assumed
