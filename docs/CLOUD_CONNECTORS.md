# Codencer Cloud Connector Matrix

This document describes the priority provider connectors currently implemented in the cloud alpha. The matrix below separates what exists in code from what is verified in repo tests and what remains intentionally partial.

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

Installations now also persist:

- `owner_membership_id`
- `health`
- `last_seen_at`
- `last_sync_at`
- `last_validated_at`
- `last_webhook_at`
- `last_action_at`
- `last_error`

Practical interpretation:

- `health=healthy` means the latest validate, webhook, action, or worker poll succeeded
- `health=degraded` means the latest provider-facing operation failed
- `health=disabled` means the operator explicitly disabled the installation
- `health=unknown` is the initial create-time value before any provider check succeeds

## Provider Capability Matrix

| Provider | Validation | Webhook ingest | Polling ingest | Actions implemented | Verified in repo tests | Current status |
| --- | --- | --- | --- | --- | --- | --- |
| GitHub | Yes, token validation against `/user` with user metadata | Yes, signature-verified webhook events | No | `create_issue_comment`, `create_issue` | validation, issue/PR/push normalization, comment create, issue create | Useful alpha connector; still narrow |
| GitLab | Yes, token validation against `/user` with user metadata | Yes, token-verified webhook events | No | `create_issue_note`, `create_issue` | validation, issue/MR/push normalization, note create, issue create | Useful alpha connector; still narrow |
| Jira | Yes, basic-auth validation against `/myself` with polling-mode status details | No | Yes, via `codencer-cloudworkerd` | `add_issue_comment`, `transition_issue` | validation, comment create, transition action, polling snapshot normalization, worker sync | Polling-first by design; webhook ingest intentionally not implemented |
| Linear | Yes, viewer query validation with stronger identity checks | Yes, signature-verified webhook events | No | `create_issue`, `add_comment` | validation, issue create, comment create, webhook normalization | Useful alpha connector; still narrow |
| Slack | Yes, `auth.test` validation with stricter identity checks | Yes, signature-verified event, interactive, and slash-command payloads | No | `post_message`, `update_message` | validation, post/update message, event callback, interactive, slash-command normalization | Useful alpha connector for notifications and approvals |

## Provider Notes

### GitHub

- normalizes issues, pull requests, and push events
- supports `create_issue_comment` and `create_issue`
- tests cover issue, pull request, and push normalization
- install validation requires token and a valid API base URL; webhook secret is optional for action-only installs
- status is derived from validation plus webhook verification, with health/timestamp fields persisted on installation records
- not implemented: PR review actions, labels, state transitions, installation/OAuth app flow

### GitLab

- normalizes issues, merge requests, and push events
- supports `create_issue_note` and `create_issue`
- tests cover issue, merge request, and push normalization
- install validation requires token and a valid API base URL; webhook secret is optional for action-only installs
- status is derived from validation plus webhook verification, with health/timestamp fields persisted on installation records
- not implemented: merge request notes, labels, state transitions, app-install flow

### Jira

- the cloud worker polls Jira search using `config.jql` or `config.project_key`
- `config.username` and a provider token are required
- install validation now also requires one of `config.jql` or `config.project_key` when the installation is intended for polling
- webhook verification returns an explicit not-implemented message in this alpha pass
- supports `add_issue_comment` and `transition_issue`
- status explicitly reports polling-first behavior
- worker sync updates `last_sync_at`, `last_seen_at`, `health`, and `last_error`
- use `codencer-cloudworkerd --once` for a safe no-op smoke run or run it continuously for live polling
- not implemented: webhook ingest, transition discovery, rich sync cursors, live provider-account proof in this repo

### Linear

- normalizes issue webhooks
- supports `create_issue` and `add_comment`
- webhook ingest is implemented; polling is not
- install validation requires token and a valid API base URL; webhook secret is optional for action-only installs
- status is derived from validation plus webhook verification, with health/timestamp fields persisted on installation records
- not implemented: state transitions, richer project/team discovery, live provider-account proof in this repo

### Slack

- normalizes event callbacks, slash commands, and interactive payloads
- supports `post_message` and `update_message`
- webhook ingest is implemented; polling is not
- install validation requires token and a valid API base URL; webhook secret is optional for action-only installs
- status is derived from validation plus webhook verification, with health/timestamp fields persisted on installation records
- not implemented: reactions, view submissions, richer message update/event coverage, live workspace proof in this repo

## Practical Interpretation

- The cloud connector surface is useful for self-host operator use now.
- Jira is the only provider that is intentionally polling-first.
- Do not claim external provider coverage beyond the matrix above.
- If a provider installation is disabled, it should not be processed by the worker or treated as available for control-plane operations.
- Installation ownership is now attributable to a membership when the create request is made with a membership-linked token.
- These are still alpha connectors, not full vendor-depth integrations. The strongest proof in this repo is unit/integration-style HTTP coverage against mocked provider APIs.

For bootstrap and smoke guidance, see [CLOUD_SELF_HOST.md](CLOUD_SELF_HOST.md). For the top-level cloud overview, see [CLOUD.md](CLOUD.md).
