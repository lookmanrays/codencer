# Beta Gap Register

> [!WARNING]
> Historical v0.2 beta record. This is not the current v0.3 local/self-host RC release contract. Use [README](../../README.md), [Local Quickstart](../quickstart-local.md), and [Self-Host Relay Quickstart](../quickstart-self-host-relay.md) for current guidance.

Severity scale:

- `critical`: direct security/scope blocker
- `high`: correctness blocker for a beta track
- `medium`: important proof/contract gap
- `low`: useful cleanup, not a beta gate by itself

This register preserves the historical blocker list plus the phase-by-phase closure record. As of 2026-04-23, no beta-blocking gaps remain open.

References below to WS-specific "remain outside" or "remaining work" notes describe the handoff state at the end of that phase, not current open beta blockers.

| ID | Title | Category | Impact | Severity | Affected files / areas | Beta-blocker | Next phase | Owner / workstream |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| BG-001 | Cloud token revocation ignores tenant scope | Cloud control plane | A scoped token can revoke another tenant token if it knows the ID. | `critical` | `internal/cloud/router.go`, `internal/cloud/store.go` | Yes | Phase 3 | WS-C1 Cloud |
| BG-002 | Cloud events API leaks cross-tenant event history | Cloud control plane | `GET /api/cloud/v1/events` can expose events outside the caller scope when `installation_id` is omitted. | `critical` | `internal/cloud/router.go`, `internal/cloud/store.go` | Yes | Phase 3 | WS-C1 Cloud |
| BG-003 | Cloud runtime HTTP route scopes are under-enforced | Cloud control plane | Cloud HTTP can proxy step/gate operations with weaker scopes than intended. | `critical` | `internal/cloud/runtime_api.go`, `internal/relay/auth.go` | Yes | Phase 3 | WS-C1 Cloud |
| BG-004 | Cloud audit API is only org-filtered | Cloud control plane | Workspace/project-scoped callers can read sibling audit rows. | `high` | `internal/cloud/router.go`, `internal/cloud/auth.go` | Yes | Phase 3 | WS-C1 Cloud |
| BG-005 | `share --instance-id` can persist an unroutable shared instance | Relay / connector | Operators can see a “shared” entry that will never advertise or route. | `high` | `internal/connector/admin.go`, `internal/connector/registry.go`, `docs/CONNECTOR.md` | Yes | Phase 2 | WS-R1 Relay/Connector |
| BG-006 | Provider webhook history overwrites repeated events | Provider connectors | Event storage is too shallow for a trustworthy connector audit trail. | `high` | `internal/cloud/router.go`, `internal/cloud/store.go`, provider normalizers | Yes | Phase 5 | WS-PC1 Providers |
| BG-007 | Jira webhook deferment is not enforced in runtime code | Provider connectors | Docs say Jira webhook ingest is deferred, but the route can still accept it. | `high` | `internal/cloud/connectors/jira.go`, `internal/cloud/router.go`, `docs/CLOUD*.md` | Yes | Phase 5 | WS-PC1 Providers |
| BG-008 | Provider action logs omit request body and completion time | Provider connectors | Publishability and audit quality are weaker than the control-plane story implies. | `medium` | `internal/cloud/router.go`, `internal/cloud/store.go` | Yes | Phase 5 | WS-PC1 Providers |
| BG-009 | Cloud runtime HTTP proxy paths are thinner in proof than relay | Planner/client + cloud | Claim/list/visibility is tested, but live run/step proxy proof is still weak. | `medium` | `internal/cloud/runtime_api.go`, `internal/cloud/runtime_api_test.go` | Yes | Phase 3 | WS-C1 Cloud |
| BG-010 | Cloud MCP streamable compatibility proof is incomplete | Planner/client integration | No direct proof for cloud SSE `GET`, `DELETE`, call alias, browser-origin handling, or SDK interop. | `medium` | `internal/cloud/mcp_server.go`, `internal/cloud/mcp_server_test.go`, `cmd/mcp-sdk-smoke` | Yes | Phase 4 | WS-P1 Planner/Clients |
| BG-011 | Nested broker module tests do not compile | Secondary surface | `cmd/broker` builds, but its own test suite is broken. | `medium` | `cmd/broker/task_test.go` | No | Phase 6 | WS-S1 Secondary |
| BG-012 | VS Code extension exists without meaningful repo proof | Secondary surface | Extension remains a code surface that is easy to overclaim. | `medium` | `extension/*` | No | Phase 6 | WS-S1 Secondary |
| BG-013 | Agent-broker task sessions are in-memory only | Secondary surface | Restart orphaning makes it unsafe to include in the beta promise. | `medium` | `cmd/broker/main.go`, `cmd/broker/README.md` | No | Phase 6 | WS-S1 Secondary |
| BG-014 | Release automation is manual-only | Release engineering | No repo CI pipeline is visible; public beta proof is still operator-driven. | `medium` | repo root, `Makefile`, scripts, missing `.github/` workflows | Yes | Phase 6 | WS-RE1 Release |
| BG-015 | Historical internal docs contradict current repo truth | Release engineering | Older docs still imply “no cloud” or overclaim completion/test stability. | `medium` | `docs/internal/*`, `docs/10_implementation_prompts.md` | Yes | Phase 6 | WS-RE1 Release |
| BG-016 | Policy schema does not match runtime policy model | Local core | Weakens any stable external policy contract claim. | `medium` | `schemas/policy.schema.json`, `internal/domain/policy.go`, `internal/service/policy_service.go` | No | Phase 1 | WS-L1 Local Core |
| BG-017 | Primary adapter proof is uneven beyond simulation | Local core | Codex/Claude/Qwen support is present, but release claims must stay narrow until proof is stronger. | `medium` | `internal/adapters/*`, `README.md`, smoke coverage | Yes | Phase 1 | WS-L1 Local Core |
| BG-018 | Legacy same-run local parity evidence is conflicting | Local core | One delegated run failed, while the direct Phase 0 rerun passed. The path needs deterministic repeatability before being promised. | `medium` | `scripts/smoke_test_v1.sh`, `internal/service/run_service.go`, `cmd/orchestratorctl/main.go` | No | Phase 1 | WS-L1 Local Core |
| BG-019 | Cloud docs overstate current smoke coverage for events | Docs / release truth | `docs/CLOUD.md` says `scripts/cloud_smoke.sh` covers events, but it does not. | `low` | `docs/CLOUD.md`, `scripts/cloud_smoke.sh` | No | Phase 6 | WS-RE1 Release |
| BG-020 | Linker environment still emits `grpc` search-path warnings | Release engineering | Builds succeed, but the build environment carries noisy warning output. | `low` | local toolchain / build env | No | Phase 6 | WS-RE1 Release |

## Phase 1 WS-C1 Update (2026-04-23)

This section records the cloud-control outcomes from the Phase 1 WS-C1 execution round without rewriting the historical Phase 0 table above.

| Gap | Status after WS-C1 | Evidence | Notes |
| --- | --- | --- | --- |
| BG-001 | `closed` | `TestTokenRevokeRequiresAuthorizedScopeAndRevokedTokenFailsAcrossCloudSurfaces`, `go test ./internal/cloud/... -count=1` | Revocation now loads the target token, enforces tenant scope against the target org/workspace/project, and revoked tokens fail both cloud HTTP and cloud MCP auth. |
| BG-002 | `closed` | `TestEventsListingRespectsTokenTenantScope`, `make cloud-smoke` | Event listing is now tenant-scoped even when `installation_id` is omitted, and foreign-installation reads are rejected. |
| BG-003 | `closed` | `internal/cloud/runtime_api_test.go`, composed cloud smoke with runtime HTTP | Nested runtime proxy routes now require the intended scopes (`steps:write`, `gates:read`, `runs:write`), and the relay bridge no longer widens caller scopes implicitly. |
| BG-004 | `closed` | `TestAuditListingRespectsProjectScope`, `go test ./internal/cloud/... -count=1` | Audit listing now respects workspace/project scope instead of only filtering at org level. |
| BG-009 | `closed` | composed cloud smoke with claimed runtime HTTP run/create + submit-task | Cloud runtime proof now includes a live claimed-instance path instead of claim/list-only coverage. |
| BG-010 | `closed` for cloud-side proof | `internal/cloud/mcp_server_test.go`, composed cloud smoke with `CLOUD_SMOKE_MCP=1 CLOUD_SMOKE_SDK=1` | Cloud MCP now has direct proof for SSE `GET`, `DELETE`, call alias, origin handling, session ownership, revoked-token denial, and official Go SDK interop. Broader planner/client release labeling still belongs to WS-P1. |
| BG-019 | `closed` | `docs/CLOUD.md`, `docs/CLOUD_SELF_HOST.md`, `scripts/cloud_smoke.sh` | Public docs now match the actual smoke coverage for events, runtime HTTP, cloud MCP, and optional SDK proof. |

Cloud-adjacent items that remain outside WS-C1:

- `make cloud-stack-smoke` is still required on a Docker-capable host for packaging/deployment proof and stays with WS-RE1.
- Broader planner/client compatibility wording still belongs to WS-P1 even though the cloud-side MCP/SDK proof is now present.

## Phase 2 WS-R1 Update (2026-04-23)

This section records the relay/runtime outcomes from the Phase 2 WS-R1 execution round without rewriting the historical Phase 0 table above.

| Gap | Status after WS-R1 | Evidence | Notes |
| --- | --- | --- | --- |
| BG-005 | `closed` | `TestShareInstanceByInstanceIDRequiresResolvableLocalInstance`, `TestRelayConnectorUnsharePrunesRoutabilityAndReshareRestoresIt`, `SMOKE_SCENARIOS=status,audit,share-control,mcp,mcp-sdk ./scripts/self_host_smoke.sh` | `share --instance-id` now refuses to persist `share=true` unless the selector resolves back to a healthy local daemon. Unshare removes relay visibility and routing; re-share by `instance_id` restores both. |

Additional relay/runtime reductions landed during WS-R1:

- Relay MCP now carries the authenticated planner principal directly for internal route calls, which closes the duplicate-token-name scope/instance replay bug on the relay MCP path.
- Connector heartbeat handling now re-advertises when the live reachable shared-instance set changes even without a config edit, which keeps relay presence aligned when a shared daemon drops out and later returns.
- Hub pruning now removes stale per-instance mappings for a replaced connector session instead of waiting for TTL or read-loop teardown.
- Public self-host docs and smoke entrypoints now describe the real proof boundary for relay HTTP, relay MCP, official Go SDK interop, share-control, and multi-instance targeting.

Relay/runtime items that remain outside WS-R1:

- Broader planner/client compatibility freezing still belongs to WS-P1 even though relay-side HTTP/MCP/SDK proof is now materially stronger.
- Full release packaging and clean-checkout repeatability still belong to WS-RE1.

## Phase 3 WS-L1 Update (2026-04-23)

This section records the local-core outcomes from the Phase 3 WS-L1 execution round without rewriting the historical Phase 0 table above.

| Gap | Status after WS-L1 | Evidence | Notes |
| --- | --- | --- | --- |
| BG-018 | `closed` | `go test ./internal/app ./internal/service ./cmd/orchestratorctl -count=1`, `./scripts/smoke_test_v1.sh` x2, `make smoke` | WS-L1 fixed the local same-run wait/finalization race by making local `step wait` follow persisted step lifecycle state before returning the result payload. The legacy six-input smoke now auto-starts a temporary simulation daemon when needed and repeated cleanly twice. |
| BG-017 | `closed` as a beta-claim blocker | `README.md`, `docs/SETUP.md`, `docs/internal/BETA_SUPPORT_CLASSIFICATION.md`, `go test ./internal/adapters/... ./internal/mcp -count=1` | WS-L1 did not fabricate live-adapter proof. Instead it froze the local adapter table truthfully: `codex` remains simulation-heavy, `claude` keeps narrow wrapper claims, `qwen` stays secondary, and daemon-local `/mcp/call` remains compatibility-only. |

Additional local-core reductions landed during WS-L1:

- `GetResultByStep` now overlays post-attempt step lifecycle truth, so gated, rejected, and manual-attention steps no longer report stale “completed” results.
- `RetryStep` now reconciles the parent run back to `running` immediately, and `orchestratorctl step retry` exposes the existing local retry route cleanly.
- Public local docs now describe `/api/v1/compatibility` as a diagnostic surface rather than a support certificate and document the exact local smoke proof entrypoints.

Local-core items that remain outside WS-L1:

- BG-016 still remains as a non-beta-blocking schema/runtime policy drift item unless a later phase chooses to promote the external policy contract.
- Repo-wide release/public repeatability still belongs to WS-RE1 even though the local proof matrix is now materially stronger.

## Phase 4 WS-P1 Update (2026-04-23)

This section records the planner/client outcomes from the Phase 4 WS-P1 execution round without rewriting the historical Phase 0 table above.

| Gap | Status after WS-P1 | Evidence | Notes |
| --- | --- | --- | --- |
| BG-010 | `closed` end-to-end for planner/client release labeling | `go test ./internal/relay ./internal/cloud ./cmd/mcp-sdk-smoke -count=1`, `RELAY_CONFIG=... SMOKE_SCENARIOS=status,audit,share-control,mcp,mcp-sdk ./scripts/self_host_smoke.sh`, `CLOUD_RELAY_CONFIG=... CLOUD_RUNTIME_DAEMON_URL=http://127.0.0.1:8085 CLOUD_SMOKE_MCP=1 CLOUD_SMOKE_SDK=1 ./scripts/cloud_smoke.sh`, `docs/mcp/integrations.md`, `docs/mcp/cloud_tools.md` | WS-C1 had already closed the cloud-side code/proof gap. WS-P1 closed the remaining planner/client blocker by freezing the public/internal compatibility matrix, adding cloud MCP packaging parity, and keeping ChatGPT/Claude product paths at compatibility-only instead of overclaiming direct proof. |

Additional planner/client reductions landed during WS-P1:

- The public planner/client contract now names relay `/mcp` and cloud `/api/cloud/v1/mcp` as the canonical remote MCP session paths and pushes the `*/mcp/call` endpoints down to compatibility-only aliases.
- Public docs now expose repo-proven generic relay/cloud HTTP and MCP entrypoints, plus checked-in Claude Code style `.mcp.json` examples for local tester packaging.
- Internal support labels now distinguish `proven`, `expected-only`, and `compatibility-only` planner/client paths instead of treating all product-style integrations as vague docs-only expectations.

Planner/client items that remain outside WS-P1:

- Product-specific ChatGPT, Claude Code, Claude Desktop, Claude.ai, or Anthropic/OpenAI API publication workflows remain compatibility-only and are not direct repo proof.
- Repo-wide release/public repeatability still belongs to WS-RE1 even though the planner/client proof matrix is now materially stronger.

## Phase 5 WS-PC1 Update (2026-04-23)

This section records the provider-connector outcomes from the Phase 5 WS-PC1 execution round without rewriting the historical Phase 0 table above.

| Gap | Status after WS-PC1 | Evidence | Notes |
| --- | --- | --- | --- |
| BG-006 | `closed` | `TestStoreCreateConnectorEventPreservesRepeatedSourceEventHistory`, `TestWebhookHistoryPreservesRepeatedSourceEventIDs`, `go test ./internal/cloud/... -count=1` | Connector event history is now append-only. Migration 4 rebuilds `connector_events` without the overwrite-on-conflict constraint, and webhook/poll ingests now keep provider metadata instead of silently replacing prior rows. |
| BG-007 | `closed` | `TestJiraWebhookRouteReturnsDeferredWithoutPersistingEvents`, `go test ./internal/cloud -run 'Test(ServerAdminAndConnectorFlows|WebhookHistoryPreservesRepeatedSourceEventIDs|JiraWebhookRouteReturnsDeferredWithoutPersistingEvents|ConnectorActionLogsCaptureRequestCompletionAndAuditDetails|WorkerRunOncePollsJiraAndPersistsSnapshot|StoreCreateConnectorEventPreservesRepeatedSourceEventHistory)' -count=1` | Jira remains polling-first. Routed webhook calls now return a truthful deferred/not-implemented response, do not persist events, and do not emit false-positive success audit rows. |
| BG-008 | `closed` | `TestConnectorActionLogsCaptureRequestCompletionAndAuditDetails`, `go test ./internal/cloud/... -count=1` | Provider action logs now capture request payloads, response payloads, started/completed timestamps, and richer audit details. Webhook verification/deferment/normalize failures now also create explicit audit rows. |

Additional provider truth frozen during WS-PC1:

- Slack remains the strongest provider path with real install/bootstrap/local-test proof through routed tests and `make cloud-smoke`.
- GitHub, GitLab, and Linear remain intentionally narrow operator/package surfaces even though validation and action code paths are directly proven in unit tests.
- Jira remains polling-first only; action-only or webhook-driven Jira installs are not part of the beta promise.

Provider items that remain outside WS-PC1:

- live vendor-account proof for every provider remains out of scope for the current beta promise
- provider-specific end-to-end smoke beyond Slack remains optional future depth, not a current beta blocker
- a public API for listing connector action logs is still not part of the current beta contract

## Phase 6 WS-RE1 Update (2026-04-23)

This section records the release-engineering and public-testability outcomes from the Phase 6 WS-RE1 execution round without rewriting the historical Phase 0 table above.

| Gap | Status after WS-RE1 | Evidence | Notes |
| --- | --- | --- | --- |
| BG-014 | `closed` | `.github/workflows/public-testability.yml`, `make verify-beta`, detached temporary `git worktree` rerun of `make build-supported && make verify-beta` | The repo now has a visible CI workflow plus explicit supported verification targets and a clean-checkout-friendly verification script. |
| BG-015 | `closed` | `docs/internal/BETA_*.md`, `docs/10_implementation_prompts.md`, `docs/internal/GAP_AUDIT.md`, `docs/internal/PROGRESS.md`, `docs/internal/TASKS.md`, `docs/internal/IMPLEMENTATION_PLAN.md`, `docs/internal/cloud_v1_finish_log.md`, `docs/internal/v2_finish_log.md` | Frozen beta docs remain the current program truth, and older planning / backlog documents are now marked as historical instead of competing with current release guidance. |
| BG-020 | `still open` | `make build-supported`, `make verify-beta`, detached temporary `git worktree` rerun | Builds and tests stay green, but the local linker environment still emits `/usr/local/opt/grpc/lib` search-path warnings. This remains noisy rather than beta-blocking. |

Additional WS-RE1 reductions landed in this round:

- `make verify-beta` is now a real repo-level verification command that self-starts the temporary relay/runtime proof instead of assuming hidden operator setup.
- A new public tester guide freezes the supported tracks and exact commands in `docs/BETA_TESTING.md`.
- Docker cloud image version metadata is now parameterized through compose/build args instead of living only as a hard-coded string in the Dockerfile.
- Public README/setup/cloud/self-host docs now route testers to the correct track and state the real proof boundary for Docker baseline vs binary-native composed proof.

Historical handoff items after WS-RE1:

- final repo-wide beta confirmation had to rerun the frozen matrix once more without widening scope
- Docker-backed packaging proof still depended on a Docker-capable host before the repo could claim it as directly re-verified in that finalization run

## Phase 7 Final Confirmation Update (2026-04-23)

Final beta confirmation reran the frozen matrix and closed the last repo-wide proof item:

- `make build-supported` passed
- `./scripts/smoke_test_v1.sh` passed twice, and `make smoke` passed
- a fresh self-host smoke with `status,audit,share-control,multi-instance,mcp,mcp-sdk` passed
- `go test ./internal/cloud/... -count=1` passed
- `go test ./internal/relay ./internal/cloud ./cmd/mcp-sdk-smoke -count=1` passed
- `make cloud-smoke` passed
- the composed cloud smoke with `CLOUD_RELAY_CONFIG=... CLOUD_RUNTIME_DAEMON_URL=http://127.0.0.1:18085 CLOUD_SMOKE_MCP=1 CLOUD_SMOKE_SDK=1` passed
- `make verify-beta` passed
- the supported verification reran successfully from a detached temporary `git worktree` at the current `HEAD` via `make build-supported && make verify-beta`
- `make cloud-stack-smoke` passed on a host with a live Docker daemon
- the final-tree `make verify-beta-docker` rerun passed after the Phase 7 truth-normalization updates landed
- local `/usr/local/opt/grpc/lib` linker warnings still appeared during build-oriented steps, but stayed non-blocking

Current blocker truth after Phase 7:

- no beta-blocking gaps remain open
- BG-020 remains open as non-blocking build-noise only
- BG-011, BG-012, BG-013, and BG-016 remain outside the beta promise unless later promoted
