#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMPDIR_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/codencer-public-selfhost.XXXXXX")"

cleanup() {
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

home="$TMPDIR_ROOT/home"
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
"$ROOT/bin/codencer" init --json > "$TMPDIR_ROOT/init.json"
"$ROOT/bin/codencer" config show --json > "$TMPDIR_ROOT/config-default.json"
assert_no_commercial_endpoint "$TMPDIR_ROOT/config-default.json"
test "$(json_get "$TMPDIR_ROOT/config-default.json" "resolved_connection.gateway_url")" = "http://127.0.0.1:19090"
test "$(json_get "$TMPDIR_ROOT/config-default.json" "resolved_connection.mcp_url")" = "http://127.0.0.1:19090/mcp"
test "$(json_get "$TMPDIR_ROOT/config-default.json" "resolved_connection.relay_url")" = "http://127.0.0.1:8090"
test "$(json_get "$TMPDIR_ROOT/config-default.json" "config.active_profile")" = "self-host"

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
