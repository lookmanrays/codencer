# Progress

## Phase 1: MVP Foundation (Completed)
- [x] Daemon bootstrap & SQLite Ledger
- [x] State machine & Transitions
- [x] Basic Run/Step persistency
- [x] Initial CLI & MCP layout
- [x] Skeleton VS Code Extension
- [x] Local Adapter Subprocess wrappers (Codex)

## Phase 2: Runtime Hardening & MVP Refinement (Completed)
- [x] Refactor & Strengthen Orchestration Runtime
- [x] Make Codex Path Honest and Robust
- [x] Artifact, Result, and Validation Retrieval
- [x] Stronger Policy Model
- [x] Recovery and Resumability
- [x] MCP/Control Plane Completion
- [x] VS Code Extension Completion
- [x] Stronger Tests

## Phase 3: Multi-Agent Expansion & DSL Hardening (Completed)
- [x] Formalize DSL Schemas for TaskSpec and ResultSpec
- [x] Implement Validation Parsers for Explicit Payload Ingestion
- [x] Plumb YAML Task Payload consumption directly into `orchestratorctl` CLI
- [x] Scaffold Native Claude Code Adapter boundaries mapping anthropic capabilities
- [x] Scaffold Native Qwen Adapter boundaries mapping local capabilities
- [x] Deploy Universal Adapter Conformance Tests guaranteeing capability parity

## Phase 4: Benchmarking & IDE Chat Adapters (Completed)
- [x] Implement Benchmark Metrics & SQLite Scoring Ledger
- [x] Author `RoutingService` with Smart Fallback Chaining
- [x] Scaffold Targeted IDE Chat Adapter for Extension-bound Proxies
- [x] Deploy Compatibility Matrix Diagnostic Endpoints
- [x] Complete E2E Validation of the Fully Operational Bridge

## Phase 5: Orchestration & MCP Correctness (Completed)
- [x] Decompose `RunService.DispatchStep` Lifecycle
- [x] Fix `ToolRetryStep` Run Identity Resolution
- [x] Implement Structured JSON Outputs for MCP Tools
- [x] Harden Worktree Setup and Collision Handling
- [x] Decouple Recovery Paths and Improve State Reconcile
- [x] Inject Configuration-driven Paths into RunSvc (Self-Review)
- [x] Implement Step Idempotency for MCP Retries (Self-Review)

## Phase 6: Retrieval & Inspection Hardening (Completed)
- [x] Modernize `Artifact` and `ValidationResult` Domain Models
- [x] Hardened SQLite Schema for Detailed Evidence
- [x] Implement Structured Retrieval in `RunService`
- [x] Expose REST/MCP Endpoints for Validations
- [x] Native CLI Commands for Inspection
- [x] Verified missing-state robustness with unit tests

## Phase 7: Recovery & Reconciliation Hardening (Completed)
- [x] Implement Exclusive Workspace Locking
- [x] Add `RecoveryNotes` to Run Ledger
- [x] Deep Reconciliation Engine for Stale Attempts
- [x] Orphaned Worktree and Lock Cleanup
- [x] Exposed Recovery Status to API/CLI/MCP
- [x] Verified with Recovery Integration Tests

## Phase 8: VS Code Control Surface (Completed)
- [x] Hierarchical Run -> Step -> [Gate, Artifact, Validation] Tree
- [x] Status-aware Icons and Rich Tooltips
- [x] Centralized API Client with Error Handling
- [x] Actionable Commands: Approve/Reject Gate, Retry Step
- [x] Structured Inspection: View Results/Validations in JSON buffers
- [x] Corrected Backend Support for Step Retries
 
## Phase 9: Routing & Benchmark Hardening (Completed)
- [x] Explicit Heuristic Routing (Deterministic Fallback)
- [x] Simulation-Aware Benchmark Persistence
- [x] REST API for Telemetry and Routing Configuration
- [x] MCP Tools for Tactical Transparency
- [x] Honest Documentation for Adapter Logic

## Phase 10: Final Hardening (Completed)
- [x] Configuration-driven Policy Engine (`PolicyRegistry`)
- [x] Policy-bound Step Evaluation
- [x] Robust API Integration Test Suite
## Phase 12: Final Acceptance & Truth Audit (Completed)
- [x] Audited CLI / API / MCP surface for functional completeness.
- [x] Verified 100% test and build stability.
- [x] Standardized all terminology (`State`, `lowercase_properties`).
- [x] Produced Feature Status Matrix in `GAP_AUDIT.md`.
- [x] Formalized Reviewer Guide in `README.md`.
- [x] Qualified all project maturity claims with technical honesty.

## Phase 13: Relay Contract Consolidation (Completed)
- [x] Audit existing task/input/result/state contracts (Micro-task complete)
- [x] Define canonical planner-facing input contract (TaskSpec)
- [x] Synchronize ResultSpec JSON schema with Go domain models
- [x] Create canonical relay-facing result scaffold

## Phase 14: State & Simulation Clarification (Completed)
- [x] Clarify execution state semantics (timeout, needs_manual_attention)
- [x] Formalize simulation semantics in output contract and documentation

## Phase 15: State & Terminology Hardening [DONE]
- [x] Audit execution state model (Micro-task complete)
- [x] Define and document canonical execution/result state semantics
- [x] Clarify lifecycle meaning of Runs, Steps, and Attempts
- [x] Align manual-attention and simulation semantics
- [x] Terminology uniformity (State vs Status)
- [-] Refactor Attempt state management (Moved to V1.1.3)
- [-] Consolidate intervention signaling (Moved to V1.1.3)

## Phase 16: Planner-Facing CLI Surface [x]
- [x] Audit existing CLI and identify gaps (Micro-task complete)
- [x] Align task submission with canonical contract (Micro-task complete)
- [x] Standardize machine-readable JSON output (Micro-task complete)
- [x] Refine CLI for reliable machine-readability (Batch V1.2.1 Complete) <!-- id: 56 -->
- [x] Audit wait/result retrieval paths (Batch V1.2.2 Micro-task complete) <!-- id: 57 -->
- [x] Align structured result retrieval CLI (Batch V1.2.2 Micro-task complete) <!-- id: 58 -->
- [x] Add `wait` support for terminal state monitoring (Batch V1.2.2 Complete) <!-- id: 54 -->
- [x] Refine CLI wait/result consistency (Batch V1.2.2 Complete) <!-- id: 59 -->
- [ ] Implement `run list` and `step list` commands <!-- id: 52 -->
- [ ] Expose Telemetry and Routing CLI groups
