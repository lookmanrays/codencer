# ChatGPT Custom MCP App Setup

ChatGPT product proof is manual and workspace-gated. This repository can prepare the server, OAuth/dev-noauth config, MCP metadata, tool list, and activation package, but it cannot claim product UI proof until an operator exercises ChatGPT.

## Server Preconditions

```bash
./bin/codencer setup relay \
  --base-url https://relay.example.com \
  --mcp-url https://relay.example.com/mcp \
  --generate-planner-token \
  --enable-chatgpt-oauth-dev \
  --json

./bin/codencer activation chatgpt \
  --relay https://relay.example.com \
  --project codencer \
  --auth oauth \
  --json
```

Save the generated client secret and operator approval code from `$CODENCER_HOME/tokens`.

## App Values

- MCP endpoint: `https://relay.example.com/mcp`
- OAuth authorization server metadata: `https://relay.example.com/.well-known/oauth-authorization-server`
- OpenID configuration: `https://relay.example.com/.well-known/openid-configuration`
- Protected resource metadata: `https://relay.example.com/.well-known/oauth-protected-resource/mcp`
- Required behavior: use `codencer.list_projects` first.

Expected project-aware tools include:

- `codencer.list_projects`
- `codencer.get_project`
- `codencer.submit_project_task_and_wait`
- `codencer.run_project_manifest`
- `codencer.get_project_blocker`
- `codencer.get_blocker`

## Operator Steps

1. Confirm the ChatGPT workspace has Developer Mode/custom MCP app access.
2. Create a draft custom MCP app named Codencer.
3. Use the MCP endpoint above.
4. Use OAuth mode against the Codencer OAuth dev metadata for self-host testing, or an operator-owned OAuth front door for production IAM.
5. Complete the authorization flow with the operator approval code.
6. Scan/list tools and confirm project-aware tools are present.
7. Ask ChatGPT to call `codencer.list_projects`.
8. For an approved one-task proof, ask it to call `codencer.submit_project_task_and_wait`.
9. For an approved manifest proof, ask it to call `codencer.run_project_manifest`.
10. If the response includes a blocker requiring planner decision, stop and save the blocker JSON.

Do not publish or mark live ChatGPT proof passed until `client_connected`, `client_used_tool`, and `full_e2e_execution` evidence is saved.
