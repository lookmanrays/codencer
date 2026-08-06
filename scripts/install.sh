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

  json_planned_assets() {
    if [ -z "$ARTIFACT" ]; then
      printf '[]'
      return
    fi
    json_word_array "$ARTIFACT" "checksums.txt" "manifest.json"
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
    printf ',"planned_assets":'
    json_planned_assets
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
  --version <tag>         Release tag, for example vX.Y.Z. Defaults to latest.
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

  if [ "$NO_DOWNLOAD" = "1" ] && [ "$DRY_RUN" != "1" ] && [ -z "$DOWNLOAD_DIR" ]; then
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

  generated_ascii_file_valid() {
    file=$1
    # od observes the complete byte stream before awk sees only decimal text.
    # Both generators emit LF plus printable ASCII and no other byte values.
    if ! byte_dump=$(LC_ALL=C od -A n -t u1 -v "$file"); then
      return 1
    fi
    printf '%s\n' "$byte_dump" | LC_ALL=C awk '
      {
        for (i = 1; i <= NF; i++) {
          if ($i !~ /^[0-9]+$/) {
            exit 1
          }
          byte = $i + 0
          if (byte != 10 && (byte < 32 || byte > 126)) {
            exit 1
          }
        }
      }
    '
  }

  checksum_for_artifact() {
    checksums_file=$1
    artifact_name=$2
    if ! generated_ascii_file_valid "$checksums_file"; then
      return 1
    fi
    LC_ALL=C awk -v want="$artifact_name" '
      {
        if (NF != 2 || length($1) != 64 || $1 ~ /[^0-9a-f]/ || $2 == "" || index($2, "/") != 0 || seen[$2]++) {
          invalid = 1
          next
        }
        if ($2 == want) {
          matches++
          digest = $1
        }
      }
      END {
        if (invalid || matches != 1) {
          exit 1
        }
        print digest
      }
    ' "$checksums_file"
  }

  json_manifest_verify() {
    manifest_file=$1
    artifact_name=$2
    expected_sha=$3
    version_value=$4
    platform_value=$5
    os_name=${platform_value%%_*}
    arch=${platform_value#*_}

    if ! generated_ascii_file_valid "$manifest_file"; then
      return 1
    fi

    # Both manifest generators use fixed, unescaped ASCII member names. Rejecting
    # escaped object member names avoids raw-spelling versus decoded-key ambiguity.
    LC_ALL=C awk \
      -v expected_artifact="$artifact_name" \
      -v expected_sha="$expected_sha" \
      -v expected_version="$version_value" \
      -v expected_os="$os_name" \
      -v expected_arch="$arch" '
      {
        data = data $0 "\n"
      }
      END {
        n = length(data)
        pos = 1
        if (!parse_manifest()) {
          exit 1
        }
        skipws()
        if (pos <= n || !validate_manifest() || matches != 1) {
          exit 1
        }
      }
      function ch() {
        return substr(data, pos, 1)
      }
      function skipws() {
        while (pos <= n && index(" \t\r\n", ch())) {
          pos++
        }
      }
      function parse_string(  c, esc, i, hex, hex_value) {
        if (ch() != "\"") {
          return 0
        }
        pos++
        string_value = ""
        string_escaped = 0
        string_decode_ok = 1
        while (pos <= n) {
          c = ch()
          if (c == "\"") {
            pos++
            return 1
          }
          if (c == "\\") {
            string_escaped = 1
            pos++
            if (pos > n) {
              return 0
            }
            esc = ch()
            if (esc == "\"" || esc == "\\" || esc == "/" || esc == "b" || esc == "f" || esc == "n" || esc == "r" || esc == "t") {
              if (esc == "\"") string_value = string_value "\""
              else if (esc == "\\") string_value = string_value "\\"
              else if (esc == "/") string_value = string_value "/"
              else if (esc == "b") string_value = string_value sprintf("%c", 8)
              else if (esc == "f") string_value = string_value sprintf("%c", 12)
              else if (esc == "n") string_value = string_value "\n"
              else if (esc == "r") string_value = string_value "\r"
              else if (esc == "t") string_value = string_value "\t"
              pos++
              continue
            }
            if (esc != "u") {
              return 0
            }
            pos++
            for (i = 0; i < 4; i++) {
              if (pos > n) {
                return 0
              }
              hex = ch()
              if (hex !~ /[0-9A-Fa-f]/) {
                return 0
              }
              pos++
            }
            hex_value = json_hex_value(substr(data, pos - 4, 4))
            if (hex_value >= 0 && hex_value <= 127) {
              string_value = string_value sprintf("%c", hex_value)
            } else {
              string_decode_ok = 0
            }
            continue
          }
          if (c == sprintf("%c", 0) || c ~ /[\001-\037]/) {
            return 0
          }
          string_value = string_value c
          pos++
        }
        return 0
      }
      function json_hex_value(value,  i, digit, result) {
        result = 0
        value = tolower(value)
        for (i = 1; i <= 4; i++) {
          digit = index("0123456789abcdef", substr(value, i, 1)) - 1
          if (digit < 0) return -1
          result = result * 16 + digit
        }
        return result
      }
      function parse_plain_string() {
        if (!parse_string() || !string_decode_ok) {
          return 0
        }
        return 1
      }
      function parse_bool() {
        if (substr(data, pos, 4) == "true") {
          bool_value = 1
          pos += 4
          return 1
        }
        if (substr(data, pos, 5) == "false") {
          bool_value = 0
          pos += 5
          return 1
        }
        return 0
      }
      function parse_manifest(  key, c) {
        skipws()
        if (ch() != "{") {
          return 0
        }
        pos++
        skipws()
        if (ch() == "}") {
          pos++
          return 1
        }
        while (1) {
          skipws()
          if (!parse_string() || string_escaped) {
            return 0
          }
          key = string_value
          if (top_seen[key]++) {
            return 0
          }
          skipws()
          if (ch() != ":") {
            return 0
          }
          pos++
          skipws()
          if (!parse_top_value(key)) {
            return 0
          }
          skipws()
          c = ch()
          if (c == ",") {
            pos++
            continue
          }
          if (c == "}") {
            pos++
            return 1
          }
          return 0
        }
      }
      function parse_top_value(key) {
        if (key == "version") {
          if (!parse_plain_string()) return 0
          top_version = string_value
          return 1
        }
        if (key == "tag_name") {
          if (!parse_plain_string()) return 0
          top_tag_name = string_value
          return 1
        }
        if (key == "release_sha") {
          if (!parse_plain_string()) return 0
          top_release_sha = string_value
          return 1
        }
        if (key == "built_at") {
          if (!parse_plain_string()) return 0
          top_built_at = string_value
          return 1
        }
        if (key == "note") {
          return parse_string()
        }
        if (key == "assets") {
          has_assets = 1
          return parse_github_assets()
        }
        if (key == "commit") {
          if (!parse_plain_string()) return 0
          top_commit = string_value
          return 1
        }
        if (key == "targets") {
          return parse_string_array("targets")
        }
        if (key == "required_targets") {
          return parse_string_array("required_targets")
        }
        if (key == "allow_partial") {
          if (!parse_bool()) return 0
          top_allow_partial = bool_value
          return 1
        }
        if (key == "partial") {
          if (!parse_bool()) return 0
          top_partial = bool_value
          return 1
        }
        if (key == "artifacts") {
          has_artifacts = 1
          return parse_local_artifacts()
        }
        return 0
      }
      function parse_string_array(kind,  c, value) {
        if (ch() != "[") {
          return 0
        }
        pos++
        skipws()
        if (ch() == "]") {
          pos++
          return 1
        }
        while (1) {
          skipws()
          if (!parse_plain_string() || string_value == "") {
            return 0
          }
          value = string_value
          if (kind == "targets") {
            if (target_seen[value]++) return 0
            targets[++target_count] = value
          } else {
            if (required_target_seen[value]++) return 0
            required_targets[++required_target_count] = value
          }
          skipws()
          c = ch()
          if (c == ",") {
            pos++
            continue
          }
          if (c == "]") {
            pos++
            return 1
          }
          return 0
        }
      }
      function parse_github_assets(  c) {
        if (ch() != "[") {
          return 0
        }
        pos++
        skipws()
        if (ch() == "]") {
          pos++
          return 1
        }
        while (1) {
          skipws()
          if (ch() != "{" || !parse_github_asset()) {
            return 0
          }
          skipws()
          c = ch()
          if (c == ",") {
            pos++
            continue
          }
          if (c == "]") {
            pos++
            return 1
          }
          return 0
        }
      }
      function parse_github_asset(  id, key, c) {
        id = ++record_id
        pos++
        skipws()
        if (ch() == "}") {
          pos++
          return validate_github_asset(id)
        }
        while (1) {
          skipws()
          if (!parse_string() || string_escaped) return 0
          key = string_value
          if (record_seen[id, key]++) return 0
          skipws()
          if (ch() != ":") return 0
          pos++
          skipws()
          if (key == "filename" || key == "sha256" || key == "os" || key == "arch" || key == "runner") {
            if (!parse_plain_string()) return 0
            record_value[id, key] = string_value
            record_present[id, key] = 1
          } else {
            return 0
          }
          skipws()
          c = ch()
          if (c == ",") {
            pos++
            continue
          }
          if (c == "}") {
            pos++
            return validate_github_asset(id)
          }
          return 0
        }
      }
      function validate_github_asset(id,  name, sha, os_value, arch_value, runner, pair, index_value) {
        if (!record_present[id, "filename"] || !record_present[id, "sha256"] || !record_present[id, "os"] || !record_present[id, "arch"] || !record_present[id, "runner"]) return 0
        name = record_value[id, "filename"]
        sha = record_value[id, "sha256"]
        os_value = record_value[id, "os"]
        arch_value = record_value[id, "arch"]
        runner = record_value[id, "runner"]
        if (name == "" || index(name, "/") || length(sha) != 64 || sha ~ /[^0-9a-f]/ || os_value == "" || arch_value == "" || runner == "") return 0
        pair = os_value "/" arch_value
        if (manifest_name_seen[name]++ || platform_seen[pair]++) return 0
        index_value = ++github_asset_count
        github_name[index_value] = name
        github_os[index_value] = os_value
        github_arch[index_value] = arch_value
        github_runner[index_value] = runner
        if (name == expected_artifact) {
          matches++
          if (sha != expected_sha || os_value != expected_os || arch_value != expected_arch) return 0
        }
        return 1
      }
      function parse_local_artifacts(  c) {
        if (ch() != "[") return 0
        pos++
        skipws()
        if (ch() == "]") {
          pos++
          return 1
        }
        while (1) {
          skipws()
          if (ch() != "{" || !parse_local_artifact()) return 0
          skipws()
          c = ch()
          if (c == ",") {
            pos++
            continue
          }
          if (c == "]") {
            pos++
            return 1
          }
          return 0
        }
      }
      function parse_local_artifact(  id, key, c) {
        id = ++record_id
        pos++
        skipws()
        if (ch() == "}") {
          pos++
          return validate_local_artifact(id)
        }
        while (1) {
          skipws()
          if (!parse_string() || string_escaped) return 0
          key = string_value
          if (record_seen[id, key]++) return 0
          skipws()
          if (ch() != ":") return 0
          pos++
          skipws()
          if (key == "name" || key == "os" || key == "arch" || key == "sha256" || key == "status" || key == "mode") {
            if (!parse_plain_string()) return 0
            record_value[id, key] = string_value
            record_present[id, key] = 1
          } else if (key == "message") {
            if (!parse_string()) return 0
            record_present[id, key] = 1
          } else if (key == "required") {
            if (!parse_bool()) return 0
            record_bool[id, key] = bool_value
            record_present[id, key] = 1
          } else {
            return 0
          }
          skipws()
          c = ch()
          if (c == ",") {
            pos++
            continue
          }
          if (c == "}") {
            pos++
            return validate_local_artifact(id)
          }
          return 0
        }
      }
      function validate_local_artifact(id,  name, sha, os_value, arch_value, status, mode, pair, index_value) {
        if (!record_present[id, "name"] || !record_present[id, "os"] || !record_present[id, "arch"] || !record_present[id, "status"] || !record_present[id, "required"]) return 0
        name = record_value[id, "name"]
        sha = record_value[id, "sha256"]
        os_value = record_value[id, "os"]
        arch_value = record_value[id, "arch"]
        status = record_value[id, "status"]
        mode = record_value[id, "mode"]
        if (name == "" || index(name, "/") || os_value == "" || arch_value == "" || (status != "built" && status != "not_built" && status != "skipped")) return 0
        if (record_present[id, "mode"] && mode != "host" && mode != "docker") return 0
        if (status == "built") {
          if (!record_present[id, "sha256"] || length(sha) != 64 || sha ~ /[^0-9a-f]/) return 0
        } else {
          if (record_present[id, "sha256"]) return 0
          local_nonbuilt++
        }
        pair = os_value "/" arch_value
        if (manifest_name_seen[name]++ || platform_seen[pair]++) return 0
        index_value = ++local_artifact_count
        local_name[index_value] = name
        local_pair[index_value] = pair
        local_required[index_value] = record_bool[id, "required"]
        if (name == expected_artifact) {
          matches++
          if (status != "built" || sha != expected_sha || os_value != expected_os || arch_value != expected_arch) return 0
        }
        return 1
      }
      function validate_manifest(  i, pair, expected_name, expected_runner, partial_value) {
        if (!top_seen["version"] || top_version == "" || top_version != expected_version || !top_seen["built_at"] || top_built_at == "") return 0
        if (has_assets) {
          if (has_artifacts || !top_seen["tag_name"] || top_tag_name != expected_version || !top_seen["release_sha"] || top_release_sha == "" || !top_seen["note"] || top_seen["commit"] || top_seen["targets"] || top_seen["required_targets"] || top_seen["allow_partial"] || top_seen["partial"]) return 0
          if (github_asset_count != 3 || !platform_seen["linux/amd64"] || !platform_seen["darwin/amd64"] || !platform_seen["darwin/arm64"]) return 0
          for (i = 1; i <= github_asset_count; i++) {
            pair = github_os[i] "/" github_arch[i]
            expected_name = "codencer_" top_version "_" github_os[i] "_" github_arch[i] ".tar.gz"
            expected_runner = (github_os[i] == "linux" ? "ubuntu-latest" : "macos-latest")
            if (github_name[i] != expected_name || github_runner[i] != expected_runner) return 0
          }
          return 1
        }
        if (!has_artifacts || top_seen["tag_name"] || top_seen["release_sha"] || top_seen["note"] || !top_seen["commit"] || !top_seen["targets"] || !top_seen["required_targets"] || !top_seen["allow_partial"] || !top_seen["partial"]) return 0
        if (target_count == 0 || local_artifact_count != target_count) return 0
        for (i = 1; i <= required_target_count; i++) {
          if (!target_seen[required_targets[i]]) return 0
        }
        for (i = 1; i <= local_artifact_count; i++) {
          pair = local_pair[i]
          if (!target_seen[pair] || local_required[i] != (required_target_seen[pair] ? 1 : 0)) return 0
          split(pair, pair_parts, "/")
          expected_name = "codencer_" top_version "_" pair_parts[1] "_" pair_parts[2] ".tar.gz"
          if (local_name[i] != expected_name) return 0
        }
        partial_value = (local_nonbuilt > 0 ? 1 : 0)
        if (top_partial != partial_value) return 0
        return 1
      }
    ' "$manifest_file"
  }

  verify_manifest() {
    if ! json_manifest_verify "$1" "$2" "$3" "$4" "$5"; then
      echo "manifest does not match a generated Codencer release schema" >&2
      return 1
    fi
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
    if [ "$DRY_RUN" = "1" ]; then
      if [ -z "$VERSION" ]; then
        VERSION="latest"
      fi
      ARTIFACT="codencer_${VERSION}_${PLATFORM}.tar.gz"
      if [ "$JSON" = "1" ]; then
        print_json_report 1 0 "[]" ""
      else
        echo "Codencer install plan"
        echo "  mode:          $MODE"
        echo "  dry_run:       $DRY_RUN"
        echo "  repo:          $REPO"
        echo "  version:       $VERSION"
        echo "  platform:      $PLATFORM"
        echo "  artifact:      $ARTIFACT"
        echo "  assets:        $ARTIFACT, checksums.txt, manifest.json"
        echo "  install_dir:   $INSTALL_DIR"
        echo "  codencer_home: $CODENCER_HOME_VALUE"
        if [ -n "$DOWNLOAD_DIR" ]; then
          echo "  download_dir:  $DOWNLOAD_DIR"
        fi
      fi
      return
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

    if ! expected_sha=$(checksum_for_artifact "$checksums_path" "$ARTIFACT"); then
      fail "checksums.txt is malformed, ambiguous, or missing $ARTIFACT"
    fi
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
