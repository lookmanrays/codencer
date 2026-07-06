# Pre-release UI Brand Polish Acceptance

- Implementation commit: `6dc1c8302`
- Branch: `next-phase`
- Date: `2026-07-06`
- Verdict: GO

## Files Changed

- `cmd/codencer/main.go`
- `cmd/codencer/main_test.go`
- `internal/cliui/cliui.go`
- `internal/cliui/cliui_test.go`
- `web/gateway-console/api/demo-data.ts`
- `web/gateway-console/app/globals.css`
- `web/gateway-console/app/layout.tsx`
- `web/gateway-console/app/console/projects/run/page.tsx`
- `web/gateway-console/app/ui-system/page.tsx`
- `web/gateway-console/components/brand/codencer-brand.tsx`
- `web/gateway-console/components/console/project-locations-table.tsx`
- `web/gateway-console/components/console/run-result-panel.tsx`
- `web/gateway-console/components/console/task-run-form.tsx`
- `web/gateway-console/components/layout/sidebar.tsx`
- `web/gateway-console/components/layout/topbar.tsx`
- `web/gateway-console/components/ui/data-table.tsx`
- `web/gateway-console/features/console/project-run-screen.tsx`
- `web/gateway-console/features/console/projects-screen.tsx`
- `web/gateway-console/features/console/relays-screen.tsx`
- `web/gateway-console/features/console/run-detail-screen.tsx`
- `web/gateway-console/features/console/runs-screen.tsx`
- `web/gateway-console/public/brand/app-icon-light.svg`
- `web/gateway-console/public/brand/codencer-lockup.svg`
- `web/gateway-console/public/brand/codencer-mark-on-dark.svg`
- `web/gateway-console/public/brand/codencer-mark.svg`
- `web/gateway-console/public/brand/codencer-wordmark.svg`
- `web/gateway-console/public/favicon.svg`
- `web/gateway-console/public/icon.svg`
- `web/gateway-console/tests/e2e/console.spec.ts`
- `web/gateway-console/tests/live/verify-live.mjs`
- `web/gateway-console/tests/visual/visual-evidence.spec.ts`

## Implementation Summary

- Added local Codencer brand assets from the provided design handoff: four-bar row mark, lowercase `codencer` wordmark, lockup SVG, favicon, and static app icons.
- Updated Console sidebar and topbar to use the mark/lockup. Expanded sidebar shows mark plus lowercase wordmark; collapsed sidebar shows the mark only.
- Updated the Console accent token to `#ff5a1f` and wired favicon/app icon metadata without remote assets or fonts.
- Reworked Projects into a list-first inventory page with compact filters, a full-width project/location table, and row `Run` actions that open the dedicated task page at `/console/projects/run`.
- Added the dedicated project run page with selected project/location summary, executor selector, route preview, task defaults, advanced controls, submit result, and Run Detail link.
- Reworked Relays into a list-first page with a compact full-width table and moved `Add relay profile` into a modal dialog so the add form no longer consumes the desktop layout.
- Added DataTable density and table minimum-width options, then applied compact sizing to Projects and Relays to avoid unnecessary horizontal scrolling.
- Reduced Runs list width by using compact columns, truncated values, compact real/simulation label, and row-level navigation to Run Detail.
- Cleaned Run Detail by keeping metadata in the top grid, shortening the Result block, and deduping artifacts by stable IDs/hashes/name/type/size fallback.
- Added `internal/cliui` for safe static CLI brand/progress output only when stdout and stderr are TTYs and JSON/CI/no-animation/no-color guards allow it.
- Added `codencer intro` as an explicit human-only CLI brand/progress preview command. It is machine-safe for `--json`, CI, non-TTY, `NO_COLOR`, and no-animation environments.

## Checks Run

- `gofmt -w cmd/codencer/main.go cmd/codencer/main_test.go`
- `go test ./...`
- `go test ./cmd/codencer ./internal/cliui`
- `cd web/gateway-console && npm run format:check && npm run lint && npm run typecheck && npm run test && npm run build && npm run test:e2e`
- `make verify-gateway`
- `make verify-gateway-console`
- `make verify-gateway-console-live`
- `CODENCER_E2E_REAL_EXECUTORS=codex,claude CODEX_BINARY=<codex-binary> CLAUDE_BINARY=<claude-binary> make verify-public-selfhost-rc`
- `make verify-public-selfhost-release TARGETS=host REQUIRE_TARGETS=host`
- `make verify-public-release`
- `<temporary-codencer-binary> intro --json`
- `<temporary-codencer-binary> intro`
- `env -u NO_COLOR <temporary-codencer-binary> intro`
- `git diff --check`
- `git diff --cached --check`

All listed checks passed.

## Visual Evidence

- Screenshot evidence path: `reports/gateway-console-screenshots/2026-07-06-1358`
- Visual review: `reports/gateway-console-screenshots/2026-07-06-1358/visual-review.md`
- Captured expanded and collapsed sidebar states, Projects list, dedicated project run page, Runs, Run Detail, Settings, Relays list, Add Relay dialog, Audit, mobile routes, and interaction states.
- Generated visual review reports no automated screenshot, overflow, or security-gate issues.
- Mobile PNG width gate passed at exactly `390px` for every mobile capture.
- PNG evidence is intentionally left local and was not committed.

## Real Codex And Claude Proof

- RC report: `reports/public-selfhost-rc/20260706T104912Z/summary.md`
- RC verdict: `GO`
- Required real executor proofs: `codex,claude`
- Optional/deferred executor: `antigravity`

Codex:

- MCP run: `run-1783334999`
- UI run: `run-1783335035`
- Run history: `runhist_94fb4f04251215391669b1dd78d29882`
- Proof line: `adapter=codex profile=codex-workspace simulation=false`
- Binary invocation was logged with the configured `CODEX_BINARY`.

Claude:

- MCP run: `run-1783335083`
- UI run: `run-1783335097`
- Run history: `runhist_8dcacb9c73022e3087695efac6482fde`
- Proof line: `adapter=claude profile=claude-default simulation=false`

## MCP Proof Status

- `make verify-public-selfhost-rc` passed `gateway_console_live_artifact`, `real_executor_e2e_codex`, and `real_executor_e2e_claude`.
- The real executor gates exercise Gateway Console live mode and MCP submit/report paths before declaring the required executor proofs passed.
- `required_real_executor_proofs.log` records `codex=passed` and `claude=passed`.

## Safety And Boundary Notes

- No private Cloud, billing, team/admin, KMS/Vault, managed-runner, or placeholder hosted-service UI was added.
- UI tests continue to assert that product navigation hides UI System and public self-host Settings/Relay pages do not expose private placeholders.
- Visual evidence security scan found no raw token, private key, bearer header, or local absolute path leakage.
- CLI intro/progress output is explicit through `codencer intro`, writes only human-readable output outside JSON mode, and is disabled for `--json`, CI, non-TTY stdout/stderr, `CODENCER_NO_ANIMATION=1`, and `NO_COLOR`.
- Existing JSON and verifier paths remain machine-readable.

## Deferred

- Full terminal logo animation is deferred; this pass only adds safe static CLI brand/progress output.
- Cloud/official connector implementation remains out of scope for this public self-host release pass.
- Antigravity proof is optional/deferred and was not required for this GO verdict.

Verdict: GO
