# Local Connector Activation

The connector advertises explicitly shared local projects to a self-host relay. It does not expose arbitrary filesystem access or a raw shell.

## Prepare Local Project

```bash
make build
./bin/codencer setup local \
  --project-id codencer \
  --repo . \
  --adapter codex \
  --profile codex-workspace \
  --json
```

Start the local daemon by your existing operator flow or the Sprint 4 supervisor. Normal `codencer submit` commands do not auto-start production daemons.

## Share Project

```bash
./bin/codencer project share codencer \
  --daemon-url http://127.0.0.1:8085 \
  --json
```

Sharing is explicit. The user-level registry keeps full local paths; relay planner payloads expose safe labels and hashes.

## Connect To Relay

Create relay and connector config:

```bash
./bin/codencer setup relay \
  --base-url https://relay.example.com \
  --generate-planner-token \
  --json
```

Run the connector with the generated connector config:

```bash
./bin/codencer-connectord run --config "$CODENCER_HOME/runtime/connector/config.json"
```

## Verify Advertisement

```bash
export CODENCER_MCP_TOKEN=<planner-token>
./bin/codencer activation check \
  --relay https://relay.example.com \
  --project codencer \
  --token-env CODENCER_MCP_TOKEN \
  --json
```

The check verifies protected-resource metadata, unauthenticated MCP challenge behavior, MCP initialize/tools list, project visibility, and path redaction. Add `--run-fake-manifest` only when the fake project/profile is intentionally registered for server preflight.

## Planner Protocol

Remote planners should:

1. Call `codencer.list_projects` first.
2. Call `codencer.submit_project_task_and_wait` for one approved task.
3. Call `codencer.run_project_manifest` for approved multi-step work.
4. Stop and return blocker details when `planner_decision_required:true`.
5. Never infer a next action from logs.
