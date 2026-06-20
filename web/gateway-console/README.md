# Codencer Gateway Console

Public/self-host Gateway Console for Codencer Core, `codencer-gatewayd`,
self-host Relay, MCP, and controlled pre-production official connector review.

This app is not the private managed Codencer Cloud UI. Do not add billing,
team/admin workflows, support console, production provider login, managed
runners, private Cloud operations, or hosted-service secrets here.

## Commands

```bash
npm ci
npm run format:check
npm run lint
npm run typecheck
npm run test
npm run build
npm run test:e2e
npm run visual:evidence
```

Repository-level verification:

```bash
make verify-gateway-console
```

## Mode

The console defaults to live mode and reads/writes through `codencer-gatewayd`
via the server-side Next proxy. Configure the proxy with:

```bash
CODENCER_GATEWAY_API_BASE=http://127.0.0.1:19090
CODENCER_GATEWAY_MCP_TOKEN=...
```

Demo mode is explicit:

```bash
NEXT_PUBLIC_CODENCER_CONSOLE_MODE=demo
```

Live mode never silently falls back to demo fixtures. Missing Gateway endpoints
or failed mutations render explicit error/unavailable states.

## Visual Evidence

Generate Chromium screenshot evidence:

```bash
npm run visual:evidence
```

Artifacts are written under:

```text
../../reports/gateway-console-screenshots/YYYY-MM-DD-HHMM/
```

The visual evidence run asserts that mobile document/body widths do not exceed
the `390px` viewport and that generated mobile PNGs are exactly `390px` wide.
`make verify-gateway-console` includes this visual gate and runs in explicit
demo mode. `make verify-gateway-console-live` runs browser flows against an
isolated live Gateway.

Timestamped PNG artifacts are ignored by default. Commit screenshot tooling and
small curated evidence only when a review specifically needs it.
