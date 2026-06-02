# codencer.dev Update Pack

Status: `site repo update pending`.

This repository does not contain the codencer.dev website source. The only checked-in `package.json` is `extension/package.json`, which is unrelated to the public site. Apply this pack in the codencer.dev site repository.

## Release Banner Copy

Title: `Codencer self-host activation is ready for operator preflight`

Body:

Codencer is a bridge for approved coding-agent work. The new activation flow packages connector enrollment, project-aware Relay/MCP setup, Codex and Claude Code snippets, ChatGPT OAuth-dev setup guidance, and a real MCP curl smoke that initializes a session, lists tools, and calls `codencer.list_projects`.

CTA:

- Primary: `Read the activation guide`
- Secondary: `Download the release bundle`

## Navigation Updates

Add or update links:

- `Quickstart Local` -> `docs/quickstart-local.md`
- `Self-Host Relay` -> `docs/quickstart-self-host-relay.md`
- `Activation VPS Relay` -> `docs/activation-vps-relay.md`
- `Activation Local Connector` -> `docs/activation-local-connector.md`
- `MCP Integrations` -> `docs/mcp/integrations.md`
- `MCP Gateway Model` -> `docs/architecture/mcp-gateway-model.md`

## Product Truth Copy

Use this wording verbatim where the site describes product scope:

> Codencer is a bridge, not a planner. It exposes one project-aware MCP toolset through a self-host Relay and local connector. Execution stays local and daemon-first; Relay provides transport, auth, routing, and audit. Codencer surfaces structured state and blockers so the planner can decide.

Avoid:

- claiming hosted Codencer Cloud availability;
- claiming commercial billing or marketplace support;
- saying ChatGPT/Codex/Claude live product proof has passed unless evidence exists;
- presenting the local daemon as the public remote MCP endpoint.

## Activation Section

Add a section named `Self-Host Activation` with these commands:

```bash
codencer activation package --relay https://relay.example.com --project codencer --token-env CODENCER_MCP_TOKEN --json
codencer connector enroll --relay-url https://relay.example.com --daemon-url http://127.0.0.1:8085 --enrollment-token "$CODENCER_CONNECTOR_ENROLLMENT_TOKEN" --config "$CODENCER_HOME/runtime/connector/config.json" --json
codencer activation check --relay https://relay.example.com --project codencer --token-env CODENCER_MCP_TOKEN --json
```

Mention that `connector-enrollment.sh`, `curl-smoke.sh`, `codex-config.toml`, `claude-code-command.sh`, and `chatgpt-app-setup.md` are generated into `$CODENCER_HOME/artifacts/activation/<timestamp>/`.

## ChatGPT Copy

Use this wording:

> ChatGPT custom MCP setup requires a public HTTPS relay and an eligible workspace. Codencer can generate a setup sheet with endpoint, OAuth metadata, scopes, expected tools, UI steps, test prompts, and evidence checklist. The self-host OAuth dev issuer is single-user development tooling; production should use redirect allowlisting or an external IdP/front door.

## Acceptance Notes

Show GO gates as repository-side proof:

- connector enrollment documented;
- activation package includes connector enrollment;
- Claude Code command order is valid;
- ChatGPT setup sheet is complete;
- curl smoke uses real MCP initialize/tools/call flow;
- MCP gateway model doc is present.

Keep these pending unless exercised:

- live Codex product proof;
- live Claude product proof;
- ChatGPT product UI proof;
- WSL live proof;
- installed service proof.
