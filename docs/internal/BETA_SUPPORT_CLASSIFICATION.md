# Beta Support Classification

This matrix separates current repo truth from the frozen beta contract.

This is a final audited classification snapshot after Phase 7 beta confirmation. The beta contract labels below are frozen release truth, not forward-looking intent.

Phase 7 note:

- Beta was confirmed on 2026-04-23.
- Every surface labeled `canonical` or `supported-beta target` below passed its frozen beta confirmation gate.

## Label Legend

- `canonical`: core product surface; must be right for beta.
- `supported-beta target`: included in the frozen beta promise, but not treated as a canonical core surface.
- `secondary`: useful but not part of the primary beta promise.
- `compatibility`: kept for compatibility/admin reasons, not as the preferred contract.
- `experimental`: shallow or intentionally non-primary.
- `partial`: real code exists, but important proof or correctness is still missing.
- `deferred`: intentionally outside the beta promise.

## Local Runtime / Execution

| Surface | Current label | Beta contract label | Proof level | Notes |
| --- | --- | --- | --- | --- |
| Daemon core (`orchestratord`, state machine, SQLite, worktrees, artifacts, validations, recovery) | `canonical` | `canonical` | Phase 3 local smoke + repo tests | WS-L1 re-proved the same-run local barrier and local evidence retrieval path. |
| CLI (`orchestratorctl`) | `canonical` | `canonical` | Phase 3 local smoke + repo tests | WS-L1 aligned `step wait`, `step result`, and `step retry` with the persisted local lifecycle. |
| Instance identity (`/api/v1/instance`, manifest, `orchestratorctl instance`) | `canonical` | `canonical` | Phase 3 local smoke + repo tests | Proven local identity/operator surface. |
| Simulation mode | `canonical` | `canonical` | Phase 3 local smoke + repo tests | Primary deterministic proof path. |
| Task/result schemas | `canonical` | `canonical` | Repo tests + runtime use | Main task/result contracts are real. |
| `/api/v1/compatibility` | `compatibility` | `compatibility` | Repo tests + runtime use | Diagnostic surface for runtime availability/binding state, not a beta-support certificate. |
| Policy schema + policy runtime contract | `partial` | `partial` | Code + drift audit | Schema/runtime drift still exists. |
| `codex` adapter | `partial` | `supported-beta target` | Phase 3 local smoke + conformance tests | Primary intended local beta adapter, but current repo proof is still simulation-heavy and not live-binary proven. |
| `claude` adapter | `partial` | `supported-beta target` | Fake-binary tests | Good wrapper proof, no live authenticated proof. |
| `qwen` adapter | `partial` | `secondary` | Conformance tests | Kept, not primary beta promise, and still simulation-only in checked-in proof. |
| `ide-chat` adapter | `experimental` | `deferred` | Code only/manual proxy model | Not a stable local execution contract. |
| `openclaw-acpx` adapter | `experimental` | `deferred` | Unit tests only | Explicitly experimental. |
| `antigravity` + `antigravity-broker` adapters | `partial` | `secondary` | Unit/integration tests | Environment-specific and not required for beta. |

## Runtime Connectivity / Control

| Surface | Current label | Beta contract label | Proof level | Notes |
| --- | --- | --- | --- | --- |
| Relay HTTP planner API | `partial` | `supported-beta target` | Self-host smoke + repo tests | Phase 2 re-proved relay HTTP over share-control and multi-instance self-host smoke. |
| Connector enrollment/session/auth | `partial` | `supported-beta target` | Self-host smoke + repo tests | Phase 2 hardened live-set re-advertise and stale reconnect pruning without broad auth-model changes. |
| Connector discover/list/status/share/unshare | `partial` | `supported-beta target` | Self-host smoke + repo tests | Phase 2 fixed `share --instance-id` so unresolved local instances fail loudly instead of persisting fake shared state. |
| Relay audit/admin status/connectors/instances | `partial` | `supported-beta target` | Self-host smoke + repo tests | Honest docs remain in place and self-host smoke now exercises the share-control/admin view more directly. |
| Best-effort abort semantics | `compatibility` | `compatibility` | README + code | Keep, but do not overclaim hard cancellation. |

## Planner / Client Integrations

| Surface | Current label | Beta contract label | WS-P1 proof status | Notes |
| --- | --- | --- | --- | --- |
| Generic relay HTTP client path | `partial` | `supported-beta target` | `proven` | Narrow bearer-token HTTP planner flow is re-proven by relay integration tests and the current self-host smoke path. |
| Generic relay MCP client path | `partial` | `supported-beta target` | `proven` | Canonical endpoint is `/mcp`; Phase 2 and Phase 4 both re-proved initialize/list/call, SSE bootstrap, aliasing, and scoped routing. |
| Official Go SDK path to relay MCP | `partial` | `supported-beta target` | `proven` | Explicitly proven for relay MCP only, not for relay REST HTTP. |
| Generic cloud HTTP client path | `partial` | `supported-beta target` | `proven` | Composed cloud smoke and runtime tests now cover tenant-scoped run create/get plus submit-task over cloud HTTP. |
| Generic cloud MCP client path | `partial` | `supported-beta target` | `proven` | Canonical endpoint is `/api/cloud/v1/mcp`; cloud-side initialize/list/call, stream/delete, aliasing, and token-bound session behavior are directly proven. |
| Official Go SDK path to cloud MCP | `partial` | `supported-beta target` | `proven` | Explicitly proven for cloud MCP only, not for cloud REST HTTP. |
| Generic MCP clients beyond the manual JSON-RPC callers and official Go SDK helper | `compatibility` | `compatibility` | `expected-only` | Codencer's MCP protocol surface is proven, but product-specific desktop/client interoperability is not claimed universally. |
| ChatGPT-style planner path via relay/cloud | `compatibility` | `compatibility` | `compatibility-only` | Remote MCP pattern only. Public docs now point to the canonical relay/cloud MCP endpoints without claiming repo-executed ChatGPT setup. |
| Claude-style planner path via relay/cloud | `compatibility` | `compatibility` | `compatibility-only` | Remote MCP pattern only and explicitly separate from the local `claude` execution adapter proof. |
| Daemon-local `/mcp/call` | `compatibility` | `compatibility` | `compatibility-only` | Local compatibility/admin bridge, not the public remote planner MCP contract. |
| Relay `/mcp/call` alias | `compatibility` | `compatibility` | `compatibility-only` | Repo-tested POST alias, but `/mcp` remains the canonical session path. |
| Cloud `/api/cloud/v1/mcp/call` alias | `compatibility` | `compatibility` | `compatibility-only` | Repo-tested POST alias, but `/api/cloud/v1/mcp` remains the canonical session path. |
| VS Code extension | `partial` | `secondary` | Code only | Exists, but proof is too thin for beta promise. |
| `agent-broker` Windows bridge | `experimental` | `secondary` | Build only; tests broken in nested module | Keep out of beta promise. |

## Cloud Control Plane

| Surface | Current label | Beta contract label | Proof level | Notes |
| --- | --- | --- | --- | --- |
| Org/workspace/project/membership/token APIs | `partial` | `supported-beta target` | Cloud smoke + repo tests | Phase 1 fixed token-revocation scope and revoked-token denial across cloud surfaces. |
| Installation/event/audit APIs | `partial` | `supported-beta target` | Cloud smoke + repo tests | Phase 1 fixed event and audit tenant filtering and added focused scope tests. |
| Runtime connector claim/list/enable/disable/sync | `partial` | `supported-beta target` | Repo tests + composed cloud smoke | Claimed-runtime control is now proven in a live composed flow. |
| Runtime instance registry/visibility | `partial` | `supported-beta target` | Repo tests + composed cloud smoke | Phase 1 added direct proof that claimed instances remain tenant-scoped and visible through cloud runtime control. |
| Cloud runtime HTTP proxy | `partial` | `supported-beta target` | Repo tests + composed cloud smoke | Phase 1 hardened nested route scopes and added live run/create/get + submit-task proof. |
| Cloud MCP | `partial` | `supported-beta target` | Repo tests + composed cloud smoke + SDK smoke | Phase 1 aligned cloud MCP auth/session/origin behavior with the cloud HTTP tenant model. |
| Cloud worker (`codencer-cloudworkerd`) | `partial` | `supported-beta target` | Repo tests | Jira polling-first only. |
| Docker self-host baseline (`deploy/cloud`) | `partial` | `supported-beta target` | Compose config + repo-level `make verify-beta` + direct `make cloud-stack-smoke` proof components | Final beta confirmation re-proved the Docker baseline on a host with a live Docker daemon. The Docker stack remains a narrow self-host baseline rather than a managed SaaS deployment story. |

## Provider Connectors

| Surface | Current label | Beta contract label | Proof level | Notes |
| --- | --- | --- | --- | --- |
| GitHub connector | `partial` | `supported-beta target` | Unit tests + provider matrix freeze | Validation, actions, and status are proven; install/bootstrap, routed ingest, audit depth, and local operator packaging remain narrow. |
| GitLab connector | `partial` | `supported-beta target` | Unit tests + provider matrix freeze | Validation, actions, and status are proven; install/bootstrap, routed ingest, audit depth, and local operator packaging remain narrow. |
| Jira connector | `partial` | `supported-beta target` | Unit tests + worker tests + router deferment test | Polling-first ingest is real, webhook ingest is explicitly deferred, and routed webhook calls now fail truthfully instead of silently ingesting. |
| Linear connector | `partial` | `supported-beta target` | Unit tests + provider matrix freeze | Validation and action coverage are real, but install/bootstrap, routed ingest, audit depth, and local operator packaging remain narrow. |
| Slack connector | `partial` | `supported-beta target` | Unit tests + router tests + cloud smoke | Strongest current provider path: install/bootstrap/local-test proof is real, but the overall surface is still intentionally narrow rather than marketplace-complete. |

## Historical / Internal Surfaces

| Surface | Current label | Beta contract label | Proof level | Notes |
| --- | --- | --- | --- | --- |
| Older internal progress/plan docs | `deferred` | `deferred` | Audit only | Historical context, not release truth. |
| Historical “no cloud” guidance | `deferred` | `deferred` | Audit only | No longer matches repo contents. |
