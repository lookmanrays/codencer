# Gateway Console

Status: public/self-host Gateway Console live integration implemented under
`web/gateway-console`.

The current public repository ships command-line and API surfaces for local
Codencer Core, self-host Gateway, self-host Relay, MCP, activation, community
cloud-control-plane operation, and a public/self-host Gateway Console. It does
not ship billing UI, team/admin UI, support console, marketplace console, or
managed Cloud operations dashboard.

## Current Operator Surfaces

- `codencer login`, `codencer whoami`, and `codencer logout`
- `codencer connector login`
- `codencer gateway relay add|list|status|remove`
- `codencer activation self-host`
- `codencer-gatewayd`
- `codencer-relayd`
- `codencer-connectord`
- community cloud-control-plane CLIs documented in [CLOUD.md](CLOUD.md)
- `web/gateway-console` Next.js public/self-host Gateway Console

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

The console defaults to live mode over `codencer-gatewayd`. Demo mode is
explicit through `NEXT_PUBLIC_CODENCER_CONSOLE_MODE=demo`; visual evidence uses
that explicit demo mode, while `make verify-gateway-console-live` exercises an
isolated live Gateway.

Live mode browser code calls the local Next proxy at `/api/gateway/v1/*`.
Configure that proxy with server-side env vars:

```bash
CODENCER_GATEWAY_API_BASE=http://127.0.0.1:19090
CODENCER_GATEWAY_CONSOLE_TOKEN=<gateway-console-token>
```

`CODENCER_GATEWAY_MCP_TOKEN` and `CODENCER_GATEWAY_TOKEN` are accepted as
fallback token env names for local/self-host operation.

## Public Exposure Warning

The public/self-host Gateway Console currently relies on server-side Gateway
token proxying. It is intended for localhost, internal network, private network,
or controlled self-host environments unless protected by external auth.

Do not expose Gateway Console directly to the public internet without one of:

- reverse proxy authentication;
- VPN;
- Zero Trust access;
- private network only;
- production auth layer from the future private Codencer Cloud service.

This is not a managed Codencer Cloud production auth implementation.

## Public Console Scope

The public/self-host Console covers:

- dashboard;
- Relay profiles;
- machines/connectors;
- projects/project locations;
- activation commands;
- audit/events;
- settings;
- device approval;
- OAuth dev consent;
- UI system reference with light and terminal code block variants.

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
