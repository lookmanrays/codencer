#!/usr/bin/env bash
set -euo pipefail

# Codencer v1 local parity smoke.
# Verifies all 6 primary submission modes against the local daemon path.
# If no daemon is already reachable, the script auto-starts a temporary
# simulation daemon in the same shell so submit --wait remains a safe barrier.

RUN_ID="smoke-test-$(date +%s)-$$"
PROJECT="codencer-smoke"
ORCHESTRATORCTL="${ORCHESTRATORCTL:-./bin/orchestratorctl}"
ORCHESTRATORD="${ORCHESTRATORD:-./bin/orchestratord}"
HOST="${HOST:-127.0.0.1}"
PORT="${PORT:-8085}"
BASE_URL="${ORCHESTRATORD_URL:-http://${HOST}:${PORT}}"
SMOKE_V1_AUTO_START="${SMOKE_V1_AUTO_START:-1}"
PROMPT_FILE="$(mktemp "${TMPDIR:-/tmp}/codencer-smoke-prompt.XXXXXX.md")"
DAEMON_ALREADY_RUNNING=0
DAEMON_PID=""

have_cmd() {
    command -v "$1" >/dev/null 2>&1
}

parse_last_step_id() {
    local path="$1"
    if have_cmd jq; then
        jq -r 'if type == "array" and length > 0 then .[-1].id // "" else "" end' "$path"
        return
    fi
    if have_cmd python3; then
        python3 - "$path" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as handle:
    payload = json.load(handle)
if isinstance(payload, list) and payload:
    item = payload[-1]
    if isinstance(item, dict):
        print(item.get("id", ""))
PY
        return
    fi
    echo "ERROR: smoke_test_v1.sh requires jq or python3 for JSON parsing." >&2
    exit 1
}

wait_for_daemon() {
    local attempts="${1:-15}"
    for _ in $(seq 1 "$attempts"); do
        if curl -fsS "${BASE_URL}/api/v1/compatibility" 2>/dev/null | grep -q '"tier"'; then
            return 0
        fi
        sleep 1
    done
    return 1
}

ensure_binaries() {
    if [[ -x "${ORCHESTRATORCTL}" && -x "${ORCHESTRATORD}" ]]; then
        return
    fi
    make setup build > /dev/null
}

start_daemon_if_needed() {
    if wait_for_daemon 2; then
        DAEMON_ALREADY_RUNNING=1
        return
    fi

    if [[ "${SMOKE_V1_AUTO_START}" != "1" ]]; then
        echo "ERROR: no local daemon reachable at ${BASE_URL} and SMOKE_V1_AUTO_START=0" >&2
        exit 1
    fi

    echo "==> starting temporary simulation daemon for local parity smoke"
    ensure_binaries
    mkdir -p .codencer
    ALL_ADAPTERS_SIMULATION_MODE=1 "${ORCHESTRATORD}" > .codencer/smoke_v1_daemon.log 2>&1 &
    DAEMON_PID="$!"
    DAEMON_ALREADY_RUNNING=0

    if ! wait_for_daemon 15; then
        echo "ERROR: daemon failed to start. Check .codencer/smoke_v1_daemon.log" >&2
        exit 1
    fi
}

cleanup() {
    rm -f "${PROMPT_FILE}"
    if [[ "${DAEMON_ALREADY_RUNNING}" == "0" && -n "${DAEMON_PID}" ]]; then
        kill "${DAEMON_PID}" 2>/dev/null || true
        wait "${DAEMON_PID}" 2>/dev/null || true
    fi
}
trap cleanup EXIT

echo "==> starting smoke test: ${RUN_ID}"
start_daemon_if_needed

# 0. Discovery
"${ORCHESTRATORCTL}" instance --json

# 1. Start a mission
"${ORCHESTRATORCTL}" run start "${RUN_ID}" --project "${PROJECT}" --json

# 2. Test Format: Task File (YAML)
"${ORCHESTRATORCTL}" submit "${RUN_ID}" examples/tasks/bug_fix.yaml --wait --json

# 3. Test Format: Prompt File
cat > "${PROMPT_FILE}" <<'EOF'
Refactor the internal/logger package to use the new standard.
EOF
"${ORCHESTRATORCTL}" submit "${RUN_ID}" --prompt-file "${PROMPT_FILE}" --adapter codex --wait --json

# 4. Test Format: Goal (Inline)
"${ORCHESTRATORCTL}" submit "${RUN_ID}" --goal "Improve test coverage in pkg/util" --adapter codex --wait --json

# 5. Test Format: Stdin (Heredoc)
cat <<'EOF' | "${ORCHESTRATORCTL}" submit "${RUN_ID}" --stdin --title "Update README" --adapter codex --wait --json
Update the readme to mention the new features.
EOF

# 6. Test Format: Task JSON (Piped)
echo '{"version":"v1","goal":"Fix typo","title":"Quick Fix","adapter_profile":"codex"}' | \
    "${ORCHESTRATORCTL}" submit "${RUN_ID}" --task-json - --wait --json

# 7. Audit the last step
STEP_LIST_FILE="$(mktemp "${TMPDIR:-/tmp}/codencer-smoke-steps.XXXXXX.json")"
"${ORCHESTRATORCTL}" step list "${RUN_ID}" --json > "${STEP_LIST_FILE}"
LAST_STEP="$(parse_last_step_id "${STEP_LIST_FILE}")"
rm -f "${STEP_LIST_FILE}"

if [[ -z "${LAST_STEP}" ]]; then
    echo "ERROR: unable to determine last step for ${RUN_ID}" >&2
    exit 1
fi

echo "==> Auditing last step: ${LAST_STEP}"
"${ORCHESTRATORCTL}" step result "${LAST_STEP}" --json
"${ORCHESTRATORCTL}" step logs "${LAST_STEP}" --json
"${ORCHESTRATORCTL}" step artifacts "${LAST_STEP}" --json
"${ORCHESTRATORCTL}" step validations "${LAST_STEP}" --json

echo "==> smoke test complete: SUCCESS"
