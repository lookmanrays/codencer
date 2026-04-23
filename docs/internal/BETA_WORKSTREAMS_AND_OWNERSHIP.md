# Beta Workstreams And Ownership

This document defines the merge-safe work split for the post-Phase-0 program.

## Workstream Map

| Workstream | Goal | Primary future owner | File ownership boundary | Merge order | Conflict notes |
| --- | --- | --- | --- | --- | --- |
| WS-L1 Local Core Finalization | Freeze the local daemon/CLI beta contract. | Local core lead | `cmd/orchestratord`, `cmd/orchestratorctl`, `internal/app`, `internal/service`, `internal/state`, `internal/storage/sqlite`, `internal/workspace`, `schemas`, local smoke scripts | 3 | Do not mix relay/cloud scope fixes into this stream. |
| WS-R1 Relay + Connector Finalization | Freeze self-host remote runtime contract. | Relay lead | `cmd/codencer-relayd`, `cmd/codencer-connectord`, `internal/relay`, `internal/connector`, `docs/RELAY.md`, `docs/CONNECTOR.md`, `docs/SELF_HOST_REFERENCE.md`, `scripts/self_host_smoke.sh` | 2 | Owns share/discover/status semantics. |
| WS-C1 Cloud Control Plane Finalization | Fix cloud scope/security bugs and runtime proxy proof. | Cloud lead | `cmd/codencer-cloudd`, `cmd/codencer-cloudctl`, `cmd/codencer-cloudworkerd`, `internal/cloud`, `deploy/cloud`, `docs/CLOUD.md`, `docs/CLOUD_SELF_HOST.md` | 1 | No provider connector code edits here unless coordinated with WS-PC1. |
| WS-P1 Planner / Client Integration Finalization | Freeze client compatibility claims and proofs. | Planner/client lead | `internal/relay/mcp_*`, `internal/cloud/mcp_*`, `internal/mcp`, `cmd/mcp-sdk-smoke`, `docs/mcp/*` | 4 | Touch relay/cloud protocol tests, not their auth/business logic, unless coordinated. |
| WS-PC1 Provider Connector Finalization | Make the connector platform narrowly beta-ready. | Provider platform lead | `internal/cloud/connectors/*`, `internal/cloud/worker.go`, `docs/CLOUD_CONNECTORS.md`, provider fixtures | 5 | Avoid editing `internal/cloud/router.go` concurrently with WS-C1 unless a shared API change is agreed first. |
| WS-RE1 Release Engineering / Public Testability | Make the repo externally verifiable. | Release lead | `README.md`, `CHANGELOG.md`, `Makefile`, `docs/internal/*`, smoke scripts, release notes, packaging docs | 6 | Prefer landing after functional work stabilizes. |
| WS-S1 Secondary / Compatibility Surfaces | Keep secondary surfaces truthful and non-blocking. | Secondary surfaces lead | `cmd/broker`, `extension`, `internal/adapters/ide`, `internal/adapters/antigravity`, `internal/adapters/openclaw_acpx`, `internal/adapters/qwen` | 7 | Read-only until primary beta blockers are cleared unless a bug is actively harmful. |

## Subagent Assignment Pattern

Future multi-agent implementation rounds should use a bounded split like this:

- `Agent LC-A`: local core contracts, smoke, schema, and adapter support table.
- `Agent RC-A`: relay/connector auth, share-control, and self-host smoke.
- `Agent CL-A`: cloud scope/security fixes and runtime proxy proof.
- `Agent PI-A`: relay/cloud MCP compatibility and SDK proof.
- `Agent PC-A`: provider eventing, action logs, Jira deferment, provider smoke.
- `Agent RE-A`: public docs, changelog, release checklist, packaging proof.
- `Agent SX-A`: broker/extension/secondary cleanup only after primary streams settle.

## Merge Order

1. WS-C1
   - close security and scope leaks first
2. WS-R1
   - tighten self-host runtime correctness
3. WS-L1
   - finalize the local contract once remote blockers are not changing shared semantics
4. WS-P1
   - freeze client compatibility claims on top of stable relay/cloud behavior
5. WS-PC1
   - finalize provider connector claims after cloud tenancy and audit semantics settle
6. WS-RE1
   - align public docs, smoke targets, and release material
7. WS-S1
   - handle secondary surfaces without blocking beta tracks

## Merge Discipline

- No two write-capable workers should edit `internal/cloud/router.go` or `internal/cloud/runtime_api.go` at the same time.
- No two write-capable workers should edit `internal/relay/*` and `scripts/self_host_smoke.sh` at the same time.
- `README.md` and `CHANGELOG.md` should be owned by WS-RE1 only after the support matrix is frozen.
- `extension/*` and `cmd/broker/*` should stay read-only until the primary beta tracks are finished, unless a defect is actively harmful.
- Provider connector workers should stay inside `internal/cloud/connectors/*` unless they have an approved shared API change with WS-C1.

## Non-Removal Rule For Later Phases

- Secondary or excluded surfaces are not deletion targets by default.
- Remove code only if it is dead, dangerous, or actively misleading and not worth preserving.
- Prefer classification plus truthful docs over cleanup-by-deletion.

## WS-C1 Status Update (2026-04-23)

WS-C1 is complete for code, focused tests, and binary-native smoke:

- closed BG-001, BG-002, BG-003, BG-004, BG-009, BG-010, and BG-019
- proved cloud runtime HTTP over a claimed runtime connector in composed mode
- proved cloud MCP streamable behavior and official Go SDK interoperability in composed mode
- updated the public cloud docs to match the actual smoke and proof entrypoints

Remaining work that still touches cloud but is not a WS-C1 code blocker:

- `make cloud-stack-smoke` still needs to run on a Docker-capable host and remains release-engineering/package proof work
- broader planner/client release wording still belongs to WS-P1 even though the cloud-side MCP/SDK proof is now present

Recommended handoff after WS-C1:

1. continue with WS-R1 per the frozen merge order
2. keep `internal/cloud/*` read-only unless a later workstream discovers a concrete cloud regression

## WS-R1 Status Update (2026-04-23)

WS-R1 is complete for code, focused tests, and self-host smoke:

- closed BG-005
- fixed relay MCP principal replay so internal MCP route calls keep the authenticated planner identity
- hardened connector live-set presence so reachable shared-instance changes trigger a fresh advertise even without a config edit
- tightened stale connector-session pruning in the relay hub
- updated self-host smoke and public docs to match the actual relay/runtime proof boundary

Proof landed in this round:

- `go test ./internal/connector ./internal/relay ./cmd/codencer-connectord -count=1`
- `go test ./cmd/mcp-sdk-smoke -count=1`
- self-host smoke with `status,audit,share-control,mcp,mcp-sdk`
- self-host smoke with `status,audit,share-control,multi-instance,mcp,mcp-sdk`

Remaining work that still touches relay/runtime but is not a WS-R1 blocker:

- broader planner/client compatibility freezing still belongs to WS-P1
- release packaging and clean-checkout repeatability still belong to WS-RE1

Recommended handoff after WS-R1:

1. continue with WS-L1 or WS-P1 per the frozen merge order and current release needs
2. keep `internal/relay/*` and `internal/connector/*` read-only unless a later workstream discovers a concrete regression

## WS-L1 Status Update (2026-04-23)

WS-L1 is complete for code, focused tests, and local smoke:

- closed BG-018 by fixing the local same-run wait/finalization race and re-proving the legacy six-input smoke twice
- closed BG-017 as a beta-claim blocker by freezing the local adapter support table to the actual repo proof boundary
- aligned local `step result`, `step wait`, and `step retry` with the persisted local lifecycle truth
- updated local public docs so `/api/v1/compatibility`, daemon-local MCP, and the local smoke entrypoints are described honestly

Proof landed in this round:

- `go test ./internal/app ./internal/service ./cmd/orchestratorctl -count=1`
- `go test ./internal/adapters/... ./internal/mcp -count=1`
- `bash -n scripts/smoke_test_v1.sh`
- `./scripts/smoke_test_v1.sh` (twice)
- `make smoke`

Remaining work that still touches local-adjacent truth but is not a WS-L1 blocker:

- BG-016 policy schema/runtime drift remains outside the current beta promise unless later promoted
- repo-wide release/public repeatability still belongs to WS-RE1

Recommended handoff after WS-L1:

1. continue with WS-P1 per the frozen merge order
2. keep `internal/app/*`, `internal/service/*`, and local smoke scripts read-only unless a later workstream discovers a concrete local regression

## WS-P1 Status Update (2026-04-23)

WS-P1 is complete for planner/client contract freezing, focused verification, and public packaging docs:

- closed the remaining planner/client blocker around ambiguous compatibility wording after BG-010's cloud-side proof had already landed in WS-C1
- froze the public and internal planner/client matrix to `proven`, `expected-only`, `compatibility-only`, and `unsupported` claims that match current code and smoke
- added cloud MCP packaging parity with a dedicated public tools page plus generic HTTP/MCP examples
- added checked-in Claude Code style `.mcp.json` examples while keeping ChatGPT-style and Claude-style product integrations at compatibility-only

Proof landed in this round:

- `go test ./internal/relay ./internal/cloud ./cmd/mcp-sdk-smoke -count=1`
- `RELAY_CONFIG=... SMOKE_SCENARIOS=status,audit,share-control,mcp,mcp-sdk ./scripts/self_host_smoke.sh`
- `CLOUD_RELAY_CONFIG=... CLOUD_RUNTIME_DAEMON_URL=http://127.0.0.1:8085 CLOUD_SMOKE_MCP=1 CLOUD_SMOKE_SDK=1 ./scripts/cloud_smoke.sh`
- `python3 -m json.tool docs/mcp/examples/claude-code-relay.mcp.json`
- `python3 -m json.tool docs/mcp/examples/claude-code-cloud.mcp.json`

Remaining work that still touches planner/client-adjacent truth but is not a WS-P1 blocker:

- product-specific ChatGPT, Claude Code, Claude Desktop, Claude.ai, or Anthropic/OpenAI API publication workflows remain compatibility-only unless a later release phase chooses to exercise them directly
- repo-wide release/public repeatability still belongs to WS-RE1

Recommended handoff after WS-P1:

1. continue with WS-PC1 per the frozen merge order
2. keep `internal/relay/mcp_*`, `internal/cloud/mcp_*`, `cmd/mcp-sdk-smoke`, and `docs/mcp/*` read-only unless a later workstream discovers a concrete regression

## WS-PC1 Status Update (2026-04-23)

WS-PC1 is complete for provider code, focused tests, and public/internal truth freezing:

- closed BG-006, BG-007, and BG-008
- rebuilt connector event storage as append-only history instead of overwrite-on-conflict
- enforced Jira webhook deferment truthfully at the routed webhook surface
- enriched provider action logs and audit details so routed provider operations are attributable enough for beta testing
- updated the public provider docs and internal beta matrices to match the narrow provider matrix now proven by the repo

Proof landed in this round:

- `go test ./internal/cloud/... -count=1`
- `go test ./internal/cloud/connectors -count=1`
- `go test ./internal/cloud -run 'Test(ServerAdminAndConnectorFlows|WebhookHistoryPreservesRepeatedSourceEventIDs|JiraWebhookRouteReturnsDeferredWithoutPersistingEvents|ConnectorActionLogsCaptureRequestCompletionAndAuditDetails|WorkerRunOncePollsJiraAndPersistsSnapshot|StoreCreateConnectorEventPreservesRepeatedSourceEventHistory)' -count=1`
- `make cloud-smoke`

Remaining work that still touches provider-adjacent truth but is not a WS-PC1 blocker:

- live vendor-account proof for every provider remains outside the current beta promise
- provider-specific end-to-end smoke beyond Slack remains optional future depth, not a current blocker
- repo-wide release/public repeatability still belongs to WS-RE1

Recommended handoff after WS-PC1:

1. continue with WS-RE1 per the frozen merge order
2. keep `internal/cloud/connectors/*`, `internal/cloud/worker.go`, and provider docs read-only unless a later workstream discovers a concrete regression

## WS-RE1 Status Update (2026-04-23)

WS-RE1 is complete for public tester routing, repo-level verification entrypoints, CI visibility, and non-Docker repeatability:

- closed BG-014 by adding a visible GitHub Actions workflow for the supported public verification path
- closed BG-015 by making the frozen beta docs the current release truth and marking older planning/backlog docs as historical
- added `make build-supported`, `make verify-beta`, and `make verify-beta-docker` as explicit repo-level verification commands
- added `scripts/verify_beta.sh` so the supported non-Docker verification path self-starts the temporary relay/runtime proof instead of assuming hidden setup
- updated public docs so local, relay/runtime, cloud, planner/client, and provider testers route to the correct track immediately

Proof landed in this round:

- `make build-supported`
- `make verify-beta`
- `make build-supported && make verify-beta` from a clean-ish temporary checkout copy
- `make cloud-stack-config`

Remaining work that still touches release/public proof but is not a WS-RE1 blocker:

- final beta confirmation still has to rerun the frozen matrix one more time without widening scope
- `make cloud-stack-smoke` still needs a Docker-capable host before Docker-backed packaging proof can be re-confirmed directly

Recommended handoff after WS-RE1:

1. continue with Phase 7 beta confirmation only
2. keep the beta-track docs, verify targets, and CI workflow aligned with the frozen matrix during final confirmation

## Phase 7 Status Update (2026-04-23)

Phase 7 is complete:

- reran the frozen beta matrix from the working tree
- reran the supported verification from a clean-ish detached worktree overlay
- reran `make cloud-stack-smoke` successfully on a live Docker daemon host
- confirmed the repo as `v0.2.0-beta`

Program status after beta confirmation:

- the primary beta workstreams are complete
- future work should be treated as post-beta maintenance, deferred-surface cleanup, or deeper proof beyond the current beta promise
