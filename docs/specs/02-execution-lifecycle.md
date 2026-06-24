# Execution Lifecycle

Codencer uses an async-first execution lifecycle. Long-running tasks must not
depend on one long blocking HTTP or MCP request.

## Required Lifecycle

The public release must support these logical operations:

- submit;
- status;
- events;
- report;
- cancel;
- resume.

Blocking convenience wrappers may exist, but they must be implemented on top of
the async lifecycle and must not be the only reliable path.

## Run States

Runs and steps must expose stable state transitions:

- submitted;
- queued or accepted;
- route_resolved;
- relay_selected;
- connector_selected;
- executor_selected;
- running;
- waiting_for_human;
- completed;
- failed;
- blocked;
- canceled.

## Reports

`get_run_report` must work for run IDs returned by:

- simple task submit;
- manifest/run-plan submit;
- Gateway MCP submit;
- Gateway Console submit.

Reports must include safe, useful result text. If report summary is empty,
display must fall back to safe details, task/evidence summary, sanitized raw
output excerpts, validation summaries, artifact names, or blocker messages.

## Events

Run events must be queryable without opening a long blocking request. Event
surfaces may be polling, event-log APIs, or streaming where available.

Events must be safe for UI/MCP output by default.

## Cancellation and Resume

Cancel and resume must be explicit operations. If an executor cannot support
true cancellation or resume, Codencer must return a structured blocker or
capability response rather than pretending the operation succeeded.
