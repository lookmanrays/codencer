# Flagship Planner Product Path

> [!WARNING]
> Historical product-path hardening note. This is not the current v0.3 local/self-host RC release contract. Use [README](../../README.md), [MCP Integrations](../mcp/integrations.md), and [MCP Gateway Model](../architecture/mcp-gateway-model.md) for current guidance.

Status: post-beta publishability/use hardening contract

Last audited: 2026-04-28

This document freezes the narrow product-facing path for cloud planners that use Codencer to drive local Codex. It does not reopen the beta support contract and it does not claim universal ChatGPT, Claude, or generic MCP-client compatibility.

Codencer remains the bridge and resource server. The planner decides the next step. Codencer authenticates, routes, executes one submitted task, records evidence, and returns structured state.

## Canonical Endpoints

Use one of these remote MCP endpoints:

| Surface | Canonical MCP endpoint | OAuth protected-resource metadata | Compatibility alias |
| --- | --- | --- | --- |
| Self-host relay | `https://<relay-host>/mcp` | `https://<relay-host>/.well-known/oauth-protected-resource/mcp` | `POST /mcp/call` only |
| Self-host cloud, composed runtime | `https://<cloud-host>/api/cloud/v1/mcp` | `https://<cloud-host>/.well-known/oauth-protected-resource/api/cloud/v1/mcp` | `POST /api/cloud/v1/mcp/call` only |

The alias routes are for simple POST callers. They are not the primary session endpoint and are not long-lived SSE session paths.

Do not expose or target the local daemon `/mcp/call` path from a cloud planner.

For composed cloud runtime mode, the relay config loaded by cloud must advertise the public cloud origin as its `public_base_url` when connectors enroll through cloud ingress. Otherwise the connector can attach to a separate relay process while cloud sees only stored instance records and cannot proxy live runtime calls through its in-process bridge.

## Auth Modes

### Bearer-Token Mode

Bearer-token mode is the proven private/self-host execution mode.

- Relay uses `Authorization: Bearer <planner-token>`.
- Cloud uses `Authorization: Bearer <cloud-token>`.
- This mode is covered by relay/cloud MCP tests, the official Go SDK smoke helper, `scripts/self_host_smoke.sh`, `scripts/cloud_smoke.sh`, and `scripts/flagship_planner_loop_smoke.sh`.
- This mode is appropriate for private scripts, direct HTTP clients, direct SDK clients, and Claude Code-style configs that support headers.

### OAuth-Capable Front-Door Mode

Codencer exposes OAuth protected-resource metadata and `WWW-Authenticate` challenges, but it is not an OAuth authorization server.

For product-facing remote MCP clients, the deployable pattern is:

1. Publish the Codencer MCP URL over HTTPS.
2. Configure `public_base_url` to that public origin.
3. Configure `oauth_authorization_servers` to an operator-owned OAuth/OIDC issuer or gateway.
4. Have the issuer/gateway handle authorization-code, PKCE, refresh-token, and policy requirements for the product client.
5. Have the gateway validate the product OAuth access token and forward a Codencer bearer token, or mint a token that Codencer already accepts.
6. Keep Codencer scoped by relay planner-token scopes or cloud API-token scopes.

Concrete front-door setup is documented in [OAuth Front Door](../mcp/OAUTH_FRONT_DOOR.md).

## ChatGPT Product Path

Publishable/operator-usable target:

- ChatGPT custom MCP connector/app workflow on Business, Enterprise, and Edu for write/modify-capable use.
- Remote MCP only.
- OAuth front-door mode for product setup.
- Canonical endpoint: relay `/mcp` or cloud `/api/cloud/v1/mcp`.

Narrow testing/adjacent paths:

- OpenAI Responses API remote MCP can be used with a public `server_url` and an authorization token where appropriate; this is an API/client path, not the ChatGPT workspace app publish path.
- Pro/Plus availability and read/fetch-only behavior are product-plan details outside repo proof. Do not use them as the publishability baseline for write-capable Codencer operation.

Unsupported/uncounted:

- ChatGPT Agent Mode as a write-capable custom connector target.
- Local MCP servers in ChatGPT.
- Consumer-plan parity claims.
- Marketplace/public app approval.

Repo-proven boundary:

- Codencer proves the relay/cloud MCP protocol, bearer execution, OAuth protected-resource metadata/challenges, CORS-readable auth challenges, session-bound relay/cloud behavior, and local Codex loop through the flagship smoke.
- The repo does not click through ChatGPT's hosted UI or certify an OpenAI publication review.

## Claude Product Path

Publishable/operator-usable targets:

- Claude Code remote HTTP MCP with bearer headers.
- Anthropic Messages API MCP connector with a public remote HTTP MCP URL and `authorization_token`.
- Claude Desktop/claude.ai remote custom connector setup through Anthropic's connector UI, using OAuth front-door mode where the product flow requires OAuth.

Canonical endpoint:

- Relay: `https://<relay-host>/mcp`
- Cloud: `https://<cloud-host>/api/cloud/v1/mcp`

Auth:

- Claude Code private/operator path: bearer header.
- Anthropic Messages API path: `authorization_token` carrying the OAuth/bearer token accepted by the front door.
- Claude Desktop/claude.ai remote connector path: OAuth front door when the product flow requires OAuth client settings.

Repo-proven boundary:

- Claude Code-style bearer config is packaged and Codencer-side MCP behavior is proven.
- Anthropic Messages API request shape is packaged from official docs, but not executed against Anthropic's API in this repo.
- Claude Desktop/claude.ai UI setup remains product-side compatibility until the operator exercises that product flow.
- This planner path is separate from Codencer's local `claude` executor adapter.

## Flagship Loop

The external planner loop is:

1. Initialize MCP session on the canonical endpoint.
2. Call `codencer.list_instances`.
3. Select one explicit `instance_id`.
4. Call `codencer.get_instance` to confirm the target.
5. Call `codencer.start_run`.
6. Call `codencer.submit_task` with `adapter_profile: "codex"`.
7. Call `codencer.wait_step`.
8. Call `codencer.get_step_result`, `codencer.get_step_validations`, `codencer.get_step_logs`, and `codencer.list_step_artifacts`.
9. If `needs_decision` is true, call `codencer.list_run_gates`, ask the human, then call `codencer.approve_gate` or `codencer.reject_gate`.
10. The planner decides the next task externally and repeats until the phase is done.

Success states:

- `completed`
- `completed_with_warnings`

Planner decision states:

- `needs_approval`
- `needs_manual_attention`

Failure states:

- `failed_validation`
- `failed_adapter`
- `failed_bridge`
- `failed_retryable`
- `failed_terminal`
- `timeout`
- `cancelled`

Codencer never decides that a planner phase is complete.

## Proven, Expected, Compatibility-Only, Unsupported

| Claim | Label | Evidence |
| --- | --- | --- |
| Relay MCP `/mcp` with bearer auth | `proven` | relay MCP tests, self-host smoke, flagship smoke |
| Cloud MCP `/api/cloud/v1/mcp` with bearer auth | `proven` | cloud MCP/runtime tests, cloud smoke |
| OAuth protected-resource metadata and challenges | `proven` | relay/cloud MCP auth tests and smoke |
| Browser-readable MCP auth challenge headers | `proven` | relay/cloud MCP CORS tests |
| Relay/cloud MCP session ownership | `proven` | cloud token-bound sessions and relay token-bound session test |
| Local Codex selected behind the bridge | `proven with simulation and fake-binary live shape` | adapter tests and flagship smoke |
| Live authenticated Codex service run | `operator-environment proof` | `FLAGSHIP_LIVE_CODEX=1 make flagship-planner-smoke` |
| ChatGPT custom MCP connector write-capable product path | `operator-packaged` | docs/config plus Codencer-side proof; product UI not repo-clicked |
| Claude Code remote HTTP MCP bearer path | `operator-packaged` | checked-in config and Codencer-side proof |
| Anthropic Messages API MCP connector shape | `operator-packaged expected` | docs/examples aligned to official API docs; not externally executed |
| Claude Desktop/claude.ai remote connector UI | `compatibility-only until exercised` | endpoint/auth docs only |
| Generic private bearer HTTP/MCP clients | `proven` | curl/smoke/SDK |
| Universal MCP client compatibility | `expected-only` | not product-executed |
| Local daemon as cloud planner target | `unsupported` | use relay/cloud only |

## Current Remaining Blockers

- Codencer does not issue OAuth authorization-code tokens.
- ChatGPT workspace publication and Anthropic product UI setup must be exercised by the operator in those products.
- Live Codex proof depends on the operator's installed and authenticated `codex` CLI.
- Generic MCP clients outside the checked Go SDK and documented product paths are not individually certified.
