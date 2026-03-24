# Security and Operations

## Security principles

### Least privilege
Planner side gets only safe MCP/orchestrator tools.

### Explicit project root
Every run is bound to a configured project root.

### Deterministic artifacts
All artifacts stored under controlled artifact root.

### Policy before power
Destructive/risky changes require gates.

## Gate triggers for MVP

- migration file created/changed
- dependency manifest or lockfile changed
- forbidden path touched
- too many files changed
- deletes over threshold
- validations fail
- adapter reports unresolved ambiguity

## Repo safety

- detect dirty repo before run
- optional worktree isolation
- run locks
- diff capture before cleanup
- safe cleanup path

## Process safety

- child process supervision
- timeout handling
- cancellation
- retry limits
- duplicate execution prevention
- crash recovery

## Artifact safety

Artifacts may contain code and logs.

Recommendations:
- local only by default
- retention policy later
- no automatic upload
- clear directory ownership

## Required test strategy

### Unit tests
- state transitions
- policy evaluation
- schema validation
- adapter normalization

### Integration tests
- SQLite storage
- artifact store
- daemon lifecycle
- CLI behavior

### E2E
- run start
- step execution
- validation
- gate creation
- approve/retry
- completion
