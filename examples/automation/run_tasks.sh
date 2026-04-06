#!/usr/bin/env bash
set -u -o pipefail

ORCHESTRATORCTL="${ORCHESTRATORCTL:-./bin/orchestratorctl}"
RUN_ID=""
PROJECT=""
INPUT_MODE=""
TASKS_FILE=""
TITLE_PREFIX="Task"
ADAPTER=""
TIMEOUT_SECONDS=""
POLICY=""
CONTINUE_ON_FAILURE=0
VALIDATIONS=()
ACCEPTANCES=()
JSON_TOOL=""
TITLE_PREFIX_EXPLICIT=0

usage() {
  cat >&2 <<'EOF'
Usage: examples/automation/run_tasks.sh --run-id <run-id> --input-mode <task-file|task-json|prompt-file|goal> --tasks-file <path> [options]

Options:
  --project <project>            Create the run if it does not already exist.
  --continue-on-failure          Continue after non-zero task exits.
  --title-prefix <prefix>        Goal mode only. Default: "Task".
  --adapter <adapter>            Direct modes only.
  --timeout <seconds>            Direct modes only.
  --policy <policy>              Direct modes only.
  --validation <command>         Direct modes only. Repeatable.
  --acceptance <text>            Direct modes only. Repeatable.
  --orchestratorctl <path>       CLI path. Default: ./bin/orchestratorctl

Environment:
  CODENCER_CONTINUE_ON_FAILURE=1 enables continue mode unless the wrapper exits earlier due to invalid usage.
EOF
}

die() {
  printf '%s\n' "$1" >&2
  exit "${2:-1}"
}

require_value() {
  local flag="$1"
  local value="${2:-}"
  if [[ -z "$value" ]]; then
    usage
    die "$flag requires a value." 1
  fi
}

have_cmd() {
  command -v "$1" >/dev/null 2>&1
}

detect_json_tool() {
  if have_cmd jq; then
    JSON_TOOL="jq"
    return
  fi
  if have_cmd python3; then
    JSON_TOOL="python3"
    return
  fi
  die "examples/automation/run_tasks.sh requires jq or python3 for JSON parsing." 1
}

json_get_field() {
  local json_text="$1"
  local field="$2"
  if [[ "$JSON_TOOL" == "jq" ]]; then
    printf '%s\n' "$json_text" | jq -r --arg field "$field" '.[$field] // empty'
    return
  fi
  python3 - "$field" <<'PY' <<<"$json_text"
import json
import sys

field = sys.argv[1]
text = sys.stdin.read()
try:
    value = json.loads(text)
except json.JSONDecodeError:
    sys.exit(0)
if isinstance(value, dict):
    result = value.get(field, "")
    if result is None:
        result = ""
    print(result)
PY
}

json_get_last_step_id() {
  local json_text="$1"
  if [[ "$JSON_TOOL" == "jq" ]]; then
    printf '%s\n' "$json_text" | jq -r 'if type == "array" and length > 0 then .[-1].id // "" else "" end'
    return
  fi
  python3 <<'PY' <<<"$json_text"
import json
import sys

text = sys.stdin.read()
try:
    value = json.loads(text)
except json.JSONDecodeError:
    sys.exit(0)
if isinstance(value, list) and value:
    item = value[-1]
    if isinstance(item, dict):
        print(item.get("id", ""))
PY
}

append_result() {
  local index="$1"
  local source="$2"
  local step_id="$3"
  local state="$4"
  local exit_code="$5"

  if [[ "$JSON_TOOL" == "jq" ]]; then
    jq -cn \
      --argjson index "$index" \
      --arg source "$source" \
      --arg step_id "$step_id" \
      --arg state "$state" \
      --argjson exit_code "$exit_code" \
      '{index:$index,source:$source,step_id:$step_id,state:$state,exit_code:$exit_code}' >>"$RESULTS_FILE"
    return
  fi

  python3 - "$index" "$source" "$step_id" "$state" "$exit_code" >>"$RESULTS_FILE" <<'PY'
import json
import sys

print(json.dumps({
    "index": int(sys.argv[1]),
    "source": sys.argv[2],
    "step_id": sys.argv[3],
    "state": sys.argv[4],
    "exit_code": int(sys.argv[5]),
}))
PY
}

emit_summary() {
  local final_exit_code="$1"
  local continue_json="false"
  if [[ "$CONTINUE_ON_FAILURE" -eq 1 ]]; then
    continue_json="true"
  fi

  if [[ "$JSON_TOOL" == "jq" ]]; then
    jq -sc \
      --arg run_id "$RUN_ID" \
      --arg project "$PROJECT" \
      --arg input_mode "$INPUT_MODE" \
      --argjson continue_on_failure "$continue_json" \
      --argjson tasks_total "$TASKS_TOTAL" \
      --argjson tasks_succeeded "$TASKS_SUCCEEDED" \
      --argjson tasks_failed "$TASKS_FAILED" \
      --argjson final_exit_code "$final_exit_code" \
      '{
        run_id:$run_id,
        project:$project,
        input_mode:$input_mode,
        continue_on_failure:$continue_on_failure,
        tasks_total:$tasks_total,
        tasks_succeeded:$tasks_succeeded,
        tasks_failed:$tasks_failed,
        results:.,
        final_exit_code:$final_exit_code
      }' "$RESULTS_FILE"
    return
  fi

  python3 - "$RESULTS_FILE" "$RUN_ID" "$PROJECT" "$INPUT_MODE" "$continue_json" "$TASKS_TOTAL" "$TASKS_SUCCEEDED" "$TASKS_FAILED" "$final_exit_code" <<'PY'
import json
import sys

results_path, run_id, project, input_mode, continue_on_failure, tasks_total, tasks_succeeded, tasks_failed, final_exit_code = sys.argv[1:]
results = []
with open(results_path, "r", encoding="utf-8") as handle:
    for line in handle:
        line = line.strip()
        if line:
            results.append(json.loads(line))
print(json.dumps({
    "run_id": run_id,
    "project": project,
    "input_mode": input_mode,
    "continue_on_failure": continue_on_failure == "true",
    "tasks_total": int(tasks_total),
    "tasks_succeeded": int(tasks_succeeded),
    "tasks_failed": int(tasks_failed),
    "results": results,
    "final_exit_code": int(final_exit_code),
}, indent=2))
PY
}

ensure_run() {
  local output
  output="$("$ORCHESTRATORCTL" run state "$RUN_ID" --json)"
  local exit_code=$?
  if [[ "$exit_code" -eq 0 ]]; then
    if [[ -z "$PROJECT" ]]; then
      PROJECT="$(json_get_field "$output" "project_id")"
    fi
    printf 'Reusing run %s\n' "$RUN_ID" >&2
    return
  fi

  local status
  status="$(json_get_field "$output" "status")"
  if [[ "$exit_code" -eq 1 && "$status" == "404" ]]; then
    if [[ -z "$PROJECT" ]]; then
      die "Run $RUN_ID does not exist. Re-run with --project <project> so the wrapper can create it." 1
    fi
    printf 'Creating run %s for project %s\n' "$RUN_ID" "$PROJECT" >&2
    output="$("$ORCHESTRATORCTL" run start "$RUN_ID" --project "$PROJECT" --json)"
    exit_code=$?
    if [[ "$exit_code" -ne 0 ]]; then
      printf '%s\n' "$output" >&2
      die "Failed to create run $RUN_ID." "$exit_code"
    fi
    return
  fi

  printf '%s\n' "$output" >&2
  die "Failed to query run $RUN_ID." "$exit_code"
}

latest_step_id() {
  local output
  output="$("$ORCHESTRATORCTL" step list "$RUN_ID" --json)"
  local exit_code=$?
  if [[ "$exit_code" -ne 0 ]]; then
    printf '%s\n' "$output" >&2
    return 1
  fi
  json_get_last_step_id "$output"
}

load_tasks() {
  TASK_ITEMS=()
  while IFS= read -r raw_line || [[ -n "$raw_line" ]]; do
    raw_line="${raw_line%$'\r'}"
    if [[ "$raw_line" =~ ^[[:space:]]*$ ]]; then
      continue
    fi
    if [[ "$raw_line" =~ ^[[:space:]]*# ]]; then
      continue
    fi
    TASK_ITEMS+=("$raw_line")
  done <"$TASKS_FILE"

  if [[ "${#TASK_ITEMS[@]}" -eq 0 ]]; then
    die "No tasks found in $TASKS_FILE after filtering blank lines and comments." 1
  fi
}

build_submit_command() {
  local index="$1"
  local source="$2"
  SUBMIT_CMD=("$ORCHESTRATORCTL" "submit" "$RUN_ID")

  case "$INPUT_MODE" in
    task-file)
      SUBMIT_CMD+=("$source")
      ;;
    task-json)
      SUBMIT_CMD+=("--task-json" "$source")
      ;;
    prompt-file)
      SUBMIT_CMD+=("--prompt-file" "$source")
      ;;
    goal)
      SUBMIT_CMD+=("--goal" "$source" "--title" "$(printf '%s %02d' "$TITLE_PREFIX" "$index")")
      ;;
    *)
      die "Unsupported input mode: $INPUT_MODE" 1
      ;;
  esac

  if [[ "$INPUT_MODE" == "prompt-file" || "$INPUT_MODE" == "goal" ]]; then
    if [[ -n "$ADAPTER" ]]; then
      SUBMIT_CMD+=("--adapter" "$ADAPTER")
    fi
    if [[ -n "$TIMEOUT_SECONDS" ]]; then
      SUBMIT_CMD+=("--timeout" "$TIMEOUT_SECONDS")
    fi
    if [[ -n "$POLICY" ]]; then
      SUBMIT_CMD+=("--policy" "$POLICY")
    fi
    local validation
    for validation in "${VALIDATIONS[@]-}"; do
      if [[ -z "$validation" ]]; then
        continue
      fi
      SUBMIT_CMD+=("--validation" "$validation")
    done
    local acceptance
    for acceptance in "${ACCEPTANCES[@]-}"; do
      if [[ -z "$acceptance" ]]; then
        continue
      fi
      SUBMIT_CMD+=("--acceptance" "$acceptance")
    done
  fi

  SUBMIT_CMD+=("--wait" "--json")
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --run-id)
      require_value "$1" "${2:-}"
      RUN_ID="$2"
      shift 2
      ;;
    --project)
      require_value "$1" "${2:-}"
      PROJECT="$2"
      shift 2
      ;;
    --input-mode)
      require_value "$1" "${2:-}"
      INPUT_MODE="$2"
      shift 2
      ;;
    --tasks-file)
      require_value "$1" "${2:-}"
      TASKS_FILE="$2"
      shift 2
      ;;
    --continue-on-failure)
      CONTINUE_ON_FAILURE=1
      shift
      ;;
    --title-prefix)
      require_value "$1" "${2:-}"
      TITLE_PREFIX="$2"
      TITLE_PREFIX_EXPLICIT=1
      shift 2
      ;;
    --adapter)
      require_value "$1" "${2:-}"
      ADAPTER="$2"
      shift 2
      ;;
    --timeout)
      require_value "$1" "${2:-}"
      TIMEOUT_SECONDS="$2"
      shift 2
      ;;
    --policy)
      require_value "$1" "${2:-}"
      POLICY="$2"
      shift 2
      ;;
    --validation)
      require_value "$1" "${2:-}"
      VALIDATIONS+=("$2")
      shift 2
      ;;
    --acceptance)
      require_value "$1" "${2:-}"
      ACCEPTANCES+=("$2")
      shift 2
      ;;
    --orchestratorctl)
      require_value "$1" "${2:-}"
      ORCHESTRATORCTL="$2"
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      usage
      die "Unknown argument: $1" 1
      ;;
  esac
done

detect_json_tool

if [[ "${CODENCER_CONTINUE_ON_FAILURE:-0}" == "1" && "$CONTINUE_ON_FAILURE" -eq 0 ]]; then
  CONTINUE_ON_FAILURE=1
fi

if [[ -z "$RUN_ID" || -z "$INPUT_MODE" || -z "$TASKS_FILE" ]]; then
  usage
  die "--run-id, --input-mode, and --tasks-file are required." 1
fi

case "$INPUT_MODE" in
  task-file|task-json|prompt-file|goal)
    ;;
  *)
    die "--input-mode must be one of: task-file, task-json, prompt-file, goal." 1
    ;;
esac

if [[ ! -f "$TASKS_FILE" ]]; then
  die "Tasks file not found: $TASKS_FILE" 1
fi

if [[ "$INPUT_MODE" == "task-file" || "$INPUT_MODE" == "task-json" ]]; then
  if [[ -n "$ADAPTER" || -n "$TIMEOUT_SECONDS" || -n "$POLICY" || "${#VALIDATIONS[@]}" -gt 0 || "${#ACCEPTANCES[@]}" -gt 0 ]]; then
    die "Direct-mode flags (--adapter, --timeout, --policy, --validation, --acceptance) are only supported with prompt-file and goal modes." 1
  fi
fi

if [[ "$INPUT_MODE" != "goal" && "${TITLE_PREFIX_EXPLICIT:-0}" -eq 1 ]]; then
  die "--title-prefix is only supported with goal mode." 1
fi

RESULTS_FILE="$(mktemp)"
trap 'rm -f "$RESULTS_FILE"' EXIT

TASKS_TOTAL=0
TASKS_SUCCEEDED=0
TASKS_FAILED=0
FIRST_NONZERO_EXIT=0

load_tasks
ensure_run

TASKS_TOTAL="${#TASK_ITEMS[@]}"

for idx in "${!TASK_ITEMS[@]}"; do
  index=$((idx + 1))
  source="${TASK_ITEMS[$idx]}"
  printf '[%d/%d] submitting %s\n' "$index" "$TASKS_TOTAL" "$source" >&2

  build_submit_command "$index" "$source"
  output="$("${SUBMIT_CMD[@]}")"
  exit_code=$?

  state="$(json_get_field "$output" "state")"
  if [[ -z "$state" ]]; then
    state="$(json_get_field "$output" "error")"
  fi
  if [[ -z "$state" ]]; then
    state="unknown"
  fi

  step_id="$(json_get_field "$output" "step_id")"
  if [[ -z "$step_id" ]]; then
    step_id="$(latest_step_id || true)"
  fi

  append_result "$index" "$source" "$step_id" "$state" "$exit_code"

  if [[ "$exit_code" -eq 0 ]]; then
    TASKS_SUCCEEDED=$((TASKS_SUCCEEDED + 1))
  else
    TASKS_FAILED=$((TASKS_FAILED + 1))
    if [[ "$FIRST_NONZERO_EXIT" -eq 0 ]]; then
      FIRST_NONZERO_EXIT="$exit_code"
    fi
    printf '[%d/%d] task failed with exit code %d and state %s\n' "$index" "$TASKS_TOTAL" "$exit_code" "$state" >&2
    if [[ "$CONTINUE_ON_FAILURE" -ne 1 ]]; then
      emit_summary "$exit_code"
      exit "$exit_code"
    fi
  fi
done

emit_summary "$FIRST_NONZERO_EXIT"
exit "$FIRST_NONZERO_EXIT"
