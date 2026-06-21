# Self-Host Production Deployment

This runbook is the public repository path for operating Codencer yourself. It
does not use Codencer-operated commercial services.

## Topology

```text
MCP client
  -> self-host codencer-gatewayd
  -> self-host codencer-relayd
  -> local codencer-connectord
  -> local orchestratord
  -> project repo
```

Gateway Console can run beside Gateway for operator visibility. Protect the
Console with private networking, VPN, reverse-proxy auth, or Zero Trust access
before exposing it beyond localhost.

## Endpoint Defaults

Public/self-built binaries default to:

- Gateway: `http://127.0.0.1:19090`
- MCP: `http://127.0.0.1:19090/mcp`
- Relay: `http://127.0.0.1:8090`
- Console: `http://127.0.0.1:3000`

Precedence is:

```text
CLI flags > env vars > user config profile > build-time defaults > self-host defaults
```

Use:

```bash
codencer config show --json
codencer config profiles list --json
codencer config profiles use self-host --json
codencer config set gateway.url http://127.0.0.1:19090 --json
```

Env overrides are `CODENCER_GATEWAY_URL`, `CODENCER_MCP_URL`,
`CODENCER_RELAY_URL`, and `CODENCER_CONSOLE_URL`.

## Build Artifacts

```bash
make build
make build-gateway
make release-snapshot VERSION=v0.3.0-selfhost
make verify-release-artifact-selfhost VERSION=v0.3.0-selfhost TARGETS=host REQUIRE_TARGETS=host
```

Release snapshots include checksums and install scripts. Do not place runtime
databases, sessions, connector configs, tokens, `.env` files, or proof bundles
inside release archives. Before publishing binaries, verify the actual archive:
the artifact proof unpacks the host `dist/codencer_*.tar.gz` file and runs the
self-host Gateway/Relay/Connector/MCP flow from the unpacked `bin/` directory,
not from source-tree `./bin`.

## Configure Self-Host Gateway

```bash
export CODENCER_HOME=$HOME/.codencer

codencer init --json
codencer setup self-host \
  --gateway-url http://127.0.0.1:19090 \
  --relay-url http://127.0.0.1:8090 \
  --listen 127.0.0.1:19090 \
  --token-env CODENCER_GATEWAY_MCP_TOKEN \
  --default-relay-token-env CODENCER_DEFAULT_RELAY_TOKEN \
  --enable-oauth-dev \
  --json
```

Start Relay and Gateway with operator-managed secrets:

```bash
export CODENCER_DEFAULT_RELAY_TOKEN=<self-host-relay-planner-token>
export CODENCER_GATEWAY_MCP_TOKEN=<gateway-client-token>

codencer-relayd --config "$CODENCER_HOME/runtime/relay/config.json"
codencer-gatewayd serve --config "$CODENCER_HOME/runtime/gateway/config.json"
```

## Configure Local Machine

```bash
codencer machine set-label machine-a --json
codencer login --gateway http://127.0.0.1:19090 --json
codencer connector login \
  --gateway http://127.0.0.1:19090 \
  --relay default \
  --daemon-url http://127.0.0.1:8085 \
  --json
codencer project init --repo . --json
codencer project share <project-id> --json
codencer connector run --config "$CODENCER_HOME/runtime/connector/config.json"
```

## Verify

```bash
make verify-public-selfhost-release
```

That gate runs isolated config precedence checks, release snapshot checks,
unpacked release-artifact self-host proof, Gateway/Relay/connector/MCP proof,
Gateway Console live proof, docs links, public/private boundary checks, and
token/path leakage checks.

Product UI proof for ChatGPT, Claude Code, and Codex remains separate from MCP
protocol proof. Do not mark those product clients verified until an operator
runs the actual product and saves evidence.
