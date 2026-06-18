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

Sharing is explicit. The user-level registry keeps full local paths and local machine metadata; relay planner payloads expose safe labels/hashes and `locations[]` with `machine_id`, `host_label`, connector/instance ids, and status. If the same `project_id` is online from multiple machines, planners must select `machine_id` or `host_label`; the relay returns `ambiguous_project_location` instead of choosing randomly.

## Enroll Connector

On the VPS, create a short-lived connector enrollment token:

```bash
export CODENCER_MCP_TOKEN=<planner-token-with-connectors-enroll-scope>
./bin/codencer-relayd enrollment-token create \
  --relay-url https://relay.example.com \
  --token "$CODENCER_MCP_TOKEN" \
  --label local-macbook \
  --json
```

Copy only the enrollment token to the local machine, then use the `codencer` facade first:

```bash
export CODENCER_CONNECTOR_ENROLLMENT_TOKEN=<enrollment-token>
./bin/codencer connector enroll \
  --relay-url https://relay.example.com \
  --daemon-url http://127.0.0.1:8085 \
  --enrollment-token "$CODENCER_CONNECTOR_ENROLLMENT_TOKEN" \
  --config "$CODENCER_HOME/runtime/connector/config.json" \
  --label local-macbook \
  --json
```

Enrollment persists `codencer_home` into the connector config and records the connector config path in `$CODENCER_HOME/config.json` when possible.

Low-level fallback:

```bash
./bin/codencer-connectord enroll \
  --relay-url https://relay.example.com \
  --daemon-url http://127.0.0.1:8085 \
  --enrollment-token "$CODENCER_CONNECTOR_ENROLLMENT_TOKEN" \
  --config "$CODENCER_HOME/runtime/connector/config.json" \
  --label local-macbook \
  --json
```

Run and inspect the connector through the facade:

```bash
./bin/codencer connector status --config "$CODENCER_HOME/runtime/connector/config.json" --json
./bin/codencer connector config show --config "$CODENCER_HOME/runtime/connector/config.json" --json
./bin/codencer connector run --config "$CODENCER_HOME/runtime/connector/config.json"
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
