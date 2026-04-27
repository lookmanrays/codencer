# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0-beta] - 2026-04-23

### Added
- Self-hostable v2 relay path with:
  - stable daemon instance identity and manifest-backed discovery
  - outbound authenticated connector sessions with explicit shared-instance allowlists
  - self-host relay planner API, enrollment token flow, audit persistence, and relay-side MCP tools
- Cloud control-plane self-host surface with `codencer-cloudd`, `codencer-cloudctl`, and `codencer-cloudworkerd`, plus bootstrap/status/org/workspace/project/token/install/event/audit flows.
- Cloud installation enable/disable routes and matching `cloudctl install enable|disable` subcommands.
- Truthful cloud docs and smoke guidance for the bootstrap and control-plane path.
- **OpenClaw (acpx) Adapter**: 🧪 Experimental support for OpenClaw-compatible executors via the standardized ACP bridge.
- Official sequential wrapper examples for bash/zsh, PowerShell, and Python under `examples/automation/`.
- Wrapper-friendly sample task lists and prompt/task inputs for ordered execution.
- New `scripts/smoke_test_v1.sh` for verifying all 6 primary submission modes.
- Public beta tester guide in `docs/BETA_TESTING.md` with exact local, relay, cloud, planner/client, and provider test-track entrypoints.
- `make build-supported`, `make verify-beta`, and `make verify-beta-docker` as explicit repo-level verification entrypoints for the supported tracks.
- Planner-client walkthroughs for ChatGPT, Claude Desktop plus `claude.ai`, and Gemini CLI under `docs/mcp/integrations/`.
- Per-platform setup walkthroughs for macOS, Windows plus `agent-broker`, WSL, and remote VPS dev-server layouts.
- Consolidated operator boundary reference in `docs/KNOWN_LIMITATIONS.md`.
- Operator-facing beta launch notes in `docs/RELEASE_NOTES_v0.2.0-beta.md`.
- Cross-platform public-testability CI on `macos-latest` and `windows-latest`.

### Changed
- Adopted `v0.2.0-beta` as the truthful build/version string for the current v2 local/self-host beta repo state.
- Rewrote operator-facing v2 docs to match the implemented local/self-host path and current runtime truth.
- Clarified that the relay is the public remote HTTP/MCP surface and the daemon-local `/mcp/call` endpoint is only a local compatibility/admin surface.
- Documented current self-host beta boundaries explicitly: best-effort abort, bounded artifact transport, static-token auth, and relay routing that now probes only authorized online shared instances before failing closed.
- Removed duplicate public connector/relay binary surfaces in favor of the canonical `codencer-connectord` and `codencer-relayd` entrypoints.
- Tightened abort reporting so Codencer only reports success when the active step really reaches `cancelled`.
- Removed committed extension dependency/build output directories and kept only the extension manifests plus source.
- Added relay admin/status routes, connector local status snapshots, and a practical self-host smoke flow for daily operator use.
- **Unified Documentation Truth-Pass**: Cleaned and synchronized current public-facing docs (README, AI Guide, Runbook, Automation) for alignment with the implemented CLI and relay surfaces.
- Expanded automation documentation to make the shell-planner story explicit and machine-oriented.
- Clarified that ordered task execution in v1 is wrapper-based and not a native workflow engine.
- Hardened smoke/example guidance around strict JSON parsing and machine-safe CLI usage.
- Clarified the public test-track boundaries so local, relay/runtime, cloud, planner/client, and provider testing route to the right docs without mixing surfaces.
- Parameterized the Docker cloud image build version through the compose environment instead of hardcoding it only inside the Dockerfile.

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
[0.2.0-beta]: https://github.com/lookmanrays/codencer/releases/tag/v0.2.0-beta
