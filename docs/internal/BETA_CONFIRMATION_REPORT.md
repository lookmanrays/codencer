# Beta Confirmation Report

Decision date: 2026-04-23

Release truth: `v0.2.0-beta`

Beta confirmed: `yes`

## Commands Run

Working tree verification:

- `make build-supported`
- `make verify-beta`
- `docker desktop status && make cloud-stack-smoke`
- `make verify-beta-docker`

Clean-ish verification:

- initial plain rsync temp copy of the repo plus `make build-supported && make verify-beta`
- detached temp worktree overlay of the current repo contents plus `make build-supported && make verify-beta`

## Outcomes

| Check | Outcome | Notes |
| --- | --- | --- |
| Supported build surface | Pass | `make build-supported` rebuilt the main supported binaries plus the MCP SDK smoke helper. |
| Repo-level supported verification | Pass | `make verify-beta` reran main-module tests, local smoke, self-host relay/runtime MCP+SDK smoke, cloud binary smoke, and Docker compose config validation. |
| Docker-backed cloud stack proof | Pass | `make cloud-stack-smoke` passed after Docker Desktop was started and the local Docker daemon became available. |
| Post-promotion repo-level Docker-inclusive verification | Pass | `make verify-beta-docker` reran the supported repo verifier plus the Docker-backed cloud stack baseline after the beta status/version/docs promotion landed. |
| Clean-ish repo verification | Pass | The supported verification reran successfully from a detached temporary worktree overlaid with the current repo contents. |

## Notes On Environment And Method

- Docker CLI and Docker Desktop were installed at the start of the run, but the daemon was not initially running.
- Docker Desktop was started during this confirmation pass, after which `docker desktop status` reported `running` and `make cloud-stack-smoke` completed successfully.
- A plain rsync-only temp copy without `.git` was not treated as equivalent clean-checkout proof because some worktree-sensitive tests require a real Git repository. That method failed for environment-shape reasons, not because of a repo regression.
- The clean-ish confirmation proof used a detached temporary `git worktree` plus an overlay of the current repo contents, which preserved Git metadata while still proving the repo from a fresh location.

## Decision Reasoning

Beta is confirmed because all of the frozen Phase 0 through Phase 6 blocker classes are closed and the final required proofs are green:

- local-only proof is green
- self-host relay/runtime proof is green
- self-host cloud binary proof is green
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
