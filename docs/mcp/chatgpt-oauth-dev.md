# ChatGPT OAuth Dev Mode

Sprint 7 adds a minimal single-user OAuth dev front-door to `codencer-relayd` for self-host testing. It is not enterprise IAM and does not implement refresh tokens.

## Enable

```bash
./bin/codencer setup relay \
  --base-url https://relay.example.com \
  --mcp-url https://relay.example.com/mcp \
  --generate-planner-token \
  --enable-chatgpt-oauth-dev \
  --json
```

Generated secrets are written only under `$CODENCER_HOME/tokens` with `0600` permissions:

- `chatgpt-oauth-client-secret`
- `chatgpt-oauth-operator-code`

The relay config stores only hashes, issuer/client metadata, scopes, and TTLs.

## Endpoints

- `GET /.well-known/oauth-authorization-server`
- `GET /.well-known/openid-configuration`
- `GET|POST /oauth/authorize`
- `POST /oauth/token`

The flow uses authorization code plus PKCE S256. Access tokens are opaque, hashed in memory, scoped, TTL-bound, and audience/resource-bound to `/mcp`. Refresh tokens are not implemented.

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
