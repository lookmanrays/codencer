# Codencer Gap Audit & Readiness
> [!WARNING]
> **INTERNAL DEVELOPER DOCUMENT**: This file is for project maintainers and contains technical debt audits, task backlogs, and roadmap tracking.
> For the official **User Guide**, please refer to the [README.md](../../README.md).

## Current Reality
- **Lifecycle Meaning Cleanup**: [RESOLVED] Explicitly defined Run (Session), Step (Planner Unit), and Attempt (Execution Try) in domain code and README. Verified that no bridge-side decision logic is implied.
- **Terminology Inconsistency**: [RESOLVED] Renamed all outcome indicators to `State` (RunState, StepState, Result.State) for uniform operator experience.
- **Ergonomics**: [RESOLVED] Tightened the `submit` -> `wait` -> `result` sequence and established the **Canonical Local Runbook** in `EXAMPLES.md`.
- **Trust & Transparency**: [RESOLVED] Added "Known Limitations" and clarified the distinction between simulation and real-mode execution in README.
- **Release Surface**: [RESOLVED] Performed a unified v1 Truth-Pass across all README, setup, examples, and guide documentation.
- **OpenClaw Status**: [ALIGNED] Maintained as Experimental Alpha-tier for v1.0. Future promotion to stable requires sustained user verification.

## Feature Status Matrix

| Component | Status | Implementation Type | Notes |
| :--- | :--- | :--- | :--- |
| **Orchestration Core** | ✅ **Ready (Stable)** | Native (SQLite) | Persistent ledger, state machine, and Git Worktrees. |
| **CLI & MCP Layer** | ✅ **Ready (Stable)** | Native | Human-readable hints, logs, and structured JSON. |
| **Codex Adapter** | ✅ **Ready (Stable)** | CLI Wrapper | High-fidelity relay with artifact harvesting. |
| **OpenClaw Adapter** | 🧪 **Experimental (Alpha)** | ACPX Wrapper | Functional alpha; basic lifecycle tracking. |
| **Claude Adapter** | 🟢 **Supported (Beta)** | CLI Wrapper | Uses `claude -p --output-format json` with stdin prompt delivery, cwd-based execution, synthesized result mapping, and fake-binary integration coverage. Live authenticated Claude service calls are not exercised in repo tests. |
| **Qwen Adapter** | 🟡 **Functional** | CLI Wrapper | Basic subprocess wrapper; narrower evidence extraction than Codex/Claude. |
| **Simulation Mode** | ✅ **Ready (Stable)** | Native | Robust stubs for orchestrator validation. |
| **Adaptive Routing** | 🧪 **Prototype** | Heuristic | Static fallback chain; not yet benchmark-driven. |
| **Governance** | ✅ **Ready (Stable)** | Manual | MIT Licensed; `CONTRIBUTING.md` authored. |
| **Diagnostics** | ✅ **Ready (Stable)** | CLI | `doctor` command verifies versions and environment. |

## Known Technical Debt & Limitations
- **Adaptive Routing**: Routing is currently based on a static heuristic chain; benchmark-driven optimization is documented but not dynamic.
- **Process Introspection**: CLI-wrapped adapters provide limited visibility beyond standard streams.
- **Simulation Limits**: Simulation Mode stubs all actions; it validates the orchestrator's state-machine but does not test real agent logic.

## V1 Publication Audit (Phase V1.F3)

### 🚨 Critical Publication Blockers (Must Fix)
1. **LICENSE**: ✅ RESOLVED (MIT).
2. **CONTRIBUTING.md**: ✅ RESOLVED.
3. **Repository Noise**: ✅ RESOLVED (`codencer.db` removed/ignored).
4. **Makefile Version**: ✅ RESOLVED (`v1.0-release-candidate`).
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
1. **`doctor` Versioning**: ✅ RESOLVED (Version checks for `git`, `sqlite3`, `go`, and `codex-agent` implemented).
2. **`doctor` Environment**: ✅ RESOLVED (Checks for `.env` and directory structure implemented).
3. **Smoke Test Brittleness**: ✅ RESOLVED (Modernized `smoke_test.sh` with JSON parsing and `submit --wait`).
4. **Port Conflict**: ✅ RESOLVED (Standardized on 8080 in `.env` and `smoke_test.sh`).

### 🛠 Deployment/CI Hardening (Next Phase)
1. **Setup Logic**: Improve `make setup` to safely handle `.env` initialization (Current: Manual `cp`).
2. **Binary Distribution**: Move beyond `go build` to pre-compiled releases.

---
- [x] Audit Trust & Readiness Alignment (Final Alignment Complete)
- [x] V1 Publication Readiness Audit (Batch V1.F5 Complete)
- [x] Harden `doctor` with binary version checking (Batch V1.R1 Complete)
- [x] Align Smoke Test with modern CLI ergonomics (Batch V1.R1 Complete)
- [x] Batch R2: Final Metadata & Release Notes (Complete)
    - [x] Update version strings to `v1.0-release-candidate`
    - [x] Create `CHANGELOG.md`
    - [x] Final Sanity Audit
