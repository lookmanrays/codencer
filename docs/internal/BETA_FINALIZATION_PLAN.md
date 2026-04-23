# Beta Finalization Plan

Status: Phase 0 freeze document

Last audited: 2026-04-23

Source of truth: current repository code, tests, smoke runs, and build surfaces. Older internal docs are historical unless they still match code.

## Current State

- Current release truth is `v0.2.0-beta`.
- The repo is beta-confirmed as of 2026-04-23.
- The repo is already larger than the older local-only plan. It now contains:
  - local daemon + CLI + storage + worktree execution
  - self-host relay + runtime connector + relay MCP
  - self-host cloud control-plane beta track
  - provider connector platform
  - secondary IDE/broker surfaces

## Repo Truth Rules

- Code beats docs when they disagree.
- Tests and smoke runs beat older summaries when they disagree.
- Out-of-beta-scope does not imply deletion.
- Secondary or compatibility surfaces can stay in tree if they are classified truthfully.
- Beta must be a contract, not a hope.

## Target Beta Definition

A truthful beta means all of the following are true at the same time:

1. The local daemon + CLI path is stable, documented, and publicly testable.
2. The self-host relay + connector path is stable, documented, and publicly testable over narrow HTTP and MCP contracts.
3. The self-host cloud control-plane path is stable, scoped correctly, and publicly testable for bootstrap, tenancy, runtime claim/list/proxy, and audit.
4. Planner/client compatibility claims are narrow and proven. No universal client claims.
5. Provider connector claims are narrow and proven. No vendor-depth completeness claims.
6. Public docs, internal docs, version strings, smoke commands, and support labels all agree.

## Supported Beta Tracks

These are the tracks the repo should finalize for beta:

1. Local runtime track
   - `orchestratord`
   - `orchestratorctl`
   - SQLite ledger
   - worktree/provisioning/artifact/validation flow
   - simulation-mode smoke
   - narrow primary adapter promise
2. Self-host relay/runtime track
   - `codencer-relayd`
   - `codencer-connectord`
   - connector enrollment/session/share/status/discover
   - relay HTTP planner surface
   - relay MCP public surface
   - official Go SDK interoperability for relay MCP
3. Self-host cloud track
   - `codencer-cloudd`
   - `codencer-cloudctl`
   - `codencer-cloudworkerd`
   - org/workspace/project/membership/token/install/event/audit flows
   - runtime connector claim/list/instance visibility
   - tenant-scoped runtime HTTP/MCP proxying in composed mode
4. Planner/client integration track
   - generic relay HTTP client path
   - generic relay MCP client path
   - official Go SDK path to relay MCP
   - generic cloud HTTP client path
   - generic cloud MCP path in composed mode
5. Provider connector track
   - GitHub
   - GitLab
   - Jira
   - Linear
   - Slack
   - narrow validate/ingest/action/status claims only

## Excluded But Kept

These can remain in repo without being part of the beta promise:

- VS Code extension
- `agent-broker`
- `ide-chat` adapter
- `qwen` adapter
- `openclaw-acpx` adapter
- `antigravity` and `antigravity-broker` as primary beta paths
- daemon-local `/mcp/call`
- relay `/mcp/call` and cloud `/api/cloud/v1/mcp/call` beyond compatibility claims
- public SaaS UI
- enterprise IAM / SSO
- billing
- universal client compatibility claims
- hard kill / hard cancellation claims
- cloud-native runtime enrollment lifecycle
- vendor-depth provider automation completeness

## Remaining Phases After Phase 0

### Phase 1: Local Core Finalization

Goal:
- Freeze the canonical local beta promise and primary adapter set.

Exit criteria:
- Local smoke remains green.
- Legacy six-input local submission proof is either repeatably green or explicitly downgraded from beta promise.
- Policy/schema contract drift is resolved or excluded from beta promise.
- Primary adapter support table is explicit and truthful.

### Phase 2: Relay + Runtime Connector Finalization

Goal:
- Freeze the self-host relay/connector contract.

Exit criteria:
- Share/unshare behavior is deterministic and truthful.
- Self-host HTTP + MCP + SDK smoke paths are first-class and documented.
- Connector status/discover/share/unshare docs match real behavior.
- Relay/connector auth, scope, and route behavior remain green under tests.

### Phase 3: Cloud Self-Host Finalization

Goal:
- Make the cloud control plane safe enough to claim beta for self-host use.

Exit criteria:
- Tenant scope leaks are fixed.
- Token, event, audit, and runtime route authorization is correct.
- Runtime proxy paths are exercised in tests and smoke.
- Cloud docs stay narrow and honest.

### Phase 4: Planner / Client Integration Finalization

Goal:
- Freeze planner/client compatibility claims.

Exit criteria:
- Relay HTTP, relay MCP, and relay official SDK claims are explicitly proven.
- Cloud HTTP/MCP compatibility claims are either proven or narrowed.
- Local daemon MCP remains compatibility-only and is labeled that way everywhere.
- No product-specific ChatGPT/Claude overclaims remain.

### Phase 5: Provider Connector Finalization

Goal:
- Promote a narrow provider matrix to beta-ready truth.

Exit criteria:
- Jira webhook deferment is enforced or implemented honestly.
- Event history and action logging are fit for audit.
- Per-provider mock smoke coverage exists for validate/ingest/action/status.
- Provider docs match code exactly.

### Phase 6: Release Engineering / Public Testability

Goal:
- Make the beta verifiable by outsiders without tribal knowledge.

Exit criteria:
- Build/test/smoke commands are stable.
- Docker compose baseline is validated and smoke-tested.
- Historical internal docs are no longer used as current release truth.
- Secondary surfaces are either fixed or clearly excluded from beta.

### Phase 7: Beta Confirmation

Goal:
- Confirm beta without adding new scope.

Exit criteria:
- Every beta-blocker gap is closed or explicitly removed from the beta promise.
- Final verification matrix is green for the beta tracks.
- README, CHANGELOG, Makefile versioning, and support matrix agree.
- Repo state is still stable after a full confirmation pass.

## Beta Confirmation Criteria

Beta can be claimed only when:

- `go test ./...` passes in the main module.
- beta-track smoke commands pass from documented entrypoints.
- cloud scope bugs and relay/share correctness gaps are closed.
- provider connector claims are narrowed to what is actually proven.
- public docs and internal beta docs agree on the same support contract.
- remaining secondary surfaces are clearly labeled as non-beta.

## Release Readiness Criteria

Release-readiness for the beta merge requires:

- truthful version string
- truthful changelog entry
- stable build commands
- stable smoke commands
- no known P1 security/scope gaps in beta-track surfaces
- explicit support/classification matrix
- explicit gap register
- explicit workstream/ownership plan
- explicit verification matrix

## Phase 0 Outcome

- Phase 0 does not declare beta.
- Phase 0 does define the beta contract and the remaining work.
- Current repo state remained alpha until later phases closed the registered blockers and the final confirmation pass reran the frozen matrix.

## Phase 1 WS-C1 Outcome (2026-04-23)

- WS-C1 closed the cloud scope/security blockers from Phase 0: token-revocation scope, event scoping, audit scoping, and runtime HTTP under-enforcement.
- WS-C1 also closed the thin-proof gaps for cloud runtime HTTP, cloud MCP streamable behavior, and official Go SDK access to `/api/cloud/v1/mcp`.
- Public cloud docs and smoke entrypoints were updated so the documented proof now matches the actual repo behavior.
- WS-C1 does not promote the whole repo to beta. Remaining relay, local, provider, release, and broader planner/client work still has to finish before beta confirmation.
- Docker-based packaging proof remains a later release-engineering task because `make cloud-stack-smoke` still depends on a Docker-capable host.

## Phase 2 WS-R1 Outcome (2026-04-23)

- WS-R1 closed the Phase 0 relay/runtime blocker around `share --instance-id` persisting false-positive shared state.
- WS-R1 also hardened relay/runtime correctness around live-set re-advertise, stale connector-session pruning, and relay MCP principal parity.
- Public relay/connector/self-host docs and smoke entrypoints were updated so the documented proof now matches the actual self-host relay/runtime behavior.
- WS-R1 does not promote the whole repo to beta. Local-core proof, broader planner/client freezing, provider work, and release verification still remain before beta confirmation.

## Phase 3 WS-L1 Outcome (2026-04-23)

- WS-L1 closed the conflicting local parity evidence from Phase 0 by fixing the local same-run wait/finalization race and re-running the legacy six-input smoke twice successfully.
- WS-L1 also hardened local truth around step results and retry lifecycle: gated/rejected/manual-attention states now surface correctly in `step result`, and retry moves the parent run back to `running` immediately.
- Public local docs and internal support labels were updated so the local adapter matrix, compatibility surfaces, and local smoke entrypoints now match the actual repo proof.
- WS-L1 does not promote the whole repo to beta. Planner/client freezing, provider work, release/public repeatability, and final beta confirmation still remain before beta can be claimed.

## Phase 4 WS-P1 Outcome (2026-04-23)

- WS-P1 froze the public and internal planner/client compatibility matrix against the current repo truth instead of historical expectations.
- Relay HTTP, relay MCP, cloud HTTP, cloud MCP, and official Go SDK access to relay/cloud MCP are now explicitly documented as proven within narrow repo-exercised scope.
- Generic MCP clients remain expected-only, while ChatGPT-style and Claude-style paths remain compatibility-only; the local daemon stays out of the public remote MCP promise.
- Public planner/client docs now include a cloud MCP tools page, generic HTTP/MCP examples, and checked-in Claude Code style `.mcp.json` examples for local tester packaging.
- WS-P1 does not promote the whole repo to beta. Provider connector finalization, release/public repeatability, and final beta confirmation still remain before beta can be claimed.

## Phase 5 WS-PC1 Outcome (2026-04-23)

- WS-PC1 closed the provider connector beta blockers from Phase 0: repeated webhook history overwrite, Jira webhook deferment drift, and incomplete provider action/audit logging.
- Connector event history is now append-only instead of overwrite-on-conflict, and webhook/poll ingests persist enough metadata to reconstruct provider deliveries more truthfully in the cloud store.
- Jira remains polling-first by design, and the routed webhook surface now rejects Jira webhook calls truthfully instead of ingesting them as if they were supported.
- Provider action logs now persist request payloads, response payloads, and completion timestamps, and provider webhook failure/deferment paths now leave explicit audit evidence.
- Public provider docs and internal support matrices now freeze the provider matrix to narrow, code-backed claims: Slack is the strongest local tester path, while GitHub, GitLab, Jira, and Linear remain intentionally narrower operator/package surfaces.
- WS-PC1 does not promote the whole repo to beta. Release/public repeatability and final beta confirmation still remain before beta can be claimed.

## Phase 6 WS-RE1 Outcome (2026-04-23)

- WS-RE1 added explicit repo-level public verification entrypoints: `make build-supported`, `make verify-beta`, and `make verify-beta-docker`.
- WS-RE1 also added a visible CI workflow under `.github/workflows/public-testability.yml` that mirrors the supported non-Docker public verification path.
- Public tester routing is now explicit: README, setup, relay, cloud, planner/client, and provider docs all point to the right track instead of forcing testers to infer the repo promise from scattered pages.
- A new public tester guide (`docs/BETA_TESTING.md`) now freezes the supported track matrix, exact commands, and the current support boundaries in one place.
- Deployment packaging truth is tighter: the Docker cloud image now takes its version string from compose/build args instead of only a hard-coded Dockerfile literal, and the docs now distinguish Docker baseline proof from binary-native composed proof.
- WS-RE1 completed a full supported non-Docker verification pass from the active checkout and repeated that same pass from a clean-ish temporary checkout copy.
- WS-RE1 still does not promote the whole repo to beta. The remaining repo-wide step is final beta confirmation, including a rerun of Docker-backed stack smoke on a Docker-capable host.

## Phase 7 Beta Confirmation Outcome (2026-04-23)

- The frozen beta verification matrix was rerun from the working tree and from a clean-ish detached worktree overlay of the current repo contents.
- `make build-supported`, `make verify-beta`, and `make cloud-stack-smoke` all passed during the final confirmation pass.
- Docker-backed cloud-stack proof was re-executed on a host with a live Docker daemon after starting Docker Desktop in this environment.
- The repo status, version strings, and public tester docs now agree on `v0.2.0-beta`.
- Beta is confirmed for the supported local, self-host relay/runtime, self-host cloud, planner/client integration, and provider connector tracks, with the previously frozen compatibility/deferred boundaries unchanged.
