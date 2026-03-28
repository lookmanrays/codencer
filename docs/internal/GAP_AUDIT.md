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
| **Diagnostics** | ✅ **Ready (Beta)** | CLI | `doctor` command verifies versions and environment. |

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

## V1 Publication Readiness Audit (Batch V1.F5)

### 🕹 Current First-Run Path
1. `make setup build`: Creates directories and compiles binaries.
2. `cp .env.example .env`: Manual configuration.
3. `make start-sim`: Daemon starts in background.
4. `run start` -> `submit --wait`: Primary operator loop.
5. `step result/logs/artifacts`: Evidence inspection.

### 🏥 Diagnostics Capabilities
- **`doctor`**: Verifies `.codencer` directory, daemon connectivity, and basic binary existence (PATH).
- **`smoke`**: Exercises the basic relay loop in simulation mode.

### 🚨 Release Blockers (Must Fix)
1. **`doctor` Versioning**: No version checks for critical binaries (`git`, `sqlite3`, `go`, `codex-agent`).
2. **`doctor` Environment**: No check for `.env` presence.
3. **Smoke Test Brittleness**: `scripts/smoke_test.sh` uses brittle grep/cut and antiquated manual `step wait` logic.
4. **Port Conflict**: `SETUP.md` (8080) vs `smoke_test.sh` (8085) vs standard `.env` (8080).

### 🛠 What should be fixed next
1. **Harden `doctor`**: Implement version checks for all critical deps.
2. **Standardize `smoke`**: Modernize `smoke_test.sh` to use `submit --wait` and robust JSON parsing (e.g. `jq` if available or better grep).
3. **Setup Logic**: Improve `make setup` to safely handle `.env` initialization.

---
- [x] Audit Trust & Readiness Alignment (Final Alignment Complete)
- [x] V1 Publication Readiness Audit (Batch V1.F5 Complete)
- [x] Harden `doctor` with binary version checking (Batch V1.F5 Complete)
- [x] Align Smoke Test with modern CLI ergonomics (Batch V1.F6 Complete)
- [ ] Implement "Latest" ID Alias for CLI (Batch V1.L3 Roadmap)
- [ ] Setup Logic Automation (Batch V1.F6 Next)
