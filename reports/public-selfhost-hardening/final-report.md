# Public Self-host Hardening Final Report

Date: 2026-06-24

Commit hash at verification start: `72f98945aa1e69e84863919a40c94bee587b8bee`

Branch: `next-phase`

## Implementation Summary

- Added the public self-host release spec files under `docs/specs/` and the acceptance gate at `docs/acceptance/public-selfhost-release-gate.yaml`.
- Created the pre-change implementation audit at `reports/public-selfhost-hardening/implementation-audit.md`.
- Hardened the public self-host RC verifier so it emits only `GO` or `NO-GO`, rejects real-executor simulation env values, runs configured real executor gates by adapter, and fails the release gate when required real proofs are missing.
- Confirmed the real Codex path invokes the configured Codex binary with `ALL_ADAPTERS_SIMULATION_MODE=0` and `CODEX_SIMULATION_MODE=0`.
- Added `codencer run events`, `codencer run report`, `codencer run cancel`, and structured `codencer run resume` blocker behavior.
- Added `codencer sync status`, `codencer sync preview`, and `codencer sync publish` as explicit metadata-only sync controls. Raw artifacts/logs are blocked, and publish returns structured confirmation/unsupported blockers until Gateway ingest exists.
- Redacted local absolute repo/report paths from default human CLI report output.
- Added Gateway run-history `scope` metadata and exposed it through the API and Console run list/detail views.
- Added Antigravity executor profiles so executor discovery exposes Antigravity as a real profile family.
- Fixed connector shared-instance discovery so an allowlisted manifest identity is not overwritten by an unrelated daemon that happens to be listening on the manifest URL during tests.
- Tightened the live Console verifier so the real executor result check targets the Summary heading deterministically.

## Proofs

- Real Codex artifact-backed RC subgate passed:
  - Report: `reports/public-selfhost-rc/20260624T090832Z/summary.md`
  - Gate: `real_executor_e2e_codex`
  - Log: `reports/public-selfhost-rc/20260624T090832Z/real_executor_e2e_codex.log`
  - Evidence: daemon log shows `Adapter Execution: Starting process`, `adapter=codex`, and a configured Codex binary.
  - Simulation preflight printed `ALL_ADAPTERS_SIMULATION_MODE=0 CODEX_SIMULATION_MODE=0`.
- Overall public self-host RC verdict is correctly `NO-GO`:
  - `required_real_executor_proofs` failed because `claude` and `antigravity` remain missing.
  - Required proof log: `reports/public-selfhost-rc/20260624T090832Z/required_real_executor_proofs.log`.
- Gateway Console visual evidence regenerated:
  - `reports/gateway-console-screenshots/2026-06-24-1202`
  - `reports/gateway-console-screenshots/2026-06-24-1213`

## Commands Run

- `bash -n scripts/verify_public_selfhost_rc.sh` - passed
- `go test ./cmd/codencer` - passed
- `go test ./internal/localexec` - passed
- `go test ./internal/profile ./internal/adapters/codex` - passed
- `go test ./internal/connector -count=1` - passed
- `go test ./internal/gateway` - passed
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
- `CODENCER_E2E_REAL_EXECUTOR=codex CODENCER_E2E_REAL_EXECUTOR_COMMAND=<configured-codex-binary> make verify-public-selfhost-rc` - failed by design with `NO-GO` after Codex passed and Claude/Antigravity proofs were missing
- `make verify-public-release` - passed
- `make verify-public-selfhost-release TARGETS=host REQUIRE_TARGETS=host` - passed
- `git diff --check` - passed

## Remaining Blockers

- Claude Code real executor proof is not configured or proven in the public self-host RC gate.
- Antigravity real executor proof is not configured or proven in the public self-host RC gate.
- `codencer run resume` is exposed as a structured blocker because the daemon does not yet expose a resume HTTP route.
- Gateway metadata ingest for explicit sync/publish remains unimplemented. The CLI now provides safe metadata-only preview/status and structured publish blockers; no local reports, logs, artifacts, or paths are uploaded.
- Run history/audit pagination, broader filters, grouping, and synced-scope transport remain incomplete against the full public self-host hardening spec.
- Human interrupt lifecycle is still partial outside the existing gate/blocker primitives.

Verdict: NO-GO
