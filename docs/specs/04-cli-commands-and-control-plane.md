# CLI Commands and Control Plane

The CLI must behave like a normal local-first command-line tool. Gateway and
Relay are control-plane surfaces, not requirements for every local action.

## Required CLI Behavior

`codencer submit` must:

- default to local execution;
- show useful progress;
- show useful result/report output;
- avoid local path leaks by default;
- keep state, summary, and report consistent;
- return structured JSON with `--json`;
- expose clear errors and blockers.

## Required Commands

The public release must provide or document coherent equivalents for:

- `codencer config show`;
- `codencer config profiles list`;
- `codencer config profiles use <name>`;
- `codencer config set gateway.url <url>`;
- `codencer setup self-host`;
- `codencer setup relay`;
- `codencer activation self-host`;
- `codencer login` or equivalent self-host operator flow;
- `codencer project init`;
- `codencer project share`;
- `codencer project status`;
- `codencer executor list`;
- `codencer executor scan`;
- `codencer executor test <executor>`;
- `codencer executor default <executor>`;
- `codencer submit`;
- `codencer run status`;
- `codencer run events`;
- `codencer run report`;
- `codencer run cancel`;
- `codencer run resume`;
- `codencer sync` or explicit publish equivalent if sync is implemented.

## Endpoint Defaults

Public/self-built defaults must be local/self-host. Public builds must not
silently talk to Codencer commercial cloud.

Configuration precedence:

```text
CLI flags > env vars > user config profile > build-time defaults > self-host defaults
```

Official/private builds may override build-time defaults later, but public
self-host behavior must remain explicit and safe.
