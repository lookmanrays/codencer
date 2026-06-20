# Public Gateway Console Information Architecture

The public Gateway Console is for Codencer Core/community/self-host use and
controlled pre-production official connector testing. It is not the private
managed Codencer Cloud console.

## Routes

- `/ui-system`: live design-system and component reference.
- `/console`: workspace dashboard with MCP endpoint, Relay, connector, project,
  audit, and activation summaries.
- `/console/relays`: default self-host Relay and user-added self-host Relay
  backend profiles.
- `/console/connectors`: machines, host labels, connector bindings, and login
  command.
- `/console/projects`: projects and safe project-location metadata.
- `/console/activation`: Gateway-first CLI and MCP setup commands.
- `/console/audit`: recent events with type filter.
- `/console/settings`: workspace metadata, endpoints, theme, and future token
  revocation placeholder.
- `/device`: polished device-code approval page.
- `/oauth/authorize`: polished OAuth dev consent page.

## Product Boundary

Allowed in public console:

- self-host Gateway;
- self-host Relay backend profiles;
- explicit local connector and project sharing;
- MCP activation commands;
- safe machine/project location metadata;
- development/pre-production OAuth and device approval UI.

Not implemented here:

- billing and plans;
- team invites;
- support/admin console;
- production provider login;
- KMS/Vault credential management;
- marketplace credentials;
- managed execution environments.

## Backend Integration Status

The console defaults to live mode over `codencer-gatewayd` through the
server-side Next proxy. Demo data is explicit and only enabled with:

```text
NEXT_PUBLIC_CODENCER_CONSOLE_MODE=demo
```

Live mode reads workspace, relay, relay health, machine, connector, project,
activation, audit, device, and OAuth data through Gateway APIs. Relay profile
add/remove, device approval, and OAuth consent are live Gateway mutations.
