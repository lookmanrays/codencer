# Executor Profiles

Codencer is not the agent. Codencer routes approved tasks or manifests to the
selected executor profile and records the resulting state, evidence, report, and
audit events.

The current public repository exposes executor profiles as a thin user-facing
layer over existing adapter/profile configuration:

- Project default executor: stored in committed `.codencer/project.json` under
  `execution.default_adapter` and `execution.default_profile`.
- Local registry executor: adopted into `$CODENCER_HOME/projects.json` for the
  local machine and connector.
- Task override: `profile` / `adapter_profile` on CLI, Gateway API, and MCP
  calls can override the project default for a single run.

Built-in profiles include:

- `fake-success`, `fake-failure`, `fake-blocker`, `fake-timeout` for
  deterministic plumbing and CI smoke tests.
- `codex-workspace`, `codex-full`, `codex-danger-bypass` for Codex CLI.
- `claude-default` for Claude CLI.

Use the CLI to inspect and select profiles:

```bash
codencer executor list --json
codencer executor scan --json
codencer executor test codex-workspace --json
codencer executor default codex-workspace --repo . --json
```

`executor default` updates `.codencer/project.json` and updates the local
registry if the project is already adopted. It does not require reinitializing
the project per agent.

For one run only:

```bash
codencer submit --project codencer --profile codex-workspace --goal "Run the approved task" --wait --json
```

Gateway MCP tools accept the same override:

```json
{
  "project_id": "codencer",
  "relay_profile_id": "default",
  "machine_id": "mach_...",
  "profile": "codex-workspace",
  "goal": "Run the approved task"
}
```

The Gateway Console shows the selected Relay, connector/machine, project
default executor, and optional task-level override before submitting a run.

## Real Executor RC Gate

The public self-host RC verifier always runs deterministic fake executor
plumbing. A `GO` verdict additionally requires a configured real executor gate.

Codex example:

```bash
CODENCER_E2E_REAL_EXECUTOR=codex \
CODENCER_E2E_REAL_EXECUTOR_COMMAND=codex \
make verify-public-selfhost-rc
```

If no real executor is configured, the verifier reports `PARTIAL`, not `GO`.
That is expected for CI smoke and is not live product proof.
