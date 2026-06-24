# Codencer

Open-source local/self-host bridge between AI planners and coding executors.

Status: `v0.3.0-local-prod-rc.1`
License: Apache-2.0
Primary path: local-first daemon, self-host Gateway MCP, self-host Relay backend

This repository contains open-source Codencer Core plus self-hostable Gateway,
Relay, MCP, connector, Gateway Console, runbooks, and release tooling. A
future official managed Codencer service is separate from this public self-host
release and is not shipped by this repository.

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
  -> Codencer Gateway or local CLI
  -> selected self-host Relay
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
- executor profiles for fake, Codex, Claude, and task-level overrides;
- project registry in `$CODENCER_HOME/projects.json`;
- project-local committed `.codencer/project.json`;
- Grove-compatible workspace provisioning with native `.codencer/workspace.json`
  precedence and fallback reads from a safe subset of `grove.yaml` and
  `.groverc.json`;
- machine identity and editable `host_label` in `$CODENCER_HOME/machine.json`;
- manifest runner and deterministic fake profiles;
- structured blockers and validation results;
- self-host Relay;
- self-hostable Gateway daemon (`codencer-gatewayd`) implementing the official
  connector MCP surface;
- persistent Gateway user/workspace store with device-code login;
- default personal workspace and default self-host Relay profile support;
- Gateway relay profiles for routing to the default Relay or user-added
  self-host Relays;
- local connector with explicit project sharing;
- Relay-hosted MCP server and project-aware `codencer.*` MCP tools;
- Gateway-hosted MCP server and project-aware `codencer.*` MCP tools;
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
- hosted Codencer Cloud, commercial billing, or hosted UI availability from
  this repository;
- production multi-user Gateway auth beyond bearer-dev and OAuth dev metadata.

`codencer-gatewayd` is the open-source Gateway implementation used for
self-host deployments. Future official hosted Gateway/Cloud builds may override
build-time defaults to Codencer-operated domains, but public/self-built binaries
default to self-host/local endpoints and must not silently call commercial
services.

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
./bin/codencer executor list --json
./bin/codencer executor default fake-success --repo . --json
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
  --proxy-timeout-seconds 300 \
  --generate-planner-token \
  --json
```

For ChatGPT self-host testing, enable OAuth dev mode:

```bash
codencer setup relay \
  --base-url https://relay.example.com \
  --proxy-timeout-seconds 300 \
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

## Quickstart: Self-Host Gateway

Public/self-built Codencer defaults to self-host/local endpoints:

```text
Gateway: http://127.0.0.1:19090
MCP:     http://127.0.0.1:19090/mcp
Relay:   http://127.0.0.1:8090
Console: http://127.0.0.1:3000
```

Endpoint precedence is:

```text
CLI flags > env vars > user config profile > build-time defaults > self-host defaults
```

Use `codencer config show`, `codencer config profiles list`,
`codencer config profiles use self-host`, and
`codencer config set gateway.url <url>` to inspect or change local profile
state. Environment overrides are `CODENCER_GATEWAY_URL`, `CODENCER_MCP_URL`,
`CODENCER_RELAY_URL`, and `CODENCER_CONSOLE_URL`.

```bash
make build
make build-gateway

codencer setup self-host \
  --gateway-url http://127.0.0.1:19090 \
  --relay-url http://127.0.0.1:8090 \
  --listen 127.0.0.1:19090 \
  --relay-request-timeout-seconds 300 \
  --default-relay-token-env CODENCER_DEFAULT_RELAY_TOKEN \
  --token-env CODENCER_GATEWAY_MCP_TOKEN \
  --enable-oauth-dev \
  --json

export CODENCER_DEFAULT_RELAY_TOKEN=<self-host-relay-planner-token>
export CODENCER_GATEWAY_MCP_TOKEN=<gateway-client-token>
codencer-gatewayd serve --config "$CODENCER_HOME/runtime/gateway/config.json"
```

For real executor runs, keep Gateway `relay_request_timeout_seconds` and Relay
`proxy_timeout_seconds` at least as large as the task timeout. The default setup
value is `300` seconds; use the flags above when the operator sets a longer
executor timeout.

Local connector setup through self-host Gateway:

```bash
codencer login --gateway http://127.0.0.1:19090
codencer connector login --gateway http://127.0.0.1:19090 --relay default --json
codencer project share codencer --json
codencer connector run --config "$CODENCER_HOME/runtime/connector/config.json"
```

Optionally add another self-host Relay as a backend relay profile:

```bash
codencer gateway relay add \
  --gateway http://127.0.0.1:19090 \
  --name "Personal self-host Relay" \
  --url https://relay.example.com \
  --token-env CODENCER_RELAY_PERSONAL_TOKEN \
  --json

codencer connector login --gateway http://127.0.0.1:19090 --relay personal --json
```

Generate self-host Gateway activation artifacts:

```bash
codencer activation self-host \
  --gateway http://127.0.0.1:19090 \
  --relay http://127.0.0.1:8090 \
  --project codencer \
  --token-env CODENCER_GATEWAY_MCP_TOKEN \
  --json
```

Gateway tools aggregate projects across relay profiles and forward execution to
the selected Relay. If multiple relay profiles expose the same project and no
`relay_profile_id` is selected, Gateway returns `ambiguous_relay_profile`. If
the selected Relay has multiple online machine locations and no `machine_id` or
`host_label` is selected, Gateway returns `ambiguous_project_location`. Gateway
does not expose backend Relay tokens or absolute local paths.

## MCP Clients

Codencer exposes one project-aware MCP toolset through Gateway. Public
self-host users point Codex, Claude Code, ChatGPT custom MCP apps, or protocol
smoke clients at the self-host Gateway MCP URL. Client setup commands generate
snippets and instructions; they do not write user-level client config files.

```bash
./bin/codencer setup mcp --client codex --endpoint http://127.0.0.1:19090/mcp --json
./bin/codencer setup mcp --client claude-code --endpoint http://127.0.0.1:19090/mcp --json
./bin/codencer activation self-host --gateway http://127.0.0.1:19090 --relay http://127.0.0.1:8090 --project codencer --json
```

See:

- [Codex MCP activation](docs/mcp/codex-mcp-live.md)
- [Claude Code MCP activation](docs/mcp/claude-code-mcp-live.md)
- [ChatGPT custom MCP app setup](docs/mcp/chatgpt-app-setup.md)
- [Self-host production deployment](docs/deployment/self-host-production.md)
- [Self-host MCP proof](docs/mcp/self-host-mcp-proof.md)
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

## Executor Profiles

Project defaults live in `.codencer/project.json`; task-level overrides use the
same profile names through CLI, Gateway API, MCP tools, and Gateway Console.
Codencer routes to the selected executor profile and records reports/audit
events. It does not become the agent.

```bash
codencer executor list --json
codencer executor scan --json
codencer executor test codex-workspace --json
codencer executor default codex-workspace --repo . --json
codencer submit --project codencer --profile codex-workspace --goal "Run the approved task" --wait --json
```

See [Executor Profiles](docs/executor-profiles.md).

## Gateway And Self-Host Relay

The public self-host connector path is Gateway-first:

```text
AI client -> Codencer Gateway -> selected Relay -> local connector -> daemon -> project
```

Direct self-host Relay MCP remains supported for advanced, direct, and debug
testing:

```text
AI client -> user Relay /mcp
```

Future official hosted Gateway/Cloud builds may use Codencer-operated service
identity and domains. Forks and hosted services can use the OSS core under
Apache-2.0, but they must use their own name and must not imply they are the
official Codencer service.

## Docs Map

- [Docs index](docs/README.md)
- [Local quickstart](docs/quickstart-local.md)
- [Local production guide](docs/local-production.md)
- [Self-host Relay quickstart](docs/quickstart-self-host-relay.md)
- [Self-host production deployment](docs/deployment/self-host-production.md)
- [Self-host MCP proof](docs/mcp/self-host-mcp-proof.md)
- [Account device login](docs/account-device-login.md)
- [Relay profile registry](docs/relay-profile-registry.md)
- [Gateway Console status](docs/gateway-console.md)
- [Gateway Console design system](docs/ui/design-system.md)
- [VPS Relay activation](docs/activation-vps-relay.md)
- [Local connector activation](docs/activation-local-connector.md)
- [Project config](docs/project-config.md)
- [Executor profiles](docs/executor-profiles.md)
- [Codex MCP activation](docs/mcp/codex-mcp-live.md)
- [Claude Code MCP activation](docs/mcp/claude-code-mcp-live.md)
- [ChatGPT custom MCP app setup](docs/mcp/chatgpt-app-setup.md)
- [ChatGPT OAuth dev mode](docs/mcp/chatgpt-oauth-dev.md)
- [MCP integrations](docs/mcp/integrations.md)
- [Relay MCP tools](docs/mcp/relay_tools.md)
- [MCP Gateway model](docs/architecture/mcp-gateway-model.md)
- [Official vs self-host](docs/architecture/official-vs-self-host.md)
- [Public/private boundary](docs/architecture/public-private-boundary.md)
- [Runtime supervisor](docs/runtime-supervisor.md)
- [Live execution matrix](docs/live-execution-matrix.md)
- [Troubleshooting](docs/TROUBLESHOOTING.md)
- [Acceptance contract](docs/acceptance/local-production-v0.3.yaml)
- [Public self-host release acceptance](docs/acceptance/public-self-host-release.md)
- [Public repo release acceptance](docs/acceptance/public-repo-release.yaml)

Legacy setup and release-track documents are archived under `docs/archive/legacy-local-track/`.

## Development

Use Go 1.25+.

Core verification:

```bash
gofmt -w ...
go test ./...
make build-codencer
make build-supported
make build-self-host-cloud   # optional self-host/community cloud-control-plane binaries
make verify-project-config
make verify-local-execution
make verify-local-relay-mcp
make verify-gateway
make verify-runtime-recovery
make verify-live-matrix
make acceptance-local-production
make verify-release
make verify-local-prod
make activation-preflight
make verify-public-release
make verify-gateway-console
make verify-public-selfhost-rc
git diff --check
```

`make verify-public-selfhost-rc` builds a fresh release artifact, unpacks it,
uses the unpacked binaries for self-host Gateway/Relay/Connector/daemon smoke,
runs Gateway Console live verification, writes
`reports/public-selfhost-rc/<timestamp>/summary.json` and `.md`, and reports
`NO-GO` when no required real executor gate is configured or proven.

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
