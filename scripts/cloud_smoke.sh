#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_DIR="${BIN_DIR:-$ROOT_DIR/bin}"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/codencer-cloud-smoke.XXXXXX")"
CLOUD_HOST="${CLOUD_HOST:-127.0.0.1}"
CLOUD_PORT="${CLOUD_PORT:-8190}"
CLOUD_URL="${CLOUD_URL:-http://${CLOUD_HOST}:${CLOUD_PORT}}"
CLOUD_ORG_SLUG="${CLOUD_ORG_SLUG:-smoke-org}"
CLOUD_ORG_NAME="${CLOUD_ORG_NAME:-Smoke Org}"
CLOUD_WORKSPACE_SLUG="${CLOUD_WORKSPACE_SLUG:-smoke-workspace}"
CLOUD_WORKSPACE_NAME="${CLOUD_WORKSPACE_NAME:-Smoke Workspace}"
CLOUD_PROJECT_SLUG="${CLOUD_PROJECT_SLUG:-smoke-project}"
CLOUD_PROJECT_NAME="${CLOUD_PROJECT_NAME:-Smoke Project}"
CLOUD_TOKEN_NAME="${CLOUD_TOKEN_NAME:-smoke-operator}"
KEEP_CLOUD_SMOKE_STATE="${KEEP_CLOUD_SMOKE_STATE:-0}"
RELAY_CONFIG="${CLOUD_RELAY_CONFIG:-}"
CLOUD_RUNTIME_CONNECTOR_ID="${CLOUD_RUNTIME_CONNECTOR_ID:-}"
CLOUD_RUNTIME_DAEMON_URL="${CLOUD_RUNTIME_DAEMON_URL:-}"
CLOUD_RUNTIME_ADAPTER="${CLOUD_RUNTIME_ADAPTER:-codex}"
CLOUD_SMOKE_MCP="${CLOUD_SMOKE_MCP:-0}"
CLOUD_SMOKE_SDK="${CLOUD_SMOKE_SDK:-0}"
CLOUD_MCP_ORIGIN="${CLOUD_MCP_ORIGIN:-http://${CLOUD_HOST}:${CLOUD_PORT}}"
MCP_SDK_SMOKE_BIN="${MCP_SDK_SMOKE_BIN:-}"

CLOUD_DB="$TMP_DIR/cloud.db"
CLOUD_CONFIG="$TMP_DIR/cloud.json"
CLOUD_LOG="$TMP_DIR/cloud.log"
CLOUD_PID=""
RUNTIME_RELAY_ADMIN_PID=""
RUNTIME_CONNECTOR_PID=""
RUNTIME_CONNECTOR_CONFIG="$TMP_DIR/runtime-connector.json"
RUNTIME_ENROLLMENT_JSON="$TMP_DIR/runtime-enrollment-token.json"

have_cmd() {
  command -v "$1" >/dev/null 2>&1
}

json_get() {
  local file="$1"
  local expr="$2"
  if have_cmd jq; then
    jq -r "$expr" "$file"
    return
  fi
  if have_cmd python3; then
    python3 - "$file" "$expr" <<'PY'
import json
import sys

path = sys.argv[1]
expr = sys.argv[2]
with open(path, "r", encoding="utf-8") as handle:
    payload = json.load(handle)

value = payload
for part in expr.strip(".").split("."):
    if not part:
        continue
    if isinstance(value, dict):
        value = value.get(part, "")
    else:
        value = ""
        break
if value is None:
    value = ""
print(value)
PY
    return
  fi
  echo "ERROR: cloud_smoke.sh requires jq or python3 for JSON parsing." >&2
  exit 1
}

json_first_field() {
  local file="$1"
  local field="$2"
  if have_cmd jq; then
    jq -r "if length > 0 then .[0].${field} // \"\" else \"\" end" "$file"
    return
  fi
  if have_cmd python3; then
    python3 - "$file" "$field" <<'PY'
import json
import sys

path = sys.argv[1]
field = sys.argv[2]
with open(path, "r", encoding="utf-8") as handle:
    payload = json.load(handle)

value = ""
if isinstance(payload, list) and payload:
    first = payload[0]
    if isinstance(first, dict):
        value = first.get(field, "") or ""
print(value)
PY
    return
  fi
  echo "ERROR: cloud_smoke.sh requires jq or python3 for array JSON parsing." >&2
  exit 1
}

wait_for_health() {
  for _ in $(seq 1 30); do
    if curl -fsS "$CLOUD_URL/healthz" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "ERROR: timed out waiting for cloud daemon at $CLOUD_URL" >&2
  cat "$CLOUD_LOG" >&2 || true
  exit 1
}

curl_json() {
  local method="$1"
  local url="$2"
  local outfile="$3"
  local token="$4"
  local data="${5:-}"
  local args=(-fsS -X "$method" "$url" -H "Authorization: Bearer $token")
  if [[ -n "$data" ]]; then
    args+=(-H 'Content-Type: application/json' -d "$data")
  fi
  curl "${args[@]}" > "$outfile"
}

slack_signature() {
  local secret="$1"
  local body="$2"
  local timestamp="$3"
  if have_cmd python3; then
    python3 - "$secret" "$body" "$timestamp" <<'PY'
import hashlib
import hmac
import sys

secret, body, timestamp = sys.argv[1:]
base = f"v0:{timestamp}:{body}".encode("utf-8")
sig = hmac.new(secret.encode("utf-8"), base, hashlib.sha256).hexdigest()
print(f"v0={sig}")
PY
    return
  fi
  if have_cmd openssl; then
    local digest
    digest="$(
      printf 'v0:%s:%s' "$timestamp" "$body" |
        openssl dgst -sha256 -hmac "$secret" |
        awk '{print $NF}'
    )"
    printf 'v0=%s\n' "$digest"
    return
  fi
  echo "ERROR: cloud_smoke.sh requires python3 or openssl for Slack webhook signing." >&2
  exit 1
}

seed_list_token() {
  local db_path="$1"
  local raw_token="$2"
  local org_id="$3"
  local token_name="$4"
  local scopes_json="$5"
  python3 - "$db_path" "$raw_token" "$org_id" "$token_name" "$scopes_json" <<'PY'
import datetime
import hashlib
import json
import sqlite3
import sys

db_path, raw_token, org_id, token_name, scopes_json = sys.argv[1:]
scopes = json.loads(scopes_json)
now = datetime.datetime.utcnow().replace(microsecond=0).isoformat(sep=" ")
token_hash = hashlib.sha256(raw_token.encode("utf-8")).hexdigest()
token_id = "tok_" + token_hash[:16]
token_prefix = raw_token[:8]

conn = sqlite3.connect(db_path)
try:
    conn.execute("PRAGMA foreign_keys = ON")
    conn.execute(
        """
        INSERT INTO api_tokens (
            id, org_id, workspace_id, project_id, name, kind,
            token_hash, token_prefix, scopes_json, disabled,
            created_at, updated_at, last_used_at, revoked_at
        ) VALUES (?, ?, NULL, NULL, ?, 'reader', ?, ?, ?, 0, ?, ?, NULL, NULL)
        ON CONFLICT(token_hash) DO UPDATE SET
            org_id = excluded.org_id,
            workspace_id = excluded.workspace_id,
            project_id = excluded.project_id,
            name = excluded.name,
            kind = excluded.kind,
            token_prefix = excluded.token_prefix,
            scopes_json = excluded.scopes_json,
            disabled = excluded.disabled,
            updated_at = excluded.updated_at,
            revoked_at = excluded.revoked_at
        """,
        (
            token_id,
            org_id,
            token_name,
            token_hash,
            token_prefix,
            json.dumps(scopes),
            now,
            now,
        ),
    )
    conn.commit()
finally:
    conn.close()
PY
}

cleanup() {
  if [[ -n "$RUNTIME_RELAY_ADMIN_PID" ]] && kill -0 "$RUNTIME_RELAY_ADMIN_PID" >/dev/null 2>&1; then
    kill "$RUNTIME_RELAY_ADMIN_PID" >/dev/null 2>&1 || true
  fi
  if [[ -n "$RUNTIME_CONNECTOR_PID" ]] && kill -0 "$RUNTIME_CONNECTOR_PID" >/dev/null 2>&1; then
    kill "$RUNTIME_CONNECTOR_PID" >/dev/null 2>&1 || true
  fi
  if [[ -n "$CLOUD_PID" ]] && kill -0 "$CLOUD_PID" >/dev/null 2>&1; then
    kill "$CLOUD_PID" >/dev/null 2>&1 || true
  fi
  if [[ "$KEEP_CLOUD_SMOKE_STATE" != "1" ]]; then
    rm -rf "$TMP_DIR"
  else
    echo "cloud smoke temp dir kept at: $TMP_DIR" >&2
  fi
}

trap cleanup EXIT

if [[ ! -x "$BIN_DIR/codencer-cloudctl" || ! -x "$BIN_DIR/codencer-cloudd" || ! -x "$BIN_DIR/codencer-cloudworkerd" ]]; then
  echo "ERROR: expected cloud binaries in $BIN_DIR. Run 'make build-cloud' first." >&2
  exit 1
fi
MCP_SDK_SMOKE_BIN="${MCP_SDK_SMOKE_BIN:-$BIN_DIR/mcp-sdk-smoke}"

if have_cmd openssl; then
  CLOUD_MASTER_KEY="$(openssl rand -hex 32)"
else
  CLOUD_MASTER_KEY="$(LC_ALL=C tr -dc 'a-f0-9' </dev/urandom | head -c 64)"
fi

cat > "$CLOUD_CONFIG" <<EOF
{
  "host": "$CLOUD_HOST",
  "port": $CLOUD_PORT,
  "db_path": "$CLOUD_DB",
  "master_key": "$CLOUD_MASTER_KEY"
}
EOF

BOOTSTRAP_JSON="$TMP_DIR/bootstrap.json"
BOOTSTRAP_TOKEN=""
BOOTSTRAP_TOKEN_ID=""
ORG_ID=""
WORKSPACE_ID=""
PROJECT_ID=""

"$BIN_DIR/codencer-cloudctl" bootstrap \
  --config "$CLOUD_CONFIG" \
  --org-slug "$CLOUD_ORG_SLUG" \
  --org-name "$CLOUD_ORG_NAME" \
  --workspace-slug "$CLOUD_WORKSPACE_SLUG" \
  --workspace-name "$CLOUD_WORKSPACE_NAME" \
  --project-slug "$CLOUD_PROJECT_SLUG" \
  --project-name "$CLOUD_PROJECT_NAME" \
  --token-name "$CLOUD_TOKEN_NAME" \
  --json > "$BOOTSTRAP_JSON"

BOOTSTRAP_TOKEN="$(json_get "$BOOTSTRAP_JSON" '.token')"
BOOTSTRAP_TOKEN_ID="$(json_get "$BOOTSTRAP_JSON" '.record.id')"
ORG_ID="$(json_get "$BOOTSTRAP_JSON" '.org.id')"
WORKSPACE_ID="$(json_get "$BOOTSTRAP_JSON" '.workspace.id')"
PROJECT_ID="$(json_get "$BOOTSTRAP_JSON" '.project.id')"

if [[ -z "$BOOTSTRAP_TOKEN" || -z "$BOOTSTRAP_TOKEN_ID" || -z "$ORG_ID" || -z "$WORKSPACE_ID" || -z "$PROJECT_ID" ]]; then
  echo "ERROR: bootstrap output was missing required identifiers." >&2
  cat "$BOOTSTRAP_JSON" >&2
  exit 1
fi

LIST_TOKEN="cct_$(openssl rand -hex 32)"
seed_list_token \
  "$CLOUD_DB" \
  "$LIST_TOKEN" \
  "$ORG_ID" \
  "Smoke Reader" \
  '["cloud:read","orgs:read","workspaces:read","projects:read","tokens:read","installations:read","events:read","audit:read"]'

RELAY_ARGS=()
if [[ -n "$RELAY_CONFIG" ]]; then
  RELAY_ARGS+=(--relay-config "$RELAY_CONFIG")
fi

if [[ ${#RELAY_ARGS[@]} -gt 0 ]]; then
  nohup "$BIN_DIR/codencer-cloudd" --config "$CLOUD_CONFIG" "${RELAY_ARGS[@]}" > "$CLOUD_LOG" 2>&1 &
else
  nohup "$BIN_DIR/codencer-cloudd" --config "$CLOUD_CONFIG" > "$CLOUD_LOG" 2>&1 &
fi
CLOUD_PID="$!"

wait_for_health

if [[ -n "$RELAY_CONFIG" && -n "$CLOUD_RUNTIME_DAEMON_URL" && -z "$CLOUD_RUNTIME_CONNECTOR_ID" ]]; then
  nohup "$BIN_DIR/codencer-relayd" --config "$RELAY_CONFIG" > "$TMP_DIR/runtime-relay-admin.log" 2>&1 &
  RUNTIME_RELAY_ADMIN_PID="$!"
  for _ in $(seq 1 30); do
    if "$BIN_DIR/codencer-relayd" status --config "$RELAY_CONFIG" >/dev/null 2>&1; then
      break
    fi
    sleep 1
  done
  "$BIN_DIR/codencer-relayd" enrollment-token create \
    --config "$RELAY_CONFIG" \
    --label "cloud-smoke" \
    --expires-in-seconds 600 \
    --json > "$RUNTIME_ENROLLMENT_JSON"
  RUNTIME_ENROLLMENT_TOKEN="$(json_get "$RUNTIME_ENROLLMENT_JSON" '.secret')"
  if [[ -z "$RUNTIME_ENROLLMENT_TOKEN" ]]; then
    echo "ERROR: runtime enrollment token was not created for composed cloud smoke." >&2
    cat "$RUNTIME_ENROLLMENT_JSON" >&2
    exit 1
  fi

  "$BIN_DIR/codencer-connectord" enroll \
    --relay-url "$CLOUD_URL" \
    --daemon-url "$CLOUD_RUNTIME_DAEMON_URL" \
    --enrollment-token "$RUNTIME_ENROLLMENT_TOKEN" \
    --config "$RUNTIME_CONNECTOR_CONFIG" \
    --label "cloud-smoke" >/dev/null

  CLOUD_RUNTIME_CONNECTOR_ID="$(python3 - <<'PY' "$RUNTIME_CONNECTOR_CONFIG"
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as handle:
    print(json.load(handle).get("connector_id", ""))
PY
)"
  if [[ -z "$CLOUD_RUNTIME_CONNECTOR_ID" ]]; then
    echo "ERROR: cloud smoke connector config did not contain a connector id." >&2
    cat "$RUNTIME_CONNECTOR_CONFIG" >&2
    exit 1
  fi

  nohup "$BIN_DIR/codencer-connectord" run --config "$RUNTIME_CONNECTOR_CONFIG" > "$TMP_DIR/runtime-connector.log" 2>&1 &
  RUNTIME_CONNECTOR_PID="$!"
fi

STATUS_JSON="$TMP_DIR/cloud-status.json"
ORGS_JSON="$TMP_DIR/orgs.json"
WORKSPACES_JSON="$TMP_DIR/workspaces.json"
PROJECTS_JSON="$TMP_DIR/projects.json"
TOKENS_JSON="$TMP_DIR/tokens.json"
INSTALL_CREATE_JSON="$TMP_DIR/install-create.json"
INSTALL_LIST_JSON="$TMP_DIR/install-list.json"
INSTALL_GET_JSON="$TMP_DIR/install-get.json"
INSTALL_DISABLE_JSON="$TMP_DIR/install-disable.json"
INSTALL_ENABLE_JSON="$TMP_DIR/install-enable.json"
WEBHOOK_JSON="$TMP_DIR/webhook.json"
EVENTS_JSON="$TMP_DIR/events.json"
AUDIT_JSON="$TMP_DIR/audit.json"
RUNTIME_CLAIM_JSON="$TMP_DIR/runtime-claim.json"
RUNTIME_CONNECTORS_JSON="$TMP_DIR/runtime-connectors.json"
RUNTIME_INSTANCES_JSON="$TMP_DIR/runtime-instances.json"
RUNTIME_RUN_JSON="$TMP_DIR/runtime-run.json"
RUNTIME_RUN_GET_JSON="$TMP_DIR/runtime-run-get.json"
RUNTIME_STEP_JSON="$TMP_DIR/runtime-step.json"
MCP_INIT_HEADERS="$TMP_DIR/cloud-mcp-init.headers"
MCP_INIT_JSON="$TMP_DIR/cloud-mcp-init.json"
MCP_TOOLS_JSON="$TMP_DIR/cloud-mcp-tools.json"
MCP_LIST_JSON="$TMP_DIR/cloud-mcp-list.json"
MCP_SDK_JSON="$TMP_DIR/cloud-mcp-sdk.json"
WORKER_LOG="$TMP_DIR/cloud-worker.log"

"$BIN_DIR/codencer-cloudctl" status --cloud-url "$CLOUD_URL" --token "$BOOTSTRAP_TOKEN" --json > "$STATUS_JSON"
curl_json GET "$CLOUD_URL/api/cloud/v1/orgs" "$ORGS_JSON" "$LIST_TOKEN"
curl_json GET "$CLOUD_URL/api/cloud/v1/workspaces?org_id=$ORG_ID" "$WORKSPACES_JSON" "$LIST_TOKEN"
curl_json GET "$CLOUD_URL/api/cloud/v1/projects?workspace_id=$WORKSPACE_ID" "$PROJECTS_JSON" "$LIST_TOKEN"
curl_json GET "$CLOUD_URL/api/cloud/v1/tokens?org_id=$ORG_ID" "$TOKENS_JSON" "$LIST_TOKEN"

if ! grep -q '"status":"ok"' "$STATUS_JSON"; then
  echo "ERROR: cloud status did not report ok." >&2
  cat "$STATUS_JSON" >&2
  exit 1
fi

if ! grep -q "$ORG_ID" "$ORGS_JSON"; then
  echo "ERROR: org listing did not include the bootstrap org." >&2
  cat "$ORGS_JSON" >&2
  exit 1
fi

if ! grep -q "$WORKSPACE_ID" "$WORKSPACES_JSON"; then
  echo "ERROR: workspace listing did not include the bootstrap workspace." >&2
  cat "$WORKSPACES_JSON" >&2
  exit 1
fi

if ! grep -q "$PROJECT_ID" "$PROJECTS_JSON"; then
  echo "ERROR: project listing did not include the bootstrap project." >&2
  cat "$PROJECTS_JSON" >&2
  exit 1
fi

if ! grep -q "$BOOTSTRAP_TOKEN_ID" "$TOKENS_JSON"; then
  echo "ERROR: token listing did not include the bootstrap token record." >&2
  cat "$TOKENS_JSON" >&2
  exit 1
fi

"$BIN_DIR/codencer-cloudctl" install create \
  --cloud-url "$CLOUD_URL" \
  --token "$BOOTSTRAP_TOKEN" \
  --org-id "$ORG_ID" \
  --workspace-id "$WORKSPACE_ID" \
  --project-id "$PROJECT_ID" \
  --connector slack \
  --name "Smoke Slack" \
  --config api_base_url=http://127.0.0.1:9 \
  --secret token=smoke-token \
  --secret webhook_secret=smoke-secret \
  --json > "$INSTALL_CREATE_JSON"

INSTALL_ID="$(json_get "$INSTALL_CREATE_JSON" '.id')"
if [[ -z "$INSTALL_ID" ]]; then
  echo "ERROR: installation create output did not include an id." >&2
  cat "$INSTALL_CREATE_JSON" >&2
  exit 1
fi

curl_json GET "$CLOUD_URL/api/cloud/v1/installations?org_id=$ORG_ID" "$INSTALL_LIST_JSON" "$LIST_TOKEN"
"$BIN_DIR/codencer-cloudctl" install get --cloud-url "$CLOUD_URL" --token "$BOOTSTRAP_TOKEN" --installation-id "$INSTALL_ID" --json > "$INSTALL_GET_JSON"
"$BIN_DIR/codencer-cloudctl" install disable --cloud-url "$CLOUD_URL" --token "$BOOTSTRAP_TOKEN" --installation-id "$INSTALL_ID" --json > "$INSTALL_DISABLE_JSON"
"$BIN_DIR/codencer-cloudctl" install enable --cloud-url "$CLOUD_URL" --token "$BOOTSTRAP_TOKEN" --installation-id "$INSTALL_ID" --json > "$INSTALL_ENABLE_JSON"
WEBHOOK_BODY='{"type":"event_callback","event_id":"EvSmoke123","event":{"type":"app_mention","user":"U1","channel":"C1","text":"cloud smoke","ts":"1713096000.000100"}}'
WEBHOOK_TS="$(date +%s)"
WEBHOOK_SIG="$(slack_signature "smoke-secret" "$WEBHOOK_BODY" "$WEBHOOK_TS")"
curl -fsS -X POST "$CLOUD_URL/api/cloud/v1/installations/$INSTALL_ID/webhook" \
  -H 'Content-Type: application/json' \
  -H "X-Slack-Request-Timestamp: $WEBHOOK_TS" \
  -H "X-Slack-Signature: $WEBHOOK_SIG" \
  -d "$WEBHOOK_BODY" > "$WEBHOOK_JSON"
"$BIN_DIR/codencer-cloudctl" events \
  --cloud-url "$CLOUD_URL" \
  --token "$LIST_TOKEN" \
  --installation-id "$INSTALL_ID" \
  --json > "$EVENTS_JSON"
"$BIN_DIR/codencer-cloudctl" audit --cloud-url "$CLOUD_URL" --token "$LIST_TOKEN" --json > "$AUDIT_JSON"

if ! grep -q "$INSTALL_ID" "$INSTALL_LIST_JSON"; then
  echo "ERROR: installation list did not include the smoke installation." >&2
  cat "$INSTALL_LIST_JSON" >&2
  exit 1
fi

if ! grep -q '"enabled":false' "$INSTALL_DISABLE_JSON"; then
  echo "ERROR: installation disable did not flip enabled to false." >&2
  cat "$INSTALL_DISABLE_JSON" >&2
  exit 1
fi

if ! grep -q '"enabled":true' "$INSTALL_ENABLE_JSON"; then
  echo "ERROR: installation enable did not flip enabled to true." >&2
  cat "$INSTALL_ENABLE_JSON" >&2
  exit 1
fi

if ! grep -q '"verified":true' "$WEBHOOK_JSON"; then
  echo "ERROR: installation webhook did not verify successfully." >&2
  cat "$WEBHOOK_JSON" >&2
  exit 1
fi

if ! grep -q "$INSTALL_ID" "$EVENTS_JSON"; then
  echo "ERROR: events listing did not include the smoke installation." >&2
  cat "$EVENTS_JSON" >&2
  exit 1
fi

for want in create_installation disable_installation enable_installation webhook_ingest; do
  if ! grep -q "$want" "$AUDIT_JSON"; then
    echo "ERROR: audit trail did not include $want." >&2
    cat "$AUDIT_JSON" >&2
    exit 1
  fi
done

if [[ -n "$RELAY_CONFIG" && -n "$CLOUD_RUNTIME_CONNECTOR_ID" ]]; then
  "$BIN_DIR/codencer-cloudctl" runtime-connectors claim \
    --cloud-url "$CLOUD_URL" \
    --token "$BOOTSTRAP_TOKEN" \
    --org-id "$ORG_ID" \
    --workspace-id "$WORKSPACE_ID" \
    --project-id "$PROJECT_ID" \
    --connector-id "$CLOUD_RUNTIME_CONNECTOR_ID" \
    --json > "$RUNTIME_CLAIM_JSON"

  RUNTIME_CONNECTOR_RECORD_ID="$(json_get "$RUNTIME_CLAIM_JSON" '.id')"
  if [[ -z "$RUNTIME_CONNECTOR_RECORD_ID" ]]; then
    echo "ERROR: runtime connector claim did not return a record id." >&2
    cat "$RUNTIME_CLAIM_JSON" >&2
    exit 1
  fi

  "$BIN_DIR/codencer-cloudctl" runtime-connectors list \
    --cloud-url "$CLOUD_URL" \
    --token "$BOOTSTRAP_TOKEN" \
    --org-id "$ORG_ID" \
    --json > "$RUNTIME_CONNECTORS_JSON"

  for _ in $(seq 1 30); do
    "$BIN_DIR/codencer-cloudctl" runtime-instances list \
      --cloud-url "$CLOUD_URL" \
      --token "$BOOTSTRAP_TOKEN" \
      --org-id "$ORG_ID" \
      --json > "$RUNTIME_INSTANCES_JSON"
    if [[ -n "$(json_first_field "$RUNTIME_INSTANCES_JSON" 'instance_id')" ]]; then
      break
    fi
    sleep 1
  done

  if ! grep -q "$CLOUD_RUNTIME_CONNECTOR_ID" "$RUNTIME_CONNECTORS_JSON"; then
    echo "ERROR: runtime connector list did not include the claimed relay connector." >&2
    cat "$RUNTIME_CONNECTORS_JSON" >&2
    exit 1
  fi

  CLOUD_INSTANCE_ID="$(json_first_field "$RUNTIME_INSTANCES_JSON" 'instance_id')"
  if [[ -z "$CLOUD_INSTANCE_ID" ]]; then
    echo "ERROR: runtime instance list did not expose a tenant-visible shared instance." >&2
    cat "$RUNTIME_INSTANCES_JSON" >&2
    exit 1
  fi

  CLOUD_RUNTIME_RUN_ID="cloud-runtime-smoke-$(date +%s)"
  curl_json POST "$CLOUD_URL/api/cloud/v1/runtime/instances/$CLOUD_INSTANCE_ID/runs" "$RUNTIME_RUN_JSON" "$BOOTSTRAP_TOKEN" "{\"id\":\"$CLOUD_RUNTIME_RUN_ID\",\"project_id\":\"cloud-smoke-project\"}"
  curl_json GET "$CLOUD_URL/api/cloud/v1/runtime/instances/$CLOUD_INSTANCE_ID/runs/$CLOUD_RUNTIME_RUN_ID" "$RUNTIME_RUN_GET_JSON" "$BOOTSTRAP_TOKEN"
  curl_json POST "$CLOUD_URL/api/cloud/v1/runtime/instances/$CLOUD_INSTANCE_ID/runs/$CLOUD_RUNTIME_RUN_ID/steps" "$RUNTIME_STEP_JSON" "$BOOTSTRAP_TOKEN" "{\"version\":\"v1\",\"goal\":\"Verify the cloud runtime HTTP path\",\"adapter_profile\":\"$CLOUD_RUNTIME_ADAPTER\"}"

  if ! grep -q "\"id\":\"$CLOUD_RUNTIME_RUN_ID\"" "$RUNTIME_RUN_JSON"; then
    echo "ERROR: cloud runtime run create did not return the requested run id." >&2
    cat "$RUNTIME_RUN_JSON" >&2
    exit 1
  fi

  if ! grep -q "\"id\":\"$CLOUD_RUNTIME_RUN_ID\"" "$RUNTIME_RUN_GET_JSON"; then
    echo "ERROR: cloud runtime run readback did not include the requested run id." >&2
    cat "$RUNTIME_RUN_GET_JSON" >&2
    exit 1
  fi

  if ! grep -q '"id"' "$RUNTIME_STEP_JSON"; then
    echo "ERROR: cloud runtime submit_task proxy did not return a step id." >&2
    cat "$RUNTIME_STEP_JSON" >&2
    exit 1
  fi

  if [[ "$CLOUD_SMOKE_MCP" == "1" ]]; then
    curl -fsS -D "$MCP_INIT_HEADERS" -X POST "$CLOUD_URL/api/cloud/v1/mcp" \
      -H "Authorization: Bearer $BOOTSTRAP_TOKEN" \
      -H 'Content-Type: application/json' \
      -H 'Accept: application/json, text/event-stream' \
      -H "Origin: $CLOUD_MCP_ORIGIN" \
      -H 'MCP-Protocol-Version: 2025-11-25' \
      -d '{"jsonrpc":"2.0","id":"init-1","method":"initialize","params":{"protocolVersion":"2025-11-25"}}' \
      > "$MCP_INIT_JSON"

    CLOUD_MCP_SESSION_ID="$(awk 'tolower($1)=="mcp-session-id:"{print $2}' "$MCP_INIT_HEADERS" | tr -d '\r' | tail -n 1)"
    if [[ -z "$CLOUD_MCP_SESSION_ID" ]]; then
      echo "ERROR: cloud MCP initialize did not return a session id." >&2
      cat "$MCP_INIT_HEADERS" >&2
      cat "$MCP_INIT_JSON" >&2
      exit 1
    fi

    curl -fsS -X POST "$CLOUD_URL/api/cloud/v1/mcp/call" \
      -H "Authorization: Bearer $BOOTSTRAP_TOKEN" \
      -H 'Content-Type: application/json' \
      -H 'Accept: application/json, text/event-stream' \
      -H "Origin: $CLOUD_MCP_ORIGIN" \
      -H "MCP-Session-Id: $CLOUD_MCP_SESSION_ID" \
      -H 'MCP-Protocol-Version: 2025-11-25' \
      -d '{"jsonrpc":"2.0","id":"tools-1","method":"tools/list","params":{}}' \
      > "$MCP_TOOLS_JSON"

    curl -fsS -X POST "$CLOUD_URL/api/cloud/v1/mcp/call" \
      -H "Authorization: Bearer $BOOTSTRAP_TOKEN" \
      -H 'Content-Type: application/json' \
      -H 'Accept: application/json, text/event-stream' \
      -H "Origin: $CLOUD_MCP_ORIGIN" \
      -H "MCP-Session-Id: $CLOUD_MCP_SESSION_ID" \
      -H 'MCP-Protocol-Version: 2025-11-25' \
      -d '{"jsonrpc":"2.0","id":"call-1","name":"codencer.list_instances","arguments":{}}' \
      > "$MCP_LIST_JSON"

    if ! grep -q '"tools"' "$MCP_TOOLS_JSON"; then
      echo "ERROR: cloud MCP tools/list did not return tools." >&2
      cat "$MCP_TOOLS_JSON" >&2
      exit 1
    fi

    if ! grep -q "$CLOUD_INSTANCE_ID" "$MCP_LIST_JSON"; then
      echo "ERROR: cloud MCP list_instances did not return the tenant-visible instance." >&2
      cat "$MCP_LIST_JSON" >&2
      exit 1
    fi
  fi

  if [[ "$CLOUD_SMOKE_SDK" == "1" ]]; then
    if [[ ! -x "$MCP_SDK_SMOKE_BIN" ]]; then
      echo "ERROR: CLOUD_SMOKE_SDK=1 requires $MCP_SDK_SMOKE_BIN. Run 'make build-mcp-sdk-smoke' first." >&2
      exit 1
    fi
    sleep 1
    "$MCP_SDK_SMOKE_BIN" \
      --endpoint "$CLOUD_URL/api/cloud/v1/mcp" \
      --token "$BOOTSTRAP_TOKEN" \
      --origin "$CLOUD_MCP_ORIGIN" \
      --instance-id "$CLOUD_INSTANCE_ID" \
      --run-id "cloud-sdk-smoke-$(date +%s)" \
      --project-id "cloud-sdk-project" \
      --adapter-profile "$CLOUD_RUNTIME_ADAPTER" \
      --wait-timeout-ms 10000 \
      --json > "$MCP_SDK_JSON"

    if [[ "$(json_get "$MCP_SDK_JSON" '.instance_id')" != "$CLOUD_INSTANCE_ID" ]]; then
      echo "ERROR: cloud MCP SDK smoke did not report the target instance." >&2
      cat "$MCP_SDK_JSON" >&2
      exit 1
    fi
  fi
fi

"$BIN_DIR/codencer-cloudworkerd" --config "$CLOUD_CONFIG" --once > "$WORKER_LOG" 2>&1

echo "Cloud smoke completed successfully."
echo "  cloud_url: $CLOUD_URL"
echo "  org_id: $ORG_ID"
echo "  workspace_id: $WORKSPACE_ID"
echo "  project_id: $PROJECT_ID"
echo "  installation_id: $INSTALL_ID"
if [[ -s "$EVENTS_JSON" ]]; then
  echo "  events_json: $EVENTS_JSON"
fi
if [[ -s "$RUNTIME_RUN_JSON" ]]; then
  echo "  runtime_run_json: $RUNTIME_RUN_JSON"
fi
if [[ -s "$MCP_LIST_JSON" ]]; then
  echo "  cloud_mcp_list_json: $MCP_LIST_JSON"
fi
if [[ -s "$MCP_SDK_JSON" ]]; then
  echo "  cloud_mcp_sdk_json: $MCP_SDK_JSON"
fi
