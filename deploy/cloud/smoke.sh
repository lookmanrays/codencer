#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
STACK_DIR="$ROOT_DIR/deploy/cloud"
COMPOSE_FILE="$STACK_DIR/docker-compose.yml"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/codencer-cloud-stack-smoke.XXXXXX")"
ENV_FILE="$TMP_DIR/cloud-stack.env"
BOOTSTRAP_JSON="$TMP_DIR/bootstrap.json"
STATUS_JSON="$TMP_DIR/status.json"
INSTALL_JSON="$TMP_DIR/install.json"
AUDIT_JSON="$TMP_DIR/audit.json"

CLOUD_PORT="${CODENCER_CLOUD_PORT:-18190}"
COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-codencer-cloud-smoke}"
KEEP_CLOUD_STACK_SMOKE_STATE="${KEEP_CLOUD_STACK_SMOKE_STATE:-0}"

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
}

compose() {
  docker compose \
    --project-name "$COMPOSE_PROJECT_NAME" \
    --env-file "$ENV_FILE" \
    -f "$COMPOSE_FILE" \
    "$@"
}

cleanup() {
  compose down -v >/dev/null 2>&1 || true
  if [[ "$KEEP_CLOUD_STACK_SMOKE_STATE" != "1" ]]; then
    rm -rf "$TMP_DIR"
  else
    echo "cloud stack smoke temp dir kept at: $TMP_DIR" >&2
  fi
}

trap cleanup EXIT

if ! have_cmd docker; then
  echo "ERROR: docker is required for deploy/cloud/smoke.sh" >&2
  exit 1
fi

if have_cmd openssl; then
  CLOUD_MASTER_KEY="$(openssl rand -hex 32)"
  RELAY_PLANNER_TOKEN="planner_$(openssl rand -hex 24)"
  RELAY_ENROLLMENT_SECRET="enroll_$(openssl rand -hex 24)"
else
  CLOUD_MASTER_KEY="$(LC_ALL=C tr -dc 'a-f0-9' </dev/urandom | head -c 64)"
  RELAY_PLANNER_TOKEN="planner_$(LC_ALL=C tr -dc 'a-f0-9' </dev/urandom | head -c 48)"
  RELAY_ENROLLMENT_SECRET="enroll_$(LC_ALL=C tr -dc 'a-f0-9' </dev/urandom | head -c 48)"
fi

cat > "$ENV_FILE" <<EOF
CODENCER_CLOUD_PORT=$CLOUD_PORT
CODENCER_CLOUD_MASTER_KEY=$CLOUD_MASTER_KEY
RELAY_PLANNER_TOKEN=$RELAY_PLANNER_TOKEN
RELAY_ENROLLMENT_SECRET=$RELAY_ENROLLMENT_SECRET
EOF

compose build cloud worker >/dev/null

compose run --rm --entrypoint codencer-cloudctl cloud \
  bootstrap \
  --config /etc/codencer/cloud/config.json \
  --org-slug smoke-org \
  --org-name "Smoke Org" \
  --workspace-slug smoke-workspace \
  --workspace-name "Smoke Workspace" \
  --project-slug smoke-project \
  --project-name "Smoke Project" \
  --token-name smoke-operator \
  --json > "$BOOTSTRAP_JSON"

BOOTSTRAP_TOKEN="$(json_get "$BOOTSTRAP_JSON" '.token')"
ORG_ID="$(json_get "$BOOTSTRAP_JSON" '.org.id')"
WORKSPACE_ID="$(json_get "$BOOTSTRAP_JSON" '.workspace.id')"
PROJECT_ID="$(json_get "$BOOTSTRAP_JSON" '.project.id')"

if [[ -z "$BOOTSTRAP_TOKEN" || -z "$ORG_ID" || -z "$WORKSPACE_ID" || -z "$PROJECT_ID" ]]; then
  echo "ERROR: bootstrap output missing required identifiers" >&2
  cat "$BOOTSTRAP_JSON" >&2
  exit 1
fi

compose up -d cloud worker >/dev/null

for _ in $(seq 1 30); do
  if curl -fsS "http://127.0.0.1:$CLOUD_PORT/healthz" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

curl -fsS -H "Authorization: Bearer $BOOTSTRAP_TOKEN" \
  "http://127.0.0.1:$CLOUD_PORT/api/cloud/v1/status" > "$STATUS_JSON"

compose run --rm --entrypoint codencer-cloudctl cloud \
  install create \
  --cloud-url http://cloud:8190 \
  --token "$BOOTSTRAP_TOKEN" \
  --org-id "$ORG_ID" \
  --workspace-id "$WORKSPACE_ID" \
  --project-id "$PROJECT_ID" \
  --connector slack \
  --name "Slack stack smoke" \
  --config api_base_url=https://slack.com \
  --secret token=xoxb-stack-smoke \
  --secret webhook_secret=stack-secret \
  --json > "$INSTALL_JSON"

compose run --rm --entrypoint codencer-cloudctl cloud \
  audit \
  --cloud-url http://cloud:8190 \
  --token "$BOOTSTRAP_TOKEN" \
  --json > "$AUDIT_JSON"

if ! grep -q '"status":"ok"' "$STATUS_JSON"; then
  echo "ERROR: cloud stack status did not report ok" >&2
  cat "$STATUS_JSON" >&2
  exit 1
fi

if ! grep -q '"connector_key":"slack"' "$INSTALL_JSON"; then
  echo "ERROR: cloud stack installation create did not return slack installation" >&2
  cat "$INSTALL_JSON" >&2
  exit 1
fi

if ! grep -q '"action":"create_installation"' "$AUDIT_JSON"; then
  echo "ERROR: cloud stack audit did not record installation creation" >&2
  cat "$AUDIT_JSON" >&2
  exit 1
fi

echo "cloud stack smoke passed on http://127.0.0.1:$CLOUD_PORT"
