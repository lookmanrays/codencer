# CLI Automation Patterns

Codencer v1 is a local CLI bridge for terminal-capable planners and operators. The planner decides what to do next; Codencer executes one submitted task, records evidence, and reports the result. It does not include a native workflow engine, hidden planning layer, or autonomous task graph execution in v1.

## Official v1 Automation Model

The official v1 sequential model is an explicit wrapper loop outside Codencer:

1. Ensure a run exists.
2. Iterate an ordered list outside Codencer.
3. Submit one task at a time with `submit --wait --json`.
4. Inspect the exit code and terminal JSON payload.
5. Decide whether to continue or stop outside Codencer.
6. Inspect logs, artifacts, and validations later as needed.

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
- `--task-json <path|->`
- `--prompt-file <path>`
- `--goal <text>`
- `--stdin`

Full canonical task definitions:
- positional task files accept YAML or JSON
- `--task-json` is strict JSON
- `run_id` is auto-filled from the CLI run ID when omitted
- conflicting authored `run_id` is rejected locally

Direct convenience input:
- is a deterministic normalization layer over the same canonical `TaskSpec`
- does not plan, decompose work, merge sources, or invent strategy
- supports a narrow metadata set: `--title`, `--context`, `--adapter`, `--timeout`, `--policy`, repeated `--acceptance`, repeated `--validation`

Direct defaults:
- `version` defaults to `v1`
- `run_id` comes from the CLI run ID
- `title` comes from `--title`, otherwise prompt filename basename, otherwise `Direct task`
- `goal` is the exact submitted text
- repeated `--validation` flags become `validation-1`, `validation-2`, and so on

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
