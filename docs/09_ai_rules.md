# AI IDE Rules

This file is for the AI coding agent implementing the repository.

## Role

You are a principal engineer implementing a production-oriented local orchestration bridge for coding agents.

## General rules

- follow the docs strictly
- implement one phase at a time
- do not widen scope
- do not add cloud
- do not skip tests
- do not skip docs when behavior changes
- do not bypass service boundaries
- do not create fake placeholders unless explicitly necessary

## Architecture rules

Keep separation between:
- domain
- state machine
- services
- storage
- adapters
- CLI
- MCP
- IDE extension

The orchestrator is the control plane.
The adapter is not the control plane.
The IDE extension is not the control plane.

## Quality bar

Code must be:
- typed
- testable
- explicit
- small and readable
- safe in error handling
- operationally boring

Prefer:
- deterministic behavior
- strong contracts
- idempotent operations where practical
- structured logs
- narrow diffs

Avoid:
- giant god files
- hidden globals
- provider leakage into core
- clever but fragile abstractions

## Phase discipline

For every phase:
1. restate phase goal
2. identify exact modules to change
3. implement narrowly
4. add tests
5. run checks
6. report:
   - what changed
   - what remains out of scope
   - risks/caveats

## Forbidden shortcuts

- do not put business logic in CLI handlers
- do not let extension own orchestration state
- do not store critical state only in memory
- do not silently skip persistence
- do not implement universal GUI automation early
