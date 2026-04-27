# Codencer Self-Host Cloud Control Plane Guide

This guide covers the practical self-host bootstrap path for Codencer Cloud.

Use this page when cloud tenancy is the public control plane.

- Use [SELF_HOST_REFERENCE.md](SELF_HOST_REFERENCE.md) when you want the raw relay/runtime self-host path instead of cloud tenancy.
- Use [CLOUD.md](CLOUD.md) for the cloud route/scope reference.
- Use [CLOUD_CONNECTORS.md](CLOUD_CONNECTORS.md) for per-provider install/test depth and limitations.
- Use [mcp/integrations.md](mcp/integrations.md) for the planner/client compatibility matrix.

## Recommended Topology

- `codencer-cloudd` on a server, VPS, or local host
- `codencer-cloudctl` on the operator machine
- `codencer-cloudworkerd` alongside the cloud daemon or as a scheduled worker
- optional internal relay bridge under `codencer-cloudd` if you want cloud to own tenant-scoped Codencer runtime control

The cloud control plane still does not execute coding work. In this pass it can also claim runtime connectors and shared instances into org/workspace/project scope, but the daemon and connector still execute and report locally.

## Docker Baseline

The repo now includes a practical Docker baseline under `deploy/cloud/`:

- `deploy/cloud/Dockerfile`
- `deploy/cloud/docker-compose.yml`
- `deploy/cloud/.env.example`
- `deploy/cloud/config/cloud.json`
- `deploy/cloud/config/relay.json`
- `deploy/cloud/smoke.sh`

This stack is beta-track, SQLite-backed, and meant to be a serious self-host baseline rather than a production-ready managed deployment recipe.

## Compose Reality

The Docker baseline is intentionally narrow:

- it starts `codencer-cloudd` plus `codencer-cloudworkerd`
- it embeds the relay bridge inside the cloud process
- it publishes only the cloud HTTP port on `8190`
- it does **not** create a usable runtime instance by itself
- cloud-scoped runtime proof still requires an external `orchestratord` plus `codencer-connectord`

Proof boundary:

- `make cloud-stack-smoke` proves the Docker baseline only
- `make cloud-smoke` proves the binary-native cloud control-plane path
- `make cloud-smoke` with composed-mode runtime inputs proves claimed runtime HTTP, cloud MCP, and official Go SDK access

## Build

Build the cloud binaries with:

```bash
make build-cloud
```

This produces:

- `bin/codencer-cloudctl`
- `bin/codencer-cloudd`
- `bin/codencer-cloudworkerd`

## Docker Compose Quickstart

1. Copy the env file and set the required secrets:

```bash
cp deploy/cloud/.env.example deploy/cloud/.env
```

Set at least:

- `CODENCER_CLOUD_MASTER_KEY`
- `RELAY_PLANNER_TOKEN`
- `RELAY_ENROLLMENT_SECRET`

2. Bootstrap the SQLite store before starting the cloud service:

```bash
docker compose --env-file deploy/cloud/.env -f deploy/cloud/docker-compose.yml run --rm \
  --entrypoint codencer-cloudctl cloud \
  bootstrap \
  --config /etc/codencer/cloud/config.json \
  --org-slug acme \
  --workspace-slug platform \
  --project-slug core \
  --token-name operator \
  --member-name "Bootstrap Owner" \
  --member-email owner@example.com \
  --json
```

3. Start the cloud daemon and worker:

```bash
docker compose --env-file deploy/cloud/.env -f deploy/cloud/docker-compose.yml up -d cloud worker
```

4. Check health:

```bash
curl -fsS http://127.0.0.1:8190/healthz
```

5. Optional deployment smoke:

```bash
make cloud-stack-smoke
```

Persistent state in the compose baseline lives in named volumes:

- `cloud-data` for the cloud SQLite database
- `relay-data` for the composed relay SQLite database

The committed JSON config files are templates. Secrets still come from the compose env file through runtime environment overrides.

## Composed Runtime Proof With An External Daemon

When you want tenant-scoped runtime control against the cloud host, use this order:

1. bootstrap and start the cloud stack
2. start a local `orchestratord` next to the repo you want to serve
3. mint a relay enrollment token from the cloud image:

```bash
docker compose --env-file deploy/cloud/.env -f deploy/cloud/docker-compose.yml run --rm \
  --entrypoint codencer-relayd cloud \
  enrollment-token create \
  --config /etc/codencer/relay/config.json \
  --label local-dev \
  --json
```

4. enroll and run `codencer-connectord` against `http://127.0.0.1:8190`
5. claim the runtime connector into org/workspace/project scope with `codencer-cloudctl runtime-connectors claim`
6. use cloud HTTP under `/api/cloud/v1/runtime/...` or cloud MCP under `/api/cloud/v1/mcp`

If you want the scripted composed proof from a local checkout instead of the Docker baseline, use the binary-native smoke path:

```bash
make build-cloud build-mcp-sdk-smoke
CLOUD_RELAY_CONFIG=.codencer/relay/config.json \
CLOUD_RUNTIME_DAEMON_URL=http://127.0.0.1:8085 \
CLOUD_SMOKE_MCP=1 \
CLOUD_SMOKE_SDK=1 \
make cloud-smoke
```

## Cloud Config

Create a cloud config file such as `.codencer/cloud/config.json`:

```json
{
  "host": "127.0.0.1",
  "port": 8190,
  "db_path": ".codencer/cloud/cloud.db",
  "master_key": "replace-with-a-long-random-secret",
  "relay_config_path": ".codencer/relay/config.json"
}
```

Notes:

- `master_key` is required if you want encrypted installation secrets.
- `relay_config_path` is optional and only needed if you want `codencer-cloudd` to own cloud-scoped runtime control through an internal relay bridge.
- If you use the environment variables `CODENCER_CLOUD_DB_PATH`, `CODENCER_CLOUD_HOST`, `CODENCER_CLOUD_PORT`, `CODENCER_CLOUD_MASTER_KEY`, or `CODENCER_CLOUD_RELAY_CONFIG`, they override the file values.

## Bootstrap Order

Because `codencer-cloudctl bootstrap` writes directly to the SQLite store, run it before starting the daemon or while the database is idle.

```bash
./bin/codencer-cloudctl bootstrap \
  --config .codencer/cloud/config.json \
  --org-slug acme \
  --workspace-slug platform \
  --project-slug core \
  --token-name operator \
  --json
```

The bootstrap response includes:

- `org`
- `workspace`
- `project`
- `membership`
- a raw bearer token string
- the persisted token record

## Start The Cloud Daemon

Standalone cloud:

```bash
./bin/codencer-cloudd --config .codencer/cloud/config.json
```

Cloud plus relay composition:

```bash
./bin/codencer-cloudd --config .codencer/cloud/config.json --relay-config .codencer/relay/config.json
```

In composed mode, use the cloud API for tenant-scoped runtime control. Do not treat raw relay routes as the cloud contract.

Cloud-scoped MCP is also available in composed mode:

- canonical cloud MCP endpoint: `/api/cloud/v1/mcp`
- compatibility alias: `/api/cloud/v1/mcp/call`

Use relay `/mcp` only when you are operating the self-host relay directly without cloud tenancy.

For the frozen planner/client compatibility matrix, generic client examples, and cloud-vs-relay boundary, see [mcp/integrations.md](mcp/integrations.md) and [mcp/cloud_tools.md](mcp/cloud_tools.md).

## Operator Commands

Use the bearer token from bootstrap with the cloud control-plane CLI:

```bash
./bin/codencer-cloudctl status --cloud-url http://127.0.0.1:8190 --token <token> --json
curl -fsS -H "Authorization: Bearer <token>" http://127.0.0.1:8190/api/cloud/v1/orgs
curl -fsS -H "Authorization: Bearer <token>" "http://127.0.0.1:8190/api/cloud/v1/workspaces?org_id=<org-id>"
curl -fsS -H "Authorization: Bearer <token>" "http://127.0.0.1:8190/api/cloud/v1/projects?workspace_id=<workspace-id>"
curl -fsS -H "Authorization: Bearer <token>" "http://127.0.0.1:8190/api/cloud/v1/memberships?org_id=<org-id>"
curl -fsS -H "Authorization: Bearer <token>" "http://127.0.0.1:8190/api/cloud/v1/tokens?org_id=<org-id>"
curl -fsS -H "Authorization: Bearer <token>" "http://127.0.0.1:8190/api/cloud/v1/installations?org_id=<org-id>"
./bin/codencer-cloudctl events --cloud-url http://127.0.0.1:8190 --token <token> --json
./bin/codencer-cloudctl audit --cloud-url http://127.0.0.1:8190 --token <token> --json
```

Create a connector installation:

```bash
./bin/codencer-cloudctl install create \
  --cloud-url http://127.0.0.1:8190 \
  --token <token> \
  --org-id <org-id> \
  --workspace-id <workspace-id> \
  --project-id <project-id> \
  --connector slack \
  --name "Slack smoke" \
  --config api_base_url=http://127.0.0.1:9 \
  --secret token=smoke-token \
  --secret webhook_secret=smoke-secret
```

Then toggle the installation explicitly:

```bash
./bin/codencer-cloudctl install disable --cloud-url http://127.0.0.1:8190 --token <token> --installation-id <installation-id>
./bin/codencer-cloudctl install enable --cloud-url http://127.0.0.1:8190 --token <token> --installation-id <installation-id>
```

Installation records now expose:

- `owner_membership_id`
- `health`
- `last_validated_at`
- `last_webhook_at`
- `last_action_at`
- `last_sync_at`
- `last_error`

## Claim Codencer Runtime Into Cloud Scope

When `codencer-cloudd` has a relay bridge configured and the relay already knows about a local Codencer connector, claim that runtime connector into tenant scope:

```bash
./bin/codencer-cloudctl runtime-connectors claim \
  --cloud-url http://127.0.0.1:8190 \
  --token <token> \
  --org-id <org-id> \
  --workspace-id <workspace-id> \
  --project-id <project-id> \
  --connector-id <relay-connector-id> \
  --json
```

Then inspect the claimed runtime connector and its shared instances:

```bash
./bin/codencer-cloudctl runtime-connectors list --cloud-url http://127.0.0.1:8190 --token <token> --org-id <org-id> --json
./bin/codencer-cloudctl runtime-connectors instances --cloud-url http://127.0.0.1:8190 --token <token> --runtime-connector-id <runtime-connector-record-id> --json
./bin/codencer-cloudctl runtime-instances list --cloud-url http://127.0.0.1:8190 --token <token> --org-id <org-id> --json
```

You can also use the cloud HTTP surface directly for runtime work:

```bash
curl -fsS \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"id":"cloud-runtime-smoke","project_id":"cloud-smoke-project"}' \
  http://127.0.0.1:8190/api/cloud/v1/runtime/instances/<instance-id>/runs
```

Then submit a task through the same cloud-scoped prefix:

```bash
curl -fsS \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"version":"v1","goal":"Verify the cloud runtime HTTP path","adapter_profile":"codex"}' \
  http://127.0.0.1:8190/api/cloud/v1/runtime/instances/<instance-id>/runs/cloud-runtime-smoke/steps
```

Runtime steps, gates, logs, validations, and artifact content follow the same instance-scoped prefix under `/api/cloud/v1/runtime/instances/<instance-id>/...`.

The same tenant-scoped runtime access is also available through cloud MCP once the runtime bridge is active:

```bash
make build-mcp-sdk-smoke
./bin/mcp-sdk-smoke --endpoint http://127.0.0.1:8190/api/cloud/v1/mcp --token <token> --instance-id <instance-id>
```

## Connector Ingress In Composed Mode

When `codencer-cloudd` is started with the relay bridge, the cloud host itself accepts local Codencer connector ingress:

- `POST /api/v2/connectors/enroll`
- `POST /api/v2/connectors/challenge`
- `GET /ws/connectors`

That means a local Codencer connector can point its relay URL at the cloud host in composed mode.

Current limitation:

- enrollment-token creation is still relay-config backed and is not yet a cloud-native API lifecycle

For docker-compose based operators, the same image includes the relay admin CLI, so you can mint an enrollment token with:

```bash
docker compose --env-file deploy/cloud/.env -f deploy/cloud/docker-compose.yml run --rm \
  --entrypoint codencer-relayd cloud \
  enrollment-token create \
  --config /etc/codencer/relay/config.json \
  --label local-dev \
  --json
```

## Worker

`codencer-cloudworkerd` is the background worker for connector maintenance. In the current beta track:

- GitHub, GitLab, Linear, and Slack remain webhook-first
- Jira is polling-first
- Jira webhook ingest is intentionally deferred
- routed Jira webhook calls return `501 webhook_deferred` and do not persist events

Safe worker run:

```bash
./bin/codencer-cloudworkerd --config .codencer/cloud/config.json --once
```

For a live Jira installation, provide:

- `config.username`
- `config.api_base_url`
- either `config.jql` or `config.project_key`
- installation secret `token`

## Cloud Smoke

The repo includes `scripts/cloud_smoke.sh` and a `make cloud-smoke` target. The smoke script exercises:

- bootstrap
- status
- org/workspace/project listing via the HTTP API
- installation creation/list/get
- installation enable/disable
- Slack-style webhook verification for the smoke installation
- event listing for the smoke installation
- audit inspection
- a safe no-op `cloudworkerd --once` pass
- optional composed-mode runtime claim/list assertions when `CLOUD_RELAY_CONFIG` is supplied together with either `CLOUD_RUNTIME_CONNECTOR_ID` or `CLOUD_RUNTIME_DAEMON_URL`
- optional composed-mode cloud runtime HTTP proof under the same composed-mode runtime inputs
- optional composed-mode cloud MCP initialize/list/call proof when `CLOUD_SMOKE_MCP=1`
- optional composed-mode official Go SDK proof against `/api/cloud/v1/mcp` when `CLOUD_SMOKE_SDK=1`

It does not claim external provider verification.

Provider truth for this smoke:

- Slack is the current provider-shaped binary smoke path
- Jira polling is proven by focused tests and `codencer-cloudworkerd --once`, not by the baseline binary smoke
- GitHub, GitLab, and Linear remain provider-fixture and routed-install/action proof paths, not provider-specific binary smoke paths

Practical split:

- `make cloud-smoke` proves the baseline cloud control-plane path.
- `CLOUD_RELAY_CONFIG=... CLOUD_RUNTIME_CONNECTOR_ID=... make cloud-smoke` adds composed-mode claimed-runtime and cloud runtime HTTP proof.
- `CLOUD_RELAY_CONFIG=... CLOUD_RUNTIME_DAEMON_URL=http://127.0.0.1:8080 make cloud-smoke` can bootstrap a temporary connector automatically for the composed proof when you do not already have a shared connector id.
- `make build-mcp-sdk-smoke` plus `CLOUD_SMOKE_MCP=1 CLOUD_SMOKE_SDK=1` adds cloud MCP and official Go SDK proof in the same composed-mode smoke run.

Example composed-mode proof:

```bash
make build-cloud build-mcp-sdk-smoke
CLOUD_RELAY_CONFIG=.codencer/relay/config.json \
CLOUD_RUNTIME_DAEMON_URL=http://127.0.0.1:8080 \
CLOUD_SMOKE_MCP=1 \
CLOUD_SMOKE_SDK=1 \
make cloud-smoke
```

For the Docker-based deployment baseline, use:

```bash
make cloud-stack-smoke
```

That compose smoke verifies:

- image build
- bootstrap through the mounted config and SQLite volume
- cloud health on the published port
- installation create
- audit visibility

It requires a running Docker daemon. In environments where Docker CLI is installed but the daemon/socket is unavailable, use `docker compose ... config` plus the binary-native `make cloud-smoke` path instead.

## Troubleshooting

- If `bootstrap` or `status` fail, confirm the cloud server is using the same `db_path` as your config.
- If secret storage fails, confirm `master_key` is set.
- If a connector install remains `disabled`, check the enable route and the audit trail.
- If a Jira webhook call returns `webhook_deferred`, that is expected in this phase; use `codencer-cloudworkerd` for Jira ingest.
- If runtime connector claim fails, confirm the relay bridge is configured and either provide a valid shared `CLOUD_RUNTIME_CONNECTOR_ID` or set `CLOUD_RUNTIME_DAEMON_URL` so the smoke can enroll a temporary connector first.
- If a runtime instance does not appear, confirm it is still shared by the local Codencer connector.
- If cloud MCP calls fail, confirm the cloud daemon was started with `relay_config_path` or `--relay-config`, that the token is valid for the target tenant scope, and that the token includes the tool-specific scopes you are calling. `list_instances` and `get_instance` still require `runtime_instances:read`.
- If Jira polling fails, confirm `config.jql` or `config.project_key` is present and that the provider credentials are valid.
- If connector event history shows repeated source IDs, that is now expected append-only behavior rather than a silent overwrite.

For connector capability details, see [CLOUD_CONNECTORS.md](CLOUD_CONNECTORS.md). For the high-level cloud overview, see [CLOUD.md](CLOUD.md).
