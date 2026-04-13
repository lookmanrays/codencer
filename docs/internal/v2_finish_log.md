# Codencer Practical V2 Finish Log

## Goal
- Move the current repo from materially usable self-host alpha to practical v2 for serious personal operator use.
- Keep Codencer local-first, planner-controlled, evidence-oriented, and truthful about limitations.

## Locked Blockers
- Connector blocks documented evidence routes for step artifacts, validations, and logs.
- Relay `wait_step` is effectively capped by an internal 15s proxy timeout.
- Relay MCP is not yet full Streamable HTTP compatible and lacks protocol/origin hardening.
- Relay MCP is missing gate discovery.
- Planner auth ergonomics are still manual; keep static tokens but add safe local helpers.
- Connector share/unshare/list/config ergonomics are missing.
- Relay connector disable/enable is not exposed.
- Self-host cold-start docs are not yet copy-pasteable.
- Daemon truth surfaces need hardening around degraded broker status, request contracts, conflicts, and artifact confinement.

## Locked Decisions
- Planner auth remains static-token based in this pass.
- Add helper commands/scripts for planner token generation and relay config updates; do not add DB-backed planner auth lifecycle.
- Implement full Streamable HTTP compatibility on the relay MCP surface.
- Daemon-local MCP remains secondary compatibility/admin-only.

## Workstream Ownership
- Lead:
  - `docs/internal/v2_finish_log.md`
  - Final integration, docs truth pass, smoke expansion, verification
- Worker A:
  - `internal/service/run_service.go`
  - `internal/service/instance_service.go`
  - `internal/app/routes.go`
  - `internal/app/api_test.go`
  - daemon/service tests related to contract hardening
- Worker B:
  - `cmd/codencer-connectord/main.go`
  - `internal/connector/*`
  - connector tests
  - `docs/CONNECTOR.md`
- Lead local critical path before Worker D:
  - `cmd/codencer-relayd/main.go`
  - `internal/relay/{config.go,server.go,router.go,auth.go,hub.go,audit.go}`
  - `internal/relay/store/*`
  - relay tests
  - `docs/RELAY.md`
- Worker D after relay control-plane merge:
  - `internal/relay/mcp_server.go`
  - `internal/relay/mcp_tools.go`
  - `internal/relay/mcp_server_test.go`
  - `docs/mcp/*`
  - `schemas/*.json`
- Lead docs/scripts:
  - `README.md`
  - `docs/SELF_HOST_REFERENCE.md`
  - `docs/SETUP.md`
  - `docs/WSL_WINDOWS_ANTIGRAVITY.md`
  - `docs/TROUBLESHOOTING.md`
  - `docs/AI_OPERATOR_GUIDE.md`
  - `cmd/broker/README.md`
  - `scripts/self_host_smoke.sh`
  - `Makefile`

## Merge Sequence
1. Log and ownership lock
2. Daemon hardening
3. Connector completion
4. Relay control-plane/admin
5. Relay MCP maturity
6. Operator docs/scripts and smoke flow
7. Final formatting, verification, truth pass

## Status
- Merge 1: completed
- Merge 2: completed
- Merge 3: completed
- Merge 4: completed
- Merge 5: completed
- Merge 6: completed
- Merge 7: completed

## Verification Ledger
- Initial repo audit complete.
- `go test ./...` passed before implementation.
- `go build ./cmd/orchestratord ./cmd/orchestratorctl ./cmd/codencer-connectord ./cmd/codencer-relayd` passed before implementation.
- `make build-broker` previously verified during audit.
- Focused daemon, connector, and relay package tests passed after merge work:
  - `go test ./internal/app ./internal/service ./internal/connector ./cmd/codencer-connectord`
  - `go test ./internal/relay ./cmd/codencer-relayd`
- Final broad verification passed:
  - `go test ./...`
  - `make build`
  - `make build-broker`
  - live self-host smoke with temporary local relay config, simulation daemon, connector enrollment, relay status/audit, and MCP coverage via `scripts/self_host_smoke.sh`
- Explicit gate and abort lifecycle verification passed:
  - `go test ./internal/service -run 'TestE2EFlow|TestGateService_ApproveAndRejectLifecycle|TestRunService_AbortRunCancelsActiveAttempt|TestRunService_DispatchStepAsyncImmediateAbortCancelsBeforeAdapterStart|TestRunService_AbortRunWithoutConfirmedStopFailsClosed|TestRunService_AbortRunWithoutRegisteredExecutionMarksManualAttentionAndReturnsError|TestRunService_AbortRunPausedForGateFailsClosed' -v -count=1`

## Open Notes
- MCP transport work must follow official MCP transport/lifecycle/tools guidance and remain truthful if any subset is still unsupported after implementation.
- No two write-capable workers should touch the same file concurrently.
