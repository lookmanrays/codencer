#!/usr/bin/env bash
set -euo pipefail

DRY_RUN=0
JSON=0
ALLOW_MISSING=0
BIN_DIR="bin"
INSTALL_DIR="${CODENCER_INSTALL_DIR:-$HOME/.local/bin}"
CODENCER_HOME_VALUE="${CODENCER_HOME:-$HOME/.codencer}"
BINS=(codencer orchestratord codencer-relayd codencer-connectord)

json_string() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  value="${value//$'\n'/\\n}"
  value="${value//$'\r'/\\r}"
  value="${value//$'\t'/\\t}"
  printf '"%s"' "$value"
}

json_bool() {
  if [ "$1" = "1" ]; then printf 'true'; else printf 'false'; fi
}

json_array() {
  local first=1
  printf '['
  for value in "$@"; do
    [ "$first" = "1" ] || printf ','
    first=0
    json_string "$value"
  done
  printf ']'
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --dry-run) DRY_RUN=1; shift ;;
    --json) JSON=1; shift ;;
    --allow-missing) ALLOW_MISSING=1; shift ;;
    --bin-dir) BIN_DIR="${2:?--bin-dir requires a value}"; shift 2 ;;
    --install-dir) INSTALL_DIR="${2:?--install-dir requires a value}"; shift 2 ;;
    --codencer-home) CODENCER_HOME_VALUE="${2:?--codencer-home requires a value}"; shift 2 ;;
    *) echo "unknown argument: $1" >&2; exit 30 ;;
  esac
done

ACTIONS=()
MISSING=()
for bin in "${BINS[@]}"; do
  src="$BIN_DIR/$bin"
  dst="$INSTALL_DIR/$bin"
  if [ ! -f "$src" ]; then
    ACTIONS+=("missing:$src")
    MISSING+=("$bin")
  else
    ACTIONS+=("replace:$src:$dst")
  fi
done

OK=1
PARTIAL=0
if [ "${#MISSING[@]}" -gt 0 ]; then
  OK=0
  PARTIAL=1
fi

if [ "${#MISSING[@]}" -eq 0 ] || [ "$ALLOW_MISSING" = "1" ]; then
  if [ "$DRY_RUN" != "1" ]; then
    mkdir -p "$INSTALL_DIR"
    for bin in "${BINS[@]}"; do
      src="$BIN_DIR/$bin"
      dst="$INSTALL_DIR/$bin"
      if [ -f "$src" ]; then
        tmp="$dst.tmp.$$"
        install -m 0755 "$src" "$tmp"
        mv "$tmp" "$dst"
      fi
    done
    if [ -x "$INSTALL_DIR/codencer" ]; then
      CODENCER_HOME="$CODENCER_HOME_VALUE" "$INSTALL_DIR/codencer" doctor --json >/dev/null || true
      CODENCER_HOME="$CODENCER_HOME_VALUE" "$INSTALL_DIR/codencer" readiness --json >/dev/null || true
    fi
  fi
fi

if [ "$JSON" = "1" ]; then
  missing_json="[]"
  if [ "${#MISSING[@]}" -gt 0 ]; then
    missing_json="$(json_array "${MISSING[@]}")"
  fi
  actions_json="$(json_array "${ACTIONS[@]}")"
  printf '{"ok":%s,"partial":%s,"dry_run":%s,"allow_missing":%s,"bin_dir":%s,"install_dir":%s,"codencer_home":%s,"missing_binaries":%s,"actions":%s,"post_checks":["codencer doctor --json","codencer readiness --json"]}\n' \
    "$(json_bool "$OK")" \
    "$(json_bool "$PARTIAL")" \
    "$(json_bool "$DRY_RUN")" \
    "$(json_bool "$ALLOW_MISSING")" \
    "$(json_string "$BIN_DIR")" \
    "$(json_string "$INSTALL_DIR")" \
    "$(json_string "$CODENCER_HOME_VALUE")" \
    "$missing_json" \
    "$actions_json"
else
  echo "Codencer upgrade"
  echo "  dry_run:       $DRY_RUN"
  echo "  bin_dir:       $BIN_DIR"
  echo "  install_dir:   $INSTALL_DIR"
  echo "  codencer_home: $CODENCER_HOME_VALUE"
  echo "  allow_missing: $ALLOW_MISSING"
  for action in "${ACTIONS[@]}"; do
    echo "  $action"
  done
fi

if [ "${#MISSING[@]}" -gt 0 ] && [ "$ALLOW_MISSING" != "1" ]; then
  exit 30
fi
