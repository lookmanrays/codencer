# codencer.dev Update Pack

Status: `site repository update pending`.

This repository does not contain the codencer.dev website source. The only
checked-in `package.json` is `extension/package.json`, which is unrelated to
the public website. Apply this document in the codencer.dev site repository as
the implementation brief.

## Current Public Website Issues To Fix

- Public copy still references the older beta version.
- The "What exists today" section does not reflect the v0.3 local/self-host RC.
- Old beta-track language makes cloud/control-plane surfaces sound like the
  current open-source path.
- Any hosted launch date wording should be replaced with future
  Codencer Gateway/Cloud positioning.
- The site must not claim live ChatGPT, Codex, or Claude product proof unless
  actual evidence exists.

## Required Homepage Positioning

Position Codencer as:

- `v0.3.0-local-prod-rc.1`;
- an open-source local/self-host bridge between AI planners and coding
  executors;
- local-first by default;
- self-host Relay plus MCP for remote planner/client access;
- local connector for explicit project sharing;
- project-local `.codencer/project.json`;
- machine-aware routing by `machine_id` or `host_label`;
- activation support for ChatGPT custom MCP app setup, Claude Code MCP setup,
  and Codex MCP setup;
- future Codencer Gateway/Cloud as a separate planned managed layer.

## Suggested Homepage Sections

1. Hero
2. What Codencer is
3. How it works
4. Local-first flow
5. Self-host Relay + MCP flow
6. Project config + machine routing
7. MCP clients
8. Status matrix
9. OSS now / Gateway later
10. Quickstart
11. Docs links
12. License and trademark note

## Exact Copy Snippets

### Hero Headline

Codencer

### Subheadline

Open-source local/self-host bridge between AI planners and coding executors.

### Status Line

v0.3.0-local-prod-rc.1: local-first daemon, self-host Relay, project-aware MCP,
activation packages, and release snapshot packaging.

### What Codencer Is

Codencer is a bridge, not a planner. The planner decides what should happen.
The executor performs approved work. Codencer records runs, steps, attempts,
artifacts, validations, logs, and blockers so humans and planners can make the
next decision from structured evidence.

### What Exists Today

Codencer Core includes local CLI and daemon execution, project registry,
project-local `.codencer/project.json`, machine identity and host labels,
manifest execution, structured blockers, self-host Relay, local connector,
Relay-hosted MCP tools, MCP setup snippets for Codex and Claude Code, ChatGPT
custom MCP app setup guidance with OAuth dev mode, activation package
generation, readiness/acceptance/proof bundles, and release snapshots for
darwin and linux/amd64.

### What Is Not Claimed Yet

Live ChatGPT product UI proof, live Codex client proof, and live Claude Code
client proof remain pending unless those products are actually connected and
evidence is saved. Codencer does not currently claim signed/notarized binaries,
Windows-native daemon binaries, hosted Gateway/Cloud availability, commercial
billing, or hosted UI from this repository.

### Self-Host Now, Gateway Later

The open-source path today is self-host Relay plus local connector. Future
Codencer Gateway/Cloud is a separate managed layer with official service
identity. The same local connector and project-aware MCP toolset conceptually
carry forward, but the hosted service is not shipped by this repository.

### Try It Locally

```bash
make build-codencer
./bin/codencer init --json
./bin/codencer machine show --json
./bin/codencer project init --id codencer --repo . --adapter fake --profile fake-success --json
./bin/codencer demo local --json --bin-dir ./bin
```

### Deploy Self-Host Relay

```bash
make release-snapshot VERSION=v0.3.0-local-prod-rc.1
codencer setup relay --base-url https://relay.example.com --generate-planner-token --json
codencer connector enroll --relay-url https://relay.example.com --daemon-url http://127.0.0.1:18085 --enrollment-token "$CODENCER_CONNECTOR_ENROLLMENT_TOKEN" --config "$CODENCER_HOME/runtime/connector/config.json" --json
codencer activation package --relay https://relay.example.com --project codencer --token-env CODENCER_MCP_TOKEN --json
```

## Navigation Updates

Recommended navigation:

- Docs
- Quickstart
- Self-host Relay
- MCP clients
- Architecture
- GitHub
- Sponsor / Gateway waitlist, if relevant

Recommended repository doc targets:

- Quickstart: `docs/quickstart-local.md`
- Self-host Relay: `docs/quickstart-self-host-relay.md`
- VPS Relay activation: `docs/activation-vps-relay.md`
- Local connector activation: `docs/activation-local-connector.md`
- MCP clients: `docs/mcp/integrations.md`
- Relay MCP tools: `docs/mcp/relay_tools.md`
- Architecture: `docs/architecture/mcp-gateway-model.md`
- Project config: `docs/project-config.md`
- Troubleshooting: `docs/TROUBLESHOOTING.md`

## Status Matrix Copy

| Area | Status |
| --- | --- |
| Local CLI/daemon | RC, deterministic proof available |
| Project config | RC, committed `.codencer/project.json` |
| Machine identity/routing | RC, selector support by `machine_id` or `host_label` |
| Self-host Relay | RC, deterministic relay/MCP smoke available |
| Local connector | RC, explicit project sharing |
| Relay MCP tools | RC, project-aware tools |
| Codex MCP setup | Setup artifacts generated; live client proof pending unless run |
| Claude Code MCP setup | Setup artifacts generated; live client proof pending unless run |
| ChatGPT custom MCP app setup | Setup sheet/OAuth dev support; product UI proof pending unless run |
| Release snapshots | darwin and linux/amd64 packaging; not signed/notarized |
| Windows path | WSL2/Linux; Windows-native daemon not claimed |
| Hosted Gateway/Cloud | Future managed layer; not shipped in this repo |

## License And Trademark Note

Codencer Core is Apache-2.0 open-source software. Apache-2.0 lets users use,
modify, self-host, fork, and use the open-source core commercially. It does not
grant rights to Codencer trademarks, logos, domains, Official Codencer MCP,
Codencer Gateway, Codencer Cloud, or hosted Codencer services. Forks and hosted
services must use their own name and must not imply they are the official
Codencer service.

## Implementation Checklist

- Replace old beta/version copy with `v0.3.0-local-prod-rc.1`.
- Remove or demote old beta-track language from primary homepage sections.
- Keep self-host Relay as the current OSS remote path.
- Present Gateway/Cloud only as a future managed layer.
- Add project config and machine-aware routing sections.
- Add MCP client setup section for ChatGPT, Claude Code, and Codex.
- Link to current docs from the repository.
- Include Apache-2.0 and trademark note.
- Keep live product proof pending unless real evidence exists.
