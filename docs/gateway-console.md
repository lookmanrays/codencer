# Gateway Console

Status: public UI foundation implemented under `web/gateway-console`.

The current public repository ships command-line and API surfaces for local
Codencer Core, self-host Gateway, self-host Relay, MCP, activation, community
cloud-control-plane operation, and a public Gateway Console foundation. It does
not ship billing UI, team/admin UI, support console, marketplace console, or
managed Cloud operations dashboard.

## Current Operator Surfaces

- `codencer login`, `codencer whoami`, and `codencer logout`
- `codencer connector login`
- `codencer gateway relay add|list|status|remove`
- `codencer activation official`
- `codencer-gatewayd`
- `codencer-relayd`
- `codencer-connectord`
- community cloud-control-plane CLIs documented in [CLOUD.md](CLOUD.md)
- `web/gateway-console` Next.js UI foundation

## Verification And Visual Evidence

Run the standard UI gate:

```bash
make verify-gateway-console
```

Generate browser screenshot evidence:

```bash
cd web/gateway-console
npm run visual:evidence
```

The visual run writes local artifacts under
`reports/gateway-console-screenshots/YYYY-MM-DD-HHMM/` with full-page route
screenshots, interaction-state screenshots, `index.md`, and
`visual-review.md`. It fails on mobile horizontal overflow and verifies mobile
PNG widths at exactly `390px`. Timestamped PNGs are ignored by default to avoid
repository bloat. See [UI visual evidence](ui/visual-evidence.md).

The console is mock-backed by default unless
`NEXT_PUBLIC_CODENCER_CONSOLE_MOCKS=false` is configured. Live Gateway API
coverage is limited to the read-only paths actually wired in the UI data client.

## Public Console Scope

The public foundation covers:

- dashboard;
- Relay profiles;
- machines/connectors;
- projects/project locations;
- activation commands;
- audit/events;
- settings;
- device approval;
- OAuth dev consent;
- UI system reference.

## Future Private Console Scope

A future private managed-service Console may cover:

- production user/workspace session management;
- production token revocation controls;
- OAuth client setup for the managed service;
- team/admin workflows;
- billing and quota surfaces for managed Cloud plans;
- operational health views.

Managed-service Console work belongs to a separate follow-up and may live in a
private managed Codencer Cloud repository. The public repo should keep the
self-host API/CLI paths working without depending on that UI.
