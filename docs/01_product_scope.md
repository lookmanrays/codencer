> [!NOTE]
> This is a **design specification** and may not fully reflect the current implementation.
> For the latest implementation status, see the [Gap Audit](internal/GAP_AUDIT.md).

# Product Scope

## Problem

Current workflow pain:
- architecture and prompt generation happen in one AI chat
- implementation happens in another AI IDE or coding agent
- the human manually copies prompts between systems
- the human manually waits for results
- the human manually decides whether to continue

That human becomes:
- transport layer
- output watcher
- result classifier
- gatekeeper

It accepts a structured implementation plan, dispatches each step to a selected coding agent, waits for completion, captures artifacts, normalizes output into a structured result, and evaluates whether to continue, retry, or stop for approval based on policy.

## Primary use case

- external AI planning chat produces phase plan and step spec
- local orchestrator receives next step
- local orchestrator executes via local coding agent
- local orchestrator returns normalized result
- planner or policy decides next step

## Product principles

### 1. Local-first
No cloud workers in MVP.

### 2. Deterministic control plane
Use LLMs for planning and summarization.
Use deterministic code for:
- state transitions
- retries
- policy evaluation
- persistence
- locking
- artifact management

### 3. CLI-first
First substrate should be terminal / CLI agents, not IDE chat panes.

### 4. Adapter-based
The orchestrator owns the contract.
Providers implement adapters.

### 5. Artifact-driven
Never trust “the model sounded done”.
Require structured result artifacts.

### 6. Human only at gates
Human should return only for:
- architecture decisions
- schema/migration changes
- risky dependency changes
- large destructive diffs
- unresolved ambiguity
- policy threshold breaches

## Non-goals in MVP

Do not build:
- cloud control plane
- remote workers
- team collaboration suite
- browser automation
- issue tracker orchestration
- generalized autonomous project management
- universal IDE chat automation

## Success criteria

### MVP success
User can:
1. start a run
2. start a step
3. execute the step through Codex locally
4. get structured result
5. continue without copy/paste into Codex chat

### V1 success
User can:
- switch between Codex / Claude Code / Qwen Code
- control and observe from VS Code
- optionally use one supported IDE chat adapter
