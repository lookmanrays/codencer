#!/usr/bin/env python3
"""Public release boundary checks for the Codencer repository."""

from __future__ import annotations

import json
import os
import re
import subprocess
import sys
import tarfile
import zipfile
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]

REQUIRED_FILES = [
    "LICENSE",
    "NOTICE",
    "TRADEMARKS.md",
    "SECURITY.md",
    "CONTRIBUTING.md",
    "CODE_OF_CONDUCT.md",
    "README.md",
    "docs/README.md",
    "docs/architecture/README.md",
    "docs/architecture/public-private-boundary.md",
    "docs/architecture/official-vs-self-host.md",
    "docs/gateway-console.md",
    "docs/deployment/self-host-production.md",
    "docs/mcp/self-host-mcp-proof.md",
    "docs/relay-profile-registry.md",
    "docs/acceptance/public-self-host-release.md",
    "docs/acceptance/public-repo-release.yaml",
    ".github/workflows/release-assets.yml",
]

TEXT_REQUIREMENTS = [
    ("LICENSE", "Apache License"),
    ("NOTICE", "Lukman Nuriakhmetov and Codencer Contributors"),
    ("TRADEMARKS.md", "mcp.codencer.dev"),
    ("README.md", "Public/self-built Codencer defaults to self-host/local endpoints"),
    ("README.md", "codencer setup self-host"),
    ("docs/architecture/public-private-boundary.md", "Private Managed Service Candidates"),
    ("docs/architecture/official-vs-self-host.md", "The public release path is Gateway-first and self-hosted"),
    ("docs/gateway-console.md", "public/self-host Gateway Console live integration implemented"),
    ("docs/deployment/self-host-production.md", "CLI flags > env vars > user config profile > build-time defaults > self-host defaults"),
    ("docs/mcp/self-host-mcp-proof.md", "codencer.run_project_manifest"),
    ("docs/acceptance/public-self-host-release.md", "make verify-public-selfhost-release"),
    ("docs/architecture/mcp-gateway-model.md", "Direct Relay MCP"),
    ("docs/acceptance/public-repo-release.yaml", "public_managed_codencer_cloud_launch: no_go"),
]

GROVE_ACTIVE_DOC_FILES = [
    "README.md",
    "docs/README.md",
    "docs/EXAMPLES.md",
    "docs/project-config.md",
]

GROVE_CONTRACT_DOC_FILES = [
    "docs/EXAMPLES.md",
    "docs/project-config.md",
]

GROVE_CONTRACT_PHRASES = [
    "Codencer is Grove-compatible.",
    "Codencer can read a safe subset of `grove.yaml` and `.groverc.json`.",
    "Native `.codencer/workspace.json` has precedence.",
    "Codencer does not depend on the Grove CLI.",
    "Codencer does not write Grove state files.",
]

SELF_HOST_DEFAULT_FILES = [
    "internal/defaults/defaults.go",
    "internal/account/session.go",
    "internal/gateway/config.go",
    "internal/setup/setup.go",
    "internal/activation/activation.go",
    "internal/release/release.go",
    "cmd/codencer/main.go",
    "web/gateway-console/api/demo-data.ts",
    "web/gateway-console/api/workspace.ts",
    "web/gateway-console/api/oauth.ts",
]

PRIVATE_KEY_RE = re.compile(r"-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----")
BEARER_RE = re.compile(r"Authorization:\s*Bearer\s+([A-Za-z0-9._~+/=-]{12,})")
CURRENT_HOME = str(Path.home())
CURRENT_REPO = str(ROOT)
LOCAL_PATH_PATTERNS = [
    re.escape(CURRENT_HOME),
    re.escape(CURRENT_REPO),
]
if CURRENT_HOME.startswith("/Users/"):
    LOCAL_PATH_PATTERNS.append(r"/Users/[^/\s]+/Projects/codencer\b")
LOCAL_PATH_RE = re.compile("|".join(LOCAL_PATH_PATTERNS))
OFFICIAL_ENDPOINT_RE = re.compile(r"https://(?:mcp|relay)\.codencer\.dev(?:/mcp)?")
PRIMARY_STALE_RE = re.compile(
    r"\b(beta|alpha|staging)\b"
    r"|v0\.2"
    r"|https://(?:mcp|relay|app)\.codencer\.dev(?:/mcp)?"
    r"|default managed Relay"
    r"|official managed Relay"
    r"|activation official"
    r"|setup gateway",
    re.IGNORECASE,
)
ACTIVE_RELEASE_LABEL_RE = re.compile(
    r"\b(beta|alpha|staging)\b|v0\.2|verify-beta|verify_beta|beta-track",
    re.IGNORECASE,
)
PRIVATE_MANAGED_SERVICE_PATH_RE = re.compile(
    r"(^|/)(billing|metering|quotas?|plans?|kms|vault|managed[-_]?runners?|"
    r"support[-_]?admin|admin[-_]?console|marketplace[-_]?submission|"
    r"official[-_]?connector[-_]?(credentials?|secrets?))(/|\.|$)",
    re.IGNORECASE,
)
PARTIAL_RC_VERDICT_CLAIM_RE = re.compile(
    r"reports\s+`?PARTIAL`?|`?PARTIAL`?\s+instead\s+of\s+`?GO`?",
    re.IGNORECASE,
)
SECRET_ASSIGNMENT_RE = re.compile(
    r"(?i)\b[A-Z0-9_]*(TOKEN|SECRET|PASSWORD|PRIVATE_KEY|CLIENT_SECRET)[A-Z0-9_]*\s*[:=]\s*['\"]?([^'\"\s#]+)"
)

SAFE_SECRET_VALUES = (
    "$",
    "<",
    "{",
    "}",
    "example",
    "fake",
    "test",
    "dummy",
    "redacted",
    "smoke",
    "planner-token",
    "gateway-token",
    "gateway-dev-token",
    "official-relay-token",
    "selfhost-relay-token",
    "secret",
    "token",
)

ALLOWED_ENDPOINT_PREFIXES = (
    "AGENTS.md",
    "README.md",
    "TRADEMARKS.md",
    "NOTICE",
    "docs/",
    "internal/",
    "cmd/",
    "scripts/",
    "web/gateway-console/",
)

TEXT_SUFFIXES = {
    ".go",
    ".md",
    ".txt",
    ".yaml",
    ".yml",
    ".json",
    ".toml",
    ".sh",
    ".py",
    ".js",
    ".ts",
    ".tsx",
    ".html",
    ".css",
    ".example",
}

RUNTIME_NAME_PARTS = (
    ".codencer/",
    "session.json",
    "machine.json",
    "projects.json",
    "connector/config.json",
)


def fail(message: str, failures: list[str]) -> None:
    failures.append(message)


def repository_files() -> list[str]:
    tracked = subprocess.run(
        ["git", "ls-files"],
        cwd=ROOT,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=True,
    )
    values = []
    seen = set()
    for line in tracked.stdout.splitlines():
        rel = line.strip()
        if not rel or rel in seen:
            continue
        seen.add(rel)
        values.append(rel)
    return values


def read_text(path: Path) -> str | None:
    try:
        data = path.read_bytes()
    except OSError:
        return None
    if b"\x00" in data:
        return None
    try:
        return data.decode("utf-8")
    except UnicodeDecodeError:
        return None


def is_example_env(rel: str) -> bool:
    name = Path(rel).name
    return name == ".env.example" or name.endswith(".env.example") or rel.endswith(".env.example")


def is_safe_placeholder(value: str) -> bool:
    if value.startswith(("z.", "relay.", "process.env.", "env.")):
        return True
    lowered = value.lower()
    return any(marker in lowered for marker in SAFE_SECRET_VALUES)


def should_scan_secret_assignment(rel: str) -> bool:
    if rel.endswith(("_test.go", ".golden")):
        return False
    if rel.startswith("docs/archive/") or rel.startswith("docs/internal/"):
        return False
    if rel.startswith("scripts/"):
        return False
    if rel.endswith(".go"):
        return False
    return True


def should_scan_primary_stale_reference(rel: str) -> bool:
    if rel.startswith("docs/archive/") or rel.startswith("docs/internal/"):
        return False
    if rel.endswith("package-lock.json"):
        return False
    if rel == "TRADEMARKS.md":
        return False
    if rel in {"README.md", "CONTRIBUTING.md", "LICENSE", "NOTICE"}:
        return True
    if rel == "Makefile" or rel.startswith(".github/"):
        return True
    if rel.startswith("docs/") or rel.startswith("web/gateway-console/"):
        return Path(rel).suffix in TEXT_SUFFIXES
    return False


def should_scan_active_release_labels(rel: str) -> bool:
    if rel == "scripts/check_public_boundary.py":
        return False
    return rel == "Makefile" or rel.startswith(".github/") or rel.startswith("deploy/") or rel.startswith("scripts/")


def should_scan_private_managed_service_path(rel: str) -> bool:
    if rel.startswith("docs/") or rel.startswith("reports/"):
        return False
    return rel.startswith(("cmd/", "deploy/", "internal/", "scripts/", "web/"))


def endpoint_allowed(rel: str) -> bool:
    return rel.startswith(ALLOWED_ENDPOINT_PREFIXES)


def check_required_files(failures: list[str]) -> None:
    for rel in REQUIRED_FILES:
        if not (ROOT / rel).is_file():
            fail(f"missing required public release file: {rel}", failures)


def check_text_requirements(failures: list[str]) -> None:
    for rel, needle in TEXT_REQUIREMENTS:
        text = read_text(ROOT / rel)
        if text is None or needle not in text:
            fail(f"{rel} missing required text: {needle}", failures)


def normalized_contains(text: str, needle: str) -> bool:
    return " ".join(needle.split()) in " ".join(text.split())


def check_active_grove_docs(failures: list[str]) -> None:
    source = read_text(ROOT / "internal/workspace/config.go") or ""
    if "grove.yaml" not in source or ".groverc.json" not in source:
        return

    for rel in GROVE_ACTIVE_DOC_FILES:
        text = read_text(ROOT / rel)
        if text is None or "Grove" not in text:
            fail(f"{rel} must mention active Grove compatibility while workspace config supports Grove files", failures)

    contract_text = "\n".join(read_text(ROOT / rel) or "" for rel in GROVE_CONTRACT_DOC_FILES)
    for phrase in GROVE_CONTRACT_PHRASES:
        if not normalized_contains(contract_text, phrase):
            fail(f"active Grove docs missing compatibility contract phrase: {phrase}", failures)


def check_tracked_file(rel: str, failures: list[str]) -> None:
    path = ROOT / rel
    name = Path(rel).name
    if should_scan_private_managed_service_path(rel) and PRIVATE_MANAGED_SERVICE_PATH_RE.search(rel):
        fail(f"private managed-service path is not allowed in public repository: {rel}", failures)
    if name.startswith(".env") and not is_example_env(rel):
        fail(f"tracked non-example env file is not allowed: {rel}", failures)
    if any(part in rel for part in RUNTIME_NAME_PARTS):
        fail(f"tracked local runtime state is not allowed: {rel}", failures)
    if rel.startswith("dist/"):
        fail(f"release output must not be tracked: {rel}", failures)

    text = read_text(path)
    if text is None:
        return
    if PRIVATE_KEY_RE.search(text):
        fail(f"private key block found in tracked file: {rel}", failures)
    if LOCAL_PATH_RE.search(text):
        fail(f"operator-specific absolute path found in tracked file: {rel}", failures)
    for match in BEARER_RE.finditer(text):
        value = match.group(1)
        if not is_safe_placeholder(value):
            fail(f"unredacted bearer token-looking value in {rel}", failures)
    if OFFICIAL_ENDPOINT_RE.search(text) and not endpoint_allowed(rel):
        fail(f"official public endpoint appears outside docs/code/examples: {rel}", failures)
    if should_scan_secret_assignment(rel):
        for match in SECRET_ASSIGNMENT_RE.finditer(text):
            value = match.group(2)
            if len(value) >= 12 and not is_safe_placeholder(value):
                fail(f"secret-looking assignment in tracked file {rel}: {match.group(0)}", failures)
    if should_scan_primary_stale_reference(rel):
        for line_number, line in enumerate(text.splitlines(), start=1):
            if PRIMARY_STALE_RE.search(line):
                fail(f"stale primary release reference in {rel}:{line_number}: {line.strip()}", failures)
            if PARTIAL_RC_VERDICT_CLAIM_RE.search(line):
                fail(f"stale public self-host RC verdict claim in {rel}:{line_number}: {line.strip()}", failures)
    elif should_scan_active_release_labels(rel):
        for line_number, line in enumerate(text.splitlines(), start=1):
            if ACTIVE_RELEASE_LABEL_RE.search(line):
                fail(f"stale active release label in {rel}:{line_number}: {line.strip()}", failures)


def scan_source_tree(failures: list[str]) -> None:
    for rel in repository_files():
        check_tracked_file(rel, failures)


def check_hardening_final_report(failures: list[str]) -> None:
    rel = "reports/public-selfhost-hardening/final-report.md"
    text = read_text(ROOT / rel)
    if text is None:
        fail(f"missing hardening final report: {rel}", failures)
        return
    lines = [line.strip() for line in text.splitlines() if line.strip()]
    if not lines or lines[-1] not in {"Verdict: GO", "Verdict: NO-GO"}:
        fail(f"{rel} must end with exactly 'Verdict: GO' or 'Verdict: NO-GO'", failures)


def check_self_host_default_files(failures: list[str]) -> None:
    for rel in SELF_HOST_DEFAULT_FILES:
        text = read_text(ROOT / rel)
        if text is None:
            fail(f"self-host default file missing or unreadable: {rel}", failures)
            continue
        if OFFICIAL_ENDPOINT_RE.search(text) or "https://app.codencer.dev" in text:
            fail(f"commercial endpoint must not be a public default in {rel}", failures)


def check_release_workflows(failures: list[str]) -> None:
    release_assets = read_text(ROOT / ".github/workflows/release-assets.yml") or ""
    release_please = read_text(ROOT / ".github/workflows/release-please.yml") or ""
    release_config = read_text(ROOT / "release-please-config.json") or ""

    required_assets_needles = [
        "workflow_call:",
        "workflow_dispatch:",
        "replace_existing:",
        "gh release view \"$TAG_NAME\" --repo \"$GITHUB_REPOSITORY\"",
        "gh release upload \"$TAG_NAME\"",
        "codencer_${TAG_NAME}_linux_amd64.tar.gz",
        "codencer_${TAG_NAME}_darwin_arm64.tar.gz",
        "codencer_${TAG_NAME}_darwin_amd64.tar.gz",
        "darwin/arm64,darwin/amd64",
        "manifest.json",
        "checksums.txt",
        "built_at: ${{ steps.resolve.outputs.built_at }}",
        "BUILT_AT: ${{ needs.preflight.outputs.built_at }}",
    ]
    for needle in required_assets_needles:
        if needle not in release_assets:
            fail(f"release-assets workflow missing required text: {needle}", failures)
    if "--clobber" in release_assets:
        fail("release-assets workflow must not silently clobber release assets", failures)
    if "datetime.now" in release_assets:
        fail("release-assets manifest generation must not use retry-variant wall-clock timestamps", failures)
    if "expected exactly one darwin" in release_assets.lower():
        fail("release-assets workflow must not publish only one darwin host artifact", failures)
    if "make verify-release\n" in release_assets or "make verify-release " in release_assets:
        fail("release-assets workflow must not run make verify-release after building tag artifacts because it rewrites dist", failures)

    if "uses: ./.github/workflows/release-assets.yml" not in release_please:
        fail("release-please workflow must call the reusable release-assets workflow", failures)
    if "release_created == 'true'" not in release_please:
        fail("release-please workflow must gate release asset publishing on release_created", failures)
    if "build-linux-amd64:" in release_please or "build-macos-host:" in release_please:
        fail("release-please workflow should not duplicate release asset build jobs", failures)

    try:
        config = json.loads(release_config)
    except json.JSONDecodeError as exc:
        fail(f"release-please-config.json is not valid JSON: {exc}", failures)
        return
    if "release-as" in config:
        fail("release-please-config.json must not keep one-time release-as after v0.3.0", failures)


def check_install_script(failures: list[str]) -> None:
    install_script = read_text(ROOT / "scripts/install.sh") or ""
    readme = read_text(ROOT / "README.md") or ""
    release_automation = read_text(ROOT / "docs" / "release-automation.md") or ""
    release_checklist = read_text(ROOT / "docs" / "release-checklist.md") or ""
    version = (read_text(ROOT / "version.txt") or "").strip()
    release_manifest_text = read_text(ROOT / ".release-please-manifest.json") or "{}"
    expected_tag = f"v{version}" if version else ""
    try:
        release_manifest = json.loads(release_manifest_text)
    except json.JSONDecodeError as exc:
        fail(f".release-please-manifest.json is not valid JSON: {exc}", failures)
        release_manifest = {}
    manifest_version = str(release_manifest.get(".", "")).strip()
    if not version:
        fail("version.txt must contain the current release automation version", failures)
    if manifest_version and manifest_version != version:
        fail(f"version.txt ({version}) and .release-please-manifest.json ({manifest_version}) disagree", failures)
    required_needles = [
        "MODE=\"release-bootstrap\"",
        "https://github.com/$REPO/releases/download/$VERSION",
        "checksums.txt",
        "manifest.json",
        "verify_manifest",
        "json_syntax_valid",
        "json_compact_preserve_strings",
        "json_top_level_string",
        "json_top_level_key_count",
        "json_asset_records",
        "sha256sum or shasum",
        "planned_assets",
        "VERSION=\"latest\"",
        "Windows-native Codencer artifacts are not published yet. Use WSL2/Linux artifact for now.",
    ]
    for needle in required_needles:
        if needle not in install_script:
            fail(f"install.sh missing required release-bootstrap marker: {needle}", failures)
    if 'BIN_DIR="bin"' in install_script:
        fail("install.sh must not use caller-cwd bin as an implicit default", failures)
    if "gh release" in install_script:
        fail("install.sh must not require the gh CLI for one-command installs", failures)
    if "python3" in install_script or "python " in install_script:
        fail("install.sh must not use Python for one-command installer manifest verification", failures)
    if "tr -d '\\n\\r\\t '" in install_script:
        fail("install.sh must not delete whitespace inside manifest JSON string values", failures)
    for doc_name, text in {
        "README.md": readme,
        "docs/release-automation.md": release_automation,
        "docs/release-checklist.md": release_checklist,
    }.items():
        if "https://codencer.dev/install.sh" in text:
            fail(f"{doc_name} must not advertise codencer.dev/install.sh", failures)
        if "v0.3.1" in text:
            fail(f"{doc_name} still contains stale v0.3.1 installer wording", failures)
    if "GitHub Releases](https://github.com/lookmanrays/codencer/releases/latest)" not in readme:
        fail("README.md must direct users to GitHub Releases for the latest public release tag", failures)
    if 'TAG=<release-tag-from-github-releases>' not in readme or '--version "$TAG"' not in readme:
        fail("README.md must show tag-driven pinned install commands instead of a hardcoded release tag", failures)
    if expected_tag:
        for doc_name, text in {
            "README.md": readme,
            "docs/release-automation.md": release_automation,
            "docs/release-checklist.md": release_checklist,
        }.items():
            if f"--version {expected_tag}" in text or f"codencer_{expected_tag}_" in text:
                fail(f"{doc_name} must not hardcode the current release tag {expected_tag}; use TAG_NAME/TAG placeholders", failures)


def archive_members(path: Path) -> list[tuple[str, bytes]]:
    members: list[tuple[str, bytes]] = []
    if path.suffix == ".zip":
        with zipfile.ZipFile(path) as archive:
            for info in archive.infolist():
                if info.is_dir():
                    continue
                members.append((info.filename, archive.read(info)))
        return members
    if path.name.endswith(".tar.gz"):
        with tarfile.open(path, "r:gz") as archive:
            for info in archive.getmembers():
                if not info.isfile():
                    continue
                extracted = archive.extractfile(info)
                if extracted is None:
                    continue
                members.append((info.name, extracted.read()))
    return members


def check_archive_text(archive_name: str, member_name: str, data: bytes, failures: list[str]) -> None:
    if b"\x00" in data:
        return
    try:
        text = data.decode("utf-8")
    except UnicodeDecodeError:
        return
    if PRIVATE_KEY_RE.search(text):
        fail(f"private key block found in release artifact {archive_name}:{member_name}", failures)
    if LOCAL_PATH_RE.search(text):
        fail(f"operator-specific absolute path found in release artifact {archive_name}:{member_name}", failures)
    for match in BEARER_RE.finditer(text):
        if not is_safe_placeholder(match.group(1)):
            fail(f"unredacted bearer token-looking value in release artifact {archive_name}:{member_name}", failures)


def scan_release_artifacts(failures: list[str]) -> None:
    manifest_path = ROOT / "dist" / "manifest.json"
    if not manifest_path.exists():
        print("public-boundary: release artifact scan skipped; dist/manifest.json is not present")
        return
    try:
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        fail(f"release manifest cannot be read: {exc}", failures)
        return
    for artifact in manifest.get("artifacts", []):
        if artifact.get("status") != "built":
            continue
        name = artifact.get("name", "")
        path = ROOT / "dist" / name
        if not path.is_file():
            fail(f"built release artifact missing on disk: {name}", failures)
            continue
        try:
            members = archive_members(path)
        except (tarfile.TarError, zipfile.BadZipFile, OSError) as exc:
            fail(f"cannot inspect release artifact {name}: {exc}", failures)
            continue
        for member_name, data in members:
            clean = member_name.replace("\\", "/")
            if clean.endswith(".env") and not clean.endswith(".env.example"):
                fail(f"release artifact contains non-example env file: {name}:{clean}", failures)
            if clean.endswith((".db", ".sqlite", ".sqlite3")):
                fail(f"release artifact contains database file: {name}:{clean}", failures)
            if any(part in clean for part in RUNTIME_NAME_PARTS):
                fail(f"release artifact contains local runtime state: {name}:{clean}", failures)
            check_archive_text(name, clean, data, failures)


def main() -> int:
    os.chdir(ROOT)
    failures: list[str] = []
    check_required_files(failures)
    check_text_requirements(failures)
    check_active_grove_docs(failures)
    check_hardening_final_report(failures)
    check_self_host_default_files(failures)
    check_release_workflows(failures)
    check_install_script(failures)
    scan_source_tree(failures)
    scan_release_artifacts(failures)
    if failures:
        print("public-boundary: FAILED", file=sys.stderr)
        for item in failures:
            print(f"- {item}", file=sys.stderr)
        return 1
    print("public-boundary: OK")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
