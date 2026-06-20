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
    untracked = subprocess.run(
        ["git", "ls-files", "--others", "--exclude-standard"],
        cwd=ROOT,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=True,
    )
    values = []
    seen = set()
    for line in tracked.stdout.splitlines() + untracked.stdout.splitlines():
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
    if rel.startswith("docs/") or rel.startswith("web/gateway-console/"):
        return Path(rel).suffix in TEXT_SUFFIXES
    return False


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


def check_tracked_file(rel: str, failures: list[str]) -> None:
    path = ROOT / rel
    name = Path(rel).name
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


def scan_source_tree(failures: list[str]) -> None:
    for rel in repository_files():
        check_tracked_file(rel, failures)


def check_self_host_default_files(failures: list[str]) -> None:
    for rel in SELF_HOST_DEFAULT_FILES:
        text = read_text(ROOT / rel)
        if text is None:
            fail(f"self-host default file missing or unreadable: {rel}", failures)
            continue
        if OFFICIAL_ENDPOINT_RE.search(text) or "https://app.codencer.dev" in text:
            fail(f"commercial endpoint must not be a public default in {rel}", failures)


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
    check_self_host_default_files(failures)
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
