# Codencer Cloud

Codencer Cloud is the alpha cloud control plane for connector installations, operator bootstrap, and remote administration. It is separate from the local daemon execution path and separate from the self-host relay bridge.

## What It Does

- bootstraps org, workspace, project, and API token records
- serves cloud status, org, workspace, project, token, installation, event, and audit routes
- manages connector installation enable/disable state
- records connector events, action logs, and audit events
- optionally composes the relay handler under the same process

## What It Does Not Do

- it does not execute coding work
- it does not replace the local daemon, relay, or connector bridge
- it does not provide cloud billing, multi-tenant SaaS UI, or enterprise IAM
- it does not add raw shell or arbitrary filesystem access

## Binaries

- `codencer-cloudd`: cloud control-plane server
- `codencer-cloudctl`: admin CLI for cloud bootstrap and control-plane operations
- `codencer-cloudworkerd`: background worker for provider polling and maintenance

## Runtime Composition

The cloud daemon serves the cloud API under `/api/cloud/v1/*`.

It can also compose the relay handler when started with `--relay-config` or a config file that sets `relay_config_path`. In that mode, the same process can answer cloud API requests and relay requests, but the two domains remain separate:

- cloud control plane: org/workspace/project/token/install/event/audit flows
- relay bridge: `/api/v2`, `/mcp`, and `/ws/connectors`

## Public Cloud Routes

- `GET /healthz`
- `GET /api/cloud/v1/status`
- `GET|POST /api/cloud/v1/orgs`
- `GET|POST /api/cloud/v1/workspaces`
- `GET|POST /api/cloud/v1/projects`
- `GET|POST /api/cloud/v1/tokens`
- `POST /api/cloud/v1/tokens/{id}/revoke`
- `GET|POST /api/cloud/v1/installations`
- `GET /api/cloud/v1/installations/{id}`
- `POST /api/cloud/v1/installations/{id}/validate`
- `POST /api/cloud/v1/installations/{id}/enable`
- `POST /api/cloud/v1/installations/{id}/disable`
- `POST /api/cloud/v1/installations/{id}/actions`
- `POST /api/cloud/v1/installations/{id}/webhook`
- `GET /api/cloud/v1/events`
- `GET /api/cloud/v1/audit`

Planner/admin calls are bearer-token authenticated and scoped by org/workspace/project.

## Command Surface

`codencer-cloudctl` mirrors the cloud API with a narrow CLI:

- `bootstrap`
- `status`
- `orgs` / `orgs create`
- `workspaces` / `workspaces create`
- `projects` / `projects create`
- `tokens` / `tokens create|revoke`
- `install` / `install create|get|validate|enable|disable|action`
- `events`
- `audit`

Use `bootstrap` to seed a new org/workspace/project token set into the same SQLite store used by the cloud daemon. Because `bootstrap` writes the store directly, run it before starting the daemon or while the database is idle.

The HTTP list routes are the reliable list path today. The `cloudctl` list subcommands are still being normalized, so the smoke script and operator examples use the HTTP API for list assertions while relying on `cloudctl` for bootstrap, status, create, enable, disable, and revoke flows.

## Current Truth

- Jira is polling-first in this alpha pass.
- Jira webhook ingest is not implemented.
- `codencer-cloudworkerd` is the place where Jira polling runs.
- `cloud_smoke.sh` intentionally exercises bootstrap, status, list, create, get, enable, disable, events, and audit flows without claiming external provider verification.

For operator steps and startup ordering, see [CLOUD_SELF_HOST.md](CLOUD_SELF_HOST.md). For provider capability details, see [CLOUD_CONNECTORS.md](CLOUD_CONNECTORS.md).
