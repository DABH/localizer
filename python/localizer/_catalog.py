# Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
# SPDX-License-Identifier: NCSA

"""Translation catalogs: one JSON file per language, named "<BCP 47 tag>.json", mapping each English
source string to its translation. Catalogs ship inside the CLI's package and are read with
importlib.resources, so they work from wheels, zip apps and source checkouts alike."""

from __future__ import annotations

import json
import os
from dataclasses import dataclass, field
from importlib import resources
from pathlib import Path

try:
    from importlib.resources.abc import Traversable
except ImportError:  # Python 3.10
    from importlib.abc import Traversable

__all__ = ["File", "VERSION", "resolve", "languages", "load", "dumps"]

VERSION = 1


@dataclass
class File:
    """One language's catalog."""

    language: str
    messages: dict[str, str] = field(default_factory=dict)
    version: int = VERSION
    format: str = "python"  # placeholder syntax of the keys


def resolve(locales: str | os.PathLike[str] | Traversable) -> Traversable:
    """Turns a package name ("yourcli.locales"), a path or a Traversable into a Traversable."""
    if isinstance(locales, str):
        if os.sep in locales or "/" in locales or os.path.isdir(locales):
            return Path(locales)
        return resources.files(locales)
    if isinstance(locales, os.PathLike):
        return Path(locales)
    return locales


def languages(root: Traversable) -> list[str]:
    """The catalog languages in ``root``: "<tag>.json" files, ignoring "_"- and "."-prefixed names."""
    try:
        entries = list(root.iterdir())
    except (OSError, TypeError, AttributeError):
        return []
    out = []
    for e in entries:
        name = e.name
        if not name.endswith(".json") or name.startswith(("_", ".")):
            continue
        try:
            if not e.is_file():
                continue
        except OSError:
            continue
        out.append(name[: -len(".json")])
    return sorted(out)


def load(root: Traversable, lang: str) -> File:
    """Reads the catalog for ``lang`` from ``root``."""
    data = (root / (lang + ".json")).read_text(encoding="utf-8")
    raw = json.loads(data)
    if not isinstance(raw, dict):
        raise ValueError("catalog: not an object")
    version = raw.get("version", VERSION)
    if not isinstance(version, int) or version > VERSION:
        raise ValueError(f"catalog: unsupported version {version!r} (this build understands up to {VERSION})")
    messages = raw.get("messages") or {}
    if not isinstance(messages, dict) or not all(isinstance(k, str) and isinstance(v, str) for k, v in messages.items()):
        raise ValueError("catalog: messages must map strings to strings")
    return File(language=str(raw.get("language", lang)), messages=messages, version=version, format=str(raw.get("format", "")))


def dumps(f: File) -> str:
    """Renders a catalog canonically, byte for byte as the Go tooling does: version, language, format
    (when set), then messages with sorted keys, two-space indent, raw UTF-8 and a trailing newline."""
    doc: dict[str, object] = {"version": f.version or VERSION, "language": f.language}
    if f.format:
        doc["format"] = f.format
    doc["messages"] = dict(sorted(f.messages.items(), key=lambda kv: kv[0].encode("utf-8")))
    out = json.dumps(doc, ensure_ascii=False, indent=2)
    # Go's encoder always escapes these two, which JSON parsers otherwise accept raw.
    return out.replace("\u2028", "\\u2028").replace("\u2029", "\\u2029") + "\n"
