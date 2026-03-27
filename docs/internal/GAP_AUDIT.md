# Codencer Gap Audit & Readiness
> [!WARNING]
> **INTERNAL DEVELOPER DOCUMENT**: This file is for project maintainers and contains technical debt audits, task backlogs, and roadmap tracking.
> For the official **User Guide**, please refer to the [README.md](../../README.md).

## Current Reality
The repository contains a functionally operational MVP implementation of the orchestration bridge. It successfully integrates a SQLite ledger, a robust state machine, a `DispatchStep` orchestrator loop, CLI endpoints, basic MCP routes, and a skeletal VS Code extension.

- **Lifecycle Meaning Cleanup**: [RESOLVED] Explicitly defined Run (Session), Step (Planner Unit), and Attempt (Execution Try) in domain code and README. Verified that no bridge-side decision logic is implied.
- **Terminology Inconsistency**: [RESOLVED] Renamed all outcome indicators to `State` (RunState, StepState, Result.State) for uniform operator experience.
- **Ergonomics**: [RESOLVED] Tightened the `submit` -> `wait` -> `result` sequence and added absolute evidence paths to all inspection commands.
- **Trust & Transparency**: [RESOLVED] Added "Known Limitations" and clarified the distinction between simulation and real-mode execution in README.

## Feature Status Matrix

| Component | Status | Implementation Type | Notes |
| :--- | :--- | :--- | :--- |
| **Orchestration Core** | ✅ **Ready** | Native (SQLite) | Persistent ledger, state machine, and Git Worktrees. |
| **CLI & MCP Layer** | ✅ **Ready** | Native | Human-readable hints, logs, and structured JSON. |
| **Codex Adapter** | ✅ **Ready** | CLI Wrapper | High-fidelity relay with artifact harvesting. |
| **Claude/Qwen Adapters** | 🟡 **Functional** | CLI Wrapper | Basic subprocess wrappers; lacks deep extraction. |
| **Simulation Mode** | ✅ **Ready** | Native | Robust stubs for orchestrator validation. |
| **Adaptive Routing** | 🧪 **Prototype** | Heuristic | Static fallback chain; not yet benchmark-driven. |
| **Governance** | ✅ **Ready** | Manual | MIT Licensed; `CONTRIBUTING.md` authored. |

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
5. **Setup Reproducibility**: Verify `make setup build` works on a clean clone.

### 🛡 Trust & Readability Gaps (Should Fix)
1. **Agent Versioning**: `doctor` command detects binary existence but not version compatibility (e.g. `codex-agent` v0.5 vs v1.0).
2. **Internal Documentation Noise**: ✅ RESOLVED (Upgraded headers and README navigation).
3. **Example Parity**: Ensure `examples/tasks/*.yaml` are 100% compliant with the latest schema.

### 🛠 Ready for Release Finalization
1. **Governance**: ✅ RESOLVED (MIT & CONTRIBUTING.md).
2. **Health**: Enhance `doctor` with version checks for `git` and `sqlite3`.

---
- [x] Final alignment for Phase V1.C1 (Alignment Complete)
- [x] Tighten `submit` -> `wait` -> `result` sequence (V1.D2/C2 Complete)
- [x] Harden post-execution inspection (V1.D3/C2 Complete)
- [x] Clarify non-success terminal outcomes (V1.D4/C2 Complete)
- [x] Final alignment for Local Operator Flow (V1.C2 Complete)
