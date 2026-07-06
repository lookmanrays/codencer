# Local Production Quickstart

Codencer runs approved tasks and manifests through local executors, then returns structured state, blockers, artifacts, logs, and validation evidence. It does not decide the next engineering step; the planner decides.

## Build And Initialize

```bash
make build
./bin/codencer version --json
./bin/codencer setup local --json
```

To register the current checkout as a project:

```bash
./bin/codencer setup local \
  --project-id codencer \
  --repo . \
  --adapter codex \
  --profile codex-workspace \
  --json
```

This creates or verifies `$CODENCER_HOME`, config, registry, logs, runtime, tokens, and artifacts. It does not install services or call live Codex/Claude unless explicit flags are used.

For direct project config flow:

```bash
./bin/codencer project scan --repo . --json
./bin/codencer project init --repo . --id codencer --name "Codencer" --json
./bin/codencer project adopt --repo . --json
./bin/codencer machine show --json
```

`project init` creates only `repo/.codencer/project.json` by default. Machine identity and absolute repo paths remain local in `$CODENCER_HOME`. See [Project Config](project-config.md).

## Executor Profiles

Inspect available executor profiles and change the project default without
reinitializing the project:

```bash
./bin/codencer executor list --json
./bin/codencer executor scan --json
./bin/codencer executor default codex-workspace --repo . --json
./bin/codencer executor test codex-workspace --json
```

For a single run, pass `--profile <executor-profile>` to override the project
default. See [Executor Profiles](executor-profiles.md).

## Deterministic Demo

```bash
./bin/codencer demo local --json --bin-dir ./bin
```

The demo creates a temporary `CODENCER_HOME`, temporary git repo, temporary daemon state, runs fake success, fake blocker, validation failure, and readiness, then reports generated evidence paths.

## Runtime Operations

```bash
./bin/codencer service status --all --json
./bin/codencer service install --all --dry-run --json
./bin/codencer watchdog once --json
./bin/codencer recover --dry-run --json
./bin/codencer readiness --json
```

Actual launchd/systemd install/start is explicit:

```bash
./bin/codencer setup local --install-services --start-services --json
```

## Acceptance And Proof

```bash
./bin/codencer accept local-production --json --bin-dir ./bin --repo .
./bin/codencer accept reports --json
./bin/codencer proof bundle --json
```

Default acceptance uses deterministic fake execution and may return `ready_with_skips` when live Codex, live Claude, ChatGPT product UI, WSL, or installed-service smokes were intentionally not run.

## Release Artifact Check

```bash
make release-snapshot VERSION=v0.3.0-local-prod-rc.1
```

This creates real release archives under `dist/` plus a manifest and checksums. A repository source ZIP is not a release artifact. The default RC release requires Darwin arm64, Darwin amd64, and Linux amd64; if the Linux artifact cannot be built locally, use Docker support, build on Linux/WSL, or run `ALLOW_PARTIAL=1` and label the output partial.
