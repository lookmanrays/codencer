# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Self-hostable v2 relay path with:
  - stable daemon instance identity and manifest-backed discovery
  - outbound authenticated connector sessions with explicit shared-instance allowlists
  - self-host relay planner API, enrollment token flow, audit persistence, and relay-side MCP tools
- **OpenClaw (acpx) Adapter**: 🧪 Experimental support for OpenClaw-compatible executors via the standardized ACP bridge.
- Official sequential wrapper examples for bash/zsh, PowerShell, and Python under `examples/automation/`.
- Wrapper-friendly sample task lists and prompt/task inputs for ordered execution.
- New `scripts/smoke_test_v1.sh` for verifying all 6 primary submission modes.

### Changed
- Rewrote operator-facing v2 docs to match the implemented local/self-host path and current runtime truth.
- Clarified that the relay is the public remote HTTP/MCP surface and the daemon-local `/mcp/call` endpoint is only a local compatibility/admin surface.
- Documented current self-host alpha limitations explicitly: best-effort abort, opportunistic resource routing, bounded artifact transport, and static-token auth.
- **Unified v1 Documentation Truth-Pass**: Cleaned and synchronized all public-facing docs (README, AI Guide, Runbook, Automation) for 100% alignment with the CLI contract.
- Expanded automation documentation to make the shell-planner story explicit and machine-oriented.
- Clarified that ordered task execution in v1 is wrapper-based and not a native workflow engine.
- Hardened smoke/example guidance around strict JSON parsing and machine-safe CLI usage.

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
