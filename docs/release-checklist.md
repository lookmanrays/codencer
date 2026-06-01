# Release Checklist

Use this for the local/self-host production release-candidate path. It is intentionally non-live by default.

## Non-Live Verification

```bash
gofmt -w <touched-go-files>
go test ./...
make build-codencer
make verify-local-execution
make verify-local-relay-mcp
make verify-runtime-recovery
make verify-live-matrix
make acceptance-local-production
make verify-release
make verify-local-prod
```

## Release Snapshot

```bash
make release-snapshot VERSION=v0.3.0-local-prod-rc.1
```

The snapshot writes `dist/manifest.json` and `dist/checksums.txt`. Because Codencer uses `go-sqlite3` and CGO, local machines may not cross-compile every OS/arch target. The release manifest is the source of truth: buildable targets are `built`, unavailable targets are `not_built` or `skipped`, and no signed/notarized claim is made.

## Acceptance Evidence

```bash
./bin/codencer accept local-production --json --bin-dir ./bin --repo .
./bin/codencer proof bundle --json
```

Attach the acceptance report and proof bundle to release notes. Do not mark live Codex, live Claude, ChatGPT product UI, WSL live, or installed-service smokes as passed unless those exact commands were run and evidence was saved.

## Non-Goals

Sprint 6 does not add commercial Codencer Cloud, billing, hosted UI, managed execution, Windows-native daemon production, dangerous bypass by default, or planner-style decision making.
