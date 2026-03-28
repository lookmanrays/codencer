# Codencer Gap Audit & Readiness
> [!WARNING]
> **INTERNAL DEVELOPER DOCUMENT**: This file is for project maintainers and contains technical debt audits, task backlogs, and roadmap tracking.
> For the official **User Guide**, please refer to the [README.md](../../README.md).

## Current Reality
The repository contains a functionally operational MVP implementation of the orchestration bridge. It successfully integrates a SQLite ledger, a robust state machine, a `DispatchStep` orchestrator loop, CLI endpoints, basic MCP routes, and a skeletal VS Code extension.

- **Lifecycle Meaning Cleanup**: [RESOLVED] Explicitly defined Run (Session), Step (Planner Unit), and Attempt (Execution Try) in domain code and README. Verified that no bridge-side decision logic is implied.
- **Terminology Inconsistency**: [RESOLVED] Renamed all outcome indicators to `State` (RunState, StepState, Result.State) for uniform operator experience.
- **Ergonomics**: [RESOLVED] Tightened the `submit` -> `wait` -> `result` sequence and established the **Canonical Local Runbook** in `EXAMPLES.md`.
- **Trust & Transparency**: [RESOLVED] Added "Known Limitations" and clarified the distinction between simulation and real-mode execution in README.

## Feature Status Matrix

| Component | Status | Implementation Type | Notes |
| :--- | :--- | :--- | :--- |
| **Orchestration Core** | ✅ **Ready (Beta)** | Native (SQLite) | Persistent ledger, state machine, and Git Worktrees. |
| **CLI & MCP Layer** | ✅ **Ready (Beta)** | Native | Human-readable hints, logs, and structured JSON. |
| **Codex Adapter** | ✅ **Ready (Beta)** | CLI Wrapper | High-fidelity relay with artifact harvesting. |
| **Claude/Qwen Adapters** | 🟡 **Functional** | CLI Wrapper | Basic subprocess wrappers; lacks deep extraction. |
| **Simulation Mode** | ✅ **Ready (Beta)** | Native | Robust stubs for orchestrator validation. |
| **Adaptive Routing** | 🧪 **Prototype** | Heuristic | Static fallback chain; not yet benchmark-driven. |
| **Governance** | ✅ **Ready (Beta)** | Manual | MIT Licensed; `CONTRIBUTING.md` authored. |

## Known Technical Debt & Limitations
- **Adaptive Routing**: Routing is currently based on a static heuristic chain; benchmark-driven optimization is documented but not dynamic.
- **Process Introspection**: CLI-wrapped adapters provide limited visibility beyond standard streams.
- **Simulation Limits**: Simulation Mode stubs all actions; it validates the orchestrator's state-machine but does not test real agent logic.

## V1 Publication Audit (Phase V1.F3)

### 🚨 Critical Publication Blockers (Must Fix)
1. **LICENSE**: ✅ RESOLVED (MIT).
2. **CONTRIBUTING.md**: ✅ RESOLVED.
3. **Repository Noise**: ✅ RESOLVED (`codencer.db` removed/ignored).
4. **Makefile Version**: ✅ RESOLVED (`v0.1.0`).
5. **Setup Reproducibility**: ✅ RESOLVED (`make setup build` verified).

### 🛡 Trust & Readability Gaps (Should Fix)
1. **Agent Versioning**: `doctor` command detects binary existence but not version compatibility (e.g. `codex-agent` v0.5 vs v1.0).
2. **Internal Documentation Noise**: ✅ RESOLVED (Upgraded headers and README navigation).
3. **Example Parity**: Ensure `examples/tasks/*.yaml` are 100% compliant with the latest schema.

### 🛠 Ready for Release Finalization
1. **Governance**: ✅ RESOLVED (MIT & CONTRIBUTING.md).
2. **Health**: Enhance `doctor` with version checks for `git` and `sqlite3`.
3. **Documentation Quality Audit**: ✅ PASS (Rendering issues fixed; Quickstart aligned).

### 📖 V1 Documentation Audit (Readiness Check)
- **README Rendering**: Critical unclosed ` ```bash ` at line 60 (swallows 50% of content). Redundant separators.
- **Quickstart Inconsistencies**: 
    - Divergent RunIDs (`my-first-run` vs `my-first-mission`).
    - Divergent `submit` ergonomics (`submit --wait` vs explicit `step wait` in smoke test).
    - Missing projectID in some examples.
- **Current Status**: All critical V1 blockers resolved.

### 🕹 Operator Flow Audit (Readiness Check)
- **Current Flow**: `run start` -> `submit --wait` -> `step result/logs`.
- **Ambiguity Points**:
    - **ID Friction**: The server-generated UUID is the required handle, but users must manually copy-paste it from `submit` output. The YAML `step_id` is not the primary handle.
    - **Result Overlap**: `step wait` outputs the same JSON as `step result`, leading to "What now?" confusion.
    - **Discovery**: `smoke_test.sh` uses brittle grep/cut to find IDs; this highlights a programmatic friction point.
- **Next Fix**: Standardize `smoke_test.sh` for programmatic use (V1.x Roadmap).

### ⚖ Trust & Readiness Audit (Readiness Check)
- **Codex Status**: ✅ RESOLVED (Maturity Matrix updated to `Ready (Beta)`).
- **Quickstart Speed**: ✅ RESOLVED (Speed claims softened; focused on simulation).
- **Maturity Alignment**: ✅ RESOLVED (Beta qualifiers applied consistently).

- [x] Audit Trust & Readiness Alignment (Final Alignment Complete)
- [ ] Align Smoke Test & Implement "Latest" ID Alias (V1.x Roadmap)
