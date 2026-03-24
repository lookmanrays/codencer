# IMPLEMENTATION TASKS

## Priority 1: Core Orchestration Engine
- [ ] Create `OrchestrationService` (or expand `RunService`) with full `dispatch -> poll -> collect -> validate -> evaluate -> gate/complete` loop.
- [ ] Implement `Step` creation and transition logic natively.
- [ ] Implement `Attempt` persistence and lifecycle tracking.

## Priority 2: Real Codex-First Flow
- [ ] Upgrade Codex adapter to execute configurable external commands.
- [ ] Refactor `CollectArtifacts` to parse actual disk footprints dynamically without hardcoding.
- [ ] Ensure stdout/stderr/diffs are correctly wired into artifact persistence.

## Priority 3: CLI Completion
- [ ] Implement `orchestratorctl step start`.
- [ ] Implement `orchestratorctl step result`.
- [ ] Implement `orchestratorctl gate approve/reject`.

## Priority 4: MCP / Control Plane
- [ ] Add missing endpoints `start_step`, `get_result`, `list_artifacts`, `approve/reject_gate`, `retry_step`, `run_validations`.
- [ ] Implement strict input/output error taxonomies to spec.

## Priority 5: Policy, Gates, and Retry
- [ ] Implement threshold rules (changed files, dependency detection).
- [ ] Wire validation failure gating to run pauses.
- [ ] Hook gate approvals back into Run/Step resumptions.

## Priority 6: Recovery and Resumability
- [ ] Improve restart sweep so paused runs are preserved.
- [ ] Allow stale processes to be correctly detected and retried.

## Priority 7: VS Code Companion Extension
- [ ] Enhance TreeDataProvider with run details, step state, and gate actions.
- [ ] Render artifact links for exploration.

## Priority 8: Secondary Adapters
- [ ] Unify configuration paths for Claude/Qwen.
- [ ] Raise explicit limitation boundaries if CLI tools are missing.

## Priority 9: True End-to-End Test
- [ ] Write robust `scripts/e2e_test.sh` automating run creation, adapter dispatch, artifact inspection, and policy gating.
