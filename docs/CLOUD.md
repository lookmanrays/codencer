# Codencer Cloud Control-Plane Notes

> [!WARNING]
> This document describes existing experimental/self-hostable cloud-control-plane code. It is not the current open-source release path and it is not hosted Codencer Gateway/Cloud availability. For current OSS remote operation, use [Self-Host Relay Quickstart](quickstart-self-host-relay.md) and [MCP Gateway Model](architecture/mcp-gateway-model.md).

Codencer Cloud control-plane code is an experimental self-hostable control plane for tenant-scoped provider integrations and tenant-scoped Codencer runtime control. It does not execute coding work, and it is separate from the v0.3 OSS self-host Relay path.

Use this page for the cloud scope and route contract.

- Use [CLOUD_SELF_HOST.md](CLOUD_SELF_HOST.md) for bootstrap, Docker baseline, and smoke order.
- Use [CLOUD_CONNECTORS.md](CLOUD_CONNECTORS.md) for per-provider install/test depth and limitations.
- Use [mcp/integrations.md](mcp/integrations.md) for the current OSS Relay planner/client path.

## What It Does

- bootstraps org, workspace, project, and API token records
- serves cloud status, org, workspace, project, token, installation, event, and audit routes
- serves membership and role-scoped access routes for org/workspace/project operators
- manages connector installation enable/disable state
- records connector events, action logs, and audit events
- claims Codencer runtime connectors into org/workspace/project scope
- lists tenant-scoped claimed runtime connectors and shared runtime instances
- proxies tenant-scoped runtime HTTP operations through an internal relay bridge when configured
- exposes a tenant-scoped cloud MCP surface for runtime control when the relay bridge is configured

## What It Does Not Do

- it does not execute coding work
- it does not replace the local daemon, relay, or connector bridge
- it does not provide cloud billing, multi-tenant SaaS UI, or enterprise IAM
- it does not add raw shell or arbitrary filesystem access
- it does not replace the self-host relay contract for operators who are not using cloud tenancy
- it does not automatically claim or auto-assign runtime connectors into tenant scope

## Binaries

- `codencer-cloudd`: cloud control-plane server
- `codencer-cloudctl`: admin CLI for cloud bootstrap and control-plane operations
- `codencer-cloudworkerd`: background worker for provider polling and maintenance

Build these optional self-host/community cloud-control-plane binaries with
`make build-self-host-cloud` or the lower-level `make build-cloud` alias. They
are not included in the primary local/self-host release snapshot.

## Runtime Composition

The cloud daemon serves the cloud API under `/api/cloud/v1/*`.

It can also start an internal relay runtime bridge when started with `--relay-config` or a config file that sets `relay_config_path`. In that mode:

- cloud owns the public control-plane surface
- cloud claims runtime connectors into org/workspace/project scope
- cloud keeps a tenant-scoped runtime connector and runtime instance registry
- cloud proxies tenant-scoped runtime HTTP operations through the in-process relay server
- cloud exposes the tenant-scoped MCP surface at `/api/cloud/v1/mcp`
- cloud exposes OAuth protected-resource metadata at `/.well-known/oauth-protected-resource/api/cloud/v1/mcp`
- raw relay planner routes and relay MCP are implementation details in composed mode, not the cloud-facing contract

## Access Model

Cloud now includes a minimal first-class control-plane access layer:

- `membership` records belong to an org and can optionally be bound to a workspace and project
- role values are:
  - `org_owner`
  - `org_admin`
  - `workspace_admin`
  - `project_operator`
  - `project_viewer`
- API tokens can be linked to a membership and are scope-clamped by that membership role
- connector installations and claimed runtime connectors now persist `owner_membership_id`
- audit events now attribute membership-linked tokens as membership actors instead of anonymous service tokens

This is still intentionally smaller than enterprise IAM. There is no SSO, no external identity provider, and no user-facing UI in this pass.

## Public Cloud Routes

- `GET /healthz`
- `GET /api/cloud/v1/status`
- `GET|POST /api/cloud/v1/orgs`
- `GET|POST /api/cloud/v1/workspaces`
- `GET|POST /api/cloud/v1/projects`
- `GET|POST /api/cloud/v1/memberships`
- `GET /api/cloud/v1/memberships/{id}`
- `POST /api/cloud/v1/memberships/{id}/enable`
- `POST /api/cloud/v1/memberships/{id}/disable`
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
- `GET|POST|DELETE /api/cloud/v1/mcp`
- `POST /api/cloud/v1/mcp/call`
- `GET /.well-known/oauth-protected-resource/api/cloud/v1/mcp`
- `GET /api/cloud/v1/events`
- `GET /api/cloud/v1/audit`

Planner/admin calls are bearer-token authenticated and scoped by org/workspace/project. Runtime operations stay explicitly instance-scoped on the cloud HTTP surface.

When cloud is started in composed runtime mode it also accepts local Codencer connector ingress at:

- `POST /api/v2/connectors/enroll`
- `POST /api/v2/connectors/challenge`
- `GET /ws/connectors`

Those routes exist so the connector can dial the cloud host directly in composed mode. They are not the planner/admin API surface.

## Cloud-Scoped MCP Surface

The cloud-scoped canonical remote tool surface now exists at `/api/cloud/v1/mcp`.

- It uses cloud bearer tokens, not relay planner tokens.
- Transport auth only requires a valid cloud token; individual tool calls still enforce their own runtime scopes.
- It exposes OAuth protected-resource metadata and 401 bearer challenges for product-facing remote MCP deployments.
- It does not issue OAuth authorization-code tokens; use an operator-owned OAuth issuer/front door when product setup requires that flow.
- `public_base_url`, `allowed_origins`, `oauth_authorization_servers`, `oauth_scopes_supported`, and `oauth_resource_documentation` configure the public MCP metadata/CORS shape.
- It enforces org/workspace/project visibility before any runtime tool can see an instance.
- It intentionally exposes only the narrow `codencer.*` runtime tool set.
- It is only useful when `codencer-cloudd` is started with a relay bridge.

Boundary rule:

- use `/api/cloud/v1/mcp` when the control plane is cloud tenancy
- use relay `/mcp` when operating the self-host relay directly without cloud tenancy

Both surfaces ultimately route through the same local runtime bridge doctrine, but only the cloud surface is tenant-scoped.

For the current OSS Relay planner/client path, see [mcp/integrations.md](mcp/integrations.md). For this experimental cloud-control-plane MCP surface, see [mcp/cloud_tools.md](mcp/cloud_tools.md).

## Command Surface

`codencer-cloudctl` mirrors the cloud API with a narrow CLI:

- `bootstrap`
- `status`
- `orgs` / `orgs create`
- `workspaces` / `workspaces create`
- `projects` / `projects create`
- `memberships` / `memberships list|create|get|enable|disable`
- `tokens` / `tokens create|revoke`
- `install` / `install create|get|validate|enable|disable|action`
- `runtime-connectors` / `runtime-connectors claim|get|enable|disable|sync|instances`
- `runtime-instances` / `runtime-instances list|get`
- `events`
- `audit`

Use `bootstrap` to seed a new org/workspace/project/membership token set into the same SQLite store used by the cloud daemon. Because `bootstrap` writes the store directly, run it before starting the daemon or while the database is idle.

The runtime CLI covers cloud-scoped claim/list/get flows for claimed runtime connectors and instances. Provider-action and runtime-execution flows still remain easier to script directly against the HTTP API.

## Current Truth

- Cloud runtime control is tenant-scoped over HTTP and cloud-scoped MCP in this pass.
- Repo tests now cover token revocation denial, event/audit scope filtering, runtime HTTP scope enforcement, and cloud MCP session/scope parity.
- Connector event history is append-only in the cloud store; repeated source-event IDs are preserved instead of overwriting older rows.
- Provider action logs now persist request/response payloads plus start/end timestamps, and provider audit rows include richer action outcome details.
- Raw relay routes are still available from `codencer-relayd` for self-host relay use, but they are not the cloud control-plane contract.
- Cloud runtime control requires `codencer-cloudd` to be started with `relay_config_path` or `--relay-config`.
- Cloud runtime connector ownership is explicit. A relay connector must still be claimed into org/workspace/project scope before the cloud API or cloud MCP can use it.
- Connector enrollment-token issuance remains relay-config backed in this pass. Cloud hosts connector ingress in composed mode, but it does not yet add a cloud-native enrollment-token lifecycle.
- Jira is polling-first in the current experimental provider path.
- Jira webhook ingest remains deferred and routed Jira webhook calls now return `501 webhook_deferred` instead of ingesting payloads.
- `codencer-cloudworkerd` is the place where Jira polling runs.
- `cloud_smoke.sh` now exercises the binary-native bootstrap, status, list, create, get, enable, disable, webhook ingest, events, and audit flows.
- the provider-shaped binary smoke path is Slack-oriented today; GitHub, GitLab, Jira, and Linear remain narrower operator/package surfaces backed mainly by provider fixture tests plus the generic cloud install/action routes
- In composed mode, `cloud_smoke.sh` can also prove claimed runtime visibility plus a real cloud runtime HTTP run/start + submit-task path. That composed proof can use either an existing shared connector id or a temporary connector enrolled from `CLOUD_RUNTIME_DAEMON_URL`.
- In composed mode, `cloud_smoke.sh` can optionally prove cloud MCP initialize/list/call and official Go SDK interoperability.
- `deploy/cloud/smoke.sh` exercises the Docker-based self-host stack baseline with bootstrap, status, installation create, and audit verification.
- `make cloud-stack-smoke` is the Docker baseline proof only; the broader runtime/MCP/SDK proof lives in the binary-native `make cloud-smoke` path with composed-mode inputs.

For operator steps and startup ordering, see [CLOUD_SELF_HOST.md](CLOUD_SELF_HOST.md). For provider capability details, see [CLOUD_CONNECTORS.md](CLOUD_CONNECTORS.md).
