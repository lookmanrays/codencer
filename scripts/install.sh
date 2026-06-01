#!/usr/bin/env bash
set -euo pipefail

DRY_RUN=0
JSON=0
BIN_DIR="bin"
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
    --bin-dir) BIN_DIR="${2:?--bin-dir requires a value}"; shift 2 ;;
    --install-dir) INSTALL_DIR="${2:?--install-dir requires a value}"; shift 2 ;;
    --codencer-home) CODENCER_HOME_VALUE="${2:?--codencer-home requires a value}"; shift 2 ;;
    *) echo "unknown argument: $1" >&2; exit 30 ;;
  esac
done

BINS=(codencer orchestratord codencer-relayd codencer-connectord)
ACTIONS=()
for bin in "${BINS[@]}"; do
  src="$BIN_DIR/$bin"
  dst="$INSTALL_DIR/$bin"
  if [ ! -f "$src" ]; then
    ACTIONS+=("missing:$src")
  else
    ACTIONS+=("install:$src:$dst")
    if [ "$DRY_RUN" != "1" ]; then
      mkdir -p "$INSTALL_DIR"
      install -m 0755 "$src" "$dst"
    fi
  fi
done

if [ "$DRY_RUN" != "1" ]; then
  mkdir -p "$CODENCER_HOME_VALUE"
  if command -v "$INSTALL_DIR/codencer" >/dev/null 2>&1 || [ -x "$INSTALL_DIR/codencer" ]; then
    CODENCER_HOME="$CODENCER_HOME_VALUE" "$INSTALL_DIR/codencer" init --json >/dev/null || true
  fi
fi

if [ "$JSON" = "1" ]; then
  printf '{"ok":true,"dry_run":%s,"bin_dir":%s,"install_dir":%s,"codencer_home":%s,"actions":[' \
    "$([ "$DRY_RUN" = "1" ] && echo true || echo false)" \
    "$(json_string "$BIN_DIR")" \
    "$(json_string "$INSTALL_DIR")" \
    "$(json_string "$CODENCER_HOME_VALUE")"
  first=1
  for action in "${ACTIONS[@]}"; do
    [ "$first" = "1" ] || printf ','
    first=0
    json_string "$action"
  done
  printf '],"next_commands":["codencer setup local --json","codencer readiness --json"]}\n'
  exit 0
fi

echo "Codencer install"
echo "  dry_run:       $DRY_RUN"
echo "  bin_dir:       $BIN_DIR"
echo "  install_dir:   $INSTALL_DIR"
echo "  codencer_home: $CODENCER_HOME_VALUE"
for action in "${ACTIONS[@]}"; do
  echo "  $action"
done
echo
echo "Next:"
echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
echo "  codencer setup local --json"
