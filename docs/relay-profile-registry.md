# Relay Profile Registry

Gateway stores Relay profiles in its persistent workspace store. New personal
workspaces receive a default managed Relay profile from Gateway config:

```bash
codencer setup gateway \
  --base-url https://mcp.codencer.dev \
  --default-relay-url https://relay.codencer.dev \
  --default-relay-token-env CODENCER_DEFAULT_RELAY_TOKEN \
  --json
```

Users may add self-host Relays as backend profiles:

```bash
codencer gateway relay add \
  --gateway https://mcp.codencer.dev \
  --name personal \
  --url https://relay.example.com \
  --token-env CODENCER_RELAY_PERSONAL_TOKEN \
  --json

codencer gateway relay list --gateway https://mcp.codencer.dev --json
codencer gateway relay status personal --gateway https://mcp.codencer.dev --json
codencer gateway relay remove personal --gateway https://mcp.codencer.dev --json
```

Relay tokens stay server-side in the Gateway environment or token files. Gateway
MCP tools expose safe metadata such as profile id, URL, status, and whether a
token is configured; they do not expose literal backend Relay tokens to AI
clients.
