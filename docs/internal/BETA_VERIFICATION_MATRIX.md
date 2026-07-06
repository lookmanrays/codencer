# Beta Verification Matrix

> [!WARNING]
> Historical v0.2 beta record. This is not the current v0.3 local/self-host RC release contract. Use [README](../../README.md), [Local Quickstart](../quickstart-local.md), and [Self-Host Relay Quickstart](../quickstart-self-host-relay.md) for current guidance.

This matrix records the historical phase-by-phase evidence plus the final Phase 7 confirmation pass that locked beta on 2026-04-23.

## Executed In Phase 0

| Scenario | Command / method | Result | Proof type | Notes |
| --- | --- | --- | --- | --- |
| Main module test suite | `go test ./...` | Pass | Direct Phase 0 run | Main module only. |
| Main binaries build | `make build` | Pass | Direct Phase 0 run | Builds daemon, CLI, relay, connector. |
| Cloud binaries build | `make build-cloud` | Pass | Direct Phase 0 run | Builds `codencer-cloud*`. |
| Broker binary build | `make build-broker` | Pass | Direct Phase 0 run | Nested module builds successfully. |
| MCP SDK helper build | `make build-mcp-sdk-smoke` | Pass | Direct Phase 0 run | Helper binary available. |
| Local smoke | `make smoke` | Pass | Direct Phase 0 run | Single-step simulation proof. |
| Legacy six-input local smoke | `make start-sim && ./scripts/smoke_test_v1.sh && make stop` | Pass | Direct Phase 0 run | Re-run before beta because one delegated run reported a conflicting failure. |
| Self-host relay/connector smoke | `./scripts/self_host_smoke.sh` with `status,audit` | Pass | Direct Phase 0 run | HTTP relay/runtime proof. |
| Self-host relay MCP + SDK smoke | `SMOKE_SCENARIOS=status,audit,mcp,mcp-sdk ./scripts/self_host_smoke.sh` | Pass | Direct Phase 0 run | Relay MCP and official Go SDK proof. |
| Cloud binary smoke | `make cloud-smoke` | Pass | Direct Phase 0 run | Bootstrap/status/install/audit proof. |
| Docker compose config validation | `make cloud-stack-config` | Pass | Direct Phase 0 run | Compose baseline renders cleanly. |
| Broker nested module tests | `cd cmd/broker && go test ./...` | Fail | Direct Phase 0 run | `task_test.go` syntax error blocks test proof. |

## Executed In Phase 1 (WS-C1 Cloud)

| Scenario | Command / method | Result | Proof type | Notes |
| --- | --- | --- | --- | --- |
| Cloud control-plane and scope regression suite | `go test ./internal/cloud/... -count=1` | Pass | Direct Phase 1 run | Covers token revoke scope, event/audit filtering, runtime proxy scope, cloud MCP parity, and cloud SDK interop tests. |
| Cloud MCP SDK helper compile | `go test ./cmd/mcp-sdk-smoke -count=1` | Pass | Direct Phase 1 run | Confirms the helper binary still builds cleanly after the cloud proof updates. |
| Baseline cloud smoke | `make cloud-smoke` | Pass | Direct Phase 1 run | Proves bootstrap, status, install, webhook ingest, events, audit, and worker-once behavior. |
| Composed cloud runtime HTTP + MCP + SDK smoke | `CLOUD_RELAY_CONFIG=... CLOUD_RUNTIME_DAEMON_URL=... CLOUD_SMOKE_MCP=1 CLOUD_SMOKE_SDK=1 ./scripts/cloud_smoke.sh` | Pass | Direct Phase 1 run | Proves claimed runtime visibility, run create/get, submit-task, wait/result success, cloud MCP initialize/list/call, and official Go SDK interoperability in one composed flow. |
| Docker compose cloud stack smoke | `make cloud-stack-smoke` | Blocked | Direct Phase 1 run | Docker CLI was present, but the local Docker daemon/socket was unavailable (`<docker-socket>`). |
| Docker compose config validation | `make cloud-stack-config` | Pass | Direct Phase 1 run | Compose baseline still renders cleanly even though the local Docker daemon was unavailable. |

## Executed In Phase 2 (WS-R1 Relay / Runtime)

| Scenario | Command / method | Result | Proof type | Notes |
| --- | --- | --- | --- | --- |
| Relay + connector regression suite | `go test ./internal/connector ./internal/relay ./cmd/codencer-connectord -count=1` | Pass | Direct Phase 2 run | Covers share validation, live-set re-advertise, relay hub pruning, relay MCP parity, and relay HTTP integration. |
| Relay MCP SDK helper compile | `go test ./cmd/mcp-sdk-smoke -count=1` | Pass | Direct Phase 2 run | Confirms the official SDK smoke helper still builds cleanly during WS-R1. |
| Self-host smoke with share-control, MCP, and SDK | `PLANNER_TOKEN=... RELAY_CONFIG=... RELAY_URL=... DAEMON_URL=... SMOKE_SCENARIOS=status,audit,share-control,mcp,mcp-sdk ./scripts/self_host_smoke.sh` | Pass | Direct Phase 2 run | Proves enrollment, connector session, relay HTTP flow, unshare -> not routable, re-share by `instance_id` -> routable, manual relay MCP flow, and official Go SDK interop. |
| Self-host smoke with multi-instance targeting | `PLANNER_TOKEN=... RELAY_CONFIG=... RELAY_URL=... DAEMON_URL=... SMOKE_SCENARIOS=status,audit,share-control,multi-instance,mcp,mcp-sdk ./scripts/self_host_smoke.sh` | Pass | Direct Phase 2 run | Re-exercises explicit instance targeting and route isolation through one connector serving two daemons. |
| Self-host smoke script syntax | `bash -n scripts/self_host_smoke.sh` | Pass | Direct Phase 2 run | Guards the updated self-host smoke entrypoint. |

## Executed In Phase 3 (WS-L1 Local Core)

| Scenario | Command / method | Result | Proof type | Notes |
| --- | --- | --- | --- | --- |
| Local daemon/app/service/CLI regression suite | `go test ./internal/app ./internal/service ./cmd/orchestratorctl -count=1` | Pass | Direct Phase 3 run | Covers local wait/result truth, retry lifecycle, gate routes, evidence retrieval, and CLI wait/retry behavior. |
| Local adapter + daemon-local MCP package suite | `go test ./internal/adapters/... ./internal/mcp -count=1` | Pass | Direct Phase 3 run | Confirms the current local adapter package coverage and keeps daemon-local MCP classified as compatibility-only. |
| Legacy six-input smoke syntax | `bash -n scripts/smoke_test_v1.sh` | Pass | Direct Phase 3 run | Guards the updated local parity smoke entrypoint. |
| Legacy six-input local smoke, run 1 | `./scripts/smoke_test_v1.sh` | Pass | Direct Phase 3 run | Auto-started a temporary simulation daemon, exercised six submit modes in one run, and fetched result/logs/artifacts/validations. |
| Legacy six-input local smoke, run 2 | `./scripts/smoke_test_v1.sh` | Pass | Direct Phase 3 run | Repeated from a clean daemon lifecycle to close the conflicting Phase 0 parity evidence. |
| Baseline local smoke | `make smoke` | Pass | Direct Phase 3 run | Re-proved the standard local happy path after the WS-L1 wait/result/retry changes. |

## Executed In Phase 4 (WS-P1 Planner / Client Integrations)

| Scenario | Command / method | Result | Proof type | Notes |
| --- | --- | --- | --- | --- |
| Relay/cloud MCP regression suite and SDK helper compile | `go test ./internal/relay ./internal/cloud ./cmd/mcp-sdk-smoke -count=1` | Pass | Direct Phase 4 run | Re-proved the canonical relay/cloud MCP surfaces and kept the official Go SDK helper green under the frozen planner/client contract. |
| Self-host relay HTTP + MCP + SDK proof against the canonical relay surface | `RELAY_CONFIG=... SMOKE_SCENARIOS=status,audit,share-control,mcp,mcp-sdk ./scripts/self_host_smoke.sh` | Pass | Direct Phase 4 run | Re-proved relay `/api/v2` plus canonical relay `/mcp`, compatibility `/mcp/call`, official Go SDK interop, and share-control routing on a fresh PTY-held daemon and relay. |
| Composed cloud HTTP + MCP + SDK proof against the canonical cloud surface | `CLOUD_RELAY_CONFIG=... CLOUD_RUNTIME_DAEMON_URL=http://127.0.0.1:8085 CLOUD_SMOKE_MCP=1 CLOUD_SMOKE_SDK=1 ./scripts/cloud_smoke.sh` | Pass | Direct Phase 4 run | Re-proved cloud `/api/cloud/v1/runtime/...`, canonical `/api/cloud/v1/mcp`, compatibility `/api/cloud/v1/mcp/call`, and official Go SDK interop on top of a live local daemon. |
| Claude Code example config syntax | `python3 -m json.tool docs/mcp/examples/claude-code-relay.mcp.json` and `python3 -m json.tool docs/mcp/examples/claude-code-cloud.mcp.json` | Pass | Direct Phase 4 run | Confirms the checked-in project-scoped `.mcp.json` examples are valid JSON before they are referenced publicly. |

## Executed In Phase 5 (WS-PC1 Provider Connectors)

| Scenario | Command / method | Result | Proof type | Notes |
| --- | --- | --- | --- | --- |
| Provider connector regression suite | `go test ./internal/cloud/... -count=1` | Pass | Direct Phase 5 run | Re-proved append-only event history, Jira webhook deferment, action/audit attribution, provider worker polling, and the provider connector package suite after the WS-PC1 fixes. |
| Provider connector package suite | `go test ./internal/cloud/connectors -count=1` | Pass | Direct Phase 5 run | Re-ran the mocked provider validation/webhook/action/status coverage for GitHub, GitLab, Jira, Linear, and Slack. |
| Focused routed-provider proof | `go test ./internal/cloud -run 'Test(ServerAdminAndConnectorFlows|WebhookHistoryPreservesRepeatedSourceEventIDs|JiraWebhookRouteReturnsDeferredWithoutPersistingEvents|ConnectorActionLogsCaptureRequestCompletionAndAuditDetails|WorkerRunOncePollsJiraAndPersistsSnapshot|StoreCreateConnectorEventPreservesRepeatedSourceEventHistory)' -count=1` | Pass | Direct Phase 5 run | Directly proves Slack install/validate/webhook/event/audit flow, append-only repeated event history, Jira webhook deferment, Jira polling persistence, and connector action-log completeness. |
| Cloud smoke after provider fixes | `make cloud-smoke` | Pass | Direct Phase 5 run | Re-proved the generic cloud install/validate/webhook/events/audit path on top of the updated provider connector persistence and audit behavior. |

## Executed In Phase 6 (WS-RE1 Release Engineering / Public Testability)

| Scenario | Command / method | Result | Proof type | Notes |
| --- | --- | --- | --- | --- |
| Supported build surface | `make build-supported` | Pass | Direct Phase 6 run | Builds the main local/relay binaries, cloud binaries, and the MCP SDK helper in one public target. |
| Repo-level supported verification from working tree | `make verify-beta` | Pass | Direct Phase 6 run | Runs main-module tests, local smoke, self-host relay/runtime MCP+SDK smoke, cloud binary smoke, and Docker compose config validation with temporary bootstrap for the relay/runtime slice. |
| Repo-level supported verification from a fresh location | detached temporary `git worktree` run of `make build-supported && make verify-beta` | Pass | Direct Phase 6 run | Re-proved the supported non-Docker path away from the active checkout while preserving the Git metadata required by worktree-sensitive checks. |
| Docker compose config validation after deployment packaging changes | `make cloud-stack-config` | Pass | Direct Phase 6 run | Confirms the compose file still renders cleanly after version/build-arg wiring changes. |
| Docker compose cloud stack smoke | `make cloud-stack-smoke` | Blocked | Environment-limited in Phase 6 | Docker CLI is available, but the local Docker daemon/socket is unavailable in this environment. This remains the only deployment proof deferred to final beta confirmation on a Docker-capable host. |

## Executed In Phase 7 (Beta Confirmation)

| Scenario | Command / method | Result | Proof type | Notes |
| --- | --- | --- | --- | --- |
| Supported build surface rerun | `make build-supported` | Pass | Direct Phase 7 run | Rebuilt the primary supported binaries and the MCP SDK helper before final confirmation. Local `/usr/local/opt/grpc/lib` linker warnings still appeared, but they were non-blocking. |
| Legacy six-input local smoke rerun, run 1 | `./scripts/smoke_test_v1.sh` | Pass | Direct Phase 7 run | Re-ran the six-input local parity smoke from the documented entrypoint during final confirmation. |
| Legacy six-input local smoke rerun, run 2 | `./scripts/smoke_test_v1.sh` | Pass | Direct Phase 7 run | Repeated the same smoke from a fresh daemon lifecycle to preserve the two-run parity proof. |
| Baseline local smoke rerun | `make smoke` | Pass | Direct Phase 7 run | Re-ran the standard local happy path during final confirmation. |
| Fresh self-host relay/runtime smoke | `PLANNER_TOKEN=... RELAY_CONFIG=... RELAY_URL=... DAEMON_URL=... SMOKE_SCENARIOS=status,audit,share-control,multi-instance,mcp,mcp-sdk ./scripts/self_host_smoke.sh` | Pass | Direct Phase 7 run | Re-proved relay HTTP, share-control, multi-instance routing isolation, canonical relay MCP, and official Go SDK interop. |
| Cloud control-plane + provider regression suite rerun | `go test ./internal/cloud/... -count=1` | Pass | Direct Phase 7 run | Re-proved cloud scope, runtime proxy, cloud MCP, provider connector, and worker-path coverage. |
| Relay/cloud MCP regression suite + SDK helper rerun | `go test ./internal/relay ./internal/cloud ./cmd/mcp-sdk-smoke -count=1` | Pass | Direct Phase 7 run | Re-proved the canonical relay/cloud MCP surfaces and kept the official Go SDK helper green. |
| Baseline cloud smoke rerun | `make cloud-smoke` | Pass | Direct Phase 7 run | Re-proved bootstrap, status, install, events, audit, and worker-once behavior. |
| Composed cloud runtime HTTP + MCP + SDK smoke rerun | `CLOUD_RELAY_CONFIG=... CLOUD_RUNTIME_DAEMON_URL=http://127.0.0.1:18085 CLOUD_SMOKE_MCP=1 CLOUD_SMOKE_SDK=1 ./scripts/cloud_smoke.sh` | Pass | Direct Phase 7 run | Re-proved claimed runtime visibility, cloud HTTP run/create/get + submit-task, canonical cloud MCP, and official Go SDK interoperability on the confirmation daemon URL. |
| Repo-level supported verification rerun | `make verify-beta` | Pass | Direct Phase 7 run | Re-ran main-module tests, local smoke, self-host relay/runtime MCP+SDK smoke, cloud binary smoke, and Docker compose config validation from the active checkout. |
| Fresh-location repo verification rerun | detached temporary `git worktree` run of `make build-supported && make verify-beta` | Pass | Direct Phase 7 run | Re-proved the supported non-Docker path from a fresh location while preserving the Git metadata required by worktree-sensitive checks. |
| Docker compose cloud stack smoke rerun | `make cloud-stack-smoke` | Pass | Direct Phase 7 run | Re-proved the Docker-backed self-host cloud baseline on a host with a live Docker daemon. |
| Final-tree Docker-inclusive repo verifier | `make verify-beta-docker` | Pass | Direct Phase 7 run | Re-ran the supported repo verifier plus the Docker-backed cloud stack baseline after the Phase 7 truth-normalization updates landed. |

## Repo-Test Proof Already Present

| Scenario | Proof source | Coverage level | Notes |
| --- | --- | --- | --- |
| Relay HTTP run/step/result/gate/artifact routing | `internal/relay/integration_test.go` | Strong | Strongest remote HTTP proof. |
| Relay MCP streamable contract + call alias | `internal/relay/mcp_server_test.go` | Strong | Phase 2 added duplicate-token-name principal regression coverage and alias `tools/call` proof. |
| Relay official Go SDK interop | `internal/relay/mcp_server_test.go` | Strong | Explicit SDK proof. |
| Connector enrollment/session/challenge/heartbeat | `internal/connector/*_test.go` | Strong | Phase 2 added live reachable-set re-advertise proof beyond config-only changes. |
| Cloud org/workspace/project/membership/token surfaces | `internal/cloud/*_test.go`, `cmd/codencer-cloudctl/main_test.go` | Strong | Phase 1 added revoke-target scope checks and revoked-token denial coverage. |
| Cloud runtime registry / claim / scope | `internal/cloud/runtime_api_test.go` | Strong | Phase 1 added nested runtime-scope enforcement and live proxy proof. |
| Cloud MCP streamable contract + call alias | `internal/cloud/mcp_server_test.go` | Strong | Includes initialize/list/call/stream/delete, origin handling, token-bound sessions, and revoked-token denial. |
| Cloud official Go SDK interop | `internal/cloud/mcp_server_test.go`, `cmd/mcp-sdk-smoke` | Strong | Repo tests plus composed smoke now prove official Go SDK access to `/api/cloud/v1/mcp`. |
| Provider connector mocks + routed provider proof | `internal/cloud/connectors/*_test.go`, `internal/cloud/worker_test.go`, `internal/cloud/router_test.go`, `internal/cloud/store_test.go` | Medium | Stronger than Phase 0 because routed install/webhook/action/history coverage now exists, but still mock/provider-fixture proof rather than live vendor-account proof. |

## Post-Beta Flagship Planner Loop Proof

| Scenario | Command / method | Result | Proof type | Notes |
| --- | --- | --- | --- | --- |
| Codex adapter `codex exec` fake-binary proof | `go test ./internal/adapters/codex -count=1` | Pass in post-beta implementation run | Direct repo test | Proves non-simulation CLI invocation, stdin prompt delivery, legacy opt-in args, result synthesis when Codex writes only a final message, failure on missing evidence, and rejection of unsupported result states. |
| Connector wait returns at gate decision | `go test ./internal/connector -count=1` | Pass in post-beta implementation run | Direct repo test | `wait_step` now returns with `needs_decision=true` when the local step reaches `needs_approval`. |
| Cloud HTTP runtime wait/retry/gate parity | `go test ./internal/cloud -run TestCloudRuntimeHTTPProxyStepWaitRetryAndGateActions -count=1` | Pass in post-beta implementation run | Direct repo test | Closes cloud HTTP parity gaps found during the flagship loop audit. MCP remained the primary planner surface. |
| Relay/cloud MCP OAuth protected-resource metadata | `go test ./internal/relay ./internal/cloud -run 'Test.*OAuthProtectedResource' -count=1` plus smoke metadata probes | Pass in post-beta implementation run | Direct repo tests + smoke | Proves relay/cloud metadata endpoints, 401 `WWW-Authenticate` bearer challenges, public-base URL handling, relay lowercase bearer parsing, and configured CORS origin support with exposed `WWW-Authenticate`. |
| Relay/cloud MCP product hardening | `go test ./internal/relay ./internal/cloud -count=1` | Pass in post-beta hardening run | Direct repo tests | Proves relay token-bound sessions, cloud token-bound sessions, POST-only compatibility aliases, and SSE stream survival past server write timeouts. |
| Official Go SDK multi-step phase-loop capability | `go test ./cmd/mcp-sdk-smoke -count=1` plus helper build/use | Pass in post-beta implementation run | Direct repo build/test + flagship smoke | `mcp-sdk-smoke` now supports `--step-count`, rejects unsuccessful terminal states by default, and emits `tool_names` in JSON output. |
| Product-path docs and examples | `python3 -m json.tool docs/mcp/examples/*.json docs/mcp/examples/*.mcp.json` plus markdown review | Pass in post-beta hardening run | Docs/config validation | Freezes ChatGPT Business/Enterprise/Edu custom MCP connector packaging, Claude Code commands, Anthropic Messages API request shape, and OAuth front-door deployment contract. |
| Flagship relay loop smoke | `make flagship-planner-smoke` | Pass in post-beta implementation run | Scripted smoke | Proves single-step, phase-loop, strict gate, multi-instance, reconnect, MCP auth metadata/challenge, MCP, SDK, and evidence retrieval through the relay path. Default proof uses simulation while targeting `codex`; live Codex remains an operator-environment proof path. |

## Remaining Beta-Gate Work

No additional proof remains required for the frozen beta tracks. Final confirmation reran the working-tree matrix, the fresh-location detached `git worktree` matrix, the Docker-backed cloud stack baseline, and the final-tree Docker-inclusive verifier.

## Out Of Beta Verification Scope

These do not block beta unless later promoted into the beta promise:

- VS Code extension runtime proof
- `agent-broker` runtime proof
- `ide-chat` end-to-end proof
- live vendor-account proof for every provider
- enterprise/cloud SaaS concerns outside self-host scope
