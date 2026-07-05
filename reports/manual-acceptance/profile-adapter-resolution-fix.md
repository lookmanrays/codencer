# Profile Adapter Resolution Fix Acceptance

- Date: 2026-07-04
- Branch: next-phase
- Implementation commit: a751fdb26c1ec742c9a05f3e9a3925da247e69e7
- Verdict: GO
- Scope: Codex and Claude required public self-host acceptance.
- Deferred: Antigravity is optional/deferred and was not counted as a required proof.

## Root Cause

Task-level executor profile selection was not authoritative enough. When the UI selected `claude-default`, Gateway/Console sent profile metadata, but the local execution resolver still applied the project default adapter (`codex`) before profile resolution. That made the backend reject the selected profile with `profile "claude-default" is for adapter "claude", not "codex"`.

Gateway also did not forward an explicit `adapter` field from project task submit payloads, so even when the UI knew the selected executor adapter, the Connector/local executor path could not use that metadata.

## Fix Summary

- Explicit task-level profile now determines adapter when no explicit adapter is provided.
- Project default adapter is used only when no task-level profile override exists.
- Explicit adapter/profile conflicts still fail as `invalid_input`.
- Gateway project task submit now forwards `adapter`, `profile`, and `adapter_profile`.
- Connector project submit decodes and passes `adapter` into local execution.
- Gateway Console includes selected executor adapter metadata with submit requests.
- Live verifier logs UI run id/history id and continues to enforce real executor status.

## Files Changed

- `internal/profile/profile.go`
- `internal/profile/profile_test.go`
- `internal/localexec/service.go`
- `internal/localexec/service_test.go`
- `internal/connector/project_requests.go`
- `internal/gateway/tools.go`
- `internal/gateway/gateway_test.go`
- `web/gateway-console/api/runs.ts`
- `web/gateway-console/components/console/task-run-form.tsx`
- `web/gateway-console/schemas/runs.ts`
- `web/gateway-console/tests/architecture.test.ts`
- `web/gateway-console/tests/live/verify-live.mjs`

## Regression Coverage

- `internal/profile`: explicit `claude-default` resolves to adapter `claude` despite project default `codex`.
- `internal/localexec`: task submit with `Profile: claude-default` uses Claude; explicit `Adapter: codex` plus `Profile: claude-default` returns `invalid_input`.
- `internal/gateway`: project run endpoint forwards selected `adapter`, `profile`, and `adapter_profile` to Relay.
- Gateway Console architecture test: submit payload includes `adapter: input.executorAdapter`, sourced from the selected executor.
- Live verifier: real executor gate asserts adapter/profile, `is_simulation=false`, visible `Real executor`, run history/detail/audit coverage, and no simulation text in reports/logs.

## Commands Run

- `gofmt -w internal/profile/profile.go internal/profile/profile_test.go internal/localexec/service.go internal/localexec/service_test.go internal/connector/project_requests.go internal/gateway/tools.go internal/gateway/gateway_test.go`
- `go test ./internal/profile ./internal/localexec ./internal/gateway ./internal/connector`
- `cd web/gateway-console && npm run format:check && npm run lint && npm run typecheck && npm run test`
- `go test ./...`
- `cd web/gateway-console && npm run format:check && npm run lint && npm run typecheck && npm run test && npm run build && npm run test:e2e`
- `make verify-gateway`
- `make verify-gateway-console`
- `make verify-gateway-console-live`
- `cd web/gateway-console && ALL_ADAPTERS_SIMULATION_MODE=0 CODEX_SIMULATION_MODE=0 CODENCER_E2E_BIN_DIR=<repo>/bin CODENCER_E2E_EXECUTOR_ADAPTER=codex CODENCER_E2E_EXECUTOR_PROFILE=codex-workspace CODENCER_E2E_REAL_EXECUTOR_COMMAND=<codex-binary> CODEX_BINARY=<codex-binary> node tests/live/verify-live.mjs`
- `cd web/gateway-console && ALL_ADAPTERS_SIMULATION_MODE=0 CLAUDE_SIMULATION_MODE=0 CODENCER_E2E_BIN_DIR=<repo>/bin CODENCER_E2E_EXECUTOR_ADAPTER=claude CODENCER_E2E_EXECUTOR_PROFILE=claude-default CODENCER_E2E_REAL_EXECUTOR_COMMAND=<claude-binary> CLAUDE_BINARY=<claude-binary> node tests/live/verify-live.mjs`
- `CODENCER_E2E_REAL_EXECUTORS=codex,claude CODENCER_E2E_CODEX_COMMAND=<codex-binary> CODENCER_E2E_CLAUDE_COMMAND=<claude-binary> CODEX_BINARY=<codex-binary> CLAUDE_BINARY=<claude-binary> ALL_ADAPTERS_SIMULATION_MODE=0 CODEX_SIMULATION_MODE=0 CLAUDE_SIMULATION_MODE=0 make verify-public-selfhost-rc`
- `make verify-public-release`
- `make verify-public-selfhost-release TARGETS=host REQUIRE_TARGETS=host`
- `git diff --check`
- `git diff --cached --check`

## Real Executor Proof

Standalone live verifier:

- Codex: UI run `run-1783188359`, history `runhist_0d825f6085b843f3e94f382c21667ce1`, adapter `codex`, profile `codex-workspace`, simulation `false`.
- Claude: UI run `run-1783188432`, history `runhist_1978b5f099a92d6cb1c3a6645c4cadf5`, adapter `claude`, profile `claude-default`, simulation `false`.

Artifact-backed RC verifier:

- Report: `reports/public-selfhost-rc/20260704T181016Z/summary.md`
- Verdict: GO
- Required proofs: Codex and Claude passed.
- Optional/deferred: Antigravity.
- Codex MCP run: `run-1783188667`
- Codex UI run: `run-1783188709`
- Codex run history: `runhist_40e09b7d1cd5c2affbea708cdeb3fea5`
- Codex adapter/profile: `codex` / `codex-workspace`
- Codex simulation: `false`
- Claude MCP run: `run-1783188753`
- Claude UI run: `run-1783188771`
- Claude run history: `runhist_ecefbeb13740f4f2f9c5a9b30647ca17`
- Claude adapter/profile: `claude` / `claude-default`
- Claude simulation: `false`

The RC verifier logs include real-executor simulation preflight with simulation envs set to `0`, and the verifier rejects `is_simulation=true`, simulated summaries/logs, missing real output/artifacts, and adapter/profile mismatches.

## Simulation And Secret Checks

- Simulation marker check: passed for required Codex and Claude real executor gates.
- Rejected markers: `is_simulation=true`, `Executing Simulated`, `Simulated successful`, and `Simulation Mode`.
- User-facing output safety: live verifier and Console checks passed without exposing tokens/secrets in UI output.
- Fake executors were used only for plumbing smoke and were not counted toward required release acceptance.

## Run History, Detail, And Audit Proof

The live verifier exercised:

- Gateway MCP submit and report read.
- Gateway Console UI submit.
- `/console/runs` run history.
- `/console/runs/[id]` run detail.
- `/console/audit` lifecycle event links.
- Expected lifecycle events: `task_submitted`, `route_resolved`, `relay_selected`, `connector_selected`, `executor_selected`, `run_started`, `run_completed`, `report_read`.

## Visual Evidence

- `make verify-gateway-console` generated visual evidence at `reports/gateway-console-screenshots/2026-07-04-2102`.
- `make verify-public-selfhost-release TARGETS=host REQUIRE_TARGETS=host` generated visual evidence at `reports/gateway-console-screenshots/2026-07-04-2115`.
- These evidence directories are ignored by repository policy and were not committed.

## Non-Final Attempts

- A first direct Codex live verifier attempt failed before execution because `CODENCER_E2E_BIN_DIR` was relative; it was rerun with `CODENCER_E2E_BIN_DIR` pointing at the repository `bin/` directory.
- A first RC verifier attempt with only `CODENCER_E2E_REAL_EXECUTOR=codex` returned NO-GO as expected because Claude is now required by default. The successful run used `CODENCER_E2E_REAL_EXECUTORS=codex,claude`.

## Remaining Limitations

- Antigravity remains optional/deferred and is not part of the required public self-host GO.
- Release artifacts are unsigned/not notarized unless handled by a separate signing flow.
- The real executor proof depends on locally authenticated Codex and Claude CLIs on this machine.

## Final Verdict

Verdict: GO

Required Codex and Claude real executor acceptance is complete. Antigravity is explicitly deferred and must not be represented as proven.
