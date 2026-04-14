# Codencer Cloud V1 Finish Log

Last updated: 2026-04-14

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
