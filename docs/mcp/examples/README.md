# MCP Example Files

These files are packaging references for operator setup. They are not all importable by every product UI.

| File | Status | Auth mode | Use |
| --- | --- | --- | --- |
| `chatgpt-relay.mcp.json` | value-reference only | OAuth front door | Copy endpoint/auth values into the current ChatGPT custom MCP connector setup flow. Static bearer import is not the product baseline. |
| `chatgpt-cloud.mcp.json` | value-reference only | OAuth front door | Same for cloud `/api/cloud/v1/mcp`. |
| `claude-code-relay.mcp.json` | Claude Code-style HTTP MCP config shape | bearer header | Use when the planner client supports HTTP MCP plus bearer headers. |
| `claude-code-cloud.mcp.json` | Claude Code-style HTTP MCP config shape | bearer header | Same for cloud `/api/cloud/v1/mcp`. |
| `anthropic-messages-relay.mcp.json` | Anthropic Messages API request-shape reference | `authorization_token` | Current `mcp-client-2025-11-20` shape for relay `/mcp`; not executed against Anthropic by repo tests. |
| `anthropic-messages-cloud.mcp.json` | Anthropic Messages API request-shape reference | `authorization_token` | Same for cloud `/api/cloud/v1/mcp`; not executed against Anthropic by repo tests. |
| `claude-desktop-relay.mcp.json` | value-reference only | OAuth protected-resource front door | Not a `claude_desktop_config.json` import guarantee. |
| `claude-desktop-cloud.mcp.json` | value-reference only | OAuth protected-resource front door | Not a `claude_desktop_config.json` import guarantee. |
| `gemini-cli-relay.mcp.json` | expected-only packaging | bearer header | Config shape only; repo does not prove Gemini CLI product setup. |
| `oauth-front-door.env.example` | deployment contract | OAuth front door | Environment shape for an operator-owned front door. |
| `oauth-front-door.mapping.json` | deployment contract | OAuth front door | Token/scope mapping shape for an operator-owned front door. |
