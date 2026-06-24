# Public Self-host Hardening Final Report

Date: 2026-06-24

Implementation commit hash: `ece6e2914b6c6dcf7dc83bab821beb77815dca94`

Branch: `next-phase`

## Implementation Summary

- Added the public self-host release spec files under `docs/specs/` and the acceptance gate at `docs/acceptance/public-selfhost-release-gate.yaml`.
- Created the pre-change implementation audit at `reports/public-selfhost-hardening/implementation-audit.md`.
- Hardened the public self-host RC verifier so it emits only `GO` or `NO-GO`, rejects real-executor simulation env values, runs configured real executor gates by adapter, and fails the release gate when required real proofs are missing.
- Confirmed the real Codex path invokes the configured Codex binary with `ALL_ADAPTERS_SIMULATION_MODE=0` and `CODEX_SIMULATION_MODE=0`.
- Added `codencer run events`, `codencer run report`, `codencer run cancel`, and structured `codencer run resume` blocker behavior.
- Added `codencer sync status`, `codencer sync preview`, and `codencer sync publish` as explicit metadata-only sync controls. Raw artifacts/logs are blocked, and confirmed publish ingests only sanitized metadata into Gateway run history with `scope=synced`.
- Redacted local absolute repo/report paths, daemon URLs, token-like text, and unsafe executor summaries from default human CLI project/status/submit/run output while preserving explicit `--json` operator detail.
- Added Gateway run-history `scope` metadata and exposed it through the API and Console run list/detail views.
- Added Gateway-observed run/audit `limit`/`offset` pagination, server-side filters, grouped lifecycle summaries, and Console previous/next controls for Runs and Audit.
- Added first-class local `human_interrupts` records and `human_interrupt_created` Gateway audit events for blocker/question/approval/permission/system-action outcomes.
- Added Antigravity executor profiles so executor discovery exposes Antigravity as a real profile family.
- Added isolated Antigravity proof plumbing: `CODENCER_ANTIGRAVITY_DAEMON_DIR` discovery override, preservation of explicit verifier workspace roots, and live-verifier support for `CODENCER_E2E_ANTIGRAVITY_INSTANCE_JSON`, `CODENCER_E2E_ANTIGRAVITY_INSTANCE_FILE`, and `CODENCER_E2E_ANTIGRAVITY_DAEMON_DIR`.
- Fixed connector shared-instance discovery so an allowlisted manifest identity is not overwritten by an unrelated daemon that happens to be listening on the manifest URL during tests.
- Tightened the live Console verifier so the real executor result check targets the Summary heading deterministically.

## Proofs

- Codex + Claude Code artifact-backed RC subgates passed:
  - Report: `reports/public-selfhost-rc/20260624T105654Z/summary.md`
  - Gates: `real_executor_e2e_codex`, `real_executor_e2e_claude`
  - Required proof log: `reports/public-selfhost-rc/20260624T105654Z/required_real_executor_proofs.log`
  - Evidence: Codex daemon log shows `Adapter Execution: Starting process`, `adapter=codex`, and the configured Codex binary; Claude verifier log shows `primary=claude` with simulation preflight disabled.
  - Overall verdict remains `NO-GO` because Antigravity is missing.
- Antigravity source-tree live proof was attempted with isolated temp instance metadata generated from the local running Antigravity language server.
  - The verifier bound the candidate instance by PID without writing user Codencer state.
  - Both reachable local Antigravity LS candidates failed to complete through the Antigravity adapter and fell through to `ide-chat`; the verifier failed instead of accepting fallback behavior.
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

## Commands Run

- `bash -n scripts/verify_public_selfhost_rc.sh` - passed
- `go test ./cmd/codencer` - passed
- `go test ./cmd/codencer ./internal/security` - passed
- `go test ./internal/localexec` - passed
- `go test ./internal/profile ./internal/adapters/codex` - passed
- `go test ./internal/connector -count=1` - passed
- `go test ./internal/gateway` - passed
- `go test ./internal/localexec ./internal/gateway ./cmd/codencer` - passed
- `go test ./...` - passed
- `cd web/gateway-console && npm run format:check` - passed
- `cd web/gateway-console && npm run lint` - passed
- `cd web/gateway-console && npm run typecheck` - passed
- `cd web/gateway-console && npm run test` - passed
- `cd web/gateway-console && npm run test -- --run tests/schemas.test.ts` - passed
- `cd web/gateway-console && npm run build` - passed
- `cd web/gateway-console && npm run test:e2e` - passed
- `make verify-gateway` - passed
- `make verify-gateway-console` - passed
- `make verify-gateway-console-live` - passed
- `CODENCER_E2E_REAL_EXECUTOR=codex CODENCER_E2E_REAL_EXECUTOR_COMMAND=<configured-codex-binary> make verify-public-selfhost-rc` - failed by design with `NO-GO` after Codex passed and remaining required proofs were missing
- `make verify-public-release` - passed
- `make verify-public-selfhost-release TARGETS=host REQUIRE_TARGETS=host` - passed
- `CODENCER_E2E_REAL_EXECUTORS=codex,claude CODENCER_E2E_CODEX_COMMAND=<codex-binary> CODENCER_E2E_CLAUDE_COMMAND=<claude-binary> make verify-public-selfhost-rc` - failed by design with `NO-GO` after Codex and Claude passed and Antigravity was missing
- `cd web/gateway-console && CODENCER_E2E_BIN_DIR=../../bin CODENCER_E2E_EXECUTOR_ADAPTER=antigravity CODENCER_E2E_EXECUTOR_PROFILE=antigravity-default CODENCER_E2E_ANTIGRAVITY_INSTANCE_FILE=<temp-file> node tests/live/verify-live.mjs` - failed correctly; Antigravity did not produce a valid run and fallback was not accepted
- `git diff --check` - passed

## Remaining Blockers

- Antigravity real executor proof is not proven in the public self-host RC gate.
- Current local Antigravity app processes expose reachable RPC endpoints, but the available candidates do not complete Codencer's isolated real-run proof.
- `codencer run resume` is exposed as a structured blocker because the daemon does not yet expose a resume HTTP route.
- Raw log/artifact upload remains unsupported by design. `codencer sync publish --confirm` ingests metadata-only run/project summaries into Gateway history; it does not upload local reports, logs, artifacts, daemon URLs, or filesystem paths.
- Run history/audit synced-scope transport now exists for explicit metadata-only `codencer sync publish`; broader incremental sync policy and external source reconciliation remain incomplete.
- Human interrupt lifecycle is still partial: local report/event records and Gateway blocker audit exist, but complete operator answer/resume UI/MCP flows are not fully proven.
- Full cross-surface redaction proof remains incomplete. Default local human CLI output and sync preview are covered, but explicit JSON/debug/path commands still require final policy review against the release gate.

Verdict: NO-GO
