# Claude Code MCP Activation

Claude Code MCP activation generates setup artifacts only. It does not write `.mcp.json` or user-level Claude config files.

## Generate

```bash
./bin/codencer activation self-host \
  --gateway http://127.0.0.1:19090 \
  --relay https://relay.example.com \
  --token-env CODENCER_GATEWAY_MCP_TOKEN \
  --project codencer \
  --json
```

The generated package includes a Claude Code command pointing to
`http://127.0.0.1:19090/mcp` for local self-host proof.

Direct Relay debug setup remains available:

```bash
./bin/codencer activation claude-code \
  --relay https://relay.example.com \
  --token-env CODENCER_MCP_TOKEN \
  --project codencer \
  --json
```

The Gateway package's Claude Code setup output includes a command pointing to
`http://127.0.0.1:19090/mcp`. Direct Relay debug setup output includes:

- `claude mcp add --transport http --header "Authorization: Bearer $CODENCER_MCP_TOKEN" codencer https://relay.example.com/mcp`
- `.mcp.json` alternative
- endpoint check
- prompts and evidence checklist

## Suggested Prompt

Ask Claude Code to:

1. Call `codencer.list_projects`.
2. Inspect the intended project with `codencer.get_project`.
3. Run one approved fake-success or real operator-approved task.
4. Stop and return blocker details when Codencer reports a planner decision is required.

Protocol proof is covered by `make verify-public-selfhost-release`. Live Claude
Code product-client proof remains pending until Claude Code actually connects
to the Gateway MCP endpoint and calls a tool.
