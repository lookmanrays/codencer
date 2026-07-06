# Official Connector Flow

The official connector path is:

```text
AI client -> https://mcp.codencer.dev/mcp -> Codencer Gateway
  -> default official Relay or user self-host Relay profile
  -> local connector -> local daemon -> project
```

Direct self-host Relay `/mcp` remains available for advanced/debug use, but it
is not the primary ChatGPT, Claude Code, or Codex connector path.

## Local Setup

```bash
codencer login --gateway https://mcp.codencer.dev
codencer connector login --gateway https://mcp.codencer.dev --relay default --json
codencer project init --id codencer --repo . --json
codencer project share codencer --json
codencer connector run --config "$CODENCER_HOME/runtime/connector/config.json"
```

`codencer login` stores the Gateway session in `$CODENCER_HOME/session.json`.
`codencer connector login` binds the local machine and connector to the
workspace, requests a short-lived Relay enrollment secret through Gateway, writes
local connector config, and does not print the secret or private key.

## Optional Self-Host Relay

```bash
codencer gateway relay add \
  --gateway https://mcp.codencer.dev \
  --name personal \
  --url https://relay.example.com \
  --token-env CODENCER_RELAY_PERSONAL_TOKEN \
  --json

codencer connector login --gateway https://mcp.codencer.dev --relay personal --json
```

AI clients still point to `https://mcp.codencer.dev/mcp`. Gateway selects the
backend Relay profile.

## Verification

```bash
make verify-official-connector
```

The verifier starts isolated temp Gateway, official Relay, self-host Relay,
daemon, connectors, and project on random loopback ports. It checks login,
connector login, Relay profile add, MCP initialize/tools/list, Relay/project
listing, manifest execution through default and self-host profiles, run report,
relay-down blocker, ambiguity blockers, token redaction, and no absolute local
path leakage in MCP output.
