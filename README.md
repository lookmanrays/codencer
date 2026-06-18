# Codencer

Open-source local/self-host bridge between AI planners and coding executors.

Status: `v0.3.0-local-prod-rc.1`
License: Apache-2.0
Primary path: local-first daemon, self-host Relay, project-aware MCP

Codencer is a bridge, not a planner. The planner decides. The executor works.
Codencer accepts approved tasks or manifests, routes them to the right local
project/runtime, records state and evidence, and returns structured results or
blockers.

## What Codencer Is

Codencer is a stateful execution bridge for coding-agent work:

- Bridge, not brain.
- State, not chat.
- Planner decides what should happen next.
- Executor performs approved work.
- Codencer records runs, steps, attempts, artifacts, validations, logs, and
  blockers.

```text
Planner / MCP client
  -> Codencer Relay or local CLI
  -> local connector / daemon
  -> local adapter / executor
  -> structured result, evidence, blocker, or validation state
```

Codencer does not recursively plan work, invent product strategy, or provide a
generic remote shell. It preserves an auditable boundary between planning and
execution.

## What Exists Today

The v0.3 local/self-host RC includes:

- local `codencer` CLI mode;
- local daemon-first execution;
- project registry in `$CODENCER_HOME/projects.json`;
- project-local committed `.codencer/project.json`;
- machine identity and editable `host_label` in `$CODENCER_HOME/machine.json`;
- manifest runner and deterministic fake profiles;
- structured blockers and validation results;
- self-host Relay;
- local connector with explicit project sharing;
- Relay-hosted MCP server and project-aware `codencer.*` MCP tools;
- project locations with machine-aware routing by `machine_id` or `host_label`;
- Codex MCP setup snippets;
- Claude Code MCP setup snippets;
- ChatGPT custom MCP setup guidance with OAuth dev mode for self-host testing;
- activation package generation and activation preflight;
- readiness, acceptance, and proof bundle commands;
- release snapshot packaging for `darwin/arm64`, `darwin/amd64`, and
  `linux/amd64`;
- WSL2/Linux path for Windows users.

## What Is Not Claimed Yet

This repository does not claim:

- live ChatGPT product UI proof unless an operator actually runs ChatGPT and
  saves evidence;
- live Codex MCP client proof unless Codex actually connects and calls a tool;
- live Claude Code MCP client proof unless Claude Code actually connects and
  calls a tool;
- signed or notarized binaries;
- Windows-native daemon binaries or production daemon support;
- hosted Codencer Gateway, hosted Codencer Cloud, commercial billing, or hosted
  UI availability from this repository.

Self-host Relay is the open-source remote path today. Future Codencer
Gateway/Cloud is a separate managed/commercial layer and is not the same thing
as the OSS self-host Relay.

## Quickstart: Local

Prerequisites:

- Go 1.25+
- Git
- SQLite-capable local environment
- macOS, Linux, or WSL2

Build and initialize:

```bash
make build-codencer
./bin/codencer init --json
./bin/codencer machine show --json
./bin/codencer machine set-label my-laptop --json
```

Create or adopt a project config:

```bash
./bin/codencer project init \
  --id codencer \
  --repo . \
  --adapter fake \
  --profile fake-success \
  --json

./bin/codencer project status codencer --json
```

Run deterministic local proof:

```bash
./bin/codencer demo local --json --bin-dir ./bin
make verify-local-execution
```

`project init` creates `.codencer/project.json` when missing. If the file
already exists, `project init` adopts it into the local registry and does not
overwrite it unless explicitly requested. Local machine identity, daemon URLs,
absolute repo paths, connector identity, tokens, runtime state, logs, artifacts,
and proof bundles stay in `$CODENCER_HOME`, not in the committed project config.

## Quickstart: Self-Host Relay

Build release artifacts:

```bash
make release-snapshot VERSION=v0.3.0-local-prod-rc.1
```

Deploy the `linux/amd64` artifact to a Linux VPS or WSL2 host, then create a
relay config and planner token:

```bash
codencer setup relay \
  --base-url https://relay.example.com \
  --generate-planner-token \
  --json
```

For ChatGPT self-host testing, enable OAuth dev mode:

```bash
codencer setup relay \
  --base-url https://relay.example.com \
  --generate-planner-token \
  --enable-chatgpt-oauth-dev \
  --json
```

Create an enrollment token on the relay host, enroll the local connector, and
share the project explicitly:

```bash
codencer-relayd enrollment-token create --config relay.json --label laptop --json

codencer connector enroll \
  --relay-url https://relay.example.com \
  --daemon-url http://127.0.0.1:18085 \
  --enrollment-token "$CODENCER_CONNECTOR_ENROLLMENT_TOKEN" \
  --config "$CODENCER_HOME/runtime/connector/config.json" \
  --json

codencer project share codencer --json
codencer connector run --config "$CODENCER_HOME/runtime/connector/config.json"
```

Generate activation materials and run the server-side MCP smoke:

```bash
codencer activation package \
  --relay https://relay.example.com \
  --project codencer \
  --token-env CODENCER_MCP_TOKEN \
  --json

codencer activation check \
  --relay https://relay.example.com \
  --project codencer \
  --token-env CODENCER_MCP_TOKEN \
  --json
```

The relay exposes project locations safely. Planner/MCP outputs include
`locations[]` with machine and connector metadata plus safe repo labels/hashes;
they do not expose absolute local paths. If the same `project_id` is available
from multiple online machines, execution must pass `machine_id` or `host_label`.
Without a selector, Relay/MCP returns `ambiguous_project_location` instead of
choosing randomly.

## MCP Clients

Codencer exposes one project-aware MCP toolset through the self-host Relay.
Client setup commands generate snippets and instructions; they do not write
user-level client config files.

```bash
./bin/codencer setup mcp --client codex --endpoint https://relay.example.com/mcp --json
./bin/codencer setup mcp --client claude-code --endpoint https://relay.example.com/mcp --json
./bin/codencer setup mcp --client chatgpt --endpoint https://relay.example.com/mcp --json
```

See:

- [Codex MCP activation](docs/mcp/codex-mcp-live.md)
- [Claude Code MCP activation](docs/mcp/claude-code-mcp-live.md)
- [ChatGPT custom MCP app setup](docs/mcp/chatgpt-app-setup.md)
- [Relay MCP tools](docs/mcp/relay_tools.md)
- [MCP Gateway model](docs/architecture/mcp-gateway-model.md)

Do not mark product-client proof as passed until the actual product connects,
calls a tool, and evidence is saved.

## Project Config

The committed project config lives at:

```text
repo/.codencer/project.json
```

It describes stable, shareable project intent only. It must not contain secrets,
tokens, daemon URLs, relay URLs, connector identities, machine IDs, absolute
paths, runtime state, logs, artifacts, or proof bundles.

Local-only state lives under `$CODENCER_HOME`, including:

- `machine.json`
- `projects.json`
- connector config/status
- runtime logs and artifacts
- local acceptance/proof outputs

See [Project Config](docs/project-config.md).

## Self-Host Now, Gateway Later

Today’s open-source remote path is self-host Relay plus local connector. That is
the OSS path for remote planners and MCP clients.

Future Codencer Gateway/Cloud is planned as a separate managed/commercial layer
with official service identity. It must not be confused with the open-source
self-host Relay in this repository. Forks and hosted services can use the OSS
core under Apache-2.0, but they must use their own name and must not imply they
are the official Codencer service.

## Docs Map

- [Docs index](docs/README.md)
- [Local quickstart](docs/quickstart-local.md)
- [Local production guide](docs/local-production.md)
- [Self-host Relay quickstart](docs/quickstart-self-host-relay.md)
- [VPS Relay activation](docs/activation-vps-relay.md)
- [Local connector activation](docs/activation-local-connector.md)
- [Project config](docs/project-config.md)
- [Codex MCP activation](docs/mcp/codex-mcp-live.md)
- [Claude Code MCP activation](docs/mcp/claude-code-mcp-live.md)
- [ChatGPT custom MCP app setup](docs/mcp/chatgpt-app-setup.md)
- [ChatGPT OAuth dev mode](docs/mcp/chatgpt-oauth-dev.md)
- [MCP integrations](docs/mcp/integrations.md)
- [Relay MCP tools](docs/mcp/relay_tools.md)
- [MCP Gateway model](docs/architecture/mcp-gateway-model.md)
- [Runtime supervisor](docs/runtime-supervisor.md)
- [Live execution matrix](docs/live-execution-matrix.md)
- [Troubleshooting](docs/TROUBLESHOOTING.md)
- [Acceptance contract](docs/acceptance/local-production-v0.3.yaml)
- [codencer.dev update pack](docs/site-update-codencer-dev.md)

Legacy v0.2 beta documents are archived under `docs/archive/v0.2-beta/`.

## Development

Use Go 1.25+.

Core verification:

```bash
gofmt -w ...
go test ./...
make build-codencer
make verify-project-config
make verify-local-execution
make verify-local-relay-mcp
make verify-runtime-recovery
make verify-live-matrix
make acceptance-local-production
make verify-release
make verify-local-prod
make activation-preflight
git diff --check
```

If a live/external check is skipped, report it as skipped, not passed.

## License And Trademarks

Codencer Core is licensed under the Apache License, Version 2.0. See
[LICENSE](LICENSE) and [NOTICE](NOTICE).

Apache-2.0 covers the open-source software in this repository. It does not
grant rights to use Codencer trademarks, logos, domains, official connector
identity, Codencer Gateway, Codencer Cloud, Official Codencer MCP, or hosted
Codencer services.

Trademark guidance is in [TRADEMARKS.md](TRADEMARKS.md). Users may use, modify,
self-host, fork, and use the open-source core commercially. Forks and hosted
services must use their own name and must not imply they are the official
Codencer service.
