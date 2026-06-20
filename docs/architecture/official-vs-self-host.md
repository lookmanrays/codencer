# Official Vs Self-Host Codencer

Codencer supports two deployment stories that share the same bridge doctrine:
planners decide, Codencer routes approved work, local executors run it, and
Codencer returns structured state, evidence, or blockers.

## Official Connector Path

The official connector path is Gateway-first:

```text
AI client -> https://mcp.codencer.dev/mcp
  -> official Codencer Gateway
  -> default managed Relay or user-added self-host Relay profile
  -> local connector
  -> local daemon
  -> project
```

AI clients should keep using the official Gateway MCP URL. Users can attach a
self-host Relay as a backend Relay profile inside Gateway, but the AI client
still talks to Gateway. Gateway owns the official connector service identity,
workspace authorization, Relay-profile routing, and token redaction boundary.

The public repository includes `codencer-gatewayd`, which is the self-hostable
Gateway implementation and the deterministic pre-production proof surface. The
production deployment at `mcp.codencer.dev` belongs to the separate managed
Codencer service.

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
not the official hosted Codencer service unless operated by Codencer.

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

Use this for advanced self-host, corporate internal, and debug deployments. It
is not the primary official connector path for ChatGPT, Claude Code, or Codex.

## Official Managed Relay Vs Self-Host Relay

Gateway can route to:

- a default managed Relay profile for the official service; or
- user-added self-host Relay profiles.

Relay tokens are resolved server-side. AI clients must not receive backend Relay
tokens, local connector private keys, local absolute paths, daemon URLs, or
runtime state.

## Future Managed Cloud

The private managed Codencer Cloud service may later provide production OAuth,
hosted sessions, encrypted Relay credentials, rate limits, billing, hosted UI,
team/admin workflows, operational dashboards, abuse protection, and managed
execution environments. Those are not implemented in this public release task.
