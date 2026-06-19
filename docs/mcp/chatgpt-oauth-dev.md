# ChatGPT OAuth Dev Mode

Codencer provides a minimal single-user OAuth dev front-door for ChatGPT
Developer Mode testing. The official connector path uses `codencer-gatewayd`;
Relay OAuth dev remains available for direct self-host testing. This is not
enterprise IAM and does not implement refresh tokens.

## Enable

```bash
./bin/codencer setup gateway \
  --base-url https://mcp.codencer.dev \
  --mcp-url https://mcp.codencer.dev/mcp \
  --token-env CODENCER_GATEWAY_MCP_TOKEN \
  --enable-oauth-dev \
  --json
```

Generated secrets are written only under `$CODENCER_HOME/tokens` with `0600` permissions:

- `gateway-oauth-client-secret`
- `gateway-oauth-operator-code`

The Gateway config stores only hashes, issuer/client metadata, scopes, and TTLs.

OAuth dev accepts syntactically valid redirect URIs so an operator can test ChatGPT/custom-MCP setup without running a full IdP. Production deployments must use redirect allowlisting or an external IdP/front door. This dev issuer is not public multi-user production.

## Endpoints

- `GET /.well-known/oauth-authorization-server`
- `GET /.well-known/openid-configuration`
- `GET|POST /oauth/authorize`
- `POST /oauth/token`

The flow uses authorization code plus PKCE S256. Access tokens are opaque, hashed in memory, scoped, TTL-bound, and audience/resource-bound to `/mcp`. Refresh tokens are not implemented.

For direct Relay OAuth dev mode, use `codencer setup relay
--enable-chatgpt-oauth-dev`. That path is for advanced/direct/debug testing, not
the official connector endpoint.

## Dev No-Auth

```bash
./bin/codencer setup relay \
  --base-url https://relay.example.com \
  --generate-planner-token \
  --chatgpt-dev-noauth \
  --json
```

This is explicit dev-only mode. By default it is read-only and limited to fake/test project ids. Real project write tools require:

```bash
--allow-real-projects-in-dev-noauth
```

Do not use real-project dev no-auth on a public relay.
