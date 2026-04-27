# Claude Desktop and claude.ai — Codencer Beta Walkthrough

## Status

Codencer now treats the Claude Code-style HTTP MCP operator lane as `operator-packaged`.

Claude Desktop and `claude.ai` remote custom connectors remain `compatibility-only` because product-side auth/setup behavior is outside repo proof.

The only planner-side Codencer surfaces for Anthropic products are:

- relay MCP at `/mcp`
- cloud MCP at `/api/cloud/v1/mcp`

Keep the distinction explicit:

- Claude Desktop and `claude.ai` remote custom connectors are a planner-side flow.
- The local `claude` adapter in Codencer is an executor-side adapter.
- Claude Code is a separate product path with its own local/project MCP configuration model.

For the frozen beta boundary, start with [Public Beta Test Tracks](../../BETA_TESTING.md) and [Planner / Client Integration Notes](../integrations.md).

## Prerequisites

- A working Codencer self-host relay from [Self-Host Relay / Runtime Reference](../../SELF_HOST_REFERENCE.md) or a working composed cloud control plane from [Codencer Self-Host Cloud Control Plane Guide](../../CLOUD_SELF_HOST.md).
- A public HTTPS URL for the Codencer planner-side MCP surface:
  - relay: `https://<host>/mcp`
  - cloud: `https://<host>/api/cloud/v1/mcp`
- A valid Codencer bearer token for that surface:
  - relay planner token for relay `/mcp`
  - cloud API token for cloud `/api/cloud/v1/mcp`
- A Claude account with access to remote custom connectors in Claude Desktop or `claude.ai`.

Anthropic's current help states that remote custom connectors are brokered through your Claude account and connect from Anthropic's cloud infrastructure, not from your local machine. Verify against:

- [Get started with custom connectors using remote MCP](https://support.claude.com/en/articles/11175166-get-started-with-custom-connectors-using-remote-mcp)
- [Build custom connectors via remote MCP servers](https://support.claude.com/en/articles/11503834-building-custom-connectors-via-remote-mcp-servers)

That means `http://127.0.0.1`, `http://localhost`, a WSL-only loopback URL, or a daemon-local endpoint is not the correct target for this planner-side path.

## Step 1 Choose the correct Codencer surface

Pick one planner-side Codencer MCP surface and stay on it:

- relay mode: `/mcp`
- cloud tenancy mode: `/api/cloud/v1/mcp`

Do not point Claude Desktop or `claude.ai` at:

- daemon-local `/mcp/call`
- relay `/mcp/call` as the canonical target
- cloud `/api/cloud/v1/mcp/call` as the canonical target

The compatibility-call routes remain secondary aliases. The canonical session paths are documented in [Relay MCP Tools](../relay_tools.md) and [Cloud MCP Tools](../cloud_tools.md).

When the server is reachable and authorized, the tool set exposed to Anthropic clients is the repo-defined `codencer.*` namespace, such as `codencer.list_instances`, `codencer.start_run`, and `codencer.submit_task`.

## Step 2 Keep the product boundary straight

This section separates two Claude-style lanes.

### Claude Code-Style Operator Lane

Use this lane when your Claude-style planner can consume an HTTP MCP server configuration with bearer headers.

Checked-in examples:

- [claude-code-relay.mcp.json](../examples/claude-code-relay.mcp.json)
- [claude-code-cloud.mcp.json](../examples/claude-code-cloud.mcp.json)

Relay environment:

```bash
export CODENCER_RELAY_MCP_URL=https://<your-relay-host>/mcp
export CODENCER_PLANNER_TOKEN=<planner-token>
```

Cloud environment:

```bash
export CODENCER_CLOUD_MCP_URL=https://<your-cloud-host>/api/cloud/v1/mcp
export CODENCER_CLOUD_TOKEN=<cloud-token>
```

The planner loop is the same Codencer tool sequence used by ChatGPT-style clients:

1. `codencer.list_instances`
2. `codencer.get_instance`
3. `codencer.start_run`
4. `codencer.submit_task` with `adapter_profile: "codex"`
5. `codencer.wait_step`
6. `codencer.get_step_result`
7. `codencer.get_step_validations`
8. `codencer.get_step_logs`
9. `codencer.list_step_artifacts`
10. `codencer.list_run_gates`

Run the Codencer-side proof first:

```bash
make flagship-planner-smoke
```

### Claude Desktop / claude.ai Remote Connector Lane

This walkthrough is for Claude Desktop and `claude.ai` remote custom connectors.

It is not the Claude Code setup flow.

Current Anthropic docs show two separate mechanisms:

- Remote custom connectors for Claude Desktop and `claude.ai` are added through the product UI under `Customize > Connectors` or organization `Settings > Connectors`.
- `claude_desktop_config.json` is the separate local-MCP mechanism for the Claude Desktop chat app.
- If you are looking for the local-MCP file specifically, the common desktop locations are `~/Library/Application Support/Claude/claude_desktop_config.json` on macOS and `%APPDATA%\\Claude\\claude_desktop_config.json` on Windows.

Current Anthropic references to verify this distinction:

- [Get started with custom connectors using remote MCP](https://support.claude.com/en/articles/11175166-get-started-with-custom-connectors-using-remote-mcp)
- [Connect Claude Code to tools via MCP](https://code.claude.com/docs/en/mcp)
- [Use Claude Code Desktop](https://code.claude.com/docs/en/desktop)

Do not publish or rely on a remote-connector-via-`claude_desktop_config.json` claim. That would be false as of 2026-04-24.

## Step 3 Make the endpoint reachable from Anthropic

Before you touch the Claude UI, verify the Codencer endpoint shape operationally:

- self-host relay operators should use the relay path in [Self-Host Relay / Runtime Reference](../../SELF_HOST_REFERENCE.md)
- cloud operators should use the composed runtime path in [Codencer Self-Host Cloud Control Plane Guide](../../CLOUD_SELF_HOST.md)

Operational checks:

- the URL is public and reachable from Anthropic's cloud
- the endpoint is relay `/mcp` or cloud `/api/cloud/v1/mcp`
- the local daemon is not exposed directly
- connector sharing and runtime claiming are already correct on the Codencer side

Anthropic's help also states that private-network, VPN-only, or firewall-blocked endpoints will not connect unless Anthropic's source IP ranges are allowed. Use the help article above as the product-side source of truth.

## Step 4 Add the remote connector in Claude Desktop or claude.ai

Use Anthropic's current remote connector flow exactly as documented by Anthropic.

For Pro and Max plans, Anthropic currently documents:

1. Open `Customize > Connectors`.
2. Choose `Add custom connector`.
3. Enter the remote MCP server URL.
4. Optionally provide OAuth client settings if your server expects them.

For Team and Enterprise plans, Anthropic currently documents:

1. An owner adds the connector in organization `Settings > Connectors`.
2. Individual members then enable it from `Customize > Connectors`.

Codencer-specific values to carry into that UI:

- relay URL: `https://<host>/mcp`
- cloud URL: `https://<host>/api/cloud/v1/mcp`

Use the checked-in JSON files only as value-reference examples:

- [claude-desktop-relay.mcp.json](../examples/claude-desktop-relay.mcp.json)
- [claude-desktop-cloud.mcp.json](../examples/claude-desktop-cloud.mcp.json)

They are not direct import artifacts for the Anthropic remote connector UI.

## Step 5 Reconcile the current auth reality before expecting success

Codencer's current public self-host auth model on these planner-side surfaces is bearer-token based.

That is visible throughout the Codencer operator docs:

- relay examples use `Authorization: Bearer <planner-token>`
- cloud examples use `Authorization: Bearer <cloud-token>`

Anthropic's current public help for remote custom connectors documents entering the server URL and, optionally, OAuth client settings. It does not document a raw static header field for injecting a Codencer bearer token through the remote connector UI.

Treat that as a real compatibility boundary:

- Codencer's remote MCP surface is repo-documented and repo-tested.
- Anthropic's product-side remote connector UI flow is current external platform behavior.
- The combination remains `compatibility-only` because this repo does not directly prove Anthropic's product-specific auth and setup path.

If your Anthropic-side connector setup requires OAuth and your Codencer endpoint only accepts static bearer tokens, you may need an operator-owned auth front door or another compatibility layer before the product flow can succeed end to end.

## Step 6 Enable the connector for a conversation

Once the connector is added successfully in Anthropic's UI:

1. Open a Claude Desktop or `claude.ai` conversation.
2. Enable the connector from the conversation's connectors/tools picker.
3. Ask Claude to list the available Codencer tools or to call `codencer.list_instances`.

Expected shape:

- Claude sees `codencer.*` tools from the relay or cloud MCP server
- Claude operates against the shared instance list already authorized in Codencer
- Claude does not gain raw shell or arbitrary filesystem access through this path

## Verification smoke

Run Codencer-side proof before doing any Anthropic product check:

- Flagship relay path: `make flagship-planner-smoke`
- Relay path: [Public Beta Test Tracks](../../BETA_TESTING.md) and [Planner / Client Integration Notes](../integrations.md) point to `PLANNER_TOKEN=<planner-token> make self-host-smoke-mcp`
- Cloud path: use the composed cloud proof in [Codencer Self-Host Cloud Control Plane Guide](../../CLOUD_SELF_HOST.md)

Then perform a narrow operator smoke in Claude Desktop or `claude.ai`:

1. Enable the connector for one conversation.
2. Ask Claude to call `codencer.list_instances`.
3. Confirm that the returned instances match the Codencer relay or cloud visibility you already proved outside the Anthropic UI.

Keep the claim narrow:

- Codencer proves the relay/cloud MCP surface.
- This repo does not prove Anthropic's product UI flow end to end.

## Known Limitations on this surface

- Claude Code-style HTTP MCP operator use is `operator-packaged`; Claude Desktop and `claude.ai` remote connector setup remains `compatibility-only`, not direct product proof.
- The local `claude` adapter is an executor-side adapter and does not convert this planner-side path into a repo-proven Anthropic integration.
- Claude Code is separate. Its local/project MCP setup is documented elsewhere and should not be conflated with Claude Desktop or `claude.ai` remote custom connectors.
- `claude_desktop_config.json` is a separate local-MCP mechanism and is not the configuration source for Anthropic remote custom connectors.
- Codencer's documented public planner auth is currently static bearer-token based. Anthropic's current remote custom connector help centers on URL plus optional OAuth settings.
- The local daemon remains unsupported as a public remote planner target.

## Troubleshooting

- Connector cannot reach the server: verify that the target is a public relay or cloud MCP URL, not `localhost`, not WSL loopback, and not the daemon-local MCP path.
- Connector is added but tool calls fail with auth errors: re-check the current auth mismatch between Anthropic's documented remote custom connector setup and Codencer's current bearer-token requirement.
- No instances appear: verify relay sharing or cloud runtime claiming first on the Codencer side before debugging the Anthropic client.
- You only see local MCP docs: you are likely in the Claude Code or Claude Desktop local-MCP path. Return to Anthropic's remote connector help and use the product UI flow instead.
- The doc examples look importable: they are intentionally value-reference files only. Anthropic's product UI and docs remain the source of truth for actual setup steps.
