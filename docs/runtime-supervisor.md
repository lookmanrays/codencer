# Runtime Supervisor

Sprint 4 adds user-level service supervision for the local Codencer runtime. It manages the daemon, relay, and connector as local user services where the host supports it. It does not make Codencer a planner, does not require sudo, and does not create relay or connector secrets automatically.

## Architecture

```text
codencer service/watchdog/recover
  -> internal/supervisor
  -> launchd or systemd user manager
  -> orchestratord / codencer-relayd / codencer-connectord
```

Execution remains daemon-first. Normal task submission still uses the daemon HTTP API. Relay remains transport, auth, and audit. Recovery may restore runtime health or mark technical runtime state through daemon-owned endpoints, but the planner decides the next engineering step.

## Services

Required service names:

- `daemon`: runs `orchestratord --config <generated-daemon-config> --repo-root <repo>`.
- `relay`: runs `codencer-relayd serve --config <relay_config_path>` only when relay config exists.
- `connector`: runs `codencer-connectord run --config <connector_config_path>` only when connector config exists.

Generated service configs and render snapshots live under `$CODENCER_HOME/runtime/services`. Logs live under `$CODENCER_HOME/logs/<service>.stdout.log` and `.stderr.log`. The daemon config points state back at the repo-local `.codencer` directory.

## Commands

```bash
codencer service status --all --json
codencer service install daemon --dry-run --json
codencer service install --all --dry-run --json
codencer service start --all
codencer service stop --all
codencer service restart --all
codencer service logs daemon --tail 100
codencer service render daemon --format launchd
codencer service render daemon --format systemd
```

Aliases:

```bash
codencer up
codencer down
codencer restart
codencer logs daemon --tail 100
```

Use `--manager launchd`, `--manager systemd`, or `--manager manual` to force a manager. Default `auto` selects launchd on macOS, systemd user units on Linux when available, and manual fallback otherwise.

## macOS

LaunchAgent plists are rendered under:

```text
~/Library/LaunchAgents/io.codencer.<service>.plist
```

The manager uses `launchctl bootstrap`, `bootout`, `kickstart -k`, and `print`. Default verification renders plists and uses dry-run paths; live install/start/stop requires explicit operator action.

## Linux And WSL

Systemd user units are rendered under:

```text
$HOME/.config/systemd/user/codencer-<service>.service
```

The manager uses `systemctl --user daemon-reload`, `enable`, `start`, `stop`, `restart`, and `show`. Codencer never runs `sudo`. In WSL, systemd user services are used only when available. Without systemd, Codencer reports `unsupported` with manual/foreground context instead of pretending service management works.

## Watchdog

```bash
codencer watchdog once --json
```

Checks include local paths, project registry resolution, service status, daemon health, relay health when configured, connector status when configured, required executor binaries, stale runs/steps, stale workspace locks, and artifact/runtime writability.

Runtime blockers are structured and use `planner_decision_required:true`. Examples include `daemon_not_running`, `connector_offline`, `relay_unreachable`, `stale_run`, `stale_step`, `stale_lock`, `executor_missing`, and `unknown_runtime_state`.

## Recovery

```bash
codencer recover --dry-run --json
codencer recover locks --dry-run --json
codencer recover run <run-id> --dry-run --json
codencer recover --restart-services --json
```

Recovery is conservative. It may recreate missing runtime directories, remove clearly stale Codencer-owned locks when ownership is verified, and restart installed services only when `--restart-services` is set. Run recovery calls the daemon recovery endpoint and does not mutate daemon storage directly from the CLI.

Recovery does not approve gates, retry failed engineering work, continue a failed run without manifest policy, delete artifacts/logs, kill unknown processes, or emit `suggested_next_action`.

## Security

Services run as the current user. Logs may contain task output. Tokens, private keys, and bearer credentials are redacted from supervisor reports. Project registries do not store secrets; relay and connector secret lifecycle remains explicit operator configuration.

Relay deployments exposed beyond localhost should stay authenticated and be placed behind HTTPS or use the native TLS config documented in the self-host reference.

## Verification

Default verification is safe and does not install real services:

```bash
make verify-runtime-recovery
```

Live service smokes are opt-in:

```bash
CODENCER_LIVE_SERVICE_SMOKE=1 make live-service-macos-smoke
CODENCER_LIVE_SERVICE_SMOKE=1 make live-service-linux-smoke
CODENCER_LIVE_SERVICE_SMOKE=1 make live-service-wsl-smoke
```

Sprint 5 adds a safer foreground-process restart/reconnect smoke:

```bash
CODENCER_LIVE_SERVICE_RESTART=1 make live-restart-reconnect-smoke
```

That path starts temporary daemon, relay, and connector processes, proves project routing, stops/restarts connector and daemon, runs `watchdog once`, and keeps installed launchd/systemd services untouched unless `CODENCER_LIVE_INSTALLED_SERVICES=1` is explicitly set.

Remaining proof areas include live Codex execution, live Claude execution, live ChatGPT MCP product UI proof, full WSL service proof, and Windows-native broker hardening.

## Sprint 6 Setup Integration

The setup command can render/install/start services, but only with explicit flags:

```bash
codencer setup local --install-services --start-services --json
codencer setup relay --generate-planner-token --install-services --json
```

Default verification stays non-mutating:

```bash
codencer service install --all --dry-run --json
codencer service render daemon --format launchd
codencer service render daemon --format systemd
make verify-runtime-recovery
```

Release acceptance treats dry-run service render/status as deterministic proof. Live launchd/systemd install/start/stop remains opt-in and must be recorded separately before marking those gates passed.
