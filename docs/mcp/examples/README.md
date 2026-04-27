# MCP Example Files

These files are packaging references for operator setup. They are not all importable by every product UI.

| File | Status | Auth mode | Use |
| --- | --- | --- | --- |
| `chatgpt-relay.mcp.json` | value-reference only | OAuth protected-resource or bearer | Copy endpoint/auth values into the current ChatGPT remote MCP app setup flow. |
| `chatgpt-cloud.mcp.json` | value-reference only | OAuth protected-resource or bearer | Same for cloud `/api/cloud/v1/mcp`. |
| `claude-code-relay.mcp.json` | Claude Code-style HTTP MCP config shape | bearer header | Use when the planner client supports HTTP MCP plus bearer headers. |
| `claude-code-cloud.mcp.json` | Claude Code-style HTTP MCP config shape | bearer header | Same for cloud `/api/cloud/v1/mcp`. |
| `claude-desktop-relay.mcp.json` | value-reference only | OAuth protected-resource front door | Not a `claude_desktop_config.json` import guarantee. |
| `claude-desktop-cloud.mcp.json` | value-reference only | OAuth protected-resource front door | Not a `claude_desktop_config.json` import guarantee. |
| `gemini-cli-relay.mcp.json` | expected-only packaging | bearer header | Config shape only; repo does not prove Gemini CLI product setup. |
