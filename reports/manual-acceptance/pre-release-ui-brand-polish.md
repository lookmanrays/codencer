# Pre-release UI Brand Polish Acceptance

- Implementation commit: `f4a69eb714a22f10130d6726cda9fd83dc0adf83`
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
- `web/gateway-console/tests/architecture.test.ts`
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
- Replaced the static `*`-line CLI spinner with a Go port of `design_handoff_codencer_logo/cli/codencer-working.mjs`.
- Added `internal/cliui.WorkingIndicator` with 135ms tick, compact terminal mark `█▌▊▎`, ANSI truecolor orange `38;2;255;90;31`, deterministic mark flicker, braille spinner, task list rendering, cursor hide/restore, and same-block redraw.
- Updated `codencer intro` to run a 5-step visual preview matching the handoff task rhythm: `read schema`, `plan diff`, `apply patch`, `run tests`, `verify`.
- Updated interactive `codencer setup self-host` and `codencer setup relay` to use the same working indicator on stderr, while keeping `--json`, CI, non-TTY, `NO_COLOR`, and no-animation paths clean.
- Updated interactive `codencer submit --wait` to use the same working indicator on stderr while the blocking executor call runs. Existing stdout reports, JSON output, routing, storage, artifacts, and Gateway/Relay/Connector/MCP behavior remain unchanged.
- Added deterministic Go tests for mark frames, spinner cycle, task rendering, cursor controls, fallback behavior, and setup/intro machine-safety; added a web regression guard for static mark geometry and resting colors.

## Checks Run

- `gofmt -w internal/cliui/cliui.go internal/cliui/cliui_test.go cmd/codencer/main.go cmd/codencer/main_test.go`
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
- `node <handoff>/design_handoff_codencer_logo/cli/codencer-working.mjs` via the exported `createWorking()` demo
- `env -u NO_COLOR <temporary-codencer-binary> intro`
- `git diff --check`
- `git diff --cached --check`
- `gofmt -w cmd/codencer/main.go cmd/codencer/main_test.go`
- `go test ./cmd/codencer ./internal/cliui`
- `go test ./...`
- `make verify-public-release`
- `CODENCER_HOME=<acceptance-home> <artifact-codencer> submit --project codencer --repo <acceptance-test-project> --profile codex-workspace --title "CLI task - Codex README check" --goal "Read README.md and docs/README.md. Return exactly three bullets. Do not modify files." --timeout-seconds 300 --wait`
- `env -u NO_COLOR CODENCER_HOME=<acceptance-home> <artifact-codencer> submit --project codencer --repo <acceptance-test-project> --profile codex-workspace --title "CLI task - Codex README check" --goal "Read README.md and docs/README.md. Return exactly three bullets. Do not modify files." --timeout-seconds 300 --wait`

All listed checks passed.

## CLI Submit Wait Check

- Disabled-motion safety: with `NO_COLOR=1`, `codencer submit --wait` stayed plain, emitted no decorative animation, and completed run `run-1783343847`.
- Interactive TTY behavior: with `NO_COLOR` removed, `codencer submit --wait` immediately showed the Codencer compact mark, orange braille spinner, `running` header, task list, same-block redraw, cursor hide/restore, and final `✓ done` line before the normal report.
- Manual real CLI run: `run-1783343930`, profile `codex-workspace`, status `completed`.
- The indicator content contains only static safe step labels: `prepare task`, `start executor`, `wait for result`, `collect report`, and `verify output`.
- The final stdout report remained the existing human report and included the executor result after the indicator stopped.

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
- CLI intro/progress output is explicit through `codencer intro`, setup progress, and `codencer submit --wait`; it uses the handoff mark flicker and braille spinner, writes only human-readable output outside JSON mode, and falls back safely for `--json`, CI, non-TTY stdout/stderr, `CODENCER_NO_ANIMATION=1`, and `NO_COLOR`.
- `codencer submit --wait` writes the working indicator only to stderr and preserves stdout for the existing final report.
- Existing JSON and verifier paths remain machine-readable.

## Deferred

- Cloud/official connector implementation remains out of scope for this public self-host release pass.
- Antigravity proof is optional/deferred and was not required for this GO verdict.

Verdict: GO
