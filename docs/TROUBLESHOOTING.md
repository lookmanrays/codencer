# Troubleshooting Reference

This guide helps you resolve common issues encountered when running the Codencer Orchestration Bridge locally.

## 1. Quick Diagnosis

Always start here to verify your environment:
```bash
# Checks if .codencer/ is setup and if agent binaries are accessible
./bin/orchestratorctl doctor
```

---

## 2. Common Infrastructure Failures

### 2.1 "Connection Refused" (CLI cannot reach Daemon)
**Symptoms**: `./bin/orchestratorctl` returns `error connecting to orchestratord`.
- **Cause**: The `orchestratord` process is not running or is on a different port.
- **Fix**: 
  - Ensure the daemon is started: `make start` or `make start-sim`.
  - Check the port in your CLI command or `.env` file (default is `8085`).

### 2.2 "Agent Binary Not Found" (`failed_adapter`)
**Symptoms**: Submitting a task fails immediately; `doctor` shows `MISSING` for an adapter.
- **Cause**: The bridge cannot find the `codex-agent` or `claude` binary in your `$PATH`.
- **Fix**:
  - Export the specific path: `export CODEX_BINARY=/path/to/codex-agent`.
  - Or export `CLAUDE_BINARY=/path/to/claude` if the Claude CLI is installed outside your default `$PATH`.
  - Or ensure the binary is in your global `$PATH`.

### 2.2.1 "Malformed or Missing Claude Result Output" (`failed_terminal`)
**Symptoms**: Claude starts, but the final result summary mentions malformed or missing Claude output.
- **Cause**: The `claude` process did not emit a final JSON `result` object on stdout, or another tool/script polluted stdout.
- **Fix**:
  - Inspect `./bin/orchestratorctl step logs <UUID>` for the raw stdout payload.
  - Inspect `./bin/orchestratorctl step artifacts <UUID>` and review `stderr.log` for CLI/auth/runtime errors.
  - Re-run `claude -p --output-format json` directly in the repo if you suspect local CLI/environment issues.

### 2.3 "Workspace Creation Failed" (`failed_bridge`)
**Symptoms**: `failed_bridge` reported during attempt.
- **Cause**: Git worktree conflict or permission issue.
- **Fix**: Run `git worktree prune` and ensure the `~/.codencer/workspaces` directory is writable.

### 2.4 "submit requires exactly one primary input source"
**Symptoms**: `submit` exits with a usage/input error before contacting the daemon.
- **Cause**: No primary source was supplied, or multiple primary sources were supplied together.
- **Fix**: Choose exactly one of:
  - positional task file
  - `--task-json`
  - `--prompt-file`
  - `--goal`
  - `--stdin`

### 2.5 "direct metadata flags are only supported..."
**Symptoms**: `submit` rejects `--title`, `--context`, `--adapter`, `--timeout`, `--policy`, `--acceptance`, or `--validation`.
- **Cause**: Those flags were combined with a canonical task source such as a positional task file or `--task-json`.
- **Fix**: Put those fields in the YAML/JSON task itself, or switch to a direct source (`--prompt-file`, `--goal`, or `--stdin`).

### 2.6 "direct input is empty"
**Symptoms**: `submit --stdin` or another direct source exits with a usage/input error.
- **Cause**: The prompt text was empty or whitespace-only.
- **Fix**: Ensure the prompt file or stdin stream contains real task text before submitting.

### 2.7 "failed to parse task spec json"
**Symptoms**: `submit --task-json ...` fails before the request is sent.
- **Cause**: `--task-json` is strict JSON mode and the payload is invalid JSON.
- **Fix**: Validate the payload as JSON, or submit the file positionally if you want the YAML/JSON-compatible task-file parser.

### 2.8 "task run_id ... does not match submit run ID ..."
**Symptoms**: Canonical task submission fails locally.
- **Cause**: The authored task payload includes a `run_id` that conflicts with the CLI `<RUN_ID>`.
- **Fix**: Either remove `run_id` and let the CLI fill it, or make the authored `run_id` match the run you are submitting to.

### 2.9 Wrapper exits because the run is missing
**Symptoms**: An official wrapper example exits before the first task runs.
- **Cause**: The wrapper checks `run state --json` first and only creates the run automatically when `--project` is provided.
- **Fix**: Either start the run yourself first, or pass `--project <project>` to the wrapper.

### 2.10 Bash wrapper says it needs `jq` or `python3`
**Symptoms**: `examples/automation/run_tasks.sh` exits immediately.
- **Cause**: The bash wrapper needs a JSON parser for machine-safe result handling.
- **Fix**: Install `jq`, or ensure `python3` is available in `$PATH`.

### 2.11 Wrapper stops after the first failure
**Symptoms**: A sequential wrapper exits on a non-zero task result.
- **Cause**: Stop-on-failure is the official default for v1.
- **Fix**: Re-run with the wrapper’s explicit continue mode, or set `CODENCER_CONTINUE_ON_FAILURE=1` for automation.

### 2.12 Wrapper continues when you expected it to stop
**Symptoms**: The wrapper keeps iterating after a failed task.
- **Cause**: `CODENCER_CONTINUE_ON_FAILURE=1` is set in the environment or the explicit continue flag was used.
- **Fix**: Unset the environment variable or omit the continue flag.

### 2.13 "planner bearer token required" from `codencer-relayd`
**Symptoms**: `codencer-relayd status`, `instances`, `connectors`, or `enrollment-token create` fails locally.
- **Cause**: No planner token was supplied and the selected config file does not contain `planner_token` or `planner_tokens`.
- **Fix**:
  - Generate one locally: `./bin/codencer-relayd planner-token create --config .codencer/relay/config.json --write-config --name operator --scope '*'`
  - Or pass `--token <planner-token>` explicitly.

### 2.14 Connector is online but the instance never appears on the relay
**Symptoms**: `codencer-connectord status` shows activity, but `/api/v2/instances` stays empty.
- **Cause**: The instance is known locally but not marked `share=true`, or the connector is pointed at the wrong daemon URL.
- **Fix**:
  - Inspect local config: `./bin/codencer-connectord list`
  - Share the intended daemon explicitly: `./bin/codencer-connectord share --daemon-url http://127.0.0.1:8085`
  - Re-check local status: `./bin/codencer-connectord status --json`
  - Re-check relay view: `./bin/codencer-relayd instances --config .codencer/relay/config.json`

### 2.15 Relay returns `connector_disabled`
**Symptoms**: Step, gate, artifact, or result routes start failing with a `403` and `connector_disabled`.
- **Cause**: The relay-side connector record was explicitly disabled.
- **Fix**:
  - Inspect connector state: `./bin/codencer-relayd connectors --config .codencer/relay/config.json`
  - Re-enable it: `./bin/codencer-relayd connector enable <connector-id> --config .codencer/relay/config.json`

### 2.16 MCP returns `origin_denied`, `session_not_found`, or `protocol_version_mismatch`
**Symptoms**: Browser-style or session-based MCP callers fail on `/mcp`.
- **Cause**:
  - `origin_denied`: the request Origin is not allowed by relay config
  - `session_not_found`: the caller reused an expired or deleted `MCP-Session-Id`
  - `protocol_version_mismatch`: the caller changed `MCP-Protocol-Version` after initialization
- **Fix**:
  - Add the caller origin to `allowed_origins`, or omit the Origin header for non-browser clients
  - Re-run `initialize` to get a fresh `MCP-Session-Id`
  - Keep the same negotiated `MCP-Protocol-Version` for the session

### 2.17 Relay evidence routes work for result but not logs/artifacts/validations
**Symptoms**: `/api/v2/steps/{id}/result` works, but logs or evidence routes fail.
- **Cause**: The step route may exist but the connector was enrolled against a different daemon, the instance is unshared, or the step lives on another shared instance that is currently offline.
- **Fix**:
  - Verify the instance is still advertised and online through `codencer-relayd instances`
  - Check the connector config and `share=true` state with `codencer-connectord list`
  - Re-run the connector and inspect `status.json`

---

## 3. Interpreting Step States

Every task result includes a `state`. Understanding the difference between **Infrastructure** and **Goal** failure is key to recovery.

### 3.1 Goal & Task Failures (Fix the code/prompt)

- **`failed_terminal`**: The agent finished execution, but the summary reports that the goal was not met.
    - *Resolution*: Refine your task instructions or check if the agent is stuck in a loop.
- **`failed_validation`**: The agent finished and exited with 0, but your post-execution validations (tests, lint) failed.
    - *Resolution*: Review the `step validations` output and fix the code or the test.

### 3.2 Infrastructure & Bridge Failures (Fix the system)

---

## 4. Antigravity & Broker Issues (Experimental)

### 4.1 "Broker bind error: connection refused"
- **Cause**: The Windows-side broker is not running.
- **Fix**: Run `make build-broker`, start the resulting broker binary on the host machine, and verify port 8088 is open.

### 4.2 "No instances discovered"
- **Cause**: Antigravity is not active in your IDE or the `.gemini` daemon directory is hidden/unreachable.
- **Fix**: Open a project in VS Code with Antigravity enabled. Ensure your WSL mount for `C:` is working correctly.

### 4.3 "Stale Binding"
- **Cause**: The IDE instance was closed or restarted.
- **Fix**: Run `orchestratorctl antigravity list` and re-bind to the new PID.


### 4.4 "Resource Busy" or "File Locked" during workspace creation
- **Cause**: A previous git operation didn't clean up correctly, or multiple tasks are fighting for the same repo lock.
- **Fix**: Run `git worktree prune`. If the issues persist, check if other instances are targeting the same `repo_root`.

---

## 5. Interpreting Step States (Authoritative Evidence)

Codencer uses specific states to distinguish between **instructional failure** (Goal) and **system failure** (Infrastructure).

| State | Category | Meaning | Recovery |
| :--- | :--- | :--- | :--- |
| `completed` | **Success** | Goal met and all validations passed. | None. |
| `failed_terminal` | **Goal Failure** | Agent finished but reported it could NOT meet the goal. | Refine prompt/intent. |
| `failed_validation` | **Goal Failure** | Agent finished (0), but post-tests/lint failed. | Fix code logic or tests. |
| `failed_adapter` | **Infrastructure** | The agent binary crashed (e.g. API error, OOM). | Check `step logs`. |
| `failed_bridge` | **Infrastructure** | Codencer failed (e.g. Disk Full, Git Error, Provisioning). | Check daemon logs. |
| `timeout` | **Infrastructure** | Task exceeded `timeout_seconds` and was killed. | Increase timeout. |
| `cancelled` | **Infrastructure / Operator Action** | Task was explicitly interrupted before completion. | Re-run or submit a follow-up task. |

---

## 6. Resetting the Bridge

If the local state becomes corrupted or you want a fresh start:
```bash
# DANGER: This deletes the database and all artifact history
make nuke

# Rebuild and start fresh
make build
make start-sim
```
