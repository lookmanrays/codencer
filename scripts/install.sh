#!/usr/bin/env sh

_codencer_install_sourced=0
(return 0 2>/dev/null) && _codencer_install_sourced=1

_codencer_install_main() (
  set -eu

  BINS="codencer orchestratord codencer-relayd codencer-gatewayd codencer-connectord"
  DEFAULT_REPO="lookmanrays/codencer"

  DRY_RUN=0
  JSON=0
  ALLOW_MISSING=0
  KEEP_DOWNLOADS=0
  NO_DOWNLOAD=0
  EXPLICIT_BIN_DIR=0
  BIN_DIR=""
  INSTALL_DIR="${CODENCER_INSTALL_DIR:-$HOME/.local/bin}"
  CODENCER_HOME_VALUE="${CODENCER_HOME:-$HOME/.codencer}"
  DOWNLOAD_DIR=""
  AUTO_DOWNLOAD_DIR=0
  VERSION=""
  DETECTED_PLATFORM=""
  RESOLVED_LATEST_RELEASE=""
  UNPACKED_BIN_DIR=""
  REPO="$DEFAULT_REPO"
  PLATFORM=""
  MODE=""
  ARTIFACT=""
  ARTIFACT_SHA256=""
  ERROR_MESSAGE=""

  json_string() {
    value=$1
    value=$(printf '%s' "$value" | sed 's/\\/\\\\/g; s/"/\\"/g; s/	/\\t/g')
    value=$(printf '%s' "$value" | awk 'BEGIN{ORS=""} {gsub(/\r/,"\\r"); if (NR>1) printf "\\n"; printf "%s",$0}')
    printf '"%s"' "$value"
  }

  json_bool() {
    if [ "$1" = "1" ]; then printf 'true'; else printf 'false'; fi
  }

  json_word_array() {
    first=1
    printf '['
    for value in "$@"; do
      if [ "$first" = "1" ]; then first=0; else printf ','; fi
      json_string "$value"
    done
    printf ']'
  }

  json_installed_binaries() {
    first=1
    printf '['
    for bin in $BINS; do
      if [ "$first" = "1" ]; then first=0; else printf ','; fi
      json_string "$INSTALL_DIR/$bin"
    done
    printf ']'
  }

  print_json_report() {
    ok=$1
    partial=$2
    missing_values=$3
    error_value=$4
    printf '{'
    printf '"ok":%s' "$(json_bool "$ok")"
    printf ',"partial":%s' "$(json_bool "$partial")"
    printf ',"dry_run":%s' "$(json_bool "$DRY_RUN")"
    printf ',"mode":%s' "$(json_string "$MODE")"
    printf ',"version":%s' "$(json_string "$VERSION")"
    printf ',"repo":%s' "$(json_string "$REPO")"
    printf ',"platform":%s' "$(json_string "$PLATFORM")"
    printf ',"artifact":%s' "$(json_string "$ARTIFACT")"
    printf ',"artifact_sha256":%s' "$(json_string "$ARTIFACT_SHA256")"
    printf ',"download_dir":%s' "$(json_string "$DOWNLOAD_DIR")"
    printf ',"install_dir":%s' "$(json_string "$INSTALL_DIR")"
    printf ',"codencer_home":%s' "$(json_string "$CODENCER_HOME_VALUE")"
    printf ',"installed_binaries":'
    json_installed_binaries
    printf ',"missing_binaries":%s' "$missing_values"
    printf ',"next_commands":'
    json_word_array "export PATH=\"$INSTALL_DIR:\$PATH\"" "codencer setup local --json" "codencer readiness --json"
    if [ -n "$error_value" ]; then
      printf ',"error":%s' "$(json_string "$error_value")"
    fi
    printf '}\n'
  }

  fail() {
    ERROR_MESSAGE=$1
    if [ "$JSON" = "1" ]; then
      print_json_report 0 0 "[]" "$ERROR_MESSAGE"
    else
      echo "codencer install: $ERROR_MESSAGE" >&2
    fi
    exit 30
  }

  fail_partial() {
    ERROR_MESSAGE=$1
    if [ "$JSON" = "1" ]; then
      print_json_report 0 1 "[]" "$ERROR_MESSAGE"
    else
      echo "codencer install: $ERROR_MESSAGE" >&2
    fi
    exit 30
  }

  usage() {
    cat <<'EOF'
Usage: install.sh [flags]

Default mode downloads installable Codencer GitHub Release assets. Explicit
package-local mode is available with --bin-dir or when this script is executed
from an unpacked Codencer release package that contains bin/.

Flags:
  --version <tag>         Release tag, for example v0.3.1. Defaults to latest.
  --repo <owner/name>     GitHub repository. Defaults to lookmanrays/codencer.
  --platform <os_arch>    Override platform, for example linux_amd64.
  --install-dir <path>    Install directory. Defaults to $CODENCER_INSTALL_DIR or $HOME/.local/bin.
  --codencer-home <path>  Codencer home. Defaults to $CODENCER_HOME or $HOME/.codencer.
  --download-dir <path>   Directory for downloaded release assets.
  --keep-downloads        Keep automatic download directory.
  --no-download           Use existing files in --download-dir; do not use network.
  --bin-dir <path>        Explicit package-local binary directory.
  --dry-run               Verify inputs and print planned result without installing.
  --json                  Print machine-readable output.
EOF
  }

  while [ "$#" -gt 0 ]; do
    case "$1" in
      --dry-run) DRY_RUN=1; shift ;;
      --json) JSON=1; shift ;;
      --allow-missing) ALLOW_MISSING=1; shift ;;
      --bin-dir) BIN_DIR=${2:?"--bin-dir requires a value"}; EXPLICIT_BIN_DIR=1; shift 2 ;;
      --install-dir) INSTALL_DIR=${2:?"--install-dir requires a value"}; shift 2 ;;
      --codencer-home) CODENCER_HOME_VALUE=${2:?"--codencer-home requires a value"}; shift 2 ;;
      --download-dir) DOWNLOAD_DIR=${2:?"--download-dir requires a value"}; shift 2 ;;
      --keep-downloads) KEEP_DOWNLOADS=1; shift ;;
      --no-download) NO_DOWNLOAD=1; shift ;;
      --version) VERSION=${2:?"--version requires a value"}; shift 2 ;;
      --repo) REPO=${2:?"--repo requires a value"}; shift 2 ;;
      --platform) PLATFORM=${2:?"--platform requires a value"}; shift 2 ;;
      --help|-h) usage; exit 0 ;;
      *) fail "unknown argument: $1" ;;
    esac
  done

  if [ "$NO_DOWNLOAD" = "1" ] && [ -z "$DOWNLOAD_DIR" ]; then
    fail "--no-download requires --download-dir"
  fi

  script_path=""
  bash_source="${BASH_SOURCE:-}"
  if [ -n "$bash_source" ] && [ -f "$bash_source" ]; then
    script_path=$bash_source
  elif [ -n "${0:-}" ] && [ -f "$0" ]; then
    script_path=$0
  fi

  script_dir=""
  package_root=""
  if [ -n "$script_path" ]; then
    script_dir=$(CDPATH= cd -- "$(dirname -- "$script_path")" 2>/dev/null && pwd -P) || script_dir=""
    if [ -n "$script_dir" ] && [ "$(basename -- "$script_dir")" = "scripts" ]; then
      package_root=$(CDPATH= cd -- "$script_dir/.." 2>/dev/null && pwd -P) || package_root=""
    fi
  fi

  have_all_bins() {
    dir=$1
    [ -n "$dir" ] || return 1
    for bin in $BINS; do
      [ -f "$dir/$bin" ] || return 1
    done
    return 0
  }

  if [ "$EXPLICIT_BIN_DIR" = "1" ]; then
    MODE="package-local"
  elif [ -n "$package_root" ] && have_all_bins "$package_root/bin"; then
    BIN_DIR="$package_root/bin"
    MODE="package-local"
  else
    MODE="release-bootstrap"
  fi

  cleanup() {
    if [ "$AUTO_DOWNLOAD_DIR" = "1" ] && [ "$KEEP_DOWNLOADS" != "1" ] && [ -n "$DOWNLOAD_DIR" ]; then
      rm -rf "$DOWNLOAD_DIR"
    fi
  }
  trap cleanup EXIT HUP INT TERM

  install_file() {
    src=$1
    dst=$2
    if command -v install >/dev/null 2>&1; then
      install -m 0755 "$src" "$dst"
    else
      cp "$src" "$dst"
      chmod 0755 "$dst"
    fi
  }

  missing_binaries_json() {
    missing=""
    for bin in $BINS; do
      if [ ! -f "$BIN_DIR/$bin" ]; then
        missing="${missing}${missing:+
}$bin"
      fi
    done
    if [ -z "$missing" ]; then
      printf '[]'
      return
    fi
    # shellcheck disable=SC2086
    set -- $missing
    json_word_array "$@"
  }

  install_from_bin_dir() {
    if [ -z "$BIN_DIR" ]; then
      fail "package-local mode could not resolve bin directory"
    fi
    missing_json=$(missing_binaries_json)
    partial=0
    ok=1
    if [ "$missing_json" != "[]" ]; then
      partial=1
      ok=0
    fi

    if [ "$ok" = "1" ] || [ "$ALLOW_MISSING" = "1" ]; then
      if [ "$DRY_RUN" != "1" ]; then
        mkdir -p "$INSTALL_DIR"
        for bin in $BINS; do
          src="$BIN_DIR/$bin"
          dst="$INSTALL_DIR/$bin"
          if [ -f "$src" ]; then
            install_file "$src" "$dst"
          fi
        done
        mkdir -p "$CODENCER_HOME_VALUE"
        if [ -x "$INSTALL_DIR/codencer" ]; then
          init_log=$(mktemp "${TMPDIR:-/tmp}/codencer-init.XXXXXX") || fail_partial "could not create temporary file for codencer init output"
          if ! CODENCER_HOME="$CODENCER_HOME_VALUE" "$INSTALL_DIR/codencer" init --json >/dev/null 2>"$init_log"; then
            if [ "$JSON" != "1" ] && [ -s "$init_log" ]; then
              cat "$init_log" >&2
            fi
            rm -f "$init_log"
            fail_partial "codencer init failed after installing binaries"
          fi
          rm -f "$init_log"
        fi
      fi
    fi

    if [ "$JSON" = "1" ]; then
      print_json_report "$ok" "$partial" "$missing_json" ""
    else
      echo "Codencer install"
      echo "  mode:          $MODE"
      echo "  dry_run:       $DRY_RUN"
      echo "  bin_dir:       $BIN_DIR"
      echo "  install_dir:   $INSTALL_DIR"
      echo "  codencer_home: $CODENCER_HOME_VALUE"
      echo "  allow_missing: $ALLOW_MISSING"
      for bin in $BINS; do
        if [ -f "$BIN_DIR/$bin" ]; then
          echo "  install: $BIN_DIR/$bin -> $INSTALL_DIR/$bin"
        else
          echo "  missing: $BIN_DIR/$bin"
        fi
      done
      echo
      echo "Next:"
      echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
      echo "  codencer setup local --json"
    fi

    if [ "$ok" != "1" ] && [ "$ALLOW_MISSING" != "1" ]; then
      exit 30
    fi
  }

  detect_platform() {
    os=$(uname -s 2>/dev/null || printf unknown)
    arch=$(uname -m 2>/dev/null || printf unknown)
    case "$os" in
      Darwin)
        case "$arch" in
          arm64|aarch64) DETECTED_PLATFORM="darwin_arm64" ;;
          x86_64|amd64) DETECTED_PLATFORM="darwin_amd64" ;;
          *) fail "unsupported Darwin architecture: $arch" ;;
        esac
        ;;
      Linux)
        case "$arch" in
          x86_64|amd64) DETECTED_PLATFORM="linux_amd64" ;;
          *) fail "unsupported Linux architecture: $arch" ;;
        esac
        ;;
      MINGW*|MSYS*|CYGWIN*|Windows_NT)
        fail "Windows-native Codencer artifacts are not published yet. Use WSL2/Linux artifact for now."
        ;;
      *)
        fail "unsupported platform: $os/$arch"
        ;;
    esac
  }

  curl_fetch() {
    url=$1
    out=$2
    command -v curl >/dev/null 2>&1 || fail "curl is required for release-bootstrap downloads"
    curl_log=$(mktemp "${TMPDIR:-/tmp}/codencer-curl.XXXXXX") || fail "could not create temporary file for download output"
    if ! curl -fsSL --retry 3 --proto '=https' --tlsv1.2 -o "$out" "$url" 2>"$curl_log"; then
      if [ "$JSON" != "1" ] && [ -s "$curl_log" ]; then
        cat "$curl_log" >&2
      fi
      rm -f "$curl_log"
      fail "download failed: $url"
    fi
    rm -f "$curl_log"
  }

  resolve_latest_release() {
    command -v curl >/dev/null 2>&1 || fail "curl is required to resolve the latest release"
    latest_url="https://github.com/$REPO/releases/latest"
    latest_tmp=$(mktemp "${TMPDIR:-/tmp}/codencer-latest.XXXXXX") || fail "could not create temporary file for latest release lookup"
    if ! curl -fsL --retry 3 --proto '=https' --tlsv1.2 -o /dev/null -w '%{url_effective}' "$latest_url" >"$latest_tmp" 2>/dev/null; then
      rm -f "$latest_tmp"
      fail "failed to resolve latest release for $REPO"
    fi
    effective=$(cat "$latest_tmp")
    rm -f "$latest_tmp"
    tag=${effective##*/}
    case "$tag" in
      v[0-9]*.[0-9]*.[0-9]*) RESOLVED_LATEST_RELEASE=$tag ;;
      *) fail "could not resolve latest release for $REPO from $effective" ;;
    esac
  }

  sha256_file() {
    file=$1
    if command -v sha256sum >/dev/null 2>&1; then
      sha256sum "$file" | awk '{print $1}'
      return
    fi
    if command -v shasum >/dev/null 2>&1; then
      shasum -a 256 "$file" | awk '{print $1}'
      return
    fi
    fail "sha256sum or shasum is required to verify release assets"
  }

  verify_manifest() {
    manifest_file=$1
    artifact_name=$2
    expected_sha=$3
    version_value=$4
    platform_value=$5
    command -v python3 >/dev/null 2>&1 || fail "python3 is required to verify manifest.json"
    python3 - "$manifest_file" "$artifact_name" "$expected_sha" "$version_value" "$platform_value" <<'PY'
import json
import sys
from pathlib import Path

manifest_path, artifact_name, expected_sha, version, platform = sys.argv[1:]
data = json.loads(Path(manifest_path).read_text())
for key in ("version", "tag_name"):
    value = data.get(key)
    if value and value != version:
        raise SystemExit(f"manifest {key} {value!r} does not match {version!r}")

os_name, arch = platform.split("_", 1)
records = []
for asset in data.get("assets", []):
    records.append({
        "filename": asset.get("filename") or asset.get("name"),
        "sha256": asset.get("sha256"),
        "os": asset.get("os"),
        "arch": asset.get("arch"),
    })
for artifact in data.get("artifacts", []):
    records.append({
        "filename": artifact.get("filename") or artifact.get("name"),
        "sha256": artifact.get("sha256"),
        "os": artifact.get("os"),
        "arch": artifact.get("arch"),
    })

matches = [item for item in records if item.get("filename") == artifact_name]
if len(matches) != 1:
    raise SystemExit(f"manifest must reference {artifact_name} exactly once, got {len(matches)}")
item = matches[0]
if item.get("sha256") != expected_sha:
    raise SystemExit(f"manifest sha256 mismatch for {artifact_name}")
if item.get("os") and item.get("os") != os_name:
    raise SystemExit(f"manifest os mismatch for {artifact_name}")
if item.get("arch") and item.get("arch") != arch:
    raise SystemExit(f"manifest arch mismatch for {artifact_name}")
PY
  }

  find_unpacked_bin_dir() {
    unpack_root=$1
    found=""
    count=0
    for dir in "$unpack_root"/*/bin "$unpack_root"/bin; do
      [ -d "$dir" ] || continue
      if have_all_bins "$dir"; then
        found=$dir
        count=$((count + 1))
      fi
    done
    if [ "$count" -ne 1 ]; then
      fail "expected exactly one unpacked Codencer bin directory, got $count"
    fi
    UNPACKED_BIN_DIR=$found
  }

  install_release_bootstrap() {
    if [ -z "$PLATFORM" ]; then
      detect_platform
      PLATFORM=$DETECTED_PLATFORM
    fi
    if [ -z "$VERSION" ]; then
      if [ "$NO_DOWNLOAD" = "1" ]; then
        fail "--version is required with --no-download"
      fi
      resolve_latest_release
      VERSION=$RESOLVED_LATEST_RELEASE
    fi

    ARTIFACT="codencer_${VERSION}_${PLATFORM}.tar.gz"
    base_url="https://github.com/$REPO/releases/download/$VERSION"

    if [ -z "$DOWNLOAD_DIR" ]; then
      DOWNLOAD_DIR=$(mktemp -d "${TMPDIR:-/tmp}/codencer-install.XXXXXX")
      AUTO_DOWNLOAD_DIR=1
    else
      mkdir -p "$DOWNLOAD_DIR"
    fi

    artifact_path="$DOWNLOAD_DIR/$ARTIFACT"
    checksums_path="$DOWNLOAD_DIR/checksums.txt"
    manifest_path="$DOWNLOAD_DIR/manifest.json"

    if [ "$NO_DOWNLOAD" != "1" ]; then
      curl_fetch "$base_url/$ARTIFACT" "$artifact_path"
      curl_fetch "$base_url/checksums.txt" "$checksums_path"
      curl_fetch "$base_url/manifest.json" "$manifest_path"
    fi

    [ -f "$artifact_path" ] || fail "release artifact missing: $artifact_path"
    [ -f "$checksums_path" ] || fail "checksums.txt missing: $checksums_path"
    [ -f "$manifest_path" ] || fail "manifest.json missing: $manifest_path"

    expected_sha=$(awk -v name="$ARTIFACT" '($2 == name || $2 == "./" name || $2 == "*" name) {print $1; exit}' "$checksums_path")
    [ -n "$expected_sha" ] || fail "$ARTIFACT is missing from checksums.txt"
    actual_sha=$(sha256_file "$artifact_path")
    if [ "$actual_sha" != "$expected_sha" ]; then
      fail "sha256 mismatch for $ARTIFACT"
    fi
    ARTIFACT_SHA256=$actual_sha
    manifest_log=$(mktemp "${TMPDIR:-/tmp}/codencer-manifest.XXXXXX") || fail "could not create temporary file for manifest verification output"
    if ! verify_manifest "$manifest_path" "$ARTIFACT" "$ARTIFACT_SHA256" "$VERSION" "$PLATFORM" 2>"$manifest_log"; then
      if [ "$JSON" != "1" ] && [ -s "$manifest_log" ]; then
        cat "$manifest_log" >&2
      fi
      rm -f "$manifest_log"
      fail "manifest verification failed for $ARTIFACT"
    fi
    rm -f "$manifest_log"

    unpack_dir=$(mktemp -d "${TMPDIR:-/tmp}/codencer-install-unpack.XXXXXX")
    tar_log="$unpack_dir/tar.err"
    if ! tar -xzf "$artifact_path" -C "$unpack_dir" 2>"$tar_log"; then
      if [ "$JSON" != "1" ]; then
        cat "$tar_log" >&2
      fi
      fail "failed to extract $ARTIFACT"
    fi
    find_unpacked_bin_dir "$unpack_dir"
    BIN_DIR=$UNPACKED_BIN_DIR
    install_from_bin_dir
    rm -rf "$unpack_dir"
  }

  if [ "$MODE" = "release-bootstrap" ]; then
    install_release_bootstrap
  else
    install_from_bin_dir
  fi
)

_codencer_install_main "$@"
_codencer_install_status=$?
if [ "$_codencer_install_sourced" = "1" ]; then
  return "$_codencer_install_status"
fi
exit "$_codencer_install_status"
