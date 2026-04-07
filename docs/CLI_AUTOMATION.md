# CLI Automation Patterns

## 🤖 AI Assistant / Shell & Tool Flows

Codencer v1 is designed to be operated by shell-capable AI assistants (like the one you are interacting with now) or automated shell planners.

### The Shell Planner Doctrine
1. **Shell Access is Required**: The planner must be able to run `orchestratorctl`.
2. **Bridge, Not Brain**: Codencer executes the task; the planner decides *which* task to run and *when* to retry or stop.
3. **Machine-Safe IO**: Always use the `--json` flag when calling `orchestratorctl` in scripts or tool calls to get parseable results.

### Example: Tool-Call Sequence for an AI Assistant

1. **Discovery**:
   ```bash
   ./bin/orchestratorctl instance --json
   ```
2. **Submission**:
   ```bash
   cat <<'EOF' | ./bin/orchestratorctl submit my-run --stdin --adapter codex --wait --json
   Refactor the data layer to use the new Repository pattern.
   EOF
   ```
3. **Decision**:
   The assistant parses the JSON output. If `state == "completed"`, proceed. If `state == "failed_validation"`, the assistant should read the validation logs and submit a follow-up task.

---

## 🔁 Sequential Execution (The Wrapper Pattern)

Codencer v1 is a local CLI bridge for terminal-capable planners and operators. The planner decides what to do next; Codencer executes one submitted task, records evidence, and reports the result. It does not include a native workflow engine, hidden planning layer, or autonomous task graph execution in v1.

## Official v1 Automation Model

The official v1 sequential model is an explicit wrapper loop outside Codencer:

1. **Target the Project**: Start or verify a daemon instance for a specific `--repo-root` (and unique `--port` if needed).
2. **Verify Identity**: Check `./bin/orchestratorctl instance --json` to ensure the bridge is anchored to the correct repo.
3. **Ensure a Run Exists**: Reuse an existing run or start a new one.
4. **Iterate Tasks**: Submit one task at a time with `submit --wait --json`.
5. **Inspect & Decide**: Use the exit code and terminal JSON payload to decide whether to continue or stop.
6. **Persistence**: All logs, artifacts, and validations are recorded as evidence for later human audit.

That model works for humans, shell planners, PowerShell, Python subprocess wrappers, and any other tool that can launch commands and read stdout/stderr.

## JSON and Exit Codes

For machine-facing automation, use `--json`:
- `stdout` contains exactly one JSON document
- `stderr` carries progress/help only
- `submit --wait --json` emits only the terminal payload

Stable wait-related exit codes:
- `0`: success
- `1`: usage error, invalid input, not found
- `2`: terminal task failure
- `3`: timeout
- `4`: cancelled, paused, rejected, intervention required
- `5`: bridge, adapter, daemon, or infrastructure failure

Wrappers should use both:
- the exit code for control flow
- the JSON payload for structured reporting

## Submit Inputs

`orchestratorctl submit` requires exactly one primary input source:
- positional task file
- `--task-json <path|->` (supports piping JSON strings)
- `--prompt-file <path>` (supports large text files)
- `--goal <text>` (supports quoted multiline strings)
- `--stdin` (supports multiline text via heredocs)

### Structured Hand-offs
For machine-based planners, `--task-json -` is the recommended way to submit fully-specified task bundles without writing temporary files:
```bash
echo "$TASK_JSON" | ./bin/orchestratorctl submit <runID> --task-json - --wait --json
```

### Antigravity Broker Execution
Planners must explicitly specify the `antigravity-broker` adapter to use the cross-side path:
- **Binding**: Is repository-scoped (Repo Root).
- **Execution**: Is run-scoped (Isolated Workspace/Worktree). 

The broker automatically receives the worktree as the `workspaceFolderAbsoluteUri` for the LS.

### Direct vs. Canonical Inputs
- **Canonical Sources** (`task-file`, `--task-json`): Strict JSON/YAML parsing. Conflict if local `run_id` does not match the submitted `run_id`.
- **Direct Sources** (`prompt-file`, `goal`, `stdin`): Deterministic normalization. Supports convenience metadata like `--adapter antigravity-broker`.

`context` and `acceptance` are preserved in the normalized task and provenance, but they are currently retained metadata rather than separate executor-driving runtime fields.

## Ordered Wrapper Examples

Codencer v1’s official sequential story lives in `examples/automation/`:
- `examples/automation/run_tasks.sh`
- `examples/automation/run_tasks.ps1`
- `examples/automation/run_tasks.py`

Sample task lists:
- `examples/automation/task_files.txt`
- `examples/automation/task_json_files.txt`
- `examples/automation/prompt_files.txt`
- `examples/automation/goals.txt`

### Bash / zsh
```bash
examples/automation/run_tasks.sh \
  --run-id run-automation-01 \
  --project codencer-demo \
  --input-mode goal \
  --tasks-file examples/automation/goals.txt \
  --adapter codex
```

Continue mode can be enabled explicitly:
```bash
examples/automation/run_tasks.sh \
  --run-id run-automation-01 \
  --project codencer-demo \
  --input-mode prompt-file \
  --tasks-file examples/automation/prompt_files.txt \
  --adapter codex \
  --continue-on-failure
```

The bash wrapper prefers `jq` and falls back to `python3` for JSON parsing.

### PowerShell
```powershell
./examples/automation/run_tasks.ps1 `
  -RunId run-automation-01 `
  -Project codencer-demo `
  -InputMode goal `
  -TasksFile examples/automation/goals.txt `
  -Adapter codex
```

PowerShell uses `ConvertFrom-Json`.

### Python
```bash
python3 examples/automation/run_tasks.py \
  --run-id run-automation-01 \
  --project codencer-demo \
  --input-mode task-file \
  --tasks-file examples/automation/task_files.txt
```

## Wrapper Contract

The official wrappers:
- require `--run-id`, `--input-mode`, and `--tasks-file`
- reuse an existing run when present
- create a missing run only when `--project` is provided
- process plain UTF-8 line lists, ignoring blank lines and `#` comments
- default to stop-on-failure
- support `CODENCER_CONTINUE_ON_FAILURE=1`
- emit progress to `stderr`
- emit one final JSON summary to `stdout`

Each summary includes:
- `run_id`
- `project`
- `input_mode`
- `continue_on_failure`
- `tasks_total`
- `tasks_succeeded`
- `tasks_failed`
- `results`
- `final_exit_code`

`results` entries include:
- `index`
- `source`
- `step_id`
- `state`
- `exit_code`

## Provenance and Auditability

Each accepted submission preserves:
- `original-input.*`
- `normalized-task.json`

These live under the attempt artifact root and make it possible to inspect both the exact submitted content and the normalized task Codencer actually executed.

Humans can inspect any machine-submitted step later with:
- `step result`
- `step logs`
- `step artifacts`
- `step validations`

## Mixed Human + Machine Workflows

A typical mixed workflow:

1. A script or planner submits tasks one by one with `submit --wait --json`.
2. The wrapper stops or continues based on the exit code policy outside Codencer.
3. A human later reviews the resulting step handles with `step result`, `step logs`, `step artifacts`, and `step validations`.

Codencer records the run, step, attempt, and artifact trail either way.

## Known Limitations

- Codencer v1 does not include a native workflow engine or manifest runner.
- Codencer does not perform hidden planning, branching, decomposition, or next-step selection.
- Codencer is local-first and repo-bound; it is not a hosted automation control plane.
- The PowerShell wrapper is for tools/operators that can reach a running daemon; it does not imply native Windows daemon support.
- Antigravity remains a separate execution path with its own broker/binding constraints.
