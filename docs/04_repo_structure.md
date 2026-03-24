# Recommended Repository Structure

```text
agent-bridge/
  README.md
  AGENTS.md
  Makefile
  go.mod
  go.sum

  docs/
    01_product_scope.md
    02_architecture.md
    03_roadmap.md
    04_repo_structure.md
    05_dsl_and_mcp.md
    06_adapters_and_ide.md
    07_security_ops.md
    08_market_research.md
    09_ai_rules.md
    10_implementation_prompts.md
    11_review_checklists.md
    references.md

  cmd/
    orchestratord/
      main.go
    orchestratorctl/
      main.go
    orchestrator-mcp/
      main.go

  internal/
    app/
      bootstrap.go
      config.go
      version.go

    domain/
      run.go
      phase.go
      step.go
      attempt.go
      artifact.go
      gate.go
      validation.go
      policy.go
      adapter.go

    state/
      machine.go
      transitions.go

    service/
      run_service.go
      step_service.go
      gate_service.go
      validation_service.go
      policy_service.go
      adapter_service.go
      workspace_service.go

    storage/
      sqlite/
        migrations/
        runs_repo.go
        steps_repo.go
        attempts_repo.go
        artifacts_repo.go
        gates_repo.go
      files/
        artifact_store.go

    adapters/
      codex/
        adapter.go
        invoke.go
        normalize.go
      claude/
        adapter.go
      qwen/
        adapter.go
      ide/
        adapter.go

    validation/
      runner.go
      commands.go
      parsers.go

    workspace/
      repo.go
      worktree.go
      locks.go

    mcp/
      server.go
      tools.go
      mapping.go

    util/
      errors.go
      fs.go
      json.go
      proc.go
      time.go

  schemas/
    task.schema.json
    result.schema.json
    policy.schema.json

  examples/
    task-basic.yaml
    task-refactor.yaml
    policy-safe-refactor.yaml

  testdata/
    sample-repo/
    fixtures/

  scripts/
    dev.sh
    test.sh
    lint.sh
    e2e.sh

  .artifacts/
    .gitkeep
```

## Notes

### cmd/
Thin entrypoints only.

### domain/
Stable business concepts only.

### service/
All orchestration behavior and lifecycle decisions.

### adapters/
Provider-specific logic only.

### storage/
Persistence only.

### mcp/
Thin bridge over services.

### .artifacts/
Deterministic local execution evidence.
