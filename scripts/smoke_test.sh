#!/usr/bin/env bash
# scripts/smoke_test.sh - Verify the local bridge relay loop.
set -e

# Default to simulation mode unless overridden
export ALL_ADAPTERS_SIMULATION_MODE=${ALL_ADAPTERS_SIMULATION_MODE:-1}
export SMOKE_PORT=${SMOKE_PORT:-8085}

echo "--- Codencer Smoke Test ---"
echo "Mode: $([ "$ALL_ADAPTERS_SIMULATION_MODE" == "1" ] && echo "Simulation" || echo "Real")"

# 1. Setup & Build
make setup build > /dev/null

# 2. Start Daemon
echo "Checking for daemon on port $SMOKE_PORT..."
if curl -s http://127.0.0.1:$SMOKE_PORT/health | grep -q "ok"; then
    echo "Daemon already running on $SMOKE_PORT. Using existing daemon."
    DAEMON_ALREADY_RUNNING=1
else
    echo "Starting daemon on port $SMOKE_PORT..."
    PORT=$SMOKE_PORT ./bin/orchestratord > .codencer/smoke_daemon.log 2>&1 &
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
    if curl -s http://127.0.0.1:$SMOKE_PORT/health | grep -q "ok"; then
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

echo "1. Starting run: $RUN_ID"
./bin/orchestratorctl run start "$RUN_ID" "$PROJECT" > /dev/null

echo "2. Submitting simulation task..."
# Provide a full TaskSpec for reliable parsing
cat <<EOF > .codencer/smoke_task.yaml
version: "1.1"
run_id: "$RUN_ID"
step_id: "smoke-step-1"
phase_id: "execution"
title: "Smoke Test Task"
goal: "Verify the bridge relay loop"
adapter_profile: "codex"
is_simulation: true
EOF

STEP_OUTPUT=$(./bin/orchestratorctl submit "$RUN_ID" .codencer/smoke_task.yaml)
# Robustly extract "id" using basic grep and tr
STEP_ID=$(echo "$STEP_OUTPUT" | grep '"id":' | cut -d'"' -f4)

echo "3. Waiting for terminal state (Step: $STEP_ID)..."
./bin/orchestratorctl step wait "$STEP_ID" --timeout 30s > .codencer/smoke_result.json

echo "--- RESULTS ---"
# Check if result is valid JSON and has a terminal state
STATE=$(grep -oP '"state":\s*"\K[^"]+' .codencer/smoke_result.json)
echo "Terminal State: $STATE"

if [[ "$STATE" == "completed" || "$STATE" == "completed_with_warnings" || "$STATE" == "failed_retryable" || "$STATE" == "failed_terminal" ]]; then
    echo "SUCCESS: Smoke test passed!"
else
    echo "FAILURE: Unexpected terminal state: $STATE"
    cat .codencer/smoke_result.json
    exit 1
fi
