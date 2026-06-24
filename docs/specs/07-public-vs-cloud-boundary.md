# Public vs Cloud Boundary

The public repository is Codencer Core / Community / Self-host. It must remain
usable without Codencer commercial cloud.

## Public Repository

Public code may include:

- CLI;
- local daemon;
- local connector;
- self-host Relay;
- self-host Gateway;
- Gateway Console;
- MCP tools/protocol surfaces;
- release packaging;
- self-host runbooks;
- public verification gates.

## Private Managed Service

Private/commercial code belongs outside this repository:

- production hosted Gateway/Cloud deployment;
- official production provider auth;
- billing, quotas, plans, usage metering;
- managed runners;
- support/admin console;
- KMS/Vault credential storage;
- official connector private credentials;
- marketplace submission secrets.

## Documentation Rules

Docs must distinguish:

- verified self-host behavior;
- future managed Codencer Gateway/Cloud;
- official connector path;
- direct self-host MCP path;
- live product proof vs protocol/setup proof.

Public docs must not claim live ChatGPT, Claude Code, Codex, or Antigravity
product proof unless evidence exists in current verification output.

## Endpoint Rules

Public/self-built defaults must be self-host/local. Official hosted endpoints
may appear only as future/official-build examples or clearly documented private
service boundaries.
