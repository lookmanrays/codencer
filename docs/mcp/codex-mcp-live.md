# Codex MCP Activation

Codex MCP activation generates configuration artifacts only. It does not write user-level `.codex/config.toml` files.

## Generate

```bash
./bin/codencer activation codex \
  --relay https://relay.example.com \
  --token-env CODENCER_MCP_TOKEN \
  --project codencer \
  --json
```

The output includes:

- `codex mcp add ... --bearer-token-env-var CODENCER_MCP_TOKEN`
- project/user TOML snippets
- endpoint check
- prompts and evidence checklist

## Suggested Prompt

Ask Codex to:

1. Use `codencer.list_projects` first.
2. Inspect the intended project with `codencer.get_project`.
3. Use `codencer.run_project_manifest` only for approved multi-step work.
4. Use `codencer.submit_project_task_and_wait` for one approved task.
5. Return blocker JSON when planner decision is required.

Live Codex client proof remains pending until Codex actually connects to the MCP endpoint and calls a tool.
