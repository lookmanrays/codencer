# VPS Relay Activation

This guide is for a self-host operator running `codencer-relayd` on a VPS and connecting local projects through `codencer-connectord`.

Codencer is a bridge, not a planner. Relay is transport, auth, routing, and audit. Execution remains daemon-first on the local machine that owns the project.

Official ChatGPT, Claude Code, and Codex connector setup should point to
Codencer Gateway, not directly to this Relay:

```text
AI client -> Codencer Gateway -> selected Relay -> local connector -> daemon -> project
```

This Relay `/mcp` endpoint remains available for advanced/direct/debug mode.

## Build Or Install

Use the `linux/amd64` release artifact on Linux/WSL2, or build from source on the VPS:

```bash
make build
```

Windows-native daemon production is not claimed. Use WSL2/Linux for production execution.

## Generate Relay Config

Bearer-token setup:

```bash
./bin/codencer setup relay \
  --base-url https://relay.example.com \
  --mcp-url https://relay.example.com/mcp \
  --generate-planner-token \
  --json
```

ChatGPT OAuth dev setup:

```bash
./bin/codencer setup relay \
  --base-url https://relay.example.com \
  --mcp-url https://relay.example.com/mcp \
  --generate-planner-token \
  --enable-chatgpt-oauth-dev \
  --json
```

The OAuth dev client secret and operator approval code are written under `$CODENCER_HOME/tokens` with `0600` permissions. The relay config stores only hashes and non-secret metadata.

Dev no-auth is only for private testing:

```bash
./bin/codencer setup relay \
  --base-url https://relay.example.com \
  --generate-planner-token \
  --chatgpt-dev-noauth \
  --json
```

By default dev no-auth grants read-only access to fake/test project ids only. Real project write tools require `--allow-real-projects-in-dev-noauth` and should not be used on a public relay.

## Start Relay

Use your preferred user-level process manager or Sprint 4 supervisor:

```bash
./bin/codencer service render relay --format systemd
./bin/codencer service install relay --manager systemd --dry-run --json
```

Public ChatGPT/Codex/Claude clients require HTTPS. Use a reverse proxy or native relay TLS config.

## Create Connector Enrollment Token

On the VPS, generate a short-lived connector enrollment token and copy only that token to the local machine:

```bash
export CODENCER_MCP_TOKEN=<planner-token-with-connectors-enroll-scope>
./bin/codencer-relayd enrollment-token create \
  --relay-url https://relay.example.com \
  --token "$CODENCER_MCP_TOKEN" \
  --label local-macbook \
  --json
```

On the local machine, prefer the facade:

```bash
./bin/codencer connector enroll \
  --relay-url https://relay.example.com \
  --daemon-url http://127.0.0.1:8085 \
  --enrollment-token "$CODENCER_CONNECTOR_ENROLLMENT_TOKEN" \
  --config "$CODENCER_HOME/runtime/connector/config.json" \
  --label local-macbook \
  --json
./bin/codencer connector run --config "$CODENCER_HOME/runtime/connector/config.json"
```

Fallback remains available through `codencer-connectord enroll` and `codencer-connectord run` when the facade is not installed.

## Activation Package

Generate official Gateway-first operator artifacts:

```bash
./bin/codencer activation gateway \
  --gateway https://mcp.codencer.dev \
  --relay https://relay.example.com \
  --project codencer \
  --token-env CODENCER_GATEWAY_MCP_TOKEN \
  --json
```

Direct Relay artifacts are still available for advanced/debug mode:

```bash
./bin/codencer activation package \
  --relay https://relay.example.com \
  --project codencer \
  --token-env CODENCER_MCP_TOKEN \
  --json
```

The package contains `README.md`, `curl-smoke.sh`, `codex-config.toml`, `claude-code-command.sh`, `chatgpt-app-setup.md`, `connector-enrollment.sh`, and `activation-package.json`.

## Remote Check

With a token:

```bash
export CODENCER_MCP_TOKEN=<planner-or-oauth-token>
./bin/codencer activation check \
  --relay https://relay.example.com \
  --project codencer \
  --token-env CODENCER_MCP_TOKEN \
  --check-oauth \
  --check-chatgpt-readiness \
  --json
```

With `--run-fake-manifest`, the check may call `codencer.run_project_manifest` through MCP. That is a server preflight only, not production live proof.

## Evidence States

Activation reports distinguish:

- `server_ready`
- `client_config_generated`
- `client_connected`
- `client_used_tool`
- `full_e2e_execution`

Do not mark ChatGPT, Codex, or Claude product proof passed until the actual client connects and calls a tool.
