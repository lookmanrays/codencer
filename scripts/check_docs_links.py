#!/usr/bin/env python3
"""Check local Markdown links in current public docs.

Archived legacy docs and internal historical notes are intentionally skipped by
default. They are preserved for history and may contain stale relative links.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path
from urllib.parse import unquote, urlparse


ROOT = Path(__file__).resolve().parents[1]
SKIP_DIR_PARTS = {
    ("docs", "archive"),
    ("docs", "internal"),
}
LINK_RE = re.compile(r"!?\[[^\]]*\]\(([^)\s]+)(?:\s+\"[^\"]*\")?\)")


def should_skip(path: Path) -> bool:
    rel = path.relative_to(ROOT)
    parts = rel.parts
    return any(parts[: len(skip)] == skip for skip in SKIP_DIR_PARTS)


def markdown_files() -> list[Path]:
    candidates = [ROOT / "README.md", ROOT / "CONTRIBUTING.md", ROOT / "TRADEMARKS.md"]
    candidates.extend(sorted((ROOT / "docs").rglob("*.md")))
    return [path for path in candidates if path.exists() and not should_skip(path)]


def is_external(target: str) -> bool:
    parsed = urlparse(target)
    return parsed.scheme in {"http", "https", "mailto", "tel"}


def strip_anchor(target: str) -> str:
    return target.split("#", 1)[0]


def check_file(path: Path) -> list[str]:
    errors: list[str] = []
    text = path.read_text(encoding="utf-8")
    for match in LINK_RE.finditer(text):
        raw = match.group(1).strip()
        if not raw or raw.startswith("#") or is_external(raw):
            continue
        target = unquote(strip_anchor(raw))
        if not target:
            continue
        resolved = (path.parent / target).resolve()
        try:
            resolved.relative_to(ROOT)
        except ValueError:
            errors.append(f"{path.relative_to(ROOT)}: link escapes repo: {raw}")
            continue
        if not resolved.exists():
            errors.append(f"{path.relative_to(ROOT)}: missing link target: {raw}")
    return errors


def main() -> int:
    errors: list[str] = []
    for path in markdown_files():
        errors.extend(check_file(path))
    if errors:
        print("Broken local Markdown links:", file=sys.stderr)
        for error in errors:
            print(f"- {error}", file=sys.stderr)
        return 1
    print("docs link check passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
