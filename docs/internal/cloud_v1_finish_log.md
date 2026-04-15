# Codencer Cloud V1 Finish Log

Last updated: 2026-04-15

## Current Cloud Hardening Pass

Mission for this pass:

- add a cloud-scoped canonical remote surface decision and implementation
- harden tenancy with memberships, roles, and better audit attribution
- deepen the priority connector lifecycle without adding more connector breadth
- add a real Docker-based self-host baseline and deployment smoke

Exact blockers locked for this pass:

1. Cloud-scoped runtime control had no cloud MCP surface
2. Tenancy lacked first-class memberships, roles, and ownership semantics
3. Provider installations lacked stronger owner/health/timestamp lifecycle depth
4. The repo had no Docker deployment baseline for the cloud stack

### Current Hardening Ownership Map

- Access and cloud MCP:
  - `internal/cloud/auth.go`
  - `internal/cloud/router.go`
  - `internal/cloud/server.go`
  - `internal/cloud/mcp_server.go`
  - `internal/cloud/mcp_tools.go`
  - `internal/cloud/membership_api.go`
  - `cmd/codencer-cloudctl/main.go`
- Connector lifecycle hardening:
  - `internal/cloud/connectors/*`
  - `internal/cloud/worker.go`
- Deployment baseline:
  - `deploy/cloud/*`
  - `Makefile`
- Truth/docs:
  - `README.md`
  - `docs/CLOUD.md`
  - `docs/CLOUD_CONNECTORS.md`
  - `docs/CLOUD_SELF_HOST.md`
  - this file

### Current Hardening Verification Ledger

| Merge | Scope | Checks | Result | Notes |
| --- | --- | --- | --- | --- |
| 1 | membership + role + audit attribution + cloud MCP | `go test ./internal/cloud/... ./cmd/codencer-cloudctl ./cmd/codencer-cloudd ./cmd/codencer-cloudworkerd` | passed | API and cloud MCP coverage landed together |
| 2 | connector lifecycle hardening | `go test ./internal/cloud/connectors ./internal/cloud/...` | passed | provider config validation and lifecycle persistence remained green |
| 3 | deployment baseline | `docker compose --env-file deploy/cloud/.env.example -f deploy/cloud/docker-compose.yml config` | passed | compose file, env wiring, mounts, and healthcheck syntax validated |
| 4 | compose smoke | `./deploy/cloud/smoke.sh` | blocked | Docker CLI was installed, but the Docker daemon/socket was unavailable in this environment, so the stack could not be started live |

## Current Deepening Pass

This pass is narrower than the original cloud-foundation push.

Mission for the current pass:

- make cloud the tenant-aware control plane for Codencer runtime when cloud mode is used
- keep the local daemon, relay, and connector execution doctrine intact
- deepen the priority provider connectors instead of adding more shallow breadth
- keep claims about connector and cloud maturity exact

This pass does **not** add UI, billing, or new low-priority connectors.

## Runtime Control-Plane Gap Lock

Current code truth at the start of this pass:

- cloud control-plane APIs exist under `/api/cloud/v1/*`
- relay runtime APIs still exist separately under `/api/v2/*`, `/mcp`, and `/ws/connectors`
- `codencer-cloudd` can compose relay in-process, but that is still process composition rather than tenant-aware runtime ownership
- cloud token scope governs cloud admin APIs only
- relay planner tokens still govern runtime routing and instance visibility
- cloud stores provider connector installations, but not tenant-scoped Codencer runtime connector installations or runtime instances

Exact blockers locked for this pass:

1. No cloud-side Codencer runtime installation model
   - missing tenant-scoped record for local Codencer connector identity, machine metadata, enabled state, last seen, health, and last error

2. No cloud-side runtime instance registry
   - missing tenant-scoped record for shared instances, instance metadata, connector ownership, enabled state, and last seen

3. No cloud-scoped runtime API surface
   - missing cloud routes for runtime connectors, instances, and runtime inspection under org/workspace/project scope

4. No cloud/relay auth alignment
   - composed cloud mode does not translate tenant ownership and cloud token scope into runtime visibility

5. Provider connectors still remain thin alpha integrations
   - GitHub, GitLab, Linear, and Slack each expose one minimal action
   - Jira remains polling-first with limited health depth
   - docs compress code existence and verified depth too aggressively

## Current Pass Ownership Map

### A. Runtime Model + Store

- Owner: write worker
- Scope:
  - add tenant-scoped Codencer runtime installation and runtime instance models
  - extend cloud store and migrations
  - add store tests for runtime registry behavior
- Files:
  - `internal/cloud/models.go`
  - `internal/cloud/store.go`
  - `internal/cloud/*_test.go` for runtime model/store coverage
- Status: completed

### B. Cloud Runtime API + Auth + Relay Alignment

- Owner: Lead
- Scope:
  - cloud runtime routes
  - cloud token scope enforcement for runtime resources
  - composed relay alignment and tenant-scoped runtime visibility
  - cloudctl runtime admin surfaces
- Files:
  - `internal/cloud/auth.go`
  - `internal/cloud/server.go`
  - `internal/cloud/router.go`
  - `cmd/codencer-cloudd/main.go`
  - `cmd/codencer-cloudctl/main.go`
  - `internal/relay/*` as needed for composed cloud alignment
- Status: completed

### C. Priority Connector Deepening

- Owner: write worker
- Scope:
  - stronger validation and provider-specific status detail where practical
  - richer action surface for priority providers
  - stronger normalization tests
  - no new providers
- Files:
  - `internal/cloud/connectors/types.go`
  - `internal/cloud/connectors/common.go`
  - `internal/cloud/connectors/github.go`
  - `internal/cloud/connectors/gitlab.go`
  - `internal/cloud/connectors/jira.go`
  - `internal/cloud/connectors/linear.go`
  - `internal/cloud/connectors/slack.go`
  - matching tests under `internal/cloud/connectors/*_test.go`
- Status: completed

### D. Docs / Truth / Verification

- Owner: Lead
- Scope:
  - update cloud docs and connector matrix
  - record exact verification after each merge
  - keep claims narrow and evidence-based
- Files:
  - `docs/CLOUD.md`
  - `docs/CLOUD_CONNECTORS.md`
  - `docs/CLOUD_SELF_HOST.md`
  - this finish log
- Status: completed

## Current Merge Order

1. Update runtime blocker lock and ownership log
2. Merge cloud runtime model/store foundation
3. Re-run `go test ./internal/cloud/...`
4. Merge cloud runtime API/auth/alignment work
5. Re-run focused cloud + relay tests
6. Merge provider connector deepening
7. Re-run `go test ./internal/cloud/connectors ./internal/cloud/...`
8. Update docs and self-host/cloud truth
9. Run broad verification: `go test ./...`, `make build`, `make build-cloud`

## Current Pass Delivery Snapshot

Implemented in this pass:

- cloud-side runtime registry foundation:
  - `RuntimeConnectorInstallation`
  - `RuntimeInstance`
  - runtime registry migrations and store methods
- cloud-scoped runtime routes under `/api/cloud/v1/runtime/*`
- cloud-scoped runtime connector claim/sync/enable/disable flows
- cloud-scoped runtime instance inspection and instance-scoped HTTP proxying for runs, steps, gates, and artifacts
- relay helper support for trusted in-process planner principals used by the cloud daemon
- deeper provider connector action surface and stronger connector tests
- updated cloud docs and optional runtime-claim smoke wiring

## Current Pass Verification Ledger

| Merge | Scope | Checks | Result | Notes |
| --- | --- | --- | --- | --- |
| 1 | runtime blocker lock + ownership map | log update only | passed | this file is the canonical pass log |
| 2 | runtime model/store foundation + connector depth slices merged | `go test ./internal/cloud/... ./internal/cloud/connectors` | passed | worker slices landed cleanly |
| 3 | relay in-process planner injection helper | `go test ./internal/relay -run 'TestPlanner|TestServeAsPlanner'` | passed | cloud can now proxy through relay without a second bearer token hop |
| 4 | cloud runtime API + cloudctl | `go test ./internal/cloud/... ./cmd/codencer-cloudctl ./internal/relay ./cmd/codencer-cloudd ./cmd/codencer-cloudworkerd` | passed | claim/list/disable runtime flows covered |
| 5 | broad verification | `go test ./...` | passed | repo-wide tests remained green |
| 6 | build verification | `make build` and `make build-cloud` | passed | core binaries and cloud binaries build |
| 7 | operator smoke | `bash -n scripts/cloud_smoke.sh` and `make cloud-smoke` | passed | runtime-claim smoke path remains optional and env-driven |

## Mission

Take the current Codencer repository from:

- practical self-host v2 alpha with daemon, connector, relay, relay MCP, and operator tooling

to:

- first-class open-source-based cloud backend/control-plane foundation without UI
- while preserving the current self-host relay/connector/daemon path
- and keeping docs, runtime behavior, and public claims truthful

## Repo Truth Lock

Current repo truth at the start of this cloud run:

- Codencer is still explicitly documented and implemented as a local-first bridge.
- The current shipped runtime is:
  - local daemon: `orchestratord`
  - local operator CLI: `orchestratorctl`
  - self-host relay: `codencer-relayd`
  - local outbound connector: `codencer-connectord`
  - optional Windows-side `agent-broker`
- The repo does **not** currently contain a true cloud domain or tenancy model.
- The repo does **not** currently contain a SaaS connector platform for GitHub, GitLab, Jira, Linear, or Slack.
- The repo does **not** currently contain a packaged cloud deployment stack.

Primary supporting file references:

- [README.md](/Users/lookman/Projects/codencer/README.md)
- [docs/01_product_scope.md](/Users/lookman/Projects/codencer/docs/01_product_scope.md)
- [docs/02_architecture.md](/Users/lookman/Projects/codencer/docs/02_architecture.md)
- [docs/SELF_HOST_REFERENCE.md](/Users/lookman/Projects/codencer/docs/SELF_HOST_REFERENCE.md)
- [docs/RELAY.md](/Users/lookman/Projects/codencer/docs/RELAY.md)
- [internal/relay/server.go](/Users/lookman/Projects/codencer/internal/relay/server.go)
- [internal/relay/store/store.go](/Users/lookman/Projects/codencer/internal/relay/store/store.go)
- [internal/storage/sqlite/migrations.go](/Users/lookman/Projects/codencer/internal/storage/sqlite/migrations.go)

## Reusable Foundations

The existing repo already provides reusable cloud-adjacent foundations:

- narrow relay control-plane patterns:
  - planner bearer-token auth
  - connector enrollment tokens
  - signed connector challenge/response
  - connector presence/session hub
  - instance registry and route hints
  - audit persistence
- stable local execution model:
  - repo-bound daemon
  - worktree isolation
  - runs / steps / attempts / gates
  - evidence retrieval
- operator CLI patterns:
  - `codencer-relayd` admin helpers
  - `codencer-connectord` admin helpers
  - smoke-script pattern for end-to-end verification

These are reusable as a cloud runtime/control-plane substrate.

## Exact Blocker Lock

The following blockers must be addressed before Codencer can be truthfully described as a cloud backend/control-plane:

1. No cloud domain model
   - missing `org`, `workspace`, `project`, membership, role, and ownership entities
   - existing `project_id` is only an execution label on runs/tasks

2. No cloud token / access model
   - current relay planner auth is static config token auth only
   - no tenant-scoped API token lifecycle, disable/revoke, or attribution model

3. No cloud persistence layer
   - no cloud DB schema for tenants, tokens, connector installations, external events, or action history
   - no cloud migration strategy beyond inline sqlite DDL in current local/relay stores

4. No cloud control-plane service
   - no `codencer-cloudd`-style backend
   - no cloud admin API
   - no cloud admin CLI

5. No external connector platform
   - current connector is a local daemon bridge, not a SaaS integration framework
   - no installation model, no secrets/config model, no normalized event model, no action dispatch model

6. No top-tier connector implementations
   - no GitHub, GitLab, Jira, Linear, or Slack connector packages exist in repo code

7. No webhook/polling ingest plane
   - no provider webhook endpoints
   - no signature verification routes
   - no sync cursor or polling state

8. No deployment/self-host cloud story
   - no packaged cloud service startup flow
   - no cloud env/config examples
   - no cloud smoke flow

9. Public docs still say “no cloud”
   - current docs explicitly position Codencer as local-first and non-cloud
   - any cloud additions must reconcile this honestly without breaking self-host truth

## Initial Delivery Target For This Run

Given current repo reality, the maximum safe target for this run is:

- add a real cloud backend foundation to the repo
- keep existing self-host v2 runtime intact
- make the new cloud backend reuse the existing relay path where possible
- implement a reusable external connector platform
- implement real minimal connectors for the priority set
- prove what is actually verified
- explicitly list whatever remains partial

This run must not overclaim:

- enterprise IAM
- fully managed SaaS maturity
- full connector parity with vendor ecosystems
- cloud-hosted execution replacing local execution truth

## Workstreams

### A. Repo Truth + Cloud Gap Lock

- Owner: Lead
- Scope:
  - this finish log
  - blocker lock
  - ownership map
  - merge plan
- Files:
  - `docs/internal/cloud_v1_finish_log.md`
- Status: completed

### B. Cloud Domain / Tenancy / Auth Foundation

- Owner: Helmholtz
- Scope:
  - cloud config
  - tenant domain model
  - token model
  - installation model
  - secrets model
  - audit attribution
  - cloud store and migrations
- Files:
  - `internal/cloud/config.go`
  - `internal/cloud/models.go`
  - `internal/cloud/store.go`
  - `internal/cloud/auth.go`
  - `internal/cloud/secrets.go`
  - tests under `internal/cloud/*_test.go`
- Status: completed

### C. Cloud Control-Plane API / Admin Surfaces

- Owner: Lead
- Scope:
  - cloud HTTP admin API
  - cloud admin CLI
  - bootstrap flows for org/workspace/project/token/installations
- Files:
  - `internal/cloud/server.go`
  - `internal/cloud/router.go`
  - `cmd/codencer-cloudd/main.go`
  - `cmd/codencer-cloudctl/main.go`
  - tests under `cmd/codencer-cloudctl/*` and `internal/cloud/*_test.go`
- Status: completed

### D. Connector Platform Foundation

- Owner: Lovelace
- Scope:
  - connector registry
  - connector contract
  - normalized events/actions
  - install/validate/action/ingest helpers
- Files:
  - `internal/cloud/connectors/registry.go`
  - `internal/cloud/connectors/types.go`
  - `internal/cloud/connectors/common.go`
  - tests under `internal/cloud/connectors/*_test.go`
- Status: completed

### E. Top-Tier Connectors

- Owner: split
- Scope:
  - GitHub
  - GitLab
  - Jira
  - Linear
  - Slack
- Files:
  - `internal/cloud/connectors/github.go`
  - `internal/cloud/connectors/gitlab.go`
  - `internal/cloud/connectors/jira.go`
  - `internal/cloud/connectors/linear.go`
  - `internal/cloud/connectors/slack.go`
  - tests under `internal/cloud/connectors/*_test.go`
- Status: completed

Connector ownership:

- Lovelace:
  - `internal/cloud/connectors/github.go`
  - `internal/cloud/connectors/gitlab.go`
- Ramanujan:
  - `internal/cloud/connectors/jira.go`
  - `internal/cloud/connectors/linear.go`
  - `internal/cloud/connectors/slack.go`

### F. Relay / Cloud Alignment

- Owner: Lead
- Scope:
  - keep existing relay intact
  - align cloud service composition with current relay
  - preserve explicit instance targeting and local execution truth
- Files:
  - `cmd/codencer-cloudd/main.go`
  - `internal/cloud/server.go`
  - `internal/cloud/router.go`
  - `docs/RELAY.md`
- Status: completed

### G. Deployment / Self-Host / Operator Flow

- Owner: Lead with docs worker support
- Scope:
  - cloud docs
  - cloud examples
  - cloud smoke path
  - startup/dependency flow
- Files:
  - cloud docs and scripts to be added after runtime shape is concrete
- Status: completed

### H. Final Test / Harden / Truth Pass

- Owner: Lead
- Scope:
  - formatting
  - targeted tests after each merge
  - broad builds/tests
  - final truth summary
- Status: completed

## Merge Order

1. Repo truth + cloud gap lock
2. Cloud domain / tenancy / auth foundation
3. Cloud control-plane API / admin surfaces
4. Connector platform foundation
5. Top-tier connector implementations
6. Relay / cloud alignment
7. Deployment / self-host / operator docs and smoke
8. Final hardening, broad tests/builds, and truth pass

## Final Delivery Snapshot

Implemented in this pass:

- new cloud domain, store, auth, and secret foundation under `internal/cloud`
- cloud admin/control-plane binaries:
  - `cmd/codencer-cloudd`
  - `cmd/codencer-cloudctl`
  - `cmd/codencer-cloudworkerd`
- cloud admin HTTP surface under `/api/cloud/v1/*`
- connector registry plus provider implementations for:
  - GitHub
  - GitLab
  - Jira
  - Linear
  - Slack
- provider webhook ingest routes where implemented
- Jira polling-first worker path
- installation enable/disable surfaces
- cloud docs, setup docs, and smoke script

## Verification Ledger

Cloud-focused checks run during this pass:

- `go test ./internal/cloud/...`
- `go test ./internal/cloud/... ./cmd/codencer-cloudctl ./cmd/codencer-cloudd ./cmd/codencer-cloudworkerd`
- `go build ./cmd/codencer-cloudctl ./cmd/codencer-cloudd ./cmd/codencer-cloudworkerd`
- `make build-cloud`
- `bash -n scripts/cloud_smoke.sh`
- `make cloud-smoke`

Broad preservation checks run after cloud integration:

- `go test ./...`
- `make build`
- `make build-broker`
- `git diff --check`

Verification outcome:

- cloud control-plane binaries build
- cloud smoke path passed end-to-end
- repo-wide tests passed
- existing local/self-host build targets still pass

## Truthful Alpha Limitations

The cloud backend is real and usable for operator self-use, but these limitations remain explicit:

- bootstrap of the first org is local/store-driven via `codencer-cloudctl bootstrap`; there is no fully HTTP-only first-org bootstrap token flow in this pass
- the cloud store is SQLite-backed in this alpha pass; Postgres/Redis/object-storage-backed deployment is not implemented
- identity is service-token/operator-token based; there is no user membership or enterprise IAM model yet
- GitHub, GitLab, Linear, and Slack connector behavior is unit-tested in-repo, but this pass does not claim live end-to-end verification against hosted provider accounts
- Jira is intentionally polling-first through `codencer-cloudworkerd`; Jira webhook ingest is not implemented in this pass
- the cloud control plane does not replace the local daemon/relay execution truth and must not be described as a planner or generic cloud workflow brain
8. Final hardening / tests / truth pass

## Verification Ledger

| Merge | Scope | Checks | Result | Notes |
| --- | --- | --- | --- | --- |
| 1 | repo truth + cloud gap lock | pending | pending | this row will be updated after merge |

## Open Questions Locked For Implementation

- Storage driver:
  - locked for this pass: sqlite-backed alpha cloud backend using isolated cloud store code
  - note clearly in docs that this is alpha self-host cloud persistence, not a production HA database posture
- Relay composition:
  - locked for this pass: `codencer-cloudd` composes the existing relay handler rather than rewriting relay internals
- Connector auth depth:
  - token bootstrap first, OAuth/app-model only where safe and proven

These must be resolved in code and docs, not by aspiration.
