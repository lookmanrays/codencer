# Run History, Audit, and Console

Gateway Console must provide a user-facing result/history surface. Audit is
secondary evidence, not the primary result view.

## Run History

Run history must include:

- compact list view;
- detail view;
- status;
- title/goal;
- executor profile;
- project;
- machine/connector;
- started/completed time;
- result summary;
- result details;
- safe artifact/log references;
- execution mode (`Real executor`, `Simulation`, or `Unknown`);
- scope (`local`, `synced`, or `Gateway-submitted`) where available.

Run detail URLs should use stable Gateway history IDs when in Gateway context.
Raw executor run IDs must still be displayed.

## Audit

Audit must include grouped lifecycle events:

- `task_submitted`;
- `route_resolved`;
- `relay_selected`;
- `connector_selected`;
- `executor_selected`;
- `run_started`;
- `run_completed`, `run_failed`, or `blocker`;
- `report_read`;
- human interrupt events;
- sync/publish events.

Audit must support pagination and filters for public release scale. Audit
events must link to run detail when run metadata is available.

## Console

Gateway Console must:

- run in live mode against Gateway;
- use explicit demo mode only;
- never silently fall back to demo data;
- show clear empty/unavailable states;
- show selected project, relay, connector/machine, executor, execution type,
  timeout, status, and result;
- keep Manifest / run-plan mode advanced unless fully guided and validated;
- hide developer-only UI from product navigation;
- avoid local path/secret leaks in rendered HTML.
