# Release Checklist

Use this for the local/self-host production release-candidate path. It is intentionally non-live by default.

## Non-Live Verification

```bash
gofmt -w <touched-go-files>
go test ./...
make build-codencer
make build-gateway
make verify-local-execution
make verify-local-relay-mcp
make verify-gateway
make verify-runtime-recovery
make verify-live-matrix
make acceptance-local-production
make verify-release
make verify-local-prod
make verify-release-artifact-selfhost VERSION=v0.3.0-local-prod-rc.1 TARGETS=host REQUIRE_TARGETS=host
make verify-public-release
```

## Release Snapshot

```bash
make release-snapshot VERSION=v0.3.0-local-prod-rc.1
```

The snapshot writes real `dist/codencer_*.tar.gz` archives, `dist/manifest.json`, and `dist/checksums.txt`. A GitHub source ZIP is not a release artifact.
Release archives include `codencer-gatewayd` alongside `codencer`,
`orchestratord`, `codencer-relayd`, and `codencer-connectord`.

Default required RC targets are:

```text
darwin/arm64,darwin/amd64,linux/amd64
```

Linux/WSL production requires a `linux/amd64` artifact or a source build on Linux/WSL. From macOS, the release helper uses Docker for Linux targets. If Docker or the Linux build is unavailable, the default release command must fail truthfully. Use partial mode only when intentionally producing a partial artifact set:

```bash
make release-snapshot VERSION=v0.3.0-local-prod-rc.1 ALLOW_PARTIAL=1
```

Partial mode is not a final multi-platform production release. Windows-native daemon binaries are not claimed in this RC; Windows operators should use WSL2/Linux. No signed/notarized claim is made.

Useful target overrides:

```bash
make release-snapshot VERSION=v0.3.0-local-prod-rc.1 TARGETS=host
make release-snapshot VERSION=v0.3.0-local-prod-rc.1 TARGETS=darwin/arm64,linux/amd64 REQUIRE_TARGETS=darwin/arm64,linux/amd64
```

Every artifact with `status:"built"` in the manifest must exist on disk and match `checksums.txt`. `make verify-release` fails if this is not true.

Binary release readiness also requires unpacked-artifact proof:

```bash
make verify-release-artifact-selfhost VERSION=v0.3.0-local-prod-rc.1 TARGETS=host REQUIRE_TARGETS=host
```

That gate selects the host artifact from `dist/manifest.json`, checks
`dist/checksums.txt`, extracts the archive to a temp directory, verifies the
required unpacked binaries, and runs the self-host Gateway/Relay/Connector/MCP
proof using only the unpacked `bin/` directory. Passing source-tree `./bin`
proof alone is not enough for binary release readiness.

Release archives must not contain local runtime state, SQLite stores, sessions,
tokens, non-example env files, or managed-service deployment credentials.
`make verify-public-release` checks the public boundary and scans built
artifacts when `dist/manifest.json` is present.

## Acceptance Evidence

```bash
./bin/codencer accept local-production --json --bin-dir ./bin --repo .
./bin/codencer proof bundle --json
```

Attach the acceptance report and proof bundle to release notes. Do not mark live Codex, live Claude, ChatGPT product UI, WSL live, or installed-service smokes as passed unless those exact commands were run and evidence was saved.

## Non-Goals

This release path does not add commercial Codencer Cloud, billing, hosted UI,
managed execution, Windows-native daemon production, dangerous bypass by
default, or planner-style decision making.
