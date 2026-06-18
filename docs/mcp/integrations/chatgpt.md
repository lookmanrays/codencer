# ChatGPT Custom MCP Integration Notes

ChatGPT product proof is manual and workspace-gated. This repository can prepare
the self-host Relay, OAuth dev metadata, activation package, tool list, and test
prompts, but it must not claim ChatGPT product UI proof until an operator
actually exercises ChatGPT and saves evidence.

Use this page with:

- [ChatGPT custom MCP app setup](../chatgpt-app-setup.md)
- [ChatGPT OAuth dev mode](../chatgpt-oauth-dev.md)
- [Relay MCP tools](../relay_tools.md)
- [MCP integrations](../integrations.md)

## Current Supported Surface

ChatGPT must target the self-host Relay MCP endpoint:

```text
https://<relay-host>/mcp
```

Do not point ChatGPT at:

- the local daemon;
- `http://127.0.0.1`;
- WSL-only loopback URLs;
- old cloud-control-plane MCP examples;
- any hosted Codencer Gateway/Cloud URL unless an official managed service
  exists and is explicitly documented.

## Prerequisites

- A public HTTPS self-host Relay.
- A valid planner token, or Relay OAuth dev mode for single-user testing.
- A local connector enrolled with the Relay.
- At least one explicitly shared project.
- An eligible ChatGPT workspace with custom MCP/developer mode access.

Generate setup materials:

```bash
codencer setup relay --base-url https://relay.example.com --generate-planner-token --enable-chatgpt-oauth-dev --json
codencer setup mcp --client chatgpt --endpoint https://relay.example.com/mcp --json
codencer activation package --relay https://relay.example.com --project codencer --token-env CODENCER_MCP_TOKEN --json
codencer activation chatgpt --relay https://relay.example.com --project codencer --auth oauth --json
```

## Narrow Product Smoke

Only mark ChatGPT proof passed when all of this evidence exists:

1. The ChatGPT workspace has custom MCP/developer mode enabled.
2. The Codencer MCP app/connector is configured against the public Relay URL.
3. ChatGPT initializes the MCP session successfully.
4. ChatGPT calls `codencer.list_projects`.
5. If execution proof is claimed, ChatGPT calls an execution tool against a
   shared fake/local project and Codencer returns a structured result or blocker.
6. Evidence is saved with timestamps and the exact Relay endpoint used.

Until then, ChatGPT proof is pending/manual, not passed.

## Expected Tools

The Relay exposes project-aware `codencer.*` tools. See
[Relay MCP tools](../relay_tools.md) for the exact tool list and selector
behavior.

Project listings include `locations[]`. If more than one online machine
advertises the same `project_id`, ChatGPT must provide `machine_id` or
`host_label`; otherwise Codencer returns structured blocker
`ambiguous_project_location`.

## Troubleshooting

- If ChatGPT cannot reach the MCP server, confirm the Relay URL is public HTTPS.
- If auth fails, confirm OAuth dev metadata or the operator-owned auth front
  door forwards a bearer token Codencer accepts.
- If no projects appear, confirm connector enrollment and `codencer project
  share`.
- If execution is ambiguous, pass `machine_id` or `host_label`.
