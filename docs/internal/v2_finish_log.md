# Codencer Practical V2 Delta Finish Log

## Goal
- Close the remaining delta from practical self-host alpha to full practical v2 for real self-use now.
- Keep Codencer local-first, planner-controlled, evidence-oriented, repo-bound, and truthful about runtime behavior and protocol maturity.

## Delta Blockers Locked From Repo Truth
- Share control is not yet fully truthful:
  - a running connector does not visibly re-advertise config-driven share changes
  - relay instance state is not pruned when a connector stops advertising a previously shared instance
- Operator discovery is incomplete:
  - discovery roots exist internally, but there is no operator-facing `codencer-connectord discover` command
- MCP maturity is only partially proven:
  - `/mcp` is usable, but `GET /mcp` is still a thin bootstrap instead of a real long-lived SSE session
  - external interoperability is not yet proven against an official SDK
- Multi-instance proof is thin:
  - the repo mostly proves single-instance relay flows rather than a two-instance select-and-target flow
- Daemon HTTP route proof is thinner than service proof:
  - direct route tests are missing for abort, gate decision, and step evidence endpoints
- Relay operator ergonomics still miss a local audit helper:
  - audit is available over HTTP, but not yet through `codencer-relayd`
- Broker/runtime docs need a truth pass:
  - standardize `agent-broker` naming everywhere
  - keep the in-memory task-session limitation explicit

## Locked Decisions
- Planner auth remains static-token based in this pass.
- The canonical remote planner surface remains relay HTTP plus relay-side MCP.
- Daemon-local MCP remains secondary/compatibility-only and does not gain new planner-facing claims.
- Add an operator-facing `codencer-connectord discover` command rather than overloading `list`.
- Prove MCP interoperability with the official Go SDK and keep compatibility claims exact.
- Make share/unshare propagate without connector restart by reloading config and re-advertising from the running connector.

## Workstream Ownership
- Lead:
  - `docs/internal/v2_finish_log.md`
  - integration, merge control, verification, final docs/smokes/truth pass
- Worker `Aristotle`:
  - `internal/app/*`
  - daemon-facing API tests
  - daemon/service tests only if route proof exposes a real mismatch
- Worker `Maxwell`:
  - `cmd/codencer-connectord/main.go`
  - `internal/connector/*`
  - connector tests
  - `docs/CONNECTOR.md`
- Worker `Fermat`:
  - `cmd/codencer-relayd/*`
  - `internal/relay/server.go`
  - `internal/relay/router.go`
  - `internal/relay/audit.go`
  - `internal/relay/store/*`
  - relay admin/integration tests
  - `docs/RELAY.md`
- Lead after relay merge:
  - `internal/relay/mcp_server.go`
  - `internal/relay/mcp_tools.go`
  - MCP tests
  - `docs/mcp/*`
  - official SDK smoke helper
  - multi-instance smoke/docs
  - broker/docs naming pass

## Merge Sequence
1. Lock delta log and ownership
2. Daemon HTTP proof hardening
3. Connector discovery plus live share propagation
4. Relay share-prune plus audit CLI
5. MCP streamable HTTP maturity plus official SDK smoke
6. Operator docs/scripts, multi-instance proof, broker truth pass
7. Formatting, broad verification, final truthful assessment

## Status
- Merge 1: completed
- Merge 2: completed
- Merge 3: completed
- Merge 4: completed
- Merge 5: completed
- Merge 6: completed
- Merge 7: completed

## Verification Ledger
- Fresh audit re-confirmed:
  - `go test ./...`
  - `make build`
  - `make build-broker`
- Current repo state before delta work:
  - branch: `codex/implement-codencer-v2`
  - untracked artifact observed: `./orchestratord`
- Required focused checks after each merge:
  - daemon: `go test ./internal/app ./internal/service`
  - connector: `go test ./cmd/codencer-connectord ./internal/connector`
  - relay: `go test ./internal/relay ./cmd/codencer-relayd`
  - final: `go test ./...`, `make build`, `make build-broker`, smoke matrix
- Merge 2 verified:
  - `go test ./internal/app ./internal/service`
  - added direct route proof for gate approve/reject, run abort, and step result/validations/logs
- Merge 3 verified:
  - `go test ./cmd/codencer-connectord ./internal/connector`
  - added `codencer-connectord discover`
  - added live config reload plus re-advertise for share/unshare propagation without connector restart
- Merge 4 verified:
  - `go test ./internal/relay ./cmd/codencer-relayd`
  - made connector advertise authoritative for relay-side shared-instance state
  - pruned stale instance rows and route hints when share state shrinks
  - added `codencer-relayd audit --limit N`
- Merge 5 verified:
  - `go test ./internal/relay ./cmd/codencer-relayd ./cmd/mcp-sdk-smoke`
  - upgraded `/mcp` from a one-shot bootstrap to a session-bound SSE stream with keepalive comments
  - added official Go SDK interoperability proof and a standalone `cmd/mcp-sdk-smoke` helper
- Merge 6 verified:
  - `bash -n scripts/self_host_smoke.sh`
  - `go test ./cmd/mcp-sdk-smoke`
  - `make build-mcp-sdk-smoke`
  - `make build-broker`
  - updated operator docs for `discover`, relay `audit`, broker naming truth, Go 1.25.0+, and the expanded smoke matrix
- Merge 7 verified:
  - `gofmt -w $(git diff --name-only -- '*.go')`
  - `go mod tidy`
  - `git diff --check`
  - `go test ./...`
  - `make build`
  - `make build-broker`
  - `make build-mcp-sdk-smoke`
  - live isolated self-host smoke with `SMOKE_SCENARIOS=all,mcp-sdk` against:
    - relay on `127.0.0.1:18090`
    - simulation daemon on `127.0.0.1:18085`
  - live smoke outcomes:
    - primary run completed with result, validations, logs, artifacts, audit, MCP, share-control, multi-instance, and official SDK proof paths exercised
    - optional gate action was skipped because the simulation path produced no gate
    - optional abort path returned HTTP `500`, matching the repo's best-effort, fail-closed abort truth rather than guaranteed confirmed cancellation
  - cleanup:
    - removed stray untracked root artifact `./orchestratord`

## Open Notes
- No two write-capable workers may edit the same files concurrently.
- MCP compatibility claims must stay exact; this repo now proves the official Go SDK path and manual JSON-RPC callers, not universal client compatibility.
- Share/unshare remote invisibility is now proven without restarting the connector in the live smoke flow.
- Live abort remains best-effort and may return HTTP `500` when cancellation cannot be confirmed quickly; that is expected truth, not a hidden regression.
