# Research Anchors

These anchors guide public self-host hardening decisions.

## Local-first Tools

Local-first developer tools must preserve local state ownership. Remote control
planes may index, route, and synchronize metadata, but local execution reports
and artifacts remain local unless explicitly published.

## MCP and Long-running Work

MCP tools should avoid depending on a single long blocking request for
long-running executor work. Submit/status/report/event patterns are safer for
real coding tasks, flaky networks, and human interrupts.

## Human Interrupts

Coding executors may need approval, clarifying information, permissions, OS
dialogs, credentials, or manual setup. These must become explicit lifecycle
states with resume/cancel paths and audit evidence.

## Redaction

Developer tools naturally handle local paths, repo roots, environment values,
tokens, raw logs, and private keys. Public control planes must assume those are
sensitive and redact them by default.

## Release Evidence

Release gates must verify the artifact and user path, not only source-tree unit
tests. Fake adapters are useful for deterministic plumbing but are not proof of
real executor value.
