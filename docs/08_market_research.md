> [!NOTE]
> This is a **design specification** and may not fully reflect the current implementation.
> For the latest implementation status, see the [Gap Audit](internal/GAP_AUDIT.md).

# Market Research

Prepared on 2026-03-24.

## Research question

Is there already something that solves:

- planning in one AI chat
- local execution in coding agent
- structured result return
- gated continuation
- vendor-neutral orchestration
- later IDE support

## What exists

### Native coding agents
Relevant:
- Codex
- Claude Code
- Qwen Code
- Cursor
- Cline

They solve **execution inside their own product loop**.
They do not fully solve an external planner-to-local-executor bridge with its own run ledger and policy engine.

### Background / remote agents
Some vendor tools support delegated async work.
That confirms demand, but those are usually:
- vendor-specific
- often cloud-centric
- not focused on planner-neutral local control

### Emerging orchestration projects
There are adjacent orchestration efforts:
- Composio Agent Orchestrator
- AWS Labs CLI Agent Orchestrator
- Maestro for Gemini CLI
- broader agent governance projects

This confirms:
**orchestration is becoming a category**.

## Whitespace

Still under-built:
A **local-first planner/executor/governance bridge** that:
- takes structured plan from external planner
- executes locally against coding agents
- normalizes results
- pauses at policy gates
- later integrates with IDE surfaces

## Strategic conclusion

Do not compete by building another coding agent.
Compete by building the **control plane around coding agents**.

## Risks

### Vendor overlap risk
Vendors may add more orchestration.
Mitigation:
- stay vendor-neutral
- support multiple providers
- own external planner bridge + local governance

### IDE automation brittleness
Mitigation:
- keep CLI path primary
- IDE chat support is optional and targeted

### Thin-wrapper risk
Mitigation:
- make state, policy, audit, and recovery first-class
