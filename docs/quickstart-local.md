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
