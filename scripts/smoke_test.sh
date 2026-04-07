#!/usr/bin/env bash
# scripts/smoke_test.sh - Verify the local bridge relay loop.
set -e

# Default to simulation mode unless overridden
export ALL_ADAPTERS_SIMULATION_MODE=${ALL_ADAPTERS_SIMULATION_MODE:-1}
export HOST=${HOST:-127.0.0.1}
export PORT=${PORT:-8085}
export SMOKE_INPUT_MODE=${SMOKE_INPUT_MODE:-file}

have_cmd() {
    command -v "$1" >/dev/null 2>&1
}

parse_json_file_field() {
    local path="$1"
    local field="$2"
    if have_cmd jq; then
        jq -r --arg field "$field" '.[$field] // empty' "$path"
        return
    fi
    if have_cmd python3; then
        python3 - "$path" "$field" <<'PY'
import json
import sys

path, field = sys.argv[1], sys.argv[2]
with open(path, "r", encoding="utf-8") as handle:
    payload = json.load(handle)
value = payload.get(field, "")
if value is None:
    value = ""
print(value)
PY
        return
    fi
    echo "ERROR: smoke_test.sh requires jq or python3 for JSON parsing." >&2
    exit 1
}

parse_last_step_id_file() {
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
    echo "ERROR: smoke_test.sh requires jq or python3 for JSON parsing." >&2
    exit 1
}

echo "--- Codencer Smoke Test ---"
echo "Mode: $([ "$ALL_ADAPTERS_SIMULATION_MODE" == "1" ] && echo "Simulation" || echo "Real")"

# 1. Setup & Build
make setup build > /dev/null

# 2. Start Daemon
echo "Checking for daemon on $HOST:$PORT..."
if curl -s http://$HOST:$PORT/api/v1/compatibility | grep -q '"tier"'; then
    echo "Daemon already running on $HOST:$PORT. Using existing daemon."
    DAEMON_ALREADY_RUNNING=1
else
    echo "Starting daemon on $HOST:$PORT..."
    ./bin/orchestratord > .codencer/smoke_daemon.log 2>&1 &
    DAEMON_PID=$!
    DAEMON_ALREADY_RUNNING=0
fi

function cleanup {
    if [ "$DAEMON_ALREADY_RUNNING" == "0" ]; then
        echo "Stopping daemon (PID: $DAEMON_PID)..."
        kill $DAEMON_PID || true
    fi
}
trap cleanup EXIT

# Wait for health check
echo "Waiting for daemon to be ready..."
for i in {1..10}; do
    if curl -s http://$HOST:$PORT/api/v1/compatibility | grep -q '"tier"'; then
        echo "Daemon ready."
        break
    fi
    if [ $i -eq 10 ]; then
        echo "ERROR: Daemon failed to start. Check .codencer/smoke_daemon.log"
        exit 1
    fi
    sleep 1
done

# 3. Execute Relay Loop
RUN_ID="smoke-run-$(date +%s)"
PROJECT="smoke-project"

echo "1. Starting run mission: $RUN_ID"
./bin/orchestratorctl run start "$RUN_ID" "$PROJECT" > /dev/null

echo "2. Submitting task (Flow: submit --wait, input mode: $SMOKE_INPUT_MODE)..."
cat <<EOF > .codencer/smoke_task.yaml
version: "v1"
run_id: "$RUN_ID"
title: "Smoke Test Validation"
goal: "Verify the bridge relay loop"
adapter_profile: "codex"
is_simulation: true
EOF

if [[ "$SMOKE_INPUT_MODE" == "goal" ]]; then
    ./bin/orchestratorctl submit "$RUN_ID" \
        --goal "Verify the bridge relay loop" \
        --title "Smoke Test Validation" \
        --adapter codex \
        --wait --json > .codencer/smoke_result.json
elif [[ "$SMOKE_INPUT_MODE" == "stdin" ]]; then
    cat <<'EOF' | ./bin/orchestratorctl submit "$RUN_ID" --stdin --title "Smoke Test Validation" --adapter codex --wait --json > .codencer/smoke_result.json
Verify the bridge relay loop via stdin.
EOF
elif [[ "$SMOKE_INPUT_MODE" == "task-json" ]]; then
    echo '{"version":"v1","goal":"Verify the bridge relay loop via task-json","title":"Smoke Test Validation","adapter_profile":"codex"}' | \
        ./bin/orchestratorctl submit "$RUN_ID" --task-json - --wait --json > .codencer/smoke_result.json
else
    ./bin/orchestratorctl submit "$RUN_ID" .codencer/smoke_task.yaml --wait --json > .codencer/smoke_result.json
fi

# Extract terminal state and step ID from the single ResultSpec payload.
# If jq or python isn't available, we'll try to find it via grep or similar, 
# but the parse_json_file_field should handle it.
STATE=$(parse_json_file_field .codencer/smoke_result.json state)
STEP_ID=$(parse_json_file_field .codencer/smoke_result.json id)
if [[ -z "$STEP_ID" ]]; then
    ./bin/orchestratorctl step list "$RUN_ID" --json > .codencer/smoke_steps.json
    STEP_ID=$(parse_last_step_id_file .codencer/smoke_steps.json)
fi

echo "--- SMOKE TEST SUMMARY ---"
echo "UUID Handle:    $STEP_ID"
echo "Terminal State: $STATE"

if [[ "$STATE" == "completed" || "$STATE" == "completed_with_warnings" || "$STATE" == "failed_terminal" ]]; then
    echo "SUCCESS: Bridge relay loop verified."
    echo ""
    echo "Authoritative Result Summary:"
    ./bin/orchestratorctl step result "$STEP_ID"
else
    echo "FAILURE: Unexpected state reached."
    cat .codencer/smoke_result.json
    exit 1
fi
