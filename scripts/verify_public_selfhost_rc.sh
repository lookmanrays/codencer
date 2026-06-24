#!/usr/bin/env bash
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${VERSION:-v0.3.0-public-selfhost-rc-verify}"
TARGETS="${TARGETS:-host}"
REQUIRE_TARGETS="${REQUIRE_TARGETS:-host}"
TMPDIR_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/codencer-public-selfhost-rc.XXXXXX")"
REPORT_ROOT="${REPORT_ROOT:-$ROOT/reports/public-selfhost-rc}"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
REPORT_DIR="$REPORT_ROOT/$TIMESTAMP"
DIST="$TMPDIR_ROOT/dist"
GATES_FILE="$REPORT_DIR/gates.jsonl"
FAILURES=0
BIN_DIR=""
ARTIFACT_NAME=""
ARTIFACT_PATH=""
ARTIFACT_SHA=""

mkdir -p "$REPORT_DIR"

cleanup() {
  rm -rf "$TMPDIR_ROOT"
}
trap cleanup EXIT

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

free_port() {
  python3 - <<'PY'
import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()
PY
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
      return 1
    fi
  done
}

write_gate() {
  local name="$1"
  local status="$2"
  local detail="$3"
  local log="$4"
  python3 - "$GATES_FILE" "$name" "$status" "$detail" "$log" <<'PY'
import json, sys
path,name,status,detail,log=sys.argv[1:]
with open(path, "a", encoding="utf-8") as fh:
  fh.write(json.dumps({"name": name, "status": status, "detail": detail, "log": log}, sort_keys=True) + "\n")
PY
}

standard_setup_contract() {
  if [ -z "$BIN_DIR" ]; then
    echo "BIN_DIR is not set" >&2
    return 1
  fi
  local home gateway_port relay_port console_port gateway_url relay_url console_url required
  home="$TMPDIR_ROOT/standard-setup-home"
  gateway_port="$(free_port)" || return 1
  relay_port="$(free_port)" || return 1
  console_port="$(free_port)" || return 1
  gateway_url="http://127.0.0.1:$gateway_port"
  relay_url="http://127.0.0.1:$relay_port"
  console_url="http://127.0.0.1:$console_port"
  required="$(required_timeout_seconds)" || return 1
  echo "Standard setup contract ports: gateway=$gateway_port relay=$relay_port console=$console_port"
  echo "Standard setup timeout requirement: >=${required}s"

  CODENCER_HOME="$home" "$BIN_DIR/codencer" init --json > "$TMPDIR_ROOT/standard-init.json" || return 1
  CODENCER_HOME="$home" "$BIN_DIR/codencer" setup self-host \
    --gateway-url "$gateway_url" \
    --mcp-url "$gateway_url/mcp" \
    --relay-url "$relay_url" \
    --console-url "$console_url" \
    --listen "127.0.0.1:$gateway_port" \
    --token-env CODENCER_GATEWAY_MCP_TOKEN \
    --default-relay-token-env CODENCER_DEFAULT_RELAY_TOKEN \
    --json > "$TMPDIR_ROOT/standard-setup-selfhost.json" || return 1
  CODENCER_HOME="$home" "$BIN_DIR/codencer" setup relay \
    --base-url "$relay_url" \
    --mcp-url "$relay_url/mcp" \
    --generate-planner-token \
    --json > "$TMPDIR_ROOT/standard-setup-relay.json" || return 1

  CODENCER_HOME="$home" "$BIN_DIR/codencer" setup self-host --help > "$TMPDIR_ROOT/standard-help-selfhost.txt" || return 1
  CODENCER_HOME="$home" "$BIN_DIR/codencer" setup relay --help > "$TMPDIR_ROOT/standard-help-relay.txt" || return 1

  assert_json_number_at_least "$home/runtime/gateway/config.json" "relay_request_timeout_seconds" "$required" "gateway config" || return 1
  assert_json_number_at_least "$home/runtime/relay/config.json" "proxy_timeout_seconds" "$required" "relay config" || return 1
  assert_json_number_at_least "$TMPDIR_ROOT/standard-setup-selfhost.json" "output.relay_request_timeout_seconds" "$required" "setup self-host output" || return 1
  assert_json_number_at_least "$TMPDIR_ROOT/standard-setup-relay.json" "output.proxy_timeout_seconds" "$required" "setup relay output" || return 1

  require_help_flags "$TMPDIR_ROOT/standard-help-selfhost.txt" \
    "--gateway-url" "--relay-url" "--console-url" "--listen" "--token-env" "--token-file" \
    "--default-relay-token-env" "--default-relay-token-file" "--enable-oauth-dev" \
    "--oauth-client-secret" "--relay-request-timeout-seconds" "--json" || return 1
  require_help_flags "$TMPDIR_ROOT/standard-help-relay.txt" \
    "--base-url" "--mcp-url" "--relay-config" "--connector-config" "--planner-token" \
    "--planner-token-env" "--generate-planner-token" "--proxy-timeout-seconds" \
    "--enable-chatgpt-oauth-dev" "--install-services" "--start-services" "--manager" \
    "--bin-dir" "--strict" "--json" || return 1

  cp "$home/runtime/gateway/config.json" "$REPORT_DIR/standard-gateway-config.json" || return 1
  cp "$home/runtime/relay/config.json" "$REPORT_DIR/standard-relay-config.json" || return 1
  cp "$TMPDIR_ROOT/standard-help-selfhost.txt" "$REPORT_DIR/standard-setup-selfhost-help.txt" || return 1
  cp "$TMPDIR_ROOT/standard-help-relay.txt" "$REPORT_DIR/standard-setup-relay-help.txt" || return 1
}

reject_real_executor_simulation_env() {
  local adapter="$1"
  local log="$2"
  local bad=0
  local adapter_env
  adapter_env="$(printf '%s_SIMULATION_MODE' "$adapter" | tr '[:lower:]-' '[:upper:]_')"
  {
    echo "Real executor simulation environment preflight:"
    for name in ALL_ADAPTERS_SIMULATION_MODE CODEX_SIMULATION_MODE "$adapter_env"; do
      local value=""
      if value="$(printenv "$name")"; then
        echo "$name=$value"
      else
        echo "$name=<unset>"
      fi
      case "$(printf '%s' "$value" | tr '[:upper:]' '[:lower:]')" in
        1|true)
          bad=1
          ;;
      esac
    done
  } > "$log"
  if [ "$bad" -ne 0 ]; then
    cat "$log" >&2
    return 1
  fi
}

required_real_executors() {
  python3 - <<'PY'
import os
raw=os.environ.get("CODENCER_E2E_REQUIRED_REAL_EXECUTORS", "codex,claude,antigravity")
items=[item.strip() for item in raw.split(",") if item.strip()]
for item in items:
  print(item)
PY
}

configured_real_executors() {
  python3 - <<'PY'
import os
raw=os.environ.get("CODENCER_E2E_REAL_EXECUTORS", "").strip()
if not raw:
  raw=os.environ.get("CODENCER_E2E_REAL_EXECUTOR", "").strip()
items=[item.strip() for item in raw.split(",") if item.strip()]
for item in items:
  print(item)
PY
}

executor_profile_for() {
  local adapter="$1"
  local adapter_key
  adapter_key="$(printf '%s' "$adapter" | tr '[:lower:]-' '[:upper:]_')"
  local specific="CODENCER_E2E_${adapter_key}_PROFILE"
  local value="${!specific:-${CODENCER_E2E_REAL_EXECUTOR_PROFILE:-}}"
  if [ -n "$value" ]; then
    echo "$value"
    return
  fi
  case "$adapter" in
    codex) echo "codex-workspace" ;;
    claude) echo "claude-default" ;;
    antigravity) echo "antigravity-default" ;;
    *) echo "$adapter" ;;
  esac
}

executor_command_for() {
  local adapter="$1"
  local adapter_key
  adapter_key="$(printf '%s' "$adapter" | tr '[:lower:]-' '[:upper:]_')"
  local specific="CODENCER_E2E_${adapter_key}_COMMAND"
  local value="${!specific:-}"
  if [ -n "$value" ]; then
    echo "$value"
    return
  fi
  case "$adapter" in
    codex) echo "${CODEX_BINARY:-${CODENCER_E2E_REAL_EXECUTOR_COMMAND:-codex}}" ;;
    claude) echo "${CLAUDE_BINARY:-${CODENCER_E2E_REAL_EXECUTOR_COMMAND:-claude}}" ;;
    antigravity) echo "${CODENCER_E2E_REAL_EXECUTOR_COMMAND:-}" ;;
    *) echo "${CODENCER_E2E_REAL_EXECUTOR_COMMAND:-$adapter}" ;;
  esac
}

assert_required_real_executor_coverage() {
  local proven="$1"
  local log="$REPORT_DIR/required_real_executor_proofs.log"
  local missing=0
  {
    echo "Required real executor proofs:"
    echo "proven=${proven:-<none>}"
    while IFS= read -r required; do
      [ -n "$required" ] || continue
      if printf '%s\n' "$proven" | tr ',' '\n' | grep -qx "$required"; then
        echo "$required=passed"
      else
        echo "$required=missing"
        missing=1
      fi
    done < <(required_real_executors)
  } > "$log"
  if [ "$missing" -ne 0 ]; then
    cat "$log" >&2
    write_gate required_real_executor_proofs "failed" "missing required real executor proof" "$log"
    FAILURES=1
    return 1
  fi
  write_gate required_real_executor_proofs "passed" "all required real executor proofs passed" "$log"
}

run_gate() {
  local name="$1"
  shift
  local log="$REPORT_DIR/$name.log"
  echo "==> $name"
  "$@" >"$log" 2>&1
  local code="$?"
  if [ "$code" -eq 0 ]; then
    write_gate "$name" "passed" "ok" "$log"
    return 0
  fi
  write_gate "$name" "failed" "exit_code=$code" "$log"
  FAILURES=1
  echo "Gate failed: $name (log: $log)" >&2
  tail -n 80 "$log" >&2 || true
  return "$code"
}

select_and_unpack_artifact() {
  local selection_json="$TMPDIR_ROOT/selection.json"
  local host_os host_arch
  host_os="$(go env GOOS)"
  host_arch="$(go env GOARCH)"
  python3 - "$DIST/manifest.json" "$DIST/checksums.txt" "$DIST" "$host_os" "$host_arch" > "$selection_json" <<'PY'
import hashlib, json, sys
from pathlib import Path

manifest_path=Path(sys.argv[1])
checksums_path=Path(sys.argv[2])
dist=Path(sys.argv[3])
host_os=sys.argv[4]
host_arch=sys.argv[5]
manifest=json.loads(manifest_path.read_text())
checksums={}
for line in checksums_path.read_text().splitlines():
  parts=line.split()
  if len(parts) >= 2:
    checksums[parts[1]]=parts[0]
matches=[
  artifact for artifact in manifest.get("artifacts", [])
  if artifact.get("os") == host_os
  and artifact.get("arch") == host_arch
  and artifact.get("status") == "built"
]
if len(matches) != 1:
  raise SystemExit(f"expected exactly one built host artifact for {host_os}/{host_arch}, got {len(matches)}")
artifact=matches[0]
name=artifact.get("name", "")
archive=dist / name
if not archive.is_file():
  raise SystemExit(f"selected artifact missing on disk: {archive}")
manifest_sha=artifact.get("sha256", "")
checksum_sha=checksums.get(name, "")
if not manifest_sha or manifest_sha != checksum_sha:
  raise SystemExit(f"{name}: manifest/checksum mismatch")
actual=hashlib.sha256(archive.read_bytes()).hexdigest()
if actual != manifest_sha:
  raise SystemExit(f"{name}: file sha256 mismatch")
print(json.dumps({"artifact_name": name, "artifact_path": str(archive), "sha256": actual}))
PY
  ARTIFACT_NAME="$(json_get "$selection_json" artifact_name)"
  ARTIFACT_PATH="$(json_get "$selection_json" artifact_path)"
  ARTIFACT_SHA="$(json_get "$selection_json" sha256)"

  local unpack_dir="$TMPDIR_ROOT/unpacked"
  mkdir -p "$unpack_dir"
  case "$ARTIFACT_PATH" in
    *.tar.gz) tar -xzf "$ARTIFACT_PATH" -C "$unpack_dir" ;;
    *.zip) python3 - "$ARTIFACT_PATH" "$unpack_dir" <<'PY'
import sys, zipfile
with zipfile.ZipFile(sys.argv[1]) as archive:
  archive.extractall(sys.argv[2])
PY
      ;;
    *) echo "unsupported release archive type: $ARTIFACT_PATH" >&2; return 1 ;;
  esac

  local bin_dirs_file="$TMPDIR_ROOT/bin-dirs.txt"
  find "$unpack_dir" -type d -name bin -print > "$bin_dirs_file"
  local bin_dir_count
  bin_dir_count="$(wc -l < "$bin_dirs_file" | tr -d ' ')"
  if [ "$bin_dir_count" != "1" ]; then
    echo "expected exactly one unpacked bin directory, got $bin_dir_count" >&2
    cat "$bin_dirs_file" >&2
    return 1
  fi
  BIN_DIR="$(cat "$bin_dirs_file")"
  case "$BIN_DIR" in
    "$ROOT/bin"|"$ROOT/bin/"*) echo "artifact proof resolved to source-tree bin directory: $BIN_DIR" >&2; return 1 ;;
  esac
  for binary in codencer orchestratord codencer-relayd codencer-gatewayd codencer-connectord; do
    if [ ! -x "$BIN_DIR/$binary" ]; then
      echo "required unpacked binary missing or not executable: $BIN_DIR/$binary" >&2
      return 1
    fi
  done
}

write_summary() {
  local verdict="$1"
  local reason="$2"
  local json="$REPORT_DIR/summary.json"
  local md="$REPORT_DIR/summary.md"
  python3 - "$GATES_FILE" "$json" "$md" "$verdict" "$reason" "$VERSION" "$ARTIFACT_NAME" "$ARTIFACT_PATH" "$ARTIFACT_SHA" "$BIN_DIR" <<'PY'
import json, sys
from pathlib import Path

gates_path, json_path, md_path, verdict, reason, version, artifact_name, artifact_path, artifact_sha, bin_dir = sys.argv[1:]
gates=[]
if Path(gates_path).exists():
  gates=[json.loads(line) for line in Path(gates_path).read_text().splitlines() if line.strip()]
payload={
  "verdict": verdict,
  "reason": reason,
  "version": version,
  "artifact": {
    "name": artifact_name,
    "path": artifact_path,
    "sha256": artifact_sha,
    "bin_dir": bin_dir,
  },
  "gates": gates,
}
Path(json_path).write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")
lines=[
  "# Public Self-host RC Verification",
  "",
  f"- Verdict: {verdict}",
  f"- Reason: {reason}",
  f"- Version: {version}",
  f"- Artifact: {artifact_name or 'n/a'}",
  f"- Artifact SHA256: {artifact_sha or 'n/a'}",
  f"- Binary directory: {bin_dir or 'n/a'}",
  "",
  "## Gates",
]
for gate in gates:
  lines.append(f"- {gate['status'].upper()}: {gate['name']} ({gate['detail']})")
Path(md_path).write_text("\n".join(lines) + "\n", encoding="utf-8")
PY
}

echo "Public self-host RC report directory: $REPORT_DIR"

run_gate build_release_snapshot make -C "$ROOT" release-snapshot VERSION="$VERSION" TARGETS="$TARGETS" REQUIRE_TARGETS="$REQUIRE_TARGETS" DIST="$DIST" || true
if [ "$FAILURES" -eq 0 ]; then
  run_gate select_unpack_artifact select_and_unpack_artifact || true
fi
if [ "$FAILURES" -eq 0 ]; then
  run_gate docs_public_release bash -c "cd '$ROOT' && python3 scripts/check_docs_links.py && python3 scripts/check_public_boundary.py" || true
fi
if [ "$FAILURES" -eq 0 ]; then
  run_gate standard_setup_contract standard_setup_contract || true
fi
if [ "$FAILURES" -eq 0 ]; then
  manifest_file="$TMPDIR_ROOT/fake-success.yaml"
  cat > "$manifest_file" <<'YAML'
version: codencer.io/v1alpha1
kind: RunManifest
metadata:
  name: fake-success
project:
  id: codencer
execution:
  adapter: fake
  profile: fake-success
policy:
  stop_on_blocker: true
  stop_on_failure: true
tasks:
  - id: fake-success
    title: Fake success
    goal: Complete a deterministic fake success task.
YAML
  run_gate gateway_artifact_smoke env CODENCER_BIN_DIR="$BIN_DIR" CODENCER_MANIFEST_FILE="$manifest_file" "$ROOT/scripts/verify_gateway.sh" || true
fi
if [ "$FAILURES" -eq 0 ]; then
  run_gate gateway_console_live_artifact bash -c "cd '$ROOT/web/gateway-console' && npm ci && npm run build && CODENCER_E2E_BIN_DIR='$BIN_DIR' node tests/live/verify-live.mjs" || true
fi

real_status="skipped"
real_reason="set CODENCER_E2E_REAL_EXECUTORS=codex,claude,antigravity and per-executor commands to run real executor gates"
proven_real_executor=""
if [ "$FAILURES" -eq 0 ]; then
  configured_file="$TMPDIR_ROOT/configured-real-executors.txt"
  configured_real_executors > "$configured_file"
  if [ ! -s "$configured_file" ]; then
    write_gate real_executor_e2e "failed" "$real_reason" ""
    FAILURES=1
  else
    real_status="configured"
    while IFS= read -r real_adapter; do
      [ -n "$real_adapter" ] || continue
      real_profile="$(executor_profile_for "$real_adapter")"
      real_command="$(executor_command_for "$real_adapter")"
      gate_name="real_executor_e2e_${real_adapter//-/_}"
      real_env_log="$REPORT_DIR/${gate_name}_env.log"
      if [ -n "$real_command" ] && ! command -v "$real_command" >/dev/null 2>&1; then
        real_status="failed"
        real_reason="executor command $real_command was not found on PATH"
        write_gate "$gate_name" "failed" "$real_reason" ""
        FAILURES=1
        continue
      fi
      if ! reject_real_executor_simulation_env "$real_adapter" "$real_env_log"; then
        real_status="failed"
        real_reason="real executor gate refused simulation environment values"
        write_gate "$gate_name" "failed" "$real_reason" "$real_env_log"
        FAILURES=1
        continue
      fi
      gate_passed=0
      if [ "$real_adapter" = "codex" ]; then
        if run_gate "$gate_name" env ALL_ADAPTERS_SIMULATION_MODE=0 CODEX_SIMULATION_MODE=0 CODEX_BINARY="$real_command" CODENCER_E2E_REAL_EXECUTOR_COMMAND="$real_command" CODENCER_E2E_BIN_DIR="$BIN_DIR" CODENCER_E2E_EXECUTOR_ADAPTER="$real_adapter" CODENCER_E2E_EXECUTOR_PROFILE="$real_profile" bash -c "cd '$ROOT/web/gateway-console' && node tests/live/verify-live.mjs"; then
          gate_passed=1
        fi
      elif [ "$real_adapter" = "claude" ]; then
        if run_gate "$gate_name" env ALL_ADAPTERS_SIMULATION_MODE=0 CLAUDE_SIMULATION_MODE=0 CLAUDE_BINARY="$real_command" CODENCER_E2E_REAL_EXECUTOR_COMMAND="$real_command" CODENCER_E2E_BIN_DIR="$BIN_DIR" CODENCER_E2E_EXECUTOR_ADAPTER="$real_adapter" CODENCER_E2E_EXECUTOR_PROFILE="$real_profile" bash -c "cd '$ROOT/web/gateway-console' && node tests/live/verify-live.mjs"; then
          gate_passed=1
        fi
      else
        adapter_env_name="$(printf '%s_SIMULATION_MODE' "$real_adapter" | tr '[:lower:]-' '[:upper:]_')"
        if run_gate "$gate_name" env ALL_ADAPTERS_SIMULATION_MODE=0 "$adapter_env_name=0" CODENCER_E2E_REAL_EXECUTOR_COMMAND="$real_command" CODENCER_E2E_BIN_DIR="$BIN_DIR" CODENCER_E2E_EXECUTOR_ADAPTER="$real_adapter" CODENCER_E2E_EXECUTOR_PROFILE="$real_profile" bash -c "cd '$ROOT/web/gateway-console' && node tests/live/verify-live.mjs"; then
          gate_passed=1
        fi
      fi
      if [ "$gate_passed" -eq 1 ]; then
        if [ -n "$proven_real_executor" ]; then
          proven_real_executor="$proven_real_executor,$real_adapter"
        else
          proven_real_executor="$real_adapter"
        fi
      fi
    done < "$configured_file"
    if [ -n "$proven_real_executor" ]; then
      real_status="passed"
      real_reason="real executor proofs passed for $proven_real_executor"
    fi
  fi
fi
assert_required_real_executor_coverage "$proven_real_executor" || true

if [ "$FAILURES" -ne 0 ]; then
  write_summary "NO-GO" "one or more deterministic release-candidate gates failed"
  echo "NO-GO for Public Self-host RC"
  echo "Report: $REPORT_DIR/summary.md"
  exit 1
fi

if [ "$real_status" = "passed" ]; then
  write_summary "GO" "$real_reason"
  echo "GO for Public Self-host RC"
else
  write_summary "NO-GO" "$real_reason"
  echo "NO-GO for Public Self-host RC"
  exit 1
fi
echo "Report: $REPORT_DIR/summary.md"
