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
- **Cause**: The bridge cannot find the `codex-agent`, `claude-code`, or `aider` binary in your `$PATH`.
- **Fix**:
  - Export the specific path: `export CODEX_BINARY=/path/to/codex-agent`.
  - Or ensure the binary is in your global `$PATH`.

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

---

## 3. Interpreting Step States

Every task result includes a `state`. Understanding the difference between **Infrastructure** and **Goal** failure is key to recovery.

### 3.1 Goal & Task Failures (Fix the code/prompt)

- **`failed_terminal`**: The agent finished execution, but the summary reports that the goal was not met.
    - *Resolution*: Refine your task instructions or check if the agent is stuck in a loop.
- **`failed_validation`**: The agent finished and exited with 0, but your post-execution validations (tests, lint) failed.
    - *Resolution*: Review the `step validations` output and fix the code or the test.

### 3.2 Infrastructure & Bridge Failures (Fix the system)

- **`failed_adapter`**: The agent process crashed or exited with a non-zero code before it could report its outcome.
    - *Resolution*: Check `step logs` for agent-side crashes (e.g., API key errors, OOM).
- **`failed_bridge`**: Codencer encountered an internal error during worktree creation or **provisioning**.
    - *Resolution*: Check the `provisioning` telemetry in `step result` for copy/link errors.
- **`timeout`**: The execution exceeded `timeout_seconds` and was killed by the bridge.
    - *Resolution*: Increase the timeout in your TaskSpec or simplify the task.

---

## 4. Antigravity & Broker Issues (Experimental)

### 4.1 "Broker bind error: connection refused"
- **Cause**: The Windows-side `agent-broker.exe` is not running.
- **Fix**: Start the broker on the host machine. Verify port 8088 is open.

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

Codencer uses specific states to distinguish between **instructional failure** and **system failure**.

| State | Category | Meaning | Recovery |
| :--- | :--- | :--- | :--- |
| `completed` | **Success** | Goal met and all validations passed. | None. |
| `failed_terminal` | **Goal Failure** | Agent finished but reported it could NOT meet the goal. | Refine prompt/intent. |
| `failed_validation` | **Goal Failure** | Agent finished (0), but post-tests/lint failed. | Fix code logic or tests. |
| `failed_adapter` | **Infrastructure** | The agent binary crashed (e.g. API error, OOM). | Check `step logs`. |
| `failed_bridge` | **Infrastructure** | Codencer failed (e.g. Disk Full, Git Error, Provisioning). | Check daemon logs. |
| `timeout` | **Infrastructure** | Task exceeded `timeout_seconds` and was killed. | Increase timeout. |

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
