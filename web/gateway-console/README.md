# Codencer Gateway Console

Public Gateway Console UI foundation for Codencer Core, self-host Gateway,
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

## Mock Mode

The console is mock-backed by default:

```bash
NEXT_PUBLIC_CODENCER_CONSOLE_MOCKS=true
```

Set `NEXT_PUBLIC_CODENCER_CONSOLE_MOCKS=false` only when a compatible local
Gateway API is available. Current live API coverage is limited to the read-only
paths actually wired in `api/client.ts`; do not claim broader production API
coverage from mock screenshots.

## Visual Evidence

Generate Chromium screenshot evidence:

```bash
npm run visual:evidence
```

Artifacts are written under:

```text
../../reports/gateway-console-screenshots/YYYY-MM-DD-HHMM/
```

Timestamped PNG artifacts are ignored by default. Commit screenshot tooling and
small curated evidence only when a review specifically needs it.
