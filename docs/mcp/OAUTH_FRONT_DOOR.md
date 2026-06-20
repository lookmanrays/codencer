# OAuth Front Door For Remote MCP

> Direct Relay note: the public connector path is the operator's self-host
> Gateway. Use this front-door pattern only for advanced direct Relay
> deployments or operator-owned experiments.

Codencer supports OAuth-capable remote MCP as a resource server. It exposes protected-resource metadata and bearer challenges, but it does not issue OAuth authorization-code tokens.

Use this pattern when ChatGPT, Claude, or another product-facing remote MCP client expects an OAuth flow.

## Deployment Pattern

```text
ChatGPT / Claude / API MCP client
  -> Codencer Gateway /mcp
  -> selected self-host Relay
  -> connector
  -> local daemon
  -> local Codex
```

The front door is responsible for:

- OAuth/OIDC discovery and authorization-code flow
- PKCE and dynamic client registration if your product client requires it
- refresh-token/offline access policy
- access-token validation, introspection, or JWT verification
- mapping product-user authorization to a narrow Codencer bearer token
- forwarding only the canonical MCP path, not arbitrary tunnels

Codencer is responsible for:

- verifying the forwarded bearer token
- enforcing Relay scopes
- routing to shared/claimed instances
- executing submitted TaskSpec steps
- returning structured evidence

## Required Direct Relay Config

Relay example:

```json
{
  "public_base_url": "https://relay.example.com",
  "oauth_authorization_servers": ["https://auth.example.com"],
  "oauth_scopes_supported": [
    "instances:read",
    "runs:read",
    "runs:write",
    "steps:read",
    "steps:write",
    "artifacts:read",
    "gates:read",
    "gates:write"
  ],
  "oauth_resource_documentation": "https://docs.example.com/codencer-relay-mcp"
}
```

Experimental cloud-control-plane example:

```json
{
  "public_base_url": "https://cloud.example.com",
  "oauth_authorization_servers": ["https://auth.example.com"],
  "oauth_scopes_supported": [
    "runtime_instances:read",
    "runs:read",
    "runs:write",
    "steps:read",
    "steps:write",
    "artifacts:read",
    "gates:read",
    "gates:write"
  ],
  "oauth_resource_documentation": "https://docs.example.com/codencer-cloud-mcp"
}
```

## Front-Door Contract

The front door should expose these public URLs:

- Relay MCP: `https://relay.example.com/mcp`
- Relay metadata: `https://relay.example.com/.well-known/oauth-protected-resource/mcp`
- Cloud-control-plane MCP: `https://cloud.example.com/api/cloud/v1/mcp`
- Cloud metadata: `https://cloud.example.com/.well-known/oauth-protected-resource/api/cloud/v1/mcp`

On successful OAuth validation, forward to Codencer with:

```http
Authorization: Bearer <codencer-planner-or-cloud-token>
```

Recommended scope mapping for relay:

```json
{
  "codencer-planner": [
    "instances:read",
    "runs:read",
    "runs:write",
    "steps:read",
    "steps:write",
    "artifacts:read",
    "gates:read",
    "gates:write"
  ]
}
```

Recommended scope mapping for cloud:

```json
{
  "codencer-cloud-runtime": [
    "runtime_instances:read",
    "runs:read",
    "runs:write",
    "steps:read",
    "steps:write",
    "artifacts:read",
    "gates:read",
    "gates:write"
  ]
}
```

Do not forward arbitrary inbound user tokens directly unless Codencer is explicitly configured to accept those exact bearer tokens. The safer default is token translation at the front door.

## Validation Checklist

Before publishing a product connector:

```bash
curl -fsS https://relay.example.com/.well-known/oauth-protected-resource/mcp

curl -i -X POST https://relay.example.com/mcp \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":"auth","method":"initialize","params":{"protocolVersion":"2025-11-25"}}'
```

Expected unauthenticated behavior:

- HTTP `401`
- `WWW-Authenticate: Bearer ... resource_metadata="https://relay.example.com/.well-known/oauth-protected-resource/mcp"`

Expected authenticated behavior:

- `initialize` returns HTTP `200`
- response has `MCP-Session-Id`
- `tools/list` returns `codencer.*` tools

## Examples

Reference files:

- [examples/oauth-front-door.env.example](examples/oauth-front-door.env.example)
- [examples/oauth-front-door.mapping.json](examples/oauth-front-door.mapping.json)

These are deployment contracts, not a bundled OAuth server.
