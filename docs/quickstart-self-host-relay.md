# Self-Host Relay/MCP Quickstart

Self-host Relay is the backend remote transport for Gateway and the advanced
direct MCP path. Execution remains local and daemon-first. Projects are exposed
only after explicit `project share`; the relay is not a raw shell or arbitrary
filesystem browser.

Official client setup should use:

```text
AI client -> Codencer Gateway -> selected Relay -> local connector -> daemon -> project
```

Direct `https://relay.example.com/mcp` remains supported for
advanced/direct/debug testing.

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

The generated planner token is written under `$CODENCER_HOME/tokens` with `0600` permissions and redacted from command output. Use your own reverse proxy or native TLS config for public HTTPS.

For ChatGPT self-host testing, enable the single-user OAuth dev front-door:

```bash
./bin/codencer setup relay \
  --base-url https://relay.example.com \
  --mcp-url https://relay.example.com/mcp \
  --generate-planner-token \
  --enable-chatgpt-oauth-dev \
  --json
```

Generated OAuth client/operator secrets are stored only under `$CODENCER_HOME/tokens`; relay config stores hashes.

## Share A Project

```bash
./bin/codencer project init --repo . --id codencer --name "Codencer" --json
./bin/codencer project share codencer --json
```

Remote project descriptors include `locations[]` with `machine_id`, `host_label`, connector/instance ids, status, and safe labels/hashes instead of absolute local paths. The local registry keeps full paths for local execution. If more than one online machine advertises the same `project_id`, MCP execution must pass `machine_id` or `host_label`; otherwise the relay returns `ambiguous_project_location`.

## Enroll Local Connector

Create the enrollment token on the relay host:

```bash
export CODENCER_MCP_TOKEN=<planner-token-with-connectors-enroll-scope>
./bin/codencer-relayd enrollment-token create --relay-url https://relay.example.com --token "$CODENCER_MCP_TOKEN" --label local-connector --json
```

Then enroll and run the local connector:

```bash
export CODENCER_CONNECTOR_ENROLLMENT_TOKEN=<enrollment-token>
./bin/codencer connector enroll \
  --relay-url https://relay.example.com \
  --daemon-url http://127.0.0.1:8085 \
  --enrollment-token "$CODENCER_CONNECTOR_ENROLLMENT_TOKEN" \
  --config "$CODENCER_HOME/runtime/connector/config.json" \
  --label local-connector \
  --json
./bin/codencer connector run --config "$CODENCER_HOME/runtime/connector/config.json"
```

Use `codencer-connectord enroll` and `codencer-connectord run` only as low-level fallbacks.

## MCP Client Snippets

Official client snippets should point to Gateway:

```bash
./bin/codencer setup mcp --client codex --endpoint https://mcp.codencer.dev/mcp --token-env CODENCER_GATEWAY_MCP_TOKEN --json
./bin/codencer setup mcp --client claude-code --endpoint https://mcp.codencer.dev/mcp --token-env CODENCER_GATEWAY_MCP_TOKEN --json
./bin/codencer setup mcp --client chatgpt --endpoint https://mcp.codencer.dev/mcp --json
```

Direct Relay snippets are available for advanced/debug mode:

```bash
./bin/codencer setup mcp --client codex --endpoint https://relay.example.com/mcp --token-env CODENCER_PLANNER_TOKEN --json
./bin/codencer setup mcp --client claude-code --endpoint https://relay.example.com/mcp --token-env CODENCER_PLANNER_TOKEN --json
./bin/codencer setup mcp --client chatgpt --endpoint https://relay.example.com/mcp --json
```

These commands generate snippets only. They do not write user-level Codex, Claude, or ChatGPT configuration.

Activation artifacts:

```bash
./bin/codencer activation gateway --gateway https://mcp.codencer.dev --relay https://relay.example.com --project codencer --token-env CODENCER_GATEWAY_MCP_TOKEN --json
./bin/codencer activation package --relay https://relay.example.com --project codencer --token-env CODENCER_MCP_TOKEN --json
./bin/codencer activation chatgpt --relay https://relay.example.com --project codencer --auth oauth --json
./bin/codencer activation codex --relay https://relay.example.com --token-env CODENCER_MCP_TOKEN --json
./bin/codencer activation claude-code --relay https://relay.example.com --token-env CODENCER_MCP_TOKEN --json
```

## Verify

```bash
make verify-local-relay-mcp
CODENCER_LIVE_RELAY_MCP=1 ./bin/codencer live relay-mcp --json --bin-dir ./bin --repo .
./bin/codencer accept local-production --profile relay --json --bin-dir ./bin --repo .
```

Live Codex/Claude executor proof and ChatGPT product UI proof remain pending unless explicitly run in the target product environment.

## Platform Notes

Self-host Linux and WSL2 deployments use the `linux/amd64` release artifact or a source build performed on Linux/WSL. Windows-native daemon binaries are not part of the Sprint 6.1 production claim. ChatGPT product UI proof remains manual and pending until exercised through public HTTPS plus OAuth dev mode or the required production auth/front-door setup.
