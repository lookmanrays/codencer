# ChatGPT Custom MCP Live Checklist

ChatGPT product proof is manual until an operator actually exercises the product UI and saves evidence. Do not mark ChatGPT live proof as passed from repo-only tests.

## Prerequisites

- Public HTTPS Gateway endpoint, such as `https://gateway.example.com/mcp`.
- Gateway OAuth dev mode or equivalent OAuth/front-door mode when required by
  the ChatGPT workspace.
- Eligible ChatGPT workspace with developer mode/custom MCP access.
- Scoped Gateway bearer token or OAuth token exchange with `projects:read`,
  `runs:*`, `steps:*`, `artifacts:*`, and `reports:read`.
- Backend Relay profile configured in Gateway.
- A project shared with `codencer project share`.
- Connector online.
- Local daemon online.

## Manual Test

1. Connect the MCP server in ChatGPT.
2. List tools and verify project-aware `codencer.*` tools are present.
3. Run `codencer.list_projects`.
4. Run `codencer.submit_project_task_and_wait` against a fake project.
5. Run `codencer.run_project_manifest` with the fake success manifest.
6. Fetch the run report with `codencer.get_run_report` or `codencer.get_execution_report`.
7. Run the fake blocker manifest and verify the structured blocker is visible.

## Evidence To Save

- Screenshots.
- Exported transcript.
- Relay audit log.
- Run report JSON.
- Timestamp.
- Codencer version/build.
- Gateway endpoint used and selected backend Relay profile.

## Acceptance Status

Keep ChatGPT product proof marked pending until real evidence is attached.

Gateway activation generates values for the product flow without claiming a
pass:

```bash
./bin/codencer activation self-host --gateway https://gateway.example.com --relay https://relay.example.com --project codencer --token-env CODENCER_GATEWAY_MCP_TOKEN --json
```

The command is configuration proof only. A passed ChatGPT gate requires an
actual eligible workspace, public HTTPS, Gateway OAuth dev mode or an
operator-owned OAuth front door, saved connector configuration, a real tool
call, and saved result evidence.
