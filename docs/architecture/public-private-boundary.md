# Public/Private Boundary

This document records the public repository boundary for the open-source
release. It is based on the files currently present in this repository.

## Public Core

The public core contains:

- `cmd/codencer`: user CLI for local setup, project config, machine identity,
  Gateway login, connector login, activation, readiness, and verification.
- `cmd/orchestratord` and `cmd/orchestratorctl`: local daemon and legacy/local
  operator CLI.
- `internal/project`, `internal/projectconfig`, `internal/local`,
  `internal/localexec`, `internal/manifest`, `internal/supervisor`,
  `internal/validation`, and related packages.
- Project-local `.codencer/project.json` support. Only this project config is
  intended to be committed from a user repository.
- Local machine identity and host label support stored under `$CODENCER_HOME`.

## Public Self-Host Gateway/Relay/MCP

The public self-host remote bridge contains:

- `cmd/codencer-gatewayd` and `internal/gateway`: self-hostable Gateway,
  workspace store, device-code development login, Relay profile registry,
  OAuth development metadata, and Gateway MCP tools.
- `cmd/codencer-relayd` and `internal/relay`: self-host Relay, connector
  sessions, Relay MCP tools, structured blockers, audit, and safe project
  location output.
- `cmd/codencer-connectord`, `internal/connector`, and `internal/connectorops`:
  local outbound connector, explicit project sharing, enrollment, and connector
  config.
- `cmd/mcp-sdk-smoke`: deterministic MCP SDK proof helper.
- `web/gateway-console`: public Gateway Console UI foundation for self-host,
  community, and controlled pre-production official connector operation.
- Docs under `docs/mcp`, activation docs, Gateway/Relay quickstarts, and
  self-host references.

These components remain public because they are required for personal,
corporate, and community self-host deployments.

## Public Community Cloud-Control-Plane

The repository also contains existing self-host/community cloud-control-plane
code:

- `cmd/codencer-cloudctl`;
- `cmd/codencer-cloudd`;
- `cmd/codencer-cloudworkerd`;
- `internal/cloud`;
- `deploy/cloud`;
- `docs/CLOUD.md`, `docs/CLOUD_SELF_HOST.md`, and
  `docs/CLOUD_CONNECTORS.md`.

This code is generic self-host/community infrastructure. It is not the
production managed Codencer Cloud service, does not ship hosted UI or billing,
and must not be documented as hosted Codencer availability.

## Private Managed Service Candidates

These belong outside the public repository when they exist:

- production `mcp.codencer.dev` and `relay.codencer.dev` deployment configs;
- production OAuth/passwordless login providers;
- persistent production session infrastructure;
- OAuth client management and redirect URI allowlists;
- token revocation UI/API beyond generic self-host primitives;
- encrypted Relay credentials backed by KMS or Vault;
- production user/workspace database migrations for the managed service;
- cloud Relay provisioning;
- rate limits, quotas, metering, billing, and plans enforcement;
- team invite/admin UI;
- support/admin console and operational dashboards;
- abuse protection;
- marketplace/app submission credentials and materials;
- official connector credentials/secrets;
- hosted managed execution environments.

## Unsafe Or Ambiguous Public Surface

The following must not be committed publicly:

- real tokens, bearer credentials, OAuth client secrets, enrollment secrets, or
  private keys;
- non-example `.env` files;
- `$CODENCER_HOME` runtime state such as `session.json`, `machine.json`,
  `projects.json`, connector configs, logs, artifacts, proof bundles, SQLite
  databases, and generated runtime configs;
- production managed-service deployment configs;
- KMS/Vault/provider/billing credentials;
- absolute local machine paths from an operator workstation;
- unredacted `Authorization: Bearer ...` examples with real-looking values.

## Release Artifact Boundary

The primary release snapshot packages:

- `codencer`;
- `orchestratord`;
- `codencer-relayd`;
- `codencer-gatewayd`;
- `codencer-connectord`;
- optional `agent-broker` when available;
- README, license/legal/security files, docs, and install scripts.

Self-host cloud-control-plane binaries are built through `make build-cloud` or
`make build-self-host-cloud`; they are not included in the primary local/self-
host release archive unless a future release target explicitly says so.

## Verification

Run:

```bash
make verify-public-release
```

The verifier checks docs links, license/legal files, public/private boundary
docs, secret/config safety, release artifact safety, official-vs-self-host
positioning, and direct self-host MCP caveats.
