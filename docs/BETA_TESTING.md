# Public Beta Test Tracks

Codencer is `v0.2.0-beta` overall. This guide freezes the **externally testable** public beta tracks and their exact proof boundaries.

Use this page when you want to answer one practical question quickly:

- local only
- self-host relay/runtime
- self-host cloud
- planner/client integrations
- provider connectors

## Prerequisites

Common requirements for the supported tracks:

- Git
- Go `1.25.0+`
- `cc` or `gcc` for the CGO SQLite build
- `curl`
- `jq` or Python 3 for shell automation helpers

Additional requirement for the Docker self-host baseline:

- Docker CLI plus a running Docker daemon

## Choose Your Test Track

| Track | Start here | Build | Proof command | Current boundary |
| --- | --- | --- | --- | --- |
| Local-only daemon + CLI | [SETUP.md](SETUP.md) | `make build` | `./scripts/smoke_test_v1.sh` then `make smoke` | Canonical local proof is simulation-first; live adapter proof stays narrow. |
| Self-host relay + runtime connector | [SELF_HOST_REFERENCE.md](SELF_HOST_REFERENCE.md) | `make build` | `PLANNER_TOKEN=<planner-token> make self-host-smoke-mcp` | Canonical remote self-host path; relay `/mcp` is the public MCP surface. |
| Self-host cloud control plane | [CLOUD_SELF_HOST.md](CLOUD_SELF_HOST.md) | `make build-cloud` | `make cloud-smoke` | Binary-native proof covers bootstrap, tenancy, provider installs, and optional composed runtime/MCP/SDK proof. |
| Planner / client integrations | [mcp/integrations.md](mcp/integrations.md) | `make build build-cloud build-mcp-sdk-smoke` | self-host or cloud smoke with MCP/SDK enabled | ChatGPT-style and Claude-style paths remain compatibility-only, not direct product proof. |
| Provider connectors | [CLOUD_CONNECTORS.md](CLOUD_CONNECTORS.md) | `make build-cloud` | `make cloud-smoke` plus provider-focused tests | Slack is strongest today; Jira is polling-first; the rest stay narrow operator/package surfaces. |

## Repo-Level Verification Commands

For a supported non-Docker verification pass from a working checkout:

```bash
make build-supported
make verify-beta
```

What `make verify-beta` covers:

- main-module tests
- local smoke
- self-host relay/runtime smoke with MCP + official Go SDK proof
- cloud binary smoke
- Docker compose config validation

For the Docker-backed cloud baseline on a Docker-capable host:

```bash
make verify-beta-docker
```

That adds:

- `make cloud-stack-smoke`

## Track Notes

### Local-only

- Start with [SETUP.md](SETUP.md).
- The canonical local proof is:

```bash
./scripts/smoke_test_v1.sh
./scripts/smoke_test_v1.sh
make smoke
```

- `/api/v1/compatibility` is a runtime diagnostic surface, not a support certificate.
- daemon-local `/mcp/call` is compatibility-only and not part of the public remote planner contract.

### Self-host relay / runtime

- Start with [SELF_HOST_REFERENCE.md](SELF_HOST_REFERENCE.md).
- The canonical public relay surfaces are:
  - HTTP: `/api/v2/...`
  - MCP: `/mcp`
- `/mcp/call` remains a compatibility alias.
- The strongest scripted relay proof is:

```bash
PLANNER_TOKEN=<planner-token> make self-host-smoke-mcp
```

For broader relay proof:

```bash
PLANNER_TOKEN=<planner-token> make self-host-smoke-all
```

### Self-host cloud

- Start with [CLOUD_SELF_HOST.md](CLOUD_SELF_HOST.md).
- Use `make cloud-stack-smoke` for the **Docker baseline only**.
- Use `make cloud-smoke` for the **binary-native cloud control-plane proof**.
- Use composed-mode inputs with `make cloud-smoke` when you want runtime HTTP, cloud MCP, and official Go SDK proof:

```bash
make build-cloud build-mcp-sdk-smoke
CLOUD_RELAY_CONFIG=.codencer/relay/config.json \
CLOUD_RUNTIME_DAEMON_URL=http://127.0.0.1:8085 \
CLOUD_SMOKE_MCP=1 \
CLOUD_SMOKE_SDK=1 \
make cloud-smoke
```

The Docker compose stack does not create a usable runtime instance by itself. For cloud-scoped runtime control, you still need an external `orchestratord` plus `codencer-connectord`.

### Planner / client integrations

- Start with [mcp/integrations.md](mcp/integrations.md).
- Repo-proven remote client surfaces are:
  - relay HTTP
  - relay MCP
  - cloud HTTP
  - cloud MCP
  - official Go SDK to relay/cloud MCP
- Generic MCP clients beyond the checked-in proof helpers remain expected-only.
- ChatGPT-style and Claude-style product integrations remain compatibility-only.

### Provider connectors

- Start with [CLOUD_CONNECTORS.md](CLOUD_CONNECTORS.md).
- Current public truth:
  - Slack is the strongest provider-shaped local test path.
  - Jira is polling-first; webhook ingest remains deferred.
  - GitHub, GitLab, and Linear are proven more narrowly through fixture and routed-package coverage.
- `make cloud-smoke` proves the generic cloud/provider install-webhook-events-audit path.
- Live vendor-account proof is not part of the current beta promise.

## Clean-Checkout Notes

When validating from a fresh checkout:

1. Copy `.env.example` to `.env` only if you are testing the local daemon convenience flow.
2. Build with `make build-supported`.
3. Run `make verify-beta`.
4. Run `make verify-beta-docker` only if Docker daemon access is available.

If you only want one track, use the track-specific commands above instead of the full repo pass.

## Known Public Boundaries

- This guide reflects the repo-wide beta contract confirmed by the final verification pass.
- `agent-broker`, the VS Code extension, daemon-local MCP, and secondary adapters remain outside the primary beta promise.
- Provider connectors are beta-installable/testable within narrow claims, not marketplace-complete.
- Product-specific ChatGPT, Claude Code, Claude Desktop, or vendor marketplace publication flows are not directly proven in this repo.

## Filing Useful Test Reports

Include all of the following when reporting a tester-facing issue:

- the track you were testing
- the exact command you ran
- OS and shell
- Go version
- whether Docker daemon access was available
- the failing log excerpt or artifact path
- whether you were using simulation mode, relay self-host mode, or cloud composed mode
