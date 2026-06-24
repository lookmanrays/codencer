# Public Self-host Hardening Final Report

Date: 2026-06-24

Implementation commit hash: `5ebc8985b7112bb770c8d90c53c8f5e667eaea90`

Branch: `next-phase`

## Implementation Summary

- Added the public self-host release spec files under `docs/specs/` and the acceptance gate at `docs/acceptance/public-selfhost-release-gate.yaml`.
- Created the pre-change implementation audit at `reports/public-selfhost-hardening/implementation-audit.md`.
- Hardened the public self-host RC verifier so it emits only `GO` or `NO-GO`, rejects real-executor simulation env values, runs configured real executor gates by adapter, and fails the release gate when required real proofs are missing.
- Confirmed the real Codex path invokes the configured Codex binary with `ALL_ADAPTERS_SIMULATION_MODE=0` and `CODEX_SIMULATION_MODE=0`.
- Added `codencer run events`, `codencer run report`, `codencer run cancel`, and structured `codencer run resume` blocker behavior.
- Added Gateway MCP async lifecycle tools: `codencer.start_project_run`, `codencer.submit_project_task`, `codencer.list_project_runs`, `codencer.get_project_run`, `codencer.get_project_run_status`, `codencer.get_gateway_run_events`, true project-scoped `codencer.cancel_project_run`, and a structured `codencer.resume_project_run` capability blocker.
- Preserved `codencer.submit_project_task_and_wait` as a compatibility tool while adding non-blocking submit/start paths for planners that should not hold one long HTTP/MCP request open.
- Updated Gateway Console simple-task submit to send `wait=false`, poll the run report until terminal status, display `pending` while waiting, and emit the terminal audit event once when report refresh observes completion.
- Wired project run cancellation through Gateway HTTP, Gateway MCP, Relay HTTP, Relay MCP, Connector project proxy, and local daemon-backed cancellation, with Gateway run history/audit events for `cancel_project_run_requested` and terminal `run_cancelled`.
- Added `codencer sync status`, `codencer sync preview`, and `codencer sync publish` as explicit metadata-only sync controls. Raw artifacts/logs are blocked, and confirmed publish ingests only sanitized metadata into Gateway run history with `scope=synced`.
- Redacted local absolute repo/report paths, daemon URLs, token-like text, and unsafe executor summaries from default human CLI project/status/submit/run output while preserving explicit `--json` operator detail.
- Added Gateway run-history `scope` metadata and exposed it through the API and Console run list/detail views.
- Added Gateway-observed run/audit `limit`/`offset` pagination, server-side filters, grouped lifecycle summaries, and Console previous/next controls for Runs and Audit.
- Added first-class local `human_interrupts` records and `human_interrupt_created` Gateway audit events for blocker/question/approval/permission/system-action outcomes.
- Added Antigravity executor profiles so executor discovery exposes Antigravity as a real profile family.
- Added isolated Antigravity proof plumbing: `CODENCER_ANTIGRAVITY_DAEMON_DIR` discovery override, preservation of explicit verifier workspace roots, and live-verifier support for `CODENCER_E2E_ANTIGRAVITY_INSTANCE_JSON`, `CODENCER_E2E_ANTIGRAVITY_INSTANCE_FILE`, and `CODENCER_E2E_ANTIGRAVITY_DAEMON_DIR`.
- Hardened Antigravity proof handling so the verifier rejects an Antigravity language-server instance unless its actual `GetWorkspaceInfos` output includes the isolated verifier repo.
- Hardened the direct Antigravity adapter so unsupported or out-of-workspace permission waits become manual-attention results instead of timing out, without exposing the requested local target path or command string.
- Fixed connector shared-instance discovery so an allowlisted manifest identity is not overwritten by an unrelated daemon that happens to be listening on the manifest URL during tests.
- Tightened the live Console verifier so the real executor result check targets the Summary heading deterministically.

## Proofs

- Current scoped Codex artifact-backed RC proof passed:
  - Report: `reports/public-selfhost-rc/20260624T122313Z/summary.md`
  - Gates: `real_executor_e2e_codex`, `required_real_executor_proofs`
  - Verdict: overall `NO-GO` for the default public gate because Claude Code and Antigravity were missing in that run; the Codex subgate itself passed.
  - Evidence: live verifier log shows `Adapter Execution: Starting process`, `adapter=codex`, and binary `/Applications/Codex.app/Contents/Resources/codex`; simulation env was forced to `ALL_ADAPTERS_SIMULATION_MODE=0` and `CODEX_SIMULATION_MODE=0`.
- Latest Codex artifact-backed RC rerun is `NO-GO` due to an external Codex usage-limit error, not simulation:
  - Report: `reports/public-selfhost-rc/20260624T125824Z/summary.md`
  - Gate: `real_executor_e2e_codex`
  - Evidence: live verifier printed `ALL_ADAPTERS_SIMULATION_MODE=0 CODEX_SIMULATION_MODE=0`, daemon log showed `Adapter Execution: Starting process` with binary `/Applications/Codex.app/Contents/Resources/codex`, and report payload had `is_simulation=false`.
  - Blocker: Codex CLI returned `You've hit your usage limit`; the verifier correctly returned `NO-GO`.
- Default all-real-executor RC proof remains `NO-GO`:
  - Report: `reports/public-selfhost-rc/20260624T115347Z/summary.md`
  - Gates passed through `real_executor_e2e_codex`, then `required_real_executor_proofs` failed because Claude Code and Antigravity were missing from that run.
- Earlier Codex + Claude Code artifact-backed RC subgates passed:
  - Report: `reports/public-selfhost-rc/20260624T105654Z/summary.md`
  - Gates: `real_executor_e2e_codex`, `real_executor_e2e_claude`
  - Required proof log: `reports/public-selfhost-rc/20260624T105654Z/required_real_executor_proofs.log`
  - Evidence: Codex daemon log shows `Adapter Execution: Starting process`, `adapter=codex`, and the configured Codex binary; Claude verifier log shows `primary=claude` with simulation preflight disabled.
  - Overall verdict remains `NO-GO` because Antigravity is missing.
- Antigravity source-tree live proof was attempted with isolated temp instance metadata generated from the local running Antigravity language server.
  - The verifier bound the candidate instance by PID without writing user Codencer state.
  - Current verifier fails early when the candidate Antigravity LS does not expose the isolated verifier repo in `GetWorkspaceInfos`; latest local attempt exited with `real Antigravity gate requires an Antigravity workspace for the isolated verifier repo; workspace_count=0`.
  - Earlier reachable local Antigravity LS candidates failed to complete through the Antigravity adapter and fell through to `ide-chat`; the verifier failed instead of accepting fallback behavior.
  - No Antigravity real proof is available.
- Gateway Console visual evidence regenerated:
  - `reports/gateway-console-screenshots/2026-06-24-1202`
  - `reports/gateway-console-screenshots/2026-06-24-1213`
  - `reports/gateway-console-screenshots/2026-06-24-1238`
  - `reports/gateway-console-screenshots/2026-06-24-1243`
  - `reports/gateway-console-screenshots/2026-06-24-1247`
  - `reports/gateway-console-screenshots/2026-06-24-1259`
  - `reports/gateway-console-screenshots/2026-06-24-1323`
  - `reports/gateway-console-screenshots/2026-06-24-1403`
  - `reports/gateway-console-screenshots/2026-06-24-1553`
  - `reports/gateway-console-screenshots/2026-06-24-1556`
  - `reports/gateway-console-screenshots/2026-06-24-1616`
  - `reports/gateway-console-screenshots/2026-06-24-1626`
  - `reports/gateway-console-screenshots/2026-06-24-1631`

## Commands Run

- `bash -n scripts/verify_public_selfhost_rc.sh` - passed
- `go test ./cmd/codencer` - passed
- `go test ./cmd/codencer ./internal/security` - passed
- `go test ./internal/localexec` - passed
- `go test ./internal/profile ./internal/adapters/codex` - passed
- `go test ./internal/connector -count=1` - passed
- `go test ./internal/gateway` - passed
- `go test ./internal/gateway` after adding Gateway MCP async lifecycle tools - passed
- `go test ./internal/localexec` after adding async report refresh - passed
- `go test ./internal/localexec ./internal/gateway ./cmd/codencer` - passed
- `go test ./...` - passed
- `go test ./...` after adding Gateway MCP async lifecycle tools - passed
- `go test ./...` after adding Gateway Console async submit/report polling - passed
- `go test ./internal/connector ./internal/gateway ./internal/relay ./internal/localexec` after adding project-scoped cancel routing - passed
- `go test ./...` after adding project-scoped cancel routing - passed
- `cd web/gateway-console && npm run format:check` - passed
- `cd web/gateway-console && npm run lint` - passed
- `cd web/gateway-console && npm run typecheck` - passed
- `cd web/gateway-console && npm run test` - passed
- `cd web/gateway-console && npm run test -- --run tests/schemas.test.ts` - passed
- `cd web/gateway-console && npm run format:check && npm run lint && npm run typecheck && npm run test && npm run test:e2e -- --grep "project task form submits demo run"` - passed
- `cd web/gateway-console && npm run format:check && npm run lint && npm run typecheck && npm run test && npm run test:e2e` - passed
- `cd web/gateway-console && npm run build` - passed
- `cd web/gateway-console && npm run test:e2e` - passed
- `make verify-gateway` - passed
- `make verify-gateway` after adding Gateway MCP async lifecycle tools - passed
- `make verify-gateway` after adding Gateway Console async submit/report polling - passed
- `make verify-gateway` after adding project-scoped cancel routing - passed
- `make verify-gateway-console` - passed
- `make verify-gateway-console` after stabilizing run-detail navigation e2e and regenerating evidence - passed
- `make verify-gateway-console-live` - passed
- `make verify-gateway-console-live` after adding terminal audit-on-report refresh - passed
- `go test ./internal/adapters/antigravity` - passed
- `cd web/gateway-console && npm run format:check && npm run lint -- tests/live/verify-live.mjs` - passed
- `CODENCER_E2E_REAL_EXECUTOR=codex CODENCER_E2E_REAL_EXECUTOR_COMMAND=/Applications/Codex.app/Contents/Resources/codex make verify-public-selfhost-rc` - failed by design with `NO-GO` after Codex passed and remaining required proofs were missing; report `reports/public-selfhost-rc/20260624T122313Z/summary.md`
- `CODENCER_E2E_REAL_EXECUTOR=codex CODENCER_E2E_REAL_EXECUTOR_COMMAND=/Applications/Codex.app/Contents/Resources/codex make verify-public-selfhost-rc` - failed with `NO-GO` due to external Codex usage limit after invoking the real Codex binary with simulation disabled; report `reports/public-selfhost-rc/20260624T125824Z/summary.md`
- `CODENCER_E2E_REQUIRED_REAL_EXECUTORS=codex CODENCER_E2E_REAL_EXECUTOR=codex CODENCER_E2E_REAL_EXECUTOR_COMMAND=<configured-codex-binary> make verify-public-selfhost-rc` - passed with scoped `GO` for Codex-only proof
- `make verify-public-release` - passed
- `make verify-public-selfhost-release TARGETS=host REQUIRE_TARGETS=host` - passed after project-scoped cancel routing and console e2e stabilization
- `CODENCER_E2E_REAL_EXECUTORS=codex,claude CODENCER_E2E_CODEX_COMMAND=<codex-binary> CODENCER_E2E_CLAUDE_COMMAND=<claude-binary> make verify-public-selfhost-rc` - failed by design with `NO-GO` after Codex and Claude passed and Antigravity was missing
- `cd web/gateway-console && CODENCER_E2E_BIN_DIR=../../bin CODENCER_E2E_EXECUTOR_ADAPTER=antigravity CODENCER_E2E_EXECUTOR_PROFILE=antigravity-default CODENCER_E2E_ANTIGRAVITY_INSTANCE_FILE=<temp-file> node tests/live/verify-live.mjs` - failed correctly; the provided Antigravity LS did not expose the isolated verifier repo workspace
- `git diff --check` - passed

## Remaining Blockers

- Antigravity real executor proof is not proven in the public self-host RC gate.
- Latest Codex real executor rerun is blocked by Codex account usage limits despite invoking the real binary with `is_simulation=false`; prior Codex proof remains the latest passing Codex proof.
- Current local Antigravity app processes expose reachable RPC endpoints, but the available candidates do not expose the isolated verifier repo workspace through `GetWorkspaceInfos`, so the verifier refuses to bind them for public release proof.
- `codencer run resume` and Gateway MCP `codencer.resume_project_run` are exposed as structured blockers because the daemon/Relay path does not yet expose a true resume route.
- Project-scoped cancel now routes through Gateway, Relay, Connector, and local daemon cancellation; whether the underlying executor stops immediately remains bounded by daemon/executor cancellation semantics.
- Raw log/artifact upload remains unsupported by design. `codencer sync publish --confirm` ingests metadata-only run/project summaries into Gateway history; it does not upload local reports, logs, artifacts, daemon URLs, or filesystem paths.
- Run history/audit synced-scope transport now exists for explicit metadata-only `codencer sync publish`; broader incremental sync policy and external source reconciliation remain incomplete.
- Human interrupt lifecycle is still partial: local report/event records and Gateway blocker audit exist, but complete operator answer/resume UI/MCP flows are not fully proven.
- Full cross-surface redaction proof remains incomplete. Default local human CLI output and sync preview are covered, but explicit JSON/debug/path commands still require final policy review against the release gate.

Verdict: NO-GO
