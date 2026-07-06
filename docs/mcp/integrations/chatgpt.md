# ChatGPT Custom MCP Integration Notes

ChatGPT product proof is manual and workspace-gated. This repository can prepare
Gateway OAuth dev metadata, activation packages, tool lists, and test prompts,
but it must not claim ChatGPT product UI proof until an operator actually
exercises ChatGPT and saves evidence.

Use this page with:

- [ChatGPT custom MCP app setup](../chatgpt-app-setup.md)
- [ChatGPT OAuth dev mode](../chatgpt-oauth-dev.md)
- [Self-host MCP proof](../self-host-mcp-proof.md)
- [MCP integrations](../integrations.md)

## Current Supported Surface

ChatGPT must target an operator-owned public HTTPS self-host Gateway endpoint:

```text
https://gateway.example.com/mcp
```

Do not point ChatGPT at:

- the local daemon;
- `http://127.0.0.1`;
- WSL-only loopback URLs;
- old cloud-control-plane MCP examples;
- a self-host Relay `/mcp` endpoint unless you are deliberately running
  advanced/direct/debug mode.

## Prerequisites

- A public HTTPS Gateway endpoint.
- Gateway bearer-dev auth or Gateway OAuth dev mode for single-user testing.
- A backend self-host Relay profile in Gateway.
- A local connector bound with `codencer connector login`.
- At least one explicitly shared project.
- An eligible ChatGPT workspace with custom MCP/developer mode access.

Generate setup materials:

```bash
codencer login --gateway https://gateway.example.com
codencer connector login --gateway https://gateway.example.com --relay personal --json
codencer gateway relay add --gateway https://gateway.example.com --name personal --url https://relay.example.com --token-env CODENCER_RELAY_PERSONAL_TOKEN --json
codencer activation self-host --gateway https://gateway.example.com --relay https://relay.example.com --project codencer --token-env CODENCER_GATEWAY_MCP_TOKEN --json
```

## Narrow Product Smoke

Only mark ChatGPT proof passed when all of this evidence exists:

1. The ChatGPT workspace has custom MCP/developer mode enabled.
2. The Codencer MCP app/connector is configured against the public Gateway URL.
3. ChatGPT initializes the MCP session successfully.
4. ChatGPT calls `codencer.list_projects`.
5. If execution proof is claimed, ChatGPT calls an execution tool against a
   shared fake/local project and Codencer returns a structured result or blocker.
6. Evidence is saved with timestamps and the exact Gateway endpoint used.

Until then, ChatGPT proof is pending/manual, not passed.

## Expected Tools

Gateway exposes project-aware `codencer.*` tools. See
[Self-host MCP proof](../self-host-mcp-proof.md) for the exact Gateway tool list
and selector behavior.

Project listings include `locations[]`. If more than one online machine
advertises the same `project_id`, ChatGPT must provide `machine_id` or
`host_label`; otherwise Codencer returns structured blocker
`ambiguous_project_location`.

## Troubleshooting

- If ChatGPT cannot reach the MCP server, confirm the Gateway URL is public
  HTTPS.
- If auth fails, confirm Gateway OAuth dev metadata or bearer-dev token setup.
- If no projects appear, confirm `codencer connector login`, `codencer project
  share`, and the selected Gateway relay profile.
- If execution is ambiguous, pass `machine_id` or `host_label`.
