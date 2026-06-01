# Live Execution Matrix

Sprint 5 adds a live-proof layer around the local production foundation. The matrix separates deterministic capability checks from live executor proof so operators can see what is installed, what is configured, what was actually exercised, and what still needs manual proof.

Codencer still does not plan. It starts approved, bounded smoke tasks, records structured reports, and surfaces blockers for the planner or operator.

## Commands

```bash
./bin/codencer live matrix --json
./bin/codencer live matrix --profile relay --json
./bin/codencer live codex --json
./bin/codencer live claude --json
./bin/codencer live relay-mcp --json
./bin/codencer live codex-mcp --json
./bin/codencer live claude-mcp --json
./bin/codencer live wsl --json
./bin/codencer live restart-reconnect --json
./bin/codencer readiness --json
```

Default commands do not call paid or authenticated live agents. Live checks require one of:

```text
CODENCER_LIVE_CODEX=1
CODENCER_LIVE_CLAUDE=1
CODENCER_LIVE_RELAY_MCP=1
CODENCER_LIVE_CODEX_MCP=1
CODENCER_LIVE_CLAUDE_MCP=1
CODENCER_LIVE_WSL=1
CODENCER_LIVE_SERVICE_RESTART=1
CODENCER_LIVE_ALL=1
```

The legacy `CODENCER_LIVE_CODEX_SMOKE=1` and `CODENCER_LIVE_CLAUDE_SMOKE=1` variables are still accepted by the Make targets.

## Reports

Live reports use stable machine-readable fields:

```json
{
  "ok": true,
  "profile": "local",
  "environment": {
    "os": "darwin",
    "arch": "arm64",
    "wsl": false,
    "codencer_home": "/tmp/codencer"
  },
  "checks": [],
  "summary": {
    "passed": 1,
    "failed": 0,
    "blocked": 0,
    "skipped": 1
  },
  "report_path": "/tmp/codencer/artifacts/live-matrix/..."
}
```

Statuses are `passed`, `failed`, `blocked`, `skipped`, `unsupported`, and `not_configured`. Skipped, unsupported, and not-configured checks are honest non-live outcomes, not failures.

Reports are written under:

```text
$CODENCER_HOME/artifacts/live-matrix/
$CODENCER_HOME/artifacts/readiness/
```

List them with:

```bash
./bin/codencer live reports --json
./bin/codencer readiness reports --json
```

## Disposable Workspace

Live executor smokes use a disposable temporary `CODENCER_HOME`, git repository, daemon state directory, and attempt workspace. The safe task asks the executor to create `codencer-live-result.txt` with `CODENCER_LIVE_SMOKE_OK`, then validates that file inside the attempt workspace.

Set `CODENCER_KEEP_LIVE_WORKSPACE=1` to keep the temporary workspace for inspection.

## Codex And Claude

`codencer live codex --json` uses `codex-workspace`. It does not use dangerous bypass or full-access mode by default.

`codencer live claude --json` uses `claude-default`.

Missing binaries are reported as skipped. Detectable login/auth failures map to `auth_required`; rate limits map to `rate_limit`; validation failures map to `validation_failed`.

## Relay/MCP

`codencer live relay-mcp --json` is opt-in and starts real temporary `orchestratord`, `codencer-relayd`, and `codencer-connectord` processes. It uses fake executor profiles through the normal daemon lifecycle, shares a temporary project to Relay, calls project-aware MCP tools, fetches evidence, and verifies blocker/validation behavior.

`codencer live restart-reconnect --json` uses the same foreground-process harness by default. It does not install or uninstall user services unless `CODENCER_LIVE_INSTALLED_SERVICES=1` is set.

## MCP Client Proof

`codencer live codex-mcp --json` and `codencer live claude-mcp --json` generate client configuration snippets and can verify an endpoint when enabled with tokens. They do not write user-level Codex or Claude config by default.

Project-scoped client config writes require:

```text
CODENCER_CODEX_CONFIG_WRITE=1
CODENCER_CLAUDE_CONFIG_WRITE=1
```

If product-client automation is not performed, the report uses `manual_client_proof_required` rather than claiming a fake pass.

## WSL

`codencer live wsl --json` reports `skipped` outside WSL. Inside WSL it detects the service manager and reports `unsupported` when systemd user services are unavailable. The primary Windows production path is Codencer inside WSL2, with GUI tools connecting through Relay or a future broker path.

## Readiness

`codencer readiness --json` aggregates local foundation, local execution, relay/MCP, runtime supervisor, and live matrix evidence into:

- `ready`
- `ready_with_skips`
- `not_ready`

`ready_with_skips` is expected when deterministic checks pass but live product checks were intentionally skipped.

## Sprint 6 Acceptance And Proof

Sprint 6 adds a release acceptance layer over readiness and live matrix:

```bash
codencer accept local-production --json --bin-dir ./bin --repo .
codencer accept local-production --profile local --json
codencer accept local-production --profile relay --json
codencer accept reports --json
codencer proof bundle --json
```

Acceptance reports distinguish required deterministic gates, optional live gates, and manual product-client proof. A default `ready_with_skips` report is honest when live Codex, live Claude, WSL, installed service, or ChatGPT product UI checks were skipped. `proof bundle` copies the latest JSON reports into `$CODENCER_HOME/artifacts/proof-bundles/` and excludes full logs and secrets by default.
