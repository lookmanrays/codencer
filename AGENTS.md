# Codencer Agent Notes

This repo follows the global agent working agreement. This file adds only
Codencer-specific facts and gates.

## Project

Codencer is a local/self-host bridge between AI planners and coding executors.
It is a bridge, not a planner.

Languages:

- Go
- TypeScript / React

## Public Repository Scope

Public Codencer includes:

- CLI
- local daemon
- local connector
- Relay
- Gateway
- MCP tools/protocol surfaces
- public Gateway Console
- self-host/community cloud-control-plane primitives

## Private Managed Service Boundary

Do not implement or commit private managed-service material here:

- production `mcp.codencer.dev` or `relay.codencer.dev` deployment configs
- production auth/provider/passwordless login
- billing, plans, or quotas
- hosted console commercial features
- KMS/Vault credential storage
- official connector credentials/secrets
- managed runners or managed execution environments
- support/admin console
- marketplace submission credentials

## Source Of Truth

Use these files first when facts conflict:

- `README.md`
- `docs/architecture/public-private-boundary.md`
- `docs/architecture/official-vs-self-host.md`
- `docs/official-connector-flow.md`
- `docs/relay-profile-registry.md`
- `docs/ui/*`

## Gotchas

- Official connector clients point to Gateway, normally
  `https://mcp.codencer.dev/mcp`.
- Self-host Relay is a backend profile for the official connector path.
- Direct self-host Relay MCP is advanced/personal/corporate/debug mode.
- `NEXT_PUBLIC_*` values are build-time browser values; never put secrets in
  them.
- Do not render raw tokens, Relay bearer tokens, enrollment secrets, connector
  private keys, or absolute local filesystem paths in UI.
- Use Radix + Tailwind + the Codencer design system, not default shadcn visual
  styling.

## Validation Gates

Go gates:

- `go test ./...`
- `make verify-official-connector`
- `make verify-gateway`
- `make verify-public-release`

Gateway Console UI gates:

- `cd web/gateway-console && npm ci`
- `cd web/gateway-console && npm run lint`
- `cd web/gateway-console && npm run typecheck`
- `cd web/gateway-console && npm run test`
- `cd web/gateway-console && npm run build`
- `cd web/gateway-console && npm run test:e2e`
- `make verify-gateway-console`

Always run `git diff --check` before final reporting.
