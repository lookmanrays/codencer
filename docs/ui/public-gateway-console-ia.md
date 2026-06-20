# Public Gateway Console Information Architecture

The public Gateway Console is for Codencer Core/community/self-host use and
controlled pre-production official connector testing. It is not the private
managed Codencer Cloud console.

## Routes

- `/ui-system`: live design-system and component reference.
- `/console`: workspace dashboard with MCP endpoint, Relay, connector, project,
  audit, and activation summaries.
- `/console/relays`: default managed Relay and user-added self-host Relay
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

## Backend Integration Plan

Current foundation defaults to seeded mock data. Read-only Gateway integration is
scaffolded for Relay profiles behind:

```text
NEXT_PUBLIC_CODENCER_CONSOLE_MOCKS=false
NEXT_PUBLIC_CODENCER_GATEWAY_API_BASE=http://127.0.0.1:19090
```

Next phase should add a browser-safe session model, then wire read-only
workspace, machine, connector, project, and audit APIs before enabling write
flows.
