#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${VERSION:-v0.3.0-selfhost-artifact-verify}"
DIST="${DIST:-$ROOT/dist}"
TARGETS="${TARGETS:-host}"
REQUIRE_TARGETS="${REQUIRE_TARGETS:-host}"
SKIP_RELEASE_SNAPSHOT="${SKIP_RELEASE_SNAPSHOT:-0}"
TMPDIR_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/codencer-release-artifact.XXXXXX")"

cleanup() {
  rm -rf "$TMPDIR_ROOT"
}
trap cleanup EXIT

json_get() {
  python3 - "$1" "$2" <<'PY'
import json, sys
value=json.load(open(sys.argv[1]))
for part in sys.argv[2].split("."):
  if part:
    value=value[part] if not part.isdigit() else value[int(part)]
print(value)
PY
}

host_os="$(go env GOOS)"
host_arch="$(go env GOARCH)"
dist_abs="$(python3 - "$DIST" "$ROOT" <<'PY'
from pathlib import Path
import sys
dist=Path(sys.argv[1])
root=Path(sys.argv[2])
if not dist.is_absolute():
  dist=root / dist
print(dist.resolve())
PY
)"

if [ "$SKIP_RELEASE_SNAPSHOT" != "1" ]; then
  echo "==> Building host release snapshot for artifact self-host proof..."
  "${MAKE:-make}" -C "$ROOT" release-snapshot \
    VERSION="$VERSION" \
    TARGETS="$TARGETS" \
    REQUIRE_TARGETS="$REQUIRE_TARGETS" \
    DIST="$dist_abs"
else
  echo "==> Using existing release snapshot in $dist_abs..."
fi

manifest_path="$dist_abs/manifest.json"
checksums_path="$dist_abs/checksums.txt"
test -f "$manifest_path"
test -f "$checksums_path"

selection_json="$TMPDIR_ROOT/selection.json"
python3 - "$manifest_path" "$checksums_path" "$dist_abs" "$host_os" "$host_arch" > "$selection_json" <<'PY'
import hashlib, json, sys
from pathlib import Path

manifest_path=Path(sys.argv[1])
checksums_path=Path(sys.argv[2])
dist=Path(sys.argv[3])
host_os=sys.argv[4]
host_arch=sys.argv[5]
manifest=json.loads(manifest_path.read_text())
checksums={}
for line in checksums_path.read_text().splitlines():
  parts=line.split()
  if len(parts) >= 2:
    checksums[parts[1]]=parts[0]

matches=[
  artifact for artifact in manifest.get("artifacts", [])
  if artifact.get("os") == host_os
  and artifact.get("arch") == host_arch
  and artifact.get("status") == "built"
]
if len(matches) != 1:
  raise SystemExit(f"expected exactly one built host artifact for {host_os}/{host_arch}, got {len(matches)}")
artifact=matches[0]
name=artifact.get("name", "")
archive=dist / name
if not archive.is_file():
  raise SystemExit(f"selected artifact missing on disk: {archive}")
manifest_sha=artifact.get("sha256", "")
checksum_sha=checksums.get(name, "")
if not manifest_sha:
  raise SystemExit(f"{name}: manifest sha256 is empty")
if not checksum_sha:
  raise SystemExit(f"{name}: missing from checksums.txt")
if manifest_sha != checksum_sha:
  raise SystemExit(f"{name}: manifest/checksums sha256 mismatch")
actual=hashlib.sha256(archive.read_bytes()).hexdigest()
if actual != manifest_sha:
  raise SystemExit(f"{name}: file sha256 mismatch")
print(json.dumps({
  "artifact_name": name,
  "artifact_path": str(archive),
  "sha256": actual,
  "os": host_os,
  "arch": host_arch,
}))
PY

artifact_name="$(json_get "$selection_json" artifact_name)"
artifact_path="$(json_get "$selection_json" artifact_path)"
artifact_sha="$(json_get "$selection_json" sha256)"

echo "==> Selected release artifact: $artifact_name"
echo "==> Artifact SHA256: $artifact_sha"

python3 - "$artifact_path" "$artifact_name" "$ROOT" <<'PY'
import re, sys, tarfile, zipfile
from pathlib import Path

archive_path=Path(sys.argv[1])
archive_name=sys.argv[2]
repo_root=str(Path(sys.argv[3]).resolve())
home=str(Path.home())
private_key_re=re.compile(r"-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----")
bearer_re=re.compile(r"Authorization:\s*Bearer\s+([A-Za-z0-9._~+/=-]{12,})", re.I)
safe_bearer_markers=("example","fake","test","dummy","redacted","planner-token","gateway-token","token")
local_path_res=[
  re.compile(re.escape(repo_root)),
  re.compile(re.escape(home)),
]
if home.startswith("/Users/"):
  local_path_res.append(re.compile(r"/Users/[^/\s]+/Projects/codencer\b"))
runtime_parts=(
  ".codencer/",
  "session.json",
  "machine.json",
  "projects.json",
  "connector/config.json",
  "reports/gateway-console-screenshots/",
  "proof-bundles/",
)
forbidden_name_parts=(
  "id_rsa",
  "id_ed25519",
  "private-key",
  "private_key",
  "service-account",
  "credentials.json",
  ".tfvars",
)

def is_safe_bearer(value: str) -> bool:
  lowered=value.lower()
  return any(marker in lowered for marker in safe_bearer_markers)

def members():
  if archive_path.name.endswith(".tar.gz"):
    with tarfile.open(archive_path, "r:gz") as archive:
      for info in archive.getmembers():
        extracted=archive.extractfile(info) if info.isfile() else None
        yield info.name, extracted.read() if extracted is not None else b""
    return
  if archive_path.suffix == ".zip":
    with zipfile.ZipFile(archive_path) as archive:
      for info in archive.infolist():
        yield info.filename, b"" if info.is_dir() else archive.read(info)
    return
  raise SystemExit(f"unsupported release archive type: {archive_path}")

failures=[]
for name, data in members():
  clean=name.replace("\\", "/")
  parts=Path(clean).parts
  lowered=clean.lower()
  if clean.startswith("/") or ".." in parts:
    failures.append(f"path traversal or absolute archive member: {clean}")
  if clean.endswith(".env") and not clean.endswith(".env.example"):
    failures.append(f"non-example env file: {clean}")
  if clean.endswith((".db", ".sqlite", ".sqlite3")):
    failures.append(f"runtime database file: {clean}")
  if clean.endswith(".png") or clean.startswith("reports/"):
    failures.append(f"generated report/screenshot artifact: {clean}")
  if any(part in clean for part in runtime_parts):
    failures.append(f"local runtime state: {clean}")
  if any(part in lowered for part in forbidden_name_parts):
    failures.append(f"secret/config-looking file name: {clean}")
  if b"\x00" in data:
    continue
  try:
    text=data.decode("utf-8")
  except UnicodeDecodeError:
    continue
  if private_key_re.search(text):
    failures.append(f"private key block in {clean}")
  for pattern in local_path_res:
    if pattern.search(text):
      failures.append(f"operator-specific absolute path in {clean}")
      break
  for match in bearer_re.finditer(text):
    if not is_safe_bearer(match.group(1)):
      failures.append(f"unredacted bearer token-looking value in {clean}")

if failures:
  print(f"release artifact safety check failed for {archive_name}", file=sys.stderr)
  for failure in failures:
    print(f"- {failure}", file=sys.stderr)
  raise SystemExit(1)
PY

unpack_dir="$TMPDIR_ROOT/unpacked"
mkdir -p "$unpack_dir"
case "$artifact_path" in
  *.tar.gz)
    tar -xzf "$artifact_path" -C "$unpack_dir"
    ;;
  *.zip)
    python3 - "$artifact_path" "$unpack_dir" <<'PY'
import sys, zipfile
with zipfile.ZipFile(sys.argv[1]) as archive:
  archive.extractall(sys.argv[2])
PY
    ;;
  *)
    echo "unsupported release archive type: $artifact_path" >&2
    exit 1
    ;;
esac

bin_dirs_file="$TMPDIR_ROOT/bin-dirs.txt"
find "$unpack_dir" -type d -name bin -print > "$bin_dirs_file"
bin_dir_count="$(wc -l < "$bin_dirs_file" | tr -d ' ')"
if [ "$bin_dir_count" != "1" ]; then
  echo "expected exactly one unpacked bin directory, got $bin_dir_count" >&2
  cat "$bin_dirs_file" >&2
  exit 1
fi
bin_dir="$(cat "$bin_dirs_file")"
case "$bin_dir" in
  "$ROOT/bin"|"$ROOT/bin/"*)
    echo "artifact proof resolved to source-tree bin directory: $bin_dir" >&2
    exit 1
    ;;
esac

for binary in codencer orchestratord codencer-relayd codencer-gatewayd codencer-connectord; do
  if [ ! -x "$bin_dir/$binary" ]; then
    echo "required unpacked binary is missing or not executable: $bin_dir/$binary" >&2
    exit 1
  fi
done

manifest_file="$TMPDIR_ROOT/fake-success.yaml"
cat > "$manifest_file" <<'YAML'
version: codencer.io/v1alpha1
kind: RunManifest
metadata:
  name: fake-success
project:
  id: codencer
execution:
  adapter: fake
  profile: fake-success
policy:
  stop_on_blocker: true
  stop_on_failure: true
tasks:
  - id: fake-success
    title: Fake success
    goal: Complete a deterministic fake success task.
YAML

echo "==> Running self-host Gateway/Relay/Connector/MCP proof from unpacked artifact..."
CODENCER_BIN_DIR="$bin_dir" CODENCER_MANIFEST_FILE="$manifest_file" "$ROOT/scripts/verify_gateway.sh"

echo "--- Release Artifact Self-Host Proof ---"
echo "Artifact: $artifact_name"
echo "Archive:  $artifact_path"
echo "Unpacked: $unpack_dir"
echo "Bin dir:  $bin_dir"
echo "Binaries: codencer orchestratord codencer-relayd codencer-gatewayd codencer-connectord"
echo "Safety:   archive names/content checked"
