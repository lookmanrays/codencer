# Codencer Cloud Self-Host Guide

This guide covers the practical self-host bootstrap path for Codencer Cloud.

## Recommended Topology

- `codencer-cloudd` on a server, VPS, or local host
- `codencer-cloudctl` on the operator machine
- `codencer-cloudworkerd` alongside the cloud daemon or as a scheduled worker
- optional internal relay bridge under `codencer-cloudd` if you want cloud to own tenant-scoped Codencer runtime control

The cloud control plane still does not execute coding work. In this pass it can also claim runtime connectors and shared instances into org/workspace/project scope, but the daemon and connector still execute and report locally.

## Build

Build the cloud binaries with:

```bash
make build-cloud
```

This produces:

- `bin/codencer-cloudctl`
- `bin/codencer-cloudd`
- `bin/codencer-cloudworkerd`

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

## Operator Commands

Use the bearer token from bootstrap with the cloud control-plane CLI:

```bash
./bin/codencer-cloudctl status --cloud-url http://127.0.0.1:8190 --token <token> --json
curl -fsS -H "Authorization: Bearer <token>" http://127.0.0.1:8190/api/cloud/v1/orgs
curl -fsS -H "Authorization: Bearer <token>" "http://127.0.0.1:8190/api/cloud/v1/workspaces?org_id=<org-id>"
curl -fsS -H "Authorization: Bearer <token>" "http://127.0.0.1:8190/api/cloud/v1/projects?workspace_id=<workspace-id>"
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
  -d '{"adapter":"sim","title":"Smoke run"}' \
  http://127.0.0.1:8190/api/cloud/v1/runtime/instances/<instance-id>/runs
```

Runtime steps, gates, logs, validations, and artifact content follow the same instance-scoped prefix under `/api/cloud/v1/runtime/instances/<instance-id>/...`.

## Worker

`codencer-cloudworkerd` is the background worker for connector maintenance. In this alpha pass:

- GitHub, GitLab, Linear, and Slack remain webhook-first
- Jira is polling-first
- Jira webhook ingest is intentionally not implemented

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
- audit inspection
- a safe no-op `cloudworkerd --once` pass
- optional runtime claim/list assertions when `CLOUD_RELAY_CONFIG` and `CLOUD_RUNTIME_CONNECTOR_ID` are supplied

It does not claim external provider verification.

## Troubleshooting

- If `bootstrap` or `status` fail, confirm the cloud server is using the same `db_path` as your config.
- If secret storage fails, confirm `master_key` is set.
- If a connector install remains `disabled`, check the enable route and the audit trail.
- If runtime connector claim fails, confirm the relay bridge is configured and that the target connector id already exists in relay state.
- If a runtime instance does not appear, confirm it is still shared by the local Codencer connector.
- If Jira polling fails, confirm `config.jql` or `config.project_key` is present and that the provider credentials are valid.

For connector capability details, see [CLOUD_CONNECTORS.md](CLOUD_CONNECTORS.md). For the high-level cloud overview, see [CLOUD.md](CLOUD.md).
