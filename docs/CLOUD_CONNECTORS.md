# Codencer Cloud Connector Matrix

> [!WARNING]
> This is an experimental cloud-control-plane/provider matrix. It is not the current v0.3 OSS self-host Relay path and does not claim hosted Codencer Gateway/Cloud availability.

This document freezes the current provider connector contract to repo-tested truth.

Status labels in the matrix below mean:

- `proven`: directly exercised by current repo tests or smoke.
- `partial`: implemented and usable within a narrow scope, but proof or operator packaging is still thin.
- `expected-only`: code/docs suggest it should work, but the repo does not directly prove it today.
- `deferred`: intentionally outside the current provider beta promise.

## Generic Install And Validate Contract

All five providers use the same cloud installation surface:

- `POST /api/cloud/v1/installations`
- `GET /api/cloud/v1/installations/{id}`
- `POST /api/cloud/v1/installations/{id}/validate`
- `POST /api/cloud/v1/installations/{id}/enable`
- `POST /api/cloud/v1/installations/{id}/disable`
- `POST /api/cloud/v1/installations/{id}/actions`
- `POST /api/cloud/v1/installations/{id}/webhook`
- `GET /api/cloud/v1/events`
- `GET /api/cloud/v1/audit`

Create requests use this shape:

```json
{
  "org_id": "org_...",
  "workspace_id": "ws_...",
  "project_id": "proj_...",
  "connector_key": "slack",
  "external_installation_id": "",
  "external_account": "",
  "name": "Slack smoke",
  "config": {
    "api_base_url": "https://slack.example",
    "username": "jira@example.com",
    "project_key": "PROJ"
  },
  "secrets": {
    "token": "...",
    "webhook_secret": "..."
  }
}
```

Generic runtime truth:

- `config` is stored as installation config JSON.
- `secrets` are stored separately and encrypted.
- current provider code only consumes generic `config.api_base_url`, `config.username`, and the raw `config` map, plus `secrets.token` and `secrets.webhook_secret`
- create does not live-check provider credentials; live validation happens later through `POST /validate`
- `POST /validate` takes no body and returns:

```json
{
  "validation": {},
  "status": {},
  "error": ""
}
```

- install records start as `status=created`, `enabled=true`, `health=unknown` through the HTTP API
- `GET /installations/{id}` exposes the persisted installation state
- `POST /validate` returns a richer provider-derived status result and updates `last_validated_at`, `health`, and `last_error`

## History, Action Logs, And Audit Truth

- connector event history is append-only in the cloud store
- repeated `source_event_id` values are preserved instead of overwriting earlier rows
- event listing is newest-first
- connector action logs now persist request JSON, response JSON, error text, started time, and completed time
- cloud audit rows for connector actions now include provider, action, status, external ID, URL, status code, and error summary when present
- webhook verification failures, webhook deferment, and normalization failures now create explicit audit rows instead of only leaving implicit HTTP responses

Current limitation:

- there is still no public API route for listing action-log rows directly; action-log depth is mainly a store/audit truth improvement in this phase

## Provider Capability Matrix

| Provider | Install/bootstrap | Validation | Ingest | Actions | Status/health | Audit | Local testability | Publishability-preparedness |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| GitHub | `partial` | `proven` | `partial` | `proven` | `proven` | `partial` | `partial` | `partial` |
| GitLab | `partial` | `proven` | `partial` | `proven` | `proven` | `partial` | `partial` | `partial` |
| Jira | `partial` | `proven` | `partial` | `partial` | `proven` | `partial` | `partial` | `partial` |
| Linear | `partial` | `proven` | `partial` | `partial` | `partial` | `partial` | `partial` | `partial` |
| Slack | `proven` | `proven` | `partial` | `partial` | `proven` | `partial` | `proven` | `partial` |

Practical interpretation:

- Slack is the strongest operator-facing connector path in the repo today.
- Jira is real and supported within a polling-first scope.
- GitHub, GitLab, and Linear are real connectors with direct mocked-provider proof, but their operator packaging and local end-to-end proof remain thinner than Slack.
- none of these providers should be described as marketplace-ready, OAuth-install ready, or vendor-depth complete in this phase

## Provider Details

### GitHub

Current scope:

- validate token against `GET /user`
- verify webhook signatures and normalize issue, pull request, and push events
- execute `create_issue_comment`
- execute `create_issue`

Required config and secrets:

- secret `token`
- optional `config.api_base_url`
- secret `webhook_secret` only when webhook ingest is desired

Local testing path:

- `go test ./internal/cloud/connectors -count=1`
- generic routed proof lives in `go test ./internal/cloud -count=1`
- no GitHub-specific binary smoke exists in the repo today

Publishability-prepared means:

- operators can see the exact config and endpoint requirements
- a token must authorize `GET /user`, `POST /repos/{owner}/{repo}/issues`, and `POST /repos/{owner}/{repo}/issues/{n}/comments`
- webhook callers need a configured webhook secret and a compatible delivery into `/api/cloud/v1/installations/{id}/webhook`

Still missing before broader publication/distribution:

- provider-native app-install or OAuth flow
- richer issue/PR actions
- live vendor-account proof in repo automation

### GitLab

Current scope:

- validate token against `GET /user`
- verify webhook token and normalize issue, merge request, and push events
- execute `create_issue_note`
- execute `create_issue`

Required config and secrets:

- secret `token`
- optional `config.api_base_url`
- secret `webhook_secret` only when webhook ingest is desired

Local testing path:

- `go test ./internal/cloud/connectors -count=1`
- generic routed proof lives in `go test ./internal/cloud -count=1`
- no GitLab-specific binary smoke exists in the repo today

Publishability-prepared means:

- operators can see the exact config and endpoint requirements
- a token must authorize `GET /user`, issue create, and issue-note endpoints against the configured GitLab API base
- webhook callers need a configured webhook secret and a compatible delivery into `/api/cloud/v1/installations/{id}/webhook`

Still missing before broader publication/distribution:

- provider-native app-install or OAuth flow
- merge-request write surface beyond the current narrow event normalization
- live vendor-account proof in repo automation

### Jira

Current scope:

- validate credentials against `GET /rest/api/3/myself`
- polling-first ingest through `codencer-cloudworkerd`
- execute `add_issue_comment`
- execute `transition_issue`
- derive installation status as polling-first with `webhook_ingest=disabled`

Required config and secrets:

- `config.api_base_url`
- `config.username`
- one of `config.jql` or `config.project_key`
- secret `token`

Important truth:

- Jira webhook ingest is deferred in this phase
- routed Jira webhook calls now return `501` with `webhook_deferred`
- Jira webhook requests do not persist events and do not emit false-positive success audit rows
- the supported ingest path is `codencer-cloudworkerd`, not `/webhook`

Local testing path:

- `go test ./internal/cloud/connectors -count=1`
- `go test ./internal/cloud -run 'TestJiraWebhookRouteReturnsDeferredWithoutPersistingEvents|TestWorkerRunOncePollsJiraAndPersistsSnapshot' -count=1`
- `./bin/codencer-cloudworkerd --config .codencer/cloud/config.json --once`

Publishability-prepared means:

- operators can see the exact config and endpoint requirements
- credentials must authorize `GET /rest/api/3/myself`, `GET /rest/api/3/search`, `POST /rest/api/3/issue/{key}/comment`, and `POST /rest/api/3/issue/{key}/transitions`
- polling must be configured explicitly through `config.jql` or `config.project_key`

Still missing before broader publication/distribution:

- webhook ingest
- transition discovery and richer polling cursor semantics
- live vendor-account proof in repo automation

### Linear

Current scope:

- validate through the viewer query
- verify webhook signatures and normalize issue webhooks
- execute `create_issue`
- execute `add_comment`

Required config and secrets:

- `config.api_base_url`
- secret `token`
- secret `webhook_secret` only when webhook ingest is desired

Local testing path:

- `go test ./internal/cloud/connectors -count=1`
- generic routed proof lives in `go test ./internal/cloud -count=1`
- no Linear-specific binary smoke exists in the repo today

Publishability-prepared means:

- operators can see the exact config and endpoint requirements
- the configured token must authorize the viewer query plus the issue/comment mutations used by the connector
- webhook callers need a configured webhook secret and a compatible delivery into `/api/cloud/v1/installations/{id}/webhook`

Still missing before broader publication/distribution:

- richer team/project/state discovery
- broader issue workflow actions
- live vendor-account proof in repo automation

### Slack

Current scope:

- validate through `auth.test`
- verify Slack signatures and normalize event callbacks, slash commands, and interactive payloads
- execute `post_message`
- execute `update_message`

Required config and secrets:

- `config.api_base_url`
- secret `token`
- secret `webhook_secret` when webhook ingest is desired

Local testing path:

- `go test ./internal/cloud/connectors -count=1`
- `go test ./internal/cloud -run 'TestServerAdminAndConnectorFlows|TestWebhookHistoryPreservesRepeatedSourceEventIDs|TestConnectorActionLogsCaptureRequestCompletionAndAuditDetails' -count=1`
- `make build-cloud && make cloud-smoke`

Publishability-prepared means:

- operators can see the exact config and endpoint requirements
- the configured token must authorize `auth.test`, `chat.postMessage`, and `chat.update`
- inbound webhook/event handling requires a signing secret and a compatible delivery into `/api/cloud/v1/installations/{id}/webhook`

Still missing before broader publication/distribution:

- broader Slack action surface such as reactions or view submissions
- provider-native install flow or workspace app packaging
- live workspace proof in repo automation

## Local Test Commands

Provider maintainers can use these repo-native checks:

```bash
go test ./internal/cloud/connectors -count=1
go test ./internal/cloud -count=1
make build-cloud
make cloud-smoke
```

Practical split:

- `go test ./internal/cloud/connectors -count=1` is the provider fixture/unit suite
- `go test ./internal/cloud -count=1` adds routed cloud install/webhook/action/worker coverage
- `make cloud-smoke` proves the generic cloud install/validate/webhook/events/audit path, with Slack as the current provider-shaped binary smoke
- `make cloud-stack-smoke` remains the Docker deployment proof path and still requires a Docker-capable host

## What This Phase Does Not Claim

- no provider is marketplace-approved, app-store-ready, or vendor-partner complete
- no provider has a repo-proven OAuth/app-install/bootstrap flow
- no provider has full live vendor-account proof in the repo
- GitHub, GitLab, Linear, and Jira do not yet have provider-specific binary smoke equivalent to Slack's current cloud smoke coverage

For the self-host operator workflow, see [CLOUD_SELF_HOST.md](CLOUD_SELF_HOST.md). For the top-level cloud overview, see [CLOUD.md](CLOUD.md).
