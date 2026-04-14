# Codencer Cloud Connector Matrix

This document describes the provider connectors currently implemented in the cloud alpha.

## Installation State Matrix

Cloud connector installations use the following state model:

| State | Meaning |
| --- | --- |
| `created` | The record exists and has not been disabled. It may still need validation or provider setup. |
| `active` | The installation is enabled and has passed its latest validation or polling pass. |
| `disabled` | The operator has explicitly disabled the installation. It is not meant to be processed or routed. |
| `error` | The last validation, webhook ingest, or worker poll failed. Review `last_error`. |

The enable/disable routes and CLI subcommands are:

- `POST /api/cloud/v1/installations/{id}/enable`
- `POST /api/cloud/v1/installations/{id}/disable`
- `codencer-cloudctl install enable`
- `codencer-cloudctl install disable`

## Provider Capability Matrix

| Provider | Validation | Webhook ingest | Polling ingest | Actions | Current status |
| --- | --- | --- | --- | --- | --- |
| GitHub | Yes, token validation against `/user` | Yes, signature-verified webhook events | No | Create issue comment | Implemented |
| GitLab | Yes, token validation against `/user` | Yes, token-verified webhook events | No | Create issue note | Implemented |
| Jira | Yes, basic-auth validation against `/myself` | No | Yes, via `codencer-cloudworkerd` | Add issue comment | Polling-first; webhook ingest intentionally not implemented |
| Linear | Yes, viewer query validation | Yes, signature-verified webhook events | No | Create issue | Implemented |
| Slack | Yes, `auth.test` validation | Yes, signature-verified event and interactive payloads | No | Post message | Implemented |

## Provider Notes

### GitHub

- normalizes issues, pull requests, and push events
- supports create-issue-comment actions
- status is derived from validation plus webhook verification

### GitLab

- normalizes issues, merge requests, and push events
- supports create-issue-note actions
- status is derived from validation plus webhook verification

### Jira

- the cloud worker polls Jira search using `config.jql` or `config.project_key`
- `config.username` and a provider token are required
- webhook verification returns an explicit not-implemented message in this alpha pass
- use `codencer-cloudworkerd --once` for a safe no-op smoke run or run it continuously for live polling

### Linear

- normalizes issue webhooks
- supports create-issue actions
- webhook ingest is implemented; polling is not

### Slack

- normalizes event callbacks, slash commands, and interactive payloads
- supports post-message actions
- webhook ingest is implemented; polling is not

## Practical Interpretation

- The cloud connector surface is useful for self-host operator use now.
- Jira is the only provider that is intentionally polling-first.
- Do not claim external provider coverage beyond the matrix above.
- If a provider installation is disabled, it should not be processed by the worker or treated as available for control-plane operations.

For bootstrap and smoke guidance, see [CLOUD_SELF_HOST.md](CLOUD_SELF_HOST.md). For the top-level cloud overview, see [CLOUD.md](CLOUD.md).
