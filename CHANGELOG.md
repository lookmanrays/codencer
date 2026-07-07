# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.3.1](https://github.com/lookmanrays/codencer/compare/v0.3.0...v0.3.1) (2026-07-07)


### Bug Fixes

* validate submit run ids and publish release assets ([#5](https://github.com/lookmanrays/codencer/issues/5)) ([5a6389a](https://github.com/lookmanrays/codencer/commit/5a6389ae3d4bd38c8d69b3a2b2604d776ed11ad3))

## [0.3.0](https://github.com/lookmanrays/codencer/compare/v0.2.0...v0.3.0) (2026-07-06)


### Features

* **codex:** harden adapter execution and result harvesting (Batch V1.3.1) ([142a4ca](https://github.com/lookmanrays/codencer/commit/142a4caaf3f67af43ff9e085536c91941c17d928))
* **codex:** harden result harvesting and artifact discovery (Batch V1.3.2) ([7240ae7](https://github.com/lookmanrays/codencer/commit/7240ae780a0d04ca643e9810cc82e56db8b20162))
* **codex:** harden validation scenario and evidence harvesting (V1.3.3) ([b7ab7c6](https://github.com/lookmanrays/codencer/commit/b7ab7c662cf1367fad4c4e0aa87df5fa8a52fa81))
* complete initial Antigravity integration (discovery, binding, and execution bridge) ([896efdc](https://github.com/lookmanrays/codencer/commit/896efdc7d46c9646f0fff08d68ea1595493f1926))
* complete local orchestration bridge implementation ([e19cce3](https://github.com/lookmanrays/codencer/commit/e19cce32dbd32947d7135e75c5b63d7e8e939ada))
* complete phase 2 production hardening (orchestration, retrieval, policy, recovery, UI) ([1fe8a3e](https://github.com/lookmanrays/codencer/commit/1fe8a3e3f40a31669100cc688602461fe16079ac))
* complete Phase 3 Multi-Agent Expansion & DSL Hardening ([7cd197a](https://github.com/lookmanrays/codencer/commit/7cd197a3237989dba27b740707c7ec81c5218cfc))
* complete ruthless gap audit resolving stubs to functional local adapters and extensions ([7f367c2](https://github.com/lookmanrays/codencer/commit/7f367c29e14aa1c891249f52ee7f75833e4db7e5))
* enhance Antigravity integration (cross-side WSL-Windows discovery, failure preservation, and routing refinement) ([97a07fe](https://github.com/lookmanrays/codencer/commit/97a07fe6802ecf27919d0e93ff27554d13b12501))
* finalize Phase 4 - Benchmarking & IDE Chat Adapters ([2bb061d](https://github.com/lookmanrays/codencer/commit/2bb061d9bf965b4d7161ab4858c4c31ff24d5037))
* harden recovery, resumability, and workspace reconciliation心志 ([8bb3776](https://github.com/lookmanrays/codencer/commit/8bb37766d704078e1152c8cf124f1b28476fdba1))
* harden relay contracts and synchronize persistence layer ([c0fdaee](https://github.com/lookmanrays/codencer/commit/c0fdaee539b21623fc7f3537fd52051fb48ac54e))
* harden result, artifact, and validation retrieval across all layers ([5f423ba](https://github.com/lookmanrays/codencer/commit/5f423ba4c7285812b8d77932e77c0170268fa3a9))
* implement Antigravity Broker (Phase 1-7) for robust WSL/Windows orchestration ([2225881](https://github.com/lookmanrays/codencer/commit/22258813fdff9c9e2afd76ec0ea8b2072363945b))
* implement Codencer v2 local self-host path ([#2](https://github.com/lookmanrays/codencer/issues/2)) ([b17d511](https://github.com/lookmanrays/codencer/commit/b17d5119e26cb860db8bcad6b4e87086b2a8b3ad))
* implement configuration-driven policies, hardened routing, and API integration tests ([0fab94d](https://github.com/lookmanrays/codencer/commit/0fab94dc1bef9b98b3cb8ba84de0bb93e720e219))
* implement orchestrator MVP (phases 1-13) ([8ab291f](https://github.com/lookmanrays/codencer/commit/8ab291f8095cef8b88f03b197d031c338d4b8025))
* implement repo-scoped binding for Antigravity Broker and fix executor docs ([2acf4dd](https://github.com/lookmanrays/codencer/commit/2acf4ddd2457201b1ee28478e5836fb85f98e7f8))
* **ops:** stabilize broker identity, provisioning persistence, and docs ([dbb84ab](https://github.com/lookmanrays/codencer/commit/dbb84ab01b2c9611fbb50c47cd24ec532374d1fc))
* **orchestration:** align local development paths and harden relay loop verification ([25945ee](https://github.com/lookmanrays/codencer/commit/25945eeb40f7a1566993905f0abcf3bf80165510))
* **orchestration:** align planner-facing CLI surface (Batch V1.2.1) ([685306e](https://github.com/lookmanrays/codencer/commit/685306eae49bb313d5d259d8c802274ac2b167bc))
* **orchestration:** align planner-facing wait and result flows (Batch V1.2.2) ([9c319fd](https://github.com/lookmanrays/codencer/commit/9c319fd8f021270d4d097e24086c1afd4e69a6a4))
* **orchestration:** harden execute-and-wait loop and implement state discovery ([beb5c3a](https://github.com/lookmanrays/codencer/commit/beb5c3a98fe1374300b5d817822cb60d26151992))
* **orchestration:** harden execution state model and relay semantics (Batch V1.1.2) ([3af16ac](https://github.com/lookmanrays/codencer/commit/3af16acd83cae45af4a88c4938c4fe78ac11c197))
* **orchestration:** harden local operator flow and promote to v0.1.0 ([e927f50](https://github.com/lookmanrays/codencer/commit/e927f504715d41d47685e1f4d986b1d9e7d48476))
* **orchestration:** harden operational maturity (Phase V1.F2) ([598422e](https://github.com/lookmanrays/codencer/commit/598422e29d5731e1f23eae91973765e99c205099))
* **orchestration:** harden packaging and governance (Phase V1.F3) ([31feed2](https://github.com/lookmanrays/codencer/commit/31feed2d9b1b3444ae45eeb1115a91916c0f80c4))
* **orchestration:** harden terminal result semantics and relay model alignment (Phase V1.4) ([9196773](https://github.com/lookmanrays/codencer/commit/9196773b3a22960e01d5bea53412312bd92a8671))
* **orchestration:** harden usability, logs, and troubleshooting (Phase V1.7) ([5cb6596](https://github.com/lookmanrays/codencer/commit/5cb659618cfedd1074d2b2b73ae5a4d23f4ffe0e))
* **orchestration:** harden V1 operational consistency and alignment ([2113ef9](https://github.com/lookmanrays/codencer/commit/2113ef9893871b91c87cf23703731692f6ee9c10))
* **provisioning:** implement workspace preparation layer with Grove compatibility and security hardening ([f737b32](https://github.com/lookmanrays/codencer/commit/f737b32aee70f22460b85155db67068b3b993333))
* release Codencer public self-host v0.3.0 ([#3](https://github.com/lookmanrays/codencer/issues/3)) ([780106f](https://github.com/lookmanrays/codencer/commit/780106fa27034a1c34065c9dd09ae8bc8ce4c0c8))
* **setup:** harden self-host onboarding, unified quickstart, and configuration logic (Phase V1.F1) ([3c250b3](https://github.com/lookmanrays/codencer/commit/3c250b3f074e6cb662350ac387bd116bb505c670))
* standardize and harden adapter layer (Codex, Claude, Qwen) ([6ecf32a](https://github.com/lookmanrays/codencer/commit/6ecf32ad3afaa753615834bbe9d56cb5fdfea465))
* upgrade VS Code extension to functional operator control surface ([2bd4b7c](https://github.com/lookmanrays/codencer/commit/2bd4b7cf8fe05662a9563b3312ff8049e400a50d))
* **usability:** align default paths and improve local startup flow (Phase V1.6) ([8f7722b](https://github.com/lookmanrays/codencer/commit/8f7722be3dd823eab5f8dc2bb147673030cdd23c))
* **usability:** clarify execution modes and config expectations (Phase V1.6) ([629e534](https://github.com/lookmanrays/codencer/commit/629e534882f618efd5a13dd57d55b8c08c58a109))
* **usability:** improve quickstart and verification flow (Phase V1.6) ([9f8bf3a](https://github.com/lookmanrays/codencer/commit/9f8bf3a4256ea57fda10234522e0fbb465d74afe))


### Bug Fixes

* **antigravity:** refine failure mapping, documentation, and discovery overrides ([137cc52](https://github.com/lookmanrays/codencer/commit/137cc521878411de613b3f92d85846cc37ada9c2))
* Batch 1 final portability and url logic ([dbbc349](https://github.com/lookmanrays/codencer/commit/dbbc349217849bacced092f9870aca3fe9133df7))
* **broker:** use stable repo root for identity and refine provisioning docs ([ed59af1](https://github.com/lookmanrays/codencer/commit/ed59af16c9edf9b4f794b21df9ebfcba0782240e))
* **claude:** align Codencer adapter with real Claude Code CLI ([598bc78](https://github.com/lookmanrays/codencer/commit/598bc78e54001a6a10663c22ba5feee29cf0602c))
* Finalize Batch 1 artifacts and execution documentation ([633fc86](https://github.com/lookmanrays/codencer/commit/633fc862bb1ee88fd4aa2a8a2b54816f641418a9))

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
