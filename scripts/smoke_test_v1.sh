#!/usr/bin/env bash
set -e

# Codencer v1 Smoke Test (Simulation Mode)
# This script verifies all 6 primary submission modes against a running simulation daemon.

RUN_ID="smoke-test-$(date +%s)"
PROJECT="codencer-smoke"
ORCHESTRATORCTL="./bin/orchestratorctl"

echo "==> starting smoke test: $RUN_ID"

# 0. Discovery
$ORCHESTRATORCTL instance --json

# 1. Start a mission
$ORCHESTRATORCTL run start "$RUN_ID" --project "$PROJECT" --json

# 2. Test Format: Task File (YAML)
$ORCHESTRATORCTL submit "$RUN_ID" examples/tasks/bug_fix.yaml --wait --json

# 3. Test Format: Prompt File
cat > /tmp/smoke-prompt.md <<EOF
Refactor the internal/logger package to use the new standard.
EOF
$ORCHESTRATORCTL submit "$RUN_ID" --prompt-file /tmp/smoke-prompt.md --adapter codex --wait --json

# 4. Test Format: Goal (Inline)
$ORCHESTRATORCTL submit "$RUN_ID" --goal "Improve test coverage in pkg/util" --adapter codex --wait --json

# 5. Test Format: Stdin (Heredoc)
cat <<EOF | $ORCHESTRATORCTL submit "$RUN_ID" --stdin --title "Update README" --adapter codex --wait --json
Update the readme to mention the new features.
EOF

# 6. Test Format: Task JSON (Piped)
echo "{\"version\":\"v1\",\"goal\":\"Fix typo\",\"title\":\"Quick Fix\",\"adapter_profile\":\"codex\"}" | \
    $ORCHESTRATORCTL submit "$RUN_ID" --task-json - --wait --json

# 7. Audit specific step (assuming last one)
LAST_STEP=$($ORCHESTRATORCTL step list "$RUN_ID" --json | jq -r '.[-1].id')
echo "==> Auditing last step: $LAST_STEP"
$ORCHESTRATORCTL step result "$LAST_STEP" --json
$ORCHESTRATORCTL step logs "$LAST_STEP" --json
$ORCHESTRATORCTL step artifacts "$LAST_STEP" --json

echo "==> smoke test complete: SUCCESS"
