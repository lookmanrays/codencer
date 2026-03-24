# DSL and MCP

## Why a DSL

Without a DSL, the system collapses into ad hoc prompt passing.

The DSL should make each step:
- declarative
- validated
- policy-aware
- provider-neutral

## TaskSpec example

```yaml
version: v1
project_id: local-agent-bridge
run_id: run-0001
phase_id: phase-04-codex-adapter
step_id: step-01
title: Implement Codex adapter invocation and artifact capture
goal: Build the first working Codex adapter that can execute a step, capture logs, and return a normalized result.
context:
  summary: >
    This is the first provider adapter. Keep it minimal but production-oriented.
constraints:
  - Do not introduce cloud functionality.
  - Do not bypass service boundaries.
allowed_paths:
  - internal/adapters/codex/**
  - internal/service/**
  - internal/domain/**
forbidden_paths:
  - internal/adapters/claude/**
  - internal/adapters/qwen/**
validations:
  - name: unit-tests
    command: go test ./...
  - name: lint
    command: golangci-lint run
acceptance:
  - Codex adapter implements common adapter interface.
  - Logs are captured to artifact storage.
  - Result is normalized into ResultSpec.
stop_conditions:
  - Adapter interface must be redesigned.
  - State machine must be rewritten.
policy_bundle: safe_refactor
adapter_profile: codex-default
```

## ResultSpec example

```json
{
  "version": "v1",
  "run_id": "run-0001",
  "phase_id": "phase-04-codex-adapter",
  "step_id": "step-01",
  "attempt_id": "attempt-01",
  "adapter": "codex",
  "status": "completed_with_warnings",
  "summary": "Implemented Codex adapter invocation and result normalization.",
  "files_changed": [
    "internal/adapters/codex/adapter.go",
    "internal/adapters/codex/invoke.go"
  ],
  "validations": [
    {"name": "unit-tests", "status": "passed"},
    {"name": "lint", "status": "failed"}
  ],
  "needs_human_decision": false,
  "warnings": ["Lint failed due to an unused import."],
  "questions": [],
  "artifacts": {
    "stdout_log": ".artifacts/.../stdout.log",
    "stderr_log": ".artifacts/.../stderr.log",
    "diff_patch": ".artifacts/.../diff.patch"
  }
}
```

## PolicySpec example

```yaml
version: v1
name: safe_refactor
continue_when:
  all_validations_pass: true
  max_changed_files: 12
  no_forbidden_paths_touched: true
  no_migrations_detected: true
gate_when:
  any_validation_fails: true
  dependency_files_changed: true
  migrations_detected: true
  changed_files_over: 12
  unresolved_questions_present: true
retry_when:
  adapter_process_failed: true
  timeout_once: true
fail_when:
  timeout_count_over: 2
  artifact_persistence_failed: true
```

## MCP tool surface

Expose only safe orchestrator primitives:

- `orchestrator.start_run`
- `orchestrator.start_step`
- `orchestrator.get_status`
- `orchestrator.get_result`
- `orchestrator.list_artifacts`
- `orchestrator.approve_gate`
- `orchestrator.reject_gate`
- `orchestrator.retry_step`
- `orchestrator.abort_run`
- `orchestrator.run_validations`

## MCP rules

- no raw shell tool exposure
- no raw DB mutation
- no unrestricted filesystem browsing
- input validation on every call
- stable machine-readable errors only
