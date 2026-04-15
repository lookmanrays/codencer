# Codencer Cloud

Codencer Cloud is the alpha cloud control plane for tenant-scoped provider integrations and tenant-scoped Codencer runtime control. It does not execute coding work, but it can now own the cloud-facing registry for claimed Codencer connectors and shared instances when started with an internal relay bridge.

## What It Does

- bootstraps org, workspace, project, and API token records
- serves cloud status, org, workspace, project, token, installation, event, and audit routes
- manages connector installation enable/disable state
- records connector events, action logs, and audit events
- claims Codencer runtime connectors into org/workspace/project scope
- lists tenant-scoped claimed runtime connectors and shared runtime instances
- proxies tenant-scoped runtime HTTP operations through an internal relay bridge when configured

## What It Does Not Do

- it does not execute coding work
- it does not replace the local daemon, relay, or connector bridge
- it does not provide cloud billing, multi-tenant SaaS UI, or enterprise IAM
- it does not add raw shell or arbitrary filesystem access
- it does not yet provide a cloud-scoped MCP surface; runtime control is cloud-scoped over HTTP in this pass

## Binaries

- `codencer-cloudd`: cloud control-plane server
- `codencer-cloudctl`: admin CLI for cloud bootstrap and control-plane operations
- `codencer-cloudworkerd`: background worker for provider polling and maintenance

## Runtime Composition

The cloud daemon serves the cloud API under `/api/cloud/v1/*`.

It can also start an internal relay runtime bridge when started with `--relay-config` or a config file that sets `relay_config_path`. In that mode:

- cloud owns the public control-plane surface
- cloud claims runtime connectors into org/workspace/project scope
- cloud keeps a tenant-scoped runtime connector and runtime instance registry
- cloud proxies tenant-scoped runtime HTTP operations through the in-process relay server
- raw relay routes are not the canonical cloud-facing surface

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
- `GET|POST /api/cloud/v1/runtime/connectors`
- `GET /api/cloud/v1/runtime/connectors/{id}`
- `POST /api/cloud/v1/runtime/connectors/{id}/enable`
- `POST /api/cloud/v1/runtime/connectors/{id}/disable`
- `POST /api/cloud/v1/runtime/connectors/{id}/sync`
- `GET /api/cloud/v1/runtime/connectors/{id}/instances`
- `GET /api/cloud/v1/runtime/instances`
- `GET /api/cloud/v1/runtime/instances/{id}`
- `GET|POST /api/cloud/v1/runtime/instances/{id}/runs`
- `GET /api/cloud/v1/runtime/instances/{id}/runs/{run_id}`
- `POST /api/cloud/v1/runtime/instances/{id}/runs/{run_id}/steps`
- `GET /api/cloud/v1/runtime/instances/{id}/runs/{run_id}/gates`
- `POST /api/cloud/v1/runtime/instances/{id}/runs/{run_id}/abort`
- `GET /api/cloud/v1/runtime/instances/{id}/steps/{step_id}`
- `GET /api/cloud/v1/runtime/instances/{id}/steps/{step_id}/result`
- `GET /api/cloud/v1/runtime/instances/{id}/steps/{step_id}/validations`
- `GET /api/cloud/v1/runtime/instances/{id}/steps/{step_id}/logs`
- `GET /api/cloud/v1/runtime/instances/{id}/steps/{step_id}/artifacts`
- `GET /api/cloud/v1/runtime/instances/{id}/artifacts/{artifact_id}/content`
- `POST /api/cloud/v1/runtime/instances/{id}/gates/{gate_id}/approve`
- `POST /api/cloud/v1/runtime/instances/{id}/gates/{gate_id}/reject`
- `GET /api/cloud/v1/events`
- `GET /api/cloud/v1/audit`

Planner/admin calls are bearer-token authenticated and scoped by org/workspace/project. Runtime operations stay explicitly instance-scoped on the cloud HTTP surface.

## Command Surface

`codencer-cloudctl` mirrors the cloud API with a narrow CLI:

- `bootstrap`
- `status`
- `orgs` / `orgs create`
- `workspaces` / `workspaces create`
- `projects` / `projects create`
- `tokens` / `tokens create|revoke`
- `install` / `install create|get|validate|enable|disable|action`
- `runtime-connectors` / `runtime-connectors claim|get|enable|disable|sync|instances`
- `runtime-instances` / `runtime-instances list|get`
- `events`
- `audit`

Use `bootstrap` to seed a new org/workspace/project token set into the same SQLite store used by the cloud daemon. Because `bootstrap` writes the store directly, run it before starting the daemon or while the database is idle.

The runtime CLI covers cloud-scoped claim/list/get flows for claimed runtime connectors and instances. Provider-action and runtime-execution flows still remain easier to script directly against the HTTP API.

## Current Truth

- Cloud runtime control is tenant-scoped over HTTP in this pass.
- Raw relay routes are still available from `codencer-relayd` for self-host relay use, but they are not the cloud control-plane contract.
- Cloud runtime control requires `codencer-cloudd` to be started with `relay_config_path` or `--relay-config`.
- Jira is polling-first in this alpha pass.
- Jira webhook ingest is not implemented.
- `codencer-cloudworkerd` is the place where Jira polling runs.
- `cloud_smoke.sh` intentionally exercises bootstrap, status, list, create, get, enable, disable, events, and audit flows. Optional runtime claim/list assertions are available when the operator already has relay runtime state and provides a runtime connector id.

For operator steps and startup ordering, see [CLOUD_SELF_HOST.md](CLOUD_SELF_HOST.md). For provider capability details, see [CLOUD_CONNECTORS.md](CLOUD_CONNECTORS.md).
