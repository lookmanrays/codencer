# Local Production Troubleshooting

## Setup Reports `ready_with_skips`

This is normal when optional live proof was not enabled. Run deterministic acceptance first:

```bash
./bin/codencer accept local-production --json --bin-dir ./bin --repo .
```

Enable live checks only when the needed product CLIs, auth, and services are available.

## Daemon Not Running

```bash
./bin/codencer service status daemon --json
./bin/codencer doctor --json
./bin/codencer watchdog once --json
```

Normal execution commands do not auto-start production daemons. Start services explicitly with `codencer up` or run a foreground daemon for temporary smoke work.

## Relay Project Does Not Appear

Check all three conditions:

```bash
./bin/codencer project get <id> --json
./bin/codencer project share <id> --json
./bin/codencer readiness --relay --json
```

Only `shared_to_relay:true` projects are advertised. The connector skips unreachable daemons and relay-instance mismatches with warnings.

## MCP Auth Fails

MCP clients should authenticate to the operator's self-host Gateway. Gateway
supports bearer-dev auth and OAuth dev mode for pre-prod product setup:

```bash
./bin/codencer setup mcp --client codex --endpoint http://127.0.0.1:19090/mcp --token-env CODENCER_GATEWAY_MCP_TOKEN --json
./bin/codencer setup mcp --client claude-code --endpoint http://127.0.0.1:19090/mcp --token-env CODENCER_GATEWAY_MCP_TOKEN --json
./bin/codencer setup mcp --client chatgpt --endpoint http://127.0.0.1:19090/mcp --json
```

Direct Relay MCP auth is still available for advanced/direct/debug mode. It
requires bearer auth, Relay OAuth dev mode, or an OAuth front door that forwards
a token Codencer accepts:

```bash
./bin/codencer setup mcp --client codex --endpoint https://relay.example.com/mcp --json
./bin/codencer setup mcp --client claude-code --endpoint https://relay.example.com/mcp --json
./bin/codencer setup mcp --client chatgpt --endpoint https://relay.example.com/mcp --json
```

ChatGPT product UI proof requires public HTTPS, OAuth-style product setup, and
an eligible workspace. For Gateway-first testing, generate the server-side
activation artifacts with `codencer activation self-host --gateway https://gateway.example.com --relay https://relay.example.com --auth oauth --json`.
Keep product proof pending until the actual product flow is exercised.

## Connector Enrollment Fails

Use the facade first so `$CODENCER_HOME/config.json` records the connector config path:

```bash
./bin/codencer connector enroll \
  --relay-url https://relay.example.com \
  --daemon-url http://127.0.0.1:8085 \
  --enrollment-token "$CODENCER_CONNECTOR_ENROLLMENT_TOKEN" \
  --config "$CODENCER_HOME/runtime/connector/config.json" \
  --json
./bin/codencer connector status --config "$CODENCER_HOME/runtime/connector/config.json" --json
```

If the facade is unavailable, fall back to `codencer-connectord enroll` with the same relay URL, daemon URL, enrollment token, and config path. Enrollment tokens are one-time/short-lived; generate a fresh token on the relay host before retrying.

## Release Snapshot Misses Some Targets

`go-sqlite3` requires CGO. Sprint 6.1 release snapshots default to required `darwin/arm64`, `darwin/amd64`, and `linux/amd64` targets. From macOS, Linux builds use Docker. If Docker is unavailable or the Linux build fails, the default release snapshot should fail.

Use one of these explicit paths:

```bash
make release-snapshot VERSION=v0.3.0-local-prod-rc.1
make release-snapshot VERSION=v0.3.0-local-prod-rc.1 TARGETS=host
make release-snapshot VERSION=v0.3.0-local-prod-rc.1 ALLOW_PARTIAL=1
```

`ALLOW_PARTIAL=1` is an honest partial artifact set, not a final multi-platform production release. Do not fake archives. Do not claim Windows-native daemon production support; use WSL2/Linux with the Linux artifact or a Linux source build.

If `codencer accept local-production` reports `release_artifacts_present` failed, regenerate the release snapshot or remove stale generated `dist/manifest.json`/`dist/checksums.txt` files. A source ZIP is not a release artifact.

## Never Do These By Default

- Do not use `codex-danger-bypass` without explicit isolated-machine intent and `CODENCER_ALLOW_DANGEROUS_BYPASS=1`.
- Do not purge `$CODENCER_HOME` unless `scripts/uninstall.sh --purge` is explicitly requested.
- Do not paste bearer tokens into reports or docs.
- Do not treat Codencer as the planner; it reports blockers and evidence.
