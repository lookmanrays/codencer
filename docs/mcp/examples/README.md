# MCP Example Files

These files are packaging references for operator setup. They are not all importable by every product UI.

| File | Status | Use |
| --- | --- | --- |
| `chatgpt-relay.mcp.json` | value-reference only | Copy endpoint/token values into the current ChatGPT remote MCP app setup flow. |
| `chatgpt-cloud.mcp.json` | value-reference only | Same for cloud `/api/cloud/v1/mcp`. |
| `claude-code-relay.mcp.json` | Claude Code-style HTTP MCP config shape | Use when the planner client supports HTTP MCP plus bearer headers. |
| `claude-code-cloud.mcp.json` | Claude Code-style HTTP MCP config shape | Same for cloud `/api/cloud/v1/mcp`. |
| `claude-desktop-relay.mcp.json` | value-reference only | Not a `claude_desktop_config.json` import guarantee. |
| `claude-desktop-cloud.mcp.json` | value-reference only | Not a `claude_desktop_config.json` import guarantee. |
| `gemini-cli-relay.mcp.json` | expected-only packaging | Config shape only; repo does not prove Gemini CLI product setup. |
