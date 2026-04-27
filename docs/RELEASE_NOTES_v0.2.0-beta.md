# Codencer v0.2.0-beta Release Notes

Use [BETA_TESTING.md](BETA_TESTING.md) for the frozen beta tracks and [mcp/integrations.md](mcp/integrations.md) for the planner/client contract.

## What shipped

`v0.2.0-beta` ships five public beta tracks:

- local-only daemon plus CLI
- self-host relay plus runtime connector
- self-host cloud control plane
- planner/client relay and cloud integrations
- provider connector beta surfaces

## What's new since v0.1.0-beta

- The repo now exposes a self-host relay path with planner HTTP, planner MCP, enrollment tokens, explicit instance sharing, and relay audit/admin surfaces.
- The cloud control plane is now part of the beta truth with bootstrap, tenancy, provider installs, claimed runtime connectors, cloud HTTP, and cloud MCP.
- Repo-level proof entrypoints now exist for supported builds and supported verification, including the Docker-backed cloud baseline on Docker-capable hosts.
- The public docs now route testers by platform and planner/client instead of forcing them to reverse-engineer the support matrix.
- Known limitations are consolidated into one operator-grade reference instead of being scattered across README, cloud docs, and planner docs.

## How to try it

Start with a clean clone, then use the supported build and verifier entrypoints before choosing a platform walkthrough and a planner walkthrough.

```bash
make build-supported
make verify-beta
make verify-beta-docker
```

Run `make verify-beta-docker` only on a Docker-capable host when you also want the Docker cloud baseline.

## What works today

- Local daemon, CLI, instance identity, simulation mode, artifacts, validations, and retry/gate flows are the canonical beta core.
- Self-host relay HTTP, relay MCP, connector enrollment/session/share control, and relay audit/admin surfaces are supported beta targets.
- Self-host cloud HTTP and cloud MCP are supported beta targets when cloud runs in composed runtime mode.
- The official Go SDK path to relay MCP and cloud MCP is proven.
- Provider connectors are included in the beta promise at the frozen documented depth, with Slack as the strongest current operator path and Jira explicitly polling-first.

## What's compatibility-only or deferred

- At beta cut, ChatGPT-style and Claude-style planner flows were `compatibility-only`. Post-beta hardening now packages narrower operator paths; use [mcp/integrations.md](mcp/integrations.md) and [internal/FLAGSHIP_PLANNER_PRODUCT_PATH.md](internal/FLAGSHIP_PLANNER_PRODUCT_PATH.md) for current truth.
- Gemini CLI remains `expected-only` in this pass because the docs are aligned to official Gemini CLI references but were not locally validated on this host.
- Generic MCP clients beyond the repo-proven manual JSON-RPC callers and the official Go SDK helper remain `expected-only`.
- Daemon-local `/mcp/call` remains a compatibility/admin bridge, not the public remote planner contract.
- `agent-broker`, the VS Code extension, `openclaw-acpx`, `ide-chat`, and vendor-depth provider flows remain outside the primary beta promise.

See [KNOWN_LIMITATIONS.md](KNOWN_LIMITATIONS.md) for the consolidated list.

## Known limitations

Use [KNOWN_LIMITATIONS.md](KNOWN_LIMITATIONS.md) for the current boundary list, workarounds, and severity labels.

## Feedback

File issues in the GitHub repository's normal Issues flow. There is no dedicated beta issue template in this repo right now, so use the default blank issue and include:

- platform (`macOS`, `Windows`, `WSL`, or `remote VPS`)
- planner/client (`ChatGPT`, `Claude Desktop`, `claude.ai`, `Gemini CLI`, or direct HTTP/MCP`)
- the track you followed from [BETA_TESTING.md](BETA_TESTING.md)
- the exact commands you ran
- the first failing output snippet
