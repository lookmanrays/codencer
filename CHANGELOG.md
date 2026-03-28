# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0-beta] - 2026-03-28

### Added
- **Orchestration Core**: Persistent SQLite ledger and robust state machine for run-to-run consistency.
- **CLI (orchestratorctl)**: Human-friendly command suite with `submit --wait`, `run`, and `step` management.
- **Relay Model**: Explicit "Bridge not Brain" architecture ensuring the orchestrator acts as a high-fidelity audit trail.
- **Diagnostics (doctor)**: Environment verification tool for Git, SQLite, Go, and adapter binary version checking.
- **Workspace Isolation**: Support for Git Worktrees to ensure agents work in clean, isolated clones.
- **Validation Engine**: Support for specifying and executing local validation commands (tests, linters) post-execution.
- **Simulation Mode**: Robust simulation adapter for testing orchestration logic without requiring LLMs.
- **Codex Adapter**: Dedicated high-fidelity relay for the `codex-agent` binary.
- **Artifact Harvesting**: Automated capture of diffs, logs, and modified files into a permanent audit trail.

### Changed
- **Unified Terminology**: Standardized on `Run` (Session), `Step` (Planner Unit), and `Attempt` (Execution Unit) across all docs and code.
- **CLI Ergonomics**: Optimized the canonical operator flow: `run start` -> `submit --wait` -> `step result`.
- **Maturity labels**: Updated all components to reflect an honest **MVP / Public Beta** status.

### Removed
- Redundant `Result.Status` (superseded by `Result.State` for uniformity).
- Inconsistent terminology regarding "Mission" vs "Run".

### Fixed
- README markdown rendering issues.
- Conflicting port defaults across documentation and setup scripts.
- Permission-check gaps in local storage diagnostics.

---

[0.1.0-beta]: https://github.com/lookmanrays/codencer/releases/tag/v0.1.0-beta
