# Public Self-host User-like Acceptance

Commit under test: `16bcca117`

## Files changed

- `internal/gateway/gateway_test.go`
- `internal/gateway/run_history.go`
- `internal/gateway/store.go`
- `internal/gateway/tools.go`
- `internal/relay/mcp_tools.go`
- `web/gateway-console/api/demo-data.ts`
- `web/gateway-console/app/ui-system/page.tsx`
- `web/gateway-console/components/console/mode-notices.tsx`
- `web/gateway-console/components/console/run-result-panel.tsx`
- `web/gateway-console/features/console/activation-screen.tsx`
- `web/gateway-console/features/console/audit-screen.tsx`
- `web/gateway-console/features/console/dashboard-screen.tsx`
- `web/gateway-console/features/console/relays-screen.tsx`
- `web/gateway-console/features/console/run-detail-screen.tsx`
- `web/gateway-console/features/console/runs-screen.tsx`
- `web/gateway-console/features/console/settings-screen.tsx`
- `web/gateway-console/schemas/run-history.ts`
- `web/gateway-console/schemas/runs.ts`
- `web/gateway-console/tests/e2e/console.spec.ts`
- `web/gateway-console/tests/live/verify-live.mjs`

## Root cause

Gateway run history and report refresh paths were reapplying the project default executor after submit. A task-level selection such as `claude/claude-default` reached the initial Gateway route and audit events, but later report/history normalization could refill executor metadata from the project default `codex/codex-workspace` when refresh arguments lacked explicit task executor metadata.

The fix preserves explicit task-level executor metadata as stronger than project defaults and only falls back to project defaults when the run record and current request have no executor metadata.

## Implementation summary

- Added `executor_adapter` to Gateway run history storage and API output.
- Preserved explicit `adapter`, `profile`, and `adapter_profile` through Gateway submit, Gateway MCP submit, direct Relay MCP submit, run history, report refresh, and audit metadata.
- Added conflict validation so incompatible explicit adapter/profile combinations return `invalid_input` before execution.
- Extracted report executor metadata with task-level precedence before top-level fallback.
- Added regression coverage for a Codex-default project with a selected Claude executor.
- Hardened Gateway Console live verification so real executor gates require matching adapter/profile metadata and `simulation=false`.

## UI changes summary

- Settings now shows only public self-host facts and removes private Cloud/token-revocation placeholders.
- Relays uses the runtime MCP endpoint and a compact table with masked token references plus a bounded add-profile panel.
- Audit defaults to grouped run lifecycle, keeps non-run events separate, and collapses repeated `report_read` entries.
- Runs list is a compact table with one-line summary preview instead of full run bodies.
- Run detail uses compact metadata, attempts, artifacts/log tables, result panel, and compressed event timeline.
- Product navigation continues to hide UI System while keeping the route for development/demo checks.

## Exact setup and test commands

Local binary paths used for Codex and Claude were supplied as environment variables during verification and are intentionally redacted from this committed report.

```bash
gofmt -w internal/gateway/run_history.go internal/gateway/store.go internal/gateway/tools.go internal/relay/mcp_tools.go internal/gateway/gateway_test.go
go test ./...
cd web/gateway-console && npm run format:check && npm run lint && npm run typecheck && npm run test && npm run build && npm run test:e2e
make verify-gateway
make verify-gateway-console
make verify-gateway-console-live
CODENCER_E2E_REAL_EXECUTORS=codex,claude CODEX_BINARY=<codex-binary> CLAUDE_BINARY=<claude-binary> make verify-public-selfhost-rc
make verify-public-release
make verify-public-selfhost-release TARGETS=host REQUIRE_TARGETS=host
git diff --check
git diff --cached --check
```

## Real executor proof

Artifact-backed public self-host RC verifier report: `reports/public-selfhost-rc/20260705T195140Z/summary.md`

The verifier reported:

```text
Verdict: GO
Reason: real executor proofs passed for codex,claude
```

Run IDs from the same RC verifier:

| Path | Adapter | Profile | Run ID | Gateway history ID | Simulation |
| --- | --- | --- | --- | --- | --- |
| Codex MCP | `codex` | `codex-workspace` | `run-1783281147` | Verified by report/history gate | `false` |
| Codex UI | `codex` | `codex-workspace` | `run-1783281188` | `runhist_6ceeb5c69fdd51ff68c90b148d0bba7f` | `false` |
| Claude MCP | `claude` | `claude-default` | `run-1783281230` | Verified by report/history gate | `false` |
| Claude UI | `claude` | `claude-default` | `run-1783281245` | `runhist_90dba12e914f554b7dcc602514c0e0f4` | `false` |

The real executor logs recorded:

```text
gateway-console-live: ui_run=run-1783281188 run_history=runhist_6ceeb5c69fdd51ff68c90b148d0bba7f adapter=codex profile=codex-workspace simulation=false
gateway-console-live: ui_run=run-1783281245 run_history=runhist_90dba12e914f554b7dcc602514c0e0f4 adapter=claude profile=claude-default simulation=false
```

## Run History, Run Detail, Audit, and MCP proof

- Run History proof: the live verifier queried `GET /api/gateway/v1/runs` and asserted the UI-submitted run records retained `executor_adapter` and `executor_profile` for both Codex and Claude.
- Run Detail proof: the live verifier opened each submitted run detail page and asserted the adapter, executor profile, run ID, result, and real-executor status were visible.
- Audit proof: the live verifier opened the grouped Audit page, asserted lifecycle events including `task_submitted`, `route_resolved`, `relay_selected`, `connector_selected`, `executor_selected`, `run_started`, `run_completed`, and `report_read`, then followed the run-specific Audit link back to the run detail page.
- MCP proof: Gateway MCP initialize/list/submit/report checks ran for Codex and Claude. `codencer.submit_project_task` forwarded task executor metadata, and `codencer.get_run_report` was rejected unless adapter/profile matched the requested real executor and `is_simulation=false`.

## Simulation marker check

The RC verifier failed earlier when simulated execution was detected. For this acceptance run it asserted:

- `is_simulation=false`
- no `Simulation Mode`
- no `Executing Simulated`
- no `Simulated successful`
- adapter/profile must match the requested executor

Both required real executor proofs passed.

## Secret and local-path exposure check

The Console live verifier ran unsafe-output checks against rendered pages and Gateway responses. New run history/detail/audit UI continues to use sanitized metadata and does not render raw tokens, Relay bearer values, daemon URLs, `report_path`, `logs_ref`, or local artifact paths.

This report redacts local binary paths and temporary artifact directories from commands and prose.

## Remaining limitations

- Antigravity is optional/deferred and was not part of the release acceptance proof.
- Fake-success remains only a plumbing smoke path and does not satisfy real executor acceptance.
- The acceptance proof is Gateway-observed history, not a global aggregate of daemon runs started outside Gateway.

Verdict: GO
