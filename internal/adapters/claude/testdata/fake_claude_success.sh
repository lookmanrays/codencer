#!/bin/sh
set -eu

PROMPT=$(cat)
printf 'claude stderr ok\n' >&2
python3 - "$PWD" "$PROMPT" <<'PY'
import json
import sys

cwd = sys.argv[1]
prompt = sys.argv[2]
payload = {
    "type": "result",
    "subtype": "success",
    "is_error": False,
    "result": f"Completed in {cwd} with prompt: {prompt}",
}
print(json.dumps(payload))
PY
