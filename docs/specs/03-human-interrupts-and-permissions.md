# Human Interrupts and Permissions

Codencer must represent human intervention as first-class lifecycle state, not
as opaque executor failure.

## Required Interrupt Types

The public release must model:

- planning approval required;
- clarifying question required;
- permission request required;
- OS/system human action required;
- executor-specific human decision required;
- resume;
- cancel.

## Required Data

Human interrupt records must include:

- run ID;
- step/task ID when available;
- project ID;
- executor profile;
- interrupt type;
- status;
- safe prompt/summary;
- requested action;
- allowed responses where applicable;
- creation/update timestamps.

They must not include local paths, raw secrets, private keys, raw environment
values, or full unredacted executor transcripts by default.

## Behavior

- When a human interrupt occurs, run state becomes `waiting_for_human` or a
  structured blocker state.
- CLI/MCP/UI must display the interrupt clearly.
- Resume must require an explicit operator action.
- Cancel must be available when the run is waiting for human action.
- Audit must record interrupt creation, operator response, resume, and cancel.

## Permission Safety

Dangerous executor profiles or elevated permission requests must require
explicit confirmation. A missing confirmation must return a structured blocker,
not silently downgrade or proceed.
