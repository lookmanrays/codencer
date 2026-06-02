# Self-Host Relay/MCP Quickstart

Self-host Relay is the remote planner transport. Execution remains local and daemon-first. Projects are exposed only after explicit `project share`; the relay is not a raw shell or arbitrary filesystem browser.

## Build

```bash
make build
```

## Create Relay And Connector Config

```bash
./bin/codencer setup relay \
  --base-url https://relay.example.com \
  --mcp-url https://relay.example.com/mcp \
  --generate-planner-token \
  --json
```

The generated planner token is written under `$CODENCER_HOME/tokens` with `0600` permissions and redacted from command output. Use your own reverse proxy or native TLS config for public HTTPS. Codencer does not issue OAuth authorization-code tokens; ChatGPT-style product setup needs an operator-owned OAuth front door when the product requires OAuth.

## Share A Project

```bash
./bin/codencer setup local --project-id codencer --repo . --adapter codex --profile codex-workspace --json
./bin/codencer project share codencer --json
```

Remote project descriptors use safe labels/hashes instead of absolute local paths. The local registry keeps full paths for local execution.

## MCP Client Snippets

```bash
./bin/codencer setup mcp --client codex --endpoint https://relay.example.com/mcp --token-env CODENCER_PLANNER_TOKEN --json
./bin/codencer setup mcp --client claude-code --endpoint https://relay.example.com/mcp --token-env CODENCER_PLANNER_TOKEN --json
./bin/codencer setup mcp --client chatgpt --endpoint https://relay.example.com/mcp --json
```

These commands generate snippets only. They do not write user-level Codex, Claude, or ChatGPT configuration.

## Verify

```bash
make verify-local-relay-mcp
CODENCER_LIVE_RELAY_MCP=1 ./bin/codencer live relay-mcp --json --bin-dir ./bin --repo .
./bin/codencer accept local-production --profile relay --json --bin-dir ./bin --repo .
```

Live Codex/Claude executor proof and ChatGPT product UI proof remain pending unless explicitly run in the target product environment.

## Platform Notes

Self-host Linux and WSL2 deployments use the `linux/amd64` release artifact or a source build performed on Linux/WSL. Windows-native daemon binaries are not part of the Sprint 6.1 production claim. ChatGPT product UI proof remains manual and pending until exercised through public HTTPS plus the required product auth/front-door setup.
