# Gemini CLI — Codencer Beta Walkthrough

## Status

This path is `expected-only` in the current Codencer beta contract. The Codencer relay MCP surface itself is repo-proven; Gemini CLI product setup is documented here as an operator packaging path aligned to the current official Gemini CLI references, not as a repo-executed product proof.

This pass was written on `2026-04-24`. The local environment for this pass did not have `gemini` installed, so nothing in this walkthrough was locally validated from this host. Treat the commands and expected outcomes below as doc-aligned operator guidance.

Current references used for this page:

- Codencer beta boundaries: [../../BETA_TESTING.md](../../BETA_TESTING.md)
- Codencer planner/client matrix: [../integrations.md](../integrations.md)
- Codencer self-host relay/runtime flow: [../../SELF_HOST_REFERENCE.md](../../SELF_HOST_REFERENCE.md)
- Codencer relay MCP tools: [../relay_tools.md](../relay_tools.md)
- Gemini CLI configuration reference: [configuration.md](https://github.com/google-gemini/gemini-cli/blob/main/docs/reference/configuration.md)
- Gemini CLI MCP server reference: [mcp-server.md](https://github.com/google-gemini/gemini-cli/blob/main/docs/tools/mcp-server.md)
- Gemini CLI releases: [releases](https://github.com/google-gemini/gemini-cli/releases)

The current stable Gemini CLI release referenced from the official release docs was `v0.39.1`, published on `2026-04-24`.

## Prerequisites

Before you wire Gemini CLI to Codencer, have the relay path working first:

- a running Codencer relay exposing the canonical remote MCP endpoint at `/mcp`
- a valid planner bearer token for that relay
- a running connector with at least one explicitly shared instance
- operator familiarity with the relay/runtime flow in [../../SELF_HOST_REFERENCE.md](../../SELF_HOST_REFERENCE.md)
- Gemini CLI installed on the operator machine by following the current official Gemini CLI install path

Keep the public planner target narrow:

- use relay `/mcp`
- do not point Gemini CLI at the local daemon `/mcp/call`
- use a Gemini MCP server alias without underscores; the Gemini CLI configuration reference explicitly warns that underscores in MCP aliases can break policy parsing

## Step 1 Configure the Codencer relay prerequisites

Bring up the relay, planner token, daemon, and connector exactly as described in [../../SELF_HOST_REFERENCE.md](../../SELF_HOST_REFERENCE.md). The important operator truth for Gemini CLI is the same as for every other remote MCP client:

- the relay is the public remote control plane
- the daemon is not the public remote MCP target
- the canonical remote MCP session path is relay `/mcp`
- bearer-token auth is required on that relay surface

If you still need to establish the relay token and connector share state, stop here and finish the self-host reference flow first.

## Step 2 Add the relay MCP server to Gemini CLI

Gemini CLI's current MCP docs support streamable HTTP servers through `httpUrl` plus custom request `headers`. For Codencer relay mode, use the checked-in example at [../examples/gemini-cli-relay.mcp.json](../examples/gemini-cli-relay.mcp.json).

Minimal project-scoped Gemini settings shape:

```json
{
  "mcpServers": {
    "codencer-relay": {
      "httpUrl": "${CODENCER_RELAY_MCP_URL:-http://127.0.0.1:8090/mcp}",
      "headers": {
        "Authorization": "Bearer ${CODENCER_PLANNER_TOKEN}"
      }
    }
  }
}
```

Operational notes:

- `httpUrl` is the right Gemini CLI field for Codencer relay `/mcp`, because the relay's canonical MCP path is streamable HTTP, not an SSE-only endpoint
- the bearer token belongs in the `Authorization` header
- the example keeps the token and URL environment-driven so operators do not hard-code secrets into checked-in config

If you prefer the Gemini CLI helper command instead of editing `settings.json` directly, the current official docs show the equivalent pattern:

```bash
gemini mcp add --transport http \
  --header "Authorization: Bearer ${CODENCER_PLANNER_TOKEN}" \
  codencer-relay \
  "${CODENCER_RELAY_MCP_URL:-http://127.0.0.1:8090/mcp}"
```

## Step 3 Confirm Gemini CLI sees the Codencer server

From the project where the Gemini settings apply, use the Gemini CLI MCP inspection command documented by Gemini:

```bash
gemini mcp list
```

Expected operator outcome on a correctly wired host:

- the `codencer-relay` server appears in the list
- transport is `http`
- connection state is reported as connected

If this step fails, fix transport, URL, auth, or relay share state before you attempt a mutating Codencer workflow.

## Step 4 Run a planning example through Codencer MCP tools

Use a planning-first prompt that forces Gemini CLI to inspect the environment before it mutates state. Example operator prompt:

```text
Use the Codencer relay MCP server only.
First call codencer.list_instances and stop if there is not exactly one healthy shared instance.
If there is one healthy shared instance, start a run with id gemini-plan-demo for project docs-demo.
Then submit a planning-only task: "Plan the smallest documentation change needed to tighten Codencer relay troubleshooting. Do not edit files."
Wait for the step to finish, then report the run id, step id, final step status, and a short summary of the plan.
```

Expected Codencer tool flow on the relay surface:

1. `codencer.list_instances`
2. `codencer.start_run`
3. `codencer.submit_task`
4. `codencer.wait_step`
5. `codencer.get_step_result`

Gemini CLI namespaces discovered MCP tools with an `mcp_` prefix plus the configured server alias. The underlying Codencer tool names are still the ones documented in [../relay_tools.md](../relay_tools.md), so the operator intent above remains grounded in `codencer.*`.

## Step 5 Handle the result like an operator

Treat the returned status as the control point, not the model prose alone.

If the planning step succeeds:

- record the returned `run_id` and `step_id`
- inspect the plan summary from `codencer.get_step_result`
- if you need structured proof before acting, fetch validations with `codencer.get_step_validations`

If the run stops behind approval or another gate:

- list gates with `codencer.list_run_gates`
- approve or reject only after the operator reviews the returned evidence

If the step fails or times out:

- fetch logs with `codencer.get_step_logs`
- confirm that the relay still sees the intended shared instance
- retry only after fixing the underlying transport, auth, or instance-selection issue

## Verification smoke

This smoke is `expected-only` for this documentation pass. It was not executed locally here because `gemini` was not installed on this host on `2026-04-24`.

On an operator machine that does have Gemini CLI installed, use this narrow smoke:

```bash
gemini mcp list
```

Then, from an interactive Gemini session in the configured project, ask for a non-destructive first call:

```text
Use the Codencer relay MCP server only. Call codencer.list_instances and report only the instance ids and share state.
```

Expected smoke outcome:

- Gemini CLI connects to the `codencer-relay` HTTP MCP server
- Codencer returns one or more shared instances, or an empty list that truthfully reflects relay state
- no local daemon MCP endpoint is involved

For Codencer-side proof boundaries, keep [../../BETA_TESTING.md](../../BETA_TESTING.md) and [../integrations.md](../integrations.md) as the release truth.

## Known Limitations on this surface

- This is an `expected-only` planner integration path in the current Codencer beta contract.
- This page is aligned to the official Gemini CLI configuration and MCP references, but it was not locally validated here because `gemini` was not installed on this host during this pass.
- Codencer proves the relay MCP protocol surface directly; it does not prove Gemini CLI product UX, approval behavior, or future UI wording.
- Gemini CLI aliases discovered MCP tools by server name, so the visible tool names may be prefixed even though the underlying Codencer tools remain `codencer.*`.
- This page stays on the relay self-host path. Cloud MCP can follow the same remote HTTP pattern, but that packaging example is out of scope for this pass.

## Troubleshooting

If `gemini mcp list` shows the server as disconnected:

- confirm the config uses `httpUrl`, not the SSE `url` field
- confirm the endpoint is the relay MCP path, usually `http://127.0.0.1:8090/mcp` or your deployed relay `/mcp`
- confirm the planner token is present in `Authorization: Bearer ...`

If tool calls return auth errors:

- rotate or reissue the relay planner token
- confirm the token scope still permits the intended planner operations
- verify that environment-variable expansion resolved the expected token value in Gemini's settings

If no instances appear:

- confirm the connector is enrolled and running
- confirm at least one instance is explicitly shared
- confirm you are querying the relay, not the local daemon

If Gemini appears to call the wrong tool name:

- check the configured server alias
- avoid underscores in that alias
- remember that Gemini prefixes discovered MCP tools with the MCP server alias even though the Codencer tool inventory is still the one documented in [../relay_tools.md](../relay_tools.md)
