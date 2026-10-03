#!/usr/bin/env python3
"""Checks that every internal link in the built site points at a file.

    python3 site/scripts/check-links.py   # after `npm run build` in site/

Reads every HTML file under site/dist and resolves each href and src that isn't external (no scheme, not
protocol-relative), ignoring the query and the fragment: a path must be a file, or a directory with an
index.html. Prints the links that resolve to nothing and exits 1 when there are any.
"""

from __future__ import annotations

import posixpath
import re
import sys
from pathlib import Path

DIST = Path(__file__).resolve().parents[1] / "dist"
LINK = re.compile(r'\b(?:href|src)="([^"]*)"')
EXTERNAL = re.compile(r"^(?:[a-z][a-z0-9+.-]*:|//)", re.I)


def resolves(path: str) -> bool:
    target = DIST / path.lstrip("/")
    return target.is_file() or (target / "index.html").is_file()


def main() -> int:
    pages = sorted(DIST.rglob("*.html"))
    if not pages:
        sys.exit(f"check-links.py: no HTML under {DIST}; run `npm run build` in site/ first")
    broken = 0
    for page in pages:
        parent = page.relative_to(DIST).parent.as_posix()
        base = "/" if parent == "." else f"/{parent}/"
        for raw in sorted(set(LINK.findall(page.read_text(encoding="utf-8")))):
            target = raw.split("#", 1)[0].split("?", 1)[0]
            if not target or EXTERNAL.match(target):
                continue
            path = target if target.startswith("/") else posixpath.normpath(posixpath.join(base, target))
            if not resolves(path):
                print(f"{page.relative_to(DIST)}: {raw}")
                broken += 1
    print(f"{len(pages)} pages checked, {broken} broken internal links")
    return 1 if broken else 0


if __name__ == "__main__":
    sys.exit(main())
