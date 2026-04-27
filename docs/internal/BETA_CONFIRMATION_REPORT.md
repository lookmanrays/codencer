# Beta Confirmation Report

Decision date: 2026-04-23

Release truth: `v0.2.0-beta`

Beta confirmed: `yes`

## Commands Run

Fresh Phase 7 confirmation evidence:

- `make build-supported`
- `./scripts/smoke_test_v1.sh`
- `./scripts/smoke_test_v1.sh`
- `make smoke`
- `PLANNER_TOKEN=... RELAY_CONFIG=... RELAY_URL=... DAEMON_URL=... SMOKE_SCENARIOS=status,audit,share-control,multi-instance,mcp,mcp-sdk ./scripts/self_host_smoke.sh`
- `go test ./internal/cloud/... -count=1`
- `go test ./internal/relay ./internal/cloud ./cmd/mcp-sdk-smoke -count=1`
- `make cloud-smoke`
- `CLOUD_RELAY_CONFIG=... CLOUD_RUNTIME_DAEMON_URL=http://127.0.0.1:18085 CLOUD_SMOKE_MCP=1 CLOUD_SMOKE_SDK=1 ./scripts/cloud_smoke.sh`
- `make verify-beta`
- detached temporary `git worktree` run of `make build-supported && make verify-beta`
- `make cloud-stack-smoke`
- `make verify-beta-docker`

## Outcomes

| Check | Outcome | Notes |
| --- | --- | --- |
| Supported build surface | Pass | `make build-supported` rebuilt the main supported binaries plus the MCP SDK smoke helper. Local `/usr/local/opt/grpc/lib` linker warnings still appeared, but they were non-blocking. |
| Local smoke confirmation | Pass | `./scripts/smoke_test_v1.sh` passed twice and `make smoke` passed, preserving the documented local beta proof from the public entrypoints. |
| Fresh self-host relay/runtime confirmation | Pass | The fresh self-host smoke with `status,audit,share-control,multi-instance,mcp,mcp-sdk` re-proved relay HTTP, share-control, multi-instance routing, canonical relay MCP, and official Go SDK interop. |
| Cloud regression + smoke confirmation | Pass | `go test ./internal/cloud/... -count=1`, `make cloud-smoke`, and the composed cloud smoke with `CLOUD_RUNTIME_DAEMON_URL=http://127.0.0.1:18085 CLOUD_SMOKE_MCP=1 CLOUD_SMOKE_SDK=1` all passed. |
| Relay/cloud MCP + SDK confirmation | Pass | `go test ./internal/relay ./internal/cloud ./cmd/mcp-sdk-smoke -count=1` re-proved the canonical relay/cloud MCP paths and kept the official Go SDK helper green. |
| Repo-level supported verification | Pass | `make verify-beta` reran main-module tests, local smoke, self-host relay/runtime MCP+SDK smoke, cloud binary smoke, and Docker compose config validation from the active checkout. |
| Fresh-location repo verification | Pass | A detached temporary `git worktree` reran `make build-supported && make verify-beta` successfully from a fresh location. |
| Docker-backed cloud stack proof | Pass | `make cloud-stack-smoke` passed on a host with a live Docker daemon. |
| Final-tree Docker-inclusive verifier | Pass | `make verify-beta-docker` re-ran the supported repo verifier plus the Docker-backed cloud stack baseline after the Phase 7 doc normalization updates landed. |

## Notes On Environment And Method

- The fresh-location confirmation proof used a detached temporary `git worktree` at the current `HEAD` before running `make build-supported && make verify-beta`.
- That method preserved the Git metadata required by worktree-sensitive checks while still proving the repo from a fresh location away from the active checkout.
- `make cloud-stack-smoke` ran on a host with a live Docker daemon.
- Local `/usr/local/opt/grpc/lib` linker search-path warnings still appeared during build-oriented steps, but they did not block any required proof.

## Decision Reasoning

Beta is confirmed because all of the frozen Phase 0 through Phase 6 blocker classes are closed and the fresh Phase 7 proofs are green:

- local-only proof is green
- self-host relay/runtime proof is green
- self-host cloud binary and composed cloud proof are green
- Docker-backed cloud stack proof is green
- planner/client MCP and official Go SDK proof are green
- provider connector proof remains green within the documented narrow scope
- public docs, version strings, and frozen internal beta docs now agree on the same support contract

## Remaining Outside The Beta Promise

These items remain intentionally outside the current beta promise:

- `agent-broker`
- VS Code extension runtime proof
- daemon-local MCP as a public remote contract
- secondary adapters such as `qwen`, `antigravity*`, `ide-chat`, and `openclaw-acpx`
- product-specific ChatGPT, Claude Code, Claude Desktop, or marketplace publication proof
- live vendor-account proof for every provider
- enterprise IAM / SSO
- billing
- cloud UI / public SaaS productization

## Remaining Non-Blocking Open Items

- BG-020: local linker environment still emits `/usr/local/opt/grpc/lib` search-path warnings during builds, but all required builds and proofs passed.
