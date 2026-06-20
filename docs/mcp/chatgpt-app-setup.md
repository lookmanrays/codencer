# ChatGPT Custom MCP App Setup

ChatGPT product proof is manual and workspace-gated. This repository can prepare the server, OAuth/dev-noauth config, MCP metadata, tool list, and activation package, but it cannot claim product UI proof until an operator exercises ChatGPT.

## Server Preconditions

```bash
./bin/codencer setup self-host \
  --gateway-url http://127.0.0.1:19090 \
  --mcp-url http://127.0.0.1:19090/mcp \
  --relay-url http://127.0.0.1:8090 \
  --default-relay-token-env CODENCER_DEFAULT_RELAY_TOKEN \
  --token-env CODENCER_GATEWAY_MCP_TOKEN \
  --enable-oauth-dev \
  --json

./bin/codencer login --gateway http://127.0.0.1:19090
./bin/codencer connector login --gateway http://127.0.0.1:19090 --relay default --json

./bin/codencer activation self-host \
  --gateway http://127.0.0.1:19090 \
  --relay http://127.0.0.1:8090 \
  --project codencer \
  --token-env CODENCER_GATEWAY_MCP_TOKEN \
  --json
```

Save the generated client secret and operator approval code from `$CODENCER_HOME/tokens`.

`codencer activation self-host --json` writes a package containing ChatGPT setup
that points to Gateway. `codencer activation chatgpt --json` remains available
for direct Relay debug setup. The generated setup sheet includes:

- `mcp_endpoint`
- `auth_mode`
- `client_id`
- `client_secret_file`
- `operator_code_file`
- `authorization_server_metadata`
- `openid_configuration`
- `authorization_endpoint`
- `token_endpoint`
- `protected_resource_metadata`
- `scopes`
- `expected_tools`
- `chatgpt_ui_steps`
- `test_prompts`
- `evidence_checklist`

Literal secrets are redacted. File paths are shown only for local token files.

## App Values

- MCP endpoint: `http://127.0.0.1:19090/mcp`
- OAuth authorization server metadata: `http://127.0.0.1:19090/.well-known/oauth-authorization-server`
- OpenID configuration: `http://127.0.0.1:19090/.well-known/openid-configuration`
- Protected resource metadata: `http://127.0.0.1:19090/.well-known/oauth-protected-resource/mcp`
- Backend Relay profile target: `http://127.0.0.1:8090`
- Required behavior: use `codencer.list_projects` first.
- OAuth dev redirect behavior: valid redirect URIs are accepted for self-host dev testing. Production must use redirect allowlisting or an external IdP. OAuth dev mode is not public multi-user production.

Expected project-aware tools include:

- `codencer.list_relays`
- `codencer.get_relay`
- `codencer.list_projects`
- `codencer.get_project`
- `codencer.list_project_locations`
- `codencer.submit_project_task_and_wait`
- `codencer.run_project_manifest`
- `codencer.get_run_report`
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
