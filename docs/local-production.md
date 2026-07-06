# Local Production Foundation

Sprint 1 added the local production foundation for Codencer as a hands-off local automation bridge. Sprint 2 added project-aware local execution through the existing daemon HTTP API. Sprint 3 adds project-aware self-host Relay/MCP routing for remote planners. Sprint 4 adds a conservative user-level runtime supervisor, watchdog, and recovery surface. Sprint 5 adds a live execution matrix and readiness reports that distinguish deterministic checks, skipped live proof, and actual live executor/client evidence. Sprint 7 adds activation packages, client preflight artifacts, and a minimal single-user OAuth dev front-door for ChatGPT testing. Sprint 8 adds the self-host Gateway MCP surface in front of backend Relays. Codencer accepts approved work, routes it to a local project/runtime/executor, captures structured state and evidence, and returns that state to the planner. Codencer surfaces state; planner decides.

Commercial Codencer Cloud is out of scope for this local production mode. The existing self-host relay, Gateway RC surface, and cloud-control-plane code remain in the repository, but this page focuses on the local production, Gateway, and self-host Relay/MCP foundation.

## Modes

Local CLI mode runs on one machine and uses the `codencer` facade for project registration, local configuration, health checks, status, run creation, task submission, and manifest execution. Execution is daemon-first: `codencer` talks to `/api/v1/runs`, `/api/v1/runs/{run}/steps`, `/api/v1/steps/{id}`, `/result`, `/artifacts`, `/validations`, and `/logs`. It does not shell out to `orchestratorctl`, create a second orchestration engine, or auto-start production daemons from normal commands.

Gateway mode exposes the public self-host MCP connector surface through `codencer-gatewayd`. Gateway stores users, personal workspaces, sessions, connector bindings, and Relay profiles, aggregates projects, routes execution to the selected Relay, and returns structured results/blockers without exposing backend Relay tokens or absolute local paths.

Self-host Relay/MCP mode keeps execution local and provides the backend route through `codencer-relayd` and `codencer-connectord`. Direct Relay `/mcp` remains available for advanced/direct/debug mode; the local daemon MCP endpoint is still a compatibility/admin bridge, not the public remote planner contract.

In Sprint 3, Relay is project-aware. The connector reads the user-level project registry, advertises only projects with `shared_to_relay:true`, and invokes the same `internal/localexec` contract used by the local CLI.

Runtime supervisor mode manages local daemon, relay, and connector processes as user services when the host supports launchd or systemd user units. It never requires sudo and does not auto-generate relay or connector secrets. Missing relay/connector config is reported as `not_configured`.

Live matrix mode reports what is installed, configured, skipped, blocked, or actually exercised. It is safe by default and does not call paid/authenticated live agents unless explicit `CODENCER_LIVE_*` variables or command flags enable those checks. See [Live Execution Matrix](live-execution-matrix.md).

Activation mode prepares self-host operators for real client setup. `codencer activation package` writes a package under `$CODENCER_HOME/artifacts/activation/`; `codencer activation check` validates local or remote relay readiness; and `codencer activation chatgpt|codex|claude-code` emits client-specific setup guidance without writing user client config. See [VPS Relay Activation](activation-vps-relay.md), [Local Connector Activation](activation-local-connector.md), and [MCP integration notes](mcp/integrations.md).

Self-host Gateway activation writes Gateway-first client artifacts with `codencer activation self-host`. See [Self-host MCP proof](mcp/self-host-mcp-proof.md) and [Self-host production deployment](deployment/self-host-production.md).

## Sprint 1 Capabilities

- A unified `codencer` CLI facade for local operators.
- User-level local production home at `CODENCER_HOME` or `$HOME/.codencer`.
- Project registry at `$CODENCER_HOME/projects.json`.
- Local config at `$CODENCER_HOME/config.json`.
- Structured `paths`, `config show`, `doctor`, `status`, and project commands with `--json`.
- Toolchain and runtime status checks that report unknown, skipped, warning, and not-running states honestly.
- Machine-readable acceptance gates for future local, Relay, and MCP production work.

## Sprint 2 Capabilities

- `codencer run start|list|get|status --project <id> --json`.
- `codencer submit --project <id> [--run <id>] (--goal|--task-file|--prompt-file|--stdin) [--wait] [--json]`.
- `codencer run-plan <manifest.yaml|json> --project <id> [--wait] --json`.
- `codencer profile list|get --json`.
- Project-aware daemon URL/profile resolution from the registry.
- Planner-facing execution reports with `blocker` and `evidence` fields.
- Deterministic fake adapter profiles for local execution verification.
- Validation command stdout/stderr artifact capture.

## Sprint 3 Capabilities

- `codencer project init --share-to-relay [--relay-instance-id <id>]`.
- `codencer project share <id> [--relay-instance-id <id>] [--daemon-url <url>] [--json]`.
- `codencer project unshare <id> [--json]`.
- Connector project discovery from `codencer_home`, `CODENCER_HOME`, or `$HOME/.codencer`.
- Relay project HTTP routes under `/api/v2/projects`.
- Relay MCP project tools such as `codencer.list_projects`, `codencer.submit_project_task_and_wait`, `codencer.run_project_manifest`, `codencer.get_execution_report`, and project step evidence tools.
- Planner token `project_ids` restrictions in addition to `instance_ids`.
- `codencer-relayd mcp-config --client codex|claude-code|chatgpt` setup snippets.
- Optional native relay TLS through `tls_cert_file` and `tls_key_file`; reverse-proxy HTTPS with `public_base_url` remains supported.

## Sprint 4 Capabilities

- `codencer service install|uninstall|start|stop|restart|status|logs|render`.
- `codencer watchdog once --json`.
- `codencer recover --dry-run --json`, `codencer recover locks --dry-run --json`, and `codencer recover run <run-id> --json`.
- macOS LaunchAgent rendering/management under `~/Library/LaunchAgents`.
- Linux systemd user unit rendering/management under `$HOME/.config/systemd/user`.
- WSL detection with systemd user manager use when available and explicit manual fallback when unavailable.
- Structured runtime blockers such as `daemon_not_running`, `connector_offline`, `relay_unreachable`, `stale_run`, `stale_step`, `stale_lock`, `executor_missing`, and `unknown_runtime_state`.

## Sprint 5 Capabilities

- `codencer live matrix --json` for non-live capability checks and honest skipped live gates.
- Guarded live commands: `codencer live codex|claude|relay-mcp|codex-mcp|claude-mcp|wsl|restart-reconnect --json`.
- `codencer readiness --json` for `ready`, `ready_with_skips`, or `not_ready` verdicts.
- Report persistence under `$CODENCER_HOME/artifacts/live-matrix/` and `$CODENCER_HOME/artifacts/readiness/`.
- Disposable live workspace harnesses that do not mutate the real repository.
- Relay/MCP live proof using real temp daemon, relay, and connector processes with fake executor profiles.
- MCP client config proof that distinguishes generated config, endpoint proof, actual client proof, and manual proof requirements.

## Sprint 7 Capabilities

- `codencer activation check|package|chatgpt|codex|claude-code --json`.
- Activation packages with README, real MCP curl smoke, Codex config, Claude Code command, ChatGPT setup sheet, and connector enrollment artifacts.
- `codencer connector enroll|run|status|config show` facade over the low-level connector binary.
- Relay OAuth dev endpoints for single-user ChatGPT testing:
  - `GET /.well-known/oauth-authorization-server`
  - `GET /.well-known/openid-configuration`
  - `GET|POST /oauth/authorize`
  - `POST /oauth/token`
- Explicit `setup relay --proxy-timeout-seconds 300 --enable-chatgpt-oauth-dev` and `--chatgpt-dev-noauth` modes.
- MCP read-only alias `codencer.get_blocker`.

OAuth dev mode is for self-host testing, not enterprise IAM. Dev no-auth is private-test only and is read-only/fake-project restricted unless `--allow-real-projects-in-dev-noauth` is explicit.

## Sprint 8 Capabilities

- `codencer-gatewayd` deployable Gateway binary.
- Gateway config at `$CODENCER_HOME/runtime/gateway/config.json`.
- `codencer setup self-host --relay-request-timeout-seconds 300 --json`.
- `codencer gateway relay add|list|status --json`.
- `codencer activation self-host --json`.
- Gateway MCP tools for `codencer.list_relays`, project listing, project location listing, manifest/task forwarding, run reports, and blockers.
- Routing by `relay_profile_id`, `machine_id`, and `host_label`.
- Structured blockers `ambiguous_relay_profile`, `ambiguous_project_location`, and `relay_unavailable`.
- Bearer-dev auth plus OAuth dev/protected-resource metadata for ChatGPT Developer Mode.
- Deterministic `make verify-gateway` E2E with isolated daemon, Relay, connector instances, Gateway, random free ports, and fake adapter execution.

## Sprint 9 Capabilities

- `codencer login`, `codencer whoami`, and `codencer logout`.
- Device-code style Gateway login with local session at `$CODENCER_HOME/session.json`.
- Default personal workspace and default self-host Relay profile creation.
- Persistent Gateway store for workspace Relay profiles and connector bindings.
- `codencer connector login --gateway ... --relay default|<profile>`.
- Remote `codencer gateway relay add|list|status|remove` against the Gateway workspace registry.
- Gateway MCP authorization using workspace-bound scopes.
- Deterministic `make verify-official-connector` E2E with isolated Gateway,
  default official Relay, self-host Relay, daemon, connectors, project, random
  free ports, MCP execution through both Relay profiles, ambiguity blockers,
  relay-down blocker, token redaction, and no absolute path leakage.

## Build

```bash
make build-codencer
make build-gateway
```

The broader `make build` target also builds `bin/codencer` alongside the existing low-level binaries, including `bin/codencer-gatewayd`.

## Initialize

```bash
./bin/codencer init
./bin/codencer init --json
```

This creates:

```text
$CODENCER_HOME/
  projects.json
  machine.json
  config.json
  logs/
  runtime/
  tokens/
  artifacts/
```

`tokens/` is created with restricted permissions. The project registry and local config do not store secrets. `machine.json` stores local-only machine identity (`machine_id`, `hostname`, editable `host_label`, OS, arch).

## Register A Project

```bash
./bin/codencer project init --id codencer --repo . --adapter codex
./bin/codencer project init --id fake --repo . --adapter fake --profile fake-success
./bin/codencer project adopt --repo . --json
./bin/codencer project scan --repo . --json
./bin/codencer machine show --json
./bin/codencer machine set-label macbook --json
./bin/codencer project share codencer --daemon-url http://127.0.0.1:8085
./bin/codencer project unshare codencer
./bin/codencer project list --json
./bin/codencer project get codencer --json
./bin/codencer project use codencer
./bin/codencer project status codencer --json
```

`project init` creates `repo/.codencer/project.json` when missing and adopts an existing file without overwriting it unless `--update-project-config` or `--force` is explicit. Default repo footprint is only `.codencer/project.json`; machine-specific state remains in `$CODENCER_HOME`. See [Project Config](project-config.md).

Project ids are stable lowercase ids matching `[a-z0-9][a-z0-9._-]{0,62}`. Repo roots are stored as absolute paths only in the local registry. A non-git directory is accepted with a warning so operators can register workspaces before Git metadata exists.

Project resolution order is:

1. explicit `--project`;
2. current project from the registry;
3. current working directory match.

`--profile` is an alias for the Sprint 1 `--adapter-profile` flag. Existing registries continue to work. A project with `--adapter codex` and no profile resolves to `codex-workspace`.

Relay sharing is explicit. A shared project is advertised only when its configured daemon is reachable and, if `relay_instance_id` is set, the live daemon reports the same instance id. Mismatches and unreachable daemons are status warnings, not silent sharing.

## Run And Submit

```bash
./bin/codencer run start --project codencer --json
./bin/codencer run list --project codencer --json
./bin/codencer run get <run-id> --project codencer --json
./bin/codencer submit --project codencer --goal "Update the docs" --wait --json
./bin/codencer submit --project codencer --run <run-id> --prompt-file task.md --profile codex-workspace --wait --json
```

If the configured daemon is not reachable, `codencer` returns a structured `daemon_not_running` blocker and exits `23`. It does not start a daemon implicitly.

## Run Plans

Run plans are YAML or JSON:

```yaml
version: codencer.io/v1alpha1
kind: RunManifest
metadata:
  name: docs-update
project:
  id: codencer
execution:
  adapter: codex
  profile: codex-workspace
  timeout: 30m
policy:
  stop_on_blocker: true
  stop_on_failure: true
  retry:
    enabled: true
    max_attempts: 2
tasks:
  - id: update-docs
    title: Update docs
    goal: Update the local production docs.
    validations:
      - name: go-test
        command: go test ./cmd/codencer
        timeout_seconds: 120
```

Sprint 2 executes tasks sequentially in listed order. Non-empty `depends_on` is rejected as `invalid_input`. Retry only runs when `policy.retry.enabled:true`, and only retryable timeout/adapter/bridge blockers are retried up to `max_attempts`. The final report is written to `$CODENCER_HOME/artifacts/run-plans/<run-id>.json`.

## Profiles

Built-in profiles:

- `codex-workspace`: default for `--adapter codex`, workspace-write posture.
- `codex-full`: full local access profile.
- `codex-danger-bypass`: requires explicit profile and `CODENCER_ALLOW_DANGEROUS_BYPASS=1`; otherwise returns `unsafe_action`.
- `claude-default`: default for `--adapter claude`.
- `fake-success`, `fake-failure`, `fake-blocker`, `fake-timeout`: deterministic daemon adapter profiles for tests.

For Codex and Claude, the daemon-facing adapter remains `codex` or `claude`. Fake profiles send daemon-facing adapter IDs such as `fake-success`.

## Blocker Protocol

Execution reports use these exit codes:

- `0`: success.
- `10`: blocked or planner decision required.
- `20`: failed terminal.
- `21`: validation failed.
- `22`: adapter failed.
- `23`: daemon or bridge failed.
- `24`: timeout.
- `30`: invalid input or config.
- `40`: internal error.

Result mapping is intentionally structured. Validation failures return `blocker.type:"validation_failed"`. Policy gates return `manual_approval_required`. Manual-attention results with questions return `question`. Adapter failures return `adapter_error`. Bridge failures and daemon connection failures return `bridge_error` or `daemon_not_running`. Timeout returns `timeout`. Reports never emit `suggested_next_action`.

## Fake Adapter Test Mode

Fake profiles are registered as daemon adapters and still use the normal daemon lifecycle:

- `fake-success`: writes completed `result.json`.
- `fake-failure`: writes `failed_terminal`.
- `fake-blocker`: writes `needs_manual_attention`, `needs_human_decision:true`, and a neutral question.
- `fake-timeout`: remains running until cancelled so daemon timeout handling can be tested.

Use `make verify-local-execution` to run the fake execution harness against a temporary daemon and project registry.

## Runtime Supervisor

Service commands operate on the required services `daemon`, `relay`, and `connector`:

```bash
./bin/codencer service status --all --json
./bin/codencer service install daemon --dry-run --json
./bin/codencer service install --all --dry-run --json
./bin/codencer service render daemon --format launchd
./bin/codencer service render daemon --format systemd
./bin/codencer service logs daemon --tail 100
```

`daemon` is configured from the resolved project and runs `orchestratord --config <generated-daemon-config> --repo-root <repo>`. The generated daemon config lives under `$CODENCER_HOME/runtime/services` and points daemon state back at the repo-local `.codencer` directory. `relay` runs only when `relay_config_path` exists. `connector` runs only when `connector_config_path` exists. Binary resolution checks explicit overrides, the current binary directory, repo `bin/`, then `PATH`.

Aliases are available for common service flow:

```bash
./bin/codencer up
./bin/codencer down
./bin/codencer restart
./bin/codencer logs daemon --tail 100
```

## Watchdog And Recovery

```bash
./bin/codencer watchdog once --json
./bin/codencer recover --dry-run --json
./bin/codencer recover locks --dry-run --json
./bin/codencer recover run <run-id> --dry-run --json
```

Watchdog checks local paths, registry/project resolution, service manager status, daemon health, relay health when configured, connector status when configured, executor binaries required by registered projects, stale workspace locks, stale active runs/steps, and artifact/runtime writability.

Recovery is intentionally conservative. Dry-run reports planned actions. Non-dry-run may recreate missing runtime directories and remove clearly stale Codencer-owned locks only when safety can be verified. Service restarts require `--restart-services`. Run recovery calls the daemon recovery endpoint; the CLI does not write daemon SQLite state directly. Recovery never invents tasks, approves gates, retries failed engineering work, deletes artifacts/logs, or emits `suggested_next_action`.

## Relay And MCP

Public client setup uses the self-host Gateway. See [Self-host MCP proof](mcp/self-host-mcp-proof.md) and [MCP Gateway model](architecture/mcp-gateway-model.md).

Self-host Relay also exposes direct/advanced/debug surfaces:

- HTTP project routes: `GET /api/v2/projects`, `GET /api/v2/projects/{id}`, `GET|POST /api/v2/projects/{id}/runs`, `POST /api/v2/projects/{id}/submit`, `POST /api/v2/projects/{id}/run-plan`, `GET /api/v2/projects/{id}/reports/run-plans/{run}`, and project step evidence routes.
- MCP endpoint: `POST|GET|DELETE /mcp`, with `/mcp/call` as a compatibility POST alias.
- OAuth protected-resource metadata: `GET /.well-known/oauth-protected-resource/mcp`.

Project MCP tools return `structuredContent` plus JSON text content. Project listings include `locations[]` with `machine_id`, `host_label`, connector/instance ids, status, and safe repo labels/hashes. Absolute local repo paths are not exposed. Project execution accepts optional `machine_id` or `host_label`; if multiple online machines advertise the same `project_id` and no selector is provided, the relay returns `ambiguous_project_location` instead of choosing randomly.

Planner blockers remain data, not transport failures. Existing instance-centric `codencer.*` MCP tools remain available for compatibility, but project tools are the preferred remote planner surface.

Example remote task submission:

```bash
curl -fsS \
  -H "Authorization: Bearer <planner-token>" \
  -H "Content-Type: application/json" \
  -d '{"goal":"Update docs","profile":"fake-success","wait":true}' \
  https://relay.example.com/api/v2/projects/codencer/submit
```

Direct Relay MCP setup snippets for advanced/debug mode:

```bash
./bin/codencer-relayd mcp-config --client codex --endpoint https://relay.example.com/mcp --token-env CODENCER_PLANNER_TOKEN
./bin/codencer-relayd mcp-config --client claude-code --endpoint https://relay.example.com/mcp --token-env CODENCER_PLANNER_TOKEN
./bin/codencer-relayd mcp-config --client chatgpt --endpoint https://relay.example.com/mcp
```

ChatGPT custom MCP connector use requires a public HTTPS endpoint, OAuth dev/front-door style authentication, and an eligible Business/Enterprise/Edu workspace with developer mode. The self-host OAuth dev mode accepts valid redirect URIs for development; production must use redirect allowlisting or an external IdP. This repository does not claim ChatGPT live product proof unless that product flow is explicitly exercised.

## Inspect

```bash
./bin/codencer paths --json
./bin/codencer config show --json
./bin/codencer doctor --json
./bin/codencer doctor toolchain --json
./bin/codencer status --json
```

Doctor checks include OS/architecture, WSL detection, Go/Git/cc/curl, `go.mod` version, CGO signal, Codencer home and registry health, low-level binary/source presence, Codex and Claude CLI availability, Relay/connector configuration presence, and daemon health when a daemon URL is configured.

Live Codex or Claude authentication is not required in Sprint 1. If live proof is not possible, commands report skipped, warning, unknown, or not-running states instead of inventing validation.

## Platform Posture

- macOS: supported local development target; Sprint 4 includes LaunchAgent renderer/manager.
- Linux: supported local development target; Sprint 4 includes systemd user renderer/manager.
- WSL2: supported target for Windows-side planner workflows that keep repos, worktrees, daemon state, connector state, and artifacts inside WSL/Linux. If systemd user services are unavailable, Codencer reports an explicit manual fallback.
- Windows-native daemon binaries are not claimed for this release candidate; use WSL2/Linux for Windows-side workflows.

## Acceptance Checklist

- `bin/codencer` builds.
- `codencer version`, `paths --json`, `doctor --json`, and `doctor toolchain --json` work.
- `codencer init` creates local production paths.
- `codencer setup local --json` guides safe local setup.
- `codencer demo local --json` proves deterministic fake execution without live agents.
- Project init/list/get/use/status/remove work with `--json`.
- `CODENCER_HOME` works for tests and automation.
- `codencer accept local-production --json` persists an acceptance report.
- `codencer proof bundle --json` collects report references without secrets or full logs.
- `make verify-local-prod` passes.
- `make verify-release` passes.
- `make release-snapshot VERSION=v0.3.0-local-prod-rc.1` writes real release archives, checksums, and an honest manifest for required Darwin arm64, Darwin amd64, and Linux amd64 targets.
- `release_artifacts_present` passes when every manifest artifact with `status:"built"` exists on disk and matches `dist/checksums.txt`; a source ZIP is not a release artifact.
- `make verify-release-artifact-selfhost VERSION=v0.3.0-local-prod-rc.1 TARGETS=host REQUIRE_TARGETS=host` unpacks the host release archive and proves the self-host Gateway/Relay/Connector/MCP flow using only the unpacked `bin/` directory.
- `make verify-local-execution` passes.
- `make verify-local-relay-mcp` passes.
- `make verify-runtime-recovery` passes.
- `docs/acceptance/local-production-v0.3.yaml` tracks implemented and pending production gates.

## Sprint 6 Setup And Release UX

Final local production setup is guided through:

```bash
./bin/codencer setup local --json
./bin/codencer setup local --project-id codencer --repo . --adapter codex --profile codex-workspace --json
```

The command initializes `$CODENCER_HOME`, verifies binaries and config paths, optionally registers a project, and runs doctor/watchdog/readiness. It does not install or start services unless `--install-services` or `--start-services` is present.

Acceptance and proof commands are the machine-readable release evidence path:

```bash
./bin/codencer accept local-production --json --bin-dir ./bin --repo .
./bin/codencer accept reports --json
./bin/codencer proof bundle --json
```

`ready_with_skips` is acceptable when required deterministic gates pass and optional live gates were intentionally skipped. Live Codex, live Claude, WSL, installed service, and ChatGPT product UI gates stay pending/skipped until actually exercised.

Release snapshots are full only when all required targets build. If Docker or a Linux host is unavailable for the Linux amd64 artifact, `ALLOW_PARTIAL=1` must be explicit and the output must be labeled partial. Windows-native daemon production is not claimed; Windows operators use WSL2/Linux.

## Remaining Work

Live authenticated Codex/Claude proof, ChatGPT product UI proof, live macOS/Linux/WSL service smokes, UI, commercial cloud, and Windows-native daemon production remain pending unless separately verified.
