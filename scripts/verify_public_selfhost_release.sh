#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMPDIR_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/codencer-public-selfhost.XXXXXX")"
DAEMON_PID=""

cleanup() {
  if [[ -n "${DAEMON_PID:-}" ]] && kill -0 "$DAEMON_PID" 2>/dev/null; then
    kill "$DAEMON_PID" 2>/dev/null || true
    wait "$DAEMON_PID" 2>/dev/null || true
  fi
  rm -rf "$TMPDIR_ROOT"
}
trap cleanup EXIT

free_port() {
  python3 - <<'PY'
import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()
PY
}

json_get() {
  python3 - "$1" "$2" <<'PY'
import json, sys
value=json.load(open(sys.argv[1]))
for part in sys.argv[2].split("."):
  if part:
    value=value[part] if not part.isdigit() else value[int(part)]
print(value)
PY
}

assert_no_commercial_endpoint() {
  local file="$1"
  if grep -q 'https://mcp.codencer.dev\|https://relay.codencer.dev\|https://app.codencer.dev' "$file"; then
    echo "commercial endpoint appeared in public self-host proof: $file" >&2
    cat "$file" >&2
    exit 1
  fi
}

required_timeout_seconds() {
  python3 - <<'PY'
import os
raw=os.environ.get("CODENCER_E2E_EXECUTOR_TIMEOUT_SECONDS", "").strip()
minimum=300
if raw:
  try:
    value=int(raw)
  except ValueError:
    raise SystemExit(f"CODENCER_E2E_EXECUTOR_TIMEOUT_SECONDS must be an integer, got {raw!r}")
  if value > minimum:
    minimum=value
print(minimum)
PY
}

assert_json_number_at_least() {
  local file="$1"
  local path="$2"
  local minimum="$3"
  local label="$4"
  python3 - "$file" "$path" "$minimum" "$label" <<'PY'
import json, sys
file,path,minimum,label=sys.argv[1],sys.argv[2],int(sys.argv[3]),sys.argv[4]
value=json.load(open(file))
for part in path.split("."):
  if part:
    value=value[part] if not part.isdigit() else value[int(part)]
if not isinstance(value, int):
  raise SystemExit(f"{label}: expected integer at {path}, got {value!r}")
if value < minimum:
  raise SystemExit(f"{label}: expected {path} >= {minimum}, got {value}")
PY
}

require_help_flags() {
  local help_file="$1"
  shift
  local flag
  for flag in "$@"; do
    if ! grep -q -- "$flag" "$help_file"; then
      echo "help output missing $flag in $help_file" >&2
      cat "$help_file" >&2
      exit 1
    fi
  done
}

assert_no_default_output_leak() {
  local file="$1"
  local label="$2"
  if grep -Fq "$TMPDIR_ROOT" "$file"; then
    echo "$label leaked verifier temp path" >&2
    cat "$file" >&2
    exit 1
  fi
  if grep -Eq '(/Users/[^[:space:]<>"'\'']+|/home/[^[:space:]<>"'\'']+|/var/folders/[^[:space:]<>"'\'']+|/tmp/[^[:space:]<>"'\'']+)' "$file"; then
    echo "$label leaked an absolute local path" >&2
    cat "$file" >&2
    exit 1
  fi
  if grep -Eiq '(access_token|refresh_token|private_key|client_secret|bearer [A-Za-z0-9._~+/=-]{8,})' "$file"; then
    echo "$label leaked token-like material" >&2
    cat "$file" >&2
    exit 1
  fi
}

wait_for_daemon() {
  local daemon_url="$1"
  local daemon_log="$2"
  for _ in $(seq 1 150); do
    if curl -fsS "$daemon_url/health" >/dev/null 2>&1; then
      return 0
    fi
    if [[ -n "${DAEMON_PID:-}" ]] && ! kill -0 "$DAEMON_PID" 2>/dev/null; then
      echo "daemon exited early; log follows" >&2
      cat "$daemon_log" >&2 || true
      return 1
    fi
    sleep 0.1
  done
  echo "daemon did not become healthy at $daemon_url; log follows" >&2
  cat "$daemon_log" >&2 || true
  return 1
}

verify_default_run_output_redaction() {
  local exec_repo="$TMPDIR_ROOT/local-exec-repo"
  local exec_home="$TMPDIR_ROOT/local-exec-home"
  local state="$TMPDIR_ROOT/local-exec-state"
  local daemon_port daemon_url daemon_config daemon_log run_id events_run_id
  mkdir -p "$exec_repo" "$exec_home" "$state"

  git -C "$exec_repo" init -q
  printf '# Public self-host local run redaction probe\n' > "$exec_repo/README.md"
  git -C "$exec_repo" add README.md
  git -C "$exec_repo" -c user.name=Codencer -c user.email=codencer@example.invalid commit -q -m "initial"

  daemon_port="$(free_port)"
  daemon_url="http://127.0.0.1:$daemon_port"
  daemon_config="$TMPDIR_ROOT/local-exec-daemon.json"
  daemon_log="$TMPDIR_ROOT/local-exec-daemon.log"
  cat > "$daemon_config" <<JSON
{
  "log_level": "error",
  "db_path": "$state/codencer.db",
  "artifact_root": "$state/artifacts",
  "workspace_root": "$state/workspace",
  "repo_root": "$exec_repo",
  "host": "127.0.0.1",
  "port": $daemon_port
}
JSON

  env \
    PORT="$daemon_port" \
    HOST="127.0.0.1" \
    DB_PATH="$state/codencer.db" \
    ARTIFACT_ROOT="$state/artifacts" \
    WORKSPACE_ROOT="$state/workspace" \
    LOG_LEVEL="error" \
    REPO_ROOT="$exec_repo" \
    "$ROOT/bin/orchestratord" --config "$daemon_config" --repo-root "$exec_repo" > "$daemon_log" 2>&1 &
  DAEMON_PID="$!"
  wait_for_daemon "$daemon_url" "$daemon_log"

  CODENCER_HOME="$exec_home" "$ROOT/bin/codencer" init --json >/dev/null
  CODENCER_HOME="$exec_home" "$ROOT/bin/codencer" project init \
    --id codencer \
    --repo "$exec_repo" \
    --adapter fake \
    --profile fake-success \
    --daemon-url "$daemon_url" \
    --json >/dev/null

  CODENCER_HOME="$exec_home" "$ROOT/bin/codencer" run start --project codencer --json > "$TMPDIR_ROOT/run-start.json"
  events_run_id="$(json_get "$TMPDIR_ROOT/run-start.json" "run.id")"
  CODENCER_HOME="$exec_home" "$ROOT/bin/codencer" run events "$events_run_id" --project codencer > "$TMPDIR_ROOT/run-events-human.txt"
  assert_no_default_output_leak "$TMPDIR_ROOT/run-events-human.txt" "codencer run events human output"
  grep -q 'event:' "$TMPDIR_ROOT/run-events-human.txt" || { cat "$TMPDIR_ROOT/run-events-human.txt" >&2; exit 1; }

  run_id="public-redaction-run"
  CODENCER_HOME="$exec_home" "$ROOT/bin/codencer" submit \
    --project codencer \
    --run "$run_id" \
    --goal "fake success submit" \
    --profile fake-success \
    --wait > "$TMPDIR_ROOT/submit-human.txt"
  assert_no_default_output_leak "$TMPDIR_ROOT/submit-human.txt" "codencer submit human output"
  grep -q 'summary:' "$TMPDIR_ROOT/submit-human.txt" || { cat "$TMPDIR_ROOT/submit-human.txt" >&2; exit 1; }

  CODENCER_HOME="$exec_home" "$ROOT/bin/codencer" run report "$run_id" --project codencer > "$TMPDIR_ROOT/run-report-human.txt"
  assert_no_default_output_leak "$TMPDIR_ROOT/run-report-human.txt" "codencer run report human output"
  grep -q 'summary:' "$TMPDIR_ROOT/run-report-human.txt" || { cat "$TMPDIR_ROOT/run-report-human.txt" >&2; exit 1; }

  set +e
  CODENCER_HOME="$exec_home" "$ROOT/bin/codencer" run resume "$run_id" --project codencer > "$TMPDIR_ROOT/run-resume-human.txt"
  local resume_code=$?
  set -e
  if [[ "$resume_code" -eq 0 ]]; then
    echo "codencer run resume unexpectedly succeeded in capability-blocker proof" >&2
    cat "$TMPDIR_ROOT/run-resume-human.txt" >&2
    exit 1
  fi
  assert_no_default_output_leak "$TMPDIR_ROOT/run-resume-human.txt" "codencer run resume human output"
  grep -q 'blocker: unsupported_operation' "$TMPDIR_ROOT/run-resume-human.txt" || { cat "$TMPDIR_ROOT/run-resume-human.txt" >&2; exit 1; }

  kill "$DAEMON_PID" 2>/dev/null || true
  wait "$DAEMON_PID" 2>/dev/null || true
  DAEMON_PID=""
}

home="$TMPDIR_ROOT/home"
repo="$TMPDIR_ROOT/repo"
gateway_port="$(free_port)"
relay_port="$(free_port)"
console_port="$(free_port)"
gateway_url="http://127.0.0.1:$gateway_port"
relay_url="http://127.0.0.1:$relay_port"
console_url="http://127.0.0.1:$console_port"
required_timeout="$(required_timeout_seconds)"

echo "Public self-host release verification ports: gateway=$gateway_port relay=$relay_port console=$console_port"
echo "Public self-host release timeout requirement: >=${required_timeout}s"

export CODENCER_HOME="$home"
mkdir -p "$repo/.git"
printf '# Public self-host redaction probe\n' > "$repo/README.md"

"$ROOT/bin/codencer" init > "$TMPDIR_ROOT/init-human.txt"
assert_no_default_output_leak "$TMPDIR_ROOT/init-human.txt" "codencer init human output"
"$ROOT/bin/codencer" init --json > "$TMPDIR_ROOT/init.json"
"$ROOT/bin/codencer" config show --json > "$TMPDIR_ROOT/config-default.json"
assert_no_commercial_endpoint "$TMPDIR_ROOT/config-default.json"
test "$(json_get "$TMPDIR_ROOT/config-default.json" "resolved_connection.gateway_url")" = "http://127.0.0.1:19090"
test "$(json_get "$TMPDIR_ROOT/config-default.json" "resolved_connection.mcp_url")" = "http://127.0.0.1:19090/mcp"
test "$(json_get "$TMPDIR_ROOT/config-default.json" "resolved_connection.relay_url")" = "http://127.0.0.1:8090"
test "$(json_get "$TMPDIR_ROOT/config-default.json" "config.active_profile")" = "self-host"

"$ROOT/bin/codencer" config show > "$TMPDIR_ROOT/config-default-human.txt"
assert_no_default_output_leak "$TMPDIR_ROOT/config-default-human.txt" "codencer config show human output"
"$ROOT/bin/codencer" config profiles list > "$TMPDIR_ROOT/config-profiles-list-human.txt"
assert_no_default_output_leak "$TMPDIR_ROOT/config-profiles-list-human.txt" "codencer config profiles list human output"
"$ROOT/bin/codencer" config profiles use self-host > "$TMPDIR_ROOT/config-profiles-use-human.txt"
assert_no_default_output_leak "$TMPDIR_ROOT/config-profiles-use-human.txt" "codencer config profiles use human output"
"$ROOT/bin/codencer" config set gateway.url "$gateway_url" > "$TMPDIR_ROOT/config-set-human.txt"
assert_no_default_output_leak "$TMPDIR_ROOT/config-set-human.txt" "codencer config set human output"
(cd "$repo" && "$ROOT/bin/codencer" project init --id codencer > "$TMPDIR_ROOT/project-init-human.txt")
assert_no_default_output_leak "$TMPDIR_ROOT/project-init-human.txt" "codencer project init human output"
(cd "$repo" && "$ROOT/bin/codencer" project status > "$TMPDIR_ROOT/project-status-human.txt")
assert_no_default_output_leak "$TMPDIR_ROOT/project-status-human.txt" "codencer project status human output"
(cd "$repo" && "$ROOT/bin/codencer" project scan > "$TMPDIR_ROOT/project-scan-human.txt")
assert_no_default_output_leak "$TMPDIR_ROOT/project-scan-human.txt" "codencer project scan human output"
"$ROOT/bin/codencer" executor list > "$TMPDIR_ROOT/executor-list-human.txt"
assert_no_default_output_leak "$TMPDIR_ROOT/executor-list-human.txt" "codencer executor list human output"
"$ROOT/bin/codencer" executor scan --repo "$repo" > "$TMPDIR_ROOT/executor-scan-human.txt"
assert_no_default_output_leak "$TMPDIR_ROOT/executor-scan-human.txt" "codencer executor scan human output"
"$ROOT/bin/codencer" executor test fake-success > "$TMPDIR_ROOT/executor-test-human.txt"
assert_no_default_output_leak "$TMPDIR_ROOT/executor-test-human.txt" "codencer executor test human output"
"$ROOT/bin/codencer" executor default fake-success --repo "$repo" > "$TMPDIR_ROOT/executor-default-human.txt"
assert_no_default_output_leak "$TMPDIR_ROOT/executor-default-human.txt" "codencer executor default human output"
"$ROOT/bin/codencer" sync preview > "$TMPDIR_ROOT/sync-preview-human.txt"
assert_no_default_output_leak "$TMPDIR_ROOT/sync-preview-human.txt" "codencer sync preview human output"
verify_default_run_output_redaction

"$ROOT/bin/codencer" config profiles list --json > "$TMPDIR_ROOT/profiles.json"
grep -q '"name": "self-host"' "$TMPDIR_ROOT/profiles.json" || { cat "$TMPDIR_ROOT/profiles.json" >&2; exit 1; }

CODENCER_GATEWAY_URL="$gateway_url" "$ROOT/bin/codencer" config show --json > "$TMPDIR_ROOT/config-env.json"
test "$(json_get "$TMPDIR_ROOT/config-env.json" "resolved_connection.gateway_url")" = "$gateway_url"
test "$(json_get "$TMPDIR_ROOT/config-env.json" "resolved_connection.source")" = "env:CODENCER_GATEWAY_URL"

"$ROOT/bin/codencer" config set gateway.url "$gateway_url" --json > "$TMPDIR_ROOT/config-set.json"
"$ROOT/bin/codencer" config set relay.url "$relay_url" --json > "$TMPDIR_ROOT/config-set-relay.json"
"$ROOT/bin/codencer" config set console.url "$console_url" --json > "$TMPDIR_ROOT/config-set-console.json"
"$ROOT/bin/codencer" config profiles use self-host --json > "$TMPDIR_ROOT/profile-use.json"
"$ROOT/bin/codencer" config show --json > "$TMPDIR_ROOT/config-profile.json"
test "$(json_get "$TMPDIR_ROOT/config-profile.json" "resolved_connection.gateway_url")" = "$gateway_url"
test "$(json_get "$TMPDIR_ROOT/config-profile.json" "resolved_connection.relay_url")" = "$relay_url"
test "$(json_get "$TMPDIR_ROOT/config-profile.json" "resolved_connection.console_url")" = "$console_url"

"$ROOT/bin/codencer" setup self-host \
  --gateway-url "$gateway_url" \
  --mcp-url "$gateway_url/mcp" \
  --relay-url "$relay_url" \
  --console-url "$console_url" \
  --listen "127.0.0.1:$gateway_port" \
  --token-env CODENCER_GATEWAY_MCP_TOKEN \
  --default-relay-token-env CODENCER_DEFAULT_RELAY_TOKEN \
  --enable-oauth-dev \
  --oauth-client-secret public-selfhost-client-secret \
  --json > "$TMPDIR_ROOT/setup-selfhost.json"
"$ROOT/bin/codencer" setup relay \
  --base-url "$relay_url" \
  --mcp-url "$relay_url/mcp" \
  --generate-planner-token \
  --json > "$TMPDIR_ROOT/setup-relay.json"
"$ROOT/bin/codencer" setup self-host \
  --gateway-url "$gateway_url" \
  --mcp-url "$gateway_url/mcp" \
  --relay-url "$relay_url" \
  --console-url "$console_url" \
  --listen "127.0.0.1:$gateway_port" \
  --token-env CODENCER_GATEWAY_MCP_TOKEN \
  --default-relay-token-env CODENCER_DEFAULT_RELAY_TOKEN \
  --enable-oauth-dev \
  --oauth-client-secret public-selfhost-client-secret-human > "$TMPDIR_ROOT/setup-selfhost-human.txt"
assert_no_default_output_leak "$TMPDIR_ROOT/setup-selfhost-human.txt" "codencer setup self-host human output"
grep -q '<redacted-local-path>' "$TMPDIR_ROOT/setup-selfhost-human.txt" || { cat "$TMPDIR_ROOT/setup-selfhost-human.txt" >&2; exit 1; }
if grep -q 'public-selfhost-client-secret-human' "$TMPDIR_ROOT/setup-selfhost-human.txt"; then
  echo "setup self-host human output leaked OAuth client secret" >&2
  cat "$TMPDIR_ROOT/setup-selfhost-human.txt" >&2
  exit 1
fi
"$ROOT/bin/codencer" setup relay \
  --base-url "$relay_url" \
  --mcp-url "$relay_url/mcp" \
  --generate-planner-token > "$TMPDIR_ROOT/setup-relay-human.txt"
assert_no_default_output_leak "$TMPDIR_ROOT/setup-relay-human.txt" "codencer setup relay human output"
grep -q '<redacted-local-path>' "$TMPDIR_ROOT/setup-relay-human.txt" || { cat "$TMPDIR_ROOT/setup-relay-human.txt" >&2; exit 1; }
"$ROOT/bin/codencer" setup self-host --help > "$TMPDIR_ROOT/setup-selfhost-help.txt"
"$ROOT/bin/codencer" setup relay --help > "$TMPDIR_ROOT/setup-relay-help.txt"
assert_no_commercial_endpoint "$TMPDIR_ROOT/setup-selfhost.json"
assert_no_commercial_endpoint "$TMPDIR_ROOT/setup-relay.json"
grep -q '"mode": "self-host"' "$TMPDIR_ROOT/setup-selfhost.json" || { cat "$TMPDIR_ROOT/setup-selfhost.json" >&2; exit 1; }
if grep -q 'public-selfhost-client-secret' "$TMPDIR_ROOT/setup-selfhost.json"; then
  echo "setup self-host leaked OAuth client secret" >&2
  cat "$TMPDIR_ROOT/setup-selfhost.json" >&2
  exit 1
fi
assert_json_number_at_least "$TMPDIR_ROOT/setup-selfhost.json" "output.relay_request_timeout_seconds" "$required_timeout" "setup self-host output"
assert_json_number_at_least "$TMPDIR_ROOT/setup-relay.json" "output.proxy_timeout_seconds" "$required_timeout" "setup relay output"
require_help_flags "$TMPDIR_ROOT/setup-selfhost-help.txt" \
  "--gateway-url" "--relay-url" "--console-url" "--listen" "--token-env" "--token-file" \
  "--default-relay-token-env" "--default-relay-token-file" "--enable-oauth-dev" \
  "--oauth-client-secret" "--relay-request-timeout-seconds" "--json"
require_help_flags "$TMPDIR_ROOT/setup-relay-help.txt" \
  "--base-url" "--mcp-url" "--relay-config" "--connector-config" "--planner-token" \
  "--planner-token-env" "--generate-planner-token" "--proxy-timeout-seconds" \
  "--enable-chatgpt-oauth-dev" "--install-services" "--start-services" "--manager" \
  "--bin-dir" "--strict" "--json"

"$ROOT/bin/codencer" setup mcp --client codex --json > "$TMPDIR_ROOT/codex-mcp.json"
"$ROOT/bin/codencer" setup mcp --client claude-code --json > "$TMPDIR_ROOT/claude-mcp.json"
"$ROOT/bin/codencer" activation self-host \
  --gateway "$gateway_url" \
  --relay "$relay_url" \
  --project codencer \
  --token-env CODENCER_GATEWAY_MCP_TOKEN \
  --json > "$TMPDIR_ROOT/chatgpt-sheet.json"
"$ROOT/bin/codencer" activation self-host \
  --gateway "$gateway_url" \
  --relay "$relay_url" \
  --project codencer \
  --token-env CODENCER_GATEWAY_MCP_TOKEN > "$TMPDIR_ROOT/activation-selfhost-human.txt"
assert_no_default_output_leak "$TMPDIR_ROOT/activation-selfhost-human.txt" "codencer activation self-host human output"
chatgpt_package="$(json_get "$TMPDIR_ROOT/chatgpt-sheet.json" "package_path")"
chatgpt_sheet="$chatgpt_package/chatgpt-app-setup.md"
test -f "$chatgpt_sheet"
for file in "$TMPDIR_ROOT/codex-mcp.json" "$TMPDIR_ROOT/claude-mcp.json" "$TMPDIR_ROOT/chatgpt-sheet.json" "$chatgpt_sheet"; do
  assert_no_commercial_endpoint "$file"
  grep -q "$gateway_url/mcp" "$file" || { cat "$file" >&2; exit 1; }
done
grep -q 'codex' "$TMPDIR_ROOT/codex-mcp.json" || { cat "$TMPDIR_ROOT/codex-mcp.json" >&2; exit 1; }
grep -q 'claude mcp add --transport http' "$TMPDIR_ROOT/claude-mcp.json" || { cat "$TMPDIR_ROOT/claude-mcp.json" >&2; exit 1; }
grep -q 'ChatGPT' "$chatgpt_sheet" || { cat "$chatgpt_sheet" >&2; exit 1; }

gateway_config="$home/runtime/gateway/config.json"
relay_config="$home/runtime/relay/config.json"
test -f "$gateway_config"
test -f "$relay_config"
assert_no_commercial_endpoint "$gateway_config"
assert_no_commercial_endpoint "$relay_config"
if grep -q 'public-selfhost-client-secret\|gateway-token\|relay-token' "$gateway_config"; then
  echo "gateway config leaked token-like material" >&2
  cat "$gateway_config" >&2
  exit 1
fi
assert_json_number_at_least "$gateway_config" "relay_request_timeout_seconds" "$required_timeout" "gateway config"
assert_json_number_at_least "$relay_config" "proxy_timeout_seconds" "$required_timeout" "relay config"

echo "--- Public Self-Host Release Config Proof ---"
echo "Gateway default: http://127.0.0.1:19090"
echo "Gateway override: $gateway_url"
echo "Relay override:   $relay_url"
echo "Console override: $console_url"
echo "Gateway timeout:  $(json_get "$gateway_config" "relay_request_timeout_seconds")"
echo "Relay timeout:    $(json_get "$relay_config" "proxy_timeout_seconds")"
echo "Codex config:     $TMPDIR_ROOT/codex-mcp.json"
echo "Claude command:   $TMPDIR_ROOT/claude-mcp.json"
echo "ChatGPT sheet:    $chatgpt_sheet"
