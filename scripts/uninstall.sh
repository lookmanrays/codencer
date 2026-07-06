#!/usr/bin/env bash
set -euo pipefail

DRY_RUN=0
JSON=0
PURGE=0
STOP_SERVICES=0
UNINSTALL_SERVICES=0
INSTALL_DIR="${CODENCER_INSTALL_DIR:-$HOME/.local/bin}"
CODENCER_HOME_VALUE="${CODENCER_HOME:-$HOME/.codencer}"

json_string() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  value="${value//$'\n'/\\n}"
  value="${value//$'\r'/\\r}"
  value="${value//$'\t'/\\t}"
  printf '"%s"' "$value"
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --dry-run) DRY_RUN=1; shift ;;
    --json) JSON=1; shift ;;
    --purge) PURGE=1; shift ;;
    --stop-services) STOP_SERVICES=1; shift ;;
    --uninstall-services) UNINSTALL_SERVICES=1; shift ;;
    --install-dir) INSTALL_DIR="${2:?--install-dir requires a value}"; shift 2 ;;
    --codencer-home) CODENCER_HOME_VALUE="${2:?--codencer-home requires a value}"; shift 2 ;;
    *) echo "unknown argument: $1" >&2; exit 30 ;;
  esac
done

ACTIONS=()
CODENCER_BIN="$INSTALL_DIR/codencer"
if [ "$STOP_SERVICES" = "1" ]; then
  ACTIONS+=("service-stop-all")
  if [ "$DRY_RUN" != "1" ] && [ -x "$CODENCER_BIN" ]; then
    CODENCER_HOME="$CODENCER_HOME_VALUE" "$CODENCER_BIN" service stop --all --json >/dev/null || true
  fi
fi
if [ "$UNINSTALL_SERVICES" = "1" ]; then
  ACTIONS+=("service-uninstall-all")
  if [ "$DRY_RUN" != "1" ] && [ -x "$CODENCER_BIN" ]; then
    CODENCER_HOME="$CODENCER_HOME_VALUE" "$CODENCER_BIN" service uninstall --all --json >/dev/null || true
  fi
fi
for bin in codencer orchestratord codencer-relayd codencer-gatewayd codencer-connectord agent-broker; do
  path="$INSTALL_DIR/$bin"
  ACTIONS+=("remove:$path")
  if [ "$DRY_RUN" != "1" ]; then
    rm -f "$path"
  fi
done
if [ "$PURGE" = "1" ]; then
  ACTIONS+=("purge:$CODENCER_HOME_VALUE")
  if [ "$DRY_RUN" != "1" ]; then
    rm -rf "$CODENCER_HOME_VALUE"
  fi
else
  ACTIONS+=("preserve-home:$CODENCER_HOME_VALUE")
fi

if [ "$JSON" = "1" ]; then
  printf '{"ok":true,"dry_run":%s,"purge":%s,"install_dir":%s,"codencer_home":%s,"actions":[' \
    "$([ "$DRY_RUN" = "1" ] && echo true || echo false)" \
    "$([ "$PURGE" = "1" ] && echo true || echo false)" \
    "$(json_string "$INSTALL_DIR")" "$(json_string "$CODENCER_HOME_VALUE")"
  first=1
  for action in "${ACTIONS[@]}"; do
    [ "$first" = "1" ] || printf ','
    first=0
    json_string "$action"
  done
  printf ']}\n'
  exit 0
fi

echo "Codencer uninstall"
echo "  dry_run:       $DRY_RUN"
echo "  install_dir:   $INSTALL_DIR"
echo "  codencer_home: $CODENCER_HOME_VALUE"
echo "  purge:         $PURGE"
for action in "${ACTIONS[@]}"; do
  echo "  $action"
done
