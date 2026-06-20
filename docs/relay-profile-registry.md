# Relay Profile Registry

Gateway stores Relay profiles in its persistent workspace store. New personal
workspaces receive a default self-host Relay profile from Gateway config:

```bash
codencer setup self-host \
  --gateway-url http://127.0.0.1:19090 \
  --relay-url http://127.0.0.1:8090 \
  --default-relay-token-env CODENCER_DEFAULT_RELAY_TOKEN \
  --json
```

Users may add self-host Relays as backend profiles:

```bash
codencer gateway relay add \
  --gateway http://127.0.0.1:19090 \
  --name personal \
  --url https://relay.example.com \
  --token-env CODENCER_RELAY_PERSONAL_TOKEN \
  --json

codencer gateway relay list --gateway http://127.0.0.1:19090 --json
codencer gateway relay status personal --gateway http://127.0.0.1:19090 --json
codencer gateway relay remove personal --gateway http://127.0.0.1:19090 --json
```

Relay tokens stay server-side in the Gateway environment or token files. Gateway
MCP tools expose safe metadata such as profile id, URL, status, and whether a
token is configured; they do not expose literal backend Relay tokens to AI
clients.
