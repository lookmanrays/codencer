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

CLOUD_DB="$TMP_DIR/cloud.db"
CLOUD_CONFIG="$TMP_DIR/cloud.json"
CLOUD_LOG="$TMP_DIR/cloud.log"
CLOUD_PID=""

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

STATUS_JSON="$TMP_DIR/status.json"
ORGS_JSON="$TMP_DIR/orgs.json"
WORKSPACES_JSON="$TMP_DIR/workspaces.json"
PROJECTS_JSON="$TMP_DIR/projects.json"
TOKENS_JSON="$TMP_DIR/tokens.json"
INSTALL_CREATE_JSON="$TMP_DIR/install-create.json"
INSTALL_LIST_JSON="$TMP_DIR/install-list.json"
INSTALL_GET_JSON="$TMP_DIR/install-get.json"
INSTALL_DISABLE_JSON="$TMP_DIR/install-disable.json"
INSTALL_ENABLE_JSON="$TMP_DIR/install-enable.json"
AUDIT_JSON="$TMP_DIR/audit.json"
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

for want in create_installation disable_installation enable_installation; do
  if ! grep -q "$want" "$AUDIT_JSON"; then
    echo "ERROR: audit trail did not include $want." >&2
    cat "$AUDIT_JSON" >&2
    exit 1
  fi
done

"$BIN_DIR/codencer-cloudworkerd" --config "$CLOUD_CONFIG" --once > "$WORKER_LOG" 2>&1

echo "Cloud smoke completed successfully."
echo "  cloud_url: $CLOUD_URL"
echo "  org_id: $ORG_ID"
echo "  workspace_id: $WORKSPACE_ID"
echo "  project_id: $PROJECT_ID"
echo "  installation_id: $INSTALL_ID"
