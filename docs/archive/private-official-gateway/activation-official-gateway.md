# Official Gateway Activation

The official Codencer connector path is Gateway-first:

```text
AI client -> Codencer Gateway -> selected Relay -> local connector -> daemon -> project
```

Use this path for ChatGPT custom MCP app setup, Claude Code MCP setup, and Codex
MCP setup. Direct self-host Relay `/mcp` remains supported for
advanced/direct/debug testing, but it is not the primary official connector
endpoint.

## Build

```bash
make build-codencer
make build-gateway
```

## Configure Gateway

Gateway config is local runtime state:

```text
$CODENCER_HOME/runtime/gateway/config.json
```

Create it with bearer-dev auth, OAuth dev metadata, persistent Gateway store,
and default managed Relay settings:

```bash
codencer setup gateway \
  --base-url https://mcp.codencer.dev \
  --mcp-url https://mcp.codencer.dev/mcp \
  --listen 127.0.0.1:19090 \
  --default-relay-url https://relay.codencer.dev \
  --default-relay-token-env CODENCER_DEFAULT_RELAY_TOKEN \
  --token-env CODENCER_GATEWAY_MCP_TOKEN \
  --enable-oauth-dev \
  --json
```

Start the Gateway with server-side Relay token environment variables:

```bash
export CODENCER_DEFAULT_RELAY_TOKEN=<managed-relay-planner-token>
codencer-gatewayd serve --config "$CODENCER_HOME/runtime/gateway/config.json"
```

Then perform self-service local connector setup:

```bash
codencer login --gateway https://mcp.codencer.dev
codencer connector login --gateway https://mcp.codencer.dev --relay default --json
codencer project share codencer --json
codencer connector run --config "$CODENCER_HOME/runtime/connector/config.json"
```

Optional: add a self-host Relay profile. This requires `codencer login` because
profiles are stored in the Gateway workspace registry.

```bash
export CODENCER_RELAY_PERSONAL_TOKEN=<relay-planner-token>

codencer gateway relay add \
  --gateway https://mcp.codencer.dev \
  --name personal \
  --url https://relay.example.com \
  --token-env CODENCER_RELAY_PERSONAL_TOKEN \
  --json

codencer gateway relay list --gateway https://mcp.codencer.dev --json
```

## Generate Client Activation Package

```bash
codencer activation official \
  --gateway https://mcp.codencer.dev \
  --relay https://relay.example.com \
  --project codencer \
  --token-env CODENCER_GATEWAY_MCP_TOKEN \
  --json
```

The package contains:

- Gateway curl smoke script;
- Codex config pointing to `https://mcp.codencer.dev/mcp`;
- Claude Code command pointing to `https://mcp.codencer.dev/mcp`;
- ChatGPT custom MCP app setup pointing to `https://mcp.codencer.dev/mcp`;
- relay-profile setup instructions;
- connector login instructions;
- evidence checklist.

## Gateway MCP Tools

Gateway exposes:

- `codencer.list_relays`
- `codencer.get_relay`
- `codencer.list_projects`
- `codencer.get_project`
- `codencer.list_project_locations`
- `codencer.submit_project_task_and_wait`
- `codencer.run_project_manifest`
- `codencer.get_run_report`
- `codencer.get_blocker`

Gateway aggregates projects across enabled Relay profiles and forwards execution
to the selected Relay. Backend Relay bearer tokens are never returned to AI
clients. Gateway sanitizes backend Relay output so absolute local repo paths are
not exposed.

## Routing

Execution tools accept:

- `relay_profile_id`
- `machine_id`
- `host_label`

If one project is available from multiple Relay profiles and no
`relay_profile_id` is selected, Gateway returns structured blocker
`ambiguous_relay_profile`.

If one selected Relay exposes the project from multiple online machines and no
`machine_id` or `host_label` is selected, Gateway returns structured blocker
`ambiguous_project_location`.

If a backend Relay is down or cannot be reached, Gateway returns structured
blocker `relay_unavailable`.

Gateway never chooses randomly.

## Deterministic Verification

```bash
make verify-official-connector
```

The verifier uses isolated temp homes and random free ports. It starts a temp
Gateway, default official Relay, self-host Relay, local daemon, connectors, and
project, then verifies device login, connector login, Relay profile add, MCP
initialize, tools/list, relay/project listing, fake manifest execution through
default and self-host profiles, run report retrieval, ambiguity blockers,
relay-down blocker, and no obvious local path/token leakage.

Live ChatGPT, Codex, and Claude product proof remains pending until those
products actually connect to the Gateway MCP endpoint and evidence is saved.
