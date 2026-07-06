# Official Vs Self-Host Codencer

Codencer supports two deployment stories that share the same bridge doctrine:
planners decide, Codencer routes approved work, local executors run it, and
Codencer returns structured state, evidence, or blockers.

## Public Self-Host Connector Path

The public release path is Gateway-first and self-hosted:

```text
AI client -> self-host Gateway /mcp
  -> default self-host Relay or user-added Relay profile
  -> local connector
  -> local daemon
  -> project
```

AI clients should use Gateway MCP rather than talking directly to a backend
Relay for the standard public path. Gateway owns workspace authorization,
Relay-profile routing, and token redaction boundaries.

The public repository includes `codencer-gatewayd`, which is the self-hostable
Gateway implementation and the deterministic proof surface. Public/self-built
binaries default to self-host/local endpoints. Future private official builds
may override build-time defaults to Codencer-operated domains.

## Self-Host Gateway/Relay Path

Personal, corporate, and internal deployments can self-host:

- `codencer-gatewayd`;
- `codencer-relayd`;
- `codencer-connectord`;
- the public/self-host Gateway Console in `web/gateway-console`;
- local `orchestratord`;
- local or corporate MCP clients.

This mode is appropriate when the operator owns the Gateway/Relay endpoint,
auth policy, DNS, TLS, and runtime credentials. It is still Codencer Core; it is
not an official hosted Codencer service unless operated by Codencer.

The public/self-host Gateway Console relies on server-side Gateway token
proxying and is intended for localhost, internal network, private network, or
controlled self-host environments unless protected by external auth. Do not
expose it directly to the public internet without one of:

- reverse proxy authentication;
- VPN;
- Zero Trust access;
- private network only;
- production auth layer from the future private Codencer Cloud service.

## Direct Relay MCP

Direct Relay MCP remains supported:

```text
AI client -> user Relay /mcp -> local connector -> daemon -> project
```

Use this for advanced self-host, corporate internal, and debug deployments. The
standard public client path is Gateway MCP.

## Relay Profiles

Gateway can route to:

- a default self-host Relay profile; or
- user-added Relay profiles.

Relay tokens are resolved server-side. AI clients must not receive backend Relay
tokens, local connector private keys, local absolute paths, daemon URLs, or
runtime state.

## Future Official Managed Service

Future private official builds may set build-time defaults to
`mcp.codencer.dev`, `relay.codencer.dev`, and `app.codencer.dev`. The private
managed Codencer service may later provide production OAuth, hosted sessions,
encrypted Relay credentials, rate limits, billing, hosted UI, team/admin
workflows, operational dashboards, abuse protection, and managed execution
environments. Those are not implemented in this public self-host release.
